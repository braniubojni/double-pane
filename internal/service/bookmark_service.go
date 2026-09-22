package service

import (
	"path/filepath"
	"strings"

	"github.com/erikharutyunyan/double-pane/internal/domain"
	"github.com/erikharutyunyan/double-pane/internal/storage"
)

// BookmarkService manages saved directory shortcuts.
type BookmarkService struct {
	db *storage.DB
}

func NewBookmarkService(db *storage.DB) *BookmarkService {
	return &BookmarkService{db: db}
}

func (s *BookmarkService) List() ([]domain.Bookmark, error) {
	return s.db.ListBookmarks()
}

func (s *BookmarkService) Add(name, path string) (domain.Bookmark, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return domain.Bookmark{}, domainEmptyPath()
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = filepath.Base(path)
		if name == "" || name == "/" || name == "." {
			name = path
		}
	}
	return s.db.AddBookmark(name, path)
}

func (s *BookmarkService) Remove(id int64) error {
	return s.db.RemoveBookmark(id)
}

type simpleError string

func (e simpleError) Error() string { return string(e) }

func domainEmptyPath() error {
	return simpleError("path cannot be empty")
}
