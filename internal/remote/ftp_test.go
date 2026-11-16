package remote

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestParsePASVAndEPSV(t *testing.T) {
	t.Parallel()
	host, port, err := parsePASV("Entering Passive Mode (127,0,0,1,4,1)")
	if err != nil {
		t.Fatal(err)
	}
	if host != "127.0.0.1" || port != 4*256+1 {
		t.Fatalf("pasv %s %d", host, port)
	}
	host, _, err = parsePASV("Entering Passive Mode (0,0,0,0,1,2)")
	if err != nil || host != "0.0.0.0" {
		t.Fatalf("zero host %s %v", host, err)
	}
	p, err := parseEPSV("Entering Extended Passive Mode (|||4021|)")
	if err != nil || p != 4021 {
		t.Fatalf("epsv %d %v", p, err)
	}
	if got := parse257(`"/docs" is current directory`); got != "/docs" {
		t.Fatalf("pwd %q", got)
	}
}

func TestParseUnixLISTNameWithSpaces(t *testing.T) {
	t.Parallel()
	body := "drwxr-xr-x 2 user group 4096 Nov 16 12:00 my docs\r\n-rw-r--r-- 1 user group 5 Nov 16 12:00 read me.txt\r\n"
	entries := parseUnixLIST(body)
	if len(entries) != 2 || !entries[0].isDir || entries[0].name != "my docs" || entries[1].name != "read me.txt" || entries[1].size != 5 {
		t.Fatalf("%+v", entries)
	}
}

