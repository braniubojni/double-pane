package service

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/erikharutyunyan/double-pane/internal/config"
	"github.com/erikharutyunyan/double-pane/internal/domain"
	"github.com/erikharutyunyan/double-pane/internal/storage"
)

const (
	kvSettings    = "settings"
	kvShortcuts   = "shortcuts"
	kvTabs        = "tabs"
	kvSearchPrefs = "search_prefs"
	kvGridPrefs   = "grid_prefs"
	kvWindow      = "window"
)

// SettingsService persists app settings and shortcuts in encrypted SQLite (app.db).
// Legacy settings.json / shortcuts.json are migrated once on first load.
type SettingsService struct {
	db  *storage.DB
	cfg *config.Store
}

func NewSettingsService(db *storage.DB, cfg *config.Store) *SettingsService {
	return &SettingsService{db: db, cfg: cfg}
}

func (s *SettingsService) GetSettings() (domain.Settings, error) {
	if err := s.migrateJSONIfNeeded(); err != nil {
		return domain.Settings{}, err
	}
	raw, err := s.db.GetKV(kvSettings)
	if err != nil {
		return domain.Settings{}, err
	}
	if raw == nil {
		def := config.DefaultSettings()
		if err := s.SaveSettings(def); err != nil {
			return domain.Settings{}, err
		}
		return def, nil
	}
	return parseSettingsJSON(raw), nil
}

func (s *SettingsService) SaveSettings(settings domain.Settings) error {
	settings = normalizeSettingsForSave(settings)
	data, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	return s.db.SetKV(kvSettings, data)
}

// GetPaneTabs returns each pane's saved tab list. If no tabs blob was ever
// saved, or a pane's list ended up empty, it falls back to a single tab
// derived from the legacy Settings.LeftPath/RightPath so existing installs
// (and e2e's settings.json seed) keep working.
func (s *SettingsService) GetPaneTabs() (domain.PaneTabs, error) {
	st, err := s.GetSettings()
	if err != nil {
		return domain.PaneTabs{}, err
	}
	raw, err := s.db.GetKV(kvTabs)
	if err != nil {
		return domain.PaneTabs{}, err
	}
	var tabs domain.PaneTabs
	if raw != nil {
		if err := json.Unmarshal(raw, &tabs); err != nil {
			tabs = domain.PaneTabs{}
		}
	}
	if len(tabs.Left) == 0 {
		tabs.Left = []domain.TabState{{Path: st.LeftPath}}
		tabs.LeftActive = 0
	}
	if len(tabs.Right) == 0 {
		tabs.Right = []domain.TabState{{Path: st.RightPath}}
		tabs.RightActive = 0
	}
	tabs.LeftActive = clampIndex(tabs.LeftActive, len(tabs.Left))
	tabs.RightActive = clampIndex(tabs.RightActive, len(tabs.Right))
	return tabs, nil
}

// SavePaneTabs persists each pane's tab list and mirrors the active tab's
// path into Settings.LeftPath/RightPath, which stays the boot fallback.
func (s *SettingsService) SavePaneTabs(tabs domain.PaneTabs) error {
	tabs.Left = dropEmptyTabs(tabs.Left)
	tabs.Right = dropEmptyTabs(tabs.Right)
	tabs.LeftActive = clampIndex(tabs.LeftActive, len(tabs.Left))
	tabs.RightActive = clampIndex(tabs.RightActive, len(tabs.Right))

	data, err := json.Marshal(tabs)
	if err != nil {
		return err
	}
	if err := s.db.SetKV(kvTabs, data); err != nil {
		return err
	}

	st, err := s.GetSettings()
	if err != nil {
		return err
	}
	if len(tabs.Left) > 0 {
		st.LeftPath = tabs.Left[tabs.LeftActive].Path
	}
	if len(tabs.Right) > 0 {
		st.RightPath = tabs.Right[tabs.RightActive].Path
	}
	return s.SaveSettings(st)
}

// GetGridPrefs returns each pane's file-grid sort, visibility, and column order.
// Missing or invalid blobs yield defaults (Name/asc, nothing hidden, empty order).
func (s *SettingsService) GetGridPrefs() (domain.GridPrefs, error) {
	raw, err := s.db.GetKV(kvGridPrefs)
	if err != nil {
		return domain.GridPrefs{}, err
	}
	if raw == nil {
		return defaultGridPrefs(), nil
	}
	var prefs domain.GridPrefs
	if err := json.Unmarshal(raw, &prefs); err != nil {
		return defaultGridPrefs(), nil
	}
	prefs.Left = normalizePaneGridPrefs(prefs.Left)
	prefs.Right = normalizePaneGridPrefs(prefs.Right)
	return prefs, nil
}

// SaveGridPrefs persists each pane's file-grid preferences.
func (s *SettingsService) SaveGridPrefs(prefs domain.GridPrefs) error {
	prefs.Left = normalizePaneGridPrefs(prefs.Left)
	prefs.Right = normalizePaneGridPrefs(prefs.Right)
	data, err := json.Marshal(prefs)
	if err != nil {
		return err
	}
	return s.db.SetKV(kvGridPrefs, data)
}

