package filesystem

import (
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"unicode/utf16"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestIsCrossDevice(t *testing.T) {
	t.Parallel()
	if isCrossDevice(nil) {
		t.Fatal("nil is not cross-device")
	}
	if !isCrossDevice(syscall.EXDEV) {
		t.Fatal("EXDEV should be cross-device")
	}
	if isCrossDevice(syscall.EEXIST) {
		t.Fatal("EEXIST is not cross-device")
	}
	if !isCrossDevice(&os.LinkError{Op: "rename", Old: "a", New: "b", Err: syscall.EXDEV}) {
		t.Fatal("LinkError(EXDEV) should be cross-device")
	}
	if isCrossDevice(&os.LinkError{Op: "rename", Old: "a", New: "b", Err: syscall.EEXIST}) {
		t.Fatal("LinkError(EEXIST) is not cross-device")
	}

	winErr := syscall.Errno(17)
	got := isCrossDevice(winErr)
	if runtime.GOOS == "windows" {
		if !got {
			t.Fatal("Windows errno 17 (ERROR_NOT_SAME_DEVICE) should be cross-device")
		}
		return
	}
	if got {
		t.Fatal("POSIX errno 17 is EEXIST, not cross-device")
	}
}

func TestXDGTrashRoundTrip(t *testing.T) {
	t.Parallel()
	work := t.TempDir()
	tr := NewXDGTrash(filepath.Join(work, "Trash"))

	file := filepath.Join(work, "note.txt")
	writeFile(t, file, "hello")
	dir := filepath.Join(work, "sub")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "inner.txt"), "inner")

	if err := tr.Put([]string{file, dir}); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if _, err := os.Lstat(file); !os.IsNotExist(err) {
		t.Fatalf("file still present: %v", err)
	}
	items, err := tr.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("list=%d want 2", len(items))
	}

	n, err := tr.RestoreOrigins([]string{file, dir})
	if err != nil {
		t.Fatalf("RestoreOrigins: %v", err)
	}
	if n != 2 {
		t.Fatalf("restored %d want 2", n)
	}
	if b, err := os.ReadFile(file); err != nil || string(b) != "hello" {
		t.Fatalf("file not restored: %v %q", err, b)
	}
	if b, err := os.ReadFile(filepath.Join(dir, "inner.txt")); err != nil || string(b) != "inner" {
		t.Fatalf("dir not restored: %v %q", err, b)
	}
}

func TestXDGRestoreDoesNotClobber(t *testing.T) {
	t.Parallel()
	work := t.TempDir()
	tr := NewXDGTrash(filepath.Join(work, "Trash"))
	file := filepath.Join(work, "note.txt")
	writeFile(t, file, "old")
	if err := tr.Put([]string{file}); err != nil {
		t.Fatal(err)
	}
	writeFile(t, file, "new")
	n, err := tr.RestoreOrigins([]string{file})
	if n != 0 {
		t.Fatalf("restored %d, want 0", n)
	}
	if err != nil {
		t.Fatalf("occupied origin should be skipped, not error: %v", err)
	}
	if b, _ := os.ReadFile(file); string(b) != "new" {
		t.Fatalf("existing file was clobbered: %q", b)
	}
}

func TestXDGEmpty(t *testing.T) {
	t.Parallel()
	work := t.TempDir()
	tr := NewXDGTrash(filepath.Join(work, "Trash"))
	file := filepath.Join(work, "note.txt")
	writeFile(t, file, "hello")
	if err := tr.Put([]string{file}); err != nil {
		t.Fatal(err)
	}
	if err := tr.Empty(); err != nil {
		t.Fatal(err)
	}
	items, err := tr.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("empty left %d items", len(items))
	}
}

func TestXDGMissingPath(t *testing.T) {
	t.Parallel()
	work := t.TempDir()
	tr := NewXDGTrash(filepath.Join(work, "Trash"))
	if err := tr.Put([]string{filepath.Join(work, "nope.txt")}); err == nil {
		t.Fatal("expected an error for a missing path")
	}
}

func TestXDGCopyToFallback(t *testing.T) {
	t.Parallel()
	work := t.TempDir()
	src := filepath.Join(work, "src.txt")
	dest := filepath.Join(work, "dest.txt")
	writeFile(t, src, "copied")
	if err := copyTo(src, dest); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(dest); err != nil || string(b) != "copied" {
		t.Fatalf("copyTo: %v %q", err, b)
	}
}

