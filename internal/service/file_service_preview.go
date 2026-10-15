package service

import (
	"github.com/erikharutyunyan/double-pane/internal/domain"
	"github.com/erikharutyunyan/double-pane/internal/preview"
)

// ReadPreview returns a local quick-view payload. Unsupported paths come back
// as kind none. It does not download, decrypt, or write.
func (s *FileService) ReadPreview(path string) (domain.Preview, error) {
	return preview.Read(path)
}
