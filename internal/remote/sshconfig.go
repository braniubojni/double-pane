package remote

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/erikharutyunyan/double-pane/internal/domain"
)

// LoadDefaultSSHConfigHosts parses all default SSH config files (user then system).
// Later files do not override earlier host aliases (first match wins when looking up).
func LoadDefaultSSHConfigHosts() []domain.SSHConfigHost {
	var all []domain.SSHConfigHost
	seen := map[string]bool{}
	for _, p := range DefaultSSHConfigPaths() {
		hosts, err := ParseSSHConfigFile(p)
		if err != nil {
			continue
		}
		for _, h := range hosts {
			key := strings.ToLower(h.Alias)
			if seen[key] {
				continue
			}
			seen[key] = true
			all = append(all, h)
		}
	}
	return all
}

// LookupSSHConfigHost finds a host by config alias (case-insensitive).
func LookupSSHConfigHost(alias string) (domain.SSHConfigHost, bool) {
	alias = strings.TrimSpace(alias)
	if alias == "" {
		return domain.SSHConfigHost{}, false
	}
	want := strings.ToLower(alias)
	for _, h := range LoadDefaultSSHConfigHosts() {
		if strings.ToLower(h.Alias) == want {
			return h, true
		}
	}
	return domain.SSHConfigHost{}, false
}

// MatchSSHConfigHost finds the best config entry for a dial target in default configs.
// Prefers alias match, then HostName+User, then HostName only.
func MatchSSHConfigHost(user, host string, port int) (domain.SSHConfigHost, bool) {
	return matchSSHConfigHostIn(LoadDefaultSSHConfigHosts(), user, host, port)
}

func matchSSHConfigHostIn(hosts []domain.SSHConfigHost, user, host string, port int) (domain.SSHConfigHost, bool) {
	hostLower := strings.ToLower(strings.TrimSpace(host))
	userLower := strings.ToLower(strings.TrimSpace(user))
	if hostLower == "" {
		return domain.SSHConfigHost{}, false
	}

	// 1) Alias equals host string (user typed config alias as host)
	for _, h := range hosts {
		if strings.ToLower(h.Alias) == hostLower {
			return h, true
		}
	}
	// 2) HostName + User; 3) HostName only (respect port when both set)
	var hostNameOnly *domain.SSHConfigHost
	for i := range hosts {
		h := &hosts[i]
		if strings.ToLower(h.HostName) != hostLower {
			continue
		}
		if port > 0 && h.Port > 0 && port != h.Port {
			continue
		}
		if userLower != "" && strings.ToLower(h.User) == userLower {
			return *h, true
		}
		if hostNameOnly == nil {
			hostNameOnly = h
		}
	}
	if hostNameOnly != nil {
		return *hostNameOnly, true
	}
	return domain.SSHConfigHost{}, false
}

// SpecFromSSHConfigHost builds a dial Spec from a config host entry.
func SpecFromSSHConfigHost(h domain.SSHConfigHost) Spec {
	port := h.Port
	if port <= 0 {
		port = 22
	}
	return Spec{
		User:          h.User,
		Host:          h.HostName,
		Port:          port,
		IdentityFiles: append([]string(nil), h.IdentityFiles...),
		ConfigAlias:   h.Alias,
		ConfigFile:    strings.TrimSpace(h.ConfigPath),
	}
}

// EnrichSpec merges IdentityFiles (and ConfigAlias) from ~/.ssh/config when possible.
// Explicit IdentityFiles on spec are kept and prepended; config files are appended uniquely.
func EnrichSpec(spec Spec) Spec {
	if spec.Port <= 0 {
		spec.Port = 22
	}
	// Prefer explicit config alias re-resolve
	if spec.ConfigAlias != "" {
		if h, ok := LookupSSHConfigHost(spec.ConfigAlias); ok {
			return mergeSpecWithConfig(spec, h)
		}
	}
	if h, ok := MatchSSHConfigHost(spec.User, spec.Host, spec.Port); ok {
		return mergeSpecWithConfig(spec, h)
	}
	return spec
}

