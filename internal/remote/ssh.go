package remote

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/erikharutyunyan/go-file-manager/internal/domain"
	"github.com/erikharutyunyan/go-file-manager/internal/filesystem"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// HostKeyStore loads/saves TOFU host key fingerprints (base64 SHA256).
type HostKeyStore interface {
	GetHostKey(hostPort string) (string, error)
	SetHostKey(hostPort string, fingerprint string) error
}

// Manager holds live SSH+SFTP sessions.
type Manager struct {
	mu       sync.Mutex
	sessions map[string]*Session // SessionKey
	keys     HostKeyStore
}

// Session is one connected SSH host (native Go crypto or system OpenSSH SFTP).
type Session struct {
	Spec     Spec
	client   *ssh.Client // nil when using system OpenSSH
	sftp     *sftp.Client
	cmd      *exec.Cmd // system ssh process when client == nil
	stderr   *bytes.Buffer
	password string // in-memory only; reused to spawn extra ssh (remote shell)
}

// NewManager creates a session manager. keys may be nil (accept all first-seen; no persist).
func NewManager(keys HostKeyStore) *Manager {
	return &Manager{
		sessions: make(map[string]*Session),
		keys:     keys,
	}
}

// Connect dials SSH and opens SFTP. password may be empty (try keys/agent first).
// Prefers system OpenSSH (`ssh -s sftp`) so ~/.ssh/config and host quirks match the CLI. Falls back to native Go crypto if OpenSSH fails (no binary, no keys, etc.).
func (m *Manager) Connect(spec Spec, password string) error {
	spec = EnrichSpec(spec)
	if spec.Port <= 0 {
		spec.Port = 22
	}
	key := spec.SessionKey()

	m.mu.Lock()
	if existing, ok := m.sessions[key]; ok {
		m.mu.Unlock()
		_ = existing
		return nil
	}
	m.mu.Unlock()

	if sess, err := dialOpenSSH(spec, password); err == nil {
		return m.storeSession(key, sess)
	} else if password == "" {
		// Surface OpenSSH auth failure so UI can prompt for password/passphrase.
		if isOpenSSHAuthError(err) {
			return err
		}
		// Connection-level failures (timeout, refused, unreachable) will fail the
		// same way via native Go crypto — don't double-wait on ConnectTimeout.
		if isOpenSSHConnectionError(err) {
			return err
		}
		// Fall through to native for other OpenSSH failures (no binary, odd
		// subsystem issues, etc.). Prefer OpenSSH diagnostic if both fail.
		if !strings.Contains(err.Error(), "openssh not found") {
			if err2 := m.connectNative(spec, password); err2 != nil {
				return err
			}
			return nil
		}
	}

	// (password retry, missing OpenSSH, etc.)
	return m.connectNative(spec, password)
}

func (m *Manager) storeSession(key string, sess *Session) error {
	m.mu.Lock()
	if old, ok := m.sessions[key]; ok {
		m.mu.Unlock()
		_ = sess.closeTransport()
		_ = old
		return nil
	}
	m.sessions[key] = sess
	m.mu.Unlock()
	return nil
}

func (m *Manager) connectNative(spec Spec, password string) error {
	key := spec.SessionKey()

	auth, info := buildAuthMethods(spec.User, password, spec.IdentityFiles)
	if len(auth) == 0 {
		return fmt.Errorf("no authentication methods available: %s; provide a password or key passphrase, or set IdentityFile in ~/.ssh/config", info.summary())
	}

	cfg := &ssh.ClientConfig{
		User:            spec.User,
		Auth:            auth,
		HostKeyCallback: m.hostKeyCallback(spec),
		Timeout:         15 * time.Second,
	}

	conn, err := ssh.Dial("tcp", spec.DialAddr(), cfg)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "unable to authenticate") {
			hint := info.summary()
			if password == "" {
				return fmt.Errorf("authentication required: public key auth failed (%s). Provide a server password or key passphrase, or add IdentityFile in ~/.ssh/config: %w", hint, err)
			}
			return fmt.Errorf("authentication required: still unable to authenticate (%s): %w", hint, err)
		}
		return fmt.Errorf("ssh dial %s: %w", spec.DialAddr(), err)
	}

	sftpClient, err := sftp.NewClient(conn, sftpClientOpts()...)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("sftp: %w", err)
	}

	return m.storeSession(key, &Session{Spec: spec, client: conn, sftp: sftpClient, password: password})
}

