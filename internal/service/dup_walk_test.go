package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/erikharutyunyan/go-file-manager/internal/domain"
)

func TestWalkForDuplicatesPastSearchCaps(t *testing.T) {
	const n = 2500
	ents := make([]domain.FileEntry, n)
	for i := 0; i < n; i++ {
		name := fmt.Sprintf("f%04d.bin", i)
		ents[i] = domain.FileEntry{Name: name, Path: "ssh://h/root/" + name, Size: 1}
	}
	list := func(path string, _ bool) ([]domain.FileEntry, error) {
		if path == "ssh://h/root" {
			return ents, nil
		}
		return nil, fmt.Errorf("unexpected %s", path)
	}
	files, skipped, err := walkForDuplicates(context.Background(), list, "ssh://h/root", false, 0, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(skipped) != 0 {
		t.Fatalf("skipped %v", skipped)
	}
	if len(files) != n {
		t.Fatalf("got %d files, walker stopped early (search cap is 2000)", len(files))
	}
}

func TestWalkForDuplicatesDeepTree(t *testing.T) {
	// walkRemote stops at depth 8; this file sits at depth 12.
	dirs := []string{"ssh://h/d0"}
	cur := "ssh://h/d0"
	for i := 1; i <= 12; i++ {
		next := fmt.Sprintf("%s/d%d", cur, i)
		dirs = append(dirs, next)
		cur = next
	}
	leaf := cur + "/deep.bin"
	tree := map[string][]domain.FileEntry{}
	for i, d := range dirs {
		if i+1 < len(dirs) {
			name := fmt.Sprintf("d%d", i+1)
			tree[d] = []domain.FileEntry{{Name: name, Path: dirs[i+1], IsDir: true}}
		} else {
			tree[d] = []domain.FileEntry{{Name: "deep.bin", Path: leaf, Size: 3}}
		}
	}
	list := func(path string, _ bool) ([]domain.FileEntry, error) {
		return tree[path], nil
	}
	files, _, err := walkForDuplicates(context.Background(), list, "ssh://h/d0", false, 0, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Path != leaf {
		t.Fatalf("expected deep file, got %+v", files)
	}
}

func TestWalkForDuplicatesChildListErrorNotFatal(t *testing.T) {
	list := func(path string, _ bool) ([]domain.FileEntry, error) {
		switch path {
		case "ssh://h/root":
			return []domain.FileEntry{
				{Name: "ok", Path: "ssh://h/root/ok", IsDir: true},
				{Name: "denied", Path: "ssh://h/root/denied", IsDir: true},
				{Name: "a.bin", Path: "ssh://h/root/a.bin", Size: 2},
			}, nil
		case "ssh://h/root/ok":
			return []domain.FileEntry{{Name: "b.bin", Path: "ssh://h/root/ok/b.bin", Size: 2}}, nil
		case "ssh://h/root/denied":
			return nil, fmt.Errorf("permission denied")
		default:
			return nil, fmt.Errorf("unexpected %s", path)
		}
	}
	files, skipped, err := walkForDuplicates(context.Background(), list, "ssh://h/root", false, 0, nil, "")
	if err != nil {
		t.Fatalf("child list error must not be fatal: %v", err)
	}
	if len(skipped) != 1 || skipped[0].Path != "ssh://h/root/denied" {
		t.Fatalf("skipped = %+v", skipped)
	}
	if len(files) != 2 {
		t.Fatalf("files %+v", files)
	}
}

func TestWalkForDuplicatesRemoteZipIsFile(t *testing.T) {
	list := func(path string, _ bool) ([]domain.FileEntry, error) {
		if path == "ssh://h/root" {
			return []domain.FileEntry{
				{Name: "photos.zip", Path: "ssh://h/root/photos.zip", Size: 99},
			}, nil
		}
		return nil, fmt.Errorf("unexpected %s", path)
	}
	files, _, err := walkForDuplicates(context.Background(), list, "ssh://h/root", false, 0, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Name != "photos.zip" {
		t.Fatalf("remote zip must be hashed as a file: %+v", files)
	}
}

func TestWalkForDuplicatesSessionDropIsFatal(t *testing.T) {
	list := func(path string, _ bool) ([]domain.FileEntry, error) {
		if path == "ssh://h/root" {
			return []domain.FileEntry{{Name: "sub", Path: "ssh://h/root/sub", IsDir: true}}, nil
		}
		return nil, fmt.Errorf("not connected to h; connect first")
	}
	_, _, err := walkForDuplicates(context.Background(), list, "ssh://h/root", false, 0, nil, "")
	if err == nil {
		t.Fatal("expected fatal session error")
	}
}

func TestWalkForDuplicatesCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	list := func(string, bool) ([]domain.FileEntry, error) {
		select {
		case <-started:
		default:
			close(started)
		}
		time.Sleep(30 * time.Second)
		return nil, nil
	}
	errCh := make(chan error, 1)
	go func() {
		_, _, err := walkForDuplicates(ctx, list, "root", false, 0, nil, "")
		errCh <- err
	}()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("list did not start")
	}
	cancel()
	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected cancel")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("walk did not stop")
	}
}

