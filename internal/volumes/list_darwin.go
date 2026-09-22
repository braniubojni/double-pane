//go:build darwin

package volumes

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/erikharutyunyan/double-pane/internal/domain"
)

func listOS() ([]domain.Volume, error) {
	ents, err := os.ReadDir("/Volumes")
	if err != nil {
		return nil, err
	}
	mounts := parseMountOutput(runCmd("mount"))
	images := parseHdiutilInfo(runCmd("hdiutil", "info", "-plist"))
	rootDevs := rootVolumePaths()

	var out []domain.Volume
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		p := filepath.Join("/Volumes", e.Name())
		fs := mounts[p].FS
		v := classify(p, fs, images, rootDevs)
		if mi, ok := mounts[p]; ok {
			v.Device = mi.Device
		}
		out = append(out, v)
	}
	return out, nil
}

func rootVolumePaths() map[string]struct{} {
	out := map[string]struct{}{"/": {}, "/System/Volumes/Data": {}}
	for _, p := range []string{"/", "/System/Volumes/Data"} {
		if eval, err := filepath.EvalSymlinks(p); err == nil {
			out[eval] = struct{}{}
		}
	}
	// /Volumes/Macintosh HD (and similar) often resolve to the data volume.
	ents, err := os.ReadDir("/Volumes")
	if err != nil {
		return out
	}
	rootInfo, err := os.Stat("/")
	if err != nil {
		return out
	}
	for _, e := range ents {
		p := filepath.Join("/Volumes", e.Name())
		info, err := os.Stat(p)
		if err != nil {
			continue
		}
		if os.SameFile(rootInfo, info) {
			out[p] = struct{}{}
		}
	}
	return out
}

func runCmd(name string, args ...string) string {
	cmd := exec.Command(name, args...)
	b, err := cmd.Output()
	if err != nil {
		return ""
	}
	return string(b)
}
