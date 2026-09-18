//go:build darwin

package filesystem

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
)

type darwinTrash struct{}

// NewPlatformTrash uses NSFileManager via JXA so Finder Put Back still works.
func NewPlatformTrash() TrashBackend { return darwinTrash{} }

func (darwinTrash) Put(paths []string) error {
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
	const chunk = 32
	for i := 0; i < len(abs); i += chunk {
		end := i + chunk
		if end > len(abs) {
			end = len(abs)
		}
		if err := jxaTrash(abs[i:end]); err != nil {
			return err
		}
	}
	return nil
}

func (d darwinTrash) List() ([]TrashItem, error) {
	var stored []string
	for _, dir := range d.dirs() {
		ents, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range ents {
			name := e.Name()
			if name == ".DS_Store" || name == ".localized" {
				continue
			}
			stored = append(stored, filepath.Join(dir, name))
		}
	}
	orig := jxaOriginalPaths(stored)
	var out []TrashItem
	for _, p := range stored {
		origin := orig[p]
		if origin == "" {
			origin = p
		}
		row, err := trashItemFromStored(origin, p)
		if err != nil {
			continue
		}
		if origin == p {
			row.Origin = ""
		}
		out = append(out, row)
	}
	return out, nil
}

func (d darwinTrash) Restore(storedPaths []string) error {
	items, err := d.List()
	if err != nil {
		return err
	}
	byStored := map[string]TrashItem{}
	for _, it := range items {
		byStored[it.Stored] = it
	}
	return restoreListed(storedPaths, byStored, func(TrashItem) {})
}

func (d darwinTrash) RestoreOrigins(origins []string) (int, error) {
	items, err := d.List()
	if err != nil {
		return 0, err
	}
	byOrigin := map[string]TrashItem{}
	for _, it := range items {
		if it.Origin != "" {
			byOrigin[it.Origin] = it
		}
	}
	return restoreOriginsMap(origins, byOrigin, func(TrashItem) {})
}

func (d darwinTrash) Empty() error {
	var first error
	for _, dir := range d.dirs() {
		ents, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range ents {
			name := e.Name()
			if name == ".DS_Store" || name == ".localized" {
				continue
			}
			if err := os.RemoveAll(filepath.Join(dir, name)); err != nil && first == nil {
				first = err
			}
		}
	}
	return first
}

func (d darwinTrash) Remove(storedPaths []string) error {
	var first error
	for _, p := range storedPaths {
		if !d.Contains(p) {
			continue
		}
		if err := Delete([]string{p}); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func (d darwinTrash) Contains(path string) bool {
	for _, dir := range d.dirs() {
		if UnderDir(path, dir) {
			return true
		}
	}
	return IsSystemTrashPath(path)
}

func (d darwinTrash) Roots() []string { return d.dirs() }

func (d darwinTrash) dirs() []string {
	var out []string
	if home, err := os.UserHomeDir(); err == nil {
		out = append(out, filepath.Join(home, ".Trash"))
	}
	uid := "501"
	if u, err := user.Current(); err == nil && u.Uid != "" {
		uid = u.Uid
	}
	ents, err := os.ReadDir("/Volumes")
	if err != nil {
		return out
	}
	for _, e := range ents {
		p := filepath.Join("/Volumes", e.Name(), ".Trashes", uid)
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			out = append(out, p)
		}
	}
	return out
}

func jxaTrash(paths []string) error {
	script := `ObjC.import('Foundation');
function run(argv) {
  var fm = $.NSFileManager.defaultManager;
  for (var i = 0; i < argv.length; i++) {
    var url = $.NSURL.fileURLWithPath(argv[i]);
    var dest = Ref();
    var err = Ref();
    var ok = fm.trashItemAtURLResultingItemURLError(url, dest, err);
    if (!ok) {
      var msg = 'trash failed';
      if (err[0]) msg = ObjC.unwrap(err[0].localizedDescription);
      return 'ERR:' + msg;
    }
  }
  return 'OK';
}`
	args := append([]string{"-l", "JavaScript", "-e", script, "--"}, paths...)
	out, err := exec.Command("osascript", args...).CombinedOutput()
	s := strings.TrimSpace(string(out))
	if err != nil {
		return fmt.Errorf("move to trash: %s", s)
	}
	if strings.HasPrefix(s, "ERR:") {
		return fmt.Errorf("move to trash: %s", strings.TrimPrefix(s, "ERR:"))
	}
	return nil
}

func jxaOriginalPaths(paths []string) map[string]string {
	out := map[string]string{}
	if len(paths) == 0 {
		return out
	}
	script := `ObjC.import('Foundation');
function run(argv) {
  var lines = [];
  for (var i = 0; i < argv.length; i++) {
    var p = argv[i];
    var orig = '';
    try {
      var url = $.NSURL.fileURLWithPath(p);
      var val = Ref();
      var err = Ref();
      if (url.getResourceValueForKeyError(val, $.NSURLTrashItemOriginalPathKey, err) && val[0]) {
        orig = ObjC.unwrap(val[0]);
      }
    } catch (e) {}
    lines.push(p + '\t' + orig);
  }
  return lines.join('\n');
}`
	const chunk = 64
	for i := 0; i < len(paths); i += chunk {
		end := i + chunk
		if end > len(paths) {
			end = len(paths)
		}
		args := append([]string{"-l", "JavaScript", "-e", script, "--"}, paths[i:end]...)
		b, err := exec.Command("osascript", args...).CombinedOutput()
		if err != nil {
			continue
		}
		for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
			p, orig, ok := strings.Cut(line, "\t")
			if !ok {
				continue
			}
			out[p] = orig
		}
	}
	return out
}
