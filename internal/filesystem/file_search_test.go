package filesystem

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/erikharutyunyan/double-pane/internal/domain"
)

func TestSearchFilesFindsBasenameOnly(t *testing.T) {
	root := t.TempDir()
	_ = os.MkdirAll(filepath.Join(root, "a", "sub"), 0o755)
	_ = os.MkdirAll(filepath.Join(root, "a", "readme"), 0o755)
	_ = os.MkdirAll(filepath.Join(root, "src"), 0o755)
	_ = os.WriteFile(filepath.Join(root, "a", "readme.md"), []byte("x"), 0o644)
	_ = os.WriteFile(filepath.Join(root, "a", "notes.txt"), []byte("x"), 0o644)
	_ = os.WriteFile(filepath.Join(root, "src", "main.go"), []byte("package main\n"), 0o644)

	var hits []domain.SearchHit
	_, err := SearchFiles(context.Background(), root, "readme", "", "", false, 50, FolderSearchCallbacks{
		OnHit: func(h domain.SearchHit) { hits = append(hits, h) },
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].Name != "readme.md" || hits[0].IsDir {
		t.Fatalf("got %#v", hits)
	}

	var folders []domain.SearchHit
	_, err = SearchFolders(context.Background(), root, "readme", "", "", false, 50, FolderSearchCallbacks{
		OnHit: func(h domain.SearchHit) { folders = append(folders, h) },
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range folders {
		if h.Name == "readme.md" || !h.IsDir {
			t.Fatalf("folder mode returned a file: %#v", folders)
		}
	}

	var pathHits []domain.SearchHit
	_, err = SearchFiles(context.Background(), root, "src", "", "", false, 50, FolderSearchCallbacks{
		OnHit: func(h domain.SearchHit) { pathHits = append(pathHits, h) },
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(pathHits) != 0 {
		t.Fatalf("basename-only: query src must not hit src/main.go, got %#v", pathHits)
	}
}

func TestSearchFilesIncludeExclude(t *testing.T) {
	root := t.TempDir()
	_ = os.MkdirAll(filepath.Join(root, "a"), 0o755)
	_ = os.WriteFile(filepath.Join(root, "a", "readme.md"), []byte("x"), 0o644)
	_ = os.WriteFile(filepath.Join(root, "a", "notes.txt"), []byte("x"), 0o644)

	var included []domain.SearchHit
	_, err := SearchFiles(context.Background(), root, "e", "*.md", "", false, 50, FolderSearchCallbacks{
		OnHit: func(h domain.SearchHit) { included = append(included, h) },
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(included) != 1 || included[0].Name != "readme.md" {
		t.Fatalf("include: %#v", included)
	}

	var excluded []domain.SearchHit
	_, err = SearchFiles(context.Background(), root, "readme", "", "*.md", false, 50, FolderSearchCallbacks{
		OnHit: func(h domain.SearchHit) { excluded = append(excluded, h) },
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(excluded) != 0 {
		t.Fatalf("exclude: %#v", excluded)
	}
}

func TestSearchFilesSkipsSymlink(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "readme.md")
	if err := os.WriteFile(target, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(root, "link-readme.md")); err != nil {
		t.Fatal(err)
	}

	var hits []domain.SearchHit
	_, err := SearchFiles(context.Background(), root, "readme", "", "", false, 50, FolderSearchCallbacks{
		OnHit: func(h domain.SearchHit) { hits = append(hits, h) },
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].Name != "readme.md" {
		t.Fatalf("got %#v", hits)
	}
}

func TestSearchFilesEmptyQueryEmitsNothing(t *testing.T) {
	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, "readme.md"), []byte("x"), 0o644)
	var hits []domain.SearchHit
	truncated, err := SearchFiles(context.Background(), root, "  ", "", "", false, 50, FolderSearchCallbacks{
		OnHit: func(h domain.SearchHit) { hits = append(hits, h) },
	})
	if err != nil {
		t.Fatal(err)
	}
	if truncated || len(hits) != 0 {
		t.Fatalf("truncated=%v hits=%#v", truncated, hits)
	}
}
