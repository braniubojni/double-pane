// Package preview classifies a local file for the F3 quick-view panel.
package preview

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/erikharutyunyan/double-pane/internal/domain"
	"github.com/erikharutyunyan/double-pane/internal/filesystem"
	"github.com/erikharutyunyan/double-pane/internal/remote"
	"github.com/erikharutyunyan/double-pane/internal/vault"
)

const (
	sniffSize = 8 << 10
	textCap   = 256 << 10
	binaryCap = 20 << 20
)

const (
	msgLocal       = "Preview is only available for local files"
	msgVault       = "Preview is not available for vault files"
	msgSelect      = "Select a file"
	msgTooLarge    = "File is too large to preview"
	msgUnsupported = "This file type cannot be previewed"
)

type previewSource interface {
	io.Reader
	io.Closer
}

// openPreviewFile is replaced in tests to count bytes read.
var openPreviewFile = func(path string) (previewSource, os.FileInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, nil, err
	}
	return f, info, nil
}

// Read returns a preview for path. Unsupported paths return kind none and a
// message. An error means a local file should have been readable and was not.
func Read(path string) (domain.Preview, error) {
	if msg, ok := blocked(path); ok {
		return none(msg), nil
	}
	src, info, err := openPreviewFile(path)
	if err != nil {
		return domain.Preview{}, fmt.Errorf("read preview: %w", err)
	}
	defer func() { _ = src.Close() }()
	if info.IsDir() {
		return none(msgSelect), nil
	}
	if !info.Mode().IsRegular() {
		return none(msgUnsupported), nil
	}
	if insideFolderVault(path) {
		return none(msgVault), nil
	}
	return classify(src, info.Size(), path)
}

func blocked(path string) (string, bool) {
	if remote.IsRemote(path) || filesystem.IsTrashPath(path) || filesystem.IsArchivePath(path) {
		return msgLocal, true
	}
	if strings.EqualFold(filepath.Ext(path), ".dpenc") {
		return msgVault, true
	}
	return "", false
}

func insideFolderVault(path string) bool {
	dir := filepath.Dir(path)
	for {
		if _, ok := vault.Detect(dir); ok {
			return true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return false
		}
		dir = parent
	}
}

func classify(r io.Reader, size int64, path string) (domain.Preview, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp":
		return readCapped(r, size, binaryCap, imageMIME(path), "image")
	case ".pdf":
		return readCapped(r, size, binaryCap, "application/pdf", "pdf")
	case ".heic", ".heif", ".mp4", ".mov", ".mkv", ".webm", ".avi", ".mp3", ".wav", ".flac", ".aac", ".ogg", ".m4a":
		return none(msgUnsupported), nil
	default:
		return readText(r, size)
	}
}

func imageMIME(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	default:
		return "image/png"
	}
}

func readCapped(r io.Reader, size, cap int64, mime, kind string) (domain.Preview, error) {
	if size > cap {
		return none(msgTooLarge), nil
	}
	data, err := io.ReadAll(io.LimitReader(r, cap+1))
	if err != nil {
		return domain.Preview{}, fmt.Errorf("read preview: %w", err)
	}
	if int64(len(data)) > cap {
		return none(msgTooLarge), nil
	}
	return domain.Preview{Kind: kind, MIME: mime, Data: data}, nil
}

func readText(r io.Reader, size int64) (domain.Preview, error) {
	if size > textCap {
		buf, err := readSniff(r)
		if err != nil {
			return domain.Preview{}, err
		}
		if binarySniff(buf) {
			return none(msgUnsupported), nil
		}
		return none(msgTooLarge), nil
	}
	data, err := io.ReadAll(io.LimitReader(r, textCap+1))
	if err != nil {
		return domain.Preview{}, fmt.Errorf("read preview: %w", err)
	}
	if int64(len(data)) > textCap {
		return none(msgTooLarge), nil
	}
	if binarySniff(data) {
		return none(msgUnsupported), nil
	}
	return domain.Preview{Kind: "text", MIME: "text/plain", Text: string(data)}, nil
}

func readSniff(r io.Reader) ([]byte, error) {
	buf := make([]byte, sniffSize)
	n, err := io.ReadFull(r, buf)
	if err == io.EOF || err == io.ErrUnexpectedEOF {
		return buf[:n], nil
	}
	if err != nil {
		return nil, fmt.Errorf("read preview: %w", err)
	}
	return buf, nil
}

func binarySniff(b []byte) bool {
	return bytes.IndexByte(b, 0) >= 0 || !utf8.Valid(b)
}

func none(message string) domain.Preview {
	return domain.Preview{Kind: "none", Message: message}
}
