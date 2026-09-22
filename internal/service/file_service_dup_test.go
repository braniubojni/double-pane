package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/erikharutyunyan/double-pane/internal/domain"
	"github.com/erikharutyunyan/double-pane/internal/filesystem"
)

func waitDupDone(t *testing.T, done <-chan domain.DupDonePayload) domain.DupDonePayload {
	t.Helper()
	select {
	case p := <-done:
		return p
	case <-time.After(10 * time.Second):
		t.Fatal("timeout waiting for dup:done")
		return domain.DupDonePayload{}
	}
}

func TestDuplicateScanIdenticalAndDifferent(t *testing.T) {
	work := t.TempDir()
	root := filepath.Join(work, "scan")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	same := []byte("same-bytes-for-dup")
	if err := os.WriteFile(filepath.Join(root, "a.bin"), same, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "b.bin"), same, 0o644); err != nil {
		t.Fatal(err)
	}
	other := []byte("DIFF-bytes-for-dup")
	if len(other) != len(same) {
		t.Fatal("fixture sizes")
	}
	if err := os.WriteFile(filepath.Join(root, "c.bin"), other, 0o644); err != nil {
		t.Fatal(err)
	}

	s := NewFileService(nil, nil, nil, filepath.Join(work, "trash"))
	est, err := s.EstimateDuplicateScan(root, false, 0, "", false, 90, false)
	if err != nil {
		t.Fatal(err)
	}
	if est.FileCount != 3 || est.Protocol != "local" || est.MegaDownload {
		t.Fatalf("estimate %+v", est)
	}

	var mu sync.Mutex
	var groups []domain.DuplicateGroup
	var firstProgress *domain.DupProgressPayload
	hashed := false
	done := make(chan domain.DupDonePayload, 1)
	s.onEvent = func(name string, data any) {
		switch name {
		case "dup:progress":
			mu.Lock()
			if firstProgress == nil {
				p := data.(domain.DupProgressPayload)
				cp := p
				firstProgress = &cp
				if hashed {
					t.Error("dup:progress totals emitted after hash started")
				}
			}
			mu.Unlock()
		case "dup:done":
			p := data.(domain.DupDonePayload)
			mu.Lock()
			groups = p.Groups
			mu.Unlock()
			done <- p
		}
	}
	realHash := s.hashFile
	s.hashFile = func(ctx context.Context, path string) (string, error) {
		mu.Lock()
		hashed = true
		mu.Unlock()
		if realHash != nil {
			return realHash(ctx, path)
		}
		return filesystem.HashFile(ctx, path)
	}
	id := s.NewJobID()
	if err := s.StartDuplicateScan(id, root, false, 0, "", false, 90, false); err != nil {
		t.Fatal(err)
	}
	got := waitDupDone(t, done)
	if got.Error != "" {
		t.Fatal(got.Error)
	}
	mu.Lock()
	defer mu.Unlock()
	if firstProgress == nil {
		t.Fatal("expected dup:progress before hash")
	}
	if firstProgress.DoneFiles != 0 || firstProgress.TotalFiles != 3 {
		t.Fatalf("first progress %+v", firstProgress)
	}
	if len(groups) != 1 {
		t.Fatalf("want 1 group, got %d %+v", len(groups), groups)
	}
	if len(groups[0].Files) != 2 {
		t.Fatalf("want 2 identical files, got %+v", groups[0].Files)
	}
}

