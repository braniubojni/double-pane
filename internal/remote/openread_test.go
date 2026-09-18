package remote

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/erikharutyunyan/go-file-manager/internal/filesystem"
)

func TestOpenReadNotConnected(t *testing.T) {
	t.Parallel()
	m := NewManager(nil)
	if _, err := m.OpenRead("ssh://u@h:22/a.bin"); err == nil {
		t.Fatal("expected not connected")
	} else if !strings.Contains(err.Error(), "not connected") {
		t.Fatalf("got %v", err)
	}
	smb := NewSMBManager()
	if _, err := smb.OpenRead("smb://u@h:445/Share/a.bin"); err == nil {
		t.Fatal("expected not connected")
	} else if !strings.Contains(err.Error(), "not connected") {
		t.Fatalf("got %v", err)
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

func TestOpenReadHashDoesNotBufferWholeFile(t *testing.T) {
	// OpenRead returns io.ReadCloser (sftp.File / smb2.File). Hashing must
	// stream with io.Copy, never ReadAll.
	const size = 5 << 20
	r := &chunkCounter{remain: size}
	sum, err := filesystem.HashReader(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	if sum == "" {
		t.Fatal("empty hash")
	}
	if r.max > 64<<10 {
		t.Fatalf("read chunk %d; hashing buffered the file", r.max)
	}
}
