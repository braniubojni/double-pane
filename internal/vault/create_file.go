package vault

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/erikharutyunyan/double-pane/internal/domain"
)

// CreateFileVault streams-encrypts a single file into a sibling <name>.dpenc,
// then removes the plaintext original.
func CreateFileVault(ctx context.Context, file, password, hint string, ev func(domain.VaultJobEvent)) error {
	file, err := filepath.Abs(file)
	if err != nil {
		return err
	}
	if len(password) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}
	info, err := os.Stat(file)
	if err != nil || info.IsDir() {
		return fmt.Errorf("not a file: %s", file)
	}
	if err := checkFreeSpace(file, info.Size()); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	man, dek, vaultID, err := newManifest(password, hint)
	if err != nil {
		return err
	}
	defer Wipe(dek)
	gcm, err := aesGCM(dek)
	if err != nil {
		return err
	}

	dest := file + fileVaultExt
	src, err := os.Open(file)
	if err != nil {
		return err
	}
	defer func() { _ = src.Close() }()

	tmp := dest + ".tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	// A file vault holds exactly one object with no filename to embed a
	// content id in, so it always uses the all-zero objectID — still bound
	// to this exact vaultID (and no swapping is possible with only one
	// object to begin with).
	werr := writeFileVaultHeader(out, man)
	if werr == nil {
		werr = writeObject(out, gcm, vaultID, [idSize]byte{}, filepath.Base(file), info.Size(), info.ModTime().UnixMilli(), src)
	}
	cerr := out.Close()
	if werr != nil {
		_ = os.Remove(tmp)
		emitJob(ev, "create-file", file, file, werr.Error(), true, 0, 1, 0, info.Size())
		return werr
	}
	if cerr != nil {
		_ = os.Remove(tmp)
		return cerr
	}
	if err := os.Rename(tmp, dest); err != nil {
		return err
	}
	if err := src.Close(); err != nil {
		return err
	}
	if err := os.Remove(file); err != nil {
		return err
	}
	emitJob(ev, "create-file", file, dest, "", true, 1, 1, info.Size(), info.Size())
	return nil
}
