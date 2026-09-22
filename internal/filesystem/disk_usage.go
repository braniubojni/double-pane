package filesystem

import (
	"fmt"

	"github.com/erikharutyunyan/double-pane/internal/domain"
)

// DiskUsage reports total, used, and caller-available free bytes for the
// volume that contains path.
func DiskUsage(path string) (domain.DiskUsage, error) {
	abs, err := Resolve(path)
	if err != nil {
		return domain.DiskUsage{}, err
	}
	u, err := diskUsage(abs)
	if err != nil {
		return domain.DiskUsage{}, fmt.Errorf("disk usage %s: %w", abs, err)
	}
	if u.Path == "" {
		u.Path = abs
	}
	return u, nil
}
