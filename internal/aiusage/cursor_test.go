package aiusage

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func testJWT(sub string) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	payload, _ := json.Marshal(map[string]string{"sub": sub})
	return header + "." + base64.RawURLEncoding.EncodeToString(payload) + ".sig"
}

func TestJWTSub(t *testing.T) {
	got, err := jwtSub(testJWT("user_123"))
	if err != nil || got != "user_123" {
		t.Fatalf("jwtSub = %q %v", got, err)
	}

	cases := []string{"", "no-dots", "a.%%%notb64%%%.c", "eyJhbGciOiJub25lIn0.e30.sig"}
	for _, in := range cases {
		if _, err := jwtSub(in); err == nil {
			t.Errorf("jwtSub(%q) expected error", in)
		}
	}
}

func TestParseCursorUsagePersonal(t *testing.T) {
	in := `{
		"billingCycleEnd": "2026-08-04T00:35:51.000Z",
		"membershipType": "ultra",
		"isUnlimited": false,
		"individualUsage": {
			"plan": { "autoPercentUsed": 98.109, "apiPercentUsed": 100, "totalPercentUsed": 98.5128 },
			"onDemand": { "enabled": false }
		}
	}`
	limits, details, ok := parseCursorUsage([]byte(in))
	if !ok {
		t.Fatal("expected ok")
	}
	if len(limits) != 3 {
		t.Fatalf("limits=%+v", limits)
	}
	if limits[0].Label != "Included" || limits[0].Percent != 99 {
		t.Fatalf("included=%+v", limits[0])
	}
	if limits[1].Label != "Cursor Models" || limits[1].Percent != 98 {
		t.Fatalf("auto=%+v", limits[1])
	}
	if limits[2].Label != "Other Models" || limits[2].Percent != 100 {
		t.Fatalf("api=%+v", limits[2])
	}
	wantReset := formatResetAt("2026-08-04T00:35:51.000Z")
	if limits[0].ResetAt != wantReset || wantReset == "" {
		t.Fatalf("resetAt=%q want %q", limits[0].ResetAt, wantReset)
	}
	if len(details) != 1 || details[0].Label != "Plan" || details[0].Value != "Ultra" {
		t.Fatalf("details=%+v", details)
	}
}

func TestParseCursorUsageUnlimited(t *testing.T) {
	in := `{"billingCycleEnd":"2026-08-04T00:35:51.000Z","membershipType":"ultra","isUnlimited":true}`
	limits, details, ok := parseCursorUsage([]byte(in))
	if !ok {
		t.Fatal("expected ok")
	}
	if len(limits) != 0 {
		t.Fatalf("limits=%+v", limits)
	}
	var haveUnlimited, havePlan bool
	for _, d := range details {
		if d.Label == "Unlimited" {
			haveUnlimited = true
		}
		if d.Label == "Plan" && d.Value == "Ultra" {
			havePlan = true
		}
	}
	if !haveUnlimited || !havePlan {
		t.Fatalf("details=%+v", details)
	}
}

func TestParseCursorUsageOver100(t *testing.T) {
	in := `{
		"billingCycleEnd": "2026-08-04T00:00:00Z",
		"membershipType": "pro",
		"individualUsage": {
			"plan": { "autoPercentUsed": 142.7, "apiPercentUsed": 5, "totalPercentUsed": 80 }
		}
	}`
	limits, _, ok := parseCursorUsage([]byte(in))
	if !ok || limits[1].Percent != 143 || limits[0].Percent != 80 {
		t.Fatalf("limits=%+v ok=%v", limits, ok)
	}
}

func TestParseCursorUsageOnDemand(t *testing.T) {
	in := `{
		"billingCycleEnd": "2026-08-04T00:35:51.000Z",
		"membershipType": "ultra",
		"individualUsage": {
			"plan": { "autoPercentUsed": 10, "apiPercentUsed": 20, "totalPercentUsed": 15 },
			"onDemand": { "enabled": true, "used": 1785, "limit": 35000 }
		}
	}`
	_, details, ok := parseCursorUsage([]byte(in))
	if !ok {
		t.Fatal("expected ok")
	}
	var have bool
	for _, d := range details {
		if d.Label == "On-demand" && d.Value == "$17.85 / $350.00" {
			have = true
		}
	}
	if !have {
		t.Fatalf("details=%+v", details)
	}
}

func TestParseCursorUsageTeamMessages(t *testing.T) {
	in := `{
		"billingCycleEnd": "2026-08-04T00:35:51.000Z",
		"membershipType": "team",
		"autoModelSelectedDisplayMessage": "You've used 42% of your included total usage",
		"namedModelSelectedDisplayMessage": "You've used 100% of your included API usage",
		"teamUsage": { "onDemand": { "enabled": true } }
	}`
	limits, details, ok := parseCursorUsage([]byte(in))
	if !ok {
		t.Fatal("expected ok")
	}
	if limits[0].Percent != 100 || limits[1].Percent != 42 || limits[2].Percent != 100 {
		t.Fatalf("limits=%+v", limits)
	}
	var havePlan, haveOD bool
	for _, d := range details {
		if d.Label == "Plan" && d.Value == "Team (team)" {
			havePlan = true
		}
		if d.Label == "On-demand" && d.Value == "on" {
			haveOD = true
		}
	}
	if !havePlan || !haveOD {
		t.Fatalf("details=%+v", details)
	}
}