// Disconnect closes a session by key (user@host:port) or any path under that host.
func (m *Manager) Disconnect(keyOrPath string) error {
	key := keyOrPath
	if IsRemote(keyOrPath) {
		loc, err := ParseLocation(keyOrPath)
		if err != nil {
			return err
		}
		key = loc.SessionKey()
	}
	m.mu.Lock()
	sess, ok := m.sessions[key]
	if ok {
		delete(m.sessions, key)
	}
	m.mu.Unlock()
	if !ok {
		return nil
	}
	return sess.closeTransport()
}

// ListSessions returns active session keys.
func (m *Manager) ListSessions() []domain.ActiveSession {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]domain.ActiveSession, 0, len(m.sessions))
	for _, s := range m.sessions {
		out = append(out, domain.ActiveSession{
			Key:      s.Spec.SessionKey(),
			Protocol: "ssh",
			User:     s.Spec.User,
			Host:     s.Spec.Host,
			Port:     s.Spec.Port,
			RootPath: s.Spec.RootPath(),
		})
	}
	return out
}

// CloseAll closes every session (app shutdown).
func (m *Manager) CloseAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, s := range m.sessions {
		_ = s.closeTransport()
		delete(m.sessions, k)
	}
}

func (m *Manager) get(loc Location) (*Session, error) {
	key := loc.SessionKey()
	m.mu.Lock()
	s, ok := m.sessions[key]
	m.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("not connected to %s; connect first", key)
	}
	return s, nil
}

// OpenRead returns a streaming reader for a remote file (SFTP). Caller closes it.
func (m *Manager) OpenRead(vpath string) (io.ReadCloser, error) {
	loc, err := ParseLocation(vpath)
	if err != nil {
		return nil, err
	}
	s, err := m.get(loc)
	if err != nil {
		return nil, err
	}
	rp := remoteAbsPath(loc.RemotePath)
	f, err := s.sftp.Open(rp)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", rp, err)
	}
	return f, nil
}

// HomePath returns remote home directory as virtual path, or root.
func (m *Manager) HomePath(spec Spec) (string, error) {
	loc := Location{Spec: spec, RemotePath: "."}
	s, err := m.get(loc)
	if err != nil {
		return "", err
	}
	wd, err := s.sftp.Getwd()
	if err != nil || wd == "" {
		return spec.RootPath(), nil
	}
	if !strings.HasPrefix(wd, "/") {
		wd = "/" + wd
	}
	return spec.JoinPath(wd), nil
}

// ListDir lists a remote directory (virtual path).
func (m *Manager) ListDir(vpath string, showHidden bool) ([]domain.FileEntry, error) {
	loc, err := ParseLocation(vpath)
	if err != nil {
		return nil, err
	}
	s, err := m.get(loc)
	if err != nil {
		return nil, err
	}
	rp := remoteAbsPath(loc.RemotePath)
	entries, err := s.sftp.ReadDir(rp)
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", rp, err)
	}

	result := make([]domain.FileEntry, 0, len(entries)+1)
	// Parent ".."
	if rp != "/" {
		parent := ParentRemote(loc)
		result = append(result, domain.FileEntry{
			Name:  "..",
			Path:  parent.JoinPath(parent.RemotePath),
			IsDir: true,
		})
	}

	for _, e := range entries {
		name := e.Name()
		if !showHidden && strings.HasPrefix(name, ".") {
			continue
		}
		child := remoteChildPath(rp, name)
		mode := e.Mode()
		isDir := sftpEntryIsDir(s.sftp, e, child)
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
			IsSymlink: mode&os.ModeSymlink != 0,
		})
	}
	return result, nil
}

// Exists checks a virtual path.
func (m *Manager) Exists(vpath string) (bool, error) {
	loc, err := ParseLocation(vpath)
	if err != nil {
		return false, err
	}
	s, err := m.get(loc)
	if err != nil {
		return false, err
	}
	_, err = s.sftp.Stat(loc.RemotePath)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	// sftp may wrap
	if strings.Contains(err.Error(), "not exist") || strings.Contains(err.Error(), "no such file") {
		return false, nil
	}
	return false, err
}

