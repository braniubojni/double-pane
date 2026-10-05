package vault

import (
	"context"
	"crypto/cipher"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/erikharutyunyan/double-pane/internal/domain"
)

// ObjectMeta is decrypted metadata for one object, returned by Open.
type ObjectMeta struct {
	RelPath string
	Size    int64
	MTime   int64 // unix milliseconds
}

type vaultObject struct {
	relPath string // ends with "/" for an explicit empty-dir marker
	objID   [idSize]byte
	objPath string
	size    int64
	mtime   int64
}

func sessionCipher(root string) (dek []byte, release func(), gcm cipher.AEAD, vaultID [idSize]byte, err error) {
	dek, release, ok := beginJob(root)
	if !ok {
		return nil, func() {}, nil, vaultID, ErrLocked
	}
	gcm, err = aesGCM(dek)
	if err != nil {
		release()
		return nil, func() {}, nil, vaultID, err
	}
	mu.Lock()
	if s := sessions[root]; s != nil {
		vaultID = s.vaultID
	}
	mu.Unlock()
	return dek, release, gcm, vaultID, nil
}

// listObjects decrypts every object's name under root's unlocked session.
// The session is only held for the duration of this metadata scan.
func listObjects(root string) ([]vaultObject, error) {
	_, release, gcm, vaultID, err := sessionCipher(root)
	if err != nil {
		return nil, err
	}
	defer release()
	objDir := filepath.Join(root, dpvaultDir, objectsDir)
	entries, err := os.ReadDir(objDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	out := make([]vaultObject, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), objectExt) {
			continue
		}
		objID, ok := parseObjectFileName(e.Name())
		if !ok {
			continue
		}
		objPath := filepath.Join(objDir, e.Name())
		name, size, mtime, err := peekObjectName(objPath, gcm, vaultID, objID)
		if err != nil {
			continue // unreadable/foreign entry — skip, don't fail the whole listing
		}
		out = append(out, vaultObject{relPath: name, objID: objID, objPath: objPath, size: size, mtime: mtime})
	}
	return out, nil
}

