//go:build !darwin && !linux && !windows

package volumes

import "github.com/erikharutyunyan/double-pane/internal/domain"

func listOS() ([]domain.Volume, error) {
	return nil, nil
}
