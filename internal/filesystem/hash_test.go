package filesystem

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestHashFileIdenticalAndDifferent(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.bin")
	b := filepath.Join(dir, "b.bin")
	c := filepath.Join(dir, "c.bin")
	same := bytes.Repeat([]byte("dup"), 4096)
	if err := os.WriteFile(a, same, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, same, 0o644); err != nil {
		t.Fatal(err)
	}
	diff := append([]byte{}, same...)
	diff[0] ^= 0xff
	if err := os.WriteFile(c, diff, 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	ha, err := HashFile(ctx, a)
	if err != nil {
		t.Fatal(err)
	}
	hb, err := HashFile(ctx, b)
	if err != nil {
		t.Fatal(err)
	}
	hc, err := HashFile(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	if ha != hb {
		t.Fatalf("identical files hashed differently: %s vs %s", ha, hb)
	}
	want := sha256.Sum256(same)
	if ha != hex.EncodeToString(want[:]) {
		t.Fatalf("hash mismatch")
	}
	if hc == ha {
		t.Fatal("same-size different bytes must not share a hash")
	}
}

type chunkCounter struct {
	remain int
	max    int
}

func (c *chunkCounter) Read(p []byte) (int, error) {
	if len(p) > c.max {
		c.max = len(p)
	}
	if c.remain <= 0 {
		return 0, io.EOF
	}
	n := len(p)
	if n > c.remain {
		n = c.remain
	}
	for i := 0; i < n; i++ {
		p[i] = 'x'
	}
	c.remain -= n
	return n, nil
}

func (c *chunkCounter) Close() error { return nil }

func TestHashReaderDoesNotBufferWholeFile(t *testing.T) {
	const size = 8 << 20
	r := &chunkCounter{remain: size}
	sum, err := HashReader(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	if sum == "" {
		t.Fatal("empty hash")
	}
	if r.max > 64<<10 {
		t.Fatalf("read chunk %d looks like ReadAll of the whole file", r.max)
	}
}

func TestHashReaderCancel(t *testing.T) {
	pr, pw := io.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		_, err := HashReader(ctx, pr)
		errCh <- err
	}()
	time.Sleep(20 * time.Millisecond)
	cancel()
	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected cancel error")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("hash did not unblock on cancel")
	}
	_ = pw.Close()
}
