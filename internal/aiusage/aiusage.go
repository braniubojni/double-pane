// Package aiusage reports quota/usage snapshots for locally installed AI
// coding-agent CLIs (Claude Code, Grok Build, Cursor).
package aiusage

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/erikharutyunyan/double-pane/internal/domain"
)

type provider struct {
	id, name string
	bins     []string                        // candidate binaries, first found wins
	collect  func(bin string) domain.AIUsage // nil => unsupported row
}

var providers = []provider{
	{"claude", "Claude Code", []string{"claude", "~/.local/bin/claude"}, collectClaude},
	{"grok", "Grok Build", []string{"grok", "~/.grok/bin/grok"}, collectGrok},
	{"cursor", "Cursor", nil, collectCursor},
}

// List collects a usage row per known provider, in parallel (the Claude CLI
// call alone takes a few seconds).
func List() []domain.AIUsage {
	out := make([]domain.AIUsage, len(providers))
	var wg sync.WaitGroup
	for i, p := range providers {
		wg.Add(1)
		go func(i int, p provider) {
			defer wg.Done()
			out[i] = rowFor(p)
		}(i, p)
	}
	wg.Wait()
	return out
}

func rowFor(p provider) domain.AIUsage {
	if p.collect == nil {
		return emptyRow(p.id, p.name, "unsupported")
	}
	var bin string
	if len(p.bins) > 0 {
		bin = lookBin(p.bins)
		if bin == "" {
			return emptyRow(p.id, p.name, "not-installed")
		}
	}
	row := p.collect(bin)
	// JS reads .limits/.details as plain arrays, never null.
	if row.Limits == nil {
		row.Limits = []domain.AIUsageLimit{}
	}
	if row.Details == nil {
		row.Details = []domain.AIUsageDetail{}
	}
	return row
}

func emptyRow(id, name, status string) domain.AIUsage {
	return domain.AIUsage{
		ID: id, Name: name, Status: status,
		Limits: []domain.AIUsageLimit{}, Details: []domain.AIUsageDetail{},
	}
}

// lookBin resolves the first available candidate binary. A Wails .app
// launched from Finder does not inherit the user's shell PATH, so plain
// exec.LookPath misses CLIs installed under e.g. ~/.local/bin — absolute
// fallback candidates cover that.
//
// ponytail: static candidate list, not a PATH probe. Upgrade path if a real
// install location is missed: cache one `$SHELL -lc "command -v <bin>"` per
// provider via sync.Once.
func lookBin(cands []string) string {
	home, _ := os.UserHomeDir()
	for _, c := range cands {
		if strings.HasPrefix(c, "~/") {
			if home == "" {
				continue
			}
			p := filepath.Join(home, c[2:])
			if st, err := os.Stat(p); err == nil && !st.IsDir() {
				return p
			}
			continue
		}
		if p, err := exec.LookPath(c); err == nil {
			return p
		}
	}
	return ""
}

func collectClaude(bin string) domain.AIUsage {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, "--safe-mode", "-p", "/usage")
	cmd.Dir = os.TempDir()
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	// Exit errors are ignored: parse whatever the CLI printed regardless of
	// its exit code, same tolerance as internal/ports/list_unix.go.
	if err := cmd.Run(); err != nil && out.Len() == 0 {
		row := emptyRow("claude", "Claude Code", "error")
		row.Error = err.Error()
		return row
	}

	limits, details := parseClaudeUsage(out.String())
	if len(limits) == 0 && len(details) == 0 {
		row := emptyRow("claude", "Claude Code", "error")
		row.Error = "unrecognized /usage output"
		return row
	}
	return domain.AIUsage{ID: "claude", Name: "Claude Code", Status: "ok", Limits: limits, Details: details}
}

func collectGrok(bin string) domain.AIUsage {
	_ = bin // presence already confirmed by lookBin; data comes from local session logs, not a CLI call
	home, err := os.UserHomeDir()
	if err != nil {
		row := emptyRow("grok", "Grok Build", "error")
		row.Error = err.Error()
		return row
	}

	in24, out24, calls24 := sumGrokWindow(filepath.Join(home, ".grok", "sessions"), 24*time.Hour)
	in7d, out7d, calls7d := sumGrokWindow(filepath.Join(home, ".grok", "sessions"), 7*24*time.Hour)

	return domain.AIUsage{
		ID: "grok", Name: "Grok Build", Status: "ok", Estimate: true,
		Details: []domain.AIUsageDetail{
			{Label: "Last 24h", Value: tokenSummary(in24, out24, calls24), Depth: 0},
			{Label: "Last 7d", Value: tokenSummary(in7d, out7d, calls7d), Depth: 0},
		},
	}
}

// sumGrokWindow sums token usage across every updates.jsonl under sessions
// modified within window.
func sumGrokWindow(sessionsDir string, window time.Duration) (in, out int64, calls int) {
	cutoff := time.Now().Add(-window)
	_ = filepath.WalkDir(sessionsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() != "updates.jsonl" {
			return nil
		}
		info, err := d.Info()
		if err != nil || info.ModTime().Before(cutoff) {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer func() { _ = f.Close() }()
		i, o, _, c := sumGrokTokens(f)
		in += i
		out += o
		calls += c
		return nil
	})
	return in, out, calls
}

func tokenSummary(in, out int64, calls int) string {
	return fmt.Sprintf("%d requests · %d input / %d output tokens", calls, in, out)
}
