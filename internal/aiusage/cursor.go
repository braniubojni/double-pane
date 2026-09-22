package aiusage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/erikharutyunyan/double-pane/internal/domain"
	_ "modernc.org/sqlite"
)

// Undocumented dashboard endpoint (same one the Cursor website calls).
// Token is read from disk per refresh, sent only here, never logged or stored.
const defaultCursorSummaryURL = "https://cursor.com/api/usage-summary"

const cursorBrowserUA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

const cursorSummaryMaxBody = 1 << 20

var cursorHTTPClient = &http.Client{Timeout: 15 * time.Second}

func collectCursor(_ string) domain.AIUsage {
	return fetchCursorUsage(defaultCursorSummaryURL, cursorStateDBPath(), cursorAgentAuthPaths())
}

func fetchCursorUsage(summaryURL, dbPath string, authPaths []string) domain.AIUsage {
	token, found, err := resolveCursorToken(dbPath, authPaths)
	if err != nil {
		row := emptyRow("cursor", "Cursor", "error")
		row.Error = sanitizeCursorErr(err)
		return row
	}
	if token == "" {
		if !found {
			return emptyRow("cursor", "Cursor", "not-installed")
		}
		row := emptyRow("cursor", "Cursor", "error")
		row.Error = "not signed in"
		return row
	}

	sub, err := jwtSub(token)
	if err != nil {
		row := emptyRow("cursor", "Cursor", "error")
		row.Error = "not signed in"
		return row
	}

	body, err := getCursorUsageSummary(summaryURL, sub, token)
	if err != nil {
		row := emptyRow("cursor", "Cursor", "error")
		row.Error = sanitizeCursorErr(err)
		return row
	}

	limits, details, ok := parseCursorUsage(body)
	if !ok {
		row := emptyRow("cursor", "Cursor", "error")
		row.Error = "unrecognized usage-summary output"
		return row
	}
	return domain.AIUsage{
		ID: "cursor", Name: "Cursor", Status: "ok",
		Limits: limits, Details: details,
	}
}

func cursorStateDBPath() string {
	cfg, err := os.UserConfigDir()
	if err != nil || cfg == "" {
		return ""
	}
	return filepath.Join(cfg, "Cursor", "User", "globalStorage", "state.vscdb")
}

func cursorAgentAuthPaths() []string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return nil
	}
	return []string{
		filepath.Join(home, ".cursor", "agent", "auth.json"),
		filepath.Join(home, ".cursor", "auth.json"),
	}
}

func resolveCursorToken(dbPath string, authPaths []string) (token string, found bool, err error) {
	if dbPath != "" {
		if st, statErr := os.Stat(dbPath); statErr == nil && !st.IsDir() {
			found = true
			tok, readErr := readTokenFromVSCDB(dbPath)
			if readErr != nil {
				return "", true, readErr
			}
			if tok != "" {
				return tok, true, nil
			}
		}
	}
	for _, p := range authPaths {
		if p == "" {
			continue
		}
		st, statErr := os.Stat(p)
		if statErr != nil || st.IsDir() {
			continue
		}
		found = true
		tok, readErr := readTokenFromAuthJSON(p)
		if readErr != nil {
			return "", true, readErr
		}
		if tok != "" {
			return tok, true, nil
		}
	}
	return "", found, nil
}

func readTokenFromVSCDB(path string) (string, error) {
	dsn := path + "?mode=ro&_pragma=busy_timeout(2000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return "", err
	}
	defer func() { _ = db.Close() }()

	var value string
	err = db.QueryRow(`SELECT value FROM ItemTable WHERE key = 'cursorAuth/accessToken'`).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return normalizeCursorToken(value), nil
}

func readTokenFromAuthJSON(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var doc struct {
		AccessToken string `json:"accessToken"`
		AccessSnake string `json:"access_token"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		return "", err
	}
	if doc.AccessToken != "" {
		return normalizeCursorToken(doc.AccessToken), nil
	}
	return normalizeCursorToken(doc.AccessSnake), nil
}

func normalizeCursorToken(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, `"`)
	return s
}

func getCursorUsageSummary(summaryURL, sub, token string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, summaryURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Cookie", fmt.Sprintf("WorkosCursorSessionToken=%s%%3A%%3A%s", sub, token))
	req.Header.Set("Origin", "https://cursor.com")
	req.Header.Set("User-Agent", cursorBrowserUA)

	resp, err := cursorHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("usage-summary HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, cursorSummaryMaxBody))
	if err != nil {
		return nil, err
	}
	return body, nil
}

func sanitizeCursorErr(err error) string {
	if err == nil {
		return ""
	}
	s := err.Error()
	if i := strings.Index(s, "eyJ"); i >= 0 {
		return strings.TrimSpace(s[:i]) + "redacted"
	}
	return s
}
