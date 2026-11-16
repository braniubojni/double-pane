package remote

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/erikharutyunyan/double-pane/internal/domain"
)

const (
	ftpDialTimeout = 15 * time.Second
	ftpCmdTimeout  = 30 * time.Second
)

// FTPManager holds live plain-FTP sessions (one control connection per user@host:port).
type FTPManager struct {
	mu       sync.Mutex
	sessions map[string]*ftpSession
}

type ftpSession struct {
	Spec Spec
	conn net.Conn
	br   *bufio.Reader
	home string
	mu   sync.Mutex
}

type ftpReply struct {
	code int
	text string
}

type ftpStatusError struct {
	code int
	text string
}

func (e *ftpStatusError) Error() string {
	return fmt.Sprintf("ftp %d %s", e.code, strings.TrimSpace(e.text))
}

// NewFTPManager creates an empty FTP session pool.
func NewFTPManager() *FTPManager {
	return &FTPManager{sessions: make(map[string]*ftpSession)}
}

// Connect dials a plain FTP control connection and logs in.
// password may be empty. Anonymous FTP is user "anonymous" with an empty password.
func (m *FTPManager) Connect(spec Spec, password string) error {
	if m == nil {
		return fmt.Errorf("remote not available")
	}
	spec.Scheme = "ftp"
	if spec.Port <= 0 {
		spec.Port = 21
	}
	if strings.TrimSpace(spec.User) == "" {
		return fmt.Errorf("missing user in remote path")
	}
	if strings.TrimSpace(spec.Host) == "" {
		return fmt.Errorf("missing host in remote path")
	}
	key := spec.SessionKey()

	m.mu.Lock()
	if _, ok := m.sessions[key]; ok {
		m.mu.Unlock()
		return nil
	}
	m.mu.Unlock()

	log.Printf("gfm: ftp connect host=%q port=%d user=%q passSet=%v", spec.Host, spec.Port, spec.User, password != "")
	sess, err := dialFTP(spec, password)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.sessions[key]; ok {
		_ = sess.conn.Close()
		return nil
	}
	m.sessions[key] = sess
	return nil
}

func dialFTP(spec Spec, password string) (*ftpSession, error) {
	conn, err := net.DialTimeout("tcp", spec.DialAddr(), ftpDialTimeout)
	if err != nil {
		return nil, fmt.Errorf("ftp dial %s: %w", spec.DialAddr(), err)
	}
	s := &ftpSession{Spec: spec, conn: conn, br: bufio.NewReader(conn), home: "/"}
	if err := s.login(password); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return s, nil
}

func (s *ftpSession) login(password string) error {
	greet, err := s.readReply()
	if err != nil {
		return fmt.Errorf("ftp greeting: %w", err)
	}
	if greet.code/100 != 2 {
		return fmt.Errorf("ftp greeting: %s", strings.TrimSpace(greet.text))
	}
	userRep, err := s.cmd("USER " + s.Spec.User)
	if err != nil {
		return err
	}
	switch userRep.code {
	case 230:
	case 331, 332:
		line := "PASS"
		if password != "" {
			line = "PASS " + password
		}
		passRep, err := s.cmd(line)
		if err != nil {
			return err
		}
		if passRep.code/100 != 2 {
			return fmt.Errorf("ftp login failed: %s", strings.TrimSpace(passRep.text))
		}
	default:
		return fmt.Errorf("ftp login failed: %s", strings.TrimSpace(userRep.text))
	}
	typ, err := s.cmd("TYPE I")
	if err != nil {
		return err
	}
	if typ.code/100 != 2 {
		return fmt.Errorf("ftp TYPE I: %s", strings.TrimSpace(typ.text))
	}
	pwd, err := s.cmd("PWD")
	if err != nil {
		return err
	}
	if pwd.code == 257 {
		if home := parse257(pwd.text); home != "" {
			s.home = home
		}
	}
	return nil
}

