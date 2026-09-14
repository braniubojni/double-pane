package ocr

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestAvailableMissingBinary(t *testing.T) {
	prev := Binary
	t.Cleanup(func() { Binary = prev })
	Binary = "tesseract-not-installed-xyz"
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	if Available() {
		t.Fatal("expected Available=false when binary missing")
	}
}

func TestRecognizeFakeStdout(t *testing.T) {
	bin := writeFakeTesseract(t, `#!/bin/sh
echo "Hello World Screenshot Text"
`)
	prev := Binary
	t.Cleanup(func() { Binary = prev })
	Binary = bin

	img := filepath.Join(t.TempDir(), "a.png")
	if err := os.WriteFile(img, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	text, err := Recognize(context.Background(), img)
	if err != nil {
		t.Fatal(err)
	}
	toks := Tokens(text)
	if len(toks) < 3 {
		t.Fatalf("tokens=%v", toks)
	}
}

func TestClusterTwoScreenshotsSameText(t *testing.T) {
	text := "Login button submit form dashboard settings profile logout"
	items := []Item{
		{Path: "/a.png", Tokens: Tokens(text), Text: text},
		{Path: "/b.png", Tokens: Tokens(text), Text: text},
	}
	groups := Cluster(items, 90)
	if len(groups) != 1 || len(groups[0].Items) != 2 {
		t.Fatalf("groups=%+v", groups)
	}
	if groups[0].Snippet == "" {
		t.Fatal("expected snippet")
	}
}

func TestClusterEmptyTokensSkipped(t *testing.T) {
	items := []Item{
		{Path: "/a.png", Tokens: nil, Text: ""},
		{Path: "/b.png", Tokens: nil, Text: ""},
	}
	if g := Cluster(items, 90); len(g) != 0 {
		t.Fatalf("expected no groups, got %+v", g)
	}
}

func TestRecognizeTimeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell sleep script")
	}
	bin := writeFakeTesseract(t, `#!/bin/sh
exec sleep 60
`)
	prev := Binary
	t.Cleanup(func() { Binary = prev })
	Binary = bin

	img := filepath.Join(t.TempDir(), "a.png")
	if err := os.WriteFile(img, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_, err := Recognize(ctx, img)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if time.Since(start) > 5*time.Second {
		t.Fatalf("cancel too slow: %v", time.Since(start))
	}
}

func writeFakeTesseract(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "tesseract")
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}
