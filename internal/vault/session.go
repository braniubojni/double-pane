package vault

import (
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/erikharutyunyan/double-pane/internal/domain"
)

// session is process-memory-only state for one unlocked vault. The DEK is
// wiped on Lock/LockAll and never touches disk, settings, or logs.
type session struct {
	dek         []byte
	vaultID     [idSize]byte
	isFile      bool
	hint        string
	version     int
	unlockedAt  time.Time
	inFlight    int
	pendingLock bool
}

var (
	mu           sync.Mutex
	sessions     = map[string]*session{} // key: abs vault root (folder or .dpenc file)
	idleMinutes  int
	lastActivity time.Time
)

// resetForTest clears all package-level state. Same-package tests only.
func resetForTest() {
	mu.Lock()
	defer mu.Unlock()
	for _, s := range sessions {
		Wipe(s.dek)
	}
	sessions = map[string]*session{}
	idleMinutes = 0
	lastActivity = time.Time{}
}

func isUnder(path, root string) bool {
	if path == root {
		return true
	}
	return strings.HasPrefix(path, root+string(filepath.Separator))
}

// RootFor returns the unlocked vault root that path is inside (or equals),
// if any.
func RootFor(path string) (string, bool) {
	mu.Lock()
	defer mu.Unlock()
	for root := range sessions {
		if isUnder(path, root) {
			return root, true
		}
	}
	return "", false
}

// SessionUnlocked reports whether path is inside (or equal to) a currently
// unlocked vault session.
func SessionUnlocked(path string) bool {
	_, ok := RootFor(path)
	return ok
}

// Sessions lists every currently unlocked vault as a domain.VaultInfo row
// (Locked is always false here — see ListUnlockedVaults).
func Sessions() []domain.VaultInfo {
	mu.Lock()
	defer mu.Unlock()
	out := make([]domain.VaultInfo, 0, len(sessions))
	for root, s := range sessions {
		out = append(out, domain.VaultInfo{Root: root, Locked: false, IsFile: s.isFile, Hint: s.hint, Version: s.version})
	}
	return out
}

// beginJob marks the session for root as holding one more in-flight vault
// job (an encrypt/decrypt in progress). Returns the DEK to use and a release
// func; the DEK must not be read after release is called. ok is false when
// the vault is not (or no longer) unlocked.
func beginJob(root string) (dek []byte, release func(), ok bool) {
	mu.Lock()
	defer mu.Unlock()
	s, exists := sessions[root]
	if !exists {
		return nil, func() {}, false
	}
	s.inFlight++
	return s.dek, func() { endJob(root) }, true
}

func endJob(root string) {
	mu.Lock()
	s, exists := sessions[root]
	if !exists {
		mu.Unlock()
		return
	}
	s.inFlight--
	pending := s.pendingLock && s.inFlight <= 0
	mu.Unlock()
	if pending {
		lockRoot(root)
	}
}

// lockRoot drops the session and wipes its DEK, unless a job is still in
// flight — in which case the lock is deferred until that job releases.
func lockRoot(root string) {
	mu.Lock()
	s, exists := sessions[root]
	if !exists {
		mu.Unlock()
		return
	}
	if s.inFlight > 0 {
		s.pendingLock = true
		mu.Unlock()
		return
	}
	delete(sessions, root)
	mu.Unlock()
	Wipe(s.dek)
}

// Lock drops the session for the vault that owns path (root or a
// descendant), if any.
func Lock(path string) error {
	root, ok := RootFor(path)
	if !ok {
		return nil
	}
	lockRoot(root)
	return nil
}

// LockAll drops every unlocked session (app shutdown, idle timeout).
func LockAll() {
	mu.Lock()
	roots := make([]string, 0, len(sessions))
	for root := range sessions {
		roots = append(roots, root)
	}
	mu.Unlock()
	for _, root := range roots {
		lockRoot(root)
	}
}

// AutoLockOutsidePaths locks every unlocked vault session whose root is not
// a prefix of (or equal to) any of the given paths — e.g. both pane paths —
// and returns the roots that were locked.
func AutoLockOutsidePaths(paths ...string) []string {
	mu.Lock()
	var toLock []string
	for root := range sessions {
		inUse := false
		for _, p := range paths {
			if p != "" && isUnder(p, root) {
				inUse = true
				break
			}
		}
		if !inUse {
			toLock = append(toLock, root)
		}
	}
	mu.Unlock()
	for _, root := range toLock {
		lockRoot(root)
	}
	return toLock
}

// SetIdleLockMinutes configures the idle auto-lock timeout; 0 disables it.
func SetIdleLockMinutes(m int) {
	mu.Lock()
	idleMinutes = m
	mu.Unlock()
}

// Touch records vault activity (list/copy/move/unlock/…), resetting the
// idle-lock clock.
func Touch() { touch(time.Now()) }

func touch(now time.Time) {
	mu.Lock()
	lastActivity = now
	mu.Unlock()
}

// AdjustIdle locks every session if idle time (now - last activity) has
// reached the configured idle-lock minutes. Intended to be called from a
// ticker; tests pass a synthetic now instead of sleeping.
func AdjustIdle(now time.Time) {
	mu.Lock()
	m := idleMinutes
	last := lastActivity
	hasSessions := len(sessions) > 0
	mu.Unlock()
	if m <= 0 || last.IsZero() || !hasSessions {
		return
	}
	if now.Sub(last) >= time.Duration(m)*time.Minute {
		LockAll()
	}
}
