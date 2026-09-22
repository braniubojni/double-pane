package remote

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/erikharutyunyan/double-pane/internal/filesystem"
	mega "github.com/t3rm1n4l/go-mega"
)

func (m *MEGAManager) Download(sources []string, localDestDir string) error {
	return m.DownloadCtx(context.Background(), sources, localDestDir, nil)
}

func (m *MEGAManager) DownloadCtx(ctx context.Context, sources []string, localDestDir string, onProgress filesystem.ProgressFunc) error {
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
	for _, src := range sources {
		if err := ctx.Err(); err != nil {
			return err
		}
		sess, n, loc, err := m.resolve(src)
		if err != nil {
			return err
		}
		base := filepath.Base(loc.RemotePath)
		if base == "" || base == "." || base == "/" {
			base = n.GetName()
		}
		if base == "" {
			return fmt.Errorf("invalid remote source: %s", src)
		}
		target := filesystem.UniquePath(filepath.Join(destAbs, base))
		if err := megaDownloadNode(ctx, sess, n, target); err != nil {
			return err
		}
	}
	if onProgress != nil {
		onProgress(filesystem.ProgressEvent{})
	}
	return nil
}

func megaDownloadNode(ctx context.Context, sess *megaSession, n *mega.Node, dest string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if n.GetType() == mega.FOLDER {
		if err := os.MkdirAll(dest, 0o755); err != nil {
			return err
		}
		kids, err := sess.client.FS.GetChildren(n)
		if err != nil {
			return err
		}
		for _, k := range kids {
			if k.GetType() != mega.FILE && k.GetType() != mega.FOLDER {
				continue
			}
			child := filepath.Join(dest, k.GetName())
			if err := megaDownloadNode(ctx, sess, k, child); err != nil {
				return err
			}
		}
		return nil
	}
	return sess.client.DownloadFile(n, dest, nil)
}

func (m *MEGAManager) Upload(localSources []string, remoteDestDir string) error {
	return m.UploadCtx(context.Background(), localSources, remoteDestDir, nil)
}

func (m *MEGAManager) UploadCtx(ctx context.Context, localSources []string, remoteDestDir string, onProgress filesystem.ProgressFunc) error {
	if m == nil {
		return fmt.Errorf("remote not available")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	sess, parent, _, err := m.resolve(remoteDestDir)
	if err != nil {
		return err
	}
	if !megaIsDir(parent) {
		return fmt.Errorf("destination is not a directory: %s", remoteDestDir)
	}
	for _, src := range localSources {
		if err := ctx.Err(); err != nil {
			return err
		}
		srcAbs, err := filesystem.Resolve(src)
		if err != nil {
			return err
		}
		if err := megaUploadPath(ctx, sess, parent, srcAbs); err != nil {
			return err
		}
	}
	if onProgress != nil {
		onProgress(filesystem.ProgressEvent{})
	}
	return nil
}

func megaUploadPath(ctx context.Context, sess *megaSession, parent *mega.Node, srcAbs string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	info, err := os.Lstat(srcAbs)
	if err != nil {
		return err
	}
	kids, err := sess.client.FS.GetChildren(parent)
	if err != nil {
		return err
	}
	name := uniqueMegaName(kids, filepath.Base(srcAbs))
	if info.IsDir() {
		dir, err := sess.client.CreateDir(name, parent)
		if err != nil {
			return err
		}
		entries, err := os.ReadDir(srcAbs)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if err := megaUploadPath(ctx, sess, dir, filepath.Join(srcAbs, e.Name())); err != nil {
				return err
			}
		}
		return nil
	}
	_, err = sess.client.UploadFile(srcAbs, parent, name, nil)
	return err
}

func (m *MEGAManager) CopyWithin(sources []string, destDir string) error {
	return m.CopyWithinCtx(context.Background(), sources, destDir, nil)
}

// CopyWithinCtx copies via a temp dir — go-mega has no server-side copy.
func (m *MEGAManager) CopyWithinCtx(ctx context.Context, sources []string, destDir string, onProgress filesystem.ProgressFunc) error {
	if ctx == nil {
		ctx = context.Background()
	}
	tmp, err := os.MkdirTemp("", "gfm-mega-copy-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	if err := m.DownloadCtx(ctx, sources, tmp, nil); err != nil {
		return err
	}
	entries, err := os.ReadDir(tmp)
	if err != nil {
		return err
	}
	locals := make([]string, 0, len(entries))
	for _, e := range entries {
		locals = append(locals, filepath.Join(tmp, e.Name()))
	}
	return m.UploadCtx(ctx, locals, destDir, onProgress)
}

func (m *MEGAManager) MoveWithin(sources []string, destDir string) error {
	return m.MoveWithinCtx(context.Background(), sources, destDir, nil)
}

func (m *MEGAManager) MoveWithinCtx(ctx context.Context, sources []string, destDir string, onProgress filesystem.ProgressFunc) error {
	if ctx == nil {
		ctx = context.Background()
	}
	sess, destNode, dloc, err := m.resolve(destDir)
	if err != nil {
		return err
	}
	if !megaIsDir(destNode) {
		return fmt.Errorf("destination is not a directory: %s", destDir)
	}
	for _, src := range sources {
		if err := ctx.Err(); err != nil {
			return err
		}
		_, n, sloc, err := m.resolve(src)
		if err != nil {
			return err
		}
		if sloc.SessionKey() != dloc.SessionKey() {
			return fmt.Errorf("cross-account move not supported")
		}
		if strings.Trim(sloc.RemotePath, "/") == "" {
			return fmt.Errorf("cannot move cloud drive")
		}
		if err := sess.client.Move(n, destNode); err != nil {
			return err
		}
	}
	if onProgress != nil {
		onProgress(filesystem.ProgressEvent{})
	}
	return nil
}