func TestFTPAnonymousAndPassword(t *testing.T) {
	host, port := startFTPStub(t, false)
	m := NewFTPManager()

	anon := Spec{Scheme: "ftp", User: "anonymous", Host: host, Port: port}
	if err := m.Connect(anon, ""); err != nil {
		t.Fatal(err)
	}
	if err := m.Connect(anon, ""); err != nil {
		t.Fatal(err)
	}
	root := anon.RootPath()
	entries, err := m.ListDir(root, false)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]domainName{}
	for _, e := range entries {
		names[e.Name] = domainName{dir: e.IsDir, path: e.Path}
	}
	if _, ok := names[".secret"]; ok {
		t.Fatal("hidden file listed")
	}
	if !names["docs"].dir || names["readme.txt"].dir {
		t.Fatalf("entries %+v", names)
	}
	if !strings.HasPrefix(names["readme.txt"].path, "ftp://anonymous@") {
		t.Fatalf("path %s", names["readme.txt"].path)
	}
	text, err := m.ReadTextFile(names["readme.txt"].path)
	if err != nil || text != "hello" {
		t.Fatalf("read %q %v", text, err)
	}
	rc, err := m.OpenRead(names["readme.txt"].path)
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(rc)
	_ = rc.Close()
	if err != nil || string(body) != "hello" {
		t.Fatalf("open %q %v", body, err)
	}
	if err := m.WriteTextFile(anon.JoinPath("/note.txt"), "hi"); err != nil {
		t.Fatal(err)
	}
	got, err := m.ReadTextFile(anon.JoinPath("/note.txt"))
	if err != nil || got != "hi" {
		t.Fatalf("note %q %v", got, err)
	}
	if _, err := m.Mkdir(root, "extra"); err != nil {
		t.Fatal(err)
	}
	if err := m.CopyWithin([]string{anon.JoinPath("/readme.txt")}, anon.JoinPath("/docs")); err != nil {
		t.Fatal(err)
	}
	copied, err := m.ReadTextFile(anon.JoinPath("/docs/readme.txt"))
	if err != nil || copied != "hello" {
		t.Fatalf("copy %q %v", copied, err)
	}
	local := t.TempDir()
	if err := m.Download([]string{anon.JoinPath("/readme.txt")}, local); err != nil {
		t.Fatal(err)
	}
	down, err := os.ReadFile(filepath.Join(local, "readme.txt"))
	if err != nil || string(down) != "hello" {
		t.Fatalf("download %q %v", down, err)
	}
	up := filepath.Join(local, "up.txt")
	if err := os.WriteFile(up, []byte("up"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := m.Upload([]string{up}, root); err != nil {
		t.Fatal(err)
	}
	gotUp, err := m.ReadTextFile(anon.JoinPath("/up.txt"))
	if err != nil || gotUp != "up" {
		t.Fatalf("upload %q %v", gotUp, err)
	}
	if err := m.Delete([]string{anon.JoinPath("/readme.txt")}); err != nil {
		t.Fatal(err)
	}
	ok, err := m.Exists(anon.JoinPath("/readme.txt"))
	if err != nil || ok {
		t.Fatalf("exists after delete %v %v", ok, err)
	}
	home, err := m.HomePath(anon)
	if err != nil || home != root {
		t.Fatalf("home %s %v", home, err)
	}
	if err := m.Disconnect(root); err != nil {
		t.Fatal(err)
	}

	if err := m.Connect(Spec{Scheme: "ftp", User: "bob", Host: host, Port: port}, "nope"); err == nil || !strings.Contains(err.Error(), "ftp login failed") {
		t.Fatalf("bad password: %v", err)
	}
	bob := Spec{Scheme: "ftp", User: "bob", Host: host, Port: port}
	if err := m.Connect(bob, "secret"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.ListDir(bob.RootPath(), true); err != nil {
		t.Fatal(err)
	}
	nopass := Spec{Scheme: "ftp", User: "nopass", Host: host, Port: port}
	if err := NewFTPManager().Connect(nopass, ""); err != nil {
		t.Fatal(err)
	}
}

func TestFTPListFallback(t *testing.T) {
	host, port := startFTPStub(t, true)
	m := NewFTPManager()
	spec := Spec{Scheme: "ftp", User: "anonymous", Host: host, Port: port}
	if err := m.Connect(spec, ""); err != nil {
		t.Fatal(err)
	}
	entries, err := m.ListDir(spec.RootPath(), true)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) < 2 {
		t.Fatalf("entries %+v", entries)
	}
}

func TestFTPNotConnected(t *testing.T) {
	t.Parallel()
	m := NewFTPManager()
	if _, err := m.ListDir("ftp://u@h:21/", true); err == nil || !strings.Contains(err.Error(), "not connected") {
		t.Fatalf("list: %v", err)
	}
	if err := m.Download([]string{"ftp://u@h:21/a"}, t.TempDir()); err == nil {
		t.Fatal("expected not connected")
	}
}

type domainName struct {
	dir  bool
	path string
}

func startFTPStub(t *testing.T, noMLSD bool) (string, int) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	fs := newMemFS()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go serveFTPClient(c, fs, noMLSD)
		}
	}()
	addr := ln.Addr().(*net.TCPAddr)
	return addr.IP.String(), addr.Port
}

type memNode struct {
	isDir bool
	body  []byte
}

type memFS struct {
	mu    sync.Mutex
	nodes map[string]memNode
}

func newMemFS() *memFS {
	return &memFS{nodes: map[string]memNode{
		"/":           {isDir: true},
		"/docs":       {isDir: true},
		"/readme.txt": {isDir: false, body: []byte("hello")},
		"/.secret":    {isDir: false, body: []byte("x")},
	}}
}

func (fs *memFS) list(dir string) ([]memNode, []string, bool) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	dir = cleanFTPPath(dir)
	n, ok := fs.nodes[dir]
	if !ok || !n.isDir {
		return nil, nil, false
	}
	var names []string
	for p, node := range fs.nodes {
		if p != dir && path.Dir(p) == dir {
			names = append(names, p)
			_ = node
		}
	}
	sort.Strings(names)
	out := make([]memNode, len(names))
	for i, p := range names {
		out[i] = fs.nodes[p]
		names[i] = path.Base(p)
	}
	return out, names, true
}