// ReadTextFile reads a remote text file for the built-in editor.
func (m *Manager) ReadTextFile(vpath string) (string, error) {
	loc, err := ParseLocation(vpath)
	if err != nil {
		return "", err
	}
	s, err := m.get(loc)
	if err != nil {
		return "", err
	}
	info, err := s.sftp.Stat(loc.RemotePath)
	if err != nil {
		return "", fmt.Errorf("stat %s: %w", loc.RemotePath, err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("not a file: %s", loc.RemotePath)
	}
	f, err := s.sftp.Open(loc.RemotePath)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	// Same order as the local reader: format before size, so a big binary says
	// "executable" rather than "too large".
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

// WriteTextFile writes remote text content atomically (temp + rename over sftp).
func (m *Manager) WriteTextFile(vpath, content string) error {
	loc, err := ParseLocation(vpath)
	if err != nil {
		return err
	}
	s, err := m.get(loc)
	if err != nil {
		return err
	}
	if info, err := s.sftp.Stat(loc.RemotePath); err == nil && info.IsDir() {
		return fmt.Errorf("not a file: %s", loc.RemotePath)
	}
	dir := path.Dir(loc.RemotePath)
	tmp := path.Join(dir, fmt.Sprintf(".gfm-edit-%d", time.Now().UnixNano()))
	f, err := s.sftp.Create(tmp)
	if err != nil {
		return err
	}
	ok := false
	defer func() {
		if !ok {
			_ = s.sftp.Remove(tmp)
		}
	}()
	if _, err := f.Write([]byte(content)); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := s.sftp.Rename(tmp, loc.RemotePath); err != nil {
		// sftp Rename fails if dest exists on some servers; fall back to remove+rename.
		if rmErr := s.sftp.Remove(loc.RemotePath); rmErr != nil {
			return err
		}
		if err := s.sftp.Rename(tmp, loc.RemotePath); err != nil {
			return err
		}
	}
	ok = true
	return nil
}

// DirChildSizesCtx returns recursive byte sizes for each immediate child directory
// of a remote dir. Keys are ssh:// virtual paths (same form as ListDir).
// Symlink/junction children that resolve to directories are included (Windows
// OpenSSH often surfaces junctions with the symlink bit).
func (m *Manager) DirChildSizesCtx(ctx context.Context, vpath string) (domain.DirSizes, error) {
	empty := domain.DirSizes{Sizes: map[string]int64{}, Denied: []string{}}
	loc, err := ParseLocation(vpath)
	if err != nil {
		return empty, err
	}
	s, err := m.get(loc)
	if err != nil {
		return empty, err
	}
	rp := loc.RemotePath
	if rp == "" {
		rp = "/"
	}
	rp = remoteAbsPath(rp)
	info, err := s.sftp.Stat(rp)
	if err != nil {
		return empty, err
	}
	if !info.IsDir() {
		return empty, fmt.Errorf("not a directory: %s", rp)
	}
	entries, err := s.sftp.ReadDir(rp)
	if err != nil {
		return empty, err
	}
	out := domain.DirSizes{Sizes: map[string]int64{}, Denied: []string{}}
	var nDir int
	for _, e := range entries {
		if err := ctx.Err(); err != nil {
			return empty, err
		}
		child := remoteChildPath(rp, e.Name())
		if !sftpEntryIsDir(s.sftp, e, child) {
			continue
		}
		nDir++
		size, denied, err := remoteDirSizeCtx(ctx, s.sftp, child)
		if err != nil {
			if ctx.Err() != nil {
				return empty, ctx.Err()
			}
			// Still publish partial/zero size so the UI leaves <DIR> mode.
			out.Sizes[loc.JoinPath(child)] = size
			out.Denied = append(out.Denied, loc.JoinPath(child))
			continue
		}
		// Key must match ListDir's Path field exactly so the UI can look up by e.path.
		out.Sizes[loc.JoinPath(child)] = size
		if denied {
			out.Denied = append(out.Denied, loc.JoinPath(child))
		}
	}
	log.Printf("gfm: DirChildSizes remote dir=%q entries=%d dirs=%d sized=%d denied=%d",
		vpath, len(entries), nDir, len(out.Sizes), len(out.Denied))
	return out, nil
}

// remoteAbsPath ensures a leading slash for SFTP paths (Windows OpenSSH Getwd
// can return "C:/Users/…" without one).
func remoteAbsPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		return "/" + p
	}
	return p
}

// remoteChildPath joins parent/name for SFTP (always absolute).
func remoteChildPath(parent, name string) string {
	return remoteAbsPath(path.Join(parent, name))
}

