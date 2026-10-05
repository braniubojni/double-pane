package service

import (
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/erikharutyunyan/double-pane/internal/domain"
	"github.com/erikharutyunyan/double-pane/internal/vault"
)

// waitVaultDone blocks for the terminal "vault:progress" event (Done=true)
// for jobID, from a FileService whose onEvent has been wired by the caller.
func waitVaultDone(t *testing.T, events <-chan domain.VaultJobEvent, jobID string) domain.VaultJobEvent {
	t.Helper()
	for {
		select {
		case e := <-events:
			if e.JobID == jobID && e.Done {
				return e
			}
		case <-time.After(10 * time.Second):
			t.Fatalf("timeout waiting for vault job %s to finish", jobID)
		}
	}
}

func vaultEventChan(s *FileService) <-chan domain.VaultJobEvent {
	ch := make(chan domain.VaultJobEvent, 64)
	s.onEvent = func(name string, data any) {
		if name == "vault:progress" {
			ch <- data.(domain.VaultJobEvent)
		}
	}
	return ch
}

func hashBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return string(sum[:])
}

func TestFileServiceVaultLifecycle(t *testing.T) {
	work := t.TempDir()
	root := filepath.Join(work, "myvault")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	content := []byte("some file contents for the vault round trip")
	if err := os.WriteFile(filepath.Join(root, "note.txt"), content, 0o644); err != nil {
		t.Fatal(err)
	}

	s := NewFileService(nil, nil, nil, filepath.Join(work, "trash"))
	events := vaultEventChan(s)
	defer vault.LockAll()

	jobID, err := s.StartCreateVault(root, "very-secret-pw", "a hint")
	if err != nil {
		t.Fatalf("StartCreateVault: %v", err)
	}
	if done := waitVaultDone(t, events, jobID); done.Err != "" {
		t.Fatalf("create job failed: %s", done.Err)
	}

	// Parent listing shows the vault row, locked, no plaintext siblings.
	parentEntries, err := s.ListDir(work, true)
	if err != nil {
		t.Fatalf("ListDir parent: %v", err)
	}
	var row *domain.FileEntry
	for i := range parentEntries {
		if parentEntries[i].Name == "myvault" {
			row = &parentEntries[i]
		}
	}
	if row == nil || !row.IsVault || !row.VaultLocked {
		t.Fatalf("expected locked vault row, got %+v", row)
	}

	// Listing straight into a locked vault must not leak ciphertext.
	if _, err := s.ListDir(root, true); err == nil {
		t.Fatal("expected an error listing a locked vault directly")
	}

	info, err := s.DetectVault(root)
	if err != nil || info.Root != root || !info.Locked || info.Hint != "a hint" {
		t.Fatalf("DetectVault = %+v err=%v", info, err)
	}

	if err := s.UnlockVault(root, "very-secret-pw"); err != nil {
		t.Fatalf("UnlockVault: %v", err)
	}

	unlocked := s.ListUnlockedVaults()
	if len(unlocked) != 1 || unlocked[0].Root != root {
		t.Fatalf("ListUnlockedVaults = %+v", unlocked)
	}

	entries, err := s.ListDir(root, true)
	if err != nil {
		t.Fatalf("ListDir unlocked vault: %v", err)
	}
	found := false
	for _, e := range entries {
		if e.Name == "note.txt" {
			found = true
			if e.Size != int64(len(content)) {
				t.Fatalf("size mismatch: %d want %d", e.Size, len(content))
			}
		}
		if e.Name == ".dpvault" {
			t.Fatal(".dpvault must never be listed")
		}
	}
	if !found {
		t.Fatal("note.txt missing from unlocked vault listing")
	}

	// Mkdir + rename inside the vault.
	if _, err := s.Mkdir(root, "sub"); err != nil {
		t.Fatalf("Mkdir in vault: %v", err)
	}
	if _, err := s.Rename(filepath.Join(root, "note.txt"), "renamed.txt"); err != nil {
		t.Fatalf("Rename in vault: %v", err)
	}
	entries, err = s.ListDir(root, true)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, e := range entries {
		names[e.Name] = true
	}
	if !names["sub"] || !names["renamed.txt"] || names["note.txt"] {
		t.Fatalf("post rename/mkdir listing = %v", names)
	}

	// Copy out of the vault: hash equality.
	outDir := filepath.Join(work, "out")
	if err := os.Mkdir(outDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := s.Copy("", []string{filepath.Join(root, "renamed.txt")}, outDir); err != nil {
		t.Fatalf("Copy out of vault: %v", err)
	}
	gotBytes, err := os.ReadFile(filepath.Join(outDir, "renamed.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if hashBytes(gotBytes) != hashBytes(content) {
		t.Fatal("copied-out file hash mismatch")
	}

	// Copy into the vault.
	if err := os.WriteFile(filepath.Join(outDir, "incoming.txt"), []byte("incoming"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.Copy("", []string{filepath.Join(outDir, "incoming.txt")}, root); err != nil {
		t.Fatalf("Copy into vault: %v", err)
	}
	entries, _ = s.ListDir(root, true)
	names = map[string]bool{}
	for _, e := range entries {
		names[e.Name] = true
	}
	if !names["incoming.txt"] {
		t.Fatalf("incoming.txt missing after copy-in: %v", names)
	}

	// Delete inside the vault is a hard delete (no OS trash).
	if _, err := s.Delete([]string{filepath.Join(root, "incoming.txt")}); err != nil {
		t.Fatalf("Delete in vault: %v", err)
	}
	entries, _ = s.ListDir(root, true)
	for _, e := range entries {
		if e.Name == "incoming.txt" {
			t.Fatal("incoming.txt should be gone")
		}
	}

	if err := s.LockVault(root); err != nil {
		t.Fatalf("LockVault: %v", err)
	}
	if len(s.ListUnlockedVaults()) != 0 {
		t.Fatal("expected no unlocked vaults after LockVault")
	}
	info, _ = s.DetectVault(root)
	if !info.Locked {
		t.Fatal("expected locked after LockVault")
	}
}

func TestFileServiceCreateFileVaultAndRemove(t *testing.T) {
	work := t.TempDir()
	path := filepath.Join(work, "secret.bin")
	content := []byte("bytes to protect")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}

	s := NewFileService(nil, nil, nil, filepath.Join(work, "trash"))
	events := vaultEventChan(s)
	defer vault.LockAll()

	jobID, err := s.StartCreateFileVault(path, "file-vault-pw", "")
	if err != nil {
		t.Fatalf("StartCreateFileVault: %v", err)
	}
	if done := waitVaultDone(t, events, jobID); done.Err != "" {
		t.Fatalf("create-file job failed: %s", done.Err)
	}

	dpenc := path + ".dpenc"
	if _, err := os.Stat(dpenc); err != nil {
		t.Fatalf("expected %s: %v", dpenc, err)
	}
	info, err := s.DetectVault(dpenc)
	if err != nil || !info.IsFile || !info.Locked {
		t.Fatalf("DetectVault = %+v err=%v", info, err)
	}

	if err := s.UnlockVault(dpenc, "file-vault-pw"); err != nil {
		t.Fatalf("UnlockVault: %v", err)
	}

	removeJobID, err := s.StartRemoveVault(dpenc, "file-vault-pw")
	if err != nil {
		t.Fatalf("StartRemoveVault: %v", err)
	}
	if done := waitVaultDone(t, events, removeJobID); done.Err != "" {
		t.Fatalf("remove job failed: %s", done.Err)
	}

	restored, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("restored file missing: %v", err)
	}
	if hashBytes(restored) != hashBytes(content) {
		t.Fatal("restored bytes hash mismatch")
	}
	if _, err := os.Stat(dpenc); !os.IsNotExist(err) {
		t.Fatal(".dpenc should be gone after remove")
	}
}
