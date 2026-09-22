package filesystem

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/erikharutyunyan/double-pane/internal/domain"
)

const (
	defaultSearchLimit = 80
	maxSearchVisits    = 5000
	maxSearchDepth     = 12
)

var errSearchStop = errors.New("search stop")

// SearchTree finds files/folders under root whose names contain query (case-insensitive).
func SearchTree(root, query string, showHidden bool, limit int) ([]domain.SearchHit, error) {
	abs, err := Resolve(root)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("not a directory: %s", abs)
	}
	if limit <= 0 {
		limit = defaultSearchLimit
	}
	q := strings.ToLower(strings.TrimSpace(query))

	type hit struct {
		domain.SearchHit
		starts bool
		isDot  bool
	}
	type pendingDir struct {
		path  string
		rel   string
		depth int
	}
	var hits []hit
	visits := 0

	queue := []pendingDir{{path: abs, rel: "", depth: -1}}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		entries, readErr := os.ReadDir(current.path)
		if readErr != nil {
			// Skip unreadable subtrees (permissions, transient IO errors) and keep searching.
			continue
		}
		sort.SliceStable(entries, func(i, j int) bool {
			return strings.ToLower(entries[i].Name()) < strings.ToLower(entries[j].Name())
		})

		for _, entry := range entries {
			visits++
			if visits > maxSearchVisits || len(hits) >= limit*3 {
				err = errSearchStop
				break
			}

			name := entry.Name()
			if !showHidden && strings.HasPrefix(name, ".") {
				continue
			}

			depth := current.depth + 1
			if depth > maxSearchDepth {
				continue
			}

			rel := name
			if current.rel != "" {
				rel = filepath.Join(current.rel, name)
			}
			path := filepath.Join(current.path, name)

			if q == "" {
				if depth == 0 {
					hits = append(hits, hit{
						SearchHit: domain.SearchHit{
							Name:    name,
							Path:    path,
							IsDir:   entry.IsDir(),
							RelPath: filepath.ToSlash(rel),
						},
						starts: false,
						isDot:  strings.HasPrefix(name, "."),
					})
				}
			} else if strings.Contains(strings.ToLower(name), q) {
				hits = append(hits, hit{
					SearchHit: domain.SearchHit{
						Name:    name,
						Path:    path,
						IsDir:   entry.IsDir(),
						RelPath: filepath.ToSlash(rel),
					},
					starts: strings.HasPrefix(strings.ToLower(name), q),
					isDot:  strings.HasPrefix(name, "."),
				})
			}

			// Empty query: immediate children only — do not walk the tree.
			if q != "" && entry.IsDir() && depth < maxSearchDepth {
				queue = append(queue, pendingDir{path: path, rel: rel, depth: depth})
			}
		}
		if err != nil {
			break
		}
	}
	if err != nil && !errors.Is(err, errSearchStop) {
		return nil, err
	}

	out := make([]domain.SearchHit, len(hits))
	for i := range hits {
		out[i] = hits[i].SearchHit
	}
	return RankSearchHits(out, q, limit), nil
}

// RankSearchHits sorts go-to hits (dot-files last, name-starts-with-query
// first, dirs before files, then alphabetically) and truncates to limit.
// query should already be lowercased/trimmed; shared by the local walker
// above and the remote walker in internal/service/remote_walk.go.
func RankSearchHits(hits []domain.SearchHit, query string, limit int) []domain.SearchHit {
	if limit <= 0 {
		limit = defaultSearchLimit
	}
	type ranked struct {
		domain.SearchHit
		starts bool
		isDot  bool
	}
	rs := make([]ranked, len(hits))
	for i, h := range hits {
		rs[i] = ranked{
			SearchHit: h,
			starts:    strings.HasPrefix(strings.ToLower(h.Name), query),
			isDot:     strings.HasPrefix(h.Name, "."),
		}
	}
	sort.SliceStable(rs, func(i, j int) bool {
		a, b := rs[i], rs[j]
		if a.isDot != b.isDot {
			return !a.isDot
		}
		if a.starts != b.starts {
			return a.starts
		}
		if a.IsDir != b.IsDir {
			return a.IsDir
		}
		return strings.ToLower(a.Name) < strings.ToLower(b.Name)
	})
	if len(rs) > limit {
		rs = rs[:limit]
	}
	out := make([]domain.SearchHit, len(rs))
	for i := range rs {
		out[i] = rs[i].SearchHit
	}
	return out
}
