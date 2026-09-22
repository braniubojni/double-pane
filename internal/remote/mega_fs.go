package remote

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/erikharutyunyan/double-pane/internal/domain"
	"github.com/erikharutyunyan/double-pane/internal/filesystem"
	mega "github.com/t3rm1n4l/go-mega"
)

func megaIsDir(n *mega.Node) bool {
	if n == nil {
		return false
	}
	t := n.GetType()
	return t == mega.FOLDER || t == mega.ROOT
}

func (m *MEGAManager) resolve(vpath string) (*megaSession, *mega.Node, Location, error) {
	loc, err := ParseLocation(vpath)
	if err != nil {
		return nil, nil, Location{}, err
	}
	sess, err := m.get(loc)
	if err != nil {
		return nil, nil, loc, err
	}
	if h := megaHandle(vpath); h != "" {
		n := sess.client.FS.HashLookup(h)
		if n == nil {
			return nil, nil, loc, fmt.Errorf("not found: %s", vpath)
		}
		return sess, n, loc, nil
	}
	n, err := walkMega(sess, loc.RemotePath)
	if err != nil {
		return sess, nil, loc, err
	}
	return sess, n, loc, nil
}

func walkMega(sess *megaSession, remotePath string) (*mega.Node, error) {
	n := sess.client.FS.GetRoot()
	if n == nil {
		return nil, fmt.Errorf("MEGA cloud drive unavailable")
	}
	p := strings.Trim(remotePath, "/")
	if p == "" {
		return n, nil
	}
	for seg := range strings.SplitSeq(p, "/") {
		kids, err := sess.client.FS.GetChildren(n)
		if err != nil {
			return nil, err
		}
		var found *mega.Node
		for _, k := range kids {
			if k.GetName() != seg {
				continue
			}
			if found == nil {
				found = k
			}
		}
		if found == nil {
			return nil, fmt.Errorf("not found: %s", remotePath)
		}
		n = found
	}
	return n, nil
}

func megaChildPath(loc Location, name string) string {
	p := strings.TrimSuffix(loc.RemotePath, "/")
	if p == "" || p == "/" {
		return "/" + name
	}
	return p + "/" + name
}

func megaEntryPath(loc Location, n *mega.Node, collide bool) string {
	p := loc.JoinPath(megaChildPath(loc, n.GetName()))
	if collide {
		p += "?h=" + url.QueryEscape(n.GetHash())
	}
	return p
}

// ListDir lists a MEGA virtual path (Cloud Drive tree).
func (m *MEGAManager) ListDir(vpath string, showHidden bool) ([]domain.FileEntry, error) {
	sess, node, loc, err := m.resolve(vpath)
	if err != nil {
		return nil, err
	}
	if !megaIsDir(node) {
		return nil, fmt.Errorf("not a directory: %s", loc.RemotePath)
	}
	kids, err := sess.client.FS.GetChildren(node)
	if err != nil {
		return nil, err
	}
	counts := map[string]int{}
	for _, k := range kids {
		if k.GetType() != mega.FILE && k.GetType() != mega.FOLDER {
			continue
		}
		counts[k.GetName()]++
	}
	result := make([]domain.FileEntry, 0, len(kids)+1)
	if strings.Trim(loc.RemotePath, "/") != "" {
		parent := ParentRemote(loc)
		result = append(result, domain.FileEntry{
			Name:  "..",
			Path:  parent.JoinPath(parent.RemotePath),
			IsDir: true,
		})
	}
	for _, k := range kids {
		if k.GetType() != mega.FILE && k.GetType() != mega.FOLDER {
			continue
		}
		name := k.GetName()
		if !showHidden && strings.HasPrefix(name, ".") {
			continue
		}
		isDir := k.GetType() == mega.FOLDER
		ext := ""
		if !isDir {
			ext = strings.TrimPrefix(filepath.Ext(name), ".")
		}
		result = append(result, domain.FileEntry{
			Name:    name,
			Path:    megaEntryPath(loc, k, counts[name] > 1),
			IsDir:   isDir,
			Size:    k.GetSize(),
			ModTime: k.GetTimeStamp().UnixMilli(),
			Ext:     ext,
		})
	}
	return result, nil
}

// Exists checks a MEGA virtual path.
func (m *MEGAManager) Exists(vpath string) (bool, error) {
	_, n, _, err := m.resolve(vpath)
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "not connected") {
			if strings.Contains(err.Error(), "not connected") {
				return false, err
			}
			return false, nil
		}
		return false, err
	}
	return n != nil, nil
}

