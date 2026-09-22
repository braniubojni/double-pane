//go:build linux

package volumes

import (
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/erikharutyunyan/double-pane/internal/domain"
)

func listOS() ([]domain.Volume, error) {
	data, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return nil, err
	}
	u, _ := user.Current()
	prefixes := []string{"/media", "/mnt", "/run/media"}
	if u != nil && u.Username != "" {
		prefixes = append(prefixes, filepath.Join("/run/media", u.Username))
	}
	var out []domain.Volume
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		mp := fields[1]
		if !underAny(mp, prefixes) {
			continue
		}
		fs := fields[2]
		v := classify(mp, fs, nil, map[string]struct{}{"/": {}})
		v.Device = fields[0]
		out = append(out, v)
	}
	return out, nil
}

func underAny(path string, prefixes []string) bool {
	for _, p := range prefixes {
		if path == p || strings.HasPrefix(path, p+"/") {
			return path != p
		}
	}
	return false
}
