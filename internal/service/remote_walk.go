package service

import (
	"context"
	"io"
	"strings"

	"github.com/erikharutyunyan/double-pane/internal/domain"
	"github.com/erikharutyunyan/double-pane/internal/filesystem"
)

// ponytail: remote listing is a serial round trip per directory (no local-fs
// speed), so these caps are much tighter than the local walkers in
// filesystem/search.go (5000/12) and filesystem/folder_search.go (100000).
// Raise them if users complain about shallow go-to/search results on remote.
const (
	maxRemoteWalkVisits = 2000
	maxRemoteWalkDepth  = 8
)

type pendingRemoteDir struct {
	path  string
	rel   string
	depth int
}

// walkRemote breadth-first walks a remote tree via be.ListDir. Unreadable
// directories are skipped rather than aborting the whole walk. rel is the
// slash-separated path relative to root (matching filter.Match's expectation).
// fn is called for every entry and returns (descend, stop): descend controls
// whether a directory entry is queued for its own listing (independent of
// whether fn already reported it as a hit — lets callers prune excluded
// subtrees), stop aborts the entire walk immediately (e.g. limit reached).
// The walk also stops on ctx cancel or the visit/depth caps above.
func walkRemote(
	ctx context.Context,
	be remoteBackend,
	root string,
	showHidden bool,
	fn func(e domain.FileEntry, rel string, depth int) (descend, stop bool),
) error {
	queue := []pendingRemoteDir{{path: root, rel: "", depth: -1}}
	visits := 0
	for len(queue) > 0 {
		if ctx.Err() != nil {
			return nil
		}
		cur := queue[0]
		queue = queue[1:]

		entries, err := be.ListDir(cur.path, showHidden)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if ctx.Err() != nil {
				return nil
			}
			if e.Name == ".." {
				continue
			}
			visits++
			if visits > maxRemoteWalkVisits {
				return nil
			}
			if !showHidden && strings.HasPrefix(e.Name, ".") {
				continue
			}
			depth := cur.depth + 1
			rel := e.Name
			if cur.rel != "" {
				rel = cur.rel + "/" + e.Name
			}
			descend, stop := fn(e, rel, depth)
			if stop {
				return nil
			}
			if descend && e.IsDir && depth < maxRemoteWalkDepth {
				queue = append(queue, pendingRemoteDir{path: e.Path, rel: rel, depth: depth})
			}
		}
	}
	return nil
}

// searchFoldersRemote streams folder-name hits from a remote tree, reusing
// the same include/exclude glob semantics as the local filesystem.SearchFolders.
func (s *FileService) searchFoldersRemote(
	ctx context.Context,
	be remoteBackend,
	root, query, include, exclude string,
	showHidden bool,
	limit int,
	onHit func(domain.SearchHit),
	onDenied func(path string, err error),
) (truncated bool, err error) {
	if limit <= 0 {
		limit = 500
	}
	filter := filesystem.NewPathFilter(include, exclude)
	q := strings.ToLower(strings.TrimSpace(query))
	hits := 0
	walkErr := walkRemote(ctx, be, root, showHidden, func(e domain.FileEntry, rel string, depth int) (descend, stop bool) {
		if !e.IsDir {
			return false, false
		}
		if !filter.MatchDir(rel) {
			return false, false // prune: do not descend into excluded dirs
		}
		if filter.Match(rel) && (q == "" || strings.Contains(strings.ToLower(e.Name), q)) {
			onHit(domain.SearchHit{Name: e.Name, Path: e.Path, IsDir: true, RelPath: rel})
			hits++
			if hits >= limit {
				truncated = true
				return false, true
			}
		}
		return true, false
	})
	if walkErr != nil {
		return truncated, walkErr
	}
	_ = onDenied // remote ListDir errors are skipped silently by walkRemote, not surfaced per-dir
	return truncated, nil
}

// searchContentRemote streams content matches from a remote tree (SSH/SMB).
// Sequential OpenRead — no worker pool (shared remote session). Files over
// MaxContentFileBytes are skipped without opening.
func (s *FileService) searchContentRemote(
	ctx context.Context,
	be remoteBackend,
	root, query, include, exclude string,
	caseSensitive, showHidden bool,
	limit int,
	onHit func(domain.ContentSearchHit),
	onDenied func(path string, err error),
) (truncated bool, err error) {
	if strings.TrimSpace(query) == "" {
		return false, nil
	}
	if limit <= 0 {
		limit = filesystem.DefaultContentHitLimit
	}
	filter := filesystem.NewPathFilter(include, exclude)
	needle := query
	if !caseSensitive {
		needle = strings.ToLower(query)
	}
	hits := 0
	walkErr := walkRemote(ctx, be, root, showHidden, func(e domain.FileEntry, rel string, _ int) (descend, stop bool) {
		if e.IsDir {
			if !filter.MatchDir(rel) {
				return false, false
			}
			return true, false
		}
		if !filter.Match(rel) {
			return false, false
		}
		if e.Size == 0 || e.Size > filesystem.MaxContentFileBytes {
			return false, false
		}
		if ctx.Err() != nil {
			return false, true
		}

		r, openErr := be.OpenRead(e.Path)
		if openErr != nil {
			if onDenied != nil {
				onDenied(e.Path, openErr)
			}
			return false, false
		}
		data, readErr := io.ReadAll(io.LimitReader(r, filesystem.MaxContentFileBytes+1))
		_ = r.Close()
		if readErr != nil {
			if onDenied != nil {
				onDenied(e.Path, readErr)
			}
			return false, false
		}
		if int64(len(data)) > filesystem.MaxContentFileBytes {
			return false, false
		}

		for _, h := range filesystem.ScanContentBytes(e.Path, rel, data, needle, caseSensitive, limit-hits) {
			onHit(h)
			hits++
			if hits >= limit {
				truncated = true
				return false, true
			}
		}
		return false, false
	})
	if walkErr != nil {
		return truncated, walkErr
	}
	return truncated, nil
}