func (fs *memFS) file(p string) ([]byte, bool) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	n, ok := fs.nodes[cleanFTPPath(p)]
	if !ok || n.isDir {
		return nil, false
	}
	return append([]byte(nil), n.body...), true
}

func (fs *memFS) put(p string, body []byte) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.nodes[cleanFTPPath(p)] = memNode{body: append([]byte(nil), body...)}
}

func (fs *memFS) mkdir(p string) bool {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	p = cleanFTPPath(p)
	if _, ok := fs.nodes[p]; ok {
		return false
	}
	fs.nodes[p] = memNode{isDir: true}
	return true
}

func (fs *memFS) remove(p string) bool {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	p = cleanFTPPath(p)
	n, ok := fs.nodes[p]
	if !ok || n.isDir {
		return false
	}
	delete(fs.nodes, p)
	return true
}

func (fs *memFS) rmdir(p string) bool {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	p = cleanFTPPath(p)
	n, ok := fs.nodes[p]
	if !ok || !n.isDir || p == "/" {
		return false
	}
	for child := range fs.nodes {
		if child != p && strings.HasPrefix(child, p+"/") {
			return false
		}
	}
	delete(fs.nodes, p)
	return true
}

func (fs *memFS) rename(from, to string) bool {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	from, to = cleanFTPPath(from), cleanFTPPath(to)
	if _, ok := fs.nodes[from]; !ok {
		return false
	}
	type pair struct {
		p string
		n memNode
	}
	var moved []pair
	for p, n := range fs.nodes {
		if p == from || strings.HasPrefix(p, from+"/") {
			moved = append(moved, pair{p, n})
		}
	}
	for _, it := range moved {
		delete(fs.nodes, it.p)
	}
	for _, it := range moved {
		fs.nodes[to+strings.TrimPrefix(it.p, from)] = it.n
	}
	return true
}

func serveFTPClient(conn net.Conn, fs *memFS, noMLSD bool) {
	defer func() { _ = conn.Close() }()
	st := &ftpStub{c: conn, br: bufio.NewReader(conn), fs: fs, noMLSD: noMLSD}
	_, _ = fmt.Fprintf(conn, "220 stub ready\r\n")
	for {
		_ = conn.SetDeadline(time.Now().Add(20 * time.Second))
		line, err := readCrlf(st.br)
		if err != nil {
			return
		}
		verb, arg := splitFTPCmd(line)
		if !st.logged {
			st.login(verb, arg)
			continue
		}
		switch verb {
		case "TYPE":
			st.write(200, "Type set")
		case "PWD":
			st.write(257, `"/"`)
		case "PASV":
			st.pasv()
		case "MLSD":
			if st.noMLSD {
				st.write(502, "MLSD not implemented")
				st.closeData()
				continue
			}
			st.sendList(arg, true)
		case "LIST":
			st.sendList(arg, false)
		case "RETR":
			st.retr(arg)
		case "STOR":
			st.stor(arg)
		case "MKD":
			if fs.mkdir(arg) {
				st.write(257, "Created")
			} else {
				st.write(550, "Create failed")
			}
		case "DELE":
			if fs.remove(arg) {
				st.write(250, "Deleted")
			} else {
				st.write(550, "Not a file")
			}
		case "RMD":
			if fs.rmdir(arg) {
				st.write(250, "Removed")
			} else {
				st.write(550, "Not removed")
			}
		case "RNFR":
			st.rnfr = cleanFTPPath(arg)
			st.write(350, "Ready")
		case "RNTO":
			if fs.rename(st.rnfr, arg) {
				st.write(250, "Renamed")
			} else {
				st.write(550, "Rename failed")
			}
			st.rnfr = ""
		case "QUIT":
			st.write(221, "Bye")
			return
		default:
			st.write(502, "Not implemented")
		}
	}
}

type ftpStub struct {
	c      net.Conn
	br     *bufio.Reader
	fs     *memFS
	dataLn net.Listener
	noMLSD bool
	user   string
	rnfr   string
	logged bool
}