// GetWindowState returns the last saved main-window size. Missing or invalid
// blobs yield the default size (wide enough for every file-grid column).
func (s *SettingsService) GetWindowState() (domain.WindowState, error) {
	raw, err := s.db.GetKV(kvWindow)
	if err != nil {
		return domain.WindowState{}, err
	}
	if raw == nil {
		return defaultWindowState(), nil
	}
	var st domain.WindowState
	if err := json.Unmarshal(raw, &st); err != nil {
		return defaultWindowState(), nil
	}
	return clampWindowState(st), nil
}

// SaveWindowState persists the main-window size for the next launch.
func (s *SettingsService) SaveWindowState(st domain.WindowState) error {
	st = clampWindowState(st)
	data, err := json.Marshal(st)
	if err != nil {
		return err
	}
	return s.db.SetKV(kvWindow, data)
}

func defaultWindowState() domain.WindowState {
	return domain.WindowState{
		Width:  domain.DefaultWindowWidth,
		Height: domain.DefaultWindowHeight,
	}
}

func clampWindowState(st domain.WindowState) domain.WindowState {
	if st.Width < domain.MinWindowWidth || st.Width > domain.MaxWindowWidth {
		st.Width = domain.DefaultWindowWidth
	}
	if st.Height < domain.MinWindowHeight || st.Height > domain.MaxWindowHeight {
		st.Height = domain.DefaultWindowHeight
	}
	return st
}

func defaultPaneGridPrefs() domain.PaneGridPrefs {
	return domain.PaneGridPrefs{
		SortField: "displayName",
		SortDir:   "asc",
		Hidden:    []string{},
		Order:     []string{},
	}
}

func defaultGridPrefs() domain.GridPrefs {
	return domain.GridPrefs{
		Left:  defaultPaneGridPrefs(),
		Right: defaultPaneGridPrefs(),
	}
}

func normalizePaneGridPrefs(p domain.PaneGridPrefs) domain.PaneGridPrefs {
	if p.SortField == "" {
		p.SortField = "displayName"
	}
	if p.SortDir != "desc" {
		p.SortDir = "asc"
	}
	p.Hidden = filterHiddenFields(p.Hidden)
	if p.Order == nil {
		p.Order = []string{}
	}
	return p
}

func filterHiddenFields(hidden []string) []string {
	if len(hidden) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(hidden))
	for _, f := range hidden {
		if f == "" || f == "icon" || f == "displayName" {
			continue
		}
		out = append(out, f)
	}
	return out
}

func clampIndex(idx, length int) int {
	if length == 0 {
		return 0
	}
	if idx < 0 {
		return 0
	}
	if idx >= length {
		return length - 1
	}
	return idx
}

func dropEmptyTabs(tabs []domain.TabState) []domain.TabState {
	out := make([]domain.TabState, 0, len(tabs))
	for _, t := range tabs {
		if t.Path != "" {
			out = append(out, t)
		}
	}
	return out
}

func (s *SettingsService) GetShortcuts() (map[string]string, error) {
	if err := s.migrateJSONIfNeeded(); err != nil {
		return nil, err
	}
	raw, err := s.db.GetKV(kvShortcuts)
	if err != nil {
		return nil, err
	}
	if raw == nil {
		def := config.DefaultShortcuts()
		if err := s.SaveShortcuts(def); err != nil {
			return nil, err
		}
		return def, nil
	}
	var m map[string]string
	if err := json.Unmarshal(raw, &m); err != nil {
		return config.DefaultShortcuts(), nil
	}
	return mergeShortcuts(m), nil
}

func (s *SettingsService) SaveShortcuts(shortcuts map[string]string) error {
	merged := mergeShortcuts(shortcuts)
	data, err := json.Marshal(merged)
	if err != nil {
		return err
	}
	return s.db.SetKV(kvShortcuts, data)
}

func (s *SettingsService) ListShortcutDefs() ([]domain.ShortcutDef, error) {
	cur, err := s.GetShortcuts()
	if err != nil {
		return nil, err
	}
	catalog := config.ShortcutCatalog()
	for i := range catalog {
		if b, ok := cur[catalog[i].ID]; ok {
			catalog[i].Binding = b
		}
	}
	return catalog, nil
}

func (s *SettingsService) GetConfigDir() string {
	if s.cfg != nil {
		return s.cfg.Dir()
	}
	return s.db.Dir()
}

func (s *SettingsService) GetSettingsPath() string {
	// Logical path: encrypted blob lives in app.db; report config dir for UI.
	return filepath.Join(s.GetConfigDir(), "app.db")
}

func (s *SettingsService) GetShortcutsPath() string {
	return filepath.Join(s.GetConfigDir(), "app.db")
}