// List returns decrypted plaintext entries for dir's immediate children.
// dir must be the vault root or a folder inside an unlocked vault.
// .dpvault itself is never returned (hidden even with Show hidden on).
func List(dir string) ([]domain.FileEntry, error) {
	dir = filepath.Clean(dir)
	root, ok := RootFor(dir)
	if !ok {
		return nil, ErrLocked
	}
	Touch()
	objs, err := listObjects(root)
	if err != nil {
		return nil, err
	}
	relDir := filepath.ToSlash(mustRel(root, dir))
	if relDir == "." {
		relDir = ""
	}

	type child struct {
		name        string
		isDir       bool
		size, mtime int64
	}
	seen := map[string]child{}
	for _, o := range objs {
		isMarker := strings.HasSuffix(o.relPath, "/")
		name := strings.TrimSuffix(o.relPath, "/")
		if relDir != "" {
			if name != relDir && !strings.HasPrefix(name, relDir+"/") {
				continue
			}
			name = strings.TrimPrefix(name, relDir)
			name = strings.TrimPrefix(name, "/")
		}
		if name == "" {
			continue // this object is relDir itself
		}
		parts := strings.SplitN(name, "/", 2)
		first := parts[0]
		if len(parts) > 1 || isMarker {
			if cur, ok := seen[first]; !ok || !cur.isDir {
				seen[first] = child{name: first, isDir: true}
			}
			continue
		}
		seen[first] = child{name: first, isDir: false, size: o.size, mtime: o.mtime}
	}

	out := make([]domain.FileEntry, 0, len(seen))
	for _, c := range seen {
		ext := ""
		if !c.isDir {
			ext = strings.TrimPrefix(filepath.Ext(c.name), ".")
		}
		out = append(out, domain.FileEntry{
			Name: c.name, Path: filepath.Join(dir, c.name), IsDir: c.isDir,
			Size: c.size, ModTime: c.mtime, Ext: ext,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].IsDir != out[j].IsDir {
			return out[i].IsDir
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out, nil
}

// releasingReader wraps an objectReader so the session's in-flight job
// count only drops once the decrypt stream is fully consumed/closed.
type releasingReader struct {
	*objectReader
	release func()
	once    sync.Once
}

func (r *releasingReader) Close() error {
	err := r.objectReader.Close()
	r.once.Do(r.release)
	return err
}

// Open returns a streaming decrypt reader for relPath inside root's
// unlocked vault, plus its metadata. The session is held (see "Jobs must
// hold the session DEK for the duration" in the spec) until Close is called.
func Open(root, relPath string) (*releasingReader, ObjectMeta, error) {
	_, release, gcm, vaultID, err := sessionCipher(root)
	if err != nil {
		return nil, ObjectMeta{}, err
	}
	objDir := filepath.Join(root, dpvaultDir, objectsDir)
	entries, err := os.ReadDir(objDir)
	if err != nil {
		release()
		return nil, ObjectMeta{}, err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), objectExt) {
			continue
		}
		objID, ok := parseObjectFileName(e.Name())
		if !ok {
			continue
		}
		objPath := filepath.Join(objDir, e.Name())
		r, name, size, mtime, err := openObjectReader(objPath, gcm, vaultID, objID)
		if err != nil {
			continue
		}
		if name == relPath {
			return &releasingReader{objectReader: r, release: release}, ObjectMeta{RelPath: relPath, Size: size, MTime: mtime}, nil
		}
		_ = r.Close()
	}
	release()
	return nil, ObjectMeta{}, ErrNotFound
}

// Put streams r (exactly size bytes) into a new object at relPath inside
// root's unlocked vault. Used for outside→vault copies, vault→vault
// re-encrypt (feed it an Open reader), and Mkdir (nil reader, size 0).
func Put(ctx context.Context, root, relPath string, r io.Reader, size, mtime int64) error {
	_, release, gcm, vaultID, err := sessionCipher(root)
	if err != nil {
		return err
	}
	defer release()
	if err := ctx.Err(); err != nil {
		return err
	}
	objDir := filepath.Join(root, dpvaultDir, objectsDir)
	if err := os.MkdirAll(objDir, 0o755); err != nil {
		return err
	}
	return writeObjectAtomic(objDir, gcm, vaultID, relPath, size, mtime, r)
}

// Mkdir writes a zero-byte marker object so an otherwise-empty directory
// survives listing.
func Mkdir(root, relDirPath string, mtime int64) error {
	return Put(context.Background(), root, strings.TrimSuffix(relDirPath, "/")+"/", nil, 0, mtime)
}

// findObject locates the object whose decrypted name equals relPath.
func findObject(root, relPath string) (objPath string, objID [idSize]byte, gcm cipher.AEAD, vaultID [idSize]byte, release func(), err error) {
	_, release, gcm, vaultID, err = sessionCipher(root)
	if err != nil {
		return "", objID, nil, vaultID, func() {}, err
	}
	objDir := filepath.Join(root, dpvaultDir, objectsDir)
	entries, err := os.ReadDir(objDir)
	if err != nil {
		release()
		return "", objID, nil, vaultID, func() {}, err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), objectExt) {
			continue
		}
		id, ok := parseObjectFileName(e.Name())
		if !ok {
			continue
		}
		p := filepath.Join(objDir, e.Name())
		name, _, _, err := peekObjectName(p, gcm, vaultID, id)
		if err != nil {
			continue
		}
		if name == relPath {
			return p, id, gcm, vaultID, release, nil
		}
	}
	release()
	return "", objID, nil, vaultID, func() {}, ErrNotFound
}

// Remove deletes the object at relPath.
func Remove(root, relPath string) error {
	objPath, _, _, _, release, err := findObject(root, relPath)
	defer release()
	if err != nil {
		return err
	}
	return os.Remove(objPath)
}

// Rename rewrites just the encrypted name field of relPath's object to
// newRelPath — the ciphertext chunks are never re-encrypted.
func Rename(root, relPath, newRelPath string) error {
	objPath, objID, gcm, vaultID, release, err := findObject(root, relPath)
	defer release()
	if err != nil {
		return err
	}
	return renameObjectHeader(objPath, gcm, vaultID, objID, newRelPath)
}