// ReadTextFile downloads a MEGA file for the built-in editor.
func (m *MEGAManager) ReadTextFile(vpath string) (string, error) {
	sess, n, loc, err := m.resolve(vpath)
	if err != nil {
		return "", err
	}
	if megaIsDir(n) {
		return "", fmt.Errorf("not a file: %s", loc.RemotePath)
	}
	if n.GetSize() > filesystem.MaxTextFileBytes {
		return "", filesystem.TooLargeError()
	}
	tmp, err := os.CreateTemp("", "gfm-mega-read-*")
	if err != nil {
		return "", err
	}
	tmpPath := tmp.Name()
	_ = tmp.Close()
	defer func() { _ = os.Remove(tmpPath) }()
	if err := sess.client.DownloadFile(n, tmpPath, nil); err != nil {
		return "", err
	}
	data, err := os.ReadFile(tmpPath)
	if err != nil {
		return "", err
	}
	if filesystem.IsExecutable(data) {
		return "", filesystem.ErrExecutable
	}
	if !utf8.Valid(data) {
		return "", filesystem.EncodingError(loc.RemotePath)
	}
	return string(data), nil
}

// WriteTextFile uploads editor content, replacing the existing node when present.
func (m *MEGAManager) WriteTextFile(vpath, content string) error {
	sess, n, loc, err := m.resolve(vpath)
	notFound := err != nil && strings.Contains(err.Error(), "not found")
	if err != nil && !notFound {
		return err
	}
	if sess == nil {
		return err
	}
	if n != nil && megaIsDir(n) {
		return fmt.Errorf("not a file: %s", loc.RemotePath)
	}
	parent := ParentRemote(loc)
	pnode, err := walkMega(sess, parent.RemotePath)
	if err != nil {
		return err
	}
	name := path.Base(loc.RemotePath)
	tmp, err := os.CreateTemp("", "gfm-mega-write-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.WriteString(content); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	_ = tmp.Close()
	defer func() { _ = os.Remove(tmpPath) }()
	if n == nil {
		_, err = sess.client.UploadFile(tmpPath, pnode, name, nil)
		return err
	}
	tmpName := fmt.Sprintf(".gfm-edit-%d", os.Getpid())
	uploaded, err := sess.client.UploadFile(tmpPath, pnode, tmpName, nil)
	if err != nil {
		return err
	}
	_ = sess.client.Delete(n, true)
	return sess.client.Rename(uploaded, name)
}

// DirChildSizesCtx returns recursive byte sizes for each immediate child directory.
func (m *MEGAManager) DirChildSizesCtx(ctx context.Context, vpath string) (domain.DirSizes, error) {
	empty := domain.DirSizes{Sizes: map[string]int64{}, Denied: []string{}}
	sess, node, loc, err := m.resolve(vpath)
	if err != nil {
		return empty, err
	}
	if !megaIsDir(node) {
		return empty, fmt.Errorf("not a directory: %s", loc.RemotePath)
	}
	kids, err := sess.client.FS.GetChildren(node)
	if err != nil {
		return empty, err
	}
	counts := map[string]int{}
	for _, k := range kids {
		if k.GetType() == mega.FOLDER {
			counts[k.GetName()]++
		}
	}
	out := domain.DirSizes{Sizes: map[string]int64{}, Denied: []string{}}
	for _, k := range kids {
		if err := ctx.Err(); err != nil {
			return empty, err
		}
		if k.GetType() != mega.FOLDER {
			continue
		}
		childV := megaEntryPath(loc, k, counts[k.GetName()] > 1)
		size, denied, err := megaDirSize(ctx, sess, k)
		if err != nil {
			if ctx.Err() != nil {
				return empty, ctx.Err()
			}
			out.Sizes[childV] = size
			out.Denied = append(out.Denied, childV)
			continue
		}
		out.Sizes[childV] = size
		if denied {
			out.Denied = append(out.Denied, childV)
		}
	}
	return out, nil
}

func megaDirSize(ctx context.Context, sess *megaSession, n *mega.Node) (total int64, denied bool, err error) {
	if err := ctx.Err(); err != nil {
		return 0, false, err
	}
	kids, err := sess.client.FS.GetChildren(n)
	if err != nil {
		return 0, true, nil
	}
	for _, k := range kids {
		if err := ctx.Err(); err != nil {
			return total, denied, err
		}
		if k.GetType() == mega.FOLDER {
			sub, subDenied, err := megaDirSize(ctx, sess, k)
			if err != nil {
				return total, denied, err
			}
			total += sub
			denied = denied || subDenied
			continue
		}
		if k.GetType() == mega.FILE {
			total += k.GetSize()
		}
	}
	return total, denied, nil
}

// ListPathCompletions returns mega:// path suggestions (max 50).
func (m *MEGAManager) ListPathCompletions(partial string) ([]string, error) {
	partial = strings.TrimSpace(partial)
	loc, err := ParseLocation(partial)
	if err != nil {
		return nil, err
	}
	sess, err := m.get(loc)
	if err != nil {
		return nil, err
	}
	dirPath, query := loc.RemotePath, ""
	if !strings.HasSuffix(loc.RemotePath, "/") {
		dirPath = path.Dir(loc.RemotePath)
		query = path.Base(loc.RemotePath)
		if dirPath == "." {
			dirPath = "/"
		}
	}
	node, err := walkMega(sess, dirPath)
	if err != nil {
		return []string{}, nil
	}
	kids, err := sess.client.FS.GetChildren(node)
	if err != nil {
		return []string{}, nil
	}
	queryLower := strings.ToLower(query)
	dirLoc := loc
	dirLoc.RemotePath = dirPath
	type item struct {
		full, name               string
		isDir, startsWith, isDot bool
	}
	items := make([]item, 0, len(kids))
	for _, k := range kids {
		if k.GetType() != mega.FILE && k.GetType() != mega.FOLDER {
			continue
		}
		name := k.GetName()
		nameLower := strings.ToLower(name)
		if query != "" && !strings.Contains(nameLower, queryLower) {
			continue
		}
		child := megaEntryPath(dirLoc, k, false)
		if k.GetType() == mega.FOLDER {
			child += "/"
		}
		items = append(items, item{
			full:       child,
			name:       name,
			isDir:      k.GetType() == mega.FOLDER,
			startsWith: query == "" || strings.HasPrefix(nameLower, queryLower),
			isDot:      strings.HasPrefix(name, "."),
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]
		if a.isDot != b.isDot {
			return !a.isDot
		}
		if a.startsWith != b.startsWith {
			return a.startsWith
		}
		if a.isDir != b.isDir {
			return a.isDir
		}
		return strings.ToLower(a.name) < strings.ToLower(b.name)
	})
	if len(items) > 50 {
		items = items[:50]
	}
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.full
	}
	return out, nil
}

