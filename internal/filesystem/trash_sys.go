package filesystem

import (
	"os"
	"path/filepath"
	"strings"
)

// IsSystemTrashPath reports whether path is (inside) a typical OS trash folder.
func IsSystemTrashPath(path string) bool {
	if path == "" {
		return false
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	slash := filepath.ToSlash(abs)
	parts := strings.Split(slash, "/")
	for i, p := range parts {
		if p == ".Trash" || p == ".Trashes" || strings.HasPrefix(p, ".Trash-") {
			return true
		}
		if strings.EqualFold(p, "$Recycle.Bin") {
			return true
		}
		if p == "Trash" && i >= 2 && parts[i-1] == "share" && parts[i-2] == ".local" {
			return true
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		if UnderDir(abs, filepath.Join(home, ".Trash")) {
			return true
		}
		if UnderDir(abs, filepath.Join(home, ".local", "share", "Trash")) {
			return true
		}
	}
	return false
}
