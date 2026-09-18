//go:build windows

package filesystem

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

type windowsTrash struct{}

// NewPlatformTrash uses the Windows Recycle Bin.
func NewPlatformTrash() TrashBackend { return windowsTrash{} }

const (
	foDelete            = 0x0003
	fofSilent           = 0x0004
	fofNoConfirmation   = 0x0010
	fofAllowUndo        = 0x0040
	fofNoErrorUI        = 0x0400
	sherbNoConfirmation = 0x00000001
	sherbNoProgressUI   = 0x00000002
	sherbNoSound        = 0x00000004
)

type shFileOpStruct struct {
	hwnd                  uintptr
	wFunc                 uint32
	pFrom                 *uint16
	pTo                   *uint16
	fFlags                uint16
	fAnyOperationsAborted int32
	hNameMappings         uintptr
	lpszProgressTitle     *uint16
}

var (
	shell32                = windows.NewLazySystemDLL("shell32.dll")
	procSHFileOperationW   = shell32.NewProc("SHFileOperationW")
	procSHEmptyRecycleBinW = shell32.NewProc("SHEmptyRecycleBinW")
)

func (windowsTrash) Put(paths []string) error {
	abs := make([]string, 0, len(paths))
	for _, p := range paths {
		a, err := Resolve(p)
		if err != nil {
			return err
		}
		if _, err := os.Lstat(a); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("%w: %s", ErrNotFound, a)
			}
			if os.IsPermission(err) {
				return fmt.Errorf("%w: cannot access %s", ErrPermission, a)
			}
			return err
		}
		abs = append(abs, a)
	}
	if len(abs) == 0 {
		return nil
	}
	from, err := windows.UTF16PtrFromString(strings.Join(abs, "\x00") + "\x00")
	if err != nil {
		return err
	}
	op := shFileOpStruct{
		wFunc:  foDelete,
		pFrom:  from,
		fFlags: fofSilent | fofNoConfirmation | fofAllowUndo | fofNoErrorUI,
	}
	r, _, callErr := procSHFileOperationW.Call(uintptr(unsafe.Pointer(&op)))
	if r != 0 {
		if callErr != windows.Errno(0) {
			return fmt.Errorf("move to recycle bin: %w", callErr)
		}
		return fmt.Errorf("move to recycle bin: error %d", r)
	}
	return nil
}

func (w windowsTrash) List() ([]TrashItem, error) {
	var out []TrashItem
	for _, dir := range w.sidDirs() {
		ents, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range ents {
			name := e.Name()
			if !strings.HasPrefix(strings.ToUpper(name), "$I") {
				continue
			}
			raw, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				continue
			}
			origin, _, err := parseRecycleI(raw)
			if err != nil {
				continue
			}
			stored := filepath.Join(dir, "$R"+name[2:])
			if _, err := os.Lstat(stored); err != nil {
				continue
			}
			row, err := trashItemFromStored(origin, stored)
			if err != nil {
				continue
			}
			out = append(out, row)
		}
	}
	return out, nil
}

func (w windowsTrash) Restore(storedPaths []string) error {
	items, err := w.List()
	if err != nil {
		return err
	}
	byStored := map[string]TrashItem{}
	for _, it := range items {
		byStored[strings.ToLower(it.Stored)] = it
	}
	lookup := make(map[string]TrashItem, len(byStored))
	want := make([]string, 0, len(storedPaths))
	for _, p := range storedPaths {
		abs, err := Resolve(p)
		if err != nil {
			abs = p
		}
		it, ok := byStored[strings.ToLower(abs)]
		if !ok {
			continue
		}
		lookup[it.Stored] = it
		want = append(want, it.Stored)
	}
	return restoreListed(want, lookup, func(it TrashItem) {
		w.removeIFile(it.Stored)
	})
}

func (w windowsTrash) RestoreOrigins(origins []string) (int, error) {
	items, err := w.List()
	if err != nil {
		return 0, err
	}
	byOrigin := map[string]TrashItem{}
	for _, it := range items {
		byOrigin[strings.ToLower(it.Origin)] = it
	}
	matched := map[string]TrashItem{}
	keys := make([]string, 0, len(origins))
	for _, o := range origins {
		it, ok := byOrigin[strings.ToLower(o)]
		if !ok {
			continue
		}
		matched[it.Origin] = it
		keys = append(keys, it.Origin)
	}
	return restoreOriginsMap(keys, matched, func(it TrashItem) {
		w.removeIFile(it.Stored)
	})
}

func (windowsTrash) Empty() error {
	flags := uintptr(sherbNoConfirmation | sherbNoProgressUI | sherbNoSound)
	r, _, err := procSHEmptyRecycleBinW.Call(0, 0, flags)
	if r != 0 && err != windows.Errno(0) {
		return fmt.Errorf("empty recycle bin: %w", err)
	}
	return nil
}

func (w windowsTrash) Remove(storedPaths []string) error {
	var first error
	for _, p := range storedPaths {
		if !w.Contains(p) {
			continue
		}
		if err := Delete([]string{p}); err != nil && first == nil {
			first = err
		}
		w.removeIFile(p)
	}
	return first
}

func (w windowsTrash) Contains(path string) bool {
	abs, err := Resolve(path)
	if err != nil {
		abs = path
	}
	slash := strings.ToLower(filepath.ToSlash(abs))
	if strings.Contains(slash, "/$recycle.bin/") {
		return true
	}
	return IsSystemTrashPath(abs)
}

func (w windowsTrash) Roots() []string { return w.sidDirs() }

func (w windowsTrash) removeIFile(stored string) {
	base := filepath.Base(stored)
	if strings.HasPrefix(strings.ToUpper(base), "$R") {
		_ = os.Remove(filepath.Join(filepath.Dir(stored), "$I"+base[2:]))
	}
}

func (windowsTrash) sidDirs() []string {
	sid := currentSID()
	var out []string
	for _, drive := range logicalDrives() {
		root := filepath.Join(drive, "$Recycle.Bin")
		if sid != "" {
			p := filepath.Join(root, sid)
			if st, err := os.Stat(p); err == nil && st.IsDir() {
				out = append(out, p)
				continue
			}
		}
		ents, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, e := range ents {
			if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
				out = append(out, filepath.Join(root, e.Name()))
			}
		}
	}
	return out
}

func currentSID() string {
	tok := windows.GetCurrentProcessToken()
	user, err := tok.GetTokenUser()
	if err != nil {
		return ""
	}
	return user.User.Sid.String()
}

func logicalDrives() []string {
	n, err := windows.GetLogicalDriveStrings(0, nil)
	if err != nil || n == 0 {
		return nil
	}
	buf := make([]uint16, n)
	n, err = windows.GetLogicalDriveStrings(n, &buf[0])
	if err != nil {
		return nil
	}
	var out []string
	start := 0
	for i, c := range buf {
		if c == 0 {
			if i > start {
				out = append(out, windows.UTF16ToString(buf[start:i]))
			}
			start = i + 1
		}
	}
	return out
}
