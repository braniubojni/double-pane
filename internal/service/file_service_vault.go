package service

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/erikharutyunyan/double-pane/internal/domain"
	"github.com/erikharutyunyan/double-pane/internal/filesystem"
	"github.com/erikharutyunyan/double-pane/internal/vault"
)

// vaultIdleTick is how often the idle-lock clock is checked. Real time only
// — the vault package itself takes the "now" it's compared against, so
// tests exercise AdjustIdle directly with a stub clock instead of this ticker.
const vaultIdleTick = 30 * time.Second

// StartVaultIdleTicker runs for the process lifetime and locks idle vault
// sessions per Settings.VaultIdleLockMinutes (see vault.SetIdleLockMinutes,
// set from main.go/SettingsService when settings load or change). Call once
// from main.go after the app is constructed.
func StartVaultIdleTicker() {
	go func() {
		t := time.NewTicker(vaultIdleTick)
		defer t.Stop()
		for now := range t.C {
			vault.AdjustIdle(now)
		}
	}()
}

// DetectVault reports whether path is a folder or file (.dpenc) vault.
func (s *FileService) DetectVault(path string) (domain.VaultInfo, error) {
	info, ok := vault.Detect(path)
	if !ok {
		return domain.VaultInfo{}, fmt.Errorf("not a vault: %s", path)
	}
	return info, nil
}

// isInsideAnyVault reports whether path sits under a vault root (locked or
// unlocked) — used to refuse "Lock with password…" on vault internals.
func isInsideAnyVault(path string) bool {
	dir := filepath.Dir(path)
	for {
		if _, ok := vault.Detect(dir); ok {
			return true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return false
		}
		dir = parent
	}
}

// rejectVaultCreateTarget applies the spec's "too easy to brick" guards:
// home directory, filesystem root, a volume root, or already inside a vault.
func (s *FileService) rejectVaultCreateTarget(abs string) error {
	if isInsideAnyVault(abs) {
		return fmt.Errorf("already inside a vault")
	}
	if home, err := filesystem.HomeDir(); err == nil && abs == home {
		return fmt.Errorf("cannot lock the home directory")
	}
	if abs == string(filepath.Separator) {
		return fmt.Errorf("cannot lock the filesystem root")
	}
	if s.vols != nil {
		if vols, err := s.vols.List(); err == nil {
			for _, v := range vols {
				if v.Path == abs {
					return fmt.Errorf("cannot lock a volume root")
				}
			}
		}
	}
	return nil
}

// StartCreateVault locks folder in place under password. Progress/Done
// stream on the "vault:progress" event (domain.VaultJobEvent, Kind="create").
func (s *FileService) StartCreateVault(path, password, hint string) (string, error) {
	abs, err := filesystem.Resolve(path)
	if err != nil {
		return "", err
	}
	info, statErr := os.Stat(abs)
	if statErr != nil || !info.IsDir() {
		return "", fmt.Errorf("not a folder: %s", abs)
	}
	if _, ok := vault.Detect(abs); ok {
		return "", vault.ErrAlreadyVault
	}
	if err := s.rejectVaultCreateTarget(abs); err != nil {
		return "", err
	}

	jobID := s.NewJobID()
	ctx := s.jobCtx(jobID)
	go func() {
		defer func() { _ = s.FinishJob(jobID) }()
		err := vault.CreateFolderVault(ctx, abs, password, hint, func(e domain.VaultJobEvent) {
			e.JobID = jobID
			s.emit("vault:progress", e)
		})
		s.emit("vault:progress", domain.VaultJobEvent{JobID: jobID, Kind: "create", Path: abs, Err: errString(err), Done: true})
	}()
	return jobID, nil
}

// StartCreateFileVault locks a single file into a sibling <name>.dpenc.
func (s *FileService) StartCreateFileVault(path, password, hint string) (string, error) {
	abs, err := filesystem.Resolve(path)
	if err != nil {
		return "", err
	}
	info, statErr := os.Stat(abs)
	if statErr != nil || info.IsDir() {
		return "", fmt.Errorf("not a file: %s", abs)
	}
	if _, ok := vault.Detect(abs); ok {
		return "", vault.ErrAlreadyVault
	}
	if isInsideAnyVault(abs) {
		return "", fmt.Errorf("already inside a vault")
	}

	jobID := s.NewJobID()
	ctx := s.jobCtx(jobID)
	go func() {
		defer func() { _ = s.FinishJob(jobID) }()
		err := vault.CreateFileVault(ctx, abs, password, hint, func(e domain.VaultJobEvent) {
			e.JobID = jobID
			s.emit("vault:progress", e)
		})
		s.emit("vault:progress", domain.VaultJobEvent{JobID: jobID, Kind: "create-file", Path: abs, Err: errString(err), Done: true})
	}()
	return jobID, nil
}

// UnlockVault opens a process-memory session for path. password never
// touches disk, settings, or the emitted log line.
func (s *FileService) UnlockVault(path, password string) error {
	abs, err := filesystem.Resolve(path)
	if err != nil {
		return err
	}
	if err := vault.Unlock(abs, password); err != nil {
		return err
	}
	log.Printf("[vault] unlock ok")
	return nil
}

// LockVault drops the session for the vault that owns path.
func (s *FileService) LockVault(path string) error {
	abs, err := filesystem.Resolve(path)
	if err != nil {
		return err
	}
	return vault.Lock(abs)
}

// StartRemoveVault decrypts everything back to plaintext and deletes the
// vault metadata. password is required even if already unlocked (matches
// vault.RemoveProtection's self-contained contract).
func (s *FileService) StartRemoveVault(path, password string) (string, error) {
	abs, err := filesystem.Resolve(path)
	if err != nil {
		return "", err
	}
	if _, ok := vault.Detect(abs); !ok {
		return "", vault.ErrNotVault
	}

	jobID := s.NewJobID()
	ctx := s.jobCtx(jobID)
	go func() {
		defer func() { _ = s.FinishJob(jobID) }()
		err := vault.RemoveProtection(ctx, abs, password, func(e domain.VaultJobEvent) {
			e.JobID = jobID
			s.emit("vault:progress", e)
		})
		if err == nil {
			_ = vault.Lock(abs)
		}
		s.emit("vault:progress", domain.VaultJobEvent{JobID: jobID, Kind: "remove", Path: abs, Err: errString(err), Done: true})
	}()
	return jobID, nil
}

// ListUnlockedVaults lists every currently unlocked vault session, for the
// status-bar "Vault · N open" chip.
func (s *FileService) ListUnlockedVaults() []domain.VaultInfo {
	return vault.Sessions()
}

// NotifyPanePaths checks unlocked vault sessions against both current pane
// paths and locks any vault neither pane is inside anymore. Not yet called
// by the frontend (that wiring is a follow-up task) — exposed now so the
// auto-lock-on-navigate behavior only needs a call site added later.
func (s *FileService) NotifyPanePaths(leftPath, rightPath string) []string {
	locked := vault.AutoLockOutsidePaths(leftPath, rightPath)
	if len(locked) > 0 {
		s.emit("vault:locked", locked)
	}
	return locked
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
