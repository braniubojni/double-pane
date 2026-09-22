package remote

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/erikharutyunyan/double-pane/internal/domain"
	mega "github.com/t3rm1n4l/go-mega"
)

// MEGAManager holds live MEGA API sessions (one per account email).
type MEGAManager struct {
	mu       sync.Mutex
	sessions map[string]*megaSession
}

type megaSession struct {
	Spec   Spec
	client *mega.Mega
	http   *http.Client
}

func (s *megaSession) closeIdleConns() {
	if s == nil || s.http == nil {
		return
	}
	if tr, ok := s.http.Transport.(*http.Transport); ok {
		tr.CloseIdleConnections()
	}
}

func newMegaHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 60 * time.Second,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
		},
	}
}

// NewMEGAManager creates an empty MEGA session pool.
func NewMEGAManager() *MEGAManager {
	return &MEGAManager{sessions: make(map[string]*megaSession)}
}

// Connect logs into MEGA. totp may be empty when 2FA is off.
func (m *MEGAManager) Connect(spec Spec, password, totp string) error {
	if m == nil {
		return fmt.Errorf("remote not available")
	}
	spec.Scheme = "mega"
	if spec.User == "" || spec.Host == "" {
		return fmt.Errorf("invalid MEGA email")
	}
	key := spec.SessionKey()

	// ponytail: hold the lock across the login call so two concurrent
	// Connect calls for the same account can't both log in and leak a
	// session; login is a rare, user-triggered op so serializing it is fine.
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.sessions[key]; ok {
		return nil
	}

	httpClient := newMegaHTTPClient()
	client := mega.New()
	client.SetClient(httpClient)
	if err := client.MultiFactorLogin(spec.MEGAEmail(), password, totp); err != nil {
		return wrapMegaLogin(err)
	}
	m.sessions[key] = &megaSession{Spec: spec, client: client, http: httpClient}
	return nil
}

func wrapMegaLogin(err error) error {
	if err == nil {
		return nil
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "no such host") ||
		strings.Contains(msg, "network is unreachable") ||
		strings.Contains(msg, "connection refused") {
		return fmt.Errorf("cannot reach MEGA: %w", err)
	}
	return fmt.Errorf("authentication required: MEGA login failed: %w", err)
}

// Disconnect closes a session by key or any mega:// path under that account.
func (m *MEGAManager) Disconnect(keyOrPath string) error {
	if m == nil {
		return nil
	}
	key := keyOrPath
	switch {
	case IsMEGA(keyOrPath):
		loc, err := ParseLocation(keyOrPath)
		if err != nil {
			return err
		}
		key = loc.SessionKey()
	case strings.HasPrefix(key, "mega:"):
		// already a session key
	default:
		return nil
	}
	m.mu.Lock()
	delete(m.sessions, key)
	m.mu.Unlock()
	return nil
}

// ListSessions returns active MEGA sessions.
func (m *MEGAManager) ListSessions() []domain.ActiveSession {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]domain.ActiveSession, 0, len(m.sessions))
	for _, s := range m.sessions {
		out = append(out, domain.ActiveSession{
			Key:      s.Spec.SessionKey(),
			Protocol: "mega",
			User:     s.Spec.User,
			Host:     s.Spec.Host,
			RootPath: s.Spec.RootPath(),
		})
	}
	return out
}

// CloseAll drops every MEGA session (app shutdown).
func (m *MEGAManager) CloseAll() {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions = make(map[string]*megaSession)
}

func (m *MEGAManager) get(loc Location) (*megaSession, error) {
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
