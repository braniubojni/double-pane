package vault

import (
	"context"
	"crypto/cipher"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/erikharutyunyan/double-pane/internal/domain"
)

func emitJob(ev func(domain.VaultJobEvent), kind, path, current, errMsg string, done bool, filesDone, filesTotal int, bytesDone, bytesTotal int64) {
	if ev == nil {
		return
	}
	ev(domain.VaultJobEvent{
		Kind: kind, Path: path, Current: current, Err: errMsg, Done: done,
		FilesDone: filesDone, FilesTotal: filesTotal, BytesDone: bytesDone, BytesTotal: bytesTotal,
	})
}

// CreateFolderVault streams-encrypts folder in place under password. Safe to
// retry after a crash/cancel: if folder already has a partial .dpvault whose
// manifest unwraps with the given password, only the plaintext files that
// were not yet converted are processed (matched by decrypting existing
// object names) — no duplicate objects are created.
func CreateFolderVault(ctx context.Context, folder, password, hint string, ev func(domain.VaultJobEvent)) error {
	folder, err := filepath.Abs(folder)
	if err != nil {
		return err
	}
	if len(password) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}
	if info, err := os.Stat(folder); err != nil || !info.IsDir() {
		return fmt.Errorf("not a folder: %s", folder)
	}

	manPath := filepath.Join(folder, dpvaultDir, manifestFile)
	objDir := filepath.Join(folder, dpvaultDir, objectsDir)

	var man Manifest
	var dek []byte
	var vaultID [idSize]byte
	resuming := false
	if data, statErr := os.ReadFile(manPath); statErr == nil {
		if json.Unmarshal(data, &man) != nil {
			return fmt.Errorf("corrupt vault manifest")
		}
		dek, vaultID, err = unwrapManifest(man, password)
		if err != nil {
			return err
		}
		resuming = true
	} else if !os.IsNotExist(statErr) {
		return statErr
	}
	defer Wipe(dek)

	files, emptyDirs, err := walkPlain(folder)
	if err != nil {
		return err
	}

	if !resuming {
		var total int64
		for _, f := range files {
			total += f.size
		}
		if err := checkFreeSpace(folder, total); err != nil {
			return err
		}
		man, dek, vaultID, err = newManifest(password, hint)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(objDir, 0o755); err != nil {
			return err
		}
		if err := writeManifestFile(manPath, man); err != nil {
			return err
		}
	}

	gcm, err := aesGCM(dek)
	if err != nil {
		return err
	}
	already := map[string]bool{}
	if resuming {
		if already, err = existingRelPaths(objDir, gcm, vaultID); err != nil {
			return err
		}
	}

	var totalBytes, doneBytes int64
	var totalFiles, doneFiles int
	for _, f := range files {
		if !already[f.rel] {
			totalFiles++
			totalBytes += f.size
		}
	}

	fail := func(path string, err error) error {
		emitJob(ev, "create", folder, path, err.Error(), true, doneFiles, totalFiles, doneBytes, totalBytes)
		return err
	}

	for _, f := range files {
		if already[f.rel] {
			continue
		}
		if err := ctx.Err(); err != nil {
			return fail(f.rel, err)
		}
		if err := encryptOneFile(objDir, gcm, vaultID, f); err != nil {
			return fail(f.rel, err)
		}
		if err := os.Remove(f.abs); err != nil {
			return fail(f.rel, err)
		}
		doneFiles++
		doneBytes += f.size
		emitJob(ev, "create", folder, f.rel, "", false, doneFiles, totalFiles, doneBytes, totalBytes)
	}
	for _, d := range emptyDirs {
		name := d + "/"
		if already[name] {
			continue
		}
		if err := writeObjectAtomic(objDir, gcm, vaultID, name, 0, 0, nil); err != nil {
			return fail(d, err)
		}
	}

	removeEmptyPlainDirs(folder)
	emitJob(ev, "create", folder, "", "", true, doneFiles, totalFiles, doneBytes, totalBytes)
	return nil
}

func encryptOneFile(objDir string, gcm cipher.AEAD, vaultID [idSize]byte, f plainFile) error {
	src, err := os.Open(f.abs)
	if err != nil {
		return err
	}
	defer func() { _ = src.Close() }()
	info, err := src.Stat()
	if err != nil {
		return err
	}
	return writeObjectAtomic(objDir, gcm, vaultID, f.rel, info.Size(), info.ModTime().UnixMilli(), src)
}
