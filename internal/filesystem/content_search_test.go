package filesystem

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/erikharutyunyan/double-pane/internal/domain"
)

func TestSearchContentFindsMatches(t *testing.T) {
	root := t.TempDir()
	_ = os.MkdirAll(filepath.Join(root, "src"), 0o755)
	_ = os.WriteFile(filepath.Join(root, "src", "a.go"), []byte("package main\nfunc Hello() {}\n"), 0o644)
	_ = os.WriteFile(filepath.Join(root, "src", "b.txt"), []byte("hello world\nHELLO again\n"), 0o644)
	_ = os.WriteFile(filepath.Join(root, "skip.bin"), []byte{0x00, 0x01, 0x02, 'x'}, 0o644)
	_ = os.MkdirAll(filepath.Join(root, "build"), 0o755)
	_ = os.WriteFile(filepath.Join(root, "build", "out.txt"), []byte("hello in build\n"), 0o644)

	var hits []domain.ContentSearchHit
	truncated, err := SearchContent(context.Background(), root, "hello", "", "build", false, false, 50, ContentSearchCallbacks{
		OnHit: func(h domain.ContentSearchHit) { hits = append(hits, h) },
	})
	if err != nil {
		t.Fatal(err)
	}
	if truncated {
		t.Fatal("unexpected truncated")
	}
	if len(hits) < 2 {
		t.Fatalf("expected >=2 hits, got %d %#v", len(hits), hits)
	}
	for _, h := range hits {
		if strings.Contains(h.RelPath, "build") {
			t.Fatalf("build should be excluded: %s", h.RelPath)
		}
		if strings.Contains(h.RelPath, "skip.bin") {
			t.Fatal("binary should be skipped")
		}
	}
}

func TestSearchContentCaseSensitive(t *testing.T) {
	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, "a.txt"), []byte("Hello\nhello\n"), 0o644)
	var hits []domain.ContentSearchHit
	_, err := SearchContent(context.Background(), root, "Hello", "", "", false, true, 50, ContentSearchCallbacks{
		OnHit: func(h domain.ContentSearchHit) { hits = append(hits, h) },
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].Line != 1 {
		t.Fatalf("want 1 hit on line 1, got %#v", hits)
	}
}

func TestSearchContentInclude(t *testing.T) {
	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, "a.ts"), []byte("findme\n"), 0o644)
	_ = os.WriteFile(filepath.Join(root, "a.go"), []byte("findme\n"), 0o644)
	var hits []domain.ContentSearchHit
	_, err := SearchContent(context.Background(), root, "findme", "*.ts", "", false, false, 50, ContentSearchCallbacks{
		OnHit: func(h domain.ContentSearchHit) { hits = append(hits, h) },
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].RelPath != "a.ts" {
		t.Fatalf("got %#v", hits)
	}
}

func TestSearchContentCancel(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 20; i++ {
		name := filepath.Join(root, "f"+strings.Repeat("x", i+1)+".txt")
		_ = os.WriteFile(name, []byte("needle\n"), 0o644)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := SearchContent(ctx, root, "needle", "", "", false, false, 50, ContentSearchCallbacks{})
	if err != nil {
		t.Fatal(err)
	}
}

func TestWindowLinePreviewAlignsMatchIndices(t *testing.T) {
	// Long line with match far past 240 bytes.
	prefix := strings.Repeat("a", 300)
	needle := "NEEDLE"
	suffix := strings.Repeat("b", 50)
	line := prefix + needle + suffix
	matchStart := len(prefix)
	matchEnd := matchStart + len(needle)

	preview, ms, me := windowLinePreview(line, matchStart, matchEnd, 240)
	if len(preview) > 240+2*len("…") {
		t.Fatalf("preview too long: %d", len(preview))
	}
	if ms < 0 || me > len(preview) || me < ms {
		t.Fatalf("bad indices ms=%d me=%d len=%d", ms, me, len(preview))
	}
	if preview[ms:me] != needle {
		t.Fatalf("got %q want %q (preview=%q)", preview[ms:me], needle, preview)
	}
	if !strings.HasPrefix(preview, "…") {
		t.Fatalf("expected leading ellipsis for windowed line, got %q", preview)
	}
}

func TestSearchContentLongLineMatchIndices(t *testing.T) {
	root := t.TempDir()
	line := strings.Repeat("x", 300) + "findme" + strings.Repeat("y", 20) + "\n"
	_ = os.WriteFile(filepath.Join(root, "long.txt"), []byte(line), 0o644)

	var hits []domain.ContentSearchHit
	_, err := SearchContent(context.Background(), root, "findme", "", "", false, true, 10, ContentSearchCallbacks{
		OnHit: func(h domain.ContentSearchHit) { hits = append(hits, h) },
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 {
		t.Fatalf("want 1 hit, got %#v", hits)
	}
	h := hits[0]
	if h.Column != 301 {
		t.Fatalf("Column should be full-line 1-based index, got %d", h.Column)
	}
	if h.MatchStart < 0 || h.MatchEnd > len(h.LineText) {
		t.Fatalf("indices out of LineText: %#v", h)
	}
	if h.LineText[h.MatchStart:h.MatchEnd] != "findme" {
		t.Fatalf("LineText[%d:%d]=%q want findme (LineText=%q)", h.MatchStart, h.MatchEnd, h.LineText[h.MatchStart:h.MatchEnd], h.LineText)
	}
}

func TestSearchContentPermissionDenied(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod semantics differ on windows")
	}
	root := t.TempDir()
	secret := filepath.Join(root, "secret")
	if err := os.MkdirAll(secret, 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(secret, "x.txt"), []byte("needle\n"), 0o644)
	if err := os.Chmod(secret, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(secret, 0o755) })

	var denied []string
	_, err := SearchContent(context.Background(), root, "needle", "", "", false, false, 50, ContentSearchCallbacks{
		OnDenied: func(path string, _ error) { denied = append(denied, path) },
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(denied) == 0 {
		t.Fatal("expected permission denied callback")
	}
}
