//go:build darwin

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

func listOpenWithApps(_ string) ([]domain.OpenWithApp, error) {
	dirs := []string{"/Applications"}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		dirs = append(dirs, filepath.Join(home, "Applications"))
	}
	seen := make(map[string]struct{})
	var apps []domain.OpenWithApp
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			name := e.Name()
			if !strings.HasSuffix(name, ".app") {
				continue
			}
			id := filepath.Join(dir, name)
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			apps = append(apps, domain.OpenWithApp{
				ID:   id,
				Name: strings.TrimSuffix(name, ".app"),
			})
		}
	}
	sort.Slice(apps, func(i, j int) bool {
		return strings.ToLower(apps[i].Name) < strings.ToLower(apps[j].Name)
	})
	return apps, nil
}

func openWith(path, appID string) error {
	cmd, args := darwinOpenWithCmd(darwinAppName(appID), path)
	return runDetached(cmd, args...)
}

func openWithPicker(path string) error {
	out, err := exec.Command("osascript", "-e", "POSIX path of (choose application as alias)").Output()
	if err != nil {
		return fmt.Errorf("choose application: %w", err)
	}
	app := strings.TrimSpace(string(out))
	if app == "" {
		return fmt.Errorf("no application selected")
	}
	cmd, args := darwinOpenWithCmd(app, path)
	return runDetached(cmd, args...)
}