func TestLeftoverTrashStillRestorable(t *testing.T) {
	t.Parallel()
	work := t.TempDir()
	tr := NewTrash(filepath.Join(work, "trash"))
	file := filepath.Join(work, "old.txt")
	writeFile(t, file, "legacy")
	id, err := tr.MoveToTrash([]string{file})
	if err != nil || id == "" {
		t.Fatalf("MoveToTrash: %v %q", err, id)
	}
	items, err := tr.List()
	if err != nil || len(items) != 1 {
		t.Fatalf("list leftover: %v %d", err, len(items))
	}
	if err := tr.Restore(id); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(file); err != nil || string(b) != "legacy" {
		t.Fatalf("leftover restore: %v %q", err, b)
	}
}

func TestParseRecycleIV2(t *testing.T) {
	t.Parallel()
	path := `C:\Users\erik\Documents\photo.jpg`
	u := utf16.Encode([]rune(path + "\x00"))
	buf := make([]byte, 28+len(u)*2)
	binary.LittleEndian.PutUint64(buf[0:8], 2)
	binary.LittleEndian.PutUint64(buf[8:16], 1234)
	binary.LittleEndian.PutUint32(buf[24:28], uint32(len(u)))
	for i, c := range u {
		binary.LittleEndian.PutUint16(buf[28+i*2:], c)
	}
	orig, size, err := parseRecycleI(buf)
	if err != nil {
		t.Fatal(err)
	}
	if orig != path {
		t.Fatalf("orig=%q", orig)
	}
	if size != 1234 {
		t.Fatalf("size=%d", size)
	}
}

func TestParseRecycleIV1(t *testing.T) {
	t.Parallel()
	path := `C:\Users\erik\Documents\old.doc`
	u := utf16.Encode([]rune(path + "\x00"))
	buf := make([]byte, 24+len(u)*2)
	binary.LittleEndian.PutUint64(buf[0:8], 1)
	binary.LittleEndian.PutUint64(buf[8:16], 99)
	for i, c := range u {
		binary.LittleEndian.PutUint16(buf[24+i*2:], c)
	}
	orig, size, err := parseRecycleI(buf)
	if err != nil {
		t.Fatal(err)
	}
	if orig != path {
		t.Fatalf("orig=%q", orig)
	}
	if size != 99 {
		t.Fatalf("size=%d", size)
	}
}

func TestIsTrashPath(t *testing.T) {
	t.Parallel()
	if !IsTrashPath("trash://") || !IsTrashPath("trash:") {
		t.Fatal("expected trash:// to match")
	}
	if IsTrashPath("/tmp") {
		t.Fatal("local path is not trash")
	}
}

func TestSkipDuplicateTrash(t *testing.T) {
	t.Parallel()
	nested := filepath.Join(t.TempDir(), ".Trash", "gone.txt")
	if !SkipDuplicateTrash(nested, nil) {
		t.Fatal("expected OS trash path to be skipped")
	}
	work := t.TempDir()
	isolated := filepath.Join(work, "Trash")
	if SkipDuplicateTrash(isolated, nil) {
		t.Fatal("isolated config Trash is not an OS trash path")
	}
	if !SkipDuplicateTrash(isolated, []string{isolated}) {
		t.Fatal("extraRoots should skip isolated Trash")
	}
}

func TestXDGRestoreStored(t *testing.T) {
	t.Parallel()
	work := t.TempDir()
	tr := NewXDGTrash(filepath.Join(work, "Trash"))
	file := filepath.Join(work, "note.txt")
	writeFile(t, file, "hello")
	if err := tr.Put([]string{file}); err != nil {
		t.Fatal(err)
	}
	items, err := tr.List()
	if err != nil || len(items) != 1 {
		t.Fatalf("list: %v %d", err, len(items))
	}
	if err := tr.Restore([]string{items[0].Stored}); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(file); err != nil || string(b) != "hello" {
		t.Fatalf("restore stored: %v %q", err, b)
	}
}

func TestListTrashEntriesOrigin(t *testing.T) {
	t.Parallel()
	work := t.TempDir()
	xdg := NewXDGTrash(filepath.Join(work, "Trash"))
	file := filepath.Join(work, "a.txt")
	writeFile(t, file, "x")
	if err := xdg.Put([]string{file}); err != nil {
		t.Fatal(err)
	}
	ents, err := ListTrashEntries(xdg, nil, work)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range ents {
		if e.Name == "a.txt" && e.Origin == file {
			found = true
		}
	}
	if !found {
		raw, _ := json.Marshal(ents)
		t.Fatalf("missing origin in listing: %s", raw)
	}
}
