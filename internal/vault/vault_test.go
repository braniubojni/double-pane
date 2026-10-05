package vault

import (
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/erikharutyunyan/double-pane/internal/domain"
)

func randomBytes(n int, seed int64) []byte {
	b := make([]byte, n)
	rand.New(rand.NewSource(seed)).Read(b)
	return b
}

func hashOf(b []byte) string {
	sum := sha256.Sum256(b)
	return string(sum[:])
}

func readAllClose(rc io.ReadCloser) []byte {
	defer func() { _ = rc.Close() }()
	b, _ := io.ReadAll(rc)
	return b
}

// TestFolderVaultRoundTrip covers: lock a folder with a multi-chunk file, a
// nested dir, and an empty dir; raw os.ReadDir sees only ciphertext; unlock
// makes List report the original names/sizes; copy-out hashes match.
func TestFolderVaultRoundTrip(t *testing.T) {
	resetForTest()
	root := t.TempDir()

	big := randomBytes(int(2.5*1024*1024), 1) // 2.5 MiB → 3 chunks at 1 MiB
	if err := os.WriteFile(filepath.Join(root, "big.bin"), big, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	small := []byte("hello nested")
	if err := os.WriteFile(filepath.Join(root, "nested", "small.txt"), small, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "empty"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := CreateFolderVault(context.Background(), root, "correct horse battery", "", nil); err != nil {
		t.Fatalf("CreateFolderVault: %v", err)
	}

	// Raw disk view: only .dpvault, no plaintext siblings.
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != dpvaultDir {
		t.Fatalf("expected only %s on disk, got %v", dpvaultDir, entries)
	}
	objEntries, err := os.ReadDir(filepath.Join(root, dpvaultDir, objectsDir))
	if err != nil {
		t.Fatal(err)
	}
	if len(objEntries) != 3 { // big.bin, nested/small.txt, empty/ marker
		t.Fatalf("expected 3 objects, got %d", len(objEntries))
	}

	info, ok := Detect(root)
	if !ok || !info.Locked {
		t.Fatalf("Detect: %+v ok=%v", info, ok)
	}

	if err := Unlock(root, "correct horse battery"); err != nil {
		t.Fatalf("Unlock: %v", err)
	}
	if !SessionUnlocked(root) {
		t.Fatal("expected session unlocked")
	}

	top, err := List(root)
	if err != nil {
		t.Fatalf("List root: %v", err)
	}
	want := map[string]struct {
		isDir bool
		size  int64
	}{
		"big.bin": {false, int64(len(big))},
		"nested":  {true, 0},
		"empty":   {true, 0},
	}
	if len(top) != len(want) {
		t.Fatalf("List root = %+v", top)
	}
	for _, e := range top {
		w, ok := want[e.Name]
		if !ok || w.isDir != e.IsDir || (!e.IsDir && w.size != e.Size) {
			t.Fatalf("entry %+v mismatch want %+v", e, w)
		}
	}

	nested, err := List(filepath.Join(root, "nested"))
	if err != nil || len(nested) != 1 || nested[0].Name != "small.txt" || nested[0].Size != int64(len(small)) {
		t.Fatalf("List nested = %+v err=%v", nested, err)
	}

	// Copy vault → outside: streaming decrypt, hash equality.
	rc, meta, err := Open(root, "big.bin")
	if err != nil {
		t.Fatalf("Open big.bin: %v", err)
	}
	if meta.Size != int64(len(big)) {
		t.Fatalf("meta size = %d want %d", meta.Size, len(big))
	}
	got := readAllClose(rc)
	if hashOf(got) != hashOf(big) {
		t.Fatal("decrypted bytes hash mismatch")
	}

	_ = Lock(root)
	if SessionUnlocked(root) {
		t.Fatal("expected locked after Lock")
	}
}

// TestFileVaultRoundTrip covers the .dpenc single-file variant.
func TestFileVaultRoundTrip(t *testing.T) {
	resetForTest()
	dir := t.TempDir()
	path := filepath.Join(dir, "secret.txt")
	content := randomBytes(200*1024, 2)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := CreateFileVault(context.Background(), path, "another-strong-pw", "a hint", nil); err != nil {
		t.Fatalf("CreateFileVault: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("plaintext original should be gone")
	}
	dpenc := path + fileVaultExt
	if _, err := os.Stat(dpenc); err != nil {
		t.Fatalf("expected %s: %v", dpenc, err)
	}

	info, ok := Detect(dpenc)
	if !ok || !info.IsFile || info.Hint != "a hint" {
		t.Fatalf("Detect: %+v ok=%v", info, ok)
	}

	if err := Unlock(dpenc, "another-strong-pw"); err != nil {
		t.Fatalf("Unlock: %v", err)
	}

	if err := RemoveProtection(context.Background(), dpenc, "another-strong-pw", nil); err != nil {
		t.Fatalf("RemoveProtection: %v", err)
	}
	restored, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("restored file missing: %v", err)
	}
	if hashOf(restored) != hashOf(content) {
		t.Fatal("restored bytes hash mismatch")
	}
	if _, err := os.Stat(dpenc); !os.IsNotExist(err) {
		t.Fatal(".dpenc should be gone after RemoveProtection")
	}
}

func TestWrongPasswordSentinel(t *testing.T) {
	resetForTest()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := CreateFolderVault(context.Background(), root, "right-password", "", nil); err != nil {
		t.Fatal(err)
	}
	err := Unlock(root, "totally-wrong-password")
	if !errors.Is(err, ErrInvalidPassword) {
		t.Fatalf("got %v, want ErrInvalidPassword", err)
	}
}

// TestCrashMidCreateRetry: cancel after the first file is converted, then
// retry with the same password — must finish with no duplicate objects.
func TestCrashMidCreateRetry(t *testing.T) {
	resetForTest()
	root := t.TempDir()
	for _, name := range []string{"one.txt", "two.txt"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("content-"+name), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	err := CreateFolderVault(ctx, root, "resumable-pw", "", func(e domain.VaultJobEvent) {
		if e.FilesDone == 1 && !e.Done {
			cancel()
		}
	})
	if err == nil {
		t.Fatal("expected the job to be canceled")
	}

	if err := CreateFolderVault(context.Background(), root, "resumable-pw", "", nil); err != nil {
		t.Fatalf("retry CreateFolderVault: %v", err)
	}

	objEntries, err := os.ReadDir(filepath.Join(root, dpvaultDir, objectsDir))
	if err != nil {
		t.Fatal(err)
	}
	if len(objEntries) != 2 {
		t.Fatalf("expected 2 objects after retry, got %d: %v", len(objEntries), objEntries)
	}

	if err := Unlock(root, "resumable-pw"); err != nil {
		t.Fatalf("Unlock after retry: %v", err)
	}
	list, err := List(root)
	if err != nil || len(list) != 2 {
		t.Fatalf("List after retry = %+v err=%v", list, err)
	}
}

func TestAutoLockOutsidePaths(t *testing.T) {
	resetForTest()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := CreateFolderVault(context.Background(), root, "pane-test-pw", "", nil); err != nil {
		t.Fatal(err)
	}
	if err := Unlock(root, "pane-test-pw"); err != nil {
		t.Fatal(err)
	}

	// Still inside one pane path → stays unlocked.
	locked := AutoLockOutsidePaths(filepath.Join(root, "a.txt"), "/somewhere/else")
	if len(locked) != 0 || !SessionUnlocked(root) {
		t.Fatalf("expected still unlocked, locked=%v", locked)
	}

	// Neither pane path is inside the vault anymore → locked.
	locked = AutoLockOutsidePaths("/somewhere/else", "/another/place")
	if len(locked) != 1 || locked[0] != root {
		t.Fatalf("expected %s locked, got %v", root, locked)
	}
	if SessionUnlocked(root) {
		t.Fatal("expected session gone")
	}
}

func TestIdleLockWithStubClock(t *testing.T) {
	resetForTest()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := CreateFolderVault(context.Background(), root, "idle-test-pw", "", nil); err != nil {
		t.Fatal(err)
	}
	if err := Unlock(root, "idle-test-pw"); err != nil {
		t.Fatal(err)
	}

	SetIdleLockMinutes(1)
	start := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	touch(start)

	AdjustIdle(start.Add(30 * time.Second))
	if !SessionUnlocked(root) {
		t.Fatal("should still be unlocked at 30s idle")
	}

	AdjustIdle(start.Add(61 * time.Second))
	if SessionUnlocked(root) {
		t.Fatal("should be locked past the 1-minute idle timeout")
	}
}

func TestFreeSpacePreflightRefuses(t *testing.T) {
	resetForTest()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), randomBytes(1024, 3), 0o644); err != nil {
		t.Fatal(err)
	}

	old := diskUsage
	diskUsage = func(string) (domain.DiskUsage, error) {
		return domain.DiskUsage{Free: 100}, nil // far less than the file + margin
	}
	defer func() { diskUsage = old }()

	err := CreateFolderVault(context.Background(), root, "no-space-pw", "", nil)
	if !errors.Is(err, ErrInsufficientDiskSpace) {
		t.Fatalf("got %v, want ErrInsufficientDiskSpace", err)
	}
	if _, err := os.Stat(filepath.Join(root, dpvaultDir)); !os.IsNotExist(err) {
		t.Fatal(".dpvault should not have been created")
	}
}
