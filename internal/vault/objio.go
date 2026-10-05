package vault

import (
	"crypto/cipher"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/erikharutyunyan/double-pane/internal/filesystem"
)

// diskUsage is filesystem.DiskUsage, indirected so tests can simulate a full
// disk without needing an actually-full volume.
var diskUsage = filesystem.DiskUsage

const freeSpaceMarginBytes = 64 * 1024 * 1024

func checkFreeSpace(path string, needed int64) error {
	du, err := diskUsage(path)
	if err != nil {
		return nil // best-effort: unknown free space never blocks a lock
	}
	if du.Free < needed+freeSpaceMarginBytes {
		return ErrInsufficientDiskSpace
	}
	return nil
}

type plainFile struct {
	rel  string // posix, relative to vault root
	abs  string
	size int64
}

// walkPlain walks folder (skipping .dpvault) and returns every plaintext
// file plus every directory that is entirely empty (no files anywhere below
// it) — the latter need an explicit Mkdir-style marker object so they are
// not silently lost on lock.
func walkPlain(folder string) (files []plainFile, emptyDirs []string, err error) {
	err = filepath.WalkDir(folder, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if p == folder {
			return nil
		}
		rel, relErr := filepath.Rel(folder, p)
		if relErr != nil {
			return relErr
		}
		relSlash := filepath.ToSlash(rel)
		if relSlash == dpvaultDir || strings.HasPrefix(relSlash, dpvaultDir+"/") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			entries, rerr := os.ReadDir(p)
			if rerr == nil && len(entries) == 0 {
				emptyDirs = append(emptyDirs, relSlash)
			}
			return nil
		}
		info, ierr := d.Info()
		if ierr != nil {
			return ierr
		}
		files = append(files, plainFile{rel: relSlash, abs: p, size: info.Size()})
		return nil
	})
	return files, emptyDirs, err
}

// removeEmptyPlainDirs removes now-empty directories left behind after every
// plaintext file has been encrypted and deleted (deepest first), skipping
// the vault root itself and .dpvault. Best-effort: a directory that still
// has something in it (e.g. an item the walk could not read) is left alone.
func removeEmptyPlainDirs(folder string) {
	var dirs []string
	_ = filepath.WalkDir(folder, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || p == folder || !d.IsDir() {
			return nil
		}
		rel := filepath.ToSlash(mustRel(folder, p))
		if rel == dpvaultDir || strings.HasPrefix(rel, dpvaultDir+"/") {
			return filepath.SkipDir
		}
		dirs = append(dirs, p)
		return nil
	})
	sort.Slice(dirs, func(i, j int) bool { return len(dirs[i]) > len(dirs[j]) })
	for _, d := range dirs {
		_ = os.Remove(d) // no-op error if not empty
	}
}

func mustRel(base, target string) string {
	r, err := filepath.Rel(base, target)
	if err != nil {
		return target
	}
	return r
}

// existingRelPaths decrypts the name field of every object already written
// under objDir, for CreateFolderVault's crash-mid-create retry path.
func existingRelPaths(objDir string, gcm cipher.AEAD, vaultID [idSize]byte) (map[string]bool, error) {
	out := map[string]bool{}
	entries, err := os.ReadDir(objDir)
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), objectExt) {
			continue
		}
		objID, ok := parseObjectFileName(e.Name())
		if !ok {
			continue
		}
		name, _, _, err := peekObjectName(filepath.Join(objDir, e.Name()), gcm, vaultID, objID)
		if err != nil {
			continue // unreadable/foreign entry — leave it, don't fail the retry
		}
		out[name] = true
	}
	return out, nil
}

func peekObjectName(objPath string, gcm cipher.AEAD, vaultID, objectID [idSize]byte) (string, int64, int64, error) {
	f, err := os.Open(objPath)
	if err != nil {
		return "", 0, 0, err
	}
	defer func() { _ = f.Close() }()
	h, err := readRawHeader(f)
	if err != nil {
		return "", 0, 0, err
	}
	name, err := decryptName(gcm, vaultID, objectID, h)
	if err != nil {
		return "", 0, 0, err
	}
	return name, h.origSize, h.mtime, nil
}

// writeObjectAtomic encrypts r (exactly size bytes; a nil r with size 0 is a
// zero-byte marker) into a fresh object under a temp name, then renames it
// into place — crash-safe: a reader never observes a half-written object.
func writeObjectAtomic(objDir string, gcm cipher.AEAD, vaultID [idSize]byte, relPath string, size, mtime int64, r io.Reader) error {
	objID, err := randID()
	if err != nil {
		return err
	}
	tmp := filepath.Join(objDir, ".tmp-"+idHex(objID))
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	werr := writeObject(out, gcm, vaultID, objID, relPath, size, mtime, r)
	cerr := out.Close()
	if werr != nil {
		_ = os.Remove(tmp)
		return werr
	}
	if cerr != nil {
		_ = os.Remove(tmp)
		return cerr
	}
	return os.Rename(tmp, filepath.Join(objDir, objectFileName(vaultID, objID)))
}
