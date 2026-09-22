package filesystem

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"unicode/utf8"

	"github.com/erikharutyunyan/double-pane/internal/domain"
)

const (
	// DefaultContentHitLimit is the hit cap when callers pass limit <= 0.
	DefaultContentHitLimit = 2000
	// MaxContentFileBytes skips files larger than this for content search.
	MaxContentFileBytes     = 2 << 20 // 2 MiB
	maxContentSearchVisits  = 100_000
	contentBinarySniffBytes = 8192
)

// ContentSearchCallbacks receive streaming results from SearchContent.
type ContentSearchCallbacks struct {
	OnHit    func(domain.ContentSearchHit)
	OnDenied func(path string, err error)
}

// SearchContent walks root and finds literal text matches in files.
func SearchContent(
	ctx context.Context,
	root, query, include, exclude string,
	showHidden, caseSensitive bool,
	limit int,
	cb ContentSearchCallbacks,
) (truncated bool, err error) {
	if strings.TrimSpace(query) == "" {
		return false, nil
	}
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
		limit = DefaultContentHitLimit
	}

	filter := NewPathFilter(include, exclude)
	needle := query
	if !caseSensitive {
		needle = strings.ToLower(query)
	}

	type fileJob struct {
		path string
		rel  string
	}

	workers := runtime.NumCPU()
	if workers < 2 {
		workers = 2
	}
	if workers > 8 {
		workers = 8
	}

	jobs := make(chan fileJob, workers*4)
	var (
		hitCount  atomic.Int64
		visits    atomic.Int64
		truncFlag atomic.Bool
		wg        sync.WaitGroup
	)

	emitHit := func(h domain.ContentSearchHit) bool {
		if int(hitCount.Load()) >= limit {
			truncFlag.Store(true)
			return false
		}
		n := hitCount.Add(1)
		if int(n) > limit {
			truncFlag.Store(true)
			return false
		}
		if cb.OnHit != nil {
			cb.OnHit(h)
		}
		return true
	}

	scanFile := func(job fileJob) {
		if ctx.Err() != nil || truncFlag.Load() {
			return
		}
		hits, err := scanFileContent(job.path, job.rel, needle, caseSensitive, limit-int(hitCount.Load()))
		if err != nil {
			if os.IsPermission(err) && cb.OnDenied != nil {
				cb.OnDenied(job.path, err)
			}
			return
		}
		for _, h := range hits {
			if !emitHit(h) {
				return
			}
		}
	}

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				scanFile(job)
			}
		}()
	}

	type pendingDir struct {
		path string
		rel  string
	}
	queue := []pendingDir{{path: abs, rel: ""}}

walk:
	for len(queue) > 0 {
		if ctx.Err() != nil {
			break
		}
		if truncFlag.Load() || int(hitCount.Load()) >= limit {
			truncFlag.Store(true)
			break
		}
		if int(visits.Load()) > maxContentSearchVisits {
			truncFlag.Store(true)
			break
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
				break walk
			}
			v := visits.Add(1)
			if int(v) > maxContentSearchVisits {
				truncFlag.Store(true)
				break walk
			}

			name := entry.Name()
			if !showHidden && strings.HasPrefix(name, ".") {
				continue
			}

			rel := name
			if current.rel != "" {
				rel = filepath.ToSlash(filepath.Join(current.rel, name))
			}
			full := filepath.Join(current.path, name)

			// Prefer Dir() without following symlinks for type checks.
			info, err := entry.Info()
			if err != nil {
				continue
			}
			if info.Mode()&os.ModeSymlink != 0 {
				// Do not follow symlink directories; skip symlink files for content.
				continue
			}

			if entry.IsDir() {
				if !filter.MatchDir(rel) {
					continue
				}
				queue = append(queue, pendingDir{path: full, rel: rel})
				continue
			}

			if !filter.Match(rel) {
				continue
			}
			if info.Size() > MaxContentFileBytes || info.Size() == 0 {
				continue
			}

			select {
			case <-ctx.Done():
				break walk
			case jobs <- fileJob{path: full, rel: rel}:
			}
		}
	}

	close(jobs)
	wg.Wait()
	if ctx.Err() != nil && ctx.Err() != context.Canceled {
		return truncFlag.Load(), ctx.Err()
	}
	return truncFlag.Load(), nil
}

