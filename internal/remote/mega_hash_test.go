package remote

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	mega "github.com/t3rm1n4l/go-mega"
)

func TestHashViaTempCleansUpAndDoesNotReadAll(t *testing.T) {
	cache := t.TempDir()
	payload := []byte("mega-blob-contents")
	want := sha256.Sum256(payload)
	var dest string
	got, err := hashViaTemp(context.Background(), cache, func(d string) error {
		dest = d
		if strings.Contains(d, cache) == false {
			t.Fatalf("temp not under cache dir: %s", d)
		}
		return os.WriteFile(d, payload, 0o600)
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != hex.EncodeToString(want[:]) {
		t.Fatalf("hash %s", got)
	}
	if dest == "" {
		t.Fatal("download was not called")
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatalf("temp left behind: %v", err)
	}
}

func TestHashViaTempCancelRemovesTemp(t *testing.T) {
	cache := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan string, 1)
	errCh := make(chan error, 1)
	go func() {
		_, err := hashViaTemp(ctx, cache, func(dest string) error {
			if err := os.WriteFile(dest, []byte("x"), 0o600); err != nil {
				return err
			}
			started <- dest
			<-ctx.Done()
			return ctx.Err()
		})
		errCh <- err
	}()
	var dest string
	select {
	case dest = <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("download did not start")
	}
	cancel()
	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected cancel error")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("hashViaTemp did not return")
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatalf("temp left behind after cancel: %v", err)
	}
	ents, err := os.ReadDir(cache)
	if err != nil {
		t.Fatal(err)
	}
	if len(ents) != 0 {
		t.Fatalf("cache not empty: %v", ents)
	}
}

func TestHashViaTempCancelAbortsStuckDownload(t *testing.T) {
	cache := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	errCh := make(chan error, 1)
	go func() {
		_, err := hashViaTemp(ctx, cache, func(string) error {
			close(started)
			time.Sleep(30 * time.Second)
			return nil
		})
		errCh <- err
	}()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("download did not start")
	}
	cancel()
	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected cancel error")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("hashViaTemp did not abort")
	}
	ents, err := os.ReadDir(cache)
	if err != nil {
		t.Fatal(err)
	}
	if len(ents) != 0 {
		t.Fatalf("cache not empty: %v", ents)
	}
}

func TestMEGAHashFileNotConnected(t *testing.T) {
	m := NewMEGAManager()
	_, err := m.HashFile(context.Background(), "mega://alice@gmail.com/a.bin", t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "not connected") {
		t.Fatalf("got %v", err)
	}
	if _, err := m.OpenRead("mega://alice@gmail.com/a.bin"); err == nil {
		t.Fatal("OpenRead should fail on MEGA")
	}
}

func TestHashViaTempUsesStreamHashNotReadFile(t *testing.T) {
	// hashViaTemp must go through HashFile (os.Open + io.Copy), not os.ReadFile.
	cache := t.TempDir()
	const n = 2 << 20
	sum, err := hashViaTemp(context.Background(), cache, func(dest string) error {
		f, err := os.OpenFile(dest, os.O_WRONLY, 0)
		if err != nil {
			return err
		}
		defer func() { _ = f.Close() }()
		buf := make([]byte, 32<<10)
		for i := range buf {
			buf[i] = 'm'
		}
		left := n
		for left > 0 {
			w := len(buf)
			if w > left {
				w = left
			}
			if _, err := f.Write(buf[:w]); err != nil {
				return err
			}
			left -= w
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.New()
	buf := make([]byte, 32<<10)
	for i := range buf {
		buf[i] = 'm'
	}
	left := n
	for left > 0 {
		w := len(buf)
		if w > left {
			w = left
		}
		_, _ = h.Write(buf[:w])
		left -= w
	}
	if sum != hex.EncodeToString(h.Sum(nil)) {
		t.Fatal("hash mismatch")
	}
}

func TestDownloadRetrySucceedsAfterTransientFailures(t *testing.T) {
	prev := megaRetrySleep
	megaRetrySleep = func(int) {}
	defer func() { megaRetrySleep = prev }()

	cache := t.TempDir()
	dest := filepath.Join(cache, "partial")
	if err := os.WriteFile(dest, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	var n int
	err := downloadRetry(context.Background(), "mega://u@h/a.bin", dest, func() error {
		n++
		if n < 3 {
			return errors.New("unexpected EOF")
		}
		return os.WriteFile(dest, []byte("ok"), 0o600)
	}, nil)
	if err != nil {
		t.Fatalf("expected success after retries: %v", err)
	}
	if n != 3 {
		t.Fatalf("attempts=%d want 3", n)
	}
}

func TestDownloadRetrySkipsAfterThreeFailuresAndClearsTemp(t *testing.T) {
	prev := megaRetrySleep
	megaRetrySleep = func(int) {}
	defer func() { megaRetrySleep = prev }()

	cache := t.TempDir()
	var thenCalled bool
	err := withTempFile(context.Background(), cache, func(dest string) error {
		return downloadRetry(context.Background(), "mega://u@h/a.bin", dest, func() error {
			_ = os.WriteFile(dest, []byte("partial"), 0o600)
			return errors.New("Unsolicited response received on idle HTTP channel")
		}, nil)
	}, func(string) error {
		thenCalled = true
		return nil
	})
	if err == nil {
		t.Fatal("expected skip error")
	}
	if !strings.Contains(err.Error(), "mega download failed after 3 attempts") {
		t.Fatalf("got %v", err)
	}
	if thenCalled {
		t.Fatal("must not hash after failed download")
	}
	ents, err := os.ReadDir(cache)
	if err != nil {
		t.Fatal(err)
	}
	if len(ents) != 0 {
		t.Fatalf("temp left behind: %v", ents)
	}
}

func TestDownloadRetryThreeSessionDeadIsFatal(t *testing.T) {
	prev := megaRetrySleep
	megaRetrySleep = func(int) {}
	defer func() { megaRetrySleep = prev }()

	dest := filepath.Join(t.TempDir(), "x")
	_ = os.WriteFile(dest, nil, 0o600)
	err := downloadRetry(context.Background(), "mega://u@h/a.bin", dest, func() error {
		return mega.ESID
	}, nil)
	if err == nil || !strings.Contains(err.Error(), "not connected") {
		t.Fatalf("expected fatal not connected, got %v", err)
	}
}