func TestDuplicateScanDoesNotFloodProgress(t *testing.T) {
	work := t.TempDir()
	const n = 200
	for i := 0; i < n; i++ {
		name := filepath.Join(work, fmt.Sprintf("f%03d.bin", i))
		if err := os.WriteFile(name, make([]byte, i+1), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	s := NewFileService(nil, nil, nil, filepath.Join(work, "trash"))
	var mu sync.Mutex
	var ticks []domain.DupProgressPayload
	var groupN int
	done := make(chan domain.DupDonePayload, 1)
	s.onEvent = func(name string, data any) {
		switch name {
		case "dup:progress":
			mu.Lock()
			ticks = append(ticks, data.(domain.DupProgressPayload))
			mu.Unlock()
		case "dup:group":
			mu.Lock()
			groupN++
			mu.Unlock()
		case "dup:done":
			done <- data.(domain.DupDonePayload)
		}
	}
	if err := s.StartDuplicateScan(s.NewJobID(), work, false, 0, "", false, 90, false); err != nil {
		t.Fatal(err)
	}
	got := waitDupDone(t, done)
	if got.Error != "" {
		t.Fatal(got.Error)
	}
	mu.Lock()
	defer mu.Unlock()
	if groupN != 0 {
		t.Fatalf("dup:group flood: %d events (groups belong on dup:done)", groupN)
	}
	if len(ticks) > 8 {
		t.Fatalf("progress flood: %d ticks for %d unique-size files", len(ticks), n)
	}
	if ticks[0].DoneFiles != 0 || ticks[0].TotalFiles != n {
		t.Fatalf("first progress %+v", ticks[0])
	}
	last := ticks[len(ticks)-1]
	if last.DoneFiles != n {
		t.Fatalf("last progress %+v", last)
	}
}

func TestStartDuplicateScanRejectsArchiveAndBadRoot(t *testing.T) {
	work := t.TempDir()
	src := filepath.Join(work, "hello.txt")
	if err := os.WriteFile(src, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	zipPath := filepath.Join(work, "out.zip")
	if err := filesystem.Archive(context.Background(), []string{src}, zipPath, "zip", "", nil); err != nil {
		t.Fatal(err)
	}
	s := NewFileService(nil, nil, nil, filepath.Join(work, "trash"))
	if err := s.StartDuplicateScan(s.NewJobID(), zipPath, false, 0, "", false, 90, false); err == nil {
		t.Fatal("expected archive root rejected")
	}
	if err := s.StartDuplicateScan(s.NewJobID(), filepath.Join(work, "missing"), false, 0, "", false, 90, false); err == nil {
		t.Fatal("expected ListDir failure")
	}
	if err := s.StartDuplicateScan("", work, false, 0, "", false, 90, false); err == nil || !strings.Contains(err.Error(), "jobID") {
		t.Fatalf("empty jobID: %v", err)
	}
	if err := s.StartDuplicateScan(s.NewJobID(), "", false, 0, "", false, 90, false); err == nil {
		t.Fatal("empty root")
	}
}

func TestStartDuplicateScanCancelDuringHash(t *testing.T) {
	work := t.TempDir()
	root := filepath.Join(work, "scan")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.bin"), []byte("aa"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "b.bin"), []byte("aa"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := NewFileService(nil, nil, nil, filepath.Join(work, "trash"))
	started := make(chan struct{})
	s.hashFile = func(ctx context.Context, path string) (string, error) {
		select {
		case <-started:
		default:
			close(started)
		}
		<-ctx.Done()
		return "", ctx.Err()
	}
	done := make(chan domain.DupDonePayload, 1)
	var fatal bool
	s.onEvent = func(name string, data any) {
		switch name {
		case "dup:error":
			if data.(domain.DupErrorPayload).Fatal {
				fatal = true
			}
		case "dup:done":
			done <- data.(domain.DupDonePayload)
		}
	}
	id := s.NewJobID()
	if err := s.StartDuplicateScan(id, root, false, 0, "", false, 90, false); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("hash did not start")
	}
	if err := s.CancelJob(id); err != nil {
		t.Fatal(err)
	}
	got := waitDupDone(t, done)
	if got.Error == "" {
		t.Fatal("expected cancel error on dup:done")
	}
	if !fatal {
		t.Fatal("expected fatal dup:error")
	}
	ents, err := os.ReadDir(s.megaCacheDir())
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if len(ents) != 0 {
		t.Fatalf("mega temps left: %v", ents)
	}
}

func TestStartDuplicateScanCancelStopsWalk(t *testing.T) {
	work := t.TempDir()
	root := filepath.Join(work, "scan")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	big := make([]byte, 2<<20)
	for i := range big {
		big[i] = byte(i)
	}
	if err := os.WriteFile(filepath.Join(root, "a.bin"), big, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "b.bin"), big, 0o644); err != nil {
		t.Fatal(err)
	}

	s := NewFileService(nil, nil, nil, filepath.Join(work, "trash"))
	s.dupCacheDir = filepath.Join(work, "dup-cache")
	if err := os.MkdirAll(s.dupCacheDir, 0o700); err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	s.listDup = func(path string, showHidden bool) ([]domain.FileEntry, error) {
		select {
		case <-started:
		default:
			close(started)
		}
		time.Sleep(30 * time.Second)
		return filesystem.ListDir(path, showHidden)
	}

	done := make(chan domain.DupDonePayload, 1)
	s.onEvent = func(name string, data any) {
		if name == "dup:done" {
			done <- data.(domain.DupDonePayload)
		}
	}
	id := s.NewJobID()
	if err := s.StartDuplicateScan(id, root, false, 0, "", false, 90, false); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("walk did not start")
	}
	if err := s.CancelJob(id); err != nil {
		t.Fatal(err)
	}
	got := waitDupDone(t, done)
	if got.Error == "" {
		t.Fatal("expected cancel error on dup:done")
	}
	ents, err := os.ReadDir(s.megaCacheDir())
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if len(ents) != 0 {
		t.Fatalf("mega temps left: %v", ents)
	}
}

