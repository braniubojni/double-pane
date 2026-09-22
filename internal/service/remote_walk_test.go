package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/erikharutyunyan/double-pane/internal/domain"
	"github.com/erikharutyunyan/double-pane/internal/filesystem"
)

// fakeRemoteBackend implements remoteBackend with an in-memory tree.
// Unimplemented methods panic if called.
type fakeRemoteBackend struct {
	remoteBackend
	tree      map[string][]domain.FileEntry // dir path -> children
	files     map[string][]byte             // file path -> content
	openFail  map[string]error              // path -> OpenRead error
	openReads int
	openPaths []string
}

func (f *fakeRemoteBackend) ListDir(path string, _ bool) ([]domain.FileEntry, error) {
	return f.tree[path], nil
}

func (f *fakeRemoteBackend) OpenRead(path string) (io.ReadCloser, error) {
	f.openReads++
	f.openPaths = append(f.openPaths, path)
	if err, ok := f.openFail[path]; ok {
		return nil, err
	}
	data, ok := f.files[path]
	if !ok {
		return nil, errors.New("no such file")
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func newFakeTree() *fakeRemoteBackend {
	return &fakeRemoteBackend{tree: map[string][]domain.FileEntry{
		"ssh://h/root": {
			{Name: "a.txt", Path: "ssh://h/root/a.txt"},
			{Name: "sub", Path: "ssh://h/root/sub", IsDir: true},
			{Name: ".hidden", Path: "ssh://h/root/.hidden"},
		},
		"ssh://h/root/sub": {
			{Name: "b.txt", Path: "ssh://h/root/sub/b.txt"},
			{Name: "excluded", Path: "ssh://h/root/sub/excluded", IsDir: true},
		},
		"ssh://h/root/sub/excluded": {
			{Name: "c.txt", Path: "ssh://h/root/sub/excluded/c.txt"},
		},
	}}
}

func newContentFake() *fakeRemoteBackend {
	root := "ssh://h/root"
	big := make([]byte, filesystem.MaxContentFileBytes+1)
	for i := range big {
		big[i] = 'x'
	}
	return &fakeRemoteBackend{
		tree: map[string][]domain.FileEntry{
			root: {
				{Name: "hit.txt", Path: root + "/hit.txt", Size: 28},
				{Name: "bin.dat", Path: root + "/bin.dat", Size: 4},
				{Name: "huge.txt", Path: root + "/huge.txt", Size: int64(len(big))},
				{Name: "deny.txt", Path: root + "/deny.txt", Size: 10},
				{Name: "other.go", Path: root + "/other.go", Size: 20},
				{Name: "skip", Path: root + "/skip", IsDir: true},
			},
			root + "/skip": {
				{Name: "nested.txt", Path: root + "/skip/nested.txt", Size: 20},
			},
		},
		files: map[string][]byte{
			root + "/hit.txt":         []byte("line one\nhello world\nline three\n"),
			root + "/bin.dat":         {0x00, 0x01, 0x02, 'x'},
			root + "/huge.txt":        big,
			root + "/deny.txt":        []byte("hello deny\n"),
			root + "/other.go":        []byte("package hello\n"),
			root + "/skip/nested.txt": []byte("hello nested\n"),
		},
		openFail: map[string]error{
			root + "/deny.txt": errors.New("permission denied"),
		},
	}
}

func TestSearchTreeRemote(t *testing.T) {
	s := &FileService{}
	be := newFakeTree()

	hits, err := s.searchTreeRemote(be, "ssh://h/root", "txt", false, 10)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, h := range hits {
		names[h.Name] = true
	}
	if !names["a.txt"] || !names["b.txt"] {
		t.Fatalf("expected a.txt and b.txt, got %+v", hits)
	}
	if names[".hidden"] {
		t.Fatalf("hidden entry should not appear with showHidden=false: %+v", hits)
	}
}

func TestSearchFoldersRemotePrunesExcluded(t *testing.T) {
	s := &FileService{}
	be := newFakeTree()

	var hits []domain.SearchHit
	_, err := s.searchFoldersRemote(context.Background(), be, "ssh://h/root", "", "", "excluded", false, 10,
		func(h domain.SearchHit) { hits = append(hits, h) },
		func(string, error) {},
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range hits {
		if h.Name == "excluded" {
			t.Fatalf("excluded dir should be pruned, got %+v", hits)
		}
	}
	found := false
	for _, h := range hits {
		if h.Name == "sub" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected sub folder hit, got %+v", hits)
	}
}

func TestSearchContentRemoteFindsMatch(t *testing.T) {
	s := &FileService{}
	be := newContentFake()
	var hits []domain.ContentSearchHit
	truncated, err := s.searchContentRemote(context.Background(), be, "ssh://h/root", "hello", "", "", false, false, 50,
		func(h domain.ContentSearchHit) { hits = append(hits, h) },
		func(string, error) {},
	)
	if err != nil {
		t.Fatal(err)
	}
	if truncated {
		t.Fatal("unexpected truncated")
	}
	if len(hits) < 1 {
		t.Fatalf("expected hits, got %#v", hits)
	}
	found := false
	for _, h := range hits {
		if h.RelPath == "hit.txt" && h.Line == 2 && strings.Contains(h.LineText, "hello") {
			found = true
			if h.Column < 1 {
				t.Fatalf("bad column: %#v", h)
			}
		}
	}
	if !found {
		t.Fatalf("expected hit.txt line 2, got %#v", hits)
	}
}

func TestSearchContentRemoteSkipsHugeWithoutOpen(t *testing.T) {
	s := &FileService{}
	be := newContentFake()
	_, err := s.searchContentRemote(context.Background(), be, "ssh://h/root", "x", "", "", false, false, 50,
		func(domain.ContentSearchHit) {},
		func(string, error) {},
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range be.openPaths {
		if strings.HasSuffix(p, "huge.txt") {
			t.Fatalf("huge.txt must not be OpenRead: %v", be.openPaths)
		}
	}
}

func TestSearchContentRemoteSkipsBinary(t *testing.T) {
	s := &FileService{}
	be := newContentFake()
	var hits []domain.ContentSearchHit
	_, err := s.searchContentRemote(context.Background(), be, "ssh://h/root", "x", "", "", false, false, 50,
		func(h domain.ContentSearchHit) { hits = append(hits, h) },
		func(string, error) {},
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range hits {
		if h.RelPath == "bin.dat" {
			t.Fatalf("binary should yield no hits: %#v", hits)
		}
	}
}

func TestSearchContentRemoteDeniedContinues(t *testing.T) {
	s := &FileService{}
	be := newContentFake()
	var denied []string
	var hits []domain.ContentSearchHit
	_, err := s.searchContentRemote(context.Background(), be, "ssh://h/root", "hello", "", "", false, false, 50,
		func(h domain.ContentSearchHit) { hits = append(hits, h) },
		func(path string, _ error) { denied = append(denied, path) },
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(denied) != 1 || !strings.HasSuffix(denied[0], "deny.txt") {
		t.Fatalf("expected deny.txt denied, got %v", denied)
	}
	if len(hits) == 0 {
		t.Fatal("walk should continue after denied OpenRead")
	}
}

func TestSearchContentRemoteIncludeExclude(t *testing.T) {
	s := &FileService{}
	be := newContentFake()
	var hits []domain.ContentSearchHit
	_, err := s.searchContentRemote(context.Background(), be, "ssh://h/root", "hello", "*.go", "skip", false, false, 50,
		func(h domain.ContentSearchHit) { hits = append(hits, h) },
		func(string, error) {},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].RelPath != "other.go" {
		t.Fatalf("want only other.go, got %#v", hits)
	}
}

func TestSearchContentRemoteLimitTruncates(t *testing.T) {
	s := &FileService{}
	be := newContentFake()
	var hits []domain.ContentSearchHit
	truncated, err := s.searchContentRemote(context.Background(), be, "ssh://h/root", "hello", "", "", false, false, 1,
		func(h domain.ContentSearchHit) { hits = append(hits, h) },
		func(string, error) {},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !truncated {
		t.Fatal("expected truncated")
	}
	if len(hits) != 1 {
		t.Fatalf("want 1 hit, got %d", len(hits))
	}
}

func TestSearchContentRemoteCancel(t *testing.T) {
	s := &FileService{}
	be := newContentFake()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan struct{})
	go func() {
		_, _ = s.searchContentRemote(ctx, be, "ssh://h/root", "hello", "", "", false, false, 50,
			func(domain.ContentSearchHit) {},
			func(string, error) {},
		)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("cancel did not finish promptly")
	}
}

func TestStartSearchMEGAContentBlocked(t *testing.T) {
	s := &FileService{}
	err := s.StartSearch("job1", "mega://u@h/path", "hello", domain.SearchModeContent, "", "", false, false, 10)
	if err == nil || !strings.Contains(err.Error(), "MEGA") {
		t.Fatalf("want MEGA error, got %v", err)
	}
}
