//go:build linux

package filesystem

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

type linuxTrash struct {
	home *xdgTrash
}

// NewPlatformTrash uses the user's XDG trash plus per-volume .Trash-$UID dirs.
func NewPlatformTrash() TrashBackend {
	root := xdgDataTrashDir()
	if root == "" {
		root = filepath.Join(os.TempDir(), "Trash")
	}
	return &linuxTrash{home: NewXDGTrash(root)}
}

func (t *linuxTrash) Put(paths []string) error {
	for _, p := range paths {
		abs, err := Resolve(p)
		if err != nil {
			return err
		}
		dest := t.backendFor(abs)
		if err := dest.Put([]string{abs}); err != nil {
			return err
		}
	}
	return nil
}

func (t *linuxTrash) backendFor(abs string) *xdgTrash {
	if home, err := os.UserHomeDir(); err == nil && sameDev(abs, home) {
		return t.home
	}
	if sameDev(abs, t.home.filesDir()) || sameDev(abs, t.home.root) {
		return t.home
	}
	if vt := volumeXDG(abs); vt != nil {
		return vt
	}
	return t.home
}

func (t *linuxTrash) List() ([]TrashItem, error) {
	items, err := t.home.List()
	if err != nil {
		return nil, err
	}
	for _, root := range volumeTrashRoots() {
		more, err := NewXDGTrash(root).List()
		if err != nil {
			continue
		}
		items = append(items, more...)
	}
	return items, nil
}

func (t *linuxTrash) Restore(storedPaths []string) error {
	var first error
	ok := 0
	for _, p := range storedPaths {
		var err error
		found := false
		for _, b := range t.backends() {
			if !b.Contains(p) {
				continue
			}
			found = true
			err = b.Restore([]string{p})
			break
		}
		if !found {
			err = fmt.Errorf("not in trash: %s", p)
		}
		if err != nil {
			if first == nil {
				first = err
			}
			continue
		}
		ok++
	}
	if first != nil {
		return first
	}
	if ok == 0 {
		return fmt.Errorf("nothing left to restore")
	}
	return nil
}

func (t *linuxTrash) RestoreOrigins(origins []string) (int, error) {
	n := 0
	var first error
	for _, b := range t.backends() {
		got, err := b.RestoreOrigins(origins)
		n += got
		if err != nil && first == nil {
			first = err
		}
	}
	return n, first
}

func (t *linuxTrash) Empty() error {
	var first error
	for _, b := range t.backends() {
		if err := b.Empty(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func (t *linuxTrash) Remove(storedPaths []string) error {
	var first error
	for _, b := range t.backends() {
		if err := b.Remove(storedPaths); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func (t *linuxTrash) Contains(path string) bool {
	for _, b := range t.backends() {
		if b.Contains(path) {
			return true
		}
	}
	return IsSystemTrashPath(path)
}

func (t *linuxTrash) Roots() []string {
	var out []string
	for _, b := range t.backends() {
		out = append(out, b.Roots()...)
	}
	return out
}

func (t *linuxTrash) backends() []*xdgTrash {
	out := []*xdgTrash{t.home}
	for _, root := range volumeTrashRoots() {
		out = append(out, NewXDGTrash(root))
	}
	return out
}

func sameDev(a, b string) bool {
	var sa, sb syscall.Stat_t
	if err := syscall.Lstat(a, &sa); err != nil {
		return false
	}
	for b != "" && b != "." {
		if err := syscall.Lstat(b, &sb); err == nil {
			return sa.Dev == sb.Dev
		}
		parent := filepath.Dir(b)
		if parent == b {
			break
		}
		b = parent
	}
	return false
}

func mountRoot(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	info, err := os.Lstat(abs)
	if err != nil {
		return abs
	}
	dir := abs
	if !info.IsDir() {
		dir = filepath.Dir(abs)
	}
	var st syscall.Stat_t
	if err := syscall.Stat(dir, &st); err != nil {
		return dir
	}
	for {
		parent := filepath.Dir(dir)
		if parent == dir {
			return dir
		}
		var pst syscall.Stat_t
		if err := syscall.Stat(parent, &pst); err != nil {
			return dir
		}
		if pst.Dev != st.Dev {
			return dir
		}
		dir = parent
	}
}

func volumeXDG(abs string) *xdgTrash {
	root := mountRoot(abs)
	uid := os.Getuid()
	sticky := filepath.Join(root, ".Trash")
	if st, err := os.Lstat(sticky); err == nil && st.IsDir() && st.Mode()&os.ModeSticky != 0 && st.Mode()&os.ModeSymlink == 0 {
		dir := filepath.Join(sticky, fmt.Sprintf("%d", uid))
		return NewXDGTrash(dir)
	}
	return NewXDGTrash(filepath.Join(root, fmt.Sprintf(".Trash-%d", uid)))
}

func volumeTrashRoots() []string {
	uid := os.Getuid()
	seen := map[string]struct{}{}
	var out []string
	add := func(p string) {
		if _, ok := seen[p]; ok {
			return
		}
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			seen[p] = struct{}{}
			out = append(out, p)
		}
	}
	data, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return out
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		mp := fields[1]
		if mp == "/" || mp == "/home" {
			continue
		}
		add(filepath.Join(mp, ".Trash", fmt.Sprintf("%d", uid)))
		add(filepath.Join(mp, fmt.Sprintf(".Trash-%d", uid)))
	}
	return out
}
