// Package vault implements Double Pane's first-party folder/file password
// vault: streaming AES-256-GCM encryption of a folder (or single file) in
// place, with an Argon2id-derived key-encryption-key wrapping a random
// data-encryption-key. No mounting, no shelling out — see specs/folder-vault/v1.md.
package vault

import (
	"encoding/hex"
	"errors"
)

// Sentinel errors. ErrInvalidPassword is returned whenever the wrapped DEK
// fails to authenticate (GCM open failure) — deliberately not distinguished
// from a corrupt manifest, so a shared log line never leaks which is which.
var (
	ErrInvalidPassword       = errors.New("invalid password")
	ErrNotVault              = errors.New("not a vault")
	ErrAlreadyVault          = errors.New("already a vault")
	ErrLocked                = errors.New("vault is locked")
	ErrInsufficientDiskSpace = errors.New("not enough free disk space to lock this folder")
	ErrNotFound              = errors.New("not found in vault")
)

const (
	magic          = "DPV1OBJ\n"
	nonceSize      = 12
	idSize         = 16 // vaultID and objectID are both 16 raw bytes
	dpvaultDir     = ".dpvault"
	objectsDir     = "d"
	manifestFile   = "manifest.json"
	objectExt      = ".dpobj"
	fileVaultExt   = ".dpenc"
	fileHeaderSize = 512 // fixed-size JSON header before magic, .dpenc only

	// defaultChunkSize is the plaintext size per GCM seal. Tests may shrink
	// it via setChunkSizeForTest so multi-chunk fixtures stay small; end
	// users never see a setting for this.
	defaultChunkSize = 1 << 20

	// nameChunkIndex is the reserved AAD chunk-index value for the name
	// field's own seal, distinct from any real chunk index (0..N-1).
	nameChunkIndex = 0xFFFFFFFF
)

var chunkSize = defaultChunkSize

// Argon2Params mirrors manifest.json's "argon2" object.
type Argon2Params struct {
	Time      uint32 `json:"time"`
	MemoryKiB uint32 `json:"memoryKiB"`
	Threads   uint8  `json:"threads"`
}

// DefaultArgon2Params is 64 MiB / 3 passes / 4 threads — acceptable on the
// M-series machines this app targets (spec: do not raise without a setting).
func DefaultArgon2Params() Argon2Params {
	return Argon2Params{Time: 3, MemoryKiB: 64 * 1024, Threads: 4}
}

// Manifest is the unencrypted metadata stored in manifest.json (folder
// vault) or the 512-byte header (file vault).
type Manifest struct {
	Version      int          `json:"version"`
	KDF          string       `json:"kdf"`
	Argon2       Argon2Params `json:"argon2"`
	Salt         string       `json:"salt"`         // base64
	DEKWrapNonce string       `json:"dekWrapNonce"` // base64, 12 bytes
	DEKWrapped   string       `json:"dekWrapped"`   // base64 AES-256-GCM(KEK, DEK), AAD=vaultID
	VaultID      string       `json:"vaultID"`      // hex, 16 bytes
	Hint         string       `json:"hint,omitempty"`
}

func idHex(b [idSize]byte) string { return hex.EncodeToString(b[:]) }

func parseIDHex(s string) ([idSize]byte, error) {
	var out [idSize]byte
	b, err := hex.DecodeString(s)
	if err != nil || len(b) != idSize {
		return out, errors.New("bad id")
	}
	copy(out[:], b)
	return out, nil
}

// objectFileName is <vaultID-short>-<objectID>.dpobj (spec on-disk format).
func objectFileName(vaultID, objectID [idSize]byte) string {
	return hex.EncodeToString(vaultID[:4]) + "-" + idHex(objectID) + objectExt
}

// parseObjectFileName recovers the objectID from a .dpobj name. The
// vaultID-short prefix is fixed length (8 hex chars) so this is a plain slice.
func parseObjectFileName(name string) ([idSize]byte, bool) {
	if len(name) != 8+1+32+len(objectExt) || name[8] != '-' {
		return [idSize]byte{}, false
	}
	id, err := parseIDHex(name[9 : 9+32])
	if err != nil {
		return [idSize]byte{}, false
	}
	return id, true
}
