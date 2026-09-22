package service

import (
	"github.com/erikharutyunyan/double-pane/internal/domain"
	"github.com/erikharutyunyan/double-pane/internal/ports"
)

// PortService lists local TCP listeners and force-kills processes by PID.
type PortService struct{}

func NewPortService() *PortService {
	return &PortService{}
}

func (s *PortService) List() ([]domain.PortListener, error) {
	return ports.List()
}

func (s *PortService) ListProcesses() ([]domain.ProcessInfo, error) {
	return ports.ListProcesses()
}

func (s *PortService) Kill(pid int) error {
	return ports.Kill(pid)
}

func (s *PortService) KillAll(pids []int) error {
	return ports.KillAll(pids)
}
