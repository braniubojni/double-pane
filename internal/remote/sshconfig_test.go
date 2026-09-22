package remote

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/erikharutyunyan/double-pane/internal/domain"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "ssh_config_*")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	return f.Name()
}

func TestParseSSHConfigFile_basic(t *testing.T) {
	cfg := writeConfig(t, `
Host pahestain
    HostName 100.97.100.94
    User mfmso
    PreferredAuthentications publickey
    IdentitiesOnly yes

Host myserver
    HostName example.com
    User deploy
    Port 2222
    IdentityFile ~/.ssh/id_deploy
`)
	hosts, err := ParseSSHConfigFile(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hosts) != 2 {
		t.Fatalf("expected 2 hosts, got %d: %+v", len(hosts), hosts)
	}

	h0 := hosts[0]
	if h0.Alias != "pahestain" {
		t.Errorf("alias: want pahestain, got %s", h0.Alias)
	}
	if h0.HostName != "100.97.100.94" {
		t.Errorf("hostname: want 100.97.100.94, got %s", h0.HostName)
	}
	if h0.User != "mfmso" {
		t.Errorf("user: want mfmso, got %s", h0.User)
	}
	if h0.Port != 22 {
		t.Errorf("port: want 22, got %d", h0.Port)
	}

	h1 := hosts[1]
	if h1.Alias != "myserver" {
		t.Errorf("alias: want myserver, got %s", h1.Alias)
	}
	if h1.Port != 2222 {
		t.Errorf("port: want 2222, got %d", h1.Port)
	}
	if len(h1.IdentityFiles) == 0 {
		t.Error("expected IdentityFiles to be set")
	}
}

func TestParseSSHConfigFile_skipsWildcard(t *testing.T) {
	cfg := writeConfig(t, `
Host *
    ServerAliveInterval 60

Host realhost
    HostName 1.2.3.4
    User admin
`)
	hosts, err := ParseSSHConfigFile(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hosts) != 1 {
		t.Fatalf("expected 1 host (wildcard skipped), got %d", len(hosts))
	}
	if hosts[0].Alias != "realhost" {
		t.Errorf("want realhost, got %s", hosts[0].Alias)
	}
}

func TestParseSSHConfigFile_defaults(t *testing.T) {
	cfg := writeConfig(t, `
Host minimal
    HostName 10.0.0.1
`)
	hosts, err := ParseSSHConfigFile(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hosts) != 1 {
		t.Fatalf("expected 1 host, got %d", len(hosts))
	}
	h := hosts[0]
	if h.Port != 22 {
		t.Errorf("default port: want 22, got %d", h.Port)
	}
	// HostName falls back to alias when absent
	cfg2 := writeConfig(t, `
Host aliasonly
    User someone
`)
	hosts2, _ := ParseSSHConfigFile(cfg2)
	if len(hosts2) != 1 {
		t.Fatalf("expected 1 host, got %d", len(hosts2))
	}
	if hosts2[0].HostName != "aliasonly" {
		t.Errorf("hostname fallback: want aliasonly, got %s", hosts2[0].HostName)
	}
}

func TestParseSSHConfigFile_missingFile(t *testing.T) {
	hosts, err := ParseSSHConfigFile(filepath.Join(t.TempDir(), "nonexistent"))
	if err != nil {
		t.Fatalf("missing file should return empty, not error: %v", err)
	}
	if len(hosts) != 0 {
		t.Errorf("expected empty slice, got %v", hosts)
	}
}

func TestParseSSHConfigFile_tildePath(t *testing.T) {
	// Verify ~ in the path argument is expanded (not treated as a literal dir).
	// We can't predict the home dir, so we just check that a tilde path to a
	// non-existent file doesn't panic or return an error (returns empty).
	hosts, err := ParseSSHConfigFile("~/this_config_does_not_exist_abc123")
	if err != nil {
		t.Fatalf("unexpected error for non-existent tilde path: %v", err)
	}
	if len(hosts) != 0 {
		t.Errorf("expected empty, got %v", hosts)
	}
}

func TestParseSSHConfigFile_equalSignSyntax(t *testing.T) {
	cfg := writeConfig(t, `
Host equalhost
    HostName=192.168.1.5
    User=admin
    Port=2200
`)
	hosts, err := ParseSSHConfigFile(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hosts) != 1 {
		t.Fatalf("expected 1 host, got %d", len(hosts))
	}
	h := hosts[0]
	if h.HostName != "192.168.1.5" {
		t.Errorf("hostname: want 192.168.1.5, got %s", h.HostName)
	}
	if h.Port != 2200 {
		t.Errorf("port: want 2200, got %d", h.Port)
	}
}

func TestParseSSHConfigFile_inlineComment(t *testing.T) {
	cfg := writeConfig(t, `
Host commenttest
    HostName 10.0.0.2 # this is a comment
    User bob
`)
	hosts, err := ParseSSHConfigFile(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hosts) != 1 {
		t.Fatalf("expected 1 host, got %d", len(hosts))
	}
	if hosts[0].HostName != "10.0.0.2" {
		t.Errorf("inline comment not stripped: got %q", hosts[0].HostName)
	}
}

func TestDefaultSSHConfigPaths_notEmpty(t *testing.T) {
	paths := DefaultSSHConfigPaths()
	if len(paths) == 0 {
		t.Error("expected at least one default config path")
	}
	for _, p := range paths {
		if p == "" {
			t.Error("empty path in DefaultSSHConfigPaths")
		}
	}
}

func TestMatchSSHConfigHostIn(t *testing.T) {
	hosts := []domain.SSHConfigHost{
		{Alias: "pahestain", HostName: "100.97.100.94", User: "mfmso", Port: 22, IdentityFiles: []string{"/k/id"}},
		{Alias: "other", HostName: "example.com", User: "root", Port: 22},
	}
	h, ok := matchSSHConfigHostIn(hosts, "", "pahestain", 22)
	if !ok || h.HostName != "100.97.100.94" {
		t.Fatalf("alias match: %+v ok=%v", h, ok)
	}
	h, ok = matchSSHConfigHostIn(hosts, "mfmso", "100.97.100.94", 22)
	if !ok || h.Alias != "pahestain" || len(h.IdentityFiles) != 1 {
		t.Fatalf("hostname+user match: %+v ok=%v", h, ok)
	}
	h, ok = matchSSHConfigHostIn(hosts, "nobody", "example.com", 22)
	if !ok || h.Alias != "other" {
		t.Fatalf("hostname-only match: %+v ok=%v", h, ok)
	}
}

func TestSpecFromSSHConfigHost(t *testing.T) {
	s := SpecFromSSHConfigHost(domain.SSHConfigHost{
		Alias: "pahestain", HostName: "100.97.100.94", User: "mfmso", Port: 22,
		IdentityFiles: []string{"/tmp/key"},
		ConfigPath:    "/home/u/.ssh/config",
	})
	if s.User != "mfmso" || s.Host != "100.97.100.94" || s.ConfigAlias != "pahestain" {
		t.Fatalf("got %+v", s)
	}
	if len(s.IdentityFiles) != 1 || s.IdentityFiles[0] != "/tmp/key" {
		t.Fatalf("identity: %+v", s.IdentityFiles)
	}
	if s.ConfigFile != "/home/u/.ssh/config" {
		t.Fatalf("config file: %q", s.ConfigFile)
	}
}

func TestMergeIdentityFiles(t *testing.T) {
	got := mergeIdentityFiles([]string{"/a", "/b"}, []string{"/b", "/c"})
	if len(got) != 3 || got[0] != "/a" || got[2] != "/c" {
		t.Fatalf("got %v", got)
	}
}