func TestParseCursorUsageUnrecognized(t *testing.T) {
	cases := []string{
		`not json`,
		`{}`,
		`{"membershipType":"pro","individualUsage":{"plan":{"autoPercentUsed":1,"apiPercentUsed":2}}}`,
		`{"autoModelSelectedDisplayMessage":"You've used 42% of your included total usage"}`,
		`{"individualUsage":{"plan":{"autoPercentUsed":1,"apiPercentUsed":2,"totalPercentUsed":null}}}`,
	}
	for _, in := range cases {
		if _, _, ok := parseCursorUsage([]byte(in)); ok {
			t.Errorf("parseCursorUsage(%s) unexpectedly ok", in)
		}
	}
}

func TestParsePercentFromMessage(t *testing.T) {
	v, ok := parsePercentFromMessage("You've used 98.1% of your included total usage")
	if !ok || v != 98.1 {
		t.Fatalf("got %v %v", v, ok)
	}
	if _, ok := parsePercentFromMessage("no percent here"); ok {
		t.Fatal("expected miss")
	}
}

func seedStateDB(t *testing.T, token string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "state.vscdb")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if _, err := db.Exec(`CREATE TABLE ItemTable (key TEXT PRIMARY KEY, value BLOB)`); err != nil {
		t.Fatal(err)
	}
	if token != "" {
		if _, err := db.Exec(`INSERT INTO ItemTable (key, value) VALUES (?, ?)`, "cursorAuth/accessToken", token); err != nil {
			t.Fatal(err)
		}
	} else {
		if _, err := db.Exec(`INSERT INTO ItemTable (key, value) VALUES (?, ?)`, "unrelated", "x"); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

func TestReadTokenFromVSCDB(t *testing.T) {
	tok := testJWT("user_abc")
	path := seedStateDB(t, tok)
	got, err := readTokenFromVSCDB(path)
	if err != nil || got != tok {
		t.Fatalf("got %q %v", got, err)
	}

	empty := seedStateDB(t, "")
	got, err = readTokenFromVSCDB(empty)
	if err != nil || got != "" {
		t.Fatalf("missing key: %q %v", got, err)
	}
}

func TestFetchCursorUsageHTTP(t *testing.T) {
	tok := testJWT("user_123")
	dbPath := seedStateDB(t, tok)

	var sawCookie, sawOrigin bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		want := "WorkosCursorSessionToken=user_123%3A%3A" + tok
		if r.Header.Get("Cookie") == want {
			sawCookie = true
		}
		if r.Header.Get("Origin") == "https://cursor.com" {
			sawOrigin = true
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"billingCycleEnd": "2026-08-04T00:35:51.000Z",
			"membershipType": "pro",
			"individualUsage": {
				"plan": { "autoPercentUsed": 10.4, "apiPercentUsed": 20.4, "totalPercentUsed": 12.4 }
			}
		}`))
	}))
	defer srv.Close()

	row := fetchCursorUsage(srv.URL, dbPath, nil)
	if row.Status != "ok" {
		t.Fatalf("row=%+v", row)
	}
	if !sawCookie || !sawOrigin {
		t.Fatalf("headers cookie=%v origin=%v", sawCookie, sawOrigin)
	}
	if len(row.Limits) != 3 || row.Limits[0].Percent != 12 || row.Limits[1].Percent != 10 || row.Limits[2].Percent != 20 {
		t.Fatalf("limits=%+v", row.Limits)
	}
}

func TestFetchCursorUsageNotInstalled(t *testing.T) {
	row := fetchCursorUsage("http://127.0.0.1:1", filepath.Join(t.TempDir(), "missing.vscdb"), nil)
	if row.Status != "not-installed" {
		t.Fatalf("status=%s", row.Status)
	}
}

func TestFetchCursorUsageNotSignedIn(t *testing.T) {
	path := seedStateDB(t, "")
	row := fetchCursorUsage("http://127.0.0.1:1", path, nil)
	if row.Status != "error" || row.Error != "not signed in" {
		t.Fatalf("row=%+v", row)
	}
}

func TestFetchCursorUsageAuthJSONFallback(t *testing.T) {
	tok := testJWT("user_json")
	authPath := filepath.Join(t.TempDir(), "auth.json")
	if err := os.WriteFile(authPath, []byte(`{"accessToken":"`+tok+`"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Cookie"), "user_json") {
			http.Error(w, "bad cookie", http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{
			"billingCycleEnd": "2026-08-04T00:00:00Z",
			"membershipType": "pro",
			"individualUsage": {
				"plan": { "autoPercentUsed": 1, "apiPercentUsed": 2, "totalPercentUsed": 1 }
			}
		}`))
	}))
	defer srv.Close()

	row := fetchCursorUsage(srv.URL, filepath.Join(t.TempDir(), "no.vscdb"), []string{authPath})
	if row.Status != "ok" || row.Limits[0].Percent != 1 {
		t.Fatalf("row=%+v", row)
	}
}

func TestSanitizeCursorErr(t *testing.T) {
	tok := testJWT("user_x")
	got := sanitizeCursorErr(errWith(tok))
	if strings.Contains(got, tok) || strings.Contains(got, "eyJ") {
		t.Fatalf("leaked token: %q", got)
	}
}

type simpleErr string

func (e simpleErr) Error() string { return string(e) }

func errWith(tok string) error { return simpleErr("failed " + tok) }
