package service

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/erikharutyunyan/double-pane/internal/domain"
	"github.com/erikharutyunyan/double-pane/internal/filesystem"
	"github.com/erikharutyunyan/double-pane/internal/vault"
)

// listVaultDir is ListDir's vault branch: handled=true means the caller
// should return (entries, err) as-is instead of falling through to a plain
// filesystem.ListDir (which would show raw ciphertext or a locked folder).
func (s *FileService) listVaultDir(path string) (entries []domain.FileEntry, handled bool, err error) {
	abs, rerr := filesystem.Resolve(path)
	if rerr != nil {
		return nil, false, nil
	}
	vault.Touch()
	if _, ok := vault.RootFor(abs); ok {
		entries, err = vault.List(abs)
		if err != nil {
			return nil, true, err
		}
		if parent := filepath.Dir(abs); parent != abs {
			entries = append([]domain.FileEntry{{Name: "..", Path: parent, IsDir: true}}, entries...)
		}
		return entries, true, nil
	}
	if info, ok := vault.Detect(abs); ok && info.Locked {
		return nil, true, vault.ErrLocked
	}
	return nil, false, nil
}

// annotateVaultFlags marks vault rows in an ordinary directory listing so
// the grid can show a lock icon without a second round-trip. Only entries
// that could plausibly be a vault (a directory, or a .dpenc file) pay the
// Detect cost.
func (s *FileService) annotateVaultFlags(entries []domain.FileEntry) {
	for i := range entries {
		e := &entries[i]
		if e.Name == ".." || (!e.IsDir && e.Ext != "dpenc") {
			continue
		}
		if info, ok := vault.Detect(e.Path); ok {
			e.IsVault = true
			e.VaultLocked = info.Locked
			e.VaultFile = info.IsFile
		}
	}
}

func vaultRelPath(root, abs string) string {
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return filepath.Base(abs)
	}
	return filepath.ToSlash(rel)
}

// copyCtx is a small io.Copy that checks ctx between reads, so a cancelled
// job stops mid-stream instead of running to completion.
func copyCtx(ctx context.Context, dst io.Writer, src io.Reader) (int64, error) {
	buf := make([]byte, 64*1024)
	var total int64
	for {
		if err := ctx.Err(); err != nil {
			return total, err
		}
		n, rerr := src.Read(buf)
		if n > 0 {
			wn, werr := dst.Write(buf[:n])
			total += int64(wn)
			if werr != nil {
				return total, werr
			}
		}
		if rerr == io.EOF {
			return total, nil
		}
		if rerr != nil {
			return total, rerr
		}
	}
}

// vaultTransferInvolved reports whether any source or the destination sits
// inside a currently unlocked vault — Copy/Move route through
// runVaultTransfer instead of the generic local/remote transfer path.
func vaultTransferInvolved(sources []string, destDir string) bool {
	if abs, err := filesystem.Resolve(destDir); err == nil {
		if _, ok := vault.RootFor(abs); ok {
			return true
		}
	}
	for _, src := range sources {
		if abs, err := filesystem.Resolve(src); err == nil {
			if _, ok := vault.RootFor(abs); ok {
				return true
			}
		}
	}
	return false
}

