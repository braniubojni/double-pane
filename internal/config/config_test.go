package config

import (
	"path/filepath"
	"testing"

	"github.com/erikharutyunyan/go-file-manager/internal/domain"
)

func TestLoadSaveSettings(t *testing.T) {
	dir := t.TempDir()
	store, err := OpenDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	s, err := store.LoadSettingsStrict()
	if err != nil {
		t.Fatal(err)
	}
	if s.Theme != domain.ThemeSystem || !s.ShowExtensions || s.ShowHidden {
		t.Fatalf("unexpected defaults: %+v", s)
	}

	s.Theme = domain.ThemeDark
	s.ShowHidden = true
	s.ShowExtensions = false
	s.LeftPath = "/tmp/a"
	s.RightPath = "/tmp/b"
	if err := store.SaveSettings(s); err != nil {
		t.Fatal(err)
	}

	loaded, err := store.LoadSettingsStrict()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Theme != domain.ThemeDark || !loaded.ShowHidden || loaded.ShowExtensions {
		t.Fatalf("unexpected loaded: %+v", loaded)
	}
	if loaded.LeftPath != "/tmp/a" || loaded.RightPath != "/tmp/b" {
		t.Fatalf("paths: %+v", loaded)
	}
}

func TestShortcutsMerge(t *testing.T) {
	dir := t.TempDir()
	store, err := OpenDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveShortcuts(map[string]string{"refresh": "F6"}); err != nil {
		t.Fatal(err)
	}
	m, err := store.LoadShortcuts()
	if err != nil {
		t.Fatal(err)
	}
	if m["refresh"] != "F6" {
		t.Fatalf("refresh=%q", m["refresh"])
	}
	if m["switchPane"] != "Tab" {
		t.Fatalf("switchPane should keep default, got %q", m["switchPane"])
	}
	if filepath.Base(store.SettingsPath()) != "settings.json" {
		t.Fatal(store.SettingsPath())
	}
}

func TestCommandPaletteShortcut(t *testing.T) {
	if got := DefaultShortcuts()["commandPalette"]; got != "Mod+K" {
		t.Fatalf("commandPalette default=%q", got)
	}
	for _, d := range ShortcutCatalog() {
		if d.ID == "commandPalette" {
			return
		}
	}
	t.Fatal("ShortcutCatalog missing commandPalette")
}

func TestSelectAllShortcut(t *testing.T) {
	if got := DefaultShortcuts()["selectAll"]; got != "Mod+A" {
		t.Fatalf("selectAll default=%q", got)
	}
	for _, d := range ShortcutCatalog() {
		if d.ID == "selectAll" {
			return
		}
	}
	t.Fatal("ShortcutCatalog missing selectAll")
}

func TestSameDirShortcuts(t *testing.T) {
	if got := DefaultShortcuts()["sameDirLeft"]; got != "Ctrl+ArrowLeft" {
		t.Fatalf("sameDirLeft default=%q", got)
	}
	if got := DefaultShortcuts()["sameDirRight"]; got != "Ctrl+ArrowRight" {
		t.Fatalf("sameDirRight default=%q", got)
	}
	var left, right bool
	for _, d := range ShortcutCatalog() {
		if d.ID == "sameDirLeft" {
			left = true
		}
		if d.ID == "sameDirRight" {
			right = true
		}
	}
	if !left || !right {
		t.Fatal("ShortcutCatalog missing sameDirLeft/sameDirRight")
	}
}

func TestDeletePermanentShortcut(t *testing.T) {
	if got := DefaultShortcuts()["deletePermanent"]; got != "Shift+Delete" {
		t.Fatalf("deletePermanent default=%q", got)
	}
	for _, d := range ShortcutCatalog() {
		if d.ID == "deletePermanent" {
			return
		}
	}
	t.Fatal("ShortcutCatalog missing deletePermanent")
}
