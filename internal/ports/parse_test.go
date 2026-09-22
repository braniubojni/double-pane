package ports

import (
	"os"
	"strings"
	"testing"

	"github.com/erikharutyunyan/double-pane/internal/domain"
)

func TestParseLsofF(t *testing.T) {
	in := `p1107
cControlCenter
PTCP
n*:5000
p20259
credis-server
PTCP
n127.0.0.1:6379
p20259
credis-server
PTCP
n[::1]:6379
p62285
collama
PTCP
n*:11434
`
	got := parseLsofF(in)
	if len(got) != 3 {
		t.Fatalf("len=%d %+v", len(got), got)
	}
	if got[0].Port != 5000 || got[0].Process != "ControlCenter" || got[0].PID != 1107 {
		t.Fatalf("first: %+v", got[0])
	}
	if got[1].Port != 6379 || got[1].PID != 20259 {
		t.Fatalf("redis should dedup ipv4/ipv6: %+v", got)
	}
	if got[2].Port != 11434 || got[2].Process != "ollama" {
		t.Fatalf("ollama: %+v", got[2])
	}
}

func TestParseAddrPort(t *testing.T) {
	cases := map[string]int{
		"*:5000":          5000,
		"127.0.0.1:6379":  6379,
		"[::1]:11434":     11434,
		"[::]:80":         80,
		"*:7000 (LISTEN)": 7000,
		"10.0.0.1:443->x": 443,
		"not-a-port":      0,
		"":                0,
	}
	for in, want := range cases {
		if got := parseAddrPort(in); got != want {
			t.Errorf("parseAddrPort(%q)=%d want %d", in, got, want)
		}
	}
}

func TestParseNetstatANO(t *testing.T) {
	in := `
  Proto  Local Address          Foreign Address        State           PID
  TCP    0.0.0.0:135            0.0.0.0:0              LISTENING       1232
  TCP    [::]:135               [::]:0                 LISTENING       1232
  TCP    127.0.0.1:6379         0.0.0.0:0              LISTENING       20259
  TCP    127.0.0.1:6379         127.0.0.1:51234        ESTABLISHED     20259
`
	got := parseNetstatANO(in)
	if len(got) != 2 {
		t.Fatalf("len=%d %+v", len(got), got)
	}
	if got[0].Port != 135 || got[0].PID != 1232 {
		t.Fatalf("rpc: %+v", got[0])
	}
	if got[1].Port != 6379 || got[1].PID != 20259 {
		t.Fatalf("redis: %+v", got[1])
	}
}

func TestParseTasklistCSV(t *testing.T) {
	in := `"svchost.exe","1232","Services","0","12,345 K"
"redis-server.exe","20259","Console","1","8,192 K"
`
	got := parseTasklistCSV(in)
	if got[1232] != "svchost" {
		t.Fatalf("svchost: %q", got[1232])
	}
	if got[20259] != "redis-server" {
		t.Fatalf("redis: %q", got[20259])
	}
}

func TestParsePsAXO(t *testing.T) {
	in := `
  1  0 /sbin/launchd launchd
  501  501 postgres postgres -D /data
  502  501 redis-server redis-server *:6379
  503  502 other other
`
	got := parsePsAXO(in, 501)
	if len(got) != 2 {
		t.Fatalf("len=%d %+v", len(got), got)
	}
	if got[0].Name != "postgres" || got[0].PID != 501 {
		t.Fatalf("postgres first: %+v", got[0])
	}
	if got[1].Name != "redis-server" || !strings.Contains(got[1].Cmd, "redis-server") {
		t.Fatalf("redis: %+v", got[1])
	}
}

func TestParseLsofCwd(t *testing.T) {
	in := `p501
fcwd
n/Users/me/app
p502
fcwd
n/tmp/redis
`
	got := parseLsofCwd(in)
	if got[501] != "/Users/me/app" || got[502] != "/tmp/redis" {
		t.Fatalf("%+v", got)
	}
	list := []domain.ProcessInfo{{PID: 501, Name: "node"}, {PID: 9, Name: "x"}}
	applyCwd(list, got)
	if list[0].Cwd != "/Users/me/app" || list[1].Cwd != "" {
		t.Fatalf("%+v", list)
	}
}

func TestProcessesFromTasklist(t *testing.T) {
	in := `"svchost.exe","1232","Services","0","12,345 K"
"redis-server.exe","20259","Console","1","8,192 K"
`
	got := processesFromTasklist(in)
	if len(got) != 2 {
		t.Fatalf("len=%d %+v", len(got), got)
	}
	if got[0].Name != "redis-server" || got[0].PID != 20259 {
		t.Fatalf("redis first by name: %+v", got)
	}
}

func TestValidatePID(t *testing.T) {
	if err := ValidatePID(0); err == nil {
		t.Fatal("pid 0")
	}
	if err := ValidatePID(1); err == nil {
		t.Fatal("pid 1")
	}
	if err := ValidatePID(os.Getpid()); err == nil {
		t.Fatal("self")
	}
	if err := ValidatePID(os.Getpid() + 1_000_000); err != nil {
		t.Fatalf("high pid: %v", err)
	}
}

func TestKillAllEmpty(t *testing.T) {
	if err := KillAll(nil); err != nil {
		t.Fatal(err)
	}
}
