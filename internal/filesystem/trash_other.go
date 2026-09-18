//go:build !darwin && !linux && !windows

package filesystem

import (
	"os"
	"path/filepath"
)

// NewPlatformTrash falls back to an XDG-style directory under the user home.
func NewPlatformTrash() TrashBackend {
	root := xdgDataTrashDir()
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			root = filepath.Join(os.TempDir(), "Trash")
		} else {
			root = filepath.Join(home, ".local", "share", "Trash")
		}
	}
	return NewXDGTrash(root)
}
