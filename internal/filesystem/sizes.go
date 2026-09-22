package filesystem

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/erikharutyunyan/double-pane/internal/domain"
)

// DirChildSizes returns recursive byte sizes for each immediate child directory of dir.
// Symlinks are not followed. Keys are absolute paths of the child dirs.
func DirChildSizes(dir string) (domain.DirSizes, error) {
	return DirChildSizesCtx(context.Background(), dir)
}

// DirChildSizesCtx is like DirChildSizes but respects ctx cancellation.
// A child that could not be fully read still gets its partial size, and is
// listed in Denied — silently undercounting is worse than saying so.
// Keys match ListDir paths (Resolve + filepath.Join with the same abs root).
func DirChildSizesCtx(ctx context.Context, dir string) (domain.DirSizes, error) {
	empty := domain.DirSizes{Sizes: map[string]int64{}, Denied: []string{}}
	abs, err := Resolve(dir)
	if err != nil {
		return empty, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return empty, err
	}
	if !info.IsDir() {
		return empty, fmt.Errorf("not a directory: %s", abs)
	}

	entries, err := os.ReadDir(abs)
	if err != nil {
		return empty, err
	}

	out := domain.DirSizes{Sizes: map[string]int64{}, Denied: []string{}}
	var nDir int
	for _, e := range entries {
		if err := ctx.Err(); err != nil {
			return empty, err
		}
		// Match ListDir: skip symlink children; only real directories.
		if e.Type()&os.ModeSymlink != 0 {
			continue
		}
		if !e.IsDir() {
			continue
		}
		child := filepath.Join(abs, e.Name())
		st, err := os.Lstat(child)
		if err != nil || st.Mode()&os.ModeSymlink != 0 || !st.IsDir() {
			continue
		}
		nDir++
		size, denied, err := dirSizeCtx(ctx, child)
		if err != nil {
			if ctx.Err() != nil {
				return empty, ctx.Err()
			}
			// Still publish partial/zero size so the UI leaves <DIR> mode.
			out.Sizes[child] = size
			out.Denied = append(out.Denied, child)
			continue
		}
		out.Sizes[child] = size
		if denied {
			out.Denied = append(out.Denied, child)
		}
	}
	log.Printf("gfm: DirChildSizes local dir=%q entries=%d dirs=%d sized=%d denied=%d",
		abs, len(entries), nDir, len(out.Sizes), len(out.Denied))
	return out, nil
}

// dirSizeCtx sums a tree. Unreadable subtrees are skipped and reported via
// denied rather than aborting the walk.
func dirSizeCtx(ctx context.Context, root string) (total int64, denied bool, err error) {
	err = filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			denied = true
			if os.IsPermission(walkErr) {
				return filepath.SkipDir
			}
			return nil
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		if !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total, denied, err
}
