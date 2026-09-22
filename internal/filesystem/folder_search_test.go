package filesystem

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/erikharutyunyan/double-pane/internal/domain"
)

func TestSearchFoldersFindsDirs(t *testing.T) {
	root := t.TempDir()
	_ = os.MkdirAll(filepath.Join(root, "src", "components"), 0o755)
	_ = os.MkdirAll(filepath.Join(root, "build", "out"), 0o755)
	_ = os.MkdirAll(filepath.Join(root, "pkg", "comp_helper"), 0o755)
	_ = os.WriteFile(filepath.Join(root, "src", "main.go"), []byte("x"), 0o644)

	var hits []domain.SearchHit
	_, err := SearchFolders(context.Background(), root, "comp", "", "build", false, 50, FolderSearchCallbacks{
		OnHit: func(h domain.SearchHit) { hits = append(hits, h) },
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) < 1 {
		t.Fatalf("expected hits, got %#v", hits)
	}
	for _, h := range hits {
		if !h.IsDir {
			t.Fatal("expected dirs only")
		}
		if h.Name == "build" || filepath.Base(h.Path) == "out" {
			t.Fatalf("build subtree should be excluded: %#v", h)
		}
	}
	found := false
	for _, h := range hits {
		if h.Name == "components" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected components dir, got %#v", hits)
	}
}

func TestSearchFoldersEmptyQueryListsDirs(t *testing.T) {
	root := t.TempDir()
	_ = os.MkdirAll(filepath.Join(root, "a"), 0o755)
	_ = os.MkdirAll(filepath.Join(root, "b"), 0o755)
	var hits []domain.SearchHit
	_, err := SearchFolders(context.Background(), root, "", "", "", false, 50, FolderSearchCallbacks{
		OnHit: func(h domain.SearchHit) { hits = append(hits, h) },
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) < 2 {
		t.Fatalf("got %#v", hits)
	}
}
