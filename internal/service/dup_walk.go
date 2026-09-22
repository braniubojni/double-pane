package service

import (
	"context"
	"strings"

	"github.com/erikharutyunyan/double-pane/internal/domain"
	"github.com/erikharutyunyan/double-pane/internal/filesystem"
	"github.com/erikharutyunyan/double-pane/internal/remote"
)

type listDirFunc func(path string, showHidden bool) ([]domain.FileEntry, error)

type dupFile struct {
	Path    string
	Name    string
	Size    int64
	ModTime int64
}

type dupSkip struct {
	Path   string
	Reason string
}

type pendingDupDir struct {
	path string
	rel  string
}

// walkForDuplicates lists a tree with no visit/depth cap (unlike walkRemote).
// A ListDir error on a child is a skip; on the root, or a dropped session, it is fatal.
// exclude uses filesystem.PathFilter (same tokens as StartSearch). Excluded
// dirs are not listed; excluded files are omitted, not recorded as skips.
func walkForDuplicates(
	ctx context.Context,
	list listDirFunc,
	root string,
	includeHidden bool,
	minSize int64,
	trashRoots []string,
	exclude string,
) (files []dupFile, skipped []dupSkip, err error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	if !remote.IsRemote(root) && filesystem.SkipDuplicateTrash(root, trashRoots) {
		return nil, nil, nil
	}
	filter := filesystem.NewPathFilter("", exclude)
	queue := []pendingDupDir{{path: root, rel: ""}}
	for len(queue) > 0 {
		if err := ctx.Err(); err != nil {
			return files, skipped, err
		}
		cur := queue[0]
		queue = queue[1:]
		entries, listErr := listDirWithCtx(ctx, list, cur.path, includeHidden)
		if listErr != nil {
			if cur.path == root || isDupFatalErr(listErr) {
				return files, skipped, listErr
			}
			skipped = append(skipped, dupSkip{Path: cur.path, Reason: listErr.Error()})
			continue
		}
		for _, e := range entries {
			if e.Name == ".." {
				continue
			}
			if !includeHidden && strings.HasPrefix(e.Name, ".") {
				continue
			}
			if !remote.IsRemote(e.Path) && filesystem.SkipDuplicateTrash(e.Path, trashRoots) {
				continue
			}
			rel := e.Name
			if cur.rel != "" {
				rel = cur.rel + "/" + e.Name
			}
			if e.IsDir {
				if !filter.MatchDir(rel) {
					continue
				}
				queue = append(queue, pendingDupDir{path: e.Path, rel: rel})
				continue
			}
			if !filter.Match(rel) {
				continue
			}
			if e.Size < minSize {
				continue
			}
			files = append(files, dupFile{
				Path:    e.Path,
				Name:    e.Name,
				Size:    e.Size,
				ModTime: e.ModTime,
			})
		}
	}
	return files, skipped, nil
}

func listDirWithCtx(ctx context.Context, list listDirFunc, path string, hidden bool) ([]domain.FileEntry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	type res struct {
		ents []domain.FileEntry
		err  error
	}
	ch := make(chan res, 1)
	go func() {
		ents, err := list(path, hidden)
		ch <- res{ents, err}
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case r := <-ch:
		return r.ents, r.err
	}
}

func isDupFatalErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	for _, n := range []string{
		"not connected",
		"authentication required",
		"connection refused",
		"connection reset",
		"broken pipe",
	} {
		if strings.Contains(msg, n) {
			return true
		}
	}
	return false
}

func scanProtocol(root string) string {
	if s := remote.SchemeOf(root); s != "" {
		return s
	}
	return "local"
}

func etaSeconds(bytes int64, proto string) int {
	var rate int64
	switch proto {
	case "smb":
		rate = 30 << 20
	case "ssh":
		rate = 10 << 20
	case "mega":
		rate = 4 << 20
	default:
		rate = 200 << 20
	}
	if bytes <= 0 {
		return 0
	}
	return int((bytes + rate - 1) / rate)
}
