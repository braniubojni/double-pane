package filesystem

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
	"unicode/utf8"

	"github.com/erikharutyunyan/double-pane/internal/domain"
	"github.com/mholt/archives"
)

// ErrArchiveReadOnly is returned for write operations on members inside an archive.
var ErrArchiveReadOnly = errors.New("archives are read-only; use Archive… or Extract here")

var browsableArchiveSuffixes = []string{
	".tar.gz", ".tar.bz2", ".tar.xz", ".tar.zst", ".tar.zstd",
	".tar.lz4", ".tar.sz", ".tar.snappy",
	".tbz2", ".tbz", ".tgz", ".txz",
	".zip", ".rar", ".7z", ".tar",
}

// SplitArchivePath walks prefixes of path and returns the archive file plus the
// inner member path (slash-separated). ok is true when a browsable archive file
// is a prefix of path. inner is empty when path is the archive file itself.
func SplitArchivePath(raw string) (archive, inner string, ok bool) {
	if strings.TrimSpace(raw) == "" || !looksLikeArchivePath(raw) {
		return "", "", false
	}
	abs, err := Resolve(raw)
	if err != nil {
		return "", "", false
	}
	p := abs
	for {
		info, err := os.Lstat(p)
		if err == nil && info.Mode().IsRegular() && IsBrowsableArchiveName(p) {
			rel, relErr := filepath.Rel(p, abs)
			if relErr != nil {
				return "", "", false
			}
			inner = ""
			if rel != "." {
				inner = normalizeInner(rel)
			}
			return p, inner, true
		}
		parent := filepath.Dir(p)
		if parent == p {
			return "", "", false
		}
		p = parent
	}
}

// IsArchivePath reports whether path is an archive file or a member inside one.
func IsArchivePath(raw string) bool {
	_, _, ok := SplitArchivePath(raw)
	return ok
}

// IsInsideArchive reports whether path is a member inside an archive (not the file itself).
func IsInsideArchive(raw string) bool {
	_, inner, ok := SplitArchivePath(raw)
	return ok && inner != ""
}

// LocalShellDir returns a directory the OS shell can cd into for path.
func LocalShellDir(raw string) string {
	if a, _, ok := SplitArchivePath(raw); ok {
		return filepath.Dir(a)
	}
	return raw
}

// IsBrowsableArchiveName reports whether basename looks like a zip/tar/7z/rar archive.
func IsBrowsableArchiveName(p string) bool {
	name := strings.ToLower(filepath.Base(p))
	for _, s := range browsableArchiveSuffixes {
		if strings.HasSuffix(name, s) {
			return true
		}
	}
	return false
}

func looksLikeArchivePath(raw string) bool {
	lower := strings.ToLower(raw)
	for _, s := range browsableArchiveSuffixes {
		if strings.Contains(lower, s) {
			return true
		}
	}
	return false
}

