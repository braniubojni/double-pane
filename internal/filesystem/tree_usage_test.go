package filesystem

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/erikharutyunyan/double-pane/internal/domain"
)

func writeBytes(t *testing.T, path string, n int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, bytes.Repeat([]byte("x"), n), 0o644); err != nil {
		t.Fatal(err)
	}
}

func kidNamed(n *domain.UsageNode, name string) *domain.UsageNode {
	if n == nil {
		return nil
	}
	for i := range n.Kids {
		if n.Kids[i].Name == name {
			return &n.Kids[i]
		}
	}
	return nil
}

func TestTreeUsageSumsFilesNotDirMetadata(t *testing.T) {
	root := t.TempDir()
	writeBytes(t, filepath.Join(root, "a.txt"), 100)
	writeBytes(t, filepath.Join(root, "sub", "b.txt"), 40)
	dirInfo, err := os.Lstat(filepath.Join(root, "sub"))
	if err != nil {
		t.Fatal(err)
	}

	tree, files, _, err := TreeUsage(context.Background(), root, 4, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if tree.Size != 140 {
		t.Fatalf("total %d, want 140 (dir metadata %d must stay out)", tree.Size, dirInfo.Size())
	}
	if files != 2 {
		t.Fatalf("files %d, want 2", files)
	}
	sub := kidNamed(tree, "sub")
	if sub == nil || sub.Size != 40 {
		t.Fatalf("sub = %#v", sub)
	}
}

func TestTreeUsageSkipsSymlinks(t *testing.T) {
	root := t.TempDir()
	writeBytes(t, filepath.Join(root, "big"), 5000)
	if err := os.Symlink(filepath.Join(root, "big"), filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	linked := filepath.Join(root, "linked")
	writeBytes(t, filepath.Join(linked, "inside"), 30)
	if err := os.Symlink(linked, filepath.Join(root, "dirlink")); err != nil {
		t.Fatal(err)
	}

	tree, _, _, err := TreeUsage(context.Background(), root, 4, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if tree.Size != 5030 {
		t.Fatalf("total %d, want 5030 (symlink targets must not be added again)", tree.Size)
	}
	if kidNamed(tree, "link") != nil || kidNamed(tree, "dirlink") != nil {
		t.Fatalf("symlink children should be omitted: %#v", tree.Kids)
	}
}

func TestTreeUsageMaxDepthKeepsBytesWithoutKids(t *testing.T) {
	root := t.TempDir()
	writeBytes(t, filepath.Join(root, "a", "deep", "f.txt"), 80)

	tree, _, _, err := TreeUsage(context.Background(), root, 1, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	a := kidNamed(tree, "a")
	if a == nil {
		t.Fatal("missing a")
	}
	if a.Size != 80 || len(a.Kids) != 0 {
		t.Fatalf("leaf a = size %d kids %d", a.Size, len(a.Kids))
	}
}

func TestTreeUsageIncludesDotfile(t *testing.T) {
	root := t.TempDir()
	writeBytes(t, filepath.Join(root, ".hidden"), 7)

	tree, _, _, err := TreeUsage(context.Background(), root, 4, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	dot := kidNamed(tree, ".hidden")
	if dot == nil || dot.Size != 7 {
		t.Fatalf(".hidden = %#v", dot)
	}
}

func TestTreeUsageCollapsesUnderOnePercent(t *testing.T) {
	root := t.TempDir()
	// total = 10000. 100 is exactly 1% and stays. 10 is under 1%.
	writeBytes(t, filepath.Join(root, "big"), 9800)
	writeBytes(t, filepath.Join(root, "edge"), 100)
	writeBytes(t, filepath.Join(root, "small-a"), 50)
	writeBytes(t, filepath.Join(root, "small-b"), 50)

	tree, _, _, err := TreeUsage(context.Background(), root, 4, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if tree.Size != 10000 {
		t.Fatalf("total %d", tree.Size)
	}
	if kidNamed(tree, "small-a") != nil || kidNamed(tree, "small-b") != nil {
		t.Fatalf("small files should collapse: %#v", tree.Kids)
	}
	if kidNamed(tree, "edge") == nil || kidNamed(tree, "big") == nil {
		t.Fatalf("files at or above 1%% should stay: %#v", tree.Kids)
	}
	other := kidNamed(tree, "Other")
	if other == nil || other.Size != 100 || other.Path != "" || other.IsDir {
		t.Fatalf("Other = %#v", other)
	}
}

func TestTreeUsageDeniedSiblingStillCounts(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod semantics differ on windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root bypasses permission bits")
	}
	root := t.TempDir()
	writeBytes(t, filepath.Join(root, "ok.txt"), 25)
	secret := filepath.Join(root, "secret")
	writeBytes(t, filepath.Join(secret, "x.txt"), 10)
	if err := os.Chmod(secret, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(secret, 0o755) })

	tree, _, denied, err := TreeUsage(context.Background(), root, 4, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if tree.Size != 25 {
		t.Fatalf("readable sibling should sum to 25, got %d", tree.Size)
	}
	found := false
	for _, p := range denied {
		if p == secret || strings.HasSuffix(p, "secret") {
			found = true
		}
	}
	if !found {
		t.Fatalf("denied = %v, want secret", denied)
	}
}

func TestTreeUsageCancelOmitsTree(t *testing.T) {
	root := t.TempDir()
	writeBytes(t, filepath.Join(root, "a"), 10)
	writeBytes(t, filepath.Join(root, "b"), 10)
	ctx, cancel := context.WithCancel(context.Background())
	tree, _, _, err := TreeUsage(ctx, root, 4, nil, func(string, int, int64) {
		cancel()
	})
	if err == nil || !strings.Contains(err.Error(), context.Canceled.Error()) {
		t.Fatalf("err = %v", err)
	}
	if tree != nil {
		t.Fatal("canceled walk should omit the tree")
	}
}
