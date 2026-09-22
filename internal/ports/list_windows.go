//go:build windows

package ports

import (
	"fmt"
	"os/exec"

	"github.com/erikharutyunyan/double-pane/internal/domain"
)

// List returns local TCP listeners (netstat + tasklist).
func List() ([]domain.PortListener, error) {
	cmd := exec.Command("netstat", "-ano", "-p", "tcp")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("netstat: %w", err)
	}
	list := parseNetstatANO(string(out))
	names := processNames()
	attachProcessNames(list, names)
	return list, nil
}

func processNames() map[int]string {
	cmd := exec.Command("tasklist", "/FO", "CSV", "/NH")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	return parseTasklistCSV(string(out))
}

func attachProcessNames(list []domain.PortListener, names map[int]string) {
	for i := range list {
		if n, ok := names[list[i].PID]; ok {
			list[i].Process = n
		}
	}
}
