//go:build windows

package filesystem

import (
	"math"

	"github.com/erikharutyunyan/double-pane/internal/domain"
	"golang.org/x/sys/windows"
)

func diskUsage(path string) (domain.DiskUsage, error) {
	ptr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return domain.DiskUsage{}, err
	}
	var avail, total, totalFree uint64
	if err := windows.GetDiskFreeSpaceEx(ptr, &avail, &total, &totalFree); err != nil {
		return domain.DiskUsage{}, err
	}
	tot := clampU64(total)
	free := clampU64(avail)
	used := tot - clampU64(totalFree)
	if used < 0 {
		used = 0
	}
	return domain.DiskUsage{
		Path:  path,
		Total: tot,
		Free:  free,
		Used:  used,
	}, nil
}

func clampU64(n uint64) int64 {
	if n > math.MaxInt64 {
		return math.MaxInt64
	}
	return int64(n)
}
