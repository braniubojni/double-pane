package remote

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/erikharutyunyan/double-pane/internal/domain"
	"github.com/erikharutyunyan/double-pane/internal/filesystem"
)

type ftpEntry struct {
	name    string
	isDir   bool
	size    int64
	mod     time.Time
	symlink bool
}

type ftpReadCloser struct {
	s    *ftpSession
	data net.Conn
	once sync.Once
	err  error
}

func (r *ftpReadCloser) Read(p []byte) (int, error) {
	return r.data.Read(p)
}

func (r *ftpReadCloser) Close() error {
	r.once.Do(func() {
		_ = r.data.Close()
		rep, err := r.s.readReply()
		if err != nil {
			r.err = err
		} else if rep.code/100 != 2 {
			r.err = &ftpStatusError{code: rep.code, text: rep.text}
		}
		r.s.mu.Unlock()
	})
	return r.err
}

// OpenRead streams a remote file. The caller must Close it.
func (m *FTPManager) OpenRead(vpath string) (io.ReadCloser, error) {
	loc, s, err := m.session(vpath)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	data, err := s.startRead("RETR", cleanFTPPath(loc.RemotePath))
	if err != nil {
		s.mu.Unlock()
		return nil, err
	}
	return &ftpReadCloser{s: s, data: data}, nil
}

// ListDir lists one FTP directory.
func (m *FTPManager) ListDir(vpath string, showHidden bool) ([]domain.FileEntry, error) {
	loc, s, err := m.session(vpath)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rp := cleanFTPPath(loc.RemotePath)
	entries, err := s.list(rp)
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", rp, err)
	}
	result := make([]domain.FileEntry, 0, len(entries)+1)
	if rp != "/" {
		parent := ParentRemote(loc)
		result = append(result, domain.FileEntry{
			Name:  "..",
			Path:  parent.JoinPath(parent.RemotePath),
			IsDir: true,
		})
	}
	for _, e := range entries {
		if !showHidden && strings.HasPrefix(e.name, ".") {
			continue
		}
		child := remoteChildPath(rp, e.name)
		ext := ""
		if !e.isDir {
			ext = strings.TrimPrefix(filepath.Ext(e.name), ".")
		}
		mod := int64(0)
		if !e.mod.IsZero() {
			mod = e.mod.UnixMilli()
		}
		result = append(result, domain.FileEntry{
			Name:      e.name,
			Path:      loc.JoinPath(child),
			IsDir:     e.isDir,
			Size:      e.size,
			ModTime:   mod,
			Ext:       ext,
			IsSymlink: e.symlink,
		})
	}
	return result, nil
}

// Exists reports whether a virtual path is on the server.
func (m *FTPManager) Exists(vpath string) (bool, error) {
	loc, s, err := m.session(vpath)
	if err != nil {
		return false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok, err := s.stat(cleanFTPPath(loc.RemotePath))
	return ok, err
}

// ReadTextFile reads a remote text file for the built-in editor.
func (m *FTPManager) ReadTextFile(vpath string) (string, error) {
	loc, s, err := m.session(vpath)
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rp := cleanFTPPath(loc.RemotePath)
	st, ok, err := s.stat(rp)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("stat %s: file not found", rp)
	}
	if st.isDir {
		return "", fmt.Errorf("not a file: %s", rp)
	}
	if st.size > filesystem.MaxTextFileBytes {
		return "", filesystem.TooLargeError()
	}
	data, err := s.readAll("RETR", rp)
	if err != nil {
		return "", err
	}
	head := data
	if len(head) > filesystem.HeadBytes {
		head = head[:filesystem.HeadBytes]
	}
	if filesystem.IsExecutable(head) {
		return "", filesystem.ErrExecutable
	}
	if !utf8.Valid(data) {
		return "", filesystem.EncodingError(rp)
	}
	return string(data), nil
}

// WriteTextFile writes remote text. It stores a temp name and renames over the dest.
func (m *FTPManager) WriteTextFile(vpath, content string) error {
	loc, s, err := m.session(vpath)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rp := cleanFTPPath(loc.RemotePath)
	st, ok, err := s.stat(rp)
	if err != nil {
		return err
	}
	if ok && st.isDir {
		return fmt.Errorf("not a file: %s", rp)
	}
	tmp := path.Join(path.Dir(rp), fmt.Sprintf(".gfm-edit-%d", time.Now().UnixNano()))
	if err := s.storBytes(tmp, []byte(content)); err != nil {
		return err
	}
	if err := s.renamePath(tmp, rp); err == nil {
		return nil
	}
	if ok {
		_, _ = s.cmd("DELE " + tmpArg(rp))
		if err := s.renamePath(tmp, rp); err == nil {
			return nil
		}
	}
	_ = s.deletePath(tmp)
	return s.storBytes(rp, []byte(content))
}

