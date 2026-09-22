package service

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/erikharutyunyan/double-pane/internal/domain"
)

func flagJPEG(t *testing.T, w, h, variant, quality int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var c color.RGBA
			if variant == 0 {
				c = color.RGBA{
					uint8(40 + x*180/w),
					uint8(80 + y*140/h),
					uint8(200 - y*90/h),
					255,
				}
				if (x-w/3)*(x-w/3)+(y-h/3)*(y-h/3) < (w*w)/16 {
					c = color.RGBA{220, 60, 40, 255}
				}
			} else {
				if (x/(w/8+1)+y/(h/8+1))%2 == 0 {
					c = color.RGBA{10, 10, 10, 255}
				} else {
					c = color.RGBA{240, 240, 40, 255}
				}
			}
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func writeFile(t *testing.T, path string, b []byte) {
	t.Helper()
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestVisualScanResizedSamePhoto(t *testing.T) {
	work := t.TempDir()
	root := filepath.Join(work, "scan")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "big.jpg"), flagJPEG(t, 256, 256, 0, 90))
	writeFile(t, filepath.Join(root, "small.jpg"), flagJPEG(t, 80, 80, 0, 70))

	s := NewFileService(nil, nil, nil, filepath.Join(work, "trash"))
	done := make(chan domain.DupDonePayload, 1)
	s.onEvent = func(name string, data any) {
		if name == "dup:done" {
			done <- data.(domain.DupDonePayload)
		}
	}
	if err := s.StartDuplicateScan(s.NewJobID(), root, false, 0, "", true, 90, false); err != nil {
		t.Fatal(err)
	}
	got := waitDupDone(t, done)
	if got.Error != "" {
		t.Fatal(got.Error)
	}
	if len(got.Groups) != 1 || got.Groups[0].Kind != "visual" {
		t.Fatalf("want 1 visual group, got %+v", got.Groups)
	}
	if got.Groups[0].Similarity < 90 {
		t.Fatalf("similarity %d", got.Groups[0].Similarity)
	}
	if len(got.Groups[0].Files) != 2 {
		t.Fatalf("files %+v", got.Groups[0].Files)
	}
}

func TestVisualScanDifferentPhotosEmpty(t *testing.T) {
	work := t.TempDir()
	root := filepath.Join(work, "scan")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "a.jpg"), flagJPEG(t, 128, 128, 0, 90))
	writeFile(t, filepath.Join(root, "b.jpg"), flagJPEG(t, 96, 96, 1, 90))

	s := NewFileService(nil, nil, nil, filepath.Join(work, "trash"))
	done := make(chan domain.DupDonePayload, 1)
	s.onEvent = func(name string, data any) {
		if name == "dup:done" {
			done <- data.(domain.DupDonePayload)
		}
	}
	if err := s.StartDuplicateScan(s.NewJobID(), root, false, 0, "", true, 90, false); err != nil {
		t.Fatal(err)
	}
	got := waitDupDone(t, done)
	if got.Error != "" {
		t.Fatal(got.Error)
	}
	if len(got.Groups) != 0 {
		t.Fatalf("unrelated photos must not group: %+v", got.Groups)
	}
}

func TestVisualScanExactPairNotInVisual(t *testing.T) {
	work := t.TempDir()
	root := filepath.Join(work, "scan")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	same := flagJPEG(t, 64, 64, 0, 90)
	writeFile(t, filepath.Join(root, "a.jpg"), same)
	writeFile(t, filepath.Join(root, "b.jpg"), same)

	s := NewFileService(nil, nil, nil, filepath.Join(work, "trash"))
	done := make(chan domain.DupDonePayload, 1)
	s.onEvent = func(name string, data any) {
		if name == "dup:done" {
			done <- data.(domain.DupDonePayload)
		}
	}
	if err := s.StartDuplicateScan(s.NewJobID(), root, false, 0, "", true, 90, false); err != nil {
		t.Fatal(err)
	}
	got := waitDupDone(t, done)
	if got.Error != "" {
		t.Fatal(got.Error)
	}
	if len(got.Groups) != 1 || got.Groups[0].Kind != "exact" {
		t.Fatalf("want only exact, got %+v", got.Groups)
	}
	if len(got.Groups[0].Files) != 2 {
		t.Fatalf("files %+v", got.Groups[0].Files)
	}
}

