package filesystem

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/erikharutyunyan/double-pane/internal/domain"
)

const defaultUsageDepth = 4

// UsageFileFunc reports one included file. files and bytes are running totals
// inside this measureTree call.
type UsageFileFunc func(path string, files int, bytes int64)

// measureTree is the dirSizeCtx walk plus a file count and the denied paths.
// Symlinks are not followed and do not add their target size. Directory inode
// sizes are not added. onFile may be nil.
func measureTree(ctx context.Context, root string, onFile UsageFileFunc) (total int64, files int, denied []string, err error) {
	err = filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			denied = append(denied, path)
			if os.IsPermission(walkErr) {
				return filepath.SkipDir
			}
			return nil
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		total += info.Size()
		files++
		if onFile != nil {
			onFile(path, files, total)
		}
		return nil
	})
	if err != nil && ctx.Err() != nil {
		err = ctx.Err()
	}
	return total, files, denied, err
}

// TreeUsage builds a depth-limited usage tree for root. maxDepth <= 0 means 4.
// A directory at maxDepth, and any vault directory, is a leaf whose Size comes
// from the dirSizeCtx walk. isVault may be nil. On cancel or failure the tree is nil.
func TreeUsage(
	ctx context.Context,
	root string,
	maxDepth int,
	isVault func(string) bool,
	onProgress func(current string, files int, bytes int64),
) (*domain.UsageNode, int, []string, error) {
	if maxDepth <= 0 {
		maxDepth = defaultUsageDepth
	}
	abs, err := Resolve(root)
	if err != nil {
		return nil, 0, nil, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, 0, nil, err
	}
	if !info.IsDir() {
		return nil, 0, nil, os.ErrInvalid
	}
	if isVault != nil && isVault(abs) {
		return nil, 0, nil, os.ErrInvalid
	}

	b := &usageBuilder{maxDepth: maxDepth, isVault: isVault, onProgress: onProgress}
	node, err := b.directory(ctx, abs, filepath.Base(abs), 0)
	if err != nil {
		return nil, 0, nil, err
	}
	collapseUsage(&node, node.Size)
	denied := b.denied
	if denied == nil {
		denied = []string{}
	}
	return &node, b.files, denied, nil
}

type usageBuilder struct {
	maxDepth   int
	isVault    func(string) bool
	onProgress func(current string, files int, bytes int64)
	files      int
	bytes      int64
	denied     []string
}

func (b *usageBuilder) progress(path string) {
	if b.onProgress != nil {
		b.onProgress(path, b.files, b.bytes)
	}
}

func (b *usageBuilder) directory(ctx context.Context, path, name string, depth int) (domain.UsageNode, error) {
	if err := ctx.Err(); err != nil {
		return domain.UsageNode{}, err
	}
	node := domain.UsageNode{Name: name, Path: path, IsDir: true}
	entries, err := os.ReadDir(path)
	if err != nil {
		if ctx.Err() != nil {
			return domain.UsageNode{}, ctx.Err()
		}
		b.denied = append(b.denied, path)
		return node, nil
	}
	for _, e := range entries {
		if err := ctx.Err(); err != nil {
			return domain.UsageNode{}, err
		}
		childPath := filepath.Join(path, e.Name())
		info, lerr := os.Lstat(childPath)
		if lerr != nil {
			b.denied = append(b.denied, childPath)
			continue
		}
		if info.Mode()&os.ModeSymlink != 0 {
			continue
		}
		if info.IsDir() {
			child, err := b.childDir(ctx, childPath, e.Name(), depth+1)
			if err != nil {
				return domain.UsageNode{}, err
			}
			node.Size += child.Size
			node.Kids = append(node.Kids, child)
			continue
		}
		sz := info.Size()
		b.files++
		b.bytes += sz
		node.Size += sz
		node.Kids = append(node.Kids, domain.UsageNode{
			Name: e.Name(), Path: childPath, Size: sz,
		})
		b.progress(childPath)
	}
	return node, nil
}

// childDir expands a directory, or sizes it with the dirSizeCtx walk when it
// is a vault or sits at maxDepth.
func (b *usageBuilder) childDir(ctx context.Context, path, name string, depth int) (domain.UsageNode, error) {
	if err := ctx.Err(); err != nil {
		return domain.UsageNode{}, err
	}
	vault := b.isVault != nil && b.isVault(path)
	if depth < b.maxDepth && !vault {
		return b.directory(ctx, path, name, depth)
	}
	node := domain.UsageNode{Name: name, Path: path, IsDir: true}
	startFiles, startBytes := b.files, b.bytes
	total, files, denied, err := measureTree(ctx, path, func(p string, nFiles int, nBytes int64) {
		if b.onProgress != nil {
			b.onProgress(p, startFiles+nFiles, startBytes+nBytes)
		}
	})
	if err != nil {
		return domain.UsageNode{}, err
	}
	b.files = startFiles + files
	b.bytes = startBytes + total
	b.denied = append(b.denied, denied...)
	node.Size = total
	return node, nil
}

func underOnePercent(size, total int64) bool {
	if total <= 0 || size < 0 {
		return false
	}
	if size > math.MaxInt64/100 {
		return false
	}
	return size*100 < total
}

func collapseUsage(n *domain.UsageNode, rootTotal int64) {
	if n == nil || len(n.Kids) == 0 {
		return
	}
	kept := make([]domain.UsageNode, 0, len(n.Kids))
	var other int64
	for i := range n.Kids {
		k := n.Kids[i]
		if underOnePercent(k.Size, rootTotal) {
			other += k.Size
			continue
		}
		collapseUsage(&k, rootTotal)
		kept = append(kept, k)
	}
	if other > 0 {
		kept = append(kept, domain.UsageNode{Name: "Other", Size: other})
	}
	slices.SortFunc(kept, func(a, b domain.UsageNode) int {
		if a.Size != b.Size {
			if a.Size > b.Size {
				return -1
			}
			return 1
		}
		return strings.Compare(a.Name, b.Name)
	})
	if len(kept) == 0 {
		n.Kids = nil
		return
	}
	n.Kids = kept
}
