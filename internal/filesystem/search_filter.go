package filesystem

import (
	"io"
	"os"

	"github.com/erikharutyunyan/double-pane/internal/domain"
	"github.com/erikharutyunyan/double-pane/internal/imghash"
)

// SearchCandidateAllowed reports whether a walk entry may become a hit.
// Directories ignore the size bounds. Type "text" still needs the content sniff.
func SearchCandidateAllowed(f domain.SearchFilter, name string, isDir bool, size, modTime int64) bool {
	if !isDir {
		if size < f.MinSize || (f.MaxSize > 0 && size > f.MaxSize) {
			return false
		}
	}
	if f.ModifiedAfter > 0 && modTime < f.ModifiedAfter {
		return false
	}
	if f.ModifiedBefore > 0 && modTime > f.ModifiedBefore {
		return false
	}
	switch f.Type {
	case "file":
		return !isDir
	case "dir":
		return isDir
	case "image":
		return !isDir && imghash.IsVisualName(name)
	case "archive":
		return !isDir && IsBrowsableArchiveName(name)
	case "text":
		return !isDir
	default:
		return true
	}
}

func prefixIsBinary(data []byte) bool {
	if len(data) > contentBinarySniffBytes {
		data = data[:contentBinarySniffBytes]
	}
	for _, b := range data {
		if b == 0 {
			return true
		}
	}
	return false
}

// ReaderIsText is the content-search binary sniff: no NUL in the first window.
func ReaderIsText(r io.Reader) bool {
	buf := make([]byte, contentBinarySniffBytes)
	n, _ := r.Read(buf)
	return !prefixIsBinary(buf[:n])
}

func fileIsText(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer func() { _ = f.Close() }()
	return ReaderIsText(f), nil
}