// Mkdir creates a directory under parent virtual path.
func (m *MEGAManager) Mkdir(parentV, name string) (string, error) {
	sess, node, loc, err := m.resolve(parentV)
	if err != nil {
		return "", err
	}
	name = strings.TrimSpace(name)
	if name == "" || strings.ContainsAny(name, `/\`) {
		return "", fmt.Errorf("invalid name")
	}
	if !megaIsDir(node) {
		return "", fmt.Errorf("not a directory")
	}
	created, err := sess.client.CreateDir(name, node)
	if err != nil {
		return "", err
	}
	return megaEntryPath(loc, created, false), nil
}

// Rename renames a remote entry (newName is basename only).
func (m *MEGAManager) Rename(oldV, newName string) (string, error) {
	sess, n, loc, err := m.resolve(oldV)
	if err != nil {
		return "", err
	}
	newName = strings.TrimSpace(newName)
	if newName == "" || strings.ContainsAny(newName, `/\`) {
		return "", fmt.Errorf("invalid name")
	}
	if strings.Trim(loc.RemotePath, "/") == "" {
		return "", fmt.Errorf("cannot rename cloud drive")
	}
	if err := sess.client.Rename(n, newName); err != nil {
		return "", err
	}
	parent := ParentRemote(loc)
	return parent.JoinPath(megaChildPath(parent, newName)), nil
}

// Delete moves MEGA nodes to MEGA trash (not our local trash).
func (m *MEGAManager) Delete(paths []string) error {
	for _, p := range paths {
		sess, n, loc, err := m.resolve(p)
		if err != nil {
			return err
		}
		if strings.Trim(loc.RemotePath, "/") == "" {
			return fmt.Errorf("cannot delete cloud drive")
		}
		if err := sess.client.Delete(n, false); err != nil {
			return err
		}
	}
	return nil
}

func uniqueMegaName(kids []*mega.Node, want string) string {
	taken := map[string]bool{}
	for _, k := range kids {
		taken[k.GetName()] = true
	}
	if !taken[want] {
		return want
	}
	ext := path.Ext(want)
	stem := strings.TrimSuffix(want, ext)
	for i := 1; ; i++ {
		cand := fmt.Sprintf("%s (%d)%s", stem, i, ext)
		if !taken[cand] {
			return cand
		}
	}
}