// Disconnect closes the session for a key or any ftp:// path on that host.
func (m *FTPManager) Disconnect(keyOrPath string) error {
	if m == nil {
		return nil
	}
	key := strings.TrimSpace(keyOrPath)
	if IsFTP(keyOrPath) {
		loc, err := ParseLocation(keyOrPath)
		if err != nil {
			return err
		}
		key = loc.SessionKey()
	}
	m.mu.Lock()
	s, ok := m.sessions[key]
	if ok {
		delete(m.sessions, key)
	}
	m.mu.Unlock()
	if !ok {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, _ = s.cmd("QUIT")
	return s.conn.Close()
}

// ListSessions returns live FTP sessions.
func (m *FTPManager) ListSessions() []domain.ActiveSession {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]domain.ActiveSession, 0, len(m.sessions))
	for _, s := range m.sessions {
		out = append(out, domain.ActiveSession{
			Key:      s.Spec.SessionKey(),
			Protocol: "ftp",
			User:     s.Spec.User,
			Host:     s.Spec.Host,
			Port:     s.Spec.Port,
			RootPath: s.Spec.RootPath(),
		})
	}
	return out
}

// CloseAll closes every FTP session.
func (m *FTPManager) CloseAll() {
	if m == nil {
		return
	}
	m.mu.Lock()
	all := m.sessions
	m.sessions = make(map[string]*ftpSession)
	m.mu.Unlock()
	for _, s := range all {
		s.mu.Lock()
		_, _ = s.cmd("QUIT")
		_ = s.conn.Close()
		s.mu.Unlock()
	}
}

// HomePath is the virtual path of the server's PWD after login, or the root.
func (m *FTPManager) HomePath(spec Spec) (string, error) {
	if m == nil {
		return "", fmt.Errorf("remote not available")
	}
	spec.Scheme = "ftp"
	if spec.Port <= 0 {
		spec.Port = 21
	}
	m.mu.Lock()
	s, ok := m.sessions[spec.SessionKey()]
	m.mu.Unlock()
	if !ok {
		return "", fmt.Errorf("not connected to %s; connect first", spec.SessionKey())
	}
	s.mu.Lock()
	home := s.home
	s.mu.Unlock()
	if home == "" {
		home = "/"
	}
	return spec.JoinPath(home), nil
}

func (m *FTPManager) get(loc Location) (*ftpSession, error) {
	if m == nil {
		return nil, fmt.Errorf("remote not available")
	}
	key := loc.SessionKey()
	m.mu.Lock()
	s, ok := m.sessions[key]
	m.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("not connected to %s; connect first", key)
	}
	return s, nil
}

func (s *ftpSession) cmd(line string) (ftpReply, error) {
	if strings.ContainsAny(line, "\r\n") {
		return ftpReply{}, fmt.Errorf("invalid ftp command")
	}
	_ = s.conn.SetDeadline(time.Now().Add(ftpCmdTimeout))
	defer func() { _ = s.conn.SetDeadline(time.Time{}) }()
	if _, err := io.WriteString(s.conn, line+"\r\n"); err != nil {
		return ftpReply{}, err
	}
	return s.readReply()
}

func (s *ftpSession) readReply() (ftpReply, error) {
	line, err := readCrlf(s.br)
	if err != nil {
		return ftpReply{}, err
	}
	if len(line) < 3 {
		return ftpReply{}, fmt.Errorf("ftp short reply %q", line)
	}
	code, err := strconv.Atoi(line[:3])
	if err != nil {
		return ftpReply{}, fmt.Errorf("ftp bad reply %q", line)
	}
	if len(line) == 3 || line[3] == ' ' {
		msg := ""
		if len(line) > 4 {
			msg = line[4:]
		}
		return ftpReply{code: code, text: msg}, nil
	}
	if line[3] != '-' {
		return ftpReply{}, fmt.Errorf("ftp bad reply %q", line)
	}
	var b strings.Builder
	if len(line) > 4 {
		b.WriteString(line[4:])
	}
	prefix := line[:3]
	for {
		next, err := readCrlf(s.br)
		if err != nil {
			return ftpReply{}, err
		}
		if len(next) >= 4 && strings.HasPrefix(next, prefix) && next[3] == ' ' {
			if b.Len() > 0 {
				b.WriteByte('\n')
			}
			if len(next) > 4 {
				b.WriteString(next[4:])
			}
			break
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(next)
	}
	return ftpReply{code: code, text: b.String()}, nil
}

func readCrlf(br *bufio.Reader) (string, error) {
	s, err := br.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimRight(s, "\r\n"), nil
}

func (s *ftpSession) passive() (net.Conn, error) {
	tcp, _ := s.conn.RemoteAddr().(*net.TCPAddr)
	if tcp != nil && tcp.IP.To4() == nil {
		return s.dialEPSV(tcp)
	}
	return s.dialPASV(tcp)
}

func (s *ftpSession) dialPASV(ctrl *net.TCPAddr) (net.Conn, error) {
	rep, err := s.cmd("PASV")
	if err != nil {
		return nil, err
	}
	if rep.code != 227 {
		if ctrl != nil && ctrl.IP.To4() == nil {
			return s.dialEPSV(ctrl)
		}
		return nil, &ftpStatusError{code: rep.code, text: rep.text}
	}
	host, port, err := parsePASV(rep.text)
	if err != nil {
		return nil, err
	}
	if host == "0.0.0.0" || host == "" {
		if ctrl == nil {
			return nil, fmt.Errorf("ftp PASV returned %s", host)
		}
		host = ctrl.IP.String()
	}
	return net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(port)), ftpDialTimeout)
}

