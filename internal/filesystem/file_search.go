package filesystem

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/erikharutyunyan/double-pane/internal/domain"
)

// SearchFiles finds files under root whose basenames contain query (case-insensitive).
// Walk limits, hidden/symlink skips, and include/exclude globs match SearchFolders.
// An empty query emits nothing.
func SearchFiles(
	ctx context.Context,
	root, query, include, exclude string,
	showHidden bool,
	limit int,
	hitFilter domain.SearchFilter,
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

	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return false, nil
	}

	filter := NewPathFilter(include, exclude)

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
			info, infoErr := entry.Info()
			if infoErr == nil && info.Mode()&os.ModeSymlink != 0 {
				continue
			}

			rel := name
			if current.rel != "" {
				rel = filepath.ToSlash(filepath.Join(current.rel, name))
			}
			full := filepath.Join(current.path, name)

			if entry.IsDir() {
				if !filter.MatchDir(rel) {
					continue
				}
				queue = append(queue, pendingDir{path: full, rel: rel})
				continue
			}

			if !filter.Match(rel) || !strings.Contains(strings.ToLower(name), q) {
				continue
			}
			var size, modTime int64
			if infoErr == nil {
				size = info.Size()
				modTime = info.ModTime().UnixMilli()
			}
			if !SearchCandidateAllowed(hitFilter, name, false, size, modTime) {
				continue
			}
			if hitFilter.Type == "text" {
				text, err := fileIsText(full)
				if err != nil {
					if os.IsPermission(err) && cb.OnDenied != nil {
						cb.OnDenied(full, err)
					}
					continue
				}
				if !text {
					continue
				}
			}
			if cb.OnHit != nil {
				cb.OnHit(domain.SearchHit{
					Name:    name,
					Path:    full,
					IsDir:   false,
					RelPath: rel,
					Size:    size,
					ModTime: modTime,
				})
			}
			hits++
			if hits >= limit {
				return true, nil
			}
		}
	}
	return false, nil
}
