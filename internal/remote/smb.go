package remote

import (
	"context"
	"fmt"
	"log"
	"net"
	"os/user"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/cloudsoda/go-smb2"
	"github.com/erikharutyunyan/double-pane/internal/domain"
)

// SMBManager holds live SMB sessions (one per host; shares mount lazily).
type SMBManager struct {
	mu       sync.Mutex
	sessions map[string]*smbSession
}

type smbSession struct {
	Spec       Spec // original spec (user/domain as requested, not NTLM-resolved)
	ntlmUser   string
	ntlmDomain string
	client     *smb2.Session
	mounts     map[string]*smb2.Share
}

// NewSMBManager creates an empty SMB session pool.
func NewSMBManager() *SMBManager {
	return &SMBManager{sessions: make(map[string]*smbSession)}
}

// Connect dials SMB over TCP and authenticates with NTLM.
// password may be empty. Empty user uses the OS login name (Finder-like), not Guest.
func (m *SMBManager) Connect(spec Spec, password string) error {
	if m == nil {
		return fmt.Errorf("remote not available")
	}
	spec.Scheme = "smb"
	if spec.Port <= 0 {
		spec.Port = 445
	}
	spec.Host = normalizeSMBDialHost(spec.Host)
	if err := validateSMBHost(spec.Host); err != nil {
		return err
	}
	key := spec.SessionKey()

	m.mu.Lock()
	if _, ok := m.sessions[key]; ok {
		m.mu.Unlock()
		return nil
	}
	m.mu.Unlock()

	ntlmUser, ntlmDomain, userSource := resolveSMBUser(spec.User, spec.Domain)
	spec.Domain = ntlmDomain
	addr := spec.DialAddr()
	log.Printf("gfm: smb connect start host=%q port=%d user=%q domain=%q userSource=%s os=%s passSet=%v",
		spec.Host, spec.Port, ntlmUser, spec.Domain, userSource, runtime.GOOS, password != "")

	if err := probeSMB(addr); err != nil {
		class := classifySMBError(err)
		log.Printf("gfm: smb tcp probe addr=%s ok=false class=%s err=%v", addr, class, err)
		return wrapSMBConnectError(addr, err, class)
	}
	log.Printf("gfm: smb tcp probe addr=%s ok=true", addr)

	var last error
	for _, try := range ntlmAttempts(ntlmUser, ntlmDomain, password) {
		log.Printf("gfm: smb ntlm try addr=%s user=%q domain=%q source=%s passSet=%v",
			addr, try.user, try.domain, try.source, try.pass != "")
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		d := &smb2.Dialer{
			Initiator: &smb2.NTLMInitiator{
				User:     try.user,
				Password: try.pass,
				Domain:   try.domain,
			},
		}
		client, err := d.Dial(ctx, addr)
		cancel()
		if err != nil {
			class := classifySMBError(err)
			log.Printf("gfm: smb ntlm fail addr=%s user=%q source=%s class=%s err=%v",
				addr, try.user, try.source, class, err)
			last = wrapSMBConnectError(addr, err, class)
			if class != smbClassAuth {
				return last
			}
			continue
		}
		log.Printf("gfm: smb ntlm ok addr=%s user=%q source=%s", addr, try.user, try.source)
		m.mu.Lock()
		defer m.mu.Unlock()
		if _, ok := m.sessions[key]; ok {
			_ = client.Logoff()
			return nil
		}
		m.sessions[key] = &smbSession{
			Spec:       spec,
			ntlmUser:   try.user,
			ntlmDomain: try.domain,
			client:     client,
			mounts:     make(map[string]*smb2.Share),
		}
		return nil
	}
	if last == nil {
		last = fmt.Errorf("authentication required: no SMB logon method succeeded for %s", addr)
	}
	return last
}

type ntlmTry struct {
	user, pass, domain, source string
}

// ntlmAttempts builds the NTLM try list from the *resolved* user (after
// resolveSMBUser). Connect never passes a raw empty username: empty form user
// becomes the OS login (or "Guest" if that fails), so account tries run before
// Guest. This helper itself: non-Guest user → account first, then Guest fallback;
// user empty or already "Guest" → Guest-only (anonymous browse).
func ntlmAttempts(user, domain, password string) []ntlmTry {
	var out []ntlmTry
	add := func(t ntlmTry) {
		for _, e := range out {
			if e.user == t.user && e.pass == t.pass && e.domain == t.domain {
				return
			}
		}
		out = append(out, t)
	}
	hasAccount := user != "" && !strings.EqualFold(user, "Guest")

	if hasAccount {
		add(ntlmTry{user: user, pass: password, domain: domain, source: "account"})
		if domain == "" {
			add(ntlmTry{user: user, pass: password, domain: "WORKGROUP", source: "account-workgroup"})
		}
	}
	add(ntlmTry{user: "Guest", pass: "", domain: domain, source: "guest"})
	add(ntlmTry{user: "Guest", pass: "", domain: "WORKGROUP", source: "guest-workgroup"})
	return out
}

