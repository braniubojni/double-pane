//go:build !darwin && !linux && !windows

package config

import (
	"fmt"

	"github.com/erikharutyunyan/double-pane/internal/domain"
)

func listOpenWithApps(string) ([]domain.OpenWithApp, error) {
	return nil, nil
}

func openWith(string, string) error {
	return fmt.Errorf("open with is not supported on this platform")
}

func openWithPicker(string) error {
	return fmt.Errorf("open with is not supported on this platform")
}
