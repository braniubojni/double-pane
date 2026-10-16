package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/erikharutyunyan/double-pane/internal/domain"
	"github.com/erikharutyunyan/double-pane/internal/filesystem"
	"github.com/erikharutyunyan/double-pane/internal/remote"
	"github.com/erikharutyunyan/double-pane/internal/vault"
)

const (
	usageProgressGap  = 100 * time.Millisecond
	usagePayloadLimit = 2 << 20
	usageDeniedCap    = 100
)

// TreeUsage walks a local folder and emits usage:progress and one usage:done.
// jobID comes from NewJobID. The call returns once the walk is scheduled.
func (s *FileService) TreeUsage(jobID, root string, maxDepth int) error {
	if jobID == "" {
		return fmt.Errorf("jobID required")
	}
	if filesystem.IsTrashPath(root) {
		return fmt.Errorf("disk usage map is not available in trash")
	}
	if filesystem.IsArchivePath(root) {
		return fmt.Errorf("disk usage map is not available inside archives")
	}
	if remote.IsRemote(root) {
		return fmt.Errorf("disk usage map is not available on remote connections")
	}
	abs, err := filesystem.Resolve(root)
	if err != nil {
		return err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("not a directory: %s", abs)
	}
	if _, ok := vault.Detect(abs); ok {
		return fmt.Errorf("disk usage map is not available for a vault")
	}
	ctx := s.jobCtx(jobID)
	go s.runTreeUsage(ctx, jobID, abs, maxDepth)
	return nil
}

func (s *FileService) runTreeUsage(ctx context.Context, jobID, root string, maxDepth int) {
	defer func() { _ = s.FinishJob(jobID) }()

	lastEmit := time.Time{}
	tree, files, denied, err := filesystem.TreeUsage(ctx, root, maxDepth, func(path string) bool {
		_, ok := vault.Detect(path)
		return ok
	}, func(current string, nFiles int, nBytes int64) {
		now := time.Now()
		if !lastEmit.IsZero() && now.Sub(lastEmit) < usageProgressGap {
			return
		}
		lastEmit = now
		s.emit("usage:progress", domain.UsageProgress{
			JobID:   jobID,
			Current: current,
			Files:   nFiles,
			Bytes:   nBytes,
		})
	})
	if err != nil {
		s.emit("usage:done", domain.UsageDone{
			JobID:  jobID,
			Root:   root,
			Denied: []string{},
			Error:  err.Error(),
		})
		return
	}
	deniedCount := len(denied)
	if len(denied) > usageDeniedCap {
		denied = append([]string(nil), denied[:usageDeniedCap]...)
	}
	if denied == nil {
		denied = []string{}
	}
	done := domain.UsageDone{
		JobID:       jobID,
		Root:        root,
		Total:       tree.Size,
		Files:       files,
		Denied:      denied,
		DeniedCount: deniedCount,
		Tree:        tree,
	}
	fitUsagePayload(&done, usagePayloadLimit)
	s.emit("usage:done", done)
}

// fitUsagePayload drops kids on the deepest directories until the JSON payload
// fits in limit. Those directories stay as sized leaves.
func fitUsagePayload(done *domain.UsageDone, limit int) {
	for {
		raw, err := json.Marshal(done)
		if err != nil || len(raw) <= limit {
			return
		}
		if done.Tree == nil || !stripDeepestKids(done.Tree) {
			return
		}
		done.Truncated = true
	}
}

func stripDeepestKids(root *domain.UsageNode) bool {
	maxDepth := -1
	var mark func(*domain.UsageNode, int)
	mark = func(n *domain.UsageNode, depth int) {
		if len(n.Kids) > 0 && depth > maxDepth {
			maxDepth = depth
		}
		for i := range n.Kids {
			mark(&n.Kids[i], depth+1)
		}
	}
	mark(root, 0)
	if maxDepth < 0 {
		return false
	}
	var strip func(*domain.UsageNode, int)
	strip = func(n *domain.UsageNode, depth int) {
		if depth == maxDepth {
			n.Kids = nil
			return
		}
		for i := range n.Kids {
			strip(&n.Kids[i], depth+1)
		}
	}
	strip(root, 0)
	return true
}