// sftpEntryIsDir reports whether e is a directory at full, matching ListDir:
// follow symlink/junction targets so Windows reparse points count as folders.
func sftpEntryIsDir(c *sftp.Client, e os.FileInfo, full string) bool {
	if e.Mode()&os.ModeSymlink != 0 {
		st, err := c.Stat(full)
		return err == nil && st.IsDir()
	}
	if e.IsDir() {
		return true
	}
	// Some Windows OpenSSH builds omit type bits on ReadDir; Stat is authoritative.
	// Only probe zero-size non-regular names to avoid doubling RTTs on every file.
	if !sftpDirProbeCandidate(e) {
		return false
	}
	st, err := c.Stat(full)
	return err == nil && st.IsDir()
}

func sftpDirProbeCandidate(e os.FileInfo) bool {
	// Do not use IsRegular(): omitted type bits look regular, which is the
	// Windows OpenSSH case we still need to Stat.
	if e.IsDir() || e.Mode()&os.ModeSymlink != 0 {
		return false
	}
	return e.Size() == 0
}

// remoteDirSizeCtx sums a remote tree. An unreadable subdirectory contributes 0
// and sets denied instead of aborting: on a Windows host most of the tree is
// unreadable, and aborting made every top-level child disappear from the result.
// Nested symlinks are not followed (loop risk); junctions already resolved at
// the parent listing are walked as normal directories after Stat.
// ponytail: serial — one SFTP round-trip per directory. If deep trees feel slow,
// the upgrade is a bounded worker pool over the child directories.
func remoteDirSizeCtx(ctx context.Context, c *sftp.Client, root string) (total int64, denied bool, err error) {
	entries, err := c.ReadDir(root)
	if err != nil {
		return 0, true, nil
	}
	for _, e := range entries {
		if err := ctx.Err(); err != nil {
			return 0, denied, err
		}
		child := remoteChildPath(root, e.Name())
		// Skip pure symlinks inside the tree (do not follow). Still count
		// real directories, including ones that only Stat can identify.
		if e.Mode()&os.ModeSymlink != 0 {
			continue
		}
		if sftpEntryIsDir(c, e, child) {
			sub, subDenied, err := remoteDirSizeCtx(ctx, c, child)
			if err != nil {
				return 0, denied, err
			}
			total += sub
			denied = denied || subDenied
			continue
		}
		total += e.Size()
	}
	return total, denied, nil
}