func (s *SettingsService) OpenInOS(path string) error {
	return config.OpenInOS(path)
}

func (s *SettingsService) RevealInOS(path string) error {
	return config.RevealInOS(path)
}

// GetSearchPrefs returns last Find-in-files dialog fields.
func (s *SettingsService) GetSearchPrefs() (domain.SearchPrefs, error) {
	raw, err := s.db.GetKV(kvSearchPrefs)
	if err != nil {
		return domain.SearchPrefs{}, err
	}
	if raw == nil {
		return defaultSearchPrefs(), nil
	}
	var p domain.SearchPrefs
	if err := json.Unmarshal(raw, &p); err != nil {
		return defaultSearchPrefs(), nil
	}
	return normalizeSearchPrefs(p), nil
}

// SaveSearchPrefs persists Find-in-files dialog fields.
func (s *SettingsService) SaveSearchPrefs(prefs domain.SearchPrefs) error {
	prefs = normalizeSearchPrefs(prefs)
	data, err := json.Marshal(prefs)
	if err != nil {
		return err
	}
	return s.db.SetKV(kvSearchPrefs, data)
}

// AddSearchHistory records a non-empty value for field (query|replace|include|exclude).
func (s *SettingsService) AddSearchHistory(field, value string) error {
	return s.db.AddSearchHistory(field, value)
}

// ListSearchHistory returns newest-first history for field (max 500).
func (s *SettingsService) ListSearchHistory(field string, limit int) ([]string, error) {
	return s.db.ListSearchHistory(field, limit)
}

func defaultSearchPrefs() domain.SearchPrefs {
	return domain.SearchPrefs{
		Mode: domain.SearchModeContent,
	}
}

func normalizeSearchPrefs(p domain.SearchPrefs) domain.SearchPrefs {
	switch p.Mode {
	case domain.SearchModeContent, domain.SearchModeFolders:
	default:
		p.Mode = domain.SearchModeContent
	}
	return p
}

// migrateJSONIfNeeded imports settings.json / shortcuts.json when present.
// Files are always re-imported then renamed to .bak so e2e can rewrite JSON
// without deleting app.db (which would break an open SQLite handle).
func (s *SettingsService) migrateJSONIfNeeded() error {
	dir := s.GetConfigDir()

	p := filepath.Join(dir, "settings.json")
	if data, err := os.ReadFile(p); err == nil {
		if err := s.db.SetKV(kvSettings, data); err != nil {
			return err
		}
		_ = os.Rename(p, p+".bak")
	} else if !os.IsNotExist(err) {
		return err
	}

	p = filepath.Join(dir, "shortcuts.json")
	if data, err := os.ReadFile(p); err == nil {
		if err := s.db.SetKV(kvShortcuts, data); err != nil {
			return err
		}
		_ = os.Rename(p, p+".bak")
	} else if !os.IsNotExist(err) {
		return err
	}
	return nil
}

func parseSettingsJSON(data []byte) domain.Settings {
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return config.DefaultSettings()
	}
	out := config.DefaultSettings()
	if v, ok := m["theme"].(string); ok {
		switch v {
		case domain.ThemeSystem, domain.ThemeDark, domain.ThemeLight:
			out.Theme = v
		}
	}
	if v, ok := m["showHidden"].(bool); ok {
		out.ShowHidden = v
	}
	if v, ok := m["showExtensions"].(bool); ok {
		out.ShowExtensions = v
	}
	if v, ok := m["showGitStatus"].(bool); ok {
		out.ShowGitStatus = v
	}
	if v, ok := m["useBuiltInEditor"].(bool); ok {
		out.UseBuiltInEditor = v
	}
	if v, ok := m["autoCheckUpdates"].(bool); ok {
		out.AutoCheckUpdates = v
	}
	if v, ok := m["updateCheckIntervalDays"].(float64); ok && int(v) > 0 {
		out.UpdateCheckIntervalDays = int(v)
	}
	if v, ok := m["lastUpdateCheckAt"].(string); ok {
		out.LastUpdateCheckAt = v
	}
	if v, ok := m["skippedUpdateVersion"].(string); ok {
		out.SkippedUpdateVersion = v
	}
	if v, ok := m["leftPath"].(string); ok {
		out.LeftPath = v
	}
	if v, ok := m["rightPath"].(string); ok {
		out.RightPath = v
	}
	return out
}

func normalizeSettingsForSave(in domain.Settings) domain.Settings {
	out := in
	switch out.Theme {
	case domain.ThemeSystem, domain.ThemeDark, domain.ThemeLight:
	default:
		out.Theme = domain.ThemeSystem
	}
	if out.UpdateCheckIntervalDays <= 0 {
		out.UpdateCheckIntervalDays = 10
	}
	return out
}

func mergeShortcuts(raw map[string]string) map[string]string {
	out := config.DefaultShortcuts()
	for k, v := range raw {
		if v != "" {
			out[k] = v
		}
	}
	return out
}
