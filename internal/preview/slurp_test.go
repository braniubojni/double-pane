package preview

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestOverCapDoesNotSlurp(t *testing.T) {
	dir := t.TempDir()
	png := filepath.Join(dir, "big.png")
	txt := filepath.Join(dir, "big.txt")
	if err := writeSparse(png, binaryCap+1); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(txt, bytesRepeat('a', int(textCap)+1), 0o644); err != nil {
		t.Fatal(err)
	}

	var reads int64
	orig := openPreviewFile
	openPreviewFile = func(path string) (previewSource, os.FileInfo, error) {
		src, info, err := orig(path)
		if err != nil {
			return nil, nil, err
		}
		return countingReader{ReadCloser: src, n: &reads}, info, nil
	}
	t.Cleanup(func() { openPreviewFile = orig })

	got, err := Read(png)
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != "none" || len(got.Data) != 0 || got.Text != "" {
		t.Fatalf("png over cap: %+v", got)
	}
	if reads != 0 {
		t.Fatalf("png over cap read %d bytes", reads)
	}

	reads = 0
	got, err = Read(txt)
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != "none" || got.Message != msgTooLarge || got.Text != "" || len(got.Data) != 0 {
		t.Fatalf("txt over cap: %+v", got)
	}
	if reads == 0 || reads > sniffSize {
		t.Fatalf("txt over cap sniffed %d bytes, want 1..%d", reads, sniffSize)
	}
}

type countingReader struct {
	io.ReadCloser
	n *int64
}

func (c countingReader) Read(p []byte) (int, error) {
	n, err := c.ReadCloser.Read(p)
	*c.n += int64(n)
	return n, err
}

func writeSparse(path string, size int64) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := f.Truncate(size); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

func bytesRepeat(b byte, n int) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = b
	}
	return out
}
