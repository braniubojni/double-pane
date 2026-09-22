//go:build linux

package config

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/erikharutyunyan/double-pane/internal/domain"
)

const linuxDesktopCap = 50

func listOpenWithApps(path string) ([]domain.OpenWithApp, error) {
	mime := linuxFileMIME(path)
	if mime == "" {
		return nil, nil
	}
	var apps []domain.OpenWithApp
	seen := make(map[string]struct{})
	for _, dir := range linuxDesktopDirs() {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() || !strings.HasSuffix(name, ".desktop") {
				continue
			}
			raw, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				continue
			}
			app, ok := parseDesktopEntry(name, string(raw))
			if !ok || !desktopMatchesMIME(app.MimeTypes, mime) {
				continue
			}
			if _, dup := seen[app.ID]; dup {
				continue
			}
			seen[app.ID] = struct{}{}
			apps = append(apps, domain.OpenWithApp{ID: app.ID, Name: app.Name})
		}
	}
	sort.Slice(apps, func(i, j int) bool {
		return strings.ToLower(apps[i].Name) < strings.ToLower(apps[j].Name)
	})
	if len(apps) > linuxDesktopCap {
		apps = apps[:linuxDesktopCap]
	}
	return apps, nil
}

func linuxFileMIME(path string) string {
	if _, err := exec.LookPath("xdg-mime"); err != nil {
		return ""
	}
	out, err := exec.Command("xdg-mime", "query", "filetype", path).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func linuxDesktopDirs() []string {
	dirs := []string{"/usr/share/applications"}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		dirs = append([]string{filepath.Join(home, ".local/share/applications")}, dirs...)
	}
	return dirs
}

func openWith(path, appID string) error {
	if _, err := exec.LookPath("gtk-launch"); err == nil {
		cmd, args := linuxGtkLaunchCmd(appID, path)
		if err := runDetached(cmd, args...); err == nil {
			return nil
		}
	}
	if desktop := findDesktopFile(appID); desktop != "" {
		if _, err := exec.LookPath("gio"); err == nil {
			cmd, args := linuxGioLaunchCmd(desktop, path)
			if err := runDetached(cmd, args...); err == nil {
				return nil
			}
		}
	}
	return runDetached("xdg-open", path)
}

func findDesktopFile(id string) string {
	if filepath.IsAbs(id) {
		if _, err := os.Stat(id); err == nil {
			return id
		}
	}
	name := id
	if !strings.HasSuffix(name, ".desktop") {
		name += ".desktop"
	}
	for _, dir := range linuxDesktopDirs() {
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func openWithPicker(path string) error {
	if _, err := exec.LookPath("mimeopen"); err != nil {
		return fmt.Errorf("no application picker (install mimeopen / perl-file-mimeinfo)")
	}
	cmd, args := linuxPickerCmd(path)
	return runDetached(cmd, args...)
}
