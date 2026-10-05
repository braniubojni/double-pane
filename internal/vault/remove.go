package vault

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/erikharutyunyan/double-pane/internal/domain"
)

// RemoveProtection decrypts every object back to plaintext beside .dpvault
// (or, for a .dpenc file, to the sibling name without the suffix), then
// deletes the vault metadata. Safe to retry: a plaintext file already
// written from a previous attempt is left alone (skip, don't overwrite).
func RemoveProtection(ctx context.Context, path, password string, ev func(domain.VaultJobEvent)) error {
	info, ok := Detect(path)
	if !ok {
		return ErrNotVault
	}
	if info.IsFile {
		return removeFileVault(ctx, info.Root, password, ev)
	}
	return removeFolderVault(ctx, info.Root, password, ev)
}

func removeFolderVault(ctx context.Context, root, password string, ev func(domain.VaultJobEvent)) error {
	manPath := filepath.Join(root, dpvaultDir, manifestFile)
	man, err := readManifestFile(manPath)
	if err != nil {
		return ErrNotVault
	}
	dek, vaultID, err := unwrapManifest(man, password)
	if err != nil {
		return err
	}
	defer Wipe(dek)
	gcm, err := aesGCM(dek)
	if err != nil {
		return err
	}

	objDir := filepath.Join(root, dpvaultDir, objectsDir)
	entries, err := os.ReadDir(objDir)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	total := len(entries)
	done := 0
	fail := func(cur string, err error) error {
		emitJob(ev, "remove", root, cur, err.Error(), true, done, total, 0, 0)
		return err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), objectExt) {
			total--
			continue
		}
		if err := ctx.Err(); err != nil {
			return fail(e.Name(), err)
		}
		objID, ok := parseObjectFileName(e.Name())
		if !ok {
			total--
			continue
		}
		objPath := filepath.Join(objDir, e.Name())
		r, name, _, _, err := openObjectReader(objPath, gcm, vaultID, objID)
		if err != nil {
			return fail(e.Name(), err)
		}
		isMarker := strings.HasSuffix(name, "/")
		destRel := strings.TrimSuffix(name, "/")
		dest := filepath.Join(root, filepath.FromSlash(destRel))
		if isMarker {
			_ = os.MkdirAll(dest, 0o755)
			_ = r.Close()
		} else if _, statErr := os.Stat(dest); statErr == nil {
			_ = r.Close() // already restored by a previous attempt — skip, don't overwrite
		} else {
			if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
				_ = r.Close()
				return fail(name, err)
			}
			werr := writePlainFile(dest, r)
			_ = r.Close()
			if werr != nil {
				return fail(name, werr)
			}
		}
		done++
		emitJob(ev, "remove", root, name, "", false, done, total, 0, 0)
	}
	if err := os.RemoveAll(filepath.Join(root, dpvaultDir)); err != nil {
		return fail(root, err)
	}
	emitJob(ev, "remove", root, "", "", true, done, total, 0, 0)
	return nil
}

func removeFileVault(ctx context.Context, path, password string, ev func(domain.VaultJobEvent)) error {
	man, ok := readFileVaultHeader(path)
	if !ok {
		return ErrNotVault
	}
	dek, vaultID, err := unwrapManifest(man, password)
	if err != nil {
		return err
	}
	defer Wipe(dek)
	gcm, err := aesGCM(dek)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	// A file vault's one object always uses the all-zero objectID (see
	// CreateFileVault) and its own name field is ignored — the plaintext
	// name is always this path with the .dpenc suffix stripped.
	r, _, _, _, err := openObjectReaderAt(path, fileHeaderSize, gcm, vaultID, [idSize]byte{})
	if err != nil {
		return err
	}
	dest := strings.TrimSuffix(path, fileVaultExt)
	werr := writePlainFile(dest, r)
	_ = r.Close()
	if werr != nil {
		emitJob(ev, "remove", path, path, werr.Error(), true, 0, 1, 0, 0)
		return werr
	}
	if err := os.Remove(path); err != nil {
		return err
	}
	emitJob(ev, "remove", path, dest, "", true, 1, 1, 0, 0)
	return nil
}

// writePlainFile streams r to a temp file beside dest, then renames into
// place — a reader of dest never observes a partially-decrypted file.
func writePlainFile(dest string, r io.Reader) error {
	tmp := dest + ".tmp-restore"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	_, werr := io.Copy(out, r)
	cerr := out.Close()
	if werr != nil {
		_ = os.Remove(tmp)
		return werr
	}
	if cerr != nil {
		_ = os.Remove(tmp)
		return cerr
	}
	return os.Rename(tmp, dest)
}
