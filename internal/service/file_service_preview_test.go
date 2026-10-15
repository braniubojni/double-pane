package service

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/erikharutyunyan/double-pane/internal/vault"
)

func TestReadPreview(t *testing.T) {
	dir := t.TempDir()
	png := filepath.Join(dir, "dot.png")
	if err := os.WriteFile(png, []byte{0x89, 'P', 'N', 'G'}, 0o644); err != nil {
		t.Fatal(err)
	}
	txt := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(txt, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	nul := filepath.Join(dir, "bin.dat")
	if err := os.WriteFile(nul, []byte{'a', 0, 'b'}, 0o644); err != nil {
		t.Fatal(err)
	}
	pdf := filepath.Join(dir, "doc.pdf")
	if err := os.WriteFile(pdf, []byte("%PDF-1.4\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	heic := filepath.Join(dir, "photo.heic")
	if err := os.WriteFile(heic, []byte("not-heic"), 0o644); err != nil {
		t.Fatal(err)
	}
	dpenc := filepath.Join(dir, "secret.txt.dpenc")
	if err := os.WriteFile(dpenc, []byte("ciphertext"), 0o644); err != nil {
		t.Fatal(err)
	}
	zipPath := filepath.Join(dir, "bundle.zip")
	if err := writePreviewZip(zipPath); err != nil {
		t.Fatal(err)
	}

	vaultRoot := filepath.Join(dir, "locked")
	if err := os.Mkdir(vaultRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := vault.CreateFolderVault(context.Background(), vaultRoot, "vault-pass", "", nil); err != nil {
		t.Fatal(err)
	}
	inside := filepath.Join(vaultRoot, "note.txt")
	if err := os.WriteFile(inside, []byte("plaintext-should-not-leak"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := &FileService{}
	tests := []struct {
		name    string
		path    string
		kind    string
		mime    string
		text    string
		hasData bool
		message string
	}{
		{name: "png", path: png, kind: "image", mime: "image/png", hasData: true},
		{name: "txt", path: txt, kind: "text", mime: "text/plain", text: "hello"},
		{name: "nul", path: nul, kind: "none", message: "This file type cannot be previewed"},
		{name: "pdf", path: pdf, kind: "pdf", mime: "application/pdf", hasData: true},
		{name: "dir", path: dir, kind: "none", message: "Select a file"},
		{name: "ssh", path: "ssh://user@host:22/tmp/a.txt", kind: "none", message: "Preview is only available for local files"},
		{name: "trash", path: "trash://a.txt", kind: "none", message: "Preview is only available for local files"},
		{name: "archive", path: filepath.Join(zipPath, "inside.txt"), kind: "none", message: "Preview is only available for local files"},
		{name: "dpenc", path: dpenc, kind: "none", message: "Preview is not available for vault files"},
		{name: "folder vault", path: inside, kind: "none", message: "Preview is not available for vault files"},
		{name: "heic", path: heic, kind: "none", message: "This file type cannot be previewed"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := s.ReadPreview(tc.path)
			if err != nil {
				t.Fatal(err)
			}
			if got.Kind != tc.kind {
				t.Fatalf("kind=%q", got.Kind)
			}
			if tc.mime != "" && got.MIME != tc.mime {
				t.Fatalf("mime=%q", got.MIME)
			}
			if got.Text != tc.text {
				t.Fatalf("text=%q", got.Text)
			}
			if tc.hasData != (len(got.Data) > 0) {
				t.Fatalf("data len=%d", len(got.Data))
			}
			if !tc.hasData && len(got.Data) != 0 {
				t.Fatal("data should be empty")
			}
			if tc.message != "" && got.Message != tc.message {
				t.Fatalf("message=%q", got.Message)
			}
		})
	}
}

func writePreviewZip(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	zw := zip.NewWriter(f)
	w, err := zw.Create("inside.txt")
	if err != nil {
		return err
	}
	if _, err := w.Write([]byte("secret")); err != nil {
		return err
	}
	return zw.Close()
}
