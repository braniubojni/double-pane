package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/erikharutyunyan/go-file-manager/internal/domain"
)

func TestOCRScanPairsSameText(t *testing.T) {
	work := t.TempDir()
	root := filepath.Join(work, "scan")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "a.jpg"), flagJPEG(t, 64, 64, 0, 90))
	writeFile(t, filepath.Join(root, "b.jpg"), flagJPEG(t, 48, 48, 1, 90))

	const shared = "Login button submit form dashboard settings profile logout welcome"
	s := NewFileService(nil, nil, nil, filepath.Join(work, "trash"))
	s.ocrFile = func(ctx context.Context, path string) (string, error) {
		return shared, nil
	}
	done := make(chan domain.DupDonePayload, 1)
	s.onEvent = func(name string, data any) {
		if name == "dup:done" {
			done <- data.(domain.DupDonePayload)
		}
	}
	if err := s.StartDuplicateScan(s.NewJobID(), root, false, 0, "", false, 90, true); err != nil {
		t.Fatal(err)
	}
	got := waitDupDone(t, done)
	if got.Error != "" {
		t.Fatal(got.Error)
	}
	if len(got.Groups) != 1 || got.Groups[0].Kind != "ocr" {
		t.Fatalf("want 1 ocr group, got %+v", got.Groups)
	}
	if got.Groups[0].Snippet == "" {
		t.Fatal("expected snippet")
	}
	if len(got.Groups[0].Files) != 2 {
		t.Fatalf("files %+v", got.Groups[0].Files)
	}
}

func TestOCRScanEmptyTextNoGroup(t *testing.T) {
	work := t.TempDir()
	root := filepath.Join(work, "scan")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "a.jpg"), flagJPEG(t, 64, 64, 0, 90))
	writeFile(t, filepath.Join(root, "b.jpg"), flagJPEG(t, 48, 48, 1, 90))

	s := NewFileService(nil, nil, nil, filepath.Join(work, "trash"))
	s.ocrFile = func(ctx context.Context, path string) (string, error) {
		return "  ", nil
	}
	done := make(chan domain.DupDonePayload, 1)
	s.onEvent = func(name string, data any) {
		if name == "dup:done" {
			done <- data.(domain.DupDonePayload)
		}
	}
	if err := s.StartDuplicateScan(s.NewJobID(), root, false, 0, "", false, 90, true); err != nil {
		t.Fatal(err)
	}
	got := waitDupDone(t, done)
	if got.Error != "" {
		t.Fatal(got.Error)
	}
	if len(got.Groups) != 0 {
		t.Fatalf("want no groups, got %+v", got.Groups)
	}
}

func TestOCRScanExactPairNotInOCR(t *testing.T) {
	work := t.TempDir()
	root := filepath.Join(work, "scan")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	same := flagJPEG(t, 64, 64, 0, 90)
	writeFile(t, filepath.Join(root, "a.jpg"), same)
	writeFile(t, filepath.Join(root, "b.jpg"), same)

	s := NewFileService(nil, nil, nil, filepath.Join(work, "trash"))
	s.ocrFile = func(ctx context.Context, path string) (string, error) {
		t.Fatal("ocr should not run on exact twins")
		return "", nil
	}
	done := make(chan domain.DupDonePayload, 1)
	s.onEvent = func(name string, data any) {
		if name == "dup:done" {
			done <- data.(domain.DupDonePayload)
		}
	}
	if err := s.StartDuplicateScan(s.NewJobID(), root, false, 0, "", false, 90, true); err != nil {
		t.Fatal(err)
	}
	got := waitDupDone(t, done)
	if got.Error != "" {
		t.Fatal(got.Error)
	}
	if len(got.Groups) != 1 || got.Groups[0].Kind != "exact" {
		t.Fatalf("want exact only, got %+v", got.Groups)
	}
}

func TestOCRScanTimeoutSkipNotFatal(t *testing.T) {
	work := t.TempDir()
	root := filepath.Join(work, "scan")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "a.jpg"), flagJPEG(t, 64, 64, 0, 90))
	writeFile(t, filepath.Join(root, "b.jpg"), flagJPEG(t, 48, 48, 1, 90))

	s := NewFileService(nil, nil, nil, filepath.Join(work, "trash"))
	s.ocrFile = func(ctx context.Context, path string) (string, error) {
		return "", context.DeadlineExceeded
	}
	done := make(chan domain.DupDonePayload, 1)
	s.onEvent = func(name string, data any) {
		if name == "dup:done" {
			done <- data.(domain.DupDonePayload)
		}
	}
	if err := s.StartDuplicateScan(s.NewJobID(), root, false, 0, "", false, 90, true); err != nil {
		t.Fatal(err)
	}
	got := waitDupDone(t, done)
	if got.Error != "" {
		t.Fatalf("timeout should not be fatal: %s", got.Error)
	}
	if len(got.Groups) != 0 {
		t.Fatalf("groups %+v", got.Groups)
	}
}

func TestEstimateOcrSecondsAndMegaBytes(t *testing.T) {
	work := t.TempDir()
	root := filepath.Join(work, "scan")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "a.jpg"), flagJPEG(t, 32, 32, 0, 90))
	writeFile(t, filepath.Join(root, "b.jpg"), flagJPEG(t, 40, 24, 1, 90))
	writeFile(t, filepath.Join(root, "c.txt"), []byte("x"))

	s := NewFileService(nil, nil, nil, filepath.Join(work, "trash"))
	off, err := s.EstimateDuplicateScan(root, false, 0, "", false, 90, false)
	if err != nil {
		t.Fatal(err)
	}
	if off.EtaOcrSeconds != 0 {
		t.Fatalf("ocr off eta=%d", off.EtaOcrSeconds)
	}
	on, err := s.EstimateDuplicateScan(root, false, 0, "", false, 90, true)
	if err != nil {
		t.Fatal(err)
	}
	if on.EtaOcrSeconds <= 0 {
		t.Fatalf("ocr on eta=%d", on.EtaOcrSeconds)
	}
	if on.ImageCount != 2 {
		t.Fatalf("imageCount=%d", on.ImageCount)
	}
	if on.MegaDownloadBytes <= off.MegaDownloadBytes {
		t.Fatalf("mega bytes off=%d on=%d", off.MegaDownloadBytes, on.MegaDownloadBytes)
	}
}

func TestOCRAvailableThin(t *testing.T) {
	s := NewFileService(nil, nil, nil, t.TempDir())
	// just ensure method is callable; value depends on host PATH
	_ = s.OCRAvailable()
}

func TestOCRScanCancelAborts(t *testing.T) {
	work := t.TempDir()
	root := filepath.Join(work, "scan")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "a.jpg"), flagJPEG(t, 48, 48, 0, 90))
	writeFile(t, filepath.Join(root, "b.jpg"), flagJPEG(t, 32, 32, 1, 90))

	s := NewFileService(nil, nil, nil, filepath.Join(work, "trash"))
	block := make(chan struct{})
	s.ocrFile = func(ctx context.Context, path string) (string, error) {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-block:
			return "never", nil
		}
	}
	done := make(chan domain.DupDonePayload, 1)
	s.onEvent = func(name string, data any) {
		if name == "dup:done" {
			done <- data.(domain.DupDonePayload)
		}
	}
	id := s.NewJobID()
	if err := s.StartDuplicateScan(id, root, false, 0, "", false, 90, true); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	if err := s.CancelJob(id); err != nil {
		t.Fatal(err)
	}
	got := waitDupDone(t, done)
	if got.Error == "" {
		t.Fatal("expected cancel error")
	}
	close(block)
}
