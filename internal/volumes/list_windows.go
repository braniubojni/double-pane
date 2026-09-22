//go:build windows

package volumes

import (
	"os"

	"github.com/erikharutyunyan/double-pane/internal/domain"
)

func listOS() ([]domain.Volume, error) {
	var out []domain.Volume
	for c := 'A'; c <= 'Z'; c++ {
		p := string(c) + `:\`
		if _, err := os.Stat(p); err != nil {
			continue
		}
		v := domain.Volume{
			Path:        p,
			Name:        string(c) + ":",
			Kind:        "internal",
			Unmountable: false,
		}
		if c != 'C' {
			v.Kind = "external"
			v.Unmountable = false
		}
		out = append(out, v)
	}
	return out, nil
}
