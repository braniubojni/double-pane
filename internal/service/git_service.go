package service

import (
	"context"

	"github.com/erikharutyunyan/double-pane/internal/domain"
	"github.com/erikharutyunyan/double-pane/internal/filesystem"
	"github.com/erikharutyunyan/double-pane/internal/gitstatus"
	"github.com/erikharutyunyan/double-pane/internal/remote"
)

// GitService exposes cheap, cached git working-tree status for the file list.
type GitService struct {
	cache *gitstatus.Cache
}

func NewGitService() *GitService {
	return &GitService{cache: gitstatus.NewCache()}
}

// StatusForDir returns git status for immediate children of path.
// Remote paths and non-repos return an empty result (no error).
func (s *GitService) StatusForDir(path string) (domain.GitDirStatus, error) {
	if path == "" || remote.IsRemote(path) || filesystem.IsArchivePath(path) {
		return domain.GitDirStatus{}, nil
	}
	st, err := s.cache.StatusForDir(context.Background(), path)
	if err != nil {
		return domain.GitDirStatus{}, nil
	}
	out := domain.GitDirStatus{
		RepoRoot: st.RepoRoot,
		Entries:  make([]domain.GitStatusEntry, 0, len(st.Entries)),
	}
	for _, e := range st.Entries {
		out.Entries = append(out.Entries, domain.GitStatusEntry{Name: e.Name, Status: e.Status})
	}
	return out, nil
}

// FileDiff returns HEAD vs working-tree text for a local file (for the built-in diff viewer).
// Soft-fails with Message set; remote paths return an empty result.
func (s *GitService) FileDiff(path string) (domain.GitFileDiff, error) {
	if path == "" || remote.IsRemote(path) {
		return domain.GitFileDiff{Path: path, Message: "remote paths are not supported"}, nil
	}
	if filesystem.IsArchivePath(path) {
		return domain.GitFileDiff{Path: path, Message: "archive paths are not supported"}, nil
	}
	d := s.cache.FileDiff(context.Background(), path)
	return domain.GitFileDiff{
		Path:      d.Path,
		RepoRoot:  d.RepoRoot,
		RelPath:   d.RelPath,
		Status:    d.Status,
		OldText:   d.OldText,
		NewText:   d.NewText,
		Binary:    d.Binary,
		Truncated: d.Truncated,
		Message:   d.Message,
	}, nil
}
