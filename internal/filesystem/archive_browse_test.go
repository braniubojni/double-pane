package filesystem

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/erikharutyunyan/double-pane/internal/domain"
)

func TestSplitArchivePath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	zipPath := filepath.Join(root, "bundle.zip")
	if err := writeTestZip(zipPath, map[string]string{
		"hello.txt":    "hi",
		"docs/a.txt":   "a",
		"docs/b/c.txt": "c",
	}); err != nil {
		t.Fatal(err)
	}
	namedDir := filepath.Join(root, "folder.zip")
	if err := os.Mkdir(namedDir, 0o755); err != nil {
		t.Fatal(err)
	}

	a, inner, ok := SplitArchivePath(zipPath)
	if !ok || a != zipPath || inner != "" {
		t.Fatalf("archive file: got ok=%v a=%q inner=%q", ok, a, inner)
	}
	nested := filepath.Join(zipPath, "docs", "a.txt")
	a, inner, ok = SplitArchivePath(nested)
	if !ok || a != zipPath || inner != "docs/a.txt" {
		t.Fatalf("member: got ok=%v a=%q inner=%q", ok, a, inner)
	}
	if _, _, ok := SplitArchivePath(filepath.Join(namedDir, "x")); ok {
		t.Fatal("directory named *.zip must not be an archive")
	}
	if IsBrowsableArchiveName("notes.gz") {
		t.Fatal("single-file gz must not be browsable")
	}
	if !IsBrowsableArchiveName("pack.tar.gz") {
		t.Fatal("tar.gz should be browsable")
	}
}

func TestListArchiveDirNested(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	zipPath := filepath.Join(root, "bundle.zip")
	if err := writeTestZip(zipPath, map[string]string{
		"hello.txt":    "hi",
		"docs/a.txt":   "a",
		"docs/b/c.txt": "c",
	}); err != nil {
		t.Fatal(err)
	}
	ents, err := ListArchiveDir(zipPath, "", false)
	if err != nil {
		t.Fatal(err)
	}
	names := entryNames(ents)
	if !containsAll(names, "..", "hello.txt", "docs") {
		t.Fatalf("root listing: %v", names)
	}
	if ents[0].Name != ".." || ents[0].Path != root {
		t.Fatalf(".. at root: %+v want parent %s", ents[0], root)
	}
	docs, err := ListArchiveDir(zipPath, "docs", false)
	if err != nil {
		t.Fatal(err)
	}
	dnames := entryNames(docs)
	if !containsAll(dnames, "..", "a.txt", "b") {
		t.Fatalf("docs listing: %v", dnames)
	}
}

func TestListArchiveDirSkipsZipSlip(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	zipPath := filepath.Join(root, "slip.zip")
	if err := writeTestZip(zipPath, map[string]string{
		"ok.txt":    "ok",
		"../evil":   "nope",
		"docs/../x": "nope",
	}); err != nil {
		t.Fatal(err)
	}
	ents, err := ListArchiveDir(zipPath, "", false)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range ents {
		if e.Name == ".." {
			continue
		}
		if strings.Contains(e.Name, "..") || e.Name == "evil" || e.Name == "x" {
			t.Fatalf("zip-slip leaked: %+v", e)
		}
	}
}

func TestExtractMembersOneFile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	zipPath := filepath.Join(root, "bundle.zip")
	if err := writeTestZip(zipPath, map[string]string{
		"hello.txt":  "hello archive",
		"docs/a.txt": "a",
	}); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(root, "out")
	if err := ExtractMembers(context.Background(), zipPath, dest, []string{"hello.txt"}, ""); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dest, "hello.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello archive" {
		t.Fatalf("got %q", data)
	}
	if _, err := os.Stat(filepath.Join(dest, "docs")); err == nil {
		t.Fatal("should not extract unselected docs/")
	}
}

func TestArchiveTarGzList(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	src := filepath.Join(root, "f.txt")
	if err := os.WriteFile(src, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(root, "out.tar.gz")
	if err := Archive(context.Background(), []string{src}, out, "tar.gz", "", nil); err != nil {
		t.Fatal(err)
	}
	ents, err := ListArchiveDir(out, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if !containsAll(entryNames(ents), "f.txt") {
		t.Fatalf("tar.gz listing: %v", entryNames(ents))
	}
}

func TestReadArchiveTextFile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	zipPath := filepath.Join(root, "bundle.zip")
	if err := writeTestZip(zipPath, map[string]string{"n.txt": "note"}); err != nil {
		t.Fatal(err)
	}
	got, err := ReadArchiveTextFile(zipPath, "n.txt", "")
	if err != nil {
		t.Fatal(err)
	}
	if got != "note" {
		t.Fatalf("got %q", got)
	}
}

func writeTestZip(dest string, files map[string]string) error {
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	zw := zip.NewWriter(f)
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			_ = zw.Close()
			return err
		}
		if _, err := w.Write([]byte(body)); err != nil {
			_ = zw.Close()
			return err
		}
	}
	if err := zw.Close(); err != nil {
		return err
	}
	return f.Close()
}

func entryNames(ents []domain.FileEntry) []string {
	out := make([]string, len(ents))
	for i, e := range ents {
		out[i] = e.Name
	}
	return out
}

func containsAll(have []string, want ...string) bool {
	set := map[string]struct{}{}
	for _, h := range have {
		set[h] = struct{}{}
	}
	for _, w := range want {
		if _, ok := set[w]; !ok {
			return false
		}
	}
	return true
}
