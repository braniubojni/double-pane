package remote

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/cloudsoda/go-smb2"
	"github.com/erikharutyunyan/go-file-manager/internal/domain"
	"github.com/erikharutyunyan/go-file-manager/internal/filesystem"
)

// ListDir lists an SMB virtual path. The host root lists shares as directories.
func (m *SMBManager) ListDir(vpath string, showHidden bool) ([]domain.FileEntry, error) {
	loc, err := ParseLocation(vpath)
	if err != nil {
		return nil, err
	}
	if loc.ShareName() == "" {
		return m.listShareEntries(loc, showHidden)
	}
	fs, err := m.shareFS(loc)
	if err != nil {
		return nil, err
	}
	rel := smbRel(loc)
	entries, err := fs.ReadDir(rel)
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", rel, err)
	}

	result := make([]domain.FileEntry, 0, len(entries)+1)
	parent := ParentRemote(loc)
	result = append(result, domain.FileEntry{
		Name:  "..",
		Path:  parent.JoinPath(parent.RemotePath),
		IsDir: true,
	})
	for _, e := range entries {
		name := e.Name()
		if !showHidden && strings.HasPrefix(name, ".") {
			continue
		}
		child := joinOnShare(loc.ShareName(), rel, name)
		isDir := e.IsDir()
		ext := ""
		if !isDir {
			ext = strings.TrimPrefix(filepath.Ext(name), ".")
		}
		result = append(result, domain.FileEntry{
			Name:      name,
			Path:      loc.JoinPath(child),
			IsDir:     isDir,
			Size:      e.Size(),
			ModTime:   e.ModTime().UnixMilli(),
			Ext:       ext,
			IsSymlink: e.Mode()&os.ModeSymlink != 0,
		})
	}
	return result, nil
}

func (m *SMBManager) listShareEntries(loc Location, showHidden bool) ([]domain.FileEntry, error) {
	shares, err := m.ListShares(loc.SessionKey(), showHidden)
	if err != nil {
		return nil, err
	}
	result := make([]domain.FileEntry, 0, len(shares))
	for _, sh := range shares {
		result = append(result, domain.FileEntry{
			Name:  sh.Name,
			Path:  loc.JoinPath("/" + sh.Name),
			IsDir: true,
		})
	}
	return result, nil
}

// OpenRead returns a streaming reader for a file on an SMB share. Caller closes it.
func (m *SMBManager) OpenRead(vpath string) (io.ReadCloser, error) {
	loc, err := ParseLocation(vpath)
	if err != nil {
		return nil, err
	}
	fs, err := m.shareFS(loc)
	if err != nil {
		return nil, err
	}
	rel := smbRel(loc)
	f, err := fs.Open(rel)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", rel, err)
	}
	return f, nil
}

// Exists checks an SMB virtual path.
func (m *SMBManager) Exists(vpath string) (bool, error) {
	loc, err := ParseLocation(vpath)
	if err != nil {
		return false, err
	}
	if loc.ShareName() == "" {
		_, err := m.get(loc)
		return err == nil, err
	}
	fs, err := m.shareFS(loc)
	if err != nil {
		return false, err
	}
	_, err = fs.Stat(smbRel(loc))
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) || smbNotExist(err) {
		return false, nil
	}
	return false, err
}

// ReadTextFile reads a remote text file for the built-in editor.
func (m *SMBManager) ReadTextFile(vpath string) (string, error) {
	loc, err := ParseLocation(vpath)
	if err != nil {
		return "", err
	}
	fs, err := m.shareFS(loc)
	if err != nil {
		return "", err
	}
	rel := smbRel(loc)
	info, err := fs.Stat(rel)
	if err != nil {
		return "", fmt.Errorf("stat %s: %w", rel, err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("not a file: %s", rel)
	}
	f, err := fs.Open(rel)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	head := make([]byte, filesystem.HeadBytes)
	n, err := io.ReadFull(f, head)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return "", err
	}
	head = head[:n]
	if filesystem.IsExecutable(head) {
		return "", filesystem.ErrExecutable
	}
	if info.Size() > filesystem.MaxTextFileBytes {
		return "", filesystem.TooLargeError()
	}
	rest, err := io.ReadAll(f)
	if err != nil {
		return "", err
	}
	data := append(head, rest...)
	if !utf8.Valid(data) {
		return "", filesystem.EncodingError(loc.RemotePath)
	}
	return string(data), nil
}

