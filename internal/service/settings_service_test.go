package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/erikharutyunyan/double-pane/internal/config"
	"github.com/erikharutyunyan/double-pane/internal/domain"
	"github.com/erikharutyunyan/double-pane/internal/storage"
)

func TestSettingsEncryptedMigrateFromJSON(t *testing.T) {
	dir := t.TempDir()
	// legacy JSON
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(`{
  "theme": "dark",
  "showHidden": true,
  "showExtensions": false,
  "leftPath": "/left",
  "rightPath": "/right"
}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	db, err := storage.OpenPath(filepath.Join(dir, "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	cfg, err := config.OpenDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	svc := NewSettingsService(db, cfg)

	s, err := svc.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if s.Theme != domain.ThemeDark || !s.ShowHidden || s.ShowExtensions {
		t.Fatalf("migrated settings: %+v", s)
	}
	if s.LeftPath != "/left" || s.RightPath != "/right" {
		t.Fatalf("paths: %+v", s)
	}

	// JSON should be renamed to .bak
	if _, err := os.Stat(filepath.Join(dir, "settings.json")); !os.IsNotExist(err) {
		t.Fatalf("expected settings.json removed/renamed, err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "settings.json.bak")); err != nil {
		t.Fatalf("expected bak: %v", err)
	}

	// Round-trip save
	s.Theme = domain.ThemeLight
	if err := svc.SaveSettings(s); err != nil {
		t.Fatal(err)
	}
	s2, err := svc.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if s2.Theme != domain.ThemeLight {
		t.Fatalf("got theme %q", s2.Theme)
	}
}

func newTestSettingsService(t *testing.T) *SettingsService {
	t.Helper()
	dir := t.TempDir()
	db, err := storage.OpenPath(filepath.Join(dir, "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	cfg, err := config.OpenDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	return NewSettingsService(db, cfg)
}

func TestPaneTabsFallsBackToSettingsPaths(t *testing.T) {
	svc := newTestSettingsService(t)

	st, err := svc.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	st.LeftPath = "/left"
	st.RightPath = "/right"
	if err := svc.SaveSettings(st); err != nil {
		t.Fatal(err)
	}

	tabs, err := svc.GetPaneTabs()
	if err != nil {
		t.Fatal(err)
	}
	if len(tabs.Left) != 1 || tabs.Left[0].Path != "/left" || tabs.LeftActive != 0 {
		t.Fatalf("left fallback: %+v", tabs)
	}
	if len(tabs.Right) != 1 || tabs.Right[0].Path != "/right" || tabs.RightActive != 0 {
		t.Fatalf("right fallback: %+v", tabs)
	}
}

func TestPaneTabsRoundTripAndClamp(t *testing.T) {
	svc := newTestSettingsService(t)

	in := domain.PaneTabs{
		Left:        []domain.TabState{{Path: "/a"}, {Path: "/b"}, {Path: ""}},
		LeftActive:  99, // out of range, should clamp
		Right:       []domain.TabState{{Path: "/c"}},
		RightActive: -1, // out of range, should clamp
	}
	if err := svc.SavePaneTabs(in); err != nil {
		t.Fatal(err)
	}

	out, err := svc.GetPaneTabs()
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Left) != 2 || out.Left[0].Path != "/a" || out.Left[1].Path != "/b" {
		t.Fatalf("left after drop-empty: %+v", out.Left)
	}
	if out.LeftActive != 1 {
		t.Fatalf("leftActive clamp: got %d", out.LeftActive)
	}
	if len(out.Right) != 1 || out.Right[0].Path != "/c" || out.RightActive != 0 {
		t.Fatalf("right: %+v active=%d", out.Right, out.RightActive)
	}

	// Active tab paths should mirror into legacy Settings fields.
	st, err := svc.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if st.LeftPath != "/b" || st.RightPath != "/c" {
		t.Fatalf("mirrored settings: %+v", st)
	}
}

func TestGridPrefsDefaultWhenMissing(t *testing.T) {
	svc := newTestSettingsService(t)

	got, err := svc.GetGridPrefs()
	if err != nil {
		t.Fatal(err)
	}
	assertDefaultPaneGridPrefs(t, "left", got.Left)
	assertDefaultPaneGridPrefs(t, "right", got.Right)
}

func TestGridPrefsRoundTrip(t *testing.T) {
	svc := newTestSettingsService(t)

	in := domain.GridPrefs{
		Left: domain.PaneGridPrefs{
			SortField: "size",
			SortDir:   "desc",
			Hidden:    []string{"ext", "displayName", "icon", "access"},
			Order:     []string{"displayName", "size", "modTime"},
		},
		Right: domain.PaneGridPrefs{
			SortField: "modTime",
			SortDir:   "asc",
			Hidden:    []string{"access"},
			Order:     []string{"icon", "displayName", "ext"},
		},
	}
	if err := svc.SaveGridPrefs(in); err != nil {
		t.Fatal(err)
	}

	out, err := svc.GetGridPrefs()
	if err != nil {
		t.Fatal(err)
	}
	if out.Left.SortField != "size" || out.Left.SortDir != "desc" {
		t.Fatalf("left sort: %+v", out.Left)
	}
	if len(out.Left.Hidden) != 2 || out.Left.Hidden[0] != "ext" || out.Left.Hidden[1] != "access" {
		t.Fatalf("left hidden (icon/name stripped): %+v", out.Left.Hidden)
	}
	if len(out.Left.Order) != 3 || out.Left.Order[0] != "displayName" {
		t.Fatalf("left order: %+v", out.Left.Order)
	}
	if out.Right.SortField != "modTime" || out.Right.SortDir != "asc" {
		t.Fatalf("right sort: %+v", out.Right)
	}
	if len(out.Right.Hidden) != 1 || out.Right.Hidden[0] != "access" {
		t.Fatalf("right hidden: %+v", out.Right.Hidden)
	}
	if len(out.Right.Order) != 3 || out.Right.Order[2] != "ext" {
		t.Fatalf("right order: %+v", out.Right.Order)
	}
}

func TestWindowStateDefaultAndRoundTrip(t *testing.T) {
	svc := newTestSettingsService(t)

	got, err := svc.GetWindowState()
	if err != nil {
		t.Fatal(err)
	}
	if got.Width != domain.DefaultWindowWidth || got.Height != domain.DefaultWindowHeight {
		t.Fatalf("default window: %+v", got)
	}

	in := domain.WindowState{Width: 1920, Height: 1080}
	if err := svc.SaveWindowState(in); err != nil {
		t.Fatal(err)
	}
	out, err := svc.GetWindowState()
	if err != nil {
		t.Fatal(err)
	}
	if out.Width != 1920 || out.Height != 1080 {
		t.Fatalf("round-trip: %+v", out)
	}

	if err := svc.SaveWindowState(domain.WindowState{Width: 100, Height: 100}); err != nil {
		t.Fatal(err)
	}
	clamped, err := svc.GetWindowState()
	if err != nil {
		t.Fatal(err)
	}
	if clamped.Width != domain.DefaultWindowWidth || clamped.Height != domain.DefaultWindowHeight {
		t.Fatalf("too-small should clamp to default: %+v", clamped)
	}
}

func assertDefaultPaneGridPrefs(t *testing.T, side string, p domain.PaneGridPrefs) {
	t.Helper()
	if p.SortField != "displayName" || p.SortDir != "asc" {
		t.Fatalf("%s sort defaults: %+v", side, p)
	}
	if len(p.Hidden) != 0 {
		t.Fatalf("%s hidden want empty, got %+v", side, p.Hidden)
	}
	if len(p.Order) != 0 {
		t.Fatalf("%s order want empty, got %+v", side, p.Order)
	}
}

func TestSearchPrefsAndHistory(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GFM_CONFIG_DIR", dir)
	db, err := storage.OpenPath(filepath.Join(dir, "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	svc := NewSettingsService(db, nil)

	prefs, err := svc.GetSearchPrefs()
	if err != nil {
		t.Fatal(err)
	}
	if prefs.Mode != domain.SearchModeContent {
		t.Fatalf("default mode: %q", prefs.Mode)
	}
	prefs.Query = "needle"
	prefs.Include = "*.go"
	prefs.Exclude = "build"
	prefs.Mode = domain.SearchModeFolders
	prefs.ReplaceOpen = true
	if err := svc.SaveSearchPrefs(prefs); err != nil {
		t.Fatal(err)
	}
	got, err := svc.GetSearchPrefs()
	if err != nil {
		t.Fatal(err)
	}
	if got.Query != "needle" || got.Mode != domain.SearchModeFolders || !got.ReplaceOpen {
		t.Fatalf("got %+v", got)
	}

	if err := svc.AddSearchHistory("query", "needle"); err != nil {
		t.Fatal(err)
	}
	if err := svc.AddSearchHistory("query", "other"); err != nil {
		t.Fatal(err)
	}
	hist, err := svc.ListSearchHistory("query", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(hist) < 2 || hist[0] != "other" {
		t.Fatalf("hist=%#v", hist)
	}
}
