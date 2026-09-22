//go:build !darwin && !linux && !windows

package ports

import "github.com/erikharutyunyan/double-pane/internal/domain"

func List() ([]domain.PortListener, error) {
	return []domain.PortListener{}, nil
}