func scanFileContent(path, rel, needle string, caseSensitive bool, remaining int) ([]domain.ContentSearchHit, error) {
	if remaining <= 0 {
		return nil, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	// Bounded read — walk already skips files > MaxContentFileBytes.
	data, err := io.ReadAll(io.LimitReader(f, MaxContentFileBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > MaxContentFileBytes {
		return nil, nil
	}
	return ScanContentBytes(path, rel, data, needle, caseSensitive, remaining), nil
}

// ScanContentBytes finds literal text matches in data. Binary files (NUL in the
// first sniff window) yield no hits. Shared by local and remote content search.
func ScanContentBytes(path, rel string, data []byte, needle string, caseSensitive bool, remaining int) []domain.ContentSearchHit {
	if remaining <= 0 || len(data) == 0 {
		return nil
	}
	sniff := data
	if len(sniff) > contentBinarySniffBytes {
		sniff = sniff[:contentBinarySniffBytes]
	}
	if bytes.IndexByte(sniff, 0) >= 0 {
		return nil
	}

	var hits []domain.ContentSearchHit
	scanner := bufio.NewScanner(bytes.NewReader(data))
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1<<20)

	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := scanner.Text()
		if !utf8.ValidString(line) {
			return hits // stop this file
		}
		searchLine := line
		if !caseSensitive {
			searchLine = strings.ToLower(line)
		}
		from := 0
		for remaining > 0 {
			idx := strings.Index(searchLine[from:], needle)
			if idx < 0 {
				break
			}
			start := from + idx
			end := start + len(needle)
			// For case-insensitive, needle length is in lowercased bytes; map via same byte indices
			// since ToLower on ASCII-heavy code keeps length for common cases. For multi-byte
			// we still use byte indices on the original line of equal length when possible.
			matchStart, matchEnd := start, end
			if !caseSensitive && len(line) != len(searchLine) {
				matchStart, matchEnd = indexFold(line, needle, from)
				if matchStart < 0 {
					break
				}
				end = matchEnd
			}
			lineText, ms, me := windowLinePreview(line, matchStart, matchEnd, 240)
			hits = append(hits, domain.ContentSearchHit{
				Path:       path,
				RelPath:    rel,
				Line:       lineNo,
				Column:     matchStart + 1, // full-line column for replace/open
				LineText:   lineText,
				MatchStart: ms,
				MatchEnd:   me,
			})
			remaining--
			from = end
			if from >= len(searchLine) {
				break
			}
		}
		if remaining <= 0 {
			break
		}
	}
	_ = scanner.Err()
	return hits
}

// indexFold finds needle (already lowercased) in s starting at from using EqualFold per rune span.
func indexFold(s, needleLower string, from int) (start, end int) {
	if from > len(s) {
		return -1, -1
	}
	// Simple path: if same length mapping, use lower of s
	lower := strings.ToLower(s)
	if len(lower) == len(s) {
		idx := strings.Index(lower[from:], needleLower)
		if idx < 0 {
			return -1, -1
		}
		st := from + idx
		return st, st + len(needleLower)
	}
	// Rare unequal lower mapping: linear scan
	nrunes := []rune(needleLower)
	srunes := []rune(s)
	// convert from byte to rune index approximately
	ri := utf8.RuneCountInString(s[:from])
	for i := ri; i+len(nrunes) <= len(srunes); i++ {
		ok := true
		for j := 0; j < len(nrunes); j++ {
			if !strings.EqualFold(string(srunes[i+j]), string(nrunes[j])) {
				ok = false
				break
			}
		}
		if ok {
			// map rune indices back to bytes
			prefix := string(srunes[:i])
			matched := string(srunes[i : i+len(nrunes)])
			st := len(prefix)
			return st, st + len(matched)
		}
	}
	return -1, -1
}

// windowLinePreview returns a ≤max-byte window of line that includes the match,
// with MatchStart/MatchEnd adjusted to be indices into the returned preview.
// Column in ContentSearchHit remains based on the full line; only LineText is windowed.
func windowLinePreview(line string, matchStart, matchEnd, max int) (preview string, ms, me int) {
	if max <= 0 {
		max = 240
	}
	if matchStart < 0 {
		matchStart = 0
	}
	if matchEnd < matchStart {
		matchEnd = matchStart
	}
	if matchEnd > len(line) {
		matchEnd = len(line)
	}
	if matchStart > len(line) {
		matchStart = len(line)
	}
	if len(line) <= max {
		return line, matchStart, matchEnd
	}

	// Prefer ~80 bytes of context before the match.
	startIdx := matchStart - 80
	if startIdx < 0 {
		startIdx = 0
	}
	endIdx := startIdx + max
	if endIdx > len(line) {
		endIdx = len(line)
		if endIdx-startIdx > max {
			startIdx = endIdx - max
		}
	}
	// Keep the match inside the window when possible.
	if matchEnd > endIdx {
		endIdx = matchEnd
		if endIdx > len(line) {
			endIdx = len(line)
		}
		startIdx = endIdx - max
		if startIdx < 0 {
			startIdx = 0
		}
	}
	if matchStart < startIdx {
		startIdx = matchStart
		endIdx = startIdx + max
		if endIdx > len(line) {
			endIdx = len(line)
		}
	}

	// Align window edges to UTF-8 rune boundaries.
	for startIdx > 0 && (line[startIdx]&0xC0) == 0x80 {
		startIdx--
	}
	for endIdx < len(line) && endIdx > 0 && (line[endIdx]&0xC0) == 0x80 {
		endIdx--
	}
	if endIdx < startIdx {
		endIdx = startIdx
	}

	core := line[startIdx:endIdx]
	prefix := ""
	suffix := ""
	if startIdx > 0 {
		prefix = "…"
	}
	if endIdx < len(line) {
		suffix = "…"
	}
	preview = prefix + core + suffix
	ms = matchStart - startIdx + len(prefix)
	me = matchEnd - startIdx + len(prefix)
	if ms < 0 {
		ms = 0
	}
	if me < ms {
		me = ms
	}
	if me > len(preview) {
		me = len(preview)
	}
	if ms > len(preview) {
		ms = len(preview)
	}
	return preview, ms, me
}