func resolveSMBUser(explicit, domain string) (name, outDomain, source string) {
	name = strings.TrimSpace(explicit)
	outDomain = strings.TrimSpace(domain)
	if name != "" {
		source = "explicit"
	} else if u, err := user.Current(); err == nil {
		name = strings.TrimSpace(u.Username)
		source = "os"
	}
	n, d := splitDomainUser(name)
	name = n
	if outDomain == "" {
		outDomain = d
	}
	if name == "" {
		return "Guest", outDomain, "guest"
	}
	return name, outDomain, source
}

func normalizeSMBDialHost(host string) string {
	h := strings.TrimSpace(strings.Trim(host, `"'`))
	if strings.HasPrefix(h, `\\`) || strings.HasPrefix(h, "//") {
		rest := strings.TrimLeft(h, `/\`)
		if i := strings.IndexAny(rest, `/\`); i >= 0 {
			rest = rest[:i]
		}
		return rest
	}
	return h
}

func validateSMBHost(host string) error {
	h := strings.TrimSpace(host)
	if h == "" || strings.EqualFold(h, "smb") || strings.EqualFold(h, "smb:") {
		return fmt.Errorf("invalid SMB host %q; enter a computer name or IP (e.g. 192.168.0.10), not the protocol", host)
	}
	if strings.Contains(h, "://") {
		return fmt.Errorf("invalid SMB host %q; paste only the computer name or IP", host)
	}
	return nil
}

func probeSMB(addr string) error {
	c, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return err
	}
	return c.Close()
}

func wrapSMBConnectError(addr string, err error, class string) error {
	switch class {
	case smbClassAuth:
		return fmt.Errorf("authentication required: logon failed for %s (bad username, password, or domain): %w", addr, err)
	case smbClassNetwork:
		return fmt.Errorf("cannot reach %s: %w; %s", addr, err, smbNetworkHint())
	default:
		return fmt.Errorf("smb dial %s: %w", addr, err)
	}
}

func smbNetworkHint() string {
	switch runtime.GOOS {
	case "darwin":
		return "on macOS, allow Local Network for this app in System Settings → Privacy & Security → Local Network, then retry"
	case "windows":
		return "on Windows, allow this app through Windows Defender Firewall and check that TCP 445 is not blocked (VPN/antivirus)"
	default:
		return "on Linux, confirm the host answers on TCP 445 and that ufw/firewalld is not blocking outbound SMB"
	}
}

// Disconnect closes a session by key or any path under that host.
func (m *SMBManager) Disconnect(keyOrPath string) error {
	if m == nil {
		return nil
	}
	key := keyOrPath
	if IsSMB(keyOrPath) {
		loc, err := ParseLocation(keyOrPath)
		if err != nil {
			return err
		}
		key = loc.SessionKey()
	} else if strings.HasPrefix(key, "smb:") {
		// already a session key
	} else {
		// SSH-shaped key — not ours
		return nil
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
	return sess.close()
}

func (s *smbSession) close() error {
	for name, fs := range s.mounts {
		_ = fs.Umount()
		delete(s.mounts, name)
	}
	if s.client != nil {
		return s.client.Logoff()
	}
	return nil
}

// ListSessions returns active SMB sessions.
func (m *SMBManager) ListSessions() []domain.ActiveSession {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]domain.ActiveSession, 0, len(m.sessions))
	for _, s := range m.sessions {
		out = append(out, domain.ActiveSession{
			Key:      s.Spec.SessionKey(),
			Protocol: "smb",
			User:     s.Spec.User,
			Host:     s.Spec.Host,
			Port:     s.Spec.Port,
			RootPath: s.Spec.RootPath(),
		})
	}
	return out
}

// CloseAll closes every SMB session (app shutdown).
func (m *SMBManager) CloseAll() {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, s := range m.sessions {
		_ = s.close()
		delete(m.sessions, k)
	}
}

// ListShares enumerates share names on a connected host.
func (m *SMBManager) ListShares(keyOrPath string, showHidden bool) ([]domain.SMBShare, error) {
	sess, err := m.sessionByKeyOrPath(keyOrPath)
	if err != nil {
		return nil, err
	}
	names, err := sess.client.ListSharenames()
	if err != nil {
		log.Printf("gfm: smb list shares key=%s err=%v", keyOrPath, err)
		return nil, fmt.Errorf("list shares: %w", err)
	}
	out := FilterSMBShares(names, showHidden)
	log.Printf("gfm: smb list shares key=%s raw=%d visible=%d showHidden=%v", keyOrPath, len(names), len(out), showHidden)
	return out, nil
}

func (m *SMBManager) sessionByKeyOrPath(keyOrPath string) (*smbSession, error) {
	if m == nil {
		return nil, fmt.Errorf("remote not available")
	}
	key := keyOrPath
	if IsSMB(keyOrPath) {
		loc, err := ParseLocation(keyOrPath)
		if err != nil {
			return nil, err
		}
		key = loc.SessionKey()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[key]
	if !ok {
		return nil, fmt.Errorf("not connected to %s; connect first", key)
	}
	return s, nil
}

func (m *SMBManager) get(loc Location) (*smbSession, error) {
	return m.sessionByKeyOrPath(loc.SessionKey())
}

func (m *SMBManager) shareFS(loc Location) (*smb2.Share, error) {
	share := loc.ShareName()
	if share == "" {
		return nil, fmt.Errorf("no share selected")
	}
	sess, err := m.get(loc)
	if err != nil {
		return nil, err
	}
	sessKey := strings.ToLower(share)
	m.mu.Lock()
	if fs, ok := sess.mounts[sessKey]; ok {
		m.mu.Unlock()
		return fs, nil
	}
	m.mu.Unlock()
	fs, err := sess.client.Mount(share)
	if err != nil {
		return nil, fmt.Errorf("mount %s: %w", share, err)
	}
	m.mu.Lock()
	if existing, ok := sess.mounts[sessKey]; ok {
		m.mu.Unlock()
		_ = fs.Umount()
		return existing, nil
	}
	sess.mounts[sessKey] = fs
	m.mu.Unlock()
	return fs, nil
}

// FilterSMBShares drops IPC$ always and $-suffix admin shares unless showHidden.
func FilterSMBShares(names []string, showHidden bool) []domain.SMBShare {
	out := make([]domain.SMBShare, 0, len(names))
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n == "" || strings.EqualFold(n, "IPC$") {
			continue
		}
		hidden := strings.HasSuffix(n, "$")
		if hidden && !showHidden {
			continue
		}
		out = append(out, domain.SMBShare{Name: n, Hidden: hidden})
	}
	return out
}

const (
	smbClassAuth    = "auth"
	smbClassNetwork = "network"
	smbClassOther   = "other"
)

func classifySMBError(err error) string {
	if err == nil {
		return smbClassOther
	}
	msg := strings.ToLower(err.Error())
	if isSMBNetworkErrorMsg(msg) {
		return smbClassNetwork
	}
	if isSMBAuthErrorMsg(msg) {
		return smbClassAuth
	}
	return smbClassOther
}

func isSMBNetworkErrorMsg(msg string) bool {
	for _, sub := range []string{
		"no route to host",
		"no such host",
		"network is unreachable",
		"connection refused",
		"i/o timeout",
		"operation timed out",
		"connection timed out",
		"host is down",
		"network down",
		"permission denied",
		"operation not permitted",
		"local network",
		// Windows (connectex / WSA*)
		"unreachable host",
		"unreachable network",
		"connection attempt failed",
		"actively refused",
		"forcibly closed",
		"semaphore timeout",
		"forbidden by its access permissions",
		"connectex",
	} {
		if strings.Contains(msg, sub) {
			return true
		}
	}
	return false
}

func isSMBAuthErrorMsg(msg string) bool {
	for _, sub := range []string{
		"logon is invalid",
		"logon failure",
		"logon_failure",
		"bad username",
		"authentication information",
		"authentication required",
		"unable to authenticate",
		"logon type not granted",
		"unknown user",
		"wrong password",
		"status_logon_failure",
		"status_logon_type_not_granted",
		"status_account_locked_out",
		"status_account_disabled",
		"status_password_must_change",
		"status_password_expired",
		"logon type has not been granted",
	} {
		if strings.Contains(msg, sub) {
			return true
		}
	}
	return false
}

func isSMBAuthError(err error) bool {
	return err != nil && isSMBAuthErrorMsg(strings.ToLower(err.Error()))
}

// IsSMBAuthError reports a failed NTLM/guest logon (not a network failure).
func IsSMBAuthError(err error) bool { return isSMBAuthError(err) }

func smbRel(loc Location) string {
	rel := loc.PathOnShare()
	if rel == "" {
		return "."
	}
	return rel
}

func joinOnShare(share, rel, name string) string {
	rel = strings.Trim(strings.ReplaceAll(rel, "\\", "/"), "/")
	if rel == "" || rel == "." {
		return "/" + share + "/" + name
	}
	return "/" + share + "/" + rel + "/" + name
}
