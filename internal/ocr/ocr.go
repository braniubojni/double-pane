package ocr

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
	"unicode"
)

const timeout = 30 * time.Second

// Binary is the tesseract executable name or path (overridable in tests).
var Binary = "tesseract"

// Available reports whether tesseract is on PATH.
func Available() bool {
	_, err := exec.LookPath(Binary)
	return err == nil
}

// Recognize runs `tesseract <file> stdout -l eng` with a 30s timeout.
func Recognize(ctx context.Context, path string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, Binary, path, "stdout", "-l", "eng")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return "", fmt.Errorf("tesseract timeout")
		}
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("tesseract: %s", msg)
	}
	return stdout.String(), nil
}

// Tokens lowercases text, collapses whitespace, and drops tokens shorter than 3.
func Tokens(text string) []string {
	fields := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return unicode.IsSpace(r) || (!unicode.IsLetter(r) && !unicode.IsNumber(r))
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if len(f) >= 3 {
			out = append(out, f)
		}
	}
	return out
}

// Snippet returns the first 80 runes of collapsed whitespace text.
func Snippet(text string) string {
	s := strings.Join(strings.Fields(text), " ")
	r := []rune(s)
	if len(r) > 80 {
		return string(r[:80])
	}
	return s
}

// Similar is true when Jaccard(token sets) >= thr, or one contains the other
// when both have at least 20 tokens.
func Similar(a, b []string, thr float64) bool {
	if len(a) == 0 || len(b) == 0 {
		return false
	}
	sa, sb := setOf(a), setOf(b)
	if jaccard(sa, sb) >= thr {
		return true
	}
	if len(sa) >= 20 && len(sb) >= 20 && (containsAll(sa, sb) || containsAll(sb, sa)) {
		return true
	}
	return false
}

func setOf(toks []string) map[string]struct{} {
	m := make(map[string]struct{}, len(toks))
	for _, t := range toks {
		m[t] = struct{}{}
	}
	return m
}

func jaccard(a, b map[string]struct{}) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 0
	}
	inter := 0
	for k := range a {
		if _, ok := b[k]; ok {
			inter++
		}
	}
	union := len(a) + len(b) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

func containsAll(outer, inner map[string]struct{}) bool {
	for k := range inner {
		if _, ok := outer[k]; !ok {
			return false
		}
	}
	return true
}