func TestEstimateMegaBytesIncludesAllImagesWhenSimilar(t *testing.T) {
	work := t.TempDir()
	root := filepath.Join(work, "scan")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	bin := []byte("size-tie-xxxx")
	writeFile(t, filepath.Join(root, "a.bin"), bin)
	writeFile(t, filepath.Join(root, "b.bin"), bin)
	jpg := flagJPEG(t, 32, 32, 0, 90)
	writeFile(t, filepath.Join(root, "solo.jpg"), jpg)
	other := flagJPEG(t, 40, 24, 1, 90)
	writeFile(t, filepath.Join(root, "other.jpg"), other)

	s := NewFileService(nil, nil, nil, filepath.Join(work, "trash"))
	off, err := s.EstimateDuplicateScan(root, false, 0, "", false, 90, false)
	if err != nil {
		t.Fatal(err)
	}
	on, err := s.EstimateDuplicateScan(root, false, 0, "", true, 90, false)
	if err != nil {
		t.Fatal(err)
	}
	if off.FileCount != on.FileCount || off.ByteCount != on.ByteCount {
		t.Fatalf("V1 totals changed: off=%+v on=%+v", off, on)
	}
	tie := int64(len(bin) * 2)
	if off.MegaDownloadBytes != tie {
		t.Fatalf("similar off mega bytes=%d want size-ties %d", off.MegaDownloadBytes, tie)
	}
	wantOn := tie + int64(len(jpg)+len(other))
	if on.MegaDownloadBytes != wantOn {
		t.Fatalf("similar on mega bytes=%d want ties+images %d (jpg=%d other=%d)", on.MegaDownloadBytes, wantOn, len(jpg), len(other))
	}
	if on.ImageCount != 2 {
		t.Fatalf("imageCount=%d", on.ImageCount)
	}
	if on.EtaVisualSeconds < 0 {
		t.Fatalf("etaVisual %d", on.EtaVisualSeconds)
	}
}

func TestVisualScanCancelStopsHash(t *testing.T) {
	work := t.TempDir()
	root := filepath.Join(work, "scan")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "a.jpg"), flagJPEG(t, 48, 48, 0, 90))
	writeFile(t, filepath.Join(root, "b.jpg"), flagJPEG(t, 32, 32, 1, 90))

	s := NewFileService(nil, nil, nil, filepath.Join(work, "trash"))
	started := make(chan struct{})
	s.dhashFile = func(ctx context.Context, path string) (uint64, error) {
		select {
		case <-started:
		default:
			close(started)
		}
		<-ctx.Done()
		return 0, ctx.Err()
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
	if err := s.StartDuplicateScan(id, root, false, 0, "", true, 90, false); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("visual hash did not start")
	}
	if err := s.CancelJob(id); err != nil {
		t.Fatal(err)
	}
	got := waitDupDone(t, done)
	if got.Error == "" {
		t.Fatal("expected cancel error")
	}
	if !fatal {
		t.Fatal("expected fatal dup:error")
	}
}

func TestSimilarOffDoesNotVisualGroup(t *testing.T) {
	work := t.TempDir()
	root := filepath.Join(work, "scan")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "big.jpg"), flagJPEG(t, 256, 256, 0, 90))
	writeFile(t, filepath.Join(root, "small.jpg"), flagJPEG(t, 80, 80, 0, 70))

	s := NewFileService(nil, nil, nil, filepath.Join(work, "trash"))
	done := make(chan domain.DupDonePayload, 1)
	s.onEvent = func(name string, data any) {
		if name == "dup:done" {
			done <- data.(domain.DupDonePayload)
		}
	}
	if err := s.StartDuplicateScan(s.NewJobID(), root, false, 0, "", false, 90, false); err != nil {
		t.Fatal(err)
	}
	got := waitDupDone(t, done)
	if got.Error != "" {
		t.Fatal(got.Error)
	}
	if len(got.Groups) != 0 {
		t.Fatalf("V1 path must not visual-group: %+v", got.Groups)
	}
}
