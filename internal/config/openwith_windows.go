//go:build windows

package config

import "github.com/erikharutyunyan/double-pane/internal/domain"

func listOpenWithApps(string) ([]domain.OpenWithApp, error) {
	return nil, nil
}

func openWith(path, appID string) error {
	return runDetached(appID, path)
}

func openWithPicker(path string) error {
	cmd, args := windowsPickerCmd(path)
	return runDetached(cmd, args...)
}
