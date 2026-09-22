package filesystem

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/adrg/xdg"
	"github.com/erikharutyunyan/double-pane/internal/domain"
)

var (
	ErrInvalidPath = errors.New("invalid path")
	ErrInvalidName = errors.New("invalid name")
	ErrNotFound    = errors.New("path not found")
	ErrExists      = errors.New("destination already exists")
	ErrSamePath    = errors.New("source and destination are the same")
	// ErrBinary is returned when a path is not valid UTF-8 text.
	ErrBinary = errors.New("binary or unsupported encoding")
	// ErrTooLarge is returned when a file exceeds MaxTextFileBytes.
	ErrTooLarge = errors.New("file too large for built-in editor")
)

// Resolve returns a cleaned absolute path.
func Resolve(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", ErrInvalidPath
	}
	abs, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidPath, err)
	}
	return abs, nil
}

// HomeDir returns the current user's home directory.
func HomeDir() (string, error) {
	return os.UserHomeDir()
}

// QuickPlaces returns well-known local folders that exist on this machine,
// in a fixed display order. Best-effort: a folder that doesn't exist or
// can't be statted is silently omitted, never an error.
func QuickPlaces() []domain.QuickPlace {
	home, err := HomeDir()
	if err != nil {
		return nil
	}
	candidates := []domain.QuickPlace{
		{Name: "Home", Path: home},
		{Name: "Desktop", Path: xdg.UserDirs.Desktop},
		{Name: "Documents", Path: xdg.UserDirs.Documents},
		{Name: "Downloads", Path: xdg.UserDirs.Download},
		{Name: "Pictures", Path: xdg.UserDirs.Pictures},
		{Name: "Music", Path: xdg.UserDirs.Music},
		{Name: "Videos", Path: xdg.UserDirs.Videos},
	}
	out := make([]domain.QuickPlace, 0, len(candidates))
	for _, c := range candidates {
		if c.Path == "" {
			continue
		}
		if info, err := os.Stat(c.Path); err == nil && info.IsDir() {
			out = append(out, c)
		}
	}
	return out
}

// Exists reports whether path exists.
func Exists(path string) (bool, error) {
	abs, err := Resolve(path)
	if err != nil {
		return false, err
	}
	_, err = os.Lstat(abs)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// ListDir lists directory entries. Directories first, then files, name-sorted.
// When not at filesystem root, a synthetic ".." entry is prepended.
// If showHidden is false, names starting with '.' are skipped (except "..").
func ListDir(path string, showHidden bool) ([]domain.FileEntry, error) {
	abs, err := Resolve(path)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrNotFound, abs)
		}
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("not a directory: %s", abs)
	}

	entries, err := os.ReadDir(abs)
	if err != nil {
		return nil, err
	}

	result := make([]domain.FileEntry, 0, len(entries)+1)
	if parent := filepath.Dir(abs); parent != abs {
		result = append(result, domain.FileEntry{
			Name:  "..",
			Path:  parent,
			IsDir: true,
		})
	}

	for _, e := range entries {
		name := e.Name()
		if !showHidden && strings.HasPrefix(name, ".") {
			continue
		}
		full := filepath.Join(abs, name)
		entry, err := entryFromDirEntry(full, e)
		if err != nil {
			// Skip unreadable entries rather than failing the whole list.
			continue
		}
		result = append(result, entry)
	}

	// Sort: keep ".." first, then dirs, then files, by name (case-insensitive).
	sort.SliceStable(result, func(i, j int) bool {
		a, b := result[i], result[j]
		if a.Name == ".." {
			return true
		}
		if b.Name == ".." {
			return false
		}
		if a.IsDir != b.IsDir {
			return a.IsDir
		}
		return strings.ToLower(a.Name) < strings.ToLower(b.Name)
	})

	return result, nil
}

type completionItem struct {
	full       string
	name       string
	isDir      bool
	startsWith bool
	isDot      bool
}

// ListPathCompletions returns absolute path suggestions for partial input (max 50).
// Filter: case-insensitive contains. Order: non-dot first, startsWith before contains-only,
// directories before files, then case-insensitive name.
func ListPathCompletions(partial string) ([]string, error) {
	partial = strings.TrimSpace(partial)
	if partial == "" {
		home, err := HomeDir()
		if err != nil {
			return nil, err
		}
		partial = home + string(os.PathSeparator)
	}

	// Expand ~ to home
	if partial == "~" || strings.HasPrefix(partial, "~/") {
		home, err := HomeDir()
		if err != nil {
			return nil, err
		}
		if partial == "~" {
			partial = home + string(os.PathSeparator)
		} else {
			partial = filepath.Join(home, partial[2:])
		}
	}

	var dir, query string
	if strings.HasSuffix(partial, "/") || strings.HasSuffix(partial, string(os.PathSeparator)) {
		dir = partial
		query = ""
	} else {
		dir = filepath.Dir(partial)
		query = filepath.Base(partial)
	}

	absDir, err := Resolve(dir)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(absDir)
	if err != nil || !info.IsDir() {
		return []string{}, nil
	}

	entries, err := os.ReadDir(absDir)
	if err != nil {
		return nil, err
	}

	queryLower := strings.ToLower(query)
	items := make([]completionItem, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		nameLower := strings.ToLower(name)
		if query != "" && !strings.Contains(nameLower, queryLower) {
			continue
		}
		full := filepath.Join(absDir, name)
		isDir := e.IsDir()
		if isDir {
			full = full + string(os.PathSeparator)
		}
		items = append(items, completionItem{
			full:       full,
			name:       name,
			isDir:      isDir,
			startsWith: query == "" || strings.HasPrefix(nameLower, queryLower),
			isDot:      strings.HasPrefix(name, "."),
		})
	}

	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]
		if a.isDot != b.isDot {
			return !a.isDot // non-dot first
		}
		if a.startsWith != b.startsWith {
			return a.startsWith
		}
		if a.isDir != b.isDir {
			return a.isDir
		}
		return strings.ToLower(a.name) < strings.ToLower(b.name)
	})

	limit := 50
	if len(items) < limit {
		limit = len(items)
	}
	out := make([]string, limit)
	for i := 0; i < limit; i++ {
		out[i] = items[i].full
	}
	return out, nil
}

