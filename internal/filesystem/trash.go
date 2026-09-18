package filesystem

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/erikharutyunyan/go-file-manager/internal/domain"
)

// TrashURI is the virtual pane path for the in-app trash folder.
const TrashURI = "trash://"

// ErrTrashReadOnly is returned for writes that do not make sense in trash
// (mkdir, copy-into, rename). Restore, empty, and permanent delete are OK.
var ErrTrashReadOnly = errors.New("the trash cannot be modified that way; restore or empty it instead")

var errOriginOccupied = errors.New("original path is occupied")

// TrashItem is one top-level entry in a trash backend.
type TrashItem struct {
	Name    string
	Origin  string
	Stored  string
	IsDir   bool
	Size    int64
	ModTime int64
}

// TrashBackend is OS trash (or an isolated XDG dir for tests / GFM_CONFIG_DIR).
type TrashBackend interface {
	Put(paths []string) error
	List() ([]TrashItem, error)
	Restore(storedPaths []string) error
	RestoreOrigins(origins []string) (int, error)
	Empty() error
	Remove(storedPaths []string) error
	Contains(path string) bool
	Roots() []string
}

// IsTrashPath reports whether p is the virtual trash:// location.
func IsTrashPath(p string) bool {
	s := strings.ToLower(strings.TrimSpace(p))
	if s == "" {
		return false
	}
	return s == "trash:" || s == "trash://" || strings.HasPrefix(s, "trash://")
}

// IsLegacyBatchID reports whether s is an old app-trash batch id.
func IsLegacyBatchID(s string) bool {
	return batchIDRe.MatchString(s)
}

// UnderDir reports whether path is root or a descendant of root.
func UnderDir(path, root string) bool {
	if root == "" {
		return false
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	base, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(base, abs)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	sep := string(filepath.Separator)
	return rel != ".." && !strings.HasPrefix(rel, ".."+sep)
}

// SkipDuplicateTrash reports whether a duplicate scan should skip path.
func SkipDuplicateTrash(path string, extraRoots []string) bool {
	if IsSystemTrashPath(path) {
		return true
	}
	for _, r := range extraRoots {
		if UnderDir(path, r) {
			return true
		}
	}
	return false
}

// Trash is leftover app-local undo batches under {config}/trash. New deletes
// go to TrashBackend; these batches stay listable until restore or Empty.
type Trash struct {
	root string
	seq  atomic.Uint64
}

const trashManifest = "manifest.json"

var batchIDRe = regexp.MustCompile(`^[0-9]{8}-[0-9]{9}-[0-9]+$`)

// NewTrash returns leftover app trash rooted at dir.
func NewTrash(dir string) *Trash { return &Trash{root: dir} }

// Root is the leftover trash directory. Empty when t is nil.
func (t *Trash) Root() string {
	if t == nil {
		return ""
	}
	return t.root
}

type trashItem struct {
	Origin string `json:"origin"`
	Stored string `json:"stored"`
}

func (t *Trash) batchDir(id string) string { return filepath.Join(t.root, id) }

func (t *Trash) newBatchID() string {
	now := time.Now().UTC()
	return fmt.Sprintf("%s-%09d-%d", now.Format("20060102"), now.Nanosecond(), t.seq.Add(1))
}

// MoveToTrash keeps leftover-batch tests working. Production Delete no longer calls this.
func (t *Trash) MoveToTrash(paths []string) (string, error) {
	if len(paths) == 0 {
		return "", nil
	}
	id := t.newBatchID()
	dir := t.batchDir(id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", Delete(paths)
	}

	items := make([]trashItem, 0, len(paths))
	for i, p := range paths {
		abs, err := Resolve(p)
		if err != nil {
			return "", err
		}
		if _, err := os.Lstat(abs); err != nil {
			if os.IsNotExist(err) {
				return "", fmt.Errorf("%w: %s", ErrNotFound, abs)
			}
			if os.IsPermission(err) {
				return "", fmt.Errorf("%w: cannot access %s", ErrPermission, abs)
			}
			return "", err
		}
		stored := filepath.Join(dir, fmt.Sprintf("%d-%s", i, filepath.Base(abs)))
		if err := os.Rename(abs, stored); err != nil {
			if isCrossDevice(err) {
				if err := copyTo(abs, stored); err != nil {
					return "", err
				}
				if err := Delete([]string{abs}); err != nil {
					return "", err
				}
			} else {
				return "", err
			}
		}
		items = append(items, trashItem{Origin: abs, Stored: stored})
	}

	if len(items) == 0 {
		_ = os.RemoveAll(dir)
		return "", nil
	}
	raw, err := json.Marshal(items)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(dir, trashManifest), raw, 0o600); err != nil {
		return "", err
	}
	return id, nil
}

