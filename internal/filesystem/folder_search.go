package filesystem

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/erikharutyunyan/double-pane/internal/domain"
)

const (
	defaultFolderSearchLimit = 500
	maxFolderSearchVisits    = 100_000
)

// FolderSearchCallbacks receive streaming folder-name hits.
type FolderSearchCallbacks struct {
	OnHit    func(domain.SearchHit)
	OnDenied func(path string, err error)
}

// SearchFolders finds directories under root whose names contain query (case-insensitive).
// include/exclude use the same simple globs as content search (matched against relative path).
func SearchFolders(
	ctx context.Context,
	root, query, include, exclude string,
	showHidden bool,
	limit int,
	cb FolderSearchCallbacks,
) (truncated bool, err error) {
	abs, err := Resolve(root)
	if err != nil {
		return false, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return false, ErrNotFound
		}
		return false, err
	}
	if !info.IsDir() {
		return false, fmt.Errorf("not a directory: %s", abs)
	}
	if limit <= 0 {
		limit = defaultFolderSearchLimit
	}

	filter := NewPathFilter(include, exclude)
	q := strings.ToLower(strings.TrimSpace(query))

	type pendingDir struct {
		path string
		rel  string
	}
	queue := []pendingDir{{path: abs, rel: ""}}
	visits := 0
	hits := 0

	for len(queue) > 0 {
		if ctx.Err() != nil {
			return truncated, nil
		}
		if hits >= limit || visits > maxFolderSearchVisits {
			return true, nil
		}

		current := queue[0]
		queue = queue[1:]

		entries, readErr := os.ReadDir(current.path)
		if readErr != nil {
			if os.IsPermission(readErr) && cb.OnDenied != nil {
				cb.OnDenied(current.path, readErr)
			}
			continue
		}

		for _, entry := range entries {
			if ctx.Err() != nil {
				return truncated, nil
			}
			visits++
			if visits > maxFolderSearchVisits || hits >= limit {
				return true, nil
			}

			name := entry.Name()
			if !showHidden && strings.HasPrefix(name, ".") {
				continue
			}

			// Skip symlinks
			if info, err := entry.Info(); err == nil && info.Mode()&os.ModeSymlink != 0 {
				continue
			}
			if !entry.IsDir() {
				continue
			}

			rel := name
			if current.rel != "" {
				rel = filepath.ToSlash(filepath.Join(current.rel, name))
			}
			full := filepath.Join(current.path, name)

			if !filter.MatchDir(rel) {
				continue
			}

			// Emit only when include allows this folder path (exclude already via MatchDir).
			if filter.Match(rel) {
				if q == "" || strings.Contains(strings.ToLower(name), q) {
					if cb.OnHit != nil {
						cb.OnHit(domain.SearchHit{
							Name:    name,
							Path:    full,
							IsDir:   true,
							RelPath: rel,
						})
					}
					hits++
					if hits >= limit {
						return true, nil
					}
				}
			}

			// Always walk children unless directory is excluded (MatchDir).
			queue = append(queue, pendingDir{path: full, rel: rel})
		}
	}
	return false, nil
}