func entryFromDirEntry(full string, e os.DirEntry) (domain.FileEntry, error) {
	info, err := e.Info()
	if err != nil {
		return domain.FileEntry{}, err
	}
	isSymlink := info.Mode()&os.ModeSymlink != 0
	isDir := e.IsDir()
	// Judge access by the target: a symlink's own mode is always 0777.
	accessInfo := info
	if isSymlink {
		if target, err := os.Stat(full); err == nil {
			isDir = target.IsDir()
			accessInfo = target
		}
	}
	size := int64(0)
	if !isDir {
		size = info.Size()
	}
	ext := ""
	if !isDir {
		ext = strings.TrimPrefix(filepath.Ext(e.Name()), ".")
	}
	return domain.FileEntry{
		Name:      e.Name(),
		Path:      full,
		IsDir:     isDir,
		Size:      size,
		ModTime:   info.ModTime().UnixMilli(),
		Ext:       ext,
		IsSymlink: isSymlink,
		Access:    AccessFor(accessInfo),
	}, nil
}

// validateName rejects empty, dotted-path, and separator-containing names.
func validateName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." {
		return ErrInvalidName
	}
	if strings.ContainsAny(name, `/\`) {
		return ErrInvalidName
	}
	if filepath.Base(name) != name {
		return ErrInvalidName
	}
	return nil
}

// Mkdir creates a directory under parent.
func Mkdir(parent, name string) (string, error) {
	if err := validateName(name); err != nil {
		return "", err
	}
	abs, err := Resolve(parent)
	if err != nil {
		return "", err
	}
	dest := filepath.Join(abs, name)
	if err := os.Mkdir(dest, 0o755); err != nil {
		if os.IsExist(err) {
			return "", fmt.Errorf("%w: %s", ErrExists, dest)
		}
		return "", err
	}
	return dest, nil
}

// Rename renames a path in place (newName is basename only).
func Rename(oldPath, newName string) (string, error) {
	if err := validateName(newName); err != nil {
		return "", err
	}
	abs, err := Resolve(oldPath)
	if err != nil {
		return "", err
	}
	dest := filepath.Join(filepath.Dir(abs), newName)
	if abs == dest {
		return abs, nil
	}
	if _, err := os.Lstat(dest); err == nil {
		return "", fmt.Errorf("%w: %s", ErrExists, dest)
	}
	if err := os.Rename(abs, dest); err != nil {
		return "", err
	}
	return dest, nil
}

// ErrPermission is returned when the OS denies delete/write access.
var ErrPermission = errors.New("permission denied")

// Delete removes files or directories (recursive for dirs). Does not follow symlinks for removal of the link itself.
func Delete(paths []string) error {
	for _, p := range paths {
		abs, err := Resolve(p)
		if err != nil {
			return err
		}
		info, err := os.Lstat(abs)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("%w: %s", ErrNotFound, abs)
			}
			if os.IsPermission(err) {
				return fmt.Errorf("%w: cannot access %s", ErrPermission, abs)
			}
			return err
		}
		var delErr error
		if info.IsDir() && info.Mode()&os.ModeSymlink == 0 {
			delErr = os.RemoveAll(abs)
		} else {
			delErr = os.Remove(abs)
		}
		if delErr != nil {
			if os.IsPermission(delErr) {
				return fmt.Errorf("%w: cannot delete %s", ErrPermission, abs)
			}
			// Normalize common OS messages (e.g. unlinkat …: permission denied)
			msg := delErr.Error()
			if strings.Contains(strings.ToLower(msg), "permission denied") {
				return fmt.Errorf("%w: cannot delete %s", ErrPermission, abs)
			}
			return fmt.Errorf("cannot delete %s: %w", abs, delErr)
		}
	}
	return nil
}

// Copy copies sources into destDir (files and directories).
func Copy(sources []string, destDir string) error {
	return CopyCtx(context.Background(), sources, destDir, nil)
}

// Move moves sources into destDir.
func Move(sources []string, destDir string) error {
	return MoveCtx(context.Background(), sources, destDir, nil)
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// UniquePath returns path if free, else "name (n).ext" in the same directory.
func UniquePath(path string) string {
	if _, err := os.Lstat(path); os.IsNotExist(err) {
		return path
	}
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	for i := 1; ; i++ {
		candidate := filepath.Join(dir, fmt.Sprintf("%s (%d)%s", name, i, ext))
		if _, err := os.Lstat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
}
