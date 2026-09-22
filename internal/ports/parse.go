package ports

import (
	"encoding/csv"
	"sort"
	"strconv"
	"strings"

	"github.com/erikharutyunyan/double-pane/internal/domain"
)

// parseLsofF parses `lsof -nP -iTCP -sTCP:LISTEN -F cPn` output.
func parseLsofF(s string) []domain.PortListener {
	var pid int
	var cmd, proto string
	var out []domain.PortListener
	for _, line := range strings.Split(s, "\n") {
		if line == "" {
			continue
		}
		tag, val := line[0], line[1:]
		switch tag {
		case 'p':
			pid, _ = strconv.Atoi(val)
			cmd, proto = "", ""
		case 'c':
			cmd = val
		case 'P':
			proto = strings.ToLower(val)
		case 'n':
			if proto != "" && proto != "tcp" {
				continue
			}
			port := parseAddrPort(val)
			if pid > 0 && port > 0 {
				out = append(out, domain.PortListener{
					Port: port, PID: pid, Process: cmd, Proto: "tcp",
				})
			}
		}
	}
	return Dedup(out)
}

func parseAddrPort(name string) int {
	name = strings.TrimSpace(name)
	if i := strings.Index(name, "->"); i >= 0 {
		name = name[:i]
	}
	if i := strings.IndexAny(name, " \t"); i >= 0 {
		name = name[:i]
	}
	name = strings.TrimSpace(name)
	i := strings.LastIndex(name, ":")
	if i < 0 || i+1 >= len(name) {
		return 0
	}
	p, err := strconv.Atoi(name[i+1:])
	if err != nil || p <= 0 || p > 65535 {
		return 0
	}
	return p
}

// parseNetstatANO parses `netstat -ano -p tcp` (Windows) LISTENING rows.
func parseNetstatANO(s string) []domain.PortListener {
	var out []domain.PortListener
	for _, line := range strings.Split(s, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		if !strings.EqualFold(fields[0], "TCP") {
			continue
		}
		state := strings.ToUpper(fields[len(fields)-2])
		if state != "LISTENING" {
			continue
		}
		pid, err := strconv.Atoi(fields[len(fields)-1])
		if err != nil || pid <= 0 {
			continue
		}
		port := parseAddrPort(fields[1])
		if port <= 0 {
			continue
		}
		out = append(out, domain.PortListener{
			Port: port, PID: pid, Proto: "tcp",
		})
	}
	return Dedup(out)
}

func parseTasklistCSV(s string) map[int]string {
	out := make(map[int]string)
	r := csv.NewReader(strings.NewReader(s))
	rows, err := r.ReadAll()
	if err != nil {
		return out
	}
	for _, row := range rows {
		if len(row) < 2 {
			continue
		}
		pid, err := strconv.Atoi(strings.TrimSpace(row[1]))
		if err != nil || pid <= 0 {
			continue
		}
		name := strings.TrimSpace(row[0])
		name = strings.TrimSuffix(name, ".exe")
		name = strings.TrimSuffix(name, ".EXE")
		out[pid] = name
	}
	return out
}

const maxCmd = 240

// parsePsAXO parses `ps -axo pid=,uid=,comm=,args=` and keeps rows for uid.
func parsePsAXO(s string, uid int) []domain.ProcessInfo {
	var out []domain.ProcessInfo
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil || pid <= 0 {
			continue
		}
		u, err := strconv.Atoi(fields[1])
		if err != nil || u != uid {
			continue
		}
		name := fields[2]
		cmd := strings.Join(fields[2:], " ")
		if len(cmd) > maxCmd {
			cmd = cmd[:maxCmd]
		}
		out = append(out, domain.ProcessInfo{PID: pid, Name: name, Cmd: cmd})
	}
	sortProcesses(out)
	return out
}

func parseLsofCwd(s string) map[int]string {
	out := make(map[int]string)
	var pid int
	for _, line := range strings.Split(s, "\n") {
		if line == "" {
			continue
		}
		switch line[0] {
		case 'p':
			pid, _ = strconv.Atoi(line[1:])
		case 'n':
			if pid > 0 && line[1:] != "" {
				out[pid] = line[1:]
			}
		}
	}
	return out
}

func applyCwd(list []domain.ProcessInfo, cwds map[int]string) {
	for i := range list {
		if d, ok := cwds[list[i].PID]; ok {
			list[i].Cwd = d
		}
	}
}

func processesFromTasklist(s string) []domain.ProcessInfo {
	names := parseTasklistCSV(s)
	out := make([]domain.ProcessInfo, 0, len(names))
	for pid, name := range names {
		out = append(out, domain.ProcessInfo{PID: pid, Name: name, Cmd: name})
	}
	sortProcesses(out)
	return out
}

func sortProcesses(out []domain.ProcessInfo) {
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
		}
		return out[i].PID < out[j].PID
	})
}

// Dedup keeps one row per port+PID, sorted by port then PID.
func Dedup(in []domain.PortListener) []domain.PortListener {
	type key struct{ port, pid int }
	seen := make(map[key]struct{}, len(in))
	out := make([]domain.PortListener, 0, len(in))
	for _, l := range in {
		k := key{l.Port, l.PID}
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		if l.Proto == "" {
			l.Proto = "tcp"
		}
		out = append(out, l)
	}
	sortListeners(out)
	return out
}
