package remote

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/erikharutyunyan/go-file-manager/internal/filesystem"
	mega "github.com/t3rm1n4l/go-mega"
)

type dupJobKey struct{}

// WithDupJobID stamps job-scoped [dup] logs onto ctx.
func WithDupJobID(ctx context.Context, jobID string) context.Context {
	return context.WithValue(ctx, dupJobKey{}, jobID)
}

func dupJobID(ctx context.Context) string {
	s, _ := ctx.Value(dupJobKey{}).(string)
	if s == "" {
		return "-"
	}
	return s
}

func megaLog(ctx context.Context, msg string, args ...any) {
	log.Printf("[dup] job=%s "+msg, append([]any{dupJobID(ctx)}, args...)...)
}

// HashFile downloads vpath into cacheDir, streams SHA-256 from disk, then
// deletes the temp. It never uses ReadTextFile or os.ReadFile on the blob.
func (m *MEGAManager) HashFile(ctx context.Context, vpath, cacheDir string) (string, error) {
	sess, n, loc, err := m.resolve(vpath)
	if err != nil {
		return "", err
	}
	if megaIsDir(n) {
		return "", fmt.Errorf("not a file: %s", loc.RemotePath)
	}
	return hashViaTemp(ctx, cacheDir, func(dest string) error {
		return megaDownloadFileCtx(ctx, sess.client, n, dest)
	})
}

// OpenRead is on remoteBackend for SSH/SMB. MEGA hashes via HashFile instead.
func (m *MEGAManager) OpenRead(string) (io.ReadCloser, error) {
	return nil, fmt.Errorf("mega does not stream; use HashFile")
}

func hashViaTemp(ctx context.Context, cacheDir string, download func(dest string) error) (string, error) {
	var sum string
	err := withTempFile(ctx, cacheDir, download, func(dest string) error {
		var e error
		sum, e = filesystem.HashFile(ctx, dest)
		return e
	})
	return sum, err
}

// WithTempFile downloads vpath into cacheDir, runs fn on the local copy, then deletes it.
func (m *MEGAManager) WithTempFile(ctx context.Context, vpath, cacheDir string, fn func(localPath string) error) error {
	sess, n, loc, err := m.resolve(vpath)
	if err != nil {
		return err
	}
	if megaIsDir(n) {
		return fmt.Errorf("not a file: %s", loc.RemotePath)
	}
	return withTempFile(ctx, cacheDir, func(dest string) error {
		return megaDownloadFileCtx(ctx, sess.client, n, dest)
	}, fn)
}

func withTempFile(ctx context.Context, cacheDir string, download func(dest string) error, then func(dest string) error) error {
	if err := os.MkdirAll(cacheDir, 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(cacheDir, "dup-*")
	if err != nil {
		return err
	}
	dest := f.Name()
	if err := f.Close(); err != nil {
		_ = os.Remove(dest)
		return err
	}
	megaLog(ctx, "mega temp create path=%s", dest)
	defer func() {
		_ = os.Remove(dest)
		megaLog(ctx, "mega temp cleanup path=%s", dest)
	}()
	if err := ctx.Err(); err != nil {
		return err
	}
	errCh := make(chan error, 1)
	go func() { errCh <- download(dest) }()
	select {
	case err := <-errCh:
		if err != nil {
			return err
		}
	case <-ctx.Done():
		return ctx.Err()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if then == nil {
		return nil
	}
	return then(dest)
}

func megaDownloadFileCtx(ctx context.Context, client *mega.Mega, n *mega.Node, dest string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	d, err := client.NewDownload(n)
	if err != nil {
		return err
	}
	out, err := os.OpenFile(dest, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()
	for id := 0; id < d.Chunks(); id++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		chunk, err := d.DownloadChunk(id)
		if err != nil {
			return err
		}
		pos, _, err := d.ChunkLocation(id)
		if err != nil {
			return err
		}
		if _, err := out.WriteAt(chunk, pos); err != nil {
			return err
		}
	}
	if err := out.Close(); err != nil {
		return err
	}
	return d.Finish()
}
