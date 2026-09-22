package ports

import (
	"sort"

	"github.com/erikharutyunyan/double-pane/internal/domain"
)

func sortListeners(out []domain.PortListener) {
	sort.Slice(out, func(i, j int) bool {
		if out[i].Port != out[j].Port {
			return out[i].Port < out[j].Port
		}
		return out[i].PID < out[j].PID
	})
}