// runVaultTransfer implements the "Browse + mutate while unlocked" table's
// Copy/Move rows. Each source is handled individually; onProgress reports
// per-file byte totals (no cross-file byte-accurate total, unlike the
// generic transfer path — vault sizes are rarely huge multi-GB batches).
func (s *FileService) runVaultTransfer(ctx context.Context, sources []string, destDir string, isMove bool, onProgress filesystem.ProgressFunc) error {
	destAbs, err := filesystem.Resolve(destDir)
	if err != nil {
		return err
	}
	destRoot, destInVault := vault.RootFor(destAbs)

	for _, src := range sources {
		if err := ctx.Err(); err != nil {
			return err
		}
		srcAbs, err := filesystem.Resolve(src)
		if err != nil {
			return err
		}
		srcRoot, srcInVault := vault.RootFor(srcAbs)
		name := filepath.Base(srcAbs)

		switch {
		case srcInVault && destInVault && srcRoot == destRoot:
			srcRel := vaultRelPath(srcRoot, srcAbs)
			destRel := filepath.ToSlash(filepath.Join(vaultRelPath(destRoot, destAbs), name))
			if isMove {
				err = vault.Rename(srcRoot, srcRel, destRel)
			} else {
				err = s.copyWithinVault(ctx, srcRoot, srcRel, destRel)
			}
		case srcInVault && !destInVault:
			err = s.copyVaultToPlain(ctx, srcRoot, srcAbs, destAbs, name, onProgress)
			if err == nil && isMove {
				err = vault.Remove(srcRoot, vaultRelPath(srcRoot, srcAbs))
			}
		case !srcInVault && destInVault:
			destRel := filepath.ToSlash(filepath.Join(vaultRelPath(destRoot, destAbs), name))
			err = s.copyPlainToVault(ctx, srcAbs, destRoot, destRel, onProgress)
			if err == nil && isMove {
				err = os.Remove(srcAbs)
			}
		default: // both in vaults, different vaults — rare; via a plaintext temp file.
			err = s.copyAcrossVaults(ctx, srcRoot, srcAbs, destRoot, destAbs, name)
			if err == nil && isMove {
				err = vault.Remove(srcRoot, vaultRelPath(srcRoot, srcAbs))
			}
		}
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
	}
	return nil
}

func (s *FileService) copyWithinVault(ctx context.Context, root, srcRel, destRel string) error {
	rc, meta, err := vault.Open(root, srcRel)
	if err != nil {
		return err
	}
	defer func() { _ = rc.Close() }()
	pr, pw := io.Pipe()
	done := make(chan error, 1)
	go func() {
		_, cerr := copyCtx(ctx, pw, rc)
		done <- pw.CloseWithError(cerr)
	}()
	if err := vault.Put(ctx, root, destRel, pr, meta.Size, meta.MTime); err != nil {
		return err
	}
	return <-done
}

// copyVaultToPlain decrypts srcAbs (inside root) into a new file named name
// under the destDir directory.
func (s *FileService) copyVaultToPlain(ctx context.Context, root, srcAbs, destDir, name string, onProgress filesystem.ProgressFunc) error {
	rel := vaultRelPath(root, srcAbs)
	rc, meta, err := vault.Open(root, rel)
	if err != nil {
		return err
	}
	defer func() { _ = rc.Close() }()
	return decryptReaderTo(ctx, rc, meta, filesystem.UniquePath(filepath.Join(destDir, name)), srcAbs, onProgress)
}

// decryptReaderTo streams rc to a freshly created dest file.
func decryptReaderTo(ctx context.Context, rc io.Reader, meta vault.ObjectMeta, dest, srcAbs string, onProgress filesystem.ProgressFunc) error {
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()
	if onProgress != nil {
		onProgress(filesystem.ProgressEvent{Total: meta.Size, CurrentPath: srcAbs, DestPath: dest})
	}
	if _, err := copyCtx(ctx, out, rc); err != nil {
		return err
	}
	if meta.MTime > 0 {
		_ = os.Chtimes(dest, time.Now(), time.UnixMilli(meta.MTime))
	}
	return nil
}

func (s *FileService) copyPlainToVault(ctx context.Context, srcAbs, destRoot, destRel string, onProgress filesystem.ProgressFunc) error {
	f, err := os.Open(srcAbs)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if onProgress != nil {
		onProgress(filesystem.ProgressEvent{Total: info.Size(), CurrentPath: srcAbs})
	}
	return vault.Put(ctx, destRoot, destRel, f, info.Size(), info.ModTime().UnixMilli())
}

// copyAcrossVaults handles the rare case of copying between two different
// unlocked vaults: decrypt to a plaintext temp file, then encrypt that into
// the destination vault. Simpler than piping the two streams together, and
// this path is not the common one the spec calls out ("same vault" only).
func (s *FileService) copyAcrossVaults(ctx context.Context, srcRoot, srcAbs, destRoot, destAbs, name string) error {
	rel := vaultRelPath(srcRoot, srcAbs)
	rc, meta, err := vault.Open(srcRoot, rel)
	if err != nil {
		return err
	}
	defer func() { _ = rc.Close() }()

	tmp, err := os.CreateTemp("", "dp-vault-xfer-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	if _, err := copyCtx(ctx, tmp, rc); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Seek(0, 0); err != nil {
		_ = tmp.Close()
		return err
	}

	destRel := filepath.ToSlash(filepath.Join(vaultRelPath(destRoot, destAbs), name))
	err = vault.Put(ctx, destRoot, destRel, tmp, meta.Size, meta.MTime)
	_ = tmp.Close()
	return err
}

