//go:build !darwin && !linux && !windows

package ports

import "github.com/erikharutyunyan/double-pane/internal/domain"

func ListProcesses() ([]domain.ProcessInfo, error) {
	return []domain.ProcessInfo{}, nil
}
