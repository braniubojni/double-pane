package remote

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/erikharutyunyan/double-pane/internal/filesystem"
)

func TestUniqueRemotePathNaming(t *testing.T) {
	t.Parallel()
	// uniqueRemotePath depends on Lstat; exercise UniquePath local twin for naming contract.
	dir := t.TempDir()
	base := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(base, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	u1 := filesystem.UniquePath(base)
	if u1 == base {
		t.Fatalf("expected unique name when path exists, got %q", u1)
	}
	if filepath.Base(u1) != "note (1).txt" {
		t.Fatalf("got %q want note (1).txt", filepath.Base(u1))
	}
	if err := os.WriteFile(u1, []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	u2 := filesystem.UniquePath(base)
	if filepath.Base(u2) != "note (2).txt" {
		t.Fatalf("got %q want note (2).txt", filepath.Base(u2))
	}
}

func TestDownloadUploadRequireManager(t *testing.T) {
	t.Parallel()
	var m *Manager
	if err := m.Download([]string{"ssh://u@h:22/a"}, t.TempDir()); err == nil {
		t.Fatal("expected nil manager error")
	}
	if err := m.Upload([]string{t.TempDir()}, "ssh://u@h:22/"); err == nil {
		t.Fatal("expected nil manager error")
	}
}

func TestDownloadNotConnected(t *testing.T) {
	t.Parallel()
	m := NewManager(nil)
	dest := t.TempDir()
	err := m.Download([]string{"ssh://user@host:22/home/user/file.txt"}, dest)
	if err == nil {
		t.Fatal("expected not connected error")
	}
}

func TestUploadNotConnected(t *testing.T) {
	t.Parallel()
	m := NewManager(nil)
	src := filepath.Join(t.TempDir(), "f.txt")
	if err := os.WriteFile(src, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := m.Upload([]string{src}, "ssh://user@host:22/tmp")
	if err == nil {
		t.Fatal("expected not connected error")
	}
}
