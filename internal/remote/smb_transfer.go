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

	"github.com/cloudsoda/go-smb2"
	"github.com/erikharutyunyan/double-pane/internal/filesystem"
)

type smbFileJob struct {
	fs       *smb2.Share
	src, dst string
	kind     smbXferKind
}

type smbXferKind int

const (
	smbXferDown smbXferKind = iota
	smbXferUp
	smbXferCopy
)

// Download copies SMB virtual paths into a local directory.
func (m *SMBManager) Download(sources []string, localDestDir string) error {
	return m.DownloadCtx(context.Background(), sources, localDestDir, nil)
}

// DownloadCtx copies SMB virtual paths into a local directory with progress.
func (m *SMBManager) DownloadCtx(ctx context.Context, sources []string, localDestDir string, onProgress filesystem.ProgressFunc) (err error) {
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
	files, total, err := m.smbSourcesStats(sources)
	if err != nil {
		return err
	}
	rep := newRemoteProgress(total, onProgress)
	if onProgress != nil {
		onProgress(filesystem.ProgressEvent{Total: total})
	}
	workers := filesystem.CopyWorkersWith(filesystem.IONetwork, []string{destAbs}, files)
	pool := newXferPool(ctx, workers, func(ctx context.Context, j smbFileJob) error {
		return smbCopyFile(ctx, j, rep)
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
		if loc.ShareName() == "" {
			placeErr = fmt.Errorf("invalid remote source: %s", src)
			break
		}
		fs, err := m.shareFS(loc)
		if err != nil {
			placeErr = err
			break
		}
		rel := smbRel(loc)
		base := path.Base(rel)
		if rel == "." {
			base = loc.ShareName()
		}
		if base == "" || base == "." || base == "/" {
			placeErr = fmt.Errorf("invalid remote source: %s", src)
			break
		}
		target := filesystem.UniquePath(filepath.Join(destAbs, base))
		created = append(created, target)
		st, statErr := fs.Stat(rel)
		rep.setDest(target, statErr == nil && st.IsDir())
		if err := smbDownloadPlace(ctx, fs, rel, target, pool); err != nil {
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

// Upload copies local paths into an SMB virtual directory.
func (m *SMBManager) Upload(localSources []string, remoteDestDir string) error {
	return m.UploadCtx(context.Background(), localSources, remoteDestDir, nil)
}

// UploadCtx copies local paths into an SMB virtual directory with progress.
func (m *SMBManager) UploadCtx(ctx context.Context, localSources []string, remoteDestDir string, onProgress filesystem.ProgressFunc) (err error) {
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
	if dloc.ShareName() == "" {
		return fmt.Errorf("destination is not a directory: %s", remoteDestDir)
	}
	fs, err := m.shareFS(dloc)
	if err != nil {
		return err
	}
	rel := smbRel(dloc)
	st, err := fs.Stat(rel)
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
	pool := newXferPool(ctx, workers, func(ctx context.Context, j smbFileJob) error {
		return smbCopyFile(ctx, j, rep)
	})
	var created []string
	defer func() {
		if errors.Is(err, context.Canceled) {
			for _, p := range created {
				_ = fs.RemoveAll(p)
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
		dest := uniqueSMBPath(fs, path.Join(rel, base))
		created = append(created, dest)
		srcInfo, statErr := os.Lstat(srcAbs)
		rep.setDest(dloc.JoinPath(dest), statErr == nil && srcInfo.IsDir())
		if err := smbUploadPlace(ctx, fs, srcAbs, dest, pool); err != nil {
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

// CopyWithin copies sources into destDir on the same SMB host.
func (m *SMBManager) CopyWithin(sources []string, destDir string) error {
	return m.CopyWithinCtx(context.Background(), sources, destDir, nil)
}

// CopyWithinCtx copies sources into destDir on the same SMB host with progress.
func (m *SMBManager) CopyWithinCtx(ctx context.Context, sources []string, destDir string, onProgress filesystem.ProgressFunc) (err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	dloc, err := ParseLocation(destDir)
	if err != nil {
		return err
	}
	if dloc.ShareName() == "" {
		return fmt.Errorf("destination is not a directory: %s", destDir)
	}
	dfs, err := m.shareFS(dloc)
	if err != nil {
		return err
	}
	files, total, err := m.smbSourcesStats(sources)
	if err != nil {
		return err
	}
	rep := newRemoteProgress(total, onProgress)
	if onProgress != nil {
		onProgress(filesystem.ProgressEvent{Total: total})
	}
	workers := filesystem.WorkerCount(filesystem.IONetwork, files)
	pool := newXferPool(ctx, workers, func(ctx context.Context, j smbFileJob) error {
		return smbCopyFile(ctx, j, rep)
	})
	var created []string
	defer func() {
		if errors.Is(err, context.Canceled) {
			for _, p := range created {
				_ = dfs.RemoveAll(p)
			}
		}
	}()
	var placeErr error
	for _, src := range sources {
		if err := ctx.Err(); err != nil {
			placeErr = err
			break
		}
		sloc, err := ParseLocation(src)
		if err != nil {
			placeErr = err
			break
		}
		if sloc.SessionKey() != dloc.SessionKey() {
			placeErr = fmt.Errorf("cross-host copy not supported")
			break
		}
		if sloc.ShareName() != dloc.ShareName() {
			placeErr = fmt.Errorf("cross-share copy not supported")
			break
		}
		sfs, err := m.shareFS(sloc)
		if err != nil {
			placeErr = err
			break
		}
		base := path.Base(smbRel(sloc))
		dest := path.Join(smbRel(dloc), base)
		created = append(created, dest)
		st, statErr := sfs.Stat(smbRel(sloc))
		rep.setDest(dloc.JoinPath(dest), statErr == nil && st.IsDir())
		if err := smbCopyPlace(ctx, sfs, smbRel(sloc), dest, pool); err != nil {
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

// MoveWithin moves sources into destDir on the same SMB host.
func (m *SMBManager) MoveWithin(sources []string, destDir string) error {
	return m.MoveWithinCtx(context.Background(), sources, destDir, nil)
}

// MoveWithinCtx moves sources into destDir on the same SMB host with progress.
func (m *SMBManager) MoveWithinCtx(ctx context.Context, sources []string, destDir string, onProgress filesystem.ProgressFunc) error {
	if ctx == nil {
		ctx = context.Background()
	}
	dloc, err := ParseLocation(destDir)
	if err != nil {
		return err
	}
	if dloc.ShareName() == "" {
		return fmt.Errorf("destination is not a directory: %s", destDir)
	}
	for _, src := range sources {
		if err := ctx.Err(); err != nil {
			return err
		}
		sloc, err := ParseLocation(src)
		if err != nil {
			return err
		}
		if sloc.SessionKey() != dloc.SessionKey() {
			return fmt.Errorf("cross-host move not supported")
		}
		if sloc.ShareName() != dloc.ShareName() {
			return fmt.Errorf("cross-share move not supported")
		}
		sfs, err := m.shareFS(sloc)
		if err != nil {
			return err
		}
		base := path.Base(smbRel(sloc))
		dest := path.Join(smbRel(dloc), base)
		if err := sfs.Rename(smbRel(sloc), dest); err != nil {
			if err2 := smbCopyRemoteCtx(ctx, sfs, smbRel(sloc), dest, onProgress); err2 != nil {
				return err2
			}
			if err2 := sfs.RemoveAll(smbRel(sloc)); err2 != nil {
				return err2
			}
		}
	}
	return nil
}

func smbCopyRemoteCtx(ctx context.Context, fs *smb2.Share, src, dst string, onProgress filesystem.ProgressFunc) error {
	files, total, err := smbPathStats(fs, src)
	if err != nil {
		return err
	}
	rep := newRemoteProgress(total, onProgress)
	workers := filesystem.WorkerCount(filesystem.IONetwork, files)
	pool := newXferPool(ctx, workers, func(ctx context.Context, j smbFileJob) error {
		return smbCopyFile(ctx, j, rep)
	})
	if err := smbCopyPlace(ctx, fs, src, dst, pool); err != nil {
		pool.cancel()
		_ = pool.finish()
		return err
	}
	if err := pool.finish(); err != nil {
		return err
	}
	rep.finish("")
	return nil
}

func (m *SMBManager) smbSourcesStats(sources []string) (files int, total int64, err error) {
	for _, src := range sources {
		loc, err := ParseLocation(src)
		if err != nil {
			return 0, 0, err
		}
		if loc.ShareName() == "" {
			return 0, 0, fmt.Errorf("invalid remote source: %s", src)
		}
		fs, err := m.shareFS(loc)
		if err != nil {
			return 0, 0, err
		}
		nfiles, nbytes, err := smbPathStats(fs, smbRel(loc))
		if err != nil {
			return 0, 0, err
		}
		files += nfiles
		total += nbytes
	}
	return files, total, nil
}

func smbPathStats(fs *smb2.Share, remotePath string) (files int, total int64, err error) {
	st, err := fs.Stat(remotePath)
	if err != nil {
		return 0, 0, err
	}
	if !st.IsDir() {
		return 1, st.Size(), nil
	}
	var walk func(string) error
	walk = func(p string) error {
		entries, err := fs.ReadDir(p)
		if err != nil {
			return err
		}
		for _, e := range entries {
			child := path.Join(p, e.Name())
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

func smbDownloadPlace(ctx context.Context, fs *smb2.Share, remotePath, localPath string, pool *xferPool[smbFileJob]) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	st, err := fs.Stat(remotePath)
	if err != nil {
		return err
	}
	if !st.IsDir() {
		return pool.enqueue(smbFileJob{fs: fs, src: remotePath, dst: localPath, kind: smbXferDown})
	}
	if err := os.MkdirAll(localPath, 0o755); err != nil {
		return err
	}
	entries, err := fs.ReadDir(remotePath)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := smbDownloadPlace(ctx, fs, path.Join(remotePath, e.Name()), filepath.Join(localPath, e.Name()), pool); err != nil {
			return err
		}
	}
	return nil
}

func smbUploadPlace(ctx context.Context, fs *smb2.Share, localPath, remotePath string, pool *xferPool[smbFileJob]) error {
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
		return pool.enqueue(smbFileJob{fs: fs, src: localPath, dst: remotePath, kind: smbXferUp})
	}
	if err := fs.MkdirAll(remotePath, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(localPath)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := smbUploadPlace(ctx, fs, filepath.Join(localPath, e.Name()), path.Join(remotePath, e.Name()), pool); err != nil {
			return err
		}
	}
	return nil
}

func smbCopyPlace(ctx context.Context, fs *smb2.Share, src, dst string, pool *xferPool[smbFileJob]) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	st, err := fs.Stat(src)
	if err != nil {
		return err
	}
	if !st.IsDir() {
		return pool.enqueue(smbFileJob{fs: fs, src: src, dst: dst, kind: smbXferCopy})
	}
	if err := fs.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	entries, err := fs.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := smbCopyPlace(ctx, fs, path.Join(src, e.Name()), path.Join(dst, e.Name()), pool); err != nil {
			return err
		}
	}
	return nil
}

func smbCopyFile(ctx context.Context, j smbFileJob, rep *remoteProgress) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	switch j.kind {
	case smbXferDown:
		in, err := j.fs.Open(j.src)
		if err != nil {
			return err
		}
		defer func() { _ = in.Close() }()
		out, err := os.OpenFile(j.dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			return err
		}
		defer func() { _ = out.Close() }()
		return smbCopyUnwrapped(ctx, out, in, j.src, destSizeOS(out), rep)
	case smbXferUp:
		in, err := os.Open(j.src)
		if err != nil {
			return err
		}
		defer func() { _ = in.Close() }()
		out, err := j.fs.Create(j.dst)
		if err != nil {
			return err
		}
		defer func() { _ = out.Close() }()
		return smbCopyUnwrapped(ctx, out, in, j.src, destSizeSMB(out), rep)
	default:
		in, err := j.fs.Open(j.src)
		if err != nil {
			return err
		}
		defer func() { _ = in.Close() }()
		out, err := j.fs.Create(j.dst)
		if err != nil {
			return err
		}
		defer func() { _ = out.Close() }()
		return smbCopyUnwrapped(ctx, out, in, j.src, destSizeSMB(out), rep)
	}
}

func destSizeOS(f *os.File) func() int64 {
	return func() int64 {
		st, err := f.Stat()
		if err != nil {
			return 0
		}
		return st.Size()
	}
}

func destSizeSMB(f *smb2.File) func() int64 {
	return func() int64 {
		st, err := f.Stat()
		if err != nil {
			return 0
		}
		return st.Size()
	}
}

func smbCopyUnwrapped(ctx context.Context, dst io.Writer, src io.Reader, path string, destSize func() int64, rep *remoteProgress) error {
	var known int64
	if s, ok := src.(interface{ Stat() (os.FileInfo, error) }); ok {
		if st, err := s.Stat(); err == nil {
			known = st.Size()
		}
	}
	if err := copyUnwrapped(ctx, dst, src, path, known, destSize, rep); err != nil {
		return err
	}
	if c, ok := dst.(io.Closer); ok {
		return c.Close()
	}
	return nil
}

func uniqueSMBPath(fs *smb2.Share, p string) string {
	if !smbPathExists(fs, p) {
		return p
	}
	dir := path.Dir(p)
	base := path.Base(p)
	ext := path.Ext(base)
	name := strings.TrimSuffix(base, ext)
	for i := 1; ; i++ {
		candidate := path.Join(dir, fmt.Sprintf("%s (%d)%s", name, i, ext))
		if !smbPathExists(fs, candidate) {
			return candidate
		}
	}
}

func smbPathExists(fs *smb2.Share, p string) bool {
	_, err := fs.Lstat(p)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) || smbNotExist(err) {
		return false
	}
	return true
}
