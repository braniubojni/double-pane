package remote

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/erikharutyunyan/double-pane/internal/filesystem"
	"github.com/pkg/sftp"
)

// Download copies remote virtual paths into a local directory.
func (m *Manager) Download(sources []string, localDestDir string) error {
	return m.DownloadCtx(context.Background(), sources, localDestDir, nil)
}

// DownloadCtx copies remote virtual paths into a local directory with progress.
func (m *Manager) DownloadCtx(ctx context.Context, sources []string, localDestDir string, onProgress filesystem.ProgressFunc) (err error) {
	if m == nil {
		return fmt.Errorf("remote not available")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	destAbs, err := filesystem.Resolve(localDestDir)
	if err != nil {
		return err
	}
	info, err := os.Stat(destAbs)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("destination is not a directory: %s", destAbs)
	}

	files, total, err := m.remoteSourcesStats(sources)
	if err != nil {
		return err
	}
	rep := newRemoteProgress(total, onProgress)
	if onProgress != nil {
		onProgress(filesystem.ProgressEvent{Total: total})
	}

	workers := filesystem.CopyWorkersWith(filesystem.IONetwork, []string{destAbs}, files)
	pool := newXferPool(ctx, workers, func(ctx context.Context, j sftpFileJob) error {
		return downloadFileCtx(ctx, j.c, j.src, j.dst, j.mode, rep)
	})

	var created []string
	defer func() {
		if errors.Is(err, context.Canceled) {
			for _, p := range created {
				_ = os.RemoveAll(p)
			}
		}
	}()

	var placeErr error
	for _, src := range sources {
		if err := ctx.Err(); err != nil {
			placeErr = err
			break
		}
		loc, err := ParseLocation(src)
		if err != nil {
			placeErr = err
			break
		}
		s, err := m.get(loc)
		if err != nil {
			placeErr = err
			break
		}
		base := path.Base(strings.TrimRight(loc.RemotePath, "/"))
		if base == "" || base == "." || base == "/" {
			placeErr = fmt.Errorf("invalid remote source: %s", src)
			break
		}
		st, err := s.sftp.Stat(loc.RemotePath)
		if err != nil {
			placeErr = err
			break
		}
		target := filesystem.UniquePath(filepath.Join(destAbs, base))
		created = append(created, target)
		rep.setDest(target, st.IsDir())
		if err := downloadPlace(ctx, s.sftp, loc.RemotePath, target, pool); err != nil {
			placeErr = err
			break
		}
	}
	if placeErr != nil {
		pool.cancel()
	}
	waitErr := pool.finish()
	if placeErr != nil {
		err = placeErr
		return err
	}
	if waitErr != nil {
		err = waitErr
		return err
	}
	rep.finish("")
	return nil
}

// Upload copies local paths into a remote virtual directory.
func (m *Manager) Upload(localSources []string, remoteDestDir string) error {
	return m.UploadCtx(context.Background(), localSources, remoteDestDir, nil)
}

