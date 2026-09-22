package service

import (
	"context"
	"fmt"

	"github.com/erikharutyunyan/double-pane/internal/filesystem"
)

func rejectArchiveWrite(path string) error {
	if filesystem.IsArchivePath(path) {
		return filesystem.ErrArchiveReadOnly
	}
	return nil
}

func rejectInsideArchive(paths ...string) error {
	for _, p := range paths {
		if filesystem.IsInsideArchive(p) {
			return filesystem.ErrArchiveReadOnly
		}
	}
	return nil
}

func rejectArchiveDest(destDir string) error {
	if filesystem.IsArchivePath(destDir) {
		return filesystem.ErrArchiveReadOnly
	}
	return nil
}

func countInsideArchive(paths []string) int {
	n := 0
	for _, p := range paths {
		if filesystem.IsInsideArchive(p) {
			n++
		}
	}
	return n
}

func (s *FileService) extractArchiveSources(ctx context.Context, sources []string, destDir string) error {
	groups := make(map[string][]string)
	for _, src := range sources {
		a, inner, ok := filesystem.SplitArchivePath(src)
		if !ok || inner == "" {
			return fmt.Errorf("not an archive member: %s", src)
		}
		groups[a] = append(groups[a], inner)
	}
	for a, inners := range groups {
		if err := filesystem.ExtractMembers(ctx, a, destDir, inners, s.archivePassword(a)); err != nil {
			return err
		}
	}
	return nil
}
