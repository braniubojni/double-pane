package remote

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/erikharutyunyan/double-pane/internal/filesystem"
	mega "github.com/t3rm1n4l/go-mega"
)

const megaDownloadAttempts = 3

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

// megaRetrySleep is swapped in tests to avoid real waits.
var megaRetrySleep = func(attempt int) {
	// 200ms, 400ms — attempt is 1-based index of the failed try.
	time.Sleep(time.Duration(attempt) * 200 * time.Millisecond)
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
		return megaDownloadWithRetry(ctx, sess, n, dest, vpath)
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
		return megaDownloadWithRetry(ctx, sess, n, dest, vpath)
	}, fn)
}

func withTempFile(ctx context.Context, cacheDir string, download func(dest string) error, then func(dest string) error) error {
	if cacheDir == "" {
		return fmt.Errorf("dup cache dir required")
	}
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

func megaDownloadWithRetry(ctx context.Context, sess *megaSession, n *mega.Node, dest, vpath string) error {
	return downloadRetry(ctx, vpath, dest, func() error {
		return megaDownloadFileCtx(ctx, sess.client, n, dest)
	}, sess.closeIdleConns)
}

// downloadRetry retries transient MEGA download failures up to megaDownloadAttempts.
// After exhausted retries it returns a non-fatal skip error (caller must not hash).
// Three consecutive session-dead errors become a fatal "not connected" error.
func downloadRetry(ctx context.Context, vpath, dest string, attempt func() error, resetIdle func()) error {
	var (
		last        error
		sessionDead int
	)
	for i := 1; i <= megaDownloadAttempts; i++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		last = attempt()
		if last == nil {
			return nil
		}
		last = wrapMegaDownloadErr(last)

		if isMegaSessionDead(last) {
			sessionDead++
			if sessionDead >= megaDownloadAttempts {
				megaLog(ctx, "fatal path=%s reason=%s", vpath, last.Error())
				return fmt.Errorf("not connected: mega session dead: %w", last)
			}
		} else if !isMegaRetryable(last) {
			megaLog(ctx, "skip path=%s reason=%s", vpath, last.Error())
			return last
		} else {
			sessionDead = 0
		}

		_ = os.Remove(dest)
		if f, err := os.Create(dest); err == nil {
			_ = f.Close()
		}
		if resetIdle != nil {
			resetIdle()
		}
		if i < megaDownloadAttempts {
			megaRetrySleep(i)
		}
	}
	megaLog(ctx, "skip path=%s reason=%s", vpath, last.Error())
	// Do not wrap last with %w — substrings like "connection reset" are fatal in the scan loop.
	return fmt.Errorf("mega download failed after %d attempts", megaDownloadAttempts)
}

func wrapMegaDownloadErr(err error) error {
	if err == nil {
		return nil
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "mega transient download") {
		return err
	}
	for _, n := range []string{
		"unsolicited response",
		"idle http",
		"unexpected eof",
		"timeout",
		"i/o timeout",
	} {
		if strings.Contains(msg, n) {
			return fmt.Errorf("mega transient download: %w", err)
		}
	}
	return err
}

func isMegaRetryable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, mega.EAGAIN) || errors.Is(err, mega.ETEMPUNAVAIL) || errors.Is(err, mega.EEXPIRED) {
		return true
	}
	msg := strings.ToLower(err.Error())
	for _, n := range []string{
		"unsolicited response",
		"idle http",
		"unexpected eof",
		"timeout",
		"i/o timeout",
		"connection reset",
		"broken pipe",
		"http2:",
		"stream error",
		"use of closed network connection",
		"server closed idle connection",
		"mega transient download",
	} {
		if strings.Contains(msg, n) {
			return true
		}
	}
	return false
}

func isMegaSessionDead(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, mega.ESID) || errors.Is(err, mega.EBLOCKED) || errors.Is(err, mega.EMFAREQUIRED) {
		return true
	}
	msg := strings.ToLower(err.Error())
	for _, n := range []string{
		"not connected",
		"authentication required",
		"invalid or expired user session",
		"please relogin",
		"user blocked",
		"mega session dead",
	} {
		if strings.Contains(msg, n) {
			return true
		}
	}
	return false
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
