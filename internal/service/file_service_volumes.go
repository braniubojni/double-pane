package service

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/erikharutyunyan/double-pane/internal/domain"
)

func (s *FileService) ListVolumes() ([]domain.Volume, error) {
	if s.vols == nil {
		return nil, nil
	}
	return s.vols.List()
}

func (s *FileService) UnmountVolume(path string) error {
	if s.vols == nil {
		return fmt.Errorf("volumes not available")
	}
	if err := s.vols.Unmount(path); err != nil {
		return err
	}
	s.emit("volumes:changed", map[string]any{})
	return nil
}

// AttachDiskImage mounts a local disk image (macOS) and returns the mount point.
// jobID from NewJobID enables CancelJob and transfer:progress events; empty is fire-and-forget.
// password is used for encrypted images (hdiutil -stdinpass).
func (s *FileService) AttachDiskImage(jobID, path, password string) (mp string, errOut error) {
	defer func() { _ = s.FinishJob(jobID) }()
	if s.vols == nil {
		return "", fmt.Errorf("volumes not available")
	}
	ctx := s.jobCtx(jobID)
	if err := s.acquireTransfer(ctx); err != nil {
		return "", err
	}
	defer s.releaseTransfer()

	label := fmt.Sprintf("Attach %s", filepath.Base(path))
	var size int64
	if st, err := os.Stat(path); err == nil {
		size = st.Size()
	}
	emitDone := func(e error) {
		if jobID == "" {
			return
		}
		msg := ""
		if e != nil {
			msg = e.Error()
		}
		s.emit("transfer:done", domain.TransferDonePayload{JobID: jobID, Kind: "attach", Error: msg})
	}
	defer func() { emitDone(errOut) }()

	s.emitAttachProgress(jobID, label, path, 0, size)
	mp, err := s.vols.AttachDiskImage(ctx, path, password, func(pct float64) {
		if pct < 0 {
			s.emitAttachProgress(jobID, label, path, 0, 0)
			return
		}
		done := int64(pct / 100 * float64(size))
		total := size
		if total <= 0 {
			done = int64(pct)
			total = 100
		}
		s.emitAttachProgress(jobID, label, path, done, total)
	})
	if err != nil {
		return "", err
	}
	s.emit("volumes:changed", map[string]any{})
	return mp, nil
}

func (s *FileService) emitAttachProgress(jobID, label, path string, done, total int64) {
	if jobID == "" {
		return
	}
	s.emit("transfer:progress", domain.TransferProgressPayload{
		JobID:       jobID,
		Kind:        "attach",
		BytesDone:   done,
		BytesTotal:  total,
		CurrentPath: path,
		Label:       label,
		DestDir:     "",
	})
}

// IsEncryptedDiskImage reports whether a local disk image needs a passphrase.
func (s *FileService) IsEncryptedDiskImage(path string) (bool, error) {
	if s.vols == nil {
		return false, nil
	}
	return s.vols.IsEncryptedDiskImage(path)
}

func (s *FileService) rewriteDMGParent(path string, entries []domain.FileEntry) {
	if s.vols == nil || len(entries) == 0 {
		return
	}
	parent := s.vols.ParentOverride(path)
	if parent == "" {
		return
	}
	for i := range entries {
		if entries[i].Name == ".." {
			entries[i].Path = parent
		}
	}
}
