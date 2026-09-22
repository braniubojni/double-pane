package clipboard

import (
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/erikharutyunyan/double-pane/internal/filesystem"
)

// ErrEmpty means the clipboard has no file list and no image.
var ErrEmpty = errors.New("nothing to paste")

// PasteInto copies clipboard files into dest, or writes a PNG screenshot.
func PasteInto(dest string) error {
	files := Files()
	png := PNG()
	log.Printf("clipboard paste dest=%s files=%d png=%d", dest, len(files), len(png))
	return apply(dest, files, png, time.Now())
}

func apply(dest string, files []string, png []byte, now time.Time) error {
	if len(files) > 0 {
		return filesystem.Copy(files, dest)
	}
	if len(png) == 0 {
		return ErrEmpty
	}
	name := fmt.Sprintf("clipboard-%s.png", now.Format("20060102150405"))
	path := filesystem.UniquePath(filepath.Join(dest, name))
	return os.WriteFile(path, png, 0o644)
}

func parseURIList(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "file://") {
			u, err := url.Parse(line)
			if err != nil || u.Path == "" {
				continue
			}
			out = append(out, u.Path)
			continue
		}
		if strings.HasPrefix(line, "/") || (len(line) > 1 && line[1] == ':') {
			out = append(out, line)
		}
	}
	return out
}