func (st *ftpStub) write(code int, msg string) {
	_, _ = fmt.Fprintf(st.c, "%d %s\r\n", code, msg)
}

func (st *ftpStub) login(verb, arg string) {
	switch verb {
	case "USER":
		if arg == "nopass" {
			st.logged = true
			st.write(230, "Logged in")
			return
		}
		if arg == "anonymous" || arg == "bob" {
			st.user = arg
			st.write(331, "Password required")
			return
		}
		st.write(530, "Login incorrect")
	case "PASS":
		if st.user == "anonymous" || (st.user == "bob" && arg == "secret") {
			st.logged = true
			st.write(230, "Logged in")
			return
		}
		st.write(530, "Login incorrect")
	default:
		st.write(530, "Login required")
	}
}

func (st *ftpStub) pasv() {
	st.closeData()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		st.write(425, "Can't open passive")
		return
	}
	st.dataLn = ln
	port := ln.Addr().(*net.TCPAddr).Port
	st.write(227, fmt.Sprintf("Entering Passive Mode (127,0,0,1,%d,%d)", port/256, port%256))
}

func (st *ftpStub) closeData() {
	if st.dataLn != nil {
		_ = st.dataLn.Close()
		st.dataLn = nil
	}
}

func (st *ftpStub) acceptData() (net.Conn, error) {
	if st.dataLn == nil {
		return nil, fmt.Errorf("no pasv")
	}
	if tcp, ok := st.dataLn.(*net.TCPListener); ok {
		_ = tcp.SetDeadline(time.Now().Add(5 * time.Second))
	}
	c, err := st.dataLn.Accept()
	st.closeData()
	return c, err
}

func (st *ftpStub) sendList(arg string, mlsd bool) {
	nodes, names, ok := st.fs.list(arg)
	if !ok {
		st.write(550, "Not a directory")
		st.closeData()
		return
	}
	var b strings.Builder
	for i, name := range names {
		if mlsd {
			kind := "file"
			if nodes[i].isDir {
				kind = "dir"
			}
			fmt.Fprintf(&b, "type=%s;size=%d;modify=20240101120000; %s\r\n", kind, len(nodes[i].body), name)
			continue
		}
		perm := "-rw-r--r-- 1 user group"
		if nodes[i].isDir {
			perm = "drwxr-xr-x 2 user group"
		}
		fmt.Fprintf(&b, "%s %d Nov 16 12:00 %s\r\n", perm, len(nodes[i].body), name)
	}
	st.write(150, "Opening data connection")
	dc, err := st.acceptData()
	if err != nil {
		st.write(425, "Can't open data")
		return
	}
	_, _ = io.WriteString(dc, b.String())
	_ = dc.Close()
	st.write(226, "Transfer complete")
}

func (st *ftpStub) retr(arg string) {
	body, ok := st.fs.file(arg)
	if !ok {
		st.write(550, "Not a file")
		st.closeData()
		return
	}
	st.write(150, "Opening data connection")
	dc, err := st.acceptData()
	if err != nil {
		st.write(425, "Can't open data")
		return
	}
	_, _ = dc.Write(body)
	_ = dc.Close()
	st.write(226, "Transfer complete")
}

func (st *ftpStub) stor(arg string) {
	st.write(150, "Opening data connection")
	dc, err := st.acceptData()
	if err != nil {
		st.write(425, "Can't open data")
		return
	}
	body, err := io.ReadAll(dc)
	_ = dc.Close()
	if err != nil {
		st.write(426, "Transfer aborted")
		return
	}
	st.fs.put(arg, body)
	st.write(226, "Transfer complete")
}

func splitFTPCmd(line string) (string, string) {
	line = strings.TrimSpace(line)
	i := strings.IndexByte(line, ' ')
	if i < 0 {
		return strings.ToUpper(line), ""
	}
	return strings.ToUpper(line[:i]), strings.TrimSpace(line[i+1:])
}
