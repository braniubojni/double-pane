package filesystem

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

// xdgTrash implements the FreeDesktop trash spec in one directory
// ({root}/files + {root}/info). Used on Linux and as the isolated backend
// for tests / GFM_CONFIG_DIR.
type xdgTrash struct {
	root string
}

// NewXDGTrash returns a trash rooted at dir (created on first Put).
func NewXDGTrash(dir string) *xdgTrash {
	return &xdgTrash{root: dir}
}

func (x *xdgTrash) filesDir() string { return filepath.Join(x.root, "files") }
func (x *xdgTrash) infoDir() string  { return filepath.Join(x.root, "info") }

func (x *xdgTrash) ensure() error {
	if err := os.MkdirAll(x.filesDir(), 0o700); err != nil {
		return err
	}
	return os.MkdirAll(x.infoDir(), 0o700)
}

func (x *xdgTrash) Put(paths []string) error {
	if err := x.ensure(); err != nil {
		return err
	}
	for _, p := range paths {
		abs, err := Resolve(p)
		if err != nil {
			return err
		}
		if _, err := os.Lstat(abs); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("%w: %s", ErrNotFound, abs)
			}
			if os.IsPermission(err) {
				return fmt.Errorf("%w: cannot access %s", ErrPermission, abs)
			}
			return err
		}
		name := uniqueTrashName(x.filesDir(), filepath.Base(abs))
		stored := filepath.Join(x.filesDir(), name)
		if err := os.Rename(abs, stored); err != nil {
			if !isCrossDevice(err) {
				return err
			}
			if err := copyTo(abs, stored); err != nil {
				_ = os.RemoveAll(stored)
				return err
			}
			if err := Delete([]string{abs}); err != nil {
				_ = os.RemoveAll(stored)
				return err
			}
		}
		if err := writeTrashInfo(filepath.Join(x.infoDir(), name+".trashinfo"), abs); err != nil {
			return err
		}
	}
	return nil
}

func (x *xdgTrash) List() ([]TrashItem, error) {
	ents, err := os.ReadDir(x.filesDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []TrashItem
	for _, e := range ents {
		stored := filepath.Join(x.filesDir(), e.Name())
		origin := readTrashInfoPath(filepath.Join(x.infoDir(), e.Name()+".trashinfo"))
		if origin == "" {
			origin = stored
		}
		row, err := trashItemFromStored(origin, stored)
		if err != nil {
			continue
		}
		if filepath.Base(origin) == filepath.Base(stored) || origin == stored {
			row.Name = e.Name()
			if origin != stored {
				row.Name = filepath.Base(origin)
			}
		}
		out = append(out, row)
	}
	return out, nil
}

func (x *xdgTrash) Restore(storedPaths []string) error {
	items, err := x.List()
	if err != nil {
		return err
	}
	byStored := map[string]TrashItem{}
	for _, it := range items {
		byStored[it.Stored] = it
	}
	return restoreListed(storedPaths, byStored, func(it TrashItem) {
		x.removeMeta(it.Stored)
	})
}

func (x *xdgTrash) RestoreOrigins(origins []string) (int, error) {
	items, err := x.List()
	if err != nil {
		return 0, err
	}
	byOrigin := map[string]TrashItem{}
	for _, it := range items {
		if it.Origin != "" {
			byOrigin[it.Origin] = it
		}
	}
	return restoreOriginsMap(origins, byOrigin, func(it TrashItem) {
		x.removeMeta(it.Stored)
	})
}

func (x *xdgTrash) Empty() error {
	if err := os.RemoveAll(x.filesDir()); err != nil {
		return err
	}
	if err := os.RemoveAll(x.infoDir()); err != nil {
		return err
	}
	return x.ensure()
}

func (x *xdgTrash) Remove(storedPaths []string) error {
	var firstErr error
	for _, p := range storedPaths {
		abs, err := Resolve(p)
		if err != nil {
			abs = p
		}
		if !x.Contains(abs) {
			continue
		}
		if err := Delete([]string{abs}); err != nil && firstErr == nil {
			firstErr = err
		}
		x.removeMeta(abs)
	}
	return firstErr
}

func (x *xdgTrash) Contains(path string) bool {
	if x == nil || x.root == "" {
		return false
	}
	return UnderDir(path, x.filesDir()) || UnderDir(path, x.infoDir()) || UnderDir(path, x.root)
}

func (x *xdgTrash) Roots() []string {
	if x == nil || x.root == "" {
		return nil
	}
	return []string{x.root}
}

func (x *xdgTrash) removeMeta(stored string) {
	_ = os.Remove(filepath.Join(x.infoDir(), filepath.Base(stored)+".trashinfo"))
}

func uniqueTrashName(filesDir, base string) string {
	if base == "" || base == "." {
		base = "item"
	}
	if _, err := os.Lstat(filepath.Join(filesDir, base)); os.IsNotExist(err) {
		return base
	}
	for i := 2; ; i++ {
		name := fmt.Sprintf("%s.%d", base, i)
		if _, err := os.Lstat(filepath.Join(filesDir, name)); os.IsNotExist(err) {
			return name
		}
	}
}

func writeTrashInfo(path, origin string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	body := fmt.Sprintf("[Trash Info]\nPath=%s\nDeletionDate=%s\n",
		encodeTrashInfoPath(origin), time.Now().Format("2006-01-02T15:04:05"))
	return os.WriteFile(path, []byte(body), 0o600)
}

func encodeTrashInfoPath(p string) string {
	if utf8.ValidString(p) && !strings.ContainsAny(p, "\n\r") {
		return p
	}
	return strings.ReplaceAll(url.QueryEscape(p), "+", "%20")
}

func readTrashInfoPath(path string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "Path=") {
			continue
		}
		v := strings.TrimPrefix(line, "Path=")
		if dec, err := url.QueryUnescape(v); err == nil && strings.Contains(v, "%") {
			return dec
		}
		return v
	}
	return ""
}

func xdgDataTrashDir() string {
	if d := os.Getenv("XDG_DATA_HOME"); d != "" {
		return filepath.Join(d, "Trash")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".local", "share", "Trash")
}