// Restore moves a leftover batch back to its original locations.
func (t *Trash) Restore(id string) error {
	if !batchIDRe.MatchString(id) {
		return fmt.Errorf("invalid trash batch id")
	}
	dir := t.batchDir(id)
	raw, err := os.ReadFile(filepath.Join(dir, trashManifest))
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("nothing left to restore")
		}
		return err
	}
	var items []trashItem
	if err := json.Unmarshal(raw, &items); err != nil {
		return err
	}

	var firstErr error
	restored := 0
	occupied := 0
	for _, it := range items {
		err := restoreToOrigin(it.Stored, it.Origin)
		if errors.Is(err, errOriginOccupied) {
			occupied++
			continue
		}
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("cannot restore %s: %w", it.Origin, err)
			}
			continue
		}
		restored++
	}
	if restored == len(items) {
		_ = os.RemoveAll(dir)
	}
	if firstErr != nil {
		return firstErr
	}
	if restored == 0 {
		if occupied > 0 {
			return fmt.Errorf("nothing restored: original paths are occupied")
		}
		return fmt.Errorf("nothing left to restore")
	}
	return nil
}

// List leftover restorable items (missing stored files are skipped).
func (t *Trash) List() ([]TrashItem, error) {
	if t == nil || t.root == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(t.root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []TrashItem
	for _, e := range entries {
		if e.Name() == "dup-cache" || !e.IsDir() || !batchIDRe.MatchString(e.Name()) {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(t.batchDir(e.Name()), trashManifest))
		if err != nil {
			continue
		}
		var items []trashItem
		if err := json.Unmarshal(raw, &items); err != nil {
			continue
		}
		for _, it := range items {
			row, err := trashItemFromStored(it.Origin, it.Stored)
			if err != nil {
				continue
			}
			out = append(out, row)
		}
	}
	return out, nil
}

// RestoreStored puts leftover items back by on-disk path.
func (t *Trash) RestoreStored(storedPaths []string) error {
	if t == nil {
		return nil
	}
	items, err := t.List()
	if err != nil {
		return err
	}
	byStored := map[string]TrashItem{}
	for _, it := range items {
		byStored[it.Stored] = it
	}
	return restoreListed(storedPaths, byStored, func(it TrashItem) {
		t.dropStored(it.Stored)
	})
}

// RestoreOrigins restores leftover items whose Origin matches.
func (t *Trash) RestoreOrigins(origins []string) (int, error) {
	if t == nil {
		return 0, nil
	}
	items, err := t.List()
	if err != nil {
		return 0, err
	}
	byOrigin := map[string]TrashItem{}
	for _, it := range items {
		byOrigin[it.Origin] = it
	}
	return restoreOriginsMap(origins, byOrigin, func(it TrashItem) {
		t.dropStored(it.Stored)
	})
}

func (t *Trash) dropStored(stored string) {
	_ = os.RemoveAll(stored)
	dir := filepath.Dir(stored)
	if batchIDRe.MatchString(filepath.Base(dir)) {
		ents, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		if len(ents) <= 1 {
			_ = os.RemoveAll(dir)
		}
	}
}

// Empty removes leftover batches. Does not touch dup-cache.
func (t *Trash) Empty() error {
	if t == nil || t.root == "" {
		return nil
	}
	entries, err := os.ReadDir(t.root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var firstErr error
	for _, e := range entries {
		if e.Name() == "dup-cache" || !e.IsDir() || !batchIDRe.MatchString(e.Name()) {
			continue
		}
		if err := os.RemoveAll(t.batchDir(e.Name())); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// Remove permanently deletes leftover stored files.
func (t *Trash) Remove(storedPaths []string) error {
	if t == nil {
		return nil
	}
	var firstErr error
	for _, p := range storedPaths {
		if !t.Contains(p) {
			continue
		}
		if err := Delete([]string{p}); err != nil && firstErr == nil {
			firstErr = err
		}
		t.dropStored(p)
	}
	return firstErr
}

// Contains reports whether path lives under leftover app trash (not dup-cache).
func (t *Trash) Contains(path string) bool {
	if t == nil || t.root == "" {
		return false
	}
	if UnderDir(path, filepath.Join(t.root, "dup-cache")) {
		return false
	}
	return UnderDir(path, t.root)
}

func trashItemFromStored(origin, stored string) (TrashItem, error) {
	info, err := os.Lstat(stored)
	if err != nil {
		return TrashItem{}, err
	}
	size := int64(0)
	if !info.IsDir() {
		size = info.Size()
	}
	name := filepath.Base(origin)
	if name == "" || name == "." {
		name = filepath.Base(stored)
	}
	return TrashItem{
		Name:    name,
		Origin:  origin,
		Stored:  stored,
		IsDir:   info.IsDir(),
		Size:    size,
		ModTime: info.ModTime().UnixMilli(),
	}, nil
}

func restoreToOrigin(stored, origin string) error {
	if origin == "" {
		return fmt.Errorf("original path unknown")
	}
	if _, err := os.Lstat(origin); err == nil {
		return errOriginOccupied
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(origin), 0o755); err != nil {
		return err
	}
	if err := os.Rename(stored, origin); err != nil {
		if isCrossDevice(err) {
			if err := copyTo(stored, origin); err != nil {
				return err
			}
			return os.RemoveAll(stored)
		}
		return err
	}
	return nil
}

func restoreListed(storedPaths []string, byStored map[string]TrashItem, onOK func(TrashItem)) error {
	var firstErr error
	restored := 0
	occupied := 0
	for _, p := range storedPaths {
		abs, err := Resolve(p)
		if err != nil {
			abs = p
		}
		it, ok := byStored[abs]
		if !ok {
			it, ok = byStored[p]
		}
		if !ok {
			if firstErr == nil {
				firstErr = fmt.Errorf("not in trash: %s", p)
			}
			continue
		}
		err = restoreToOrigin(it.Stored, it.Origin)
		if errors.Is(err, errOriginOccupied) {
			occupied++
			continue
		}
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("cannot restore %s: %w", it.Origin, err)
			}
			continue
		}
		onOK(it)
		restored++
	}
	if firstErr != nil {
		return firstErr
	}
	if restored == 0 {
		if occupied > 0 {
			return fmt.Errorf("nothing restored: original paths are occupied")
		}
		return fmt.Errorf("nothing left to restore")
	}
	return nil
}

func restoreOriginsMap(origins []string, byOrigin map[string]TrashItem, onOK func(TrashItem)) (int, error) {
	restored := 0
	var firstErr error
	for _, o := range origins {
		it, ok := byOrigin[o]
		if !ok {
			continue
		}
		err := restoreToOrigin(it.Stored, it.Origin)
		if errors.Is(err, errOriginOccupied) {
			continue
		}
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		onOK(it)
		restored++
	}
	return restored, firstErr
}

func isCrossDevice(err error) bool {
	if err == nil {
		return false
	}
	var link *os.LinkError
	if errors.As(err, &link) {
		err = link.Err
	}
	if errors.Is(err, syscall.EXDEV) {
		return true
	}
	// Win32 ERROR_NOT_SAME_DEVICE is 17; POSIX 17 is EEXIST.
	return runtime.GOOS == "windows" && errors.Is(err, syscall.Errno(17))
}

func copyTo(src, dest string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(src)
		if err != nil {
			return err
		}
		return os.Symlink(target, dest)
	}
	if info.IsDir() {
		if err := os.MkdirAll(dest, info.Mode().Perm()); err != nil {
			return err
		}
		ents, err := os.ReadDir(src)
		if err != nil {
			return err
		}
		for _, e := range ents {
			if err := copyTo(filepath.Join(src, e.Name()), filepath.Join(dest, e.Name())); err != nil {
				return err
			}
		}
		return nil
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

// ListTrashEntries builds the trash:// listing (OS trash + leftover app batches).
func ListTrashEntries(osTrash TrashBackend, leftover *Trash, home string) ([]domain.FileEntry, error) {
	var items []TrashItem
	if osTrash != nil {
		got, err := osTrash.List()
		if err != nil {
			return nil, err
		}
		items = append(items, got...)
	}
	if leftover != nil {
		got, err := leftover.List()
		if err != nil {
			return nil, err
		}
		items = append(items, got...)
	}
	return entriesFromTrashItems(items, home), nil
}

func entriesFromTrashItems(items []TrashItem, home string) []domain.FileEntry {
	out := make([]domain.FileEntry, 0, len(items)+1)
	if home != "" {
		out = append(out, domain.FileEntry{
			Name:  "..",
			Path:  home,
			IsDir: true,
		})
	}
	for _, it := range items {
		ext := ""
		if !it.IsDir {
			ext = strings.TrimPrefix(filepath.Ext(it.Name), ".")
		}
		out = append(out, domain.FileEntry{
			Name:    it.Name,
			Path:    it.Stored,
			IsDir:   it.IsDir,
			Size:    it.Size,
			ModTime: it.ModTime,
			Ext:     ext,
			Access:  "full",
			Origin:  it.Origin,
		})
	}
	return out
}
