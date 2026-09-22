package aiusage

import (
	"bufio"
	"encoding/json"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/erikharutyunyan/double-pane/internal/domain"
)

var ansiRe = regexp.MustCompile("\x1b\\[[0-9;]*m")

// parseClaudeUsage parses `claude -p "/usage"` stdout. Unknown lines are
// silently skipped, so a CLI text change degrades to an empty result rather
// than a crash.
func parseClaudeUsage(s string) ([]domain.AIUsageLimit, []domain.AIUsageDetail) {
	var limits []domain.AIUsageLimit
	var details []domain.AIUsageDetail
	inBlock := false

	for raw := range strings.SplitSeq(s, "\n") {
		line := strings.TrimSpace(ansiRe.ReplaceAllString(raw, ""))
		if line == "" {
			continue
		}

		switch {
		case strings.Contains(line, "% used"):
			inBlock = false
			label, percent, resetAt := parseLimitLine(line)
			limits = append(limits, domain.AIUsageLimit{Label: label, Percent: percent, ResetAt: resetAt})
		case strings.HasPrefix(line, "Last "):
			inBlock = true
			label, value := splitOn(line, "·")
			details = append(details, domain.AIUsageDetail{Label: label, Value: value, Depth: 0})
		case inBlock:
			label, value := splitOn(line, ":")
			details = append(details, domain.AIUsageDetail{Label: label, Value: value, Depth: 1})
		}
	}
	return limits, details
}

// parseLimitLine parses "Current session: 9% used · resets Aug 26 at 3:59am (Europe/Warsaw)".
func parseLimitLine(line string) (label string, percent int, resetAt string) {
	left, right, _ := strings.Cut(line, "·")
	left = strings.TrimSpace(left)

	if i := strings.Index(left, "%"); i > 0 {
		j := i
		for j > 0 && (left[j-1] == '%' || (left[j-1] >= '0' && left[j-1] <= '9')) {
			j--
		}
		if n, err := strconv.Atoi(strings.TrimSuffix(left[j:i], "")); err == nil {
			percent = n
		}
		if colon := strings.Index(left, ":"); colon > 0 && colon < j {
			label = strings.TrimSpace(left[:colon])
		} else {
			label = strings.TrimSpace(left[:j])
		}
	} else {
		label = left
	}

	right = strings.TrimSpace(right)
	if _, after, ok := strings.Cut(right, "resets "); ok {
		resetAt = strings.TrimSpace(after)
	}
	return label, percent, resetAt
}

func splitOn(s, sep string) (label, value string) {
	l, v, ok := strings.Cut(s, sep)
	if !ok {
		return strings.TrimSpace(s), ""
	}
	return strings.TrimSpace(l), strings.TrimSpace(v)
}

type grokUpdateLine struct {
	Params struct {
		Update struct {
			Usage struct {
				InputTokens  int64 `json:"inputTokens"`
				OutputTokens int64 `json:"outputTokens"`
				TotalTokens  int64 `json:"totalTokens"`
			} `json:"usage"`
		} `json:"update"`
	} `json:"params"`
}

// sumGrokTokens sums per-turn token usage out of a Grok Build
// sessions/**/updates.jsonl file. Malformed lines are skipped.
func sumGrokTokens(r io.Reader) (in, out, total int64, calls int) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64<<10), 4<<20)
	for sc.Scan() {
		var l grokUpdateLine
		if err := json.Unmarshal(sc.Bytes(), &l); err != nil {
			continue
		}
		u := l.Params.Update.Usage
		if u.TotalTokens == 0 {
			continue
		}
		in += u.InputTokens
		out += u.OutputTokens
		total += u.TotalTokens
		calls++
	}
	return in, out, total, calls
}