func mergeSpecWithConfig(spec Spec, h domain.SSHConfigHost) Spec {
	if spec.ConfigAlias == "" {
		spec.ConfigAlias = h.Alias
	}
	// Prefer config HostName when current Host looks like the alias
	if strings.EqualFold(spec.Host, h.Alias) && h.HostName != "" {
		spec.Host = h.HostName
	}
	if spec.User == "" && h.User != "" {
		spec.User = h.User
	}
	if (spec.Port <= 0 || spec.Port == 22) && h.Port > 0 {
		spec.Port = h.Port
	}
	spec.IdentityFiles = mergeIdentityFiles(spec.IdentityFiles, h.IdentityFiles)
	return spec
}

func mergeIdentityFiles(primary, extra []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, p := range primary {
		p = strings.TrimSpace(p)
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	for _, p := range extra {
		p = strings.TrimSpace(p)
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out
}

// DefaultSSHConfigPaths returns the standard OpenSSH client config file paths.
func DefaultSSHConfigPaths() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return []string{"/etc/ssh/ssh_config"}
	}
	return []string{
		filepath.Join(home, ".ssh", "config"),
		"/etc/ssh/ssh_config",
	}
}

// ParseSSHConfigFile reads an OpenSSH client config file and returns the host entries.
// Host * wildcard blocks are skipped. Missing fields fall back to sensible defaults
// (port 22, current OS username). IdentityFile paths prefixed with ~ are expanded.
// The path argument itself may also contain a leading ~.
func ParseSSHConfigFile(configPath string) ([]domain.SSHConfigHost, error) {
	home, _ := os.UserHomeDir()
	configPath = expandTilde(configPath, home)

	f, err := os.Open(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []domain.SSHConfigHost{}, nil
		}
		return nil, fmt.Errorf("open ssh config %s: %w", configPath, err)
	}
	defer func() { _ = f.Close() }()

	currentUser := os.Getenv("USER")
	if currentUser == "" {
		currentUser = os.Getenv("LOGNAME")
	}

	var hosts []domain.SSHConfigHost
	var cur *domain.SSHConfigHost

	commit := func() {
		if cur == nil {
			return
		}
		if cur.HostName == "" {
			cur.HostName = cur.Alias
		}
		if cur.Port == 0 {
			cur.Port = 22
		}
		if cur.User == "" {
			cur.User = currentUser
		}
		cur.ConfigPath = configPath
		hosts = append(hosts, *cur)
		cur = nil
	}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		keyword, value, ok := sshSplitKV(line)
		if !ok {
			continue
		}
		switch strings.ToLower(keyword) {
		case "host":
			commit()
			// Skip wildcard blocks (Host *, Host *.example.com, etc.)
			if strings.ContainsAny(value, "*?") {
				cur = nil
			} else {
				// Multiple aliases on one Host line — take the first.
				aliases := strings.Fields(value)
				cur = &domain.SSHConfigHost{Alias: aliases[0]}
			}
		case "hostname":
			if cur != nil {
				cur.HostName = value
			}
		case "user":
			if cur != nil {
				cur.User = value
			}
		case "port":
			if cur != nil {
				if p, err := strconv.Atoi(value); err == nil && p > 0 && p <= 65535 {
					cur.Port = p
				}
			}
		case "identityfile":
			if cur != nil {
				cur.IdentityFiles = append(cur.IdentityFiles, expandTilde(value, home))
			}
		}
	}
	commit()
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read ssh config: %w", err)
	}
	return hosts, nil
}

// sshSplitKV parses one SSH config line into keyword and value.
// Accepts both "Keyword Value" and "Keyword=Value" forms; strips inline comments.
func sshSplitKV(line string) (keyword, value string, ok bool) {
	// "keyword=value" form
	if idx := strings.Index(line, "="); idx > 0 {
		kw := strings.TrimSpace(line[:idx])
		val := strings.Trim(strings.TrimSpace(line[idx+1:]), `"'`)
		val = stripInlineComment(val)
		if kw != "" && val != "" {
			return kw, val, true
		}
	}
	// "keyword value" form
	fields := strings.Fields(line)
	if len(fields) >= 2 {
		val := strings.Trim(strings.Join(fields[1:], " "), `"'`)
		val = stripInlineComment(val)
		if val != "" {
			return fields[0], val, true
		}
	}
	return "", "", false
}

func stripInlineComment(s string) string {
	if idx := strings.Index(s, " #"); idx >= 0 {
		return strings.TrimSpace(s[:idx])
	}
	return s
}

func expandTilde(path, home string) string {
	if home == "" {
		return path
	}
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:])
	}
	return path
}
