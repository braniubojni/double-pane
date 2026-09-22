//go:build !windows

package filesystem

import (
	"fmt"
	"math"

	"github.com/erikharutyunyan/double-pane/internal/domain"
	"golang.org/x/sys/unix"
)

func diskUsage(path string) (domain.DiskUsage, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(path, &st); err != nil {
		return domain.DiskUsage{}, err
	}
	// Darwin Bsize is int32; Linux is int64. Always widen before multiply.
	bsize := int64(st.Bsize)
	if bsize <= 0 {
		return domain.DiskUsage{}, fmt.Errorf("invalid filesystem block size %d", bsize)
	}
	total := mulBlockSize(int64(st.Blocks), bsize)
	free := mulBlockSize(int64(st.Bavail), bsize)
	bfree := mulBlockSize(int64(st.Bfree), bsize)
	used := total - bfree
	if used < 0 {
		used = 0
	}
	return domain.DiskUsage{
		Path:  path,
		Total: total,
		Free:  free,
		Used:  used,
	}, nil
}

func mulBlockSize(count, bsize int64) int64 {
	if count <= 0 || bsize <= 0 {
		return 0
	}
	if count > math.MaxInt64/bsize {
		return math.MaxInt64
	}
	return count * bsize
}
