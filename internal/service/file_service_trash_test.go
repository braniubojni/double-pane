package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/erikharutyunyan/go-file-manager/internal/filesystem"
)

func TestDeleteRestoreViaXDGTrash(t *testing.T) {
	t.Parallel()
	work := t.TempDir()
	s := NewFileService(nil, nil, nil, filepath.Join(work, "trash"))
	file := filepath.Join(work, "note.txt")
	if err := os.WriteFile(file, []byte("hi"), 0o600); err != nil {
		t.Fatal(err)
	}
	token, err := s.Delete([]string{file})
	if err != nil {
		t.Fatal(err)
	}
	if token == "" {
		t.Fatal("expected undo token")
	}
	var origins []string
	if err := json.Unmarshal([]byte(token), &origins); err != nil {
		t.Fatalf("token %q: %v", token, err)
	}
	if len(origins) != 1 || origins[0] != file {
		t.Fatalf("origins=%v", origins)
	}
	if _, err := os.Lstat(file); !os.IsNotExist(err) {
		t.Fatal("file still present")
	}
	ents, err := s.ListDir(filesystem.TrashURI, true)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	var stored string
	for _, e := range ents {
		if e.Origin == file {
			found = true
			stored = e.Path
		}
	}
	if !found {
		t.Fatal("trashed file missing from trash://")
	}
	if err := s.RestoreTrash([]string{stored}); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(file); err != nil || string(b) != "hi" {
		t.Fatalf("restore: %v %q", err, b)
	}
}

func TestDeletePermanentSkipsTrash(t *testing.T) {
	t.Parallel()
	work := t.TempDir()
	s := NewFileService(nil, nil, nil, filepath.Join(work, "trash"))
	file := filepath.Join(work, "gone.txt")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := s.DeletePermanent([]string{file}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(file); !os.IsNotExist(err) {
		t.Fatal("file still present")
	}
	ents, err := s.ListDir(filesystem.TrashURI, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range ents {
		if e.Origin == file {
			t.Fatal("permanent delete landed in trash")
		}
	}
}

func TestRestoreDeletedToken(t *testing.T) {
	t.Parallel()
	work := t.TempDir()
	s := NewFileService(nil, nil, nil, filepath.Join(work, "trash"))
	file := filepath.Join(work, "undo.txt")
	if err := os.WriteFile(file, []byte("back"), 0o600); err != nil {
		t.Fatal(err)
	}
	token, err := s.Delete([]string{file})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RestoreDeleted(token); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(file); err != nil || string(b) != "back" {
		t.Fatalf("undo: %v %q", err, b)
	}
}

func TestMkdirRejectedInTrash(t *testing.T) {
	t.Parallel()
	s := NewFileService(nil, nil, nil, filepath.Join(t.TempDir(), "trash"))
	if _, err := s.Mkdir(filesystem.TrashURI, "nope"); err == nil {
		t.Fatal("expected trash write reject")
	}
}

func TestLeftoverBatchRestoreDeleted(t *testing.T) {
	t.Parallel()
	work := t.TempDir()
	s := NewFileService(nil, nil, nil, filepath.Join(work, "trash"))
	file := filepath.Join(work, "old.txt")
	if err := os.WriteFile(file, []byte("legacy"), 0o600); err != nil {
		t.Fatal(err)
	}
	id, err := s.trash.MoveToTrash([]string{file})
	if err != nil || id == "" {
		t.Fatalf("MoveToTrash: %v %q", err, id)
	}
	ents, err := s.ListDir(filesystem.TrashURI, true)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range ents {
		if e.Origin == file {
			found = true
		}
	}
	if !found {
		t.Fatal("leftover batch missing from trash://")
	}
	if err := s.RestoreDeleted(id); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(file); err != nil || string(b) != "legacy" {
		t.Fatalf("leftover undo: %v %q", err, b)
	}
}

func TestRenameRejectedInTrash(t *testing.T) {
	t.Parallel()
	work := t.TempDir()
	s := NewFileService(nil, nil, nil, filepath.Join(work, "trash"))
	file := filepath.Join(work, "note.txt")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Delete([]string{file}); err != nil {
		t.Fatal(err)
	}
	ents, err := s.ListDir(filesystem.TrashURI, true)
	if err != nil {
		t.Fatal(err)
	}
	var stored string
	for _, e := range ents {
		if e.Origin == file {
			stored = e.Path
		}
	}
	if stored == "" {
		t.Fatal("missing stored path")
	}
	if _, err := s.Rename(stored, "nope.txt"); err == nil {
		t.Fatal("expected rename in trash to fail")
	}
}