// UploadCtx copies local paths into a remote virtual directory with progress.
func (m *Manager) UploadCtx(ctx context.Context, localSources []string, remoteDestDir string, onProgress filesystem.ProgressFunc) (err error) {
	if m == nil {
		return fmt.Errorf("remote not available")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	dloc, err := ParseLocation(remoteDestDir)
	if err != nil {
		return err
	}
	ds, err := m.get(dloc)
	if err != nil {
		return err
	}
	st, err := ds.sftp.Stat(dloc.RemotePath)
	if err != nil {
		return err
	}
	if !st.IsDir() {
		return fmt.Errorf("destination is not a directory: %s", remoteDestDir)
	}

	files, total, err := filesystem.CollectCopyStats(localSources)
	if err != nil {
		return err
	}
	rep := newRemoteProgress(total, onProgress)
	if onProgress != nil {
		onProgress(filesystem.ProgressEvent{Total: total})
	}

	workers := filesystem.CopyWorkersWith(filesystem.IONetwork, localSources, files)
	pool := newXferPool(ctx, workers, func(ctx context.Context, j sftpFileJob) error {
		return uploadFileCtx(ctx, ds.sftp, j.src, j.dst, j.mode, rep)
	})

	var created []string
	defer func() {
		if errors.Is(err, context.Canceled) {
			for _, p := range created {
				_ = removeAll(ds.sftp, p)
			}
		}
	}()

	var placeErr error
	for _, src := range localSources {
		if err := ctx.Err(); err != nil {
			placeErr = err
			break
		}
		srcAbs, err := filesystem.Resolve(src)
		if err != nil {
			placeErr = err
			break
		}
		base := filepath.Base(srcAbs)
		if base == "" || base == "." || base == string(filepath.Separator) {
			placeErr = fmt.Errorf("invalid local source: %s", src)
			break
		}
		dest := uniqueRemotePath(ds.sftp, path.Join(dloc.RemotePath, base))
		created = append(created, dest)
		srcInfo, statErr := os.Lstat(srcAbs)
		srcIsDir := statErr == nil && srcInfo.IsDir()
		rep.setDest(dloc.JoinPath(dest), srcIsDir)
		if err := uploadPlace(ctx, ds.sftp, srcAbs, dest, pool); err != nil {
			placeErr = err
			break
		}
	}
	if placeErr != nil {
		pool.cancel()
	}
	waitErr := pool.finish()
	if placeErr != nil {
		err = placeErr
		return err
	}
	if waitErr != nil {
		err = waitErr
		return err
	}
	rep.finish("")
	return nil
}

func (m *Manager) remoteSourcesStats(sources []string) (files int, total int64, err error) {
	for _, src := range sources {
		loc, err := ParseLocation(src)
		if err != nil {
			return 0, 0, err
		}
		s, err := m.get(loc)
		if err != nil {
			return 0, 0, err
		}
		nfiles, nbytes, err := remotePathStats(s.sftp, loc.RemotePath)
		if err != nil {
			return 0, 0, err
		}
		files += nfiles
		total += nbytes
	}
	return files, total, nil
}

func remotePathBytes(c *sftp.Client, remotePath string) (int64, error) {
	_, n, err := remotePathStats(c, remotePath)
	return n, err
}

func remotePathStats(c *sftp.Client, remotePath string) (files int, total int64, err error) {
	st, err := c.Lstat(remotePath)
	if err != nil {
		return 0, 0, err
	}
	if st.Mode()&os.ModeSymlink != 0 {
		return 0, 0, nil
	}
	if !st.IsDir() {
		return 1, st.Size(), nil
	}
	var walk func(string) error
	walk = func(p string) error {
		entries, err := c.ReadDir(p)
		if err != nil {
			return err
		}
		for _, e := range entries {
			child := path.Join(p, e.Name())
			if e.Mode()&os.ModeSymlink != 0 {
				continue
			}
			if e.IsDir() {
				if err := walk(child); err != nil {
					return err
				}
				continue
			}
			files++
			total += e.Size()
		}
		return nil
	}
	if err := walk(remotePath); err != nil {
		return 0, 0, err
	}
	return files, total, nil
}

type remoteProgress struct {
	mu        sync.Mutex
	done      int64
	total     int64
	destPath  string
	destSize  int64
	destIsDir bool
	on        filesystem.ProgressFunc
	lastEmit  time.Time
	force     bool
}

func newRemoteProgress(total int64, on filesystem.ProgressFunc) *remoteProgress {
	return &remoteProgress{total: total, on: on, force: true}
}

func (r *remoteProgress) eventLocked(currentPath string) filesystem.ProgressEvent {
	return filesystem.ProgressEvent{
		Done:        r.done,
		Total:       r.total,
		CurrentPath: currentPath,
		DestPath:    r.destPath,
		DestSize:    r.destSize,
		DestIsDir:   r.destIsDir,
	}
}

func (r *remoteProgress) setDest(path string, isDir bool) {
	if r == nil || r.on == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.destPath = path
	r.destSize = 0
	r.destIsDir = isDir
	r.lastEmit = time.Now()
	r.force = false
	r.on(r.eventLocked(""))
}

func (r *remoteProgress) add(n int64, currentPath string) {
	if r == nil || r.on == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.done += n
	r.destSize += n
	if r.total > 0 && r.done > r.total {
		r.done = r.total
	}
	now := time.Now()
	if !r.force && now.Sub(r.lastEmit) < 100*time.Millisecond && r.done < r.total {
		return
	}
	r.force = false
	r.lastEmit = now
	r.on(r.eventLocked(currentPath))
}

func (r *remoteProgress) finish(currentPath string) {
	if r == nil || r.on == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.total > 0 {
		r.done = r.total
	}
	r.on(r.eventLocked(currentPath))
}

// copyUnwrapped uses io.Copy so *sftp.File WriteTo/ReadFrom (concurrent
// packets) stay enabled. Progress is polled from destSize; cancel closes src/dst.
func copyUnwrapped(ctx context.Context, dst io.Writer, src io.Reader, path string, knownSize int64, destSize func() int64, rep *remoteProgress) error {
	stop := context.AfterFunc(ctx, func() {
		if c, ok := src.(io.Closer); ok {
			_ = c.Close()
		}
		if c, ok := dst.(io.Closer); ok {
			_ = c.Close()
		}
	})
	defer stop()

	var last int64
	flush := func() {
		if destSize != nil {
			n := destSize()
			if n > last {
				rep.add(n-last, path)
				last = n
			}
			return
		}
		if knownSize > last {
			rep.add(knownSize-last, path)
			last = knownSize
		}
	}

	errCh := make(chan error, 1)
	go func() {
		_, err := io.Copy(dst, src)
		errCh <- err
	}()
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case err := <-errCh:
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if err == nil || destSize != nil {
				flush()
			}
			return err
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
			if destSize != nil {
				flush()
			}
		}
	}
}

