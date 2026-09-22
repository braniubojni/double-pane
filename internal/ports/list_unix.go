//go:build darwin || linux

package ports

import (
	"errors"
	"fmt"
	"os/exec"

	"github.com/erikharutyunyan/double-pane/internal/domain"
)

// List returns local TCP listeners (lsof).
func List() ([]domain.PortListener, error) {
	cmd := exec.Command("lsof", "-nP", "-iTCP", "-sTCP:LISTEN", "-F", "cPn")
	out, err := cmd.Output()
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return []domain.PortListener{}, nil
		}
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return parseLsofF(string(out)), nil
		}
		return nil, fmt.Errorf("lsof: %w", err)
	}
	return parseLsofF(string(out)), nil
}