func TestEtaSeconds(t *testing.T) {
	if etaSeconds(0, "local") != 0 {
		t.Fatal("zero bytes")
	}
	if etaSeconds(200<<20, "local") != 1 {
		t.Fatalf("local 200MiB: %d", etaSeconds(200<<20, "local"))
	}
	if etaSeconds(4<<20, "mega") != 1 {
		t.Fatalf("mega: %d", etaSeconds(4<<20, "mega"))
	}
}

func TestWalkForDuplicatesExclude(t *testing.T) {
	listed := map[string]int{}
	list := func(path string, _ bool) ([]domain.FileEntry, error) {
		listed[path]++
		switch path {
		case "ssh://h/root":
			return []domain.FileEntry{
				{Name: "a.bin", Path: "ssh://h/root/a.bin", Size: 2},
				{Name: "skip.md", Path: "ssh://h/root/skip.md", Size: 8},
				{Name: "build", Path: "ssh://h/root/build", IsDir: true},
			}, nil
		case "ssh://h/root/build":
			return []domain.FileEntry{
				{Name: "ignored.bin", Path: "ssh://h/root/build/ignored.bin", Size: 2},
			}, nil
		default:
			return nil, fmt.Errorf("unexpected %s", path)
		}
	}
	files, skipped, err := walkForDuplicates(context.Background(), list, "ssh://h/root", false, 0, nil, "*.md, build")
	if err != nil {
		t.Fatal(err)
	}
	if len(skipped) != 0 {
		t.Fatalf("exclude must not count as skip: %+v", skipped)
	}
	if listed["ssh://h/root/build"] != 0 {
		t.Fatal("excluded dir must not be listed")
	}
	if len(files) != 1 || files[0].Name != "a.bin" {
		t.Fatalf("files %+v", files)
	}

	all, _, err := walkForDuplicates(context.Background(), list, "ssh://h/root", false, 0, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Fatalf("empty exclude should keep all files, got %d", len(all))
	}
}

func TestWalkSkipsSystemTrash(t *testing.T) {
	root := "/tmp/home"
	trash := "/tmp/home/.Trash"
	keep := root + "/keep.txt"
	list := func(path string, _ bool) ([]domain.FileEntry, error) {
		switch path {
		case root:
			return []domain.FileEntry{
				{Name: "keep.txt", Path: keep, Size: 1},
				{Name: ".Trash", Path: trash, IsDir: true},
			}, nil
		case trash:
			return []domain.FileEntry{
				{Name: "gone.txt", Path: trash + "/gone.txt", Size: 1},
			}, nil
		default:
			return nil, fmt.Errorf("unexpected %s", path)
		}
	}
	files, _, err := walkForDuplicates(context.Background(), list, root, true, 0, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Path != keep {
		t.Fatalf("files %+v", files)
	}
}
