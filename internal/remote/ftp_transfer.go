package remote

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"

	"github.com/erikharutyunyan/double-pane/internal/filesystem"
)

// Download copies FTP paths into a local directory.
func (m *FTPManager) Download(sources []string, localDestDir string) error {
	return m.DownloadCtx(context.Background(), sources, localDestDir, nil)
}

// DownloadCtx copies FTP paths into a local directory with progress.
func (m *FTPManager) DownloadCtx(ctx context.Context, sources []string, localDestDir string, onProgress filesystem.ProgressFunc) error {
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
	if len(sources) == 0 {
		return fmt.Errorf("no sources")
	}
	loc0, s, err := m.session(sources[0])
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var total int64
	for _, src := range sources {
		loc, err := ParseLocation(src)
		if err != nil {
			return err
		}
		if loc.SessionKey() != loc0.SessionKey() {
			return fmt.Errorf("cross-host copy not supported")
		}
		n, err := s.treeBytes(ctx, cleanFTPPath(loc.RemotePath))
		if err != nil {
			return err
		}
		total += n
	}
	rep := newRemoteProgress(total, onProgress)
	if onProgress != nil {
		onProgress(filesystem.ProgressEvent{Total: total})
	}
	for _, src := range sources {
		if err := ctx.Err(); err != nil {
			return err
		}
		loc, err := ParseLocation(src)
		if err != nil {
			return err
		}
		base := path.Base(cleanFTPPath(loc.RemotePath))
		if base == "" || base == "/" || base == "." {
			return fmt.Errorf("invalid remote source: %s", src)
		}
		target := filesystem.UniquePath(filepath.Join(destAbs, base))
		st, ok, err := s.stat(cleanFTPPath(loc.RemotePath))
		if err != nil {
			return err
		}
		rep.setDest(target, ok && st.isDir)
		if err := s.downloadPath(ctx, cleanFTPPath(loc.RemotePath), target, rep); err != nil {
			return err
		}
	}
	rep.finish("")
	return nil
}

// Upload copies local paths into an FTP directory.
func (m *FTPManager) Upload(localSources []string, remoteDestDir string) error {
	return m.UploadCtx(context.Background(), localSources, remoteDestDir, nil)
}

// UploadCtx copies local paths into an FTP directory with progress.
func (m *FTPManager) UploadCtx(ctx context.Context, localSources []string, remoteDestDir string, onProgress filesystem.ProgressFunc) error {
	if m == nil {
		return fmt.Errorf("remote not available")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	dloc, s, err := m.session(remoteDestDir)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	dest := cleanFTPPath(dloc.RemotePath)
	st, ok, err := s.stat(dest)
	if err != nil {
		return err
	}
	if !ok || !st.isDir {
		return fmt.Errorf("destination is not a directory: %s", remoteDestDir)
	}
	var total int64
	for _, src := range localSources {
		n, err := localTreeBytes(src)
		if err != nil {
			return err
		}
		total += n
	}
	rep := newRemoteProgress(total, onProgress)
	if onProgress != nil {
		onProgress(filesystem.ProgressEvent{Total: total})
	}
	for _, src := range localSources {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := s.uploadPath(ctx, src, dest, rep); err != nil {
			return err
		}
	}
	rep.finish("")
	return nil
}

// CopyWithin copies paths into another directory on the same FTP host.
func (m *FTPManager) CopyWithin(sources []string, destDir string) error {
	return m.CopyWithinCtx(context.Background(), sources, destDir, nil)
}

// CopyWithinCtx copies on the same host. FTP has no server-side copy, so each file is downloaded to a temp file and stored again.
func (m *FTPManager) CopyWithinCtx(ctx context.Context, sources []string, destDir string, onProgress filesystem.ProgressFunc) error {
	return m.transferWithin(ctx, sources, destDir, onProgress, false)
}

// MoveWithin moves paths into another directory on the same FTP host.
func (m *FTPManager) MoveWithin(sources []string, destDir string) error {
	return m.MoveWithinCtx(context.Background(), sources, destDir, nil)
}

// MoveWithinCtx renames on the server when it can, and otherwise copies then deletes.
func (m *FTPManager) MoveWithinCtx(ctx context.Context, sources []string, destDir string, onProgress filesystem.ProgressFunc) error {
	return m.transferWithin(ctx, sources, destDir, onProgress, true)
}

func (m *FTPManager) transferWithin(ctx context.Context, sources []string, destDir string, onProgress filesystem.ProgressFunc, move bool) error {
	if m == nil {
		return fmt.Errorf("remote not available")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	dloc, s, err := m.session(destDir)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	dest := cleanFTPPath(dloc.RemotePath)
	var total int64
	weights := make([]int64, len(sources))
	cleaned := make([]string, len(sources))
	for i, src := range sources {
		loc, err := ParseLocation(src)
		if err != nil {
			return err
		}
		if loc.SessionKey() != dloc.SessionKey() {
			return fmt.Errorf("cross-host copy not supported")
		}
		cleaned[i] = cleanFTPPath(loc.RemotePath)
		n, err := s.treeBytes(ctx, cleaned[i])
		if err != nil {
			return err
		}
		if n == 0 {
			n = 1
		}
		weights[i] = n
		total += n
	}
	rep := newRemoteProgress(total, onProgress)
	if onProgress != nil {
		onProgress(filesystem.ProgressEvent{Total: total})
	}
	for i, src := range cleaned {
		if err := ctx.Err(); err != nil {
			return err
		}
		base := path.Base(src)
		target, err := s.uniqueChild(dest, base)
		if err != nil {
			return err
		}
		st, ok, err := s.stat(src)
		if err != nil {
			return err
		}
		rep.setDest(dloc.JoinPath(target), ok && st.isDir)
		if move {
			if err := s.renamePath(src, target); err == nil {
				rep.add(weights[i], src)
				continue
			}
		}
		if err := s.copyRemote(ctx, src, target, rep); err != nil {
			return err
		}
		if move {
			if err := s.deletePath(src); err != nil {
				return err
			}
		}
	}
	rep.finish("")
	return nil
}

func (s *ftpSession) downloadPath(ctx context.Context, remotePath, localPath string, rep *remoteProgress) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	st, ok, err := s.stat(remotePath)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("not found: %s", remotePath)
	}
	if st.isDir {
		if err := os.MkdirAll(localPath, 0o755); err != nil {
			return err
		}
		entries, err := s.list(remotePath)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if err := s.downloadPath(ctx, path.Join(remotePath, e.name), filepath.Join(localPath, e.name), rep); err != nil {
				return err
			}
		}
		return nil
	}
	f, err := os.OpenFile(localPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	err = s.withRead("RETR", remotePath, func(r io.Reader) error {
		return copyFTP(ctx, f, r, rep, remotePath)
	})
	cerr := f.Close()
	if err != nil {
		_ = os.Remove(localPath)
		return err
	}
	return cerr
}