func (s *ftpSession) dialEPSV(ctrl *net.TCPAddr) (net.Conn, error) {
	rep, err := s.cmd("EPSV")
	if err != nil {
		return nil, err
	}
	if rep.code != 229 {
		return nil, &ftpStatusError{code: rep.code, text: rep.text}
	}
	port, err := parseEPSV(rep.text)
	if err != nil {
		return nil, err
	}
	if ctrl == nil {
		return nil, fmt.Errorf("ftp EPSV: missing control address")
	}
	return net.DialTimeout("tcp", net.JoinHostPort(ctrl.IP.String(), strconv.Itoa(port)), ftpDialTimeout)
}

func parsePASV(msg string) (string, int, error) {
	i := strings.LastIndex(msg, "(")
	j := strings.LastIndex(msg, ")")
	if i < 0 || j <= i {
		return "", 0, fmt.Errorf("ftp PASV: %s", msg)
	}
	parts := strings.Split(msg[i+1:j], ",")
	if len(parts) != 6 {
		return "", 0, fmt.Errorf("ftp PASV: %s", msg)
	}
	nums := make([]int, 6)
	for n, p := range parts {
		v, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil || v < 0 || v > 255 {
			return "", 0, fmt.Errorf("ftp PASV: %s", msg)
		}
		nums[n] = v
	}
	host := fmt.Sprintf("%d.%d.%d.%d", nums[0], nums[1], nums[2], nums[3])
	return host, nums[4]*256 + nums[5], nil
}

func parseEPSV(msg string) (int, error) {
	i := strings.LastIndex(msg, "(")
	j := strings.LastIndex(msg, ")")
	if i < 0 || j <= i+1 {
		return 0, fmt.Errorf("ftp EPSV: %s", msg)
	}
	inner := msg[i+1 : j]
	d := inner[0]
	parts := strings.Split(inner, string(d))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		port, err := strconv.Atoi(p)
		if err != nil || port <= 0 || port > 65535 {
			return 0, fmt.Errorf("ftp EPSV: %s", msg)
		}
		return port, nil
	}
	return 0, fmt.Errorf("ftp EPSV: %s", msg)
}

func parse257(msg string) string {
	i := strings.IndexByte(msg, '"')
	if i < 0 {
		return "/"
	}
	var b strings.Builder
	for j := i + 1; j < len(msg); j++ {
		if msg[j] == '"' {
			if j+1 < len(msg) && msg[j+1] == '"' {
				b.WriteByte('"')
				j++
				continue
			}
			break
		}
		b.WriteByte(msg[j])
	}
	p := b.String()
	if p == "" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return cleanFTPPath(p)
}

func ftpArg(p string) (string, error) {
	if strings.ContainsAny(p, "\r\n") {
		return "", fmt.Errorf("invalid ftp path")
	}
	if strings.TrimSpace(p) == "" {
		return "/", nil
	}
	return p, nil
}

func cleanFTPPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	for strings.Contains(p, "//") {
		p = strings.ReplaceAll(p, "//", "/")
	}
	if len(p) > 1 {
		p = strings.TrimRight(p, "/")
	}
	return p
}