// Mkdir creates a directory under parent.
func (m *FTPManager) Mkdir(parentV, name string) (string, error) {
	loc, s, err := m.session(parentV)
	if err != nil {
		return "", err
	}
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
		return "", fmt.Errorf("invalid name")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	full := path.Join(cleanFTPPath(loc.RemotePath), name)
	if err := s.mkdir(full); err != nil {
		return "", err
	}
	return loc.JoinPath(full), nil
}

// Rename renames an entry. newName is a basename.
func (m *FTPManager) Rename(oldV, newName string) (string, error) {
	loc, s, err := m.session(oldV)
	if err != nil {
		return "", err
	}
	newName = strings.TrimSpace(newName)
	if newName == "" || newName == "." || newName == ".." || strings.ContainsAny(newName, `/\`) {
		return "", fmt.Errorf("invalid name")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rp := cleanFTPPath(loc.RemotePath)
	next := path.Join(path.Dir(rp), newName)
	if err := s.renamePath(rp, next); err != nil {
		return "", err
	}
	return loc.JoinPath(next), nil
}

// Delete removes files and directories. Directories are removed recursively.
func (m *FTPManager) Delete(paths []string) error {
	for _, p := range paths {
		loc, s, err := m.session(p)
		if err != nil {
			return err
		}
		s.mu.Lock()
		err = s.deletePath(cleanFTPPath(loc.RemotePath))
		s.mu.Unlock()
		if err != nil {
			return err
		}
	}
	return nil
}

// DirChildSizesCtx returns recursive byte sizes for each immediate child directory.
func (m *FTPManager) DirChildSizesCtx(ctx context.Context, vpath string) (domain.DirSizes, error) {
	empty := domain.DirSizes{Sizes: map[string]int64{}, Denied: []string{}}
	loc, s, err := m.session(vpath)
	if err != nil {
		return empty, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rp := cleanFTPPath(loc.RemotePath)
	entries, err := s.list(rp)
	if err != nil {
		return empty, err
	}
	out := domain.DirSizes{Sizes: map[string]int64{}, Denied: []string{}}
	for _, e := range entries {
		if err := ctx.Err(); err != nil {
			return empty, err
		}
		if !e.isDir {
			continue
		}
		child := remoteChildPath(rp, e.name)
		size, walkErr := s.treeBytes(ctx, child)
		out.Sizes[loc.JoinPath(child)] = size
		if walkErr != nil {
			if ctx.Err() != nil {
				return empty, ctx.Err()
			}
			out.Denied = append(out.Denied, loc.JoinPath(child))
		}
	}
	return out, nil
}

// ListPathCompletions returns ftp:// path suggestions (max 50).
func (m *FTPManager) ListPathCompletions(partial string) ([]string, error) {
	partial = strings.TrimSpace(partial)
	loc, err := ParseLocation(partial)
	if err != nil {
		return nil, err
	}
	s, err := m.get(loc)
	if err != nil {
		return nil, err
	}
	rp := cleanFTPPath(loc.RemotePath)
	var dir, query string
	if strings.HasSuffix(strings.TrimSpace(loc.RemotePath), "/") || (rp == "/" && strings.HasSuffix(partial, "/")) {
		dir, query = rp, ""
	} else {
		dir, query = path.Dir(rp), path.Base(rp)
		if dir == "." {
			dir = "/"
		}
	}
	s.mu.Lock()
	entries, err := s.list(dir)
	s.mu.Unlock()
	if err != nil {
		return []string{}, nil
	}
	type item struct {
		full, name string
		isDir      bool
		starts     bool
		isDot      bool
	}
	queryLower := strings.ToLower(query)
	items := make([]item, 0, len(entries))
	for _, e := range entries {
		nameLower := strings.ToLower(e.name)
		if query != "" && !strings.Contains(nameLower, queryLower) {
			continue
		}
		full := path.Join(dir, e.name)
		if e.isDir {
			full += "/"
		}
		items = append(items, item{
			full:   loc.JoinPath(full),
			name:   e.name,
			isDir:  e.isDir,
			starts: query == "" || strings.HasPrefix(nameLower, queryLower),
			isDot:  strings.HasPrefix(e.name, "."),
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]
		if a.isDot != b.isDot {
			return !a.isDot
		}
		if a.starts != b.starts {
			return a.starts
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

func (m *FTPManager) session(vpath string) (Location, *ftpSession, error) {
	loc, err := ParseLocation(vpath)
	if err != nil {
		return Location{}, nil, err
	}
	s, err := m.get(loc)
	if err != nil {
		return Location{}, nil, err
	}
	return loc, s, nil
}

func (s *ftpSession) startRead(verb, p string) (net.Conn, error) {
	arg, err := ftpArg(p)
	if err != nil {
		return nil, err
	}
	data, err := s.passive()
	if err != nil {
		return nil, err
	}
	rep, err := s.cmd(verb + " " + arg)
	if err != nil {
		_ = data.Close()
		return nil, err
	}
	if rep.code != 150 && rep.code != 125 {
		_ = data.Close()
		return nil, &ftpStatusError{code: rep.code, text: rep.text}
	}
	return data, nil
}

func (s *ftpSession) finishData(data net.Conn) error {
	_ = data.Close()
	rep, err := s.readReply()
	if err != nil {
		return err
	}
	if rep.code/100 != 2 {
		return &ftpStatusError{code: rep.code, text: rep.text}
	}
	return nil
}

func (s *ftpSession) readData(verb, p string) (string, error) {
	b, err := s.readAll(verb, p)
	return string(b), err
}

func (s *ftpSession) readAll(verb, p string) ([]byte, error) {
	data, err := s.startRead(verb, p)
	if err != nil {
		return nil, err
	}
	buf, rerr := io.ReadAll(data)
	ferr := s.finishData(data)
	if rerr != nil {
		return nil, rerr
	}
	if ferr != nil {
		return nil, ferr
	}
	return buf, nil
}

func (s *ftpSession) withWrite(p string, fn func(io.Writer) error) error {
	arg, err := ftpArg(p)
	if err != nil {
		return err
	}
	data, err := s.passive()
	if err != nil {
		return err
	}
	rep, err := s.cmd("STOR " + arg)
	if err != nil {
		_ = data.Close()
		return err
	}
	if rep.code != 150 && rep.code != 125 {
		_ = data.Close()
		return &ftpStatusError{code: rep.code, text: rep.text}
	}
	werr := fn(data)
	ferr := s.finishData(data)
	if werr != nil {
		return werr
	}
	return ferr
}

func (s *ftpSession) storBytes(p string, body []byte) error {
	return s.withWrite(p, func(w io.Writer) error {
		_, err := w.Write(body)
		return err
	})
}

func (s *ftpSession) list(p string) ([]ftpEntry, error) {
	body, err := s.readData("MLSD", p)
	if isFTPUnsupported(err) {
		body, err = s.readData("LIST", p)
		if err != nil {
			return nil, err
		}
		return parseUnixLIST(body), nil
	}
	if err != nil {
		return nil, err
	}
	return parseMLSD(body), nil
}

func isFTPUnsupported(err error) bool {
	var st *ftpStatusError
	if errors.As(err, &st) {
		return st.code == 500 || st.code == 502 || st.code == 504
	}
	return false
}

func (s *ftpSession) stat(p string) (ftpEntry, bool, error) {
	p = cleanFTPPath(p)
	if p == "/" {
		return ftpEntry{name: "/", isDir: true}, true, nil
	}
	parent := path.Dir(p)
	if parent == "." {
		parent = "/"
	}
	entries, err := s.list(parent)
	if err != nil {
		return ftpEntry{}, false, err
	}
	base := path.Base(p)
	for _, e := range entries {
		if e.name == base {
			return e, true, nil
		}
	}
	return ftpEntry{}, false, nil
}

func (s *ftpSession) mkdir(p string) error {
	arg, err := ftpArg(p)
	if err != nil {
		return err
	}
	rep, err := s.cmd("MKD " + arg)
	if err != nil {
		return err
	}
	if rep.code/100 != 2 {
		return &ftpStatusError{code: rep.code, text: rep.text}
	}
	return nil
}

func (s *ftpSession) renamePath(from, to string) error {
	a, err := ftpArg(from)
	if err != nil {
		return err
	}
	b, err := ftpArg(to)
	if err != nil {
		return err
	}
	rep, err := s.cmd("RNFR " + a)
	if err != nil {
		return err
	}
	if rep.code != 350 && rep.code/100 != 3 {
		return &ftpStatusError{code: rep.code, text: rep.text}
	}
	rep, err = s.cmd("RNTO " + b)
	if err != nil {
		return err
	}
	if rep.code/100 != 2 {
		return &ftpStatusError{code: rep.code, text: rep.text}
	}
	return nil
}

func (s *ftpSession) deletePath(p string) error {
	st, ok, err := s.stat(p)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	if st.isDir {
		entries, err := s.list(p)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if err := s.deletePath(path.Join(p, e.name)); err != nil {
				return err
			}
		}
		arg, err := ftpArg(p)
		if err != nil {
			return err
		}
		rep, err := s.cmd("RMD " + arg)
		if err != nil {
			return err
		}
		if rep.code/100 != 2 {
			return &ftpStatusError{code: rep.code, text: rep.text}
		}
		return nil
	}
	arg, err := ftpArg(p)
	if err != nil {
		return err
	}
	rep, err := s.cmd("DELE " + arg)
	if err != nil {
		return err
	}
	if rep.code/100 != 2 {
		return &ftpStatusError{code: rep.code, text: rep.text}
	}
	return nil
}

func (s *ftpSession) treeBytes(ctx context.Context, p string) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	entries, err := s.list(p)
	if err != nil {
		st, ok, statErr := s.stat(p)
		if statErr != nil {
			return 0, statErr
		}
		if !ok || st.isDir {
			return 0, err
		}
		return st.size, nil
	}
	var total int64
	for _, e := range entries {
		if e.isDir {
			n, err := s.treeBytes(ctx, path.Join(p, e.name))
			total += n
			if err != nil {
				return total, err
			}
			continue
		}
		total += e.size
	}
	return total, nil
}

func (s *ftpSession) uniqueChild(dir, base string) (string, error) {
	cand := path.Join(dir, base)
	_, ok, err := s.stat(cand)
	if err != nil {
		return "", err
	}
	if !ok {
		return cand, nil
	}
	ext := path.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	for i := 2; i < 1000; i++ {
		name := fmt.Sprintf("%s %d%s", stem, i, ext)
		p := path.Join(dir, name)
		_, ok, err := s.stat(p)
		if err != nil {
			return "", err
		}
		if !ok {
			return p, nil
		}
	}
	return "", fmt.Errorf("no unique name for %s", base)
}

func tmpArg(p string) string {
	arg, err := ftpArg(p)
	if err != nil {
		return p
	}
	return arg
}

func parseMLSD(body string) []ftpEntry {
	var out []ftpEntry
	for _, line := range splitFTPLines(body) {
		i := strings.Index(line, "; ")
		if i < 0 {
			continue
		}
		name := line[i+2:]
		if name == "" || name == "." || name == ".." {
			continue
		}
		e := ftpEntry{name: name}
		skip := false
		for _, fact := range strings.Split(line[:i], ";") {
			fact = strings.TrimSpace(fact)
			k, v, ok := strings.Cut(fact, "=")
			if !ok {
				continue
			}
			switch strings.ToLower(k) {
			case "type":
				switch strings.ToLower(v) {
				case "dir":
					e.isDir = true
				case "cdir", "pdir":
					skip = true
				default:
					if strings.Contains(strings.ToLower(v), "slink") {
						e.symlink = true
					}
				}
			case "size":
				e.size, _ = strconv.ParseInt(v, 10, 64)
			case "modify":
				e.mod = parseFTPModify(v)
			}
		}
		if skip {
			continue
		}
		out = append(out, e)
	}
	return out
}

func parseUnixLIST(body string) []ftpEntry {
	var out []ftpEntry
	for _, line := range splitFTPLines(body) {
		if strings.HasPrefix(line, "total ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 9 {
			continue
		}
		perm := fields[0]
		if perm == "" || !strings.ContainsRune("dl-", rune(perm[0])) {
			continue
		}
		name := strings.Join(fields[8:], " ")
		symlink := perm[0] == 'l'
		if symlink {
			if i := strings.Index(name, " -> "); i > 0 {
				name = name[:i]
			}
		}
		if name == "" || name == "." || name == ".." {
			continue
		}
		size, _ := strconv.ParseInt(fields[4], 10, 64)
		out = append(out, ftpEntry{
			name:    name,
			isDir:   perm[0] == 'd',
			size:    size,
			symlink: symlink,
		})
	}
	return out
}

func parseFTPModify(v string) time.Time {
	if len(v) < 14 {
		return time.Time{}
	}
	t, err := time.Parse("20060102150405", v[:14])
	if err != nil {
		return time.Time{}
	}
	return t.UTC()
}

func splitFTPLines(body string) []string {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	body = strings.ReplaceAll(body, "\r", "\n")
	raw := strings.Split(body, "\n")
	out := make([]string, 0, len(raw))
	for _, line := range raw {
		line = strings.TrimRight(line, "\r")
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}