func (s *ftpSession) uploadPath(ctx context.Context, localPath, remoteDir string, rep *remoteProgress) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	info, err := os.Stat(localPath)
	if err != nil {
		return err
	}
	base := filepath.Base(localPath)
	dest, err := s.uniqueChild(remoteDir, base)
	if err != nil {
		return err
	}
	if info.IsDir() {
		if err := s.mkdir(dest); err != nil {
			return err
		}
		children, err := os.ReadDir(localPath)
		if err != nil {
			return err
		}
		for _, child := range children {
			if err := s.uploadPath(ctx, filepath.Join(localPath, child.Name()), dest, rep); err != nil {
				return err
			}
		}
		return nil
	}
	f, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	return s.withWrite(dest, func(w io.Writer) error {
		return copyFTP(ctx, w, f, rep, localPath)
	})
}

func (s *ftpSession) copyRemote(ctx context.Context, src, dst string, rep *remoteProgress) error {
	st, ok, err := s.stat(src)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("not found: %s", src)
	}
	if st.isDir {
		if err := s.mkdir(dst); err != nil {
			return err
		}
		entries, err := s.list(src)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if err := s.copyRemote(ctx, path.Join(src, e.name), path.Join(dst, e.name), rep); err != nil {
				return err
			}
		}
		return nil
	}
	tmp, err := os.CreateTemp("", "gfm-ftp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	err = s.withRead("RETR", src, func(r io.Reader) error {
		return copyFTP(ctx, tmp, r, rep, src)
	})
	if err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		_ = tmp.Close()
		return err
	}
	werr := s.withWrite(dst, func(w io.Writer) error {
		_, err := io.Copy(w, tmp)
		return err
	})
	_ = tmp.Close()
	return werr
}

func (s *ftpSession) withRead(verb, p string, fn func(io.Reader) error) error {
	data, err := s.startRead(verb, p)
	if err != nil {
		return err
	}
	rerr := fn(data)
	ferr := s.finishData(data)
	if rerr != nil {
		return rerr
	}
	return ferr
}

func copyFTP(ctx context.Context, dst io.Writer, src io.Reader, rep *remoteProgress, current string) error {
	buf := make([]byte, 32*1024)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		n, err := src.Read(buf)
		if n > 0 {
			if _, werr := dst.Write(buf[:n]); werr != nil {
				return werr
			}
			if rep != nil {
				rep.add(int64(n), current)
			}
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

func localTreeBytes(p string) (int64, error) {
	info, err := os.Stat(p)
	if err != nil {
		return 0, err
	}
	if !info.IsDir() {
		return info.Size(), nil
	}
	var total int64
	err = filepath.Walk(p, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info != nil && !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total, err
}
