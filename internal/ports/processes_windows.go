//go:build windows

package ports

import (
	"fmt"
	"os/exec"

	"github.com/erikharutyunyan/double-pane/internal/domain"
)

// ListProcesses returns processes from tasklist (Windows has no uid filter).
func ListProcesses() ([]domain.ProcessInfo, error) {
	cmd := exec.Command("tasklist", "/FO", "CSV", "/NH")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("tasklist: %w", err)
	}
	return processesFromTasklist(string(out)), nil
}