type sftpFileJob struct {
	c        *sftp.Client
	src, dst string
	mode     os.FileMode
}

func downloadPlace(ctx context.Context, c *sftp.Client, remotePath, localPath string, pool *xferPool[sftpFileJob]) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	st, err := c.Stat(remotePath)
	if err != nil {
		return err
	}
	if !st.IsDir() {
		return pool.enqueue(sftpFileJob{c: c, src: remotePath, dst: localPath, mode: st.Mode()})
	}
	perm := st.Mode().Perm()
	if perm == 0 {
		perm = 0o755
	}
	if err := os.MkdirAll(localPath, perm); err != nil {
		return err
	}
	entries, err := c.ReadDir(remotePath)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		name := e.Name()
		if err := downloadPlace(ctx, c, path.Join(remotePath, name), filepath.Join(localPath, name), pool); err != nil {
			return err
		}
	}
	return nil
}

func downloadFileCtx(ctx context.Context, c *sftp.Client, remotePath, localPath string, mode os.FileMode, rep *remoteProgress) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	in, err := c.Open(remotePath)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	perm := mode.Perm()
	if perm == 0 {
		perm = 0o644
	}
	out, err := os.OpenFile(localPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	var known int64
	if st, err := in.Stat(); err == nil {
		known = st.Size()
	}
	destSize := func() int64 {
		st, err := out.Stat()
		if err != nil {
			return 0
		}
		return st.Size()
	}
	if err := copyUnwrapped(ctx, out, in, remotePath, known, destSize, rep); err != nil {
		return err
	}
	return out.Close()
}

func uploadPlace(ctx context.Context, c *sftp.Client, localPath, remotePath string, pool *xferPool[sftpFileJob]) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	info, err := os.Lstat(localPath)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		info, err = os.Stat(localPath)
		if err != nil {
			return err
		}
	}
	if !info.IsDir() {
		return pool.enqueue(sftpFileJob{c: c, src: localPath, dst: remotePath, mode: info.Mode()})
	}
	if err := c.MkdirAll(remotePath); err != nil {
		return err
	}
	_ = c.Chmod(remotePath, info.Mode().Perm())
	entries, err := os.ReadDir(localPath)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		name := e.Name()
		if err := uploadPlace(ctx, c, filepath.Join(localPath, name), path.Join(remotePath, name), pool); err != nil {
			return err
		}
	}
	return nil
}

func uploadFileCtx(ctx context.Context, c *sftp.Client, localPath, remotePath string, mode os.FileMode, rep *remoteProgress) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	in, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	out, err := c.OpenFile(remotePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC)
	if err != nil {
		out, err = c.Create(remotePath)
		if err != nil {
			return err
		}
	}
	defer func() { _ = out.Close() }()

	var known int64
	if st, err := in.Stat(); err == nil {
		known = st.Size()
	}
	destSize := func() int64 {
		st, err := out.Stat()
		if err != nil {
			return 0
		}
		return st.Size()
	}
	if err := copyUnwrapped(ctx, out, in, localPath, known, destSize, rep); err != nil {
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	_ = c.Chmod(remotePath, mode.Perm())
	return nil
}

func uniqueRemotePath(c *sftp.Client, p string) string {
	if !remoteExists(c, p) {
		return p
	}
	dir := path.Dir(p)
	base := path.Base(p)
	ext := path.Ext(base)
	name := strings.TrimSuffix(base, ext)
	for i := 1; ; i++ {
		candidate := path.Join(dir, fmt.Sprintf("%s (%d)%s", name, i, ext))
		if !remoteExists(c, candidate) {
			return candidate
		}
	}
}

func remoteExists(c *sftp.Client, p string) bool {
	_, err := c.Lstat(p)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "not exist") || strings.Contains(msg, "no such file") {
		return false
	}
	return true
}
