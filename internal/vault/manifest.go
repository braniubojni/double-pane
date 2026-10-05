package vault

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/erikharutyunyan/double-pane/internal/domain"
)

func writeManifestFile(path string, man Manifest) error {
	data, err := json.MarshalIndent(man, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func readManifestFile(path string) (Manifest, error) {
	var man Manifest
	data, err := os.ReadFile(path)
	if err != nil {
		return man, err
	}
	if err := json.Unmarshal(data, &man); err != nil {
		return man, fmt.Errorf("vault: corrupt manifest: %w", err)
	}
	return man, nil
}

// writeFileVaultHeader writes the fixed 512-byte length-prefixed JSON header
// used by single-file (.dpenc) vaults, ahead of the object's own magic.
func writeFileVaultHeader(w io.Writer, man Manifest) error {
	data, err := json.Marshal(man)
	if err != nil {
		return err
	}
	if len(data) > fileHeaderSize-4 {
		return fmt.Errorf("vault: manifest too large for file header")
	}
	buf := make([]byte, fileHeaderSize)
	binary.LittleEndian.PutUint32(buf[:4], uint32(len(data)))
	copy(buf[4:], data)
	_, err = w.Write(buf)
	return err
}

// readFileVaultHeader reads and validates a .dpenc file's header, confirming
// the object magic immediately follows it (a corrupt/foreign file is simply
// "not a vault", never an error to the caller).
func readFileVaultHeader(path string) (Manifest, bool) {
	var man Manifest
	f, err := os.Open(path)
	if err != nil {
		return man, false
	}
	defer func() { _ = f.Close() }()
	hdr := make([]byte, fileHeaderSize)
	if _, err := io.ReadFull(f, hdr); err != nil {
		return man, false
	}
	l := binary.LittleEndian.Uint32(hdr[:4])
	if int(l) > fileHeaderSize-4 {
		return man, false
	}
	if json.Unmarshal(hdr[4:4+l], &man) != nil {
		return man, false
	}
	m := make([]byte, len(magic))
	if _, err := io.ReadFull(f, m); err != nil || string(m) != magic {
		return man, false
	}
	return man, true
}

// Detect reports whether path is a folder or file vault (locked or
// unlocked). It never errors: any read/parse failure just means "not a
// vault", matching the entry-menu enable/disable checks in the spec.
func Detect(path string) (domain.VaultInfo, bool) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return domain.VaultInfo{}, false
	}
	info, err := os.Stat(abs)
	if err != nil {
		return domain.VaultInfo{}, false
	}
	if info.IsDir() {
		man, err := readManifestFile(filepath.Join(abs, dpvaultDir, manifestFile))
		if err != nil {
			return domain.VaultInfo{}, false
		}
		return domain.VaultInfo{Root: abs, Locked: !SessionUnlocked(abs), IsFile: false, Hint: man.Hint, Version: man.Version}, true
	}
	if man, ok := readFileVaultHeader(abs); ok {
		return domain.VaultInfo{Root: abs, Locked: !SessionUnlocked(abs), IsFile: true, Hint: man.Hint, Version: man.Version}, true
	}
	return domain.VaultInfo{}, false
}

// Unlock derives the KEK from password and the vault's manifest, unwraps the
// DEK, and — on success — opens a process-memory session for path so
// List/Open/Put/etc. work on it. Wrong password (or a corrupt/foreign file)
// returns ErrInvalidPassword.
func Unlock(path, password string) error {
	info, ok := Detect(path)
	if !ok {
		return ErrNotVault
	}
	var man Manifest
	var err error
	if info.IsFile {
		var okHdr bool
		man, okHdr = readFileVaultHeader(info.Root)
		if !okHdr {
			return ErrNotVault
		}
	} else {
		man, err = readManifestFile(filepath.Join(info.Root, dpvaultDir, manifestFile))
		if err != nil {
			return ErrNotVault
		}
	}
	dek, vaultID, err := unwrapManifest(man, password)
	if err != nil {
		return err
	}
	mu.Lock()
	sessions[info.Root] = &session{
		dek:        dek,
		vaultID:    vaultID,
		isFile:     info.IsFile,
		hint:       man.Hint,
		version:    man.Version,
		unlockedAt: time.Now(),
	}
	mu.Unlock()
	Touch()
	return nil
}