// deleteVaultPaths implements the Delete row for items inside an unlocked
// vault: always a hard delete (no OS trash — see the spec's V1 choice to
// avoid a plaintext copy landing in Trash).
func deleteVaultPaths(paths []string) (bool, error) {
	type resolved struct {
		abs  string
		root string
	}
	var vaultPaths []resolved
	for _, p := range paths {
		abs, err := filesystem.Resolve(p)
		if err != nil {
			return false, err
		}
		if root, ok := vault.RootFor(abs); ok {
			vaultPaths = append(vaultPaths, resolved{abs, root})
		}
	}
	if len(vaultPaths) == 0 {
		return false, nil
	}
	if len(vaultPaths) != len(paths) {
		return true, fmt.Errorf("mixed vault/non-vault delete is not supported")
	}
	for _, v := range vaultPaths {
		if err := vault.Remove(v.root, vaultRelPath(v.root, v.abs)); err != nil {
			return true, err
		}
	}
	return true, nil
}

// renameVaultPath implements the Rename row (name-field rewrite only).
func renameVaultPath(oldPath, newName string) (string, bool, error) {
	abs, err := filesystem.Resolve(oldPath)
	if err != nil {
		return "", false, err
	}
	root, ok := vault.RootFor(abs)
	if !ok {
		return "", false, nil
	}
	oldRel := vaultRelPath(root, abs)
	newRel := filepath.ToSlash(filepath.Join(filepath.Dir(oldRel), newName))
	if filepath.Dir(oldRel) == "." {
		newRel = newName
	}
	if err := vault.Rename(root, oldRel, newRel); err != nil {
		return "", true, err
	}
	return filepath.Join(filepath.Dir(abs), newName), true, nil
}

// mkdirVaultPath implements the Mkdir row (zero-byte marker object).
func mkdirVaultPath(parent, name string) (string, bool, error) {
	abs, err := filesystem.Resolve(parent)
	if err != nil {
		return "", false, err
	}
	root, ok := vault.RootFor(abs)
	if !ok {
		return "", false, nil
	}
	rel := filepath.ToSlash(filepath.Join(vaultRelPath(root, abs), name))
	if err := vault.Mkdir(root, rel, time.Now().UnixMilli()); err != nil {
		return "", true, err
	}
	return filepath.Join(abs, name), true, nil
}

// readVaultTextFile/writeVaultTextFile implement the built-in editor's
// read/write for a file inside an unlocked vault. The editor already caps
// content at filesystem.MaxTextFileBytes for every file, vault or not, so
// this reads/writes the whole (size-capped) buffer directly rather than the
// spec's temp-file dance — same result, far less code.
func readVaultTextFile(path string) (string, bool, error) {
	abs, err := filesystem.Resolve(path)
	if err != nil {
		return "", false, err
	}
	root, ok := vault.RootFor(abs)
	if !ok {
		return "", false, nil
	}
	rc, meta, err := vault.Open(root, vaultRelPath(root, abs))
	if err != nil {
		return "", true, err
	}
	defer func() { _ = rc.Close() }()
	if meta.Size > filesystem.MaxTextFileBytes {
		return "", true, filesystem.TooLargeError()
	}
	data, err := io.ReadAll(io.LimitReader(rc, filesystem.MaxTextFileBytes+1))
	if err != nil {
		return "", true, err
	}
	return string(data), true, nil
}

func writeVaultTextFile(path, content string) (bool, error) {
	abs, err := filesystem.Resolve(path)
	if err != nil {
		return false, err
	}
	root, ok := vault.RootFor(abs)
	if !ok {
		return false, nil
	}
	rel := vaultRelPath(root, abs)
	r := strings.NewReader(content)
	return true, vault.Put(context.Background(), root, rel, r, int64(len(content)), time.Now().UnixMilli())
}