func normalizeInner(inner string) string {
	inner = strings.ReplaceAll(inner, `\`, `/`)
	inner = path.Clean("/" + inner)
	inner = strings.TrimPrefix(inner, "/")
	if inner == "." {
		return ""
	}
	return inner
}

func memberName(nameInArchive string) (string, bool) {
	raw := strings.ReplaceAll(nameInArchive, `\`, `/`)
	for _, seg := range strings.Split(raw, "/") {
		if seg == ".." {
			// Reject only real path-traversal segments; a filename that merely
			// contains ".." (e.g. "notes..bak.txt") is legitimate.
			return "", false
		}
	}
	name := normalizeInner(raw)
	if name == "" {
		return "", false
	}
	return name, true
}

func relToPrefix(name, prefix string) (string, bool) {
	if prefix == "" {
		return name, true
	}
	if name == prefix {
		return "", true
	}
	if strings.HasPrefix(name, prefix+"/") {
		return name[len(prefix)+1:], true
	}
	return "", false
}

// ListArchiveDir lists immediate children of inner inside archiveAbs.
// Names/sizes are readable from the central directory without a password
// even when the zip is encrypted; password is only needed to read content.
func ListArchiveDir(archiveAbs, inner string, showHidden bool) ([]domain.FileEntry, error) {
	inner = normalizeInner(inner)
	type child struct {
		isDir   bool
		size    int64
		modTime int64
	}
	children := map[string]*child{}

	err := walkArchiveNames(archiveAbs, func(fi archives.FileInfo) error {
		name, ok := memberName(fi.NameInArchive)
		if !ok {
			return nil
		}
		rel, ok := relToPrefix(name, inner)
		if !ok || rel == "" {
			return nil
		}
		first, rest, _ := strings.Cut(rel, "/")
		if first == "" || first == "." || first == ".." {
			return nil
		}
		if !showHidden && strings.HasPrefix(first, ".") {
			return nil
		}
		isDir := rest != "" || fi.IsDir()
		c := children[first]
		if c == nil {
			c = &child{isDir: isDir, size: 0, modTime: fi.ModTime().UnixMilli()}
			children[first] = c
		}
		if isDir {
			c.isDir = true
		}
		if !isDir {
			c.size = fi.Size()
			c.modTime = fi.ModTime().UnixMilli()
		} else if fi.ModTime().UnixMilli() > c.modTime {
			c.modTime = fi.ModTime().UnixMilli()
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	parent := filepath.Dir(archiveAbs)
	if inner != "" {
		parent = joinArchivePath(archiveAbs, path.Dir(inner))
		if path.Dir(inner) == "." {
			parent = archiveAbs
		}
	}
	result := []domain.FileEntry{{
		Name:   "..",
		Path:   parent,
		IsDir:  true,
		Access: "readonly",
	}}
	for name, c := range children {
		ext := ""
		if !c.isDir {
			ext = strings.TrimPrefix(filepath.Ext(name), ".")
		}
		result = append(result, domain.FileEntry{
			Name:    name,
			Path:    joinArchivePath(archiveAbs, path.Join(inner, name)),
			IsDir:   c.isDir,
			Size:    c.size,
			ModTime: c.modTime,
			Ext:     ext,
			Access:  "readonly",
		})
	}
	sort.SliceStable(result, func(i, j int) bool {
		a, b := result[i], result[j]
		if a.Name == ".." {
			return true
		}
		if b.Name == ".." {
			return false
		}
		if a.IsDir != b.IsDir {
			return a.IsDir
		}
		return strings.ToLower(a.Name) < strings.ToLower(b.Name)
	})
	return result, nil
}

func joinArchivePath(archiveAbs, inner string) string {
	inner = normalizeInner(inner)
	if inner == "" {
		return archiveAbs
	}
	return filepath.Join(archiveAbs, filepath.FromSlash(inner))
}

// ArchiveMemberExists reports whether inner exists as a file or virtual directory.
func ArchiveMemberExists(archiveAbs, inner string) (bool, error) {
	inner = normalizeInner(inner)
	if inner == "" {
		return Exists(archiveAbs)
	}
	found := false
	err := walkArchiveNames(archiveAbs, func(fi archives.FileInfo) error {
		name, ok := memberName(fi.NameInArchive)
		if !ok {
			return nil
		}
		if name == inner || strings.HasPrefix(name, inner+"/") {
			found = true
		}
		return nil
	})
	return found, err
}

// ReadArchiveTextFile reads a text member for the built-in editor. password
// is required when the archive member is encrypted (see ErrPasswordRequired).
func ReadArchiveTextFile(archiveAbs, inner, password string) (string, error) {
	inner = normalizeInner(inner)
	if inner == "" {
		return "", fmt.Errorf("not a file: %s", archiveAbs)
	}
	var data []byte
	var found bool
	err := walkArchive(context.Background(), archiveAbs, password, func(_ context.Context, fi archives.FileInfo) error {
		if found {
			return nil
		}
		name, ok := memberName(fi.NameInArchive)
		if !ok || name != inner || fi.IsDir() {
			return nil
		}
		found = true
		rc, err := fi.Open()
		if err != nil {
			return err
		}
		defer func() { _ = rc.Close() }()
		head := make([]byte, HeadBytes)
		n, err := io.ReadFull(rc, head)
		if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
			return err
		}
		head = head[:n]
		if IsExecutable(head) {
			return ErrExecutable
		}
		rest, err := io.ReadAll(io.LimitReader(rc, MaxTextFileBytes+1))
		if err != nil {
			return err
		}
		all := append(head, rest...)
		if int64(len(all)) > MaxTextFileBytes {
			return TooLargeError()
		}
		if !utf8.Valid(all) {
			return EncodingError(joinArchivePath(archiveAbs, inner))
		}
		data = all
		return nil
	})
	if err != nil {
		return "", err
	}
	if !found {
		return "", fmt.Errorf("%w: %s", ErrNotFound, joinArchivePath(archiveAbs, inner))
	}
	return string(data), nil
}

// ExtractMembers copies selected archive members (files or folders) into destDir.
func ExtractMembers(ctx context.Context, archiveAbs, destDir string, inners []string, password string) error {
	destAbs, err := Resolve(destDir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(destAbs, 0o755); err != nil {
		return err
	}
	want := make([]string, 0, len(inners))
	for _, in := range inners {
		n := normalizeInner(in)
		if n != "" {
			want = append(want, n)
		}
	}
	if len(want) == 0 {
		return fmt.Errorf("no archive members to extract")
	}
	// destRoot[inner] = unique dest path for that selection's basename
	destRoot := map[string]string{}
	for _, w := range want {
		base := path.Base(w)
		destRoot[w] = UniquePath(filepath.Join(destAbs, base))
	}
	return walkArchive(ctx, archiveAbs, password, func(ctx context.Context, fi archives.FileInfo) error {
		name, ok := memberName(fi.NameInArchive)
		if !ok {
			return nil
		}
		for _, w := range want {
			if name == w {
				if fi.IsDir() {
					return os.MkdirAll(destRoot[w], 0o755)
				}
				return writeArchiveMember(ctx, filepath.Dir(destRoot[w]), filepath.Base(destRoot[w]), fi, nil)
			}
			if strings.HasPrefix(name, w+"/") {
				rel := name[len(w)+1:]
				root := destRoot[w]
				if fi.IsDir() {
					return os.MkdirAll(filepath.Join(root, filepath.FromSlash(rel)), 0o755)
				}
				return writeArchiveMember(ctx, root, filepath.FromSlash(rel), fi, nil)
			}
		}
		return nil
	})
}