// WriteTextFile writes remote text content atomically (temp + rename).
func (m *SMBManager) WriteTextFile(vpath, content string) error {
	loc, err := ParseLocation(vpath)
	if err != nil {
		return err
	}
	fs, err := m.shareFS(loc)
	if err != nil {
		return err
	}
	rel := smbRel(loc)
	if info, err := fs.Stat(rel); err == nil && info.IsDir() {
		return fmt.Errorf("not a file: %s", rel)
	}
	dir := path.Dir(rel)
	if dir == "." {
		dir = ""
	}
	tmpName := fmt.Sprintf(".gfm-edit-%d", time.Now().UnixNano())
	tmp := tmpName
	if dir != "" {
		tmp = path.Join(dir, tmpName)
	}
	if err := fs.WriteFile(tmp, []byte(content), 0o644); err != nil {
		return err
	}
	if err := fs.Rename(tmp, rel); err != nil {
		// Some servers reject overwrite rename; only then remove dest and retry.
		// Do not delete the original on unrelated rename failures (data loss).
		if !smbDestExists(err) {
			_ = fs.Remove(tmp)
			return err
		}
		if rmErr := fs.Remove(rel); rmErr != nil {
			_ = fs.Remove(tmp)
			return err
		}
		if err2 := fs.Rename(tmp, rel); err2 != nil {
			_ = fs.Remove(tmp)
			return err2
		}
	}
	return nil
}

