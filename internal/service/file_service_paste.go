package service

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/erikharutyunyan/double-pane/internal/clipboard"
	"github.com/erikharutyunyan/double-pane/internal/remote"
)

const (
	pasteLocal     = "local"
	pasteUpload    = "upload"
	pasteUploadPNG = "upload-png"
)

var errClipboardRemote = errors.New("clipboard has a remote path; paste local files or images")

// PasteClipboard copies OS clipboard files into dest, or writes a PNG image.
// A remote dest uploads local clipboard files (or a temp PNG) as one transfer job.
func (s *FileService) PasteClipboard(dest string) error {
	if err := rejectArchiveWrite(dest); err != nil {
		return err
	}
	if err := s.rejectTrashDest(dest); err != nil {
		return err
	}
	if !remote.IsRemote(dest) {
		log.Printf("PasteClipboard dest=%s", dest)
		err := clipboard.PasteInto(dest)
		if err != nil {
			log.Printf("PasteClipboard: %v", err)
		}
		return err
	}
	files := clipboard.Files()
	png := clipboard.PNG()
	err := s.pasteRemote(dest, files, png, time.Now())
	if err != nil {
		log.Printf("PasteClipboard: %v", err)
	}
	return err
}

// pasteMode decides how to paste. Local dest stays on clipboard.PasteInto.
func pasteMode(dest string, files []string, png []byte) (string, error) {
	if err := rejectArchiveWrite(dest); err != nil {
		return "", err
	}
	if err := rejectTrashWrite(dest); err != nil {
		return "", err
	}
	if remote.IsRemote(dest) {
		for _, f := range files {
			if remote.IsRemote(f) {
				return "", errClipboardRemote
			}
		}
		if len(files) > 0 {
			return pasteUpload, nil
		}
		if len(png) > 0 {
			return pasteUploadPNG, nil
		}
		return "", clipboard.ErrEmpty
	}
	if len(files) == 0 && len(png) == 0 {
		return "", clipboard.ErrEmpty
	}
	return pasteLocal, nil
}

func (s *FileService) pasteRemote(dest string, files []string, png []byte, now time.Time) error {
	mode, err := pasteMode(dest, files, png)
	if err != nil {
		return err
	}
	if mode != pasteUpload && mode != pasteUploadPNG {
		return fmt.Errorf("paste mode %s is not a remote upload", mode)
	}
	jobID := s.NewJobID()
	sources := files
	if mode == pasteUploadPNG {
		path, werr := writePastePNG(png, now)
		if werr != nil {
			_ = s.FinishJob(jobID)
			return werr
		}
		defer removePastePNG(path)
		sources = []string{path}
	}
	if s.pasteUpload != nil {
		defer func() { _ = s.FinishJob(jobID) }()
		ctx := s.jobCtx(jobID)
		label := transferLabel("copy", sources, dest)
		return s.pasteUpload(ctx, sources, dest, s.transferProgress(jobID, "copy", label, dest))
	}
	return s.runTransfer(jobID, "copy", sources, dest, false)
}

func writePastePNG(png []byte, now time.Time) (string, error) {
	dir, err := os.MkdirTemp("", "dp-paste-")
	if err != nil {
		return "", err
	}
	name := fmt.Sprintf("clipboard-%s.png", now.Format("20060102150405"))
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, png, 0o644); err != nil {
		_ = os.RemoveAll(dir)
		return "", err
	}
	return path, nil
}

func removePastePNG(path string) {
	if path == "" {
		return
	}
	_ = os.RemoveAll(filepath.Dir(path))
}