// ListPathCompletions returns ssh:// path suggestions for a partial remote path (max 50).
func (m *Manager) ListPathCompletions(partial string) ([]string, error) {
	partial = strings.TrimSpace(partial)
	loc, err := ParseLocation(partial)
	if err != nil {
		return nil, err
	}
	s, err := m.get(loc)
	if err != nil {
		return nil, err
	}

	rp := loc.RemotePath
	var dir, query string
	if strings.HasSuffix(rp, "/") {
		dir, query = rp, ""
	} else {
		dir, query = path.Dir(rp), path.Base(rp)
		if dir == "." {
			dir = "/"
		}
	}

	entries, err := s.sftp.ReadDir(dir)
	if err != nil {
		return []string{}, nil
	}

	type item struct {
		full       string
		name       string
		isDir      bool
		startsWith bool
		isDot      bool
	}
	queryLower := strings.ToLower(query)
	items := make([]item, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		nameLower := strings.ToLower(name)
		if query != "" && !strings.Contains(nameLower, queryLower) {
			continue
		}
		isDir := e.IsDir()
		fullPath := path.Join(dir, name)
		if isDir {
			fullPath += "/"
		}
		items = append(items, item{
			full:       loc.JoinPath(fullPath),
			name:       name,
			isDir:      isDir,
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
func (m *Manager) Mkdir(parentV, name string) (string, error) {
	loc, err := ParseLocation(parentV)
	if err != nil {
		return "", err
	}
	s, err := m.get(loc)
	if err != nil {
		return "", err
	}
	name = strings.TrimSpace(name)
	if name == "" || strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return "", fmt.Errorf("invalid name")
	}
	full := path.Join(loc.RemotePath, name)
	if err := s.sftp.Mkdir(full); err != nil {
		return "", err
	}
	return loc.JoinPath(full), nil
}

// Rename renames a remote entry (newName is basename only).
func (m *Manager) Rename(oldV, newName string) (string, error) {
	loc, err := ParseLocation(oldV)
	if err != nil {
		return "", err
	}
	s, err := m.get(loc)
	if err != nil {
		return "", err
	}
	newName = strings.TrimSpace(newName)
	if newName == "" || strings.ContainsAny(newName, `/\`) {
		return "", fmt.Errorf("invalid name")
	}
	dir := path.Dir(loc.RemotePath)
	next := path.Join(dir, newName)
	if err := s.sftp.Rename(loc.RemotePath, next); err != nil {
		return "", err
	}
	return loc.JoinPath(next), nil
}

// Delete removes remote paths (files or recursive dirs).
func (m *Manager) Delete(paths []string) error {
	for _, p := range paths {
		loc, err := ParseLocation(p)
		if err != nil {
			return err
		}
		s, err := m.get(loc)
		if err != nil {
			return err
		}
		if err := removeAll(s.sftp, loc.RemotePath); err != nil {
			return err
		}
	}
	return nil
}

func removeAll(c *sftp.Client, p string) error {
	st, err := c.Stat(p)
	if err != nil {
		return err
	}
	if !st.IsDir() {
		return c.Remove(p)
	}
	entries, err := c.ReadDir(p)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := removeAll(c, path.Join(p, e.Name())); err != nil {
			return err
		}
	}
	return c.RemoveDirectory(p)
}

// CopyWithin copies sources into destDir on the same host (remote only).
func (m *Manager) CopyWithin(sources []string, destDir string) error {
	return m.CopyWithinCtx(context.Background(), sources, destDir, nil)
}

// CopyWithinCtx copies sources into destDir on the same host with progress.
func (m *Manager) CopyWithinCtx(ctx context.Context, sources []string, destDir string, onProgress filesystem.ProgressFunc) (err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	dloc, err := ParseLocation(destDir)
	if err != nil {
		return err
	}
	ds, err := m.get(dloc)
	if err != nil {
		return err
	}
	files, total, err := m.remoteSourcesStats(sources)
	if err != nil {
		return err
	}
	rep := newRemoteProgress(total, onProgress)
	if onProgress != nil {
		onProgress(filesystem.ProgressEvent{Total: total})
	}
	workers := filesystem.WorkerCount(filesystem.IONetwork, files)
	pool := newXferPool(ctx, workers, func(ctx context.Context, j sftpFileJob) error {
		return copyRemoteFile(ctx, j.c, j.src, j.dst, j.mode, rep)
	})
	var created []string
	defer func() {
		if errors.Is(err, context.Canceled) {
			for _, p := range created {
				_ = removeAll(ds.sftp, p)
			}
		}
	}()
	var placeErr error
	for _, src := range sources {
		if err := ctx.Err(); err != nil {
			placeErr = err
			break
		}
		sloc, err := ParseLocation(src)
		if err != nil {
			placeErr = err
			break
		}
		if sloc.SessionKey() != dloc.SessionKey() {
			placeErr = fmt.Errorf("cross-host copy not supported")
			break
		}
		base := path.Base(sloc.RemotePath)
		dest := uniqueRemotePath(ds.sftp, path.Join(dloc.RemotePath, base))
		created = append(created, dest)
		st, statErr := ds.sftp.Stat(sloc.RemotePath)
		srcIsDir := statErr == nil && st.IsDir()
		rep.setDest(dloc.JoinPath(dest), srcIsDir)
		if err := copyRemoteCtx(ctx, ds, sloc.RemotePath, dest, rep, pool); err != nil {
			placeErr = err
			break
		}
	}
	if placeErr != nil {
		pool.cancel()
	}
	waitErr := pool.finish()
	if placeErr != nil {
		err = placeErr
		return err
	}
	if waitErr != nil {
		err = waitErr
		return err
	}
	rep.finish("")
	return nil
}

// MoveWithin moves sources into destDir on the same host.
func (m *Manager) MoveWithin(sources []string, destDir string) error {
	return m.MoveWithinCtx(context.Background(), sources, destDir, nil)
}

// MoveWithinCtx moves sources into destDir on the same host with progress.
func (m *Manager) MoveWithinCtx(ctx context.Context, sources []string, destDir string, onProgress filesystem.ProgressFunc) error {
	if ctx == nil {
		ctx = context.Background()
	}
	dloc, err := ParseLocation(destDir)
	if err != nil {
		return err
	}
	ds, err := m.get(dloc)
	if err != nil {
		return err
	}

	var total int64
	weights := make([]int64, len(sources))
	for i, src := range sources {
		loc, err := ParseLocation(src)
		if err != nil {
			return err
		}
		ss, err := m.get(loc)
		if err != nil {
			return err
		}
		n, err := remotePathBytes(ss.sftp, loc.RemotePath)
		if err != nil {
			return err
		}
		if n == 0 {
			n = 1
		}
		weights[i] = n
		total += n
	}
	rep := newRemoteProgress(total, onProgress)
	if onProgress != nil {
		onProgress(filesystem.ProgressEvent{Total: total})
	}

	for i, src := range sources {
		if err := ctx.Err(); err != nil {
			return err
		}
		sloc, err := ParseLocation(src)
		if err != nil {
			return err
		}
		if sloc.SessionKey() != dloc.SessionKey() {
			return fmt.Errorf("cross-host move not supported")
		}
		base := path.Base(sloc.RemotePath)
		dest := uniqueRemotePath(ds.sftp, path.Join(dloc.RemotePath, base))
		st, statErr := ds.sftp.Stat(sloc.RemotePath)
		srcIsDir := statErr == nil && st.IsDir()
		rep.setDest(dloc.JoinPath(dest), srcIsDir)
		if err := ds.sftp.Rename(sloc.RemotePath, dest); err != nil {
			if err2 := copyRemoteOne(ctx, ds, sloc.RemotePath, dest, rep); err2 != nil {
				return err2
			}
			if err2 := removeAll(ds.sftp, sloc.RemotePath); err2 != nil {
				return err2
			}
			continue
		}
		rep.add(weights[i], sloc.RemotePath)
	}
	rep.finish("")
	return nil
}

func copyRemoteOne(ctx context.Context, s *Session, src, dst string, rep *remoteProgress) error {
	files, _, err := remotePathStats(s.sftp, src)
	if err != nil {
		return err
	}
	workers := filesystem.WorkerCount(filesystem.IONetwork, files)
	pool := newXferPool(ctx, workers, func(ctx context.Context, j sftpFileJob) error {
		return copyRemoteFile(ctx, j.c, j.src, j.dst, j.mode, rep)
	})
	if err := copyRemoteCtx(ctx, s, src, dst, rep, pool); err != nil {
		pool.cancel()
		_ = pool.finish()
		return err
	}
	return pool.finish()
}

func copyRemoteCtx(ctx context.Context, s *Session, src, dst string, rep *remoteProgress, pool *xferPool[sftpFileJob]) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c := s.sftp
	st, err := c.Stat(src)
	if err != nil {
		return err
	}
	if err := s.remoteCP(ctx, src, dst); err == nil {
		n := st.Size()
		if st.IsDir() {
			n, err = remotePathBytes(c, src)
			if err != nil || n <= 0 {
				n = 1
			}
		} else if n <= 0 {
			n = 1
		}
		rep.add(n, src)
		return nil
	}
	return copyRemotePlace(ctx, c, src, dst, pool)
}

func copyRemotePlace(ctx context.Context, c *sftp.Client, src, dst string, pool *xferPool[sftpFileJob]) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	st, err := c.Stat(src)
	if err != nil {
		return err
	}
	if !st.IsDir() {
		return pool.enqueue(sftpFileJob{c: c, src: src, dst: dst, mode: st.Mode()})
	}
	if err := c.MkdirAll(dst); err != nil {
		return err
	}
	entries, err := c.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := copyRemotePlace(ctx, c, path.Join(src, e.Name()), path.Join(dst, e.Name()), pool); err != nil {
			return err
		}
	}
	return nil
}

func copyRemoteFile(ctx context.Context, c *sftp.Client, src, dst string, _ os.FileMode, rep *remoteProgress) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	in, err := c.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := c.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()
	var known int64
	if st, err := in.Stat(); err == nil {
		known = st.Size()
	}
	if err := copyUnwrapped(ctx, out, in, src, known, nil, rep); err != nil {
		return err
	}
	return out.Close()
}

type authBuildInfo struct {
	agentOK        bool
	loadedKeys     int
	passphraseNeed int
	missingKeys    int
	extraTried     int
}

func (a authBuildInfo) summary() string {
	parts := []string{}
	if a.agentOK {
		parts = append(parts, "ssh-agent")
	} else {
		parts = append(parts, "no ssh-agent keys")
	}
	parts = append(parts, fmt.Sprintf("%d key file(s) loaded", a.loadedKeys))
	if a.passphraseNeed > 0 {
		parts = append(parts, fmt.Sprintf("%d encrypted key(s) need passphrase", a.passphraseNeed))
	}
	if a.missingKeys > 0 {
		parts = append(parts, fmt.Sprintf("%d key path(s) missing", a.missingKeys))
	}
	if a.extraTried > 0 {
		parts = append(parts, fmt.Sprintf("%d IdentityFile path(s)", a.extraTried))
	}
	return strings.Join(parts, "; ")
}

func buildAuthMethods(user, password string, extraKeyPaths []string) ([]ssh.AuthMethod, authBuildInfo) {
	var methods []ssh.AuthMethod
	var info authBuildInfo

	// SSH agent
	if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
		if ag, err := net.Dial("unix", sock); err == nil {
			client := agent.NewClient(ag)
			if signers, err := client.Signers(); err == nil && len(signers) > 0 {
				info.agentOK = true
				methods = append(methods, ssh.PublicKeys(signers...))
			} else {
				// Keep callback so late-loaded keys can still work
				methods = append(methods, ssh.PublicKeysCallback(client.Signers))
			}
		}
	}

	seenKey := map[string]bool{}
	tryKey := func(kp string) {
		kp = strings.TrimSpace(kp)
		if kp == "" {
			return
		}
		if abs, err := filepath.Abs(kp); err == nil {
			kp = abs
		}
		if seenKey[kp] {
			return
		}
		seenKey[kp] = true
		m, st := publicKeyFile(kp, password)
		switch st {
		case keyStatusOK:
			info.loadedKeys++
			methods = append(methods, m)
		case keyStatusPassphrase:
			info.passphraseNeed++
		case keyStatusMissing:
			info.missingKeys++
		}
	}

	// Extra identity files (SSH config IdentityFile) first — preferred by IdentitiesOnly semantics
	for _, kp := range extraKeyPaths {
		info.extraTried++
		tryKey(kp)
	}

	// Default private keys
	home, _ := os.UserHomeDir()
	if home != "" {
		for _, name := range []string{"id_ed25519", "id_rsa", "id_ecdsa", "id_ed25519_sk", "id_ecdsa_sk"} {
			tryKey(filepath.Join(home, ".ssh", name))
		}
	}

	if password != "" {
		methods = append(methods, ssh.Password(password))
		methods = append(methods, ssh.KeyboardInteractive(func(name, instruction string, questions []string, echos []bool) ([]string, error) {
			answers := make([]string, len(questions))
			for i := range questions {
				answers[i] = password
			}
			return answers, nil
		}))
	}

	_ = user
	return methods, info
}

type keyStatus int

const (
	keyStatusOK keyStatus = iota
	keyStatusMissing
	keyStatusPassphrase
	keyStatusBad
)

func publicKeyFile(path, passphrase string) (ssh.AuthMethod, keyStatus) {
	key, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, keyStatusMissing
		}
		return nil, keyStatusBad
	}
	signer, err := ssh.ParsePrivateKey(key)
	if err == nil {
		return ssh.PublicKeys(signer), keyStatusOK
	}
	// Encrypted key
	if _, ok := err.(*ssh.PassphraseMissingError); ok || strings.Contains(strings.ToLower(err.Error()), "passphrase") {
		if passphrase == "" {
			return nil, keyStatusPassphrase
		}
		signer, err = ssh.ParsePrivateKeyWithPassphrase(key, []byte(passphrase))
		if err != nil {
			return nil, keyStatusPassphrase
		}
		return ssh.PublicKeys(signer), keyStatusOK
	}
	return nil, keyStatusBad
}

func (m *Manager) hostKeyCallback(spec Spec) ssh.HostKeyCallback {
	hostPort := spec.DialAddr()
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		fp := fingerprint(key)
		if m.keys != nil {
			stored, err := m.keys.GetHostKey(hostPort)
			if err == nil && stored != "" {
				if stored != fp {
					return fmt.Errorf("host key mismatch for %s (possible MITM)", hostPort)
				}
				return nil
			}
			// TOFU: store first seen
			_ = m.keys.SetHostKey(hostPort, fp)
			return nil
		}
		// No store: accept first key (dev / tests)
		_ = hostname
		_ = remote
		return nil
	}
}

func fingerprint(key ssh.PublicKey) string {
	return key.Type() + " " + base64.StdEncoding.EncodeToString(key.Marshal())
}