// DirChildSizesCtx returns recursive byte sizes for each immediate child directory.
func (m *SMBManager) DirChildSizesCtx(ctx context.Context, vpath string) (domain.DirSizes, error) {
	empty := domain.DirSizes{Sizes: map[string]int64{}, Denied: []string{}}
	loc, err := ParseLocation(vpath)
	if err != nil {
		return empty, err
	}
	if loc.ShareName() == "" {
		return empty, nil
	}
	fs, err := m.shareFS(loc)
	if err != nil {
		return empty, err
	}
	rel := smbRel(loc)
	info, err := fs.Stat(rel)
	if err != nil {
		return empty, err
	}
	if !info.IsDir() {
		return empty, fmt.Errorf("not a directory: %s", rel)
	}
	entries, err := fs.ReadDir(rel)
	if err != nil {
		return empty, err
	}
	out := domain.DirSizes{Sizes: map[string]int64{}, Denied: []string{}}
	for _, e := range entries {
		if err := ctx.Err(); err != nil {
			return empty, err
		}
		if !e.IsDir() {
			continue
		}
		childRel := e.Name()
		if rel != "." {
			childRel = path.Join(rel, e.Name())
		}
		childV := loc.JoinPath(joinOnShare(loc.ShareName(), rel, e.Name()))
		size, denied, err := smbDirSizeCtx(ctx, fs, childRel)
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

func smbDirSizeCtx(ctx context.Context, fs *smb2.Share, root string) (total int64, denied bool, err error) {
	if err := ctx.Err(); err != nil {
		return 0, false, err
	}
	entries, err := fs.ReadDir(root)
	if err != nil {
		return 0, true, nil
	}
	for _, e := range entries {
		if err := ctx.Err(); err != nil {
			return total, denied, err
		}
		child := path.Join(root, e.Name())
		if e.IsDir() {
			sub, subDenied, err := smbDirSizeCtx(ctx, fs, child)
			if err != nil {
				return total, denied, err
			}
			total += sub
			denied = denied || subDenied
			continue
		}
		total += e.Size()
	}
	return total, denied, nil
}

// ListPathCompletions returns smb:// path suggestions (max 50).
func (m *SMBManager) ListPathCompletions(partial string) ([]string, error) {
	partial = strings.TrimSpace(partial)
	loc, err := ParseLocation(partial)
	if err != nil {
		return nil, err
	}
	if loc.ShareName() == "" {
		shares, err := m.ListShares(loc.SessionKey(), true)
		if err != nil {
			return []string{}, nil
		}
		out := make([]string, 0, len(shares))
		for _, sh := range shares {
			out = append(out, loc.JoinPath("/"+sh.Name+"/"))
			if len(out) >= 50 {
				break
			}
		}
		return out, nil
	}
	fs, err := m.shareFS(loc)
	if err != nil {
		return nil, err
	}
	rel := smbRel(loc)
	var dir, query string
	if strings.HasSuffix(loc.RemotePath, "/") {
		dir, query = rel, ""
	} else {
		dir, query = path.Dir(rel), path.Base(rel)
		if dir == "." && !strings.Contains(rel, "/") {
			dir = "."
		}
	}
	entries, err := fs.ReadDir(dir)
	if err != nil {
		return []string{}, nil
	}
	queryLower := strings.ToLower(query)
	type item struct {
		full       string
		name       string
		isDir      bool
		startsWith bool
		isDot      bool
	}
	items := make([]item, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		nameLower := strings.ToLower(name)
		if query != "" && !strings.Contains(nameLower, queryLower) {
			continue
		}
		child := joinOnShare(loc.ShareName(), dir, name)
		if e.IsDir() {
			child += "/"
		}
		items = append(items, item{
			full:       loc.JoinPath(child),
			name:       name,
			isDir:      e.IsDir(),
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
func (m *SMBManager) Mkdir(parentV, name string) (string, error) {
	loc, err := ParseLocation(parentV)
	if err != nil {
		return "", err
	}
	name = strings.TrimSpace(name)
	if name == "" || strings.ContainsAny(name, `/\`) {
		return "", fmt.Errorf("invalid name")
	}
	if loc.ShareName() == "" {
		return "", fmt.Errorf("cannot create a share")
	}
	fs, err := m.shareFS(loc)
	if err != nil {
		return "", err
	}
	rel := path.Join(smbRel(loc), name)
	if err := fs.Mkdir(rel, 0o755); err != nil {
		return "", err
	}
	return loc.JoinPath(joinOnShare(loc.ShareName(), smbRel(loc), name)), nil
}

// Rename renames a remote entry (newName is basename only).
func (m *SMBManager) Rename(oldV, newName string) (string, error) {
	loc, err := ParseLocation(oldV)
	if err != nil {
		return "", err
	}
	newName = strings.TrimSpace(newName)
	if newName == "" || strings.ContainsAny(newName, `/\`) {
		return "", fmt.Errorf("invalid name")
	}
	if loc.ShareName() == "" || loc.PathOnShare() == "." {
		return "", fmt.Errorf("cannot rename a share")
	}
	fs, err := m.shareFS(loc)
	if err != nil {
		return "", err
	}
	rel := smbRel(loc)
	next := path.Join(path.Dir(rel), newName)
	if err := fs.Rename(rel, next); err != nil {
		return "", err
	}
	dirRel := path.Dir(rel)
	return loc.JoinPath(joinOnShare(loc.ShareName(), dirRel, newName)), nil
}

// Delete removes remote paths (files or recursive dirs).
func (m *SMBManager) Delete(paths []string) error {
	for _, p := range paths {
		loc, err := ParseLocation(p)
		if err != nil {
			return err
		}
		if loc.ShareName() == "" || loc.PathOnShare() == "." {
			return fmt.Errorf("cannot delete a share")
		}
		fs, err := m.shareFS(loc)
		if err != nil {
			return err
		}
		if err := fs.RemoveAll(smbRel(loc)); err != nil {
			return err
		}
	}
	return nil
}

func smbNotExist(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "not exist") ||
		strings.Contains(msg, "no such file") ||
		strings.Contains(msg, "object name not found")
}

// smbDestExists reports rename/create failure because the destination already exists.
func smbDestExists(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, os.ErrExist) || os.IsExist(err) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "already exists") ||
		strings.Contains(msg, "object name collision") ||
		strings.Contains(msg, "file exists")
}
