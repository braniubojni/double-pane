//go:build darwin || linux

package ports

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/erikharutyunyan/double-pane/internal/domain"
)

// ListProcesses returns processes owned by the current user.
func ListProcesses() ([]domain.ProcessInfo, error) {
	cmd := exec.Command("ps", "-axww", "-o", "pid=,uid=,comm=,args=")
	out, err := cmd.Output()
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return []domain.ProcessInfo{}, nil
		}
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			list := parsePsAXO(string(out), os.Getuid())
			attachCwd(list)
			return list, nil
		}
		return nil, fmt.Errorf("ps: %w", err)
	}
	list := parsePsAXO(string(out), os.Getuid())
	attachCwd(list)
	return list, nil
}

func attachCwd(list []domain.ProcessInfo) {
	cmd := exec.Command("lsof", "-nP", "-a", "-d", "cwd", "-Fn")
	out, err := cmd.Output()
	if err != nil {
		return
	}
	applyCwd(list, parseLsofCwd(string(out)))
}
