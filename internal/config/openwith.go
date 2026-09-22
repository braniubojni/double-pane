package config

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/erikharutyunyan/double-pane/internal/domain"
)

// ListOpenWithApps returns applications that can open a local file.
func ListOpenWithApps(path string) ([]domain.OpenWithApp, error) {
	if path == "" {
		return nil, fmt.Errorf("empty path")
	}
	return listOpenWithApps(path)
}

// OpenWith opens path with the application identified by appID.
func OpenWith(path, appID string) error {
	if path == "" {
		return fmt.Errorf("empty path")
	}
	if appID == "" {
		return fmt.Errorf("empty application")
	}
	return openWith(path, appID)
}

// OpenWithPicker opens the OS “open with” chooser for path.
func OpenWithPicker(path string) error {
	if path == "" {
		return fmt.Errorf("empty path")
	}
	return openWithPicker(path)
}

func darwinOpenWithCmd(appName, path string) (string, []string) {
	return "open", []string{"-a", appName, "--", path}
}

func darwinAppName(appID string) string {
	return strings.TrimSuffix(filepath.Base(appID), ".app")
}

func linuxPickerCmd(path string) (string, []string) {
	return "mimeopen", []string{"-a", path}
}

func windowsPickerCmd(path string) (string, []string) {
	return "rundll32", []string{"shell32.dll,OpenAs_RunDLL", path}
}

func linuxGtkLaunchCmd(appID, path string) (string, []string) {
	id := strings.TrimSuffix(appID, ".desktop")
	return "gtk-launch", []string{id, path}
}

func linuxGioLaunchCmd(desktopPath, path string) (string, []string) {
	return "gio", []string{"launch", desktopPath, path}
}