func TestStartDuplicateScanOneJob(t *testing.T) {
	work := t.TempDir()
	if err := os.WriteFile(filepath.Join(work, "a.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := NewFileService(nil, nil, nil, filepath.Join(work, "trash"))
	block := make(chan struct{})
	s.hashFile = func(ctx context.Context, path string) (string, error) {
		<-block
		return "abc", ctx.Err()
	}
	// two same-size files so hashing runs
	if err := os.WriteFile(filepath.Join(work, "b.txt"), []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}
	id := s.NewJobID()
	if err := s.StartDuplicateScan(id, work, false, 0, "", false, 90, false); err != nil {
		t.Fatal(err)
	}
	if err := s.StartDuplicateScan(s.NewJobID(), work, false, 0, "", false, 90, false); err == nil {
		t.Fatal("expected already running")
	}
	close(block)
	_ = s.CancelJob(id)
}

func TestDuplicateScanExcludeMarkdown(t *testing.T) {
	work := t.TempDir()
	root := filepath.Join(work, "scan")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	same := []byte("same-bytes-for-dup")
	if err := os.WriteFile(filepath.Join(root, "a.bin"), same, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "b.bin"), same, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "skip.md"), []byte("not-a-dup"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := NewFileService(nil, nil, nil, filepath.Join(work, "trash"))
	all, err := s.EstimateDuplicateScan(root, false, 0, "", false, 90, false)
	if err != nil {
		t.Fatal(err)
	}
	if all.FileCount != 3 {
		t.Fatalf("empty exclude fileCount=%d", all.FileCount)
	}
	est, err := s.EstimateDuplicateScan(root, false, 0, "*.md", false, 90, false)
	if err != nil {
		t.Fatal(err)
	}
	if est.FileCount != 2 {
		t.Fatalf("*.md exclude fileCount=%d want 2", est.FileCount)
	}

	var mu sync.Mutex
	var skipErrs []string
	done := make(chan domain.DupDonePayload, 1)
	s.onEvent = func(name string, data any) {
		switch name {
		case "dup:error":
			mu.Lock()
			skipErrs = append(skipErrs, data.(domain.DupErrorPayload).Path)
			mu.Unlock()
		case "dup:done":
			done <- data.(domain.DupDonePayload)
		}
	}
	if err := s.StartDuplicateScan(s.NewJobID(), root, false, 0, "*.md", false, 90, false); err != nil {
		t.Fatal(err)
	}
	got := waitDupDone(t, done)
	if got.Error != "" {
		t.Fatal(got.Error)
	}
	groups := got.Groups
	if len(groups) != 1 || len(groups[0].Files) != 2 {
		t.Fatalf("want one group of bins, got %+v", groups)
	}
	for _, f := range groups[0].Files {
		if strings.HasSuffix(f.Name, ".md") {
			t.Fatalf("md should be excluded: %+v", f)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	for _, p := range skipErrs {
		if strings.HasSuffix(p, "skip.md") {
			t.Fatal("excluded files must not be listed as skips")
		}
	}
}

func TestDupCacheDirIsSiblingOfTrash(t *testing.T) {
	work := t.TempDir()
	trash := filepath.Join(work, "trash")
	s := NewFileService(nil, nil, nil, trash)
	want := filepath.Join(work, "dup-cache")
	if s.megaCacheDir() != want {
		t.Fatalf("dup-cache=%q want %q (not under trash)", s.megaCacheDir(), want)
	}
	if strings.Contains(s.megaCacheDir(), string(filepath.Separator)+"trash"+string(filepath.Separator)) {
		t.Fatal("dup-cache must not live under trash/")
	}
}

func TestMigrateDupCacheFromTrash(t *testing.T) {
	work := t.TempDir()
	trash := filepath.Join(work, "trash")
	old := filepath.Join(trash, "dup-cache")
	if err := os.MkdirAll(old, 0o700); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(old, "leftover")
	if err := os.WriteFile(legacy, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := NewFileService(nil, nil, nil, trash)
	want := filepath.Join(work, "dup-cache", "leftover")
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("migrated file missing: %v", err)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatalf("old trash/dup-cache should be gone: %v", err)
	}
	if s.megaCacheDir() != filepath.Join(work, "dup-cache") {
		t.Fatalf("cache dir=%s", s.megaCacheDir())
	}
}

func TestPurgeTrashDoesNotDeleteDupCache(t *testing.T) {
	work := t.TempDir()
	trash := filepath.Join(work, "trash")
	s := NewFileService(nil, nil, nil, trash)
	cache := s.megaCacheDir()
	if err := os.MkdirAll(cache, 0o700); err != nil {
		t.Fatal(err)
	}
	keep := filepath.Join(cache, "keep-me")
	if err := os.WriteFile(keep, []byte("alive"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Misplaced leftover under trash must also survive PurgeTrash.
	misplacedDir := filepath.Join(trash, "dup-cache")
	if err := os.MkdirAll(misplacedDir, 0o700); err != nil {
		t.Fatal(err)
	}
	misplaced := filepath.Join(misplacedDir, "still-here")
	if err := os.WriteFile(misplaced, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	oldBatch := filepath.Join(trash, "20200101-000000001-1")
	if err := os.MkdirAll(oldBatch, 0o700); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(oldBatch, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	if err := s.PurgeTrash(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatalf("sibling dup-cache file deleted by PurgeTrash: %v", err)
	}
	if _, err := os.Stat(misplaced); err != nil {
		t.Fatalf("trash/dup-cache deleted by PurgeTrash: %v", err)
	}
	if _, err := os.Stat(oldBatch); err != nil {
		t.Fatalf("leftover trash batch must survive PurgeTrash: %v", err)
	}
}

func TestPurgeDupCacheAge(t *testing.T) {
	work := t.TempDir()
	s := NewFileService(nil, nil, nil, filepath.Join(work, "trash"))
	cache := s.megaCacheDir()
	if err := os.MkdirAll(cache, 0o700); err != nil {
		t.Fatal(err)
	}
	oldPath := filepath.Join(cache, "old-temp")
	freshPath := filepath.Join(cache, "fresh-temp")
	if err := os.WriteFile(oldPath, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(freshPath, []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().Add(-25 * time.Hour)
	if err := os.Chtimes(oldPath, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	// Leave a trash batch alone — purge must never touch it.
	trashBatch := filepath.Join(work, "trash", "20200101-000000001-1")
	if err := os.MkdirAll(trashBatch, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(trashBatch, "manifest.json"), []byte("[]"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := s.PurgeDupCache(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatalf("old temp should be deleted: %v", err)
	}
	if _, err := os.Stat(freshPath); err != nil {
		t.Fatalf("fresh temp should be kept: %v", err)
	}
	if _, err := os.Stat(trashBatch); err != nil {
		t.Fatalf("trash must not be touched: %v", err)
	}
}

func TestCancelClearsInFlightDupTemp(t *testing.T) {
	work := t.TempDir()
	root := filepath.Join(work, "scan")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.bin"), []byte("aa"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "b.bin"), []byte("aa"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := NewFileService(nil, nil, nil, filepath.Join(work, "trash"))
	cache := s.megaCacheDir()
	if err := os.MkdirAll(cache, 0o700); err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	var tempPath string
	s.hashFile = func(ctx context.Context, _ string) (string, error) {
		f, err := os.CreateTemp(cache, "dup-*")
		if err != nil {
			return "", err
		}
		tempPath = f.Name()
		_ = f.Close()
		select {
		case <-started:
		default:
			close(started)
		}
		<-ctx.Done()
		// Simulate cancel/crash leaving the temp (no per-file Remove).
		return "", ctx.Err()
	}
	done := make(chan domain.DupDonePayload, 1)
	s.onEvent = func(name string, data any) {
		if name == "dup:done" {
			done <- data.(domain.DupDonePayload)
		}
	}
	id := s.NewJobID()
	if err := s.StartDuplicateScan(id, root, false, 0, "", false, 90, false); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("hash did not start")
	}
	if err := s.CancelJob(id); err != nil {
		t.Fatal(err)
	}
	got := waitDupDone(t, done)
	if got.Error == "" {
		t.Fatal("expected cancel error")
	}
	if tempPath == "" {
		t.Fatal("in-flight temp was not created")
	}
	if _, err := os.Stat(tempPath); !os.IsNotExist(err) {
		t.Fatalf("in-flight temp should be removed on job end: %v", err)
	}
	ents, err := os.ReadDir(cache)
	if err != nil {
		t.Fatal(err)
	}
	if len(ents) != 0 {
		t.Fatalf("dup-cache not empty after cancel: %v", ents)
	}
}
