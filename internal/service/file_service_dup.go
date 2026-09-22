package service

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/erikharutyunyan/double-pane/internal/domain"
	"github.com/erikharutyunyan/double-pane/internal/filesystem"
	"github.com/erikharutyunyan/double-pane/internal/imghash"
	"github.com/erikharutyunyan/double-pane/internal/ocr"
	"github.com/erikharutyunyan/double-pane/internal/remote"
)

func dupLog(jobID, msg string, args ...any) {
	log.Printf("[dup] job=%s "+msg, append([]any{jobID}, args...)...)
}

func (s *FileService) tryStartDup(jobID string) error {
	s.dupMu.Lock()
	defer s.dupMu.Unlock()
	if s.dupJob != "" {
		return fmt.Errorf("a duplicates job is already running")
	}
	s.dupJob = jobID
	return nil
}

func (s *FileService) clearDupJob(jobID string) {
	s.dupMu.Lock()
	defer s.dupMu.Unlock()
	if s.dupJob == jobID {
		s.dupJob = ""
	}
}

func (s *FileService) dupRunning() bool {
	s.dupMu.Lock()
	defer s.dupMu.Unlock()
	return s.dupJob != ""
}

func (s *FileService) trashSkipRoots() []string {
	var roots []string
	if s != nil && s.trash != nil && s.trash.Root() != "" {
		roots = append(roots, s.trash.Root())
	}
	if s != nil && s.osTrash != nil {
		roots = append(roots, s.osTrash.Roots()...)
	}
	return roots
}

func (s *FileService) megaCacheDir() string {
	if s != nil && s.dupCacheDir != "" {
		return s.dupCacheDir
	}
	return ""
}

// migrateDupCache moves an old trash/dup-cache tree into cfgDir/dup-cache once.
func migrateDupCache(oldDir, newDir string) {
	if oldDir == "" || newDir == "" || oldDir == newDir {
		return
	}
	st, err := os.Stat(oldDir)
	if err != nil || !st.IsDir() {
		return
	}
	if err := os.MkdirAll(filepath.Dir(newDir), 0o700); err != nil {
		return
	}
	if _, err := os.Stat(newDir); os.IsNotExist(err) {
		if err := os.Rename(oldDir, newDir); err == nil {
			return
		}
	}
	if err := os.MkdirAll(newDir, 0o700); err != nil {
		return
	}
	entries, err := os.ReadDir(oldDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		src := filepath.Join(oldDir, e.Name())
		dst := filepath.Join(newDir, e.Name())
		if _, err := os.Stat(dst); err == nil {
			continue
		}
		if err := os.Rename(src, dst); err != nil {
			if cpErr := copyDupCacheEntry(src, dst); cpErr != nil {
				continue
			}
			_ = os.RemoveAll(src)
		}
	}
	_ = os.RemoveAll(oldDir)
}

func copyDupCacheEntry(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("unexpected dir in dup-cache: %s", src)
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

// purgeDupCacheDir removes entries under dir. maxAge <= 0 deletes all;
// otherwise only entries with mtime older than maxAge.
func purgeDupCacheDir(dir string, maxAge time.Duration) error {
	if dir == "" {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var cutoff time.Time
	if maxAge > 0 {
		cutoff = time.Now().Add(-maxAge)
	}
	var firstErr error
	for _, e := range entries {
		path := filepath.Join(dir, e.Name())
		if maxAge > 0 {
			info, err := e.Info()
			if err != nil || info.ModTime().After(cutoff) {
				continue
			}
		}
		if err := os.RemoveAll(path); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (s *FileService) clearDupCache() {
	_ = purgeDupCacheDir(s.megaCacheDir(), 0)
}

func (s *FileService) validateDupRoot(root string) error {
	if strings.TrimSpace(root) == "" {
		return fmt.Errorf("root path is required")
	}
	if !remote.IsRemote(root) && filesystem.IsArchivePath(root) {
		return fmt.Errorf("duplicate scan is not available inside archives")
	}
	if _, err := s.ListDir(root, true); err != nil {
		return err
	}
	return nil
}

func (s *FileService) listForDup(root string) (listDirFunc, error) {
	if s.listDup != nil {
		return s.listDup, nil
	}
	if remote.IsRemote(root) {
		be, err := s.backendFor(root)
		if err != nil {
			return nil, err
		}
		return be.ListDir, nil
	}
	return filesystem.ListDir, nil
}

func (s *FileService) hashPath(ctx context.Context, jobID, path string) (string, error) {
	if s.hashFile != nil {
		return s.hashFile(ctx, path)
	}
	ctx = remote.WithDupJobID(ctx, jobID)
	switch remote.SchemeOf(path) {
	case "mega":
		return s.mega.HashFile(ctx, path, s.megaCacheDir())
	case "smb", "ssh":
		be, err := s.backendFor(path)
		if err != nil {
			return "", err
		}
		r, err := be.OpenRead(path)
		if err != nil {
			return "", err
		}
		return filesystem.HashReader(ctx, r)
	default:
		return filesystem.HashFile(ctx, path)
	}
}

func (s *FileService) dhashPath(ctx context.Context, jobID, path string) (uint64, error) {
	if s.dhashFile != nil {
		return s.dhashFile(ctx, path)
	}
	ctx = remote.WithDupJobID(ctx, jobID)
	switch remote.SchemeOf(path) {
	case "mega":
		if s.mega == nil {
			return 0, fmt.Errorf("remote not available")
		}
		var h uint64
		err := s.mega.WithTempFile(ctx, path, s.megaCacheDir(), func(p string) error {
			var e error
			h, e = imghash.HashFile(ctx, p)
			return e
		})
		return h, err
	case "smb", "ssh":
		be, err := s.backendFor(path)
		if err != nil {
			return 0, err
		}
		r, err := be.OpenRead(path)
		if err != nil {
			return 0, err
		}
		return imghash.HashReader(ctx, r)
	default:
		return imghash.HashFile(ctx, path)
	}
}

func (s *FileService) ocrPath(ctx context.Context, jobID, path string) (string, error) {
	if s.ocrFile != nil {
		return s.ocrFile(ctx, path)
	}
	ctx = remote.WithDupJobID(ctx, jobID)
	switch remote.SchemeOf(path) {
	case "mega":
		if s.mega == nil {
			return "", fmt.Errorf("remote not available")
		}
		var text string
		err := s.mega.WithTempFile(ctx, path, s.megaCacheDir(), func(p string) error {
			var e error
			text, e = ocr.Recognize(ctx, p)
			return e
		})
		return text, err
	case "smb", "ssh":
		be, err := s.backendFor(path)
		if err != nil {
			return "", err
		}
		r, err := be.OpenRead(path)
		if err != nil {
			return "", err
		}
		defer func() { _ = r.Close() }()
		cache := s.megaCacheDir()
		if cache == "" {
			return "", fmt.Errorf("dup cache dir required")
		}
		if err := os.MkdirAll(cache, 0o700); err != nil {
			return "", err
		}
		f, err := os.CreateTemp(cache, "ocr-*")
		if err != nil {
			return "", err
		}
		tmp := f.Name()
		defer func() { _ = os.Remove(tmp) }()
		if _, err := io.Copy(f, r); err != nil {
			_ = f.Close()
			return "", err
		}
		if err := f.Close(); err != nil {
			return "", err
		}
		return ocr.Recognize(ctx, tmp)
	default:
		return ocr.Recognize(ctx, path)
	}
}

// OCRAvailable reports whether the system tesseract binary is on PATH.
func (s *FileService) OCRAvailable() bool {
	return ocr.Available()
}

// EstimateDuplicateScan counts files and bytes under root without hashing.
func (s *FileService) EstimateDuplicateScan(root string, includeHidden bool, minSize int64, exclude string, similarImages bool, similarityPct int, ocrOn bool) (domain.ScanEstimate, error) {
	if s.dupRunning() {
		return domain.ScanEstimate{}, fmt.Errorf("a duplicates job is already running")
	}
	if err := s.validateDupRoot(root); err != nil {
		return domain.ScanEstimate{}, err
	}
	list, err := s.listForDup(root)
	if err != nil {
		return domain.ScanEstimate{}, err
	}
	var trash []string
	if !remote.IsRemote(root) {
		trash = s.trashSkipRoots()
	}
	files, _, err := walkForDuplicates(context.Background(), list, root, includeHidden, minSize, trash, exclude)
	if err != nil {
		return domain.ScanEstimate{}, err
	}
	est := estimateFromFiles(files, scanProtocol(root), similarImages, ocrOn)
	return est, nil
}

func estimateFromFiles(files []dupFile, proto string, similarImages, ocrOn bool) domain.ScanEstimate {
	var bytes, imageCount, imageBytes, sizeTieBytes int64
	sizeN := map[int64]int{}
	for _, f := range files {
		bytes += f.Size
		sizeN[f.Size]++
		if imghash.IsVisualName(f.Name) && f.Size <= imghash.MaxBytes {
			imageCount++
			imageBytes += f.Size
		}
	}
	seen := map[string]struct{}{}
	for _, f := range files {
		if sizeN[f.Size] < 2 {
			continue
		}
		sizeTieBytes += f.Size
		seen[f.Path] = struct{}{}
	}
	megaBytes := sizeTieBytes
	if similarImages || ocrOn {
		for _, f := range files {
			if _, ok := seen[f.Path]; ok {
				continue
			}
			if imghash.IsVisualName(f.Name) && f.Size <= imghash.MaxBytes {
				megaBytes += f.Size
			}
		}
	}
	etaExact := etaSeconds(bytes, proto)
	est := domain.ScanEstimate{
		FileCount:         int64(len(files)),
		ByteCount:         bytes,
		EtaSeconds:        etaExact,
		Protocol:          proto,
		MegaDownload:      proto == "mega",
		ImageCount:        imageCount,
		EtaExactSeconds:   etaExact,
		EtaOcrSeconds:     0,
		MegaDownloadBytes: megaBytes,
	}
	if similarImages {
		est.EtaVisualSeconds = etaSeconds(imageBytes, proto)
	}
	if ocrOn {
		est.EtaOcrSeconds = etaSeconds(imageBytes, proto) + int(imageCount)
	}
	return est
}

// StartDuplicateScan runs a cancellable SHA-256 duplicate scan in the background.
func (s *FileService) StartDuplicateScan(jobID, root string, includeHidden bool, minSize int64, exclude string, similarImages bool, similarityPct int, ocrOn bool) error {
	if jobID == "" {
		return fmt.Errorf("jobID required")
	}
	if err := s.validateDupRoot(root); err != nil {
		return err
	}
	if err := s.tryStartDup(jobID); err != nil {
		return err
	}
	ctx := remote.WithDupJobID(s.storeJob(jobID), jobID)
	dupLog(jobID, "start root=%q hidden=%v minSize=%d exclude=%q similar=%v pct=%d ocr=%v", root, includeHidden, minSize, exclude, similarImages, similarityPct, ocrOn)
	go s.runDuplicateScan(ctx, jobID, root, includeHidden, minSize, exclude, similarImages, similarityPct, ocrOn)
	return nil
}

func (s *FileService) runDuplicateScan(ctx context.Context, jobID, root string, includeHidden bool, minSize int64, exclude string, similarImages bool, similarityPct int, ocrOn bool) {
	defer func() { _ = s.FinishJob(jobID) }()
	defer s.clearDupJob(jobID)
	defer s.clearDupCache() // leftover temps after success/cancel/fatal — delete now, not 24h later

	var collected []domain.DuplicateGroup
	finish := func(path string, err error) {
		s.clearDupCache() // finished job = delete now (before done so UI/tests see a clean cache)
		msg := ""
		if err != nil {
			msg = err.Error()
			if ctx.Err() != nil {
				dupLog(jobID, "cancel received")
			}
			dupLog(jobID, "fatal path=%s err=%s", path, msg)
			s.emit("dup:error", domain.DupErrorPayload{
				JobID:   jobID,
				Path:    path,
				Message: msg,
				Fatal:   true,
			})
		} else {
			dupLog(jobID, "done groups=%d", len(collected))
		}
		s.emit("dup:done", domain.DupDonePayload{JobID: jobID, Error: msg, Groups: collected})
	}

	if err := ctx.Err(); err != nil {
		finish(root, err)
		return
	}
	list, err := s.listForDup(root)
	if err != nil {
		finish(root, err)
		return
	}
	var trash []string
	if !remote.IsRemote(root) {
		trash = s.trashSkipRoots()
	}
	files, skipped, err := walkForDuplicates(ctx, list, root, includeHidden, minSize, trash, exclude)
	if err != nil {
		finish(root, err)
		return
	}

	proto := scanProtocol(root)
	var totalBytes int64
	for _, f := range files {
		totalBytes += f.Size
	}
	totalFiles := int64(len(files))
	groups := 0
	skipN := len(skipped)
	var doneFiles, doneBytes int64
	phase := "walk"
	var doneImages, totalImages int64
	progress := func(path string) {
		s.emit("dup:progress", domain.DupProgressPayload{
			JobID:       jobID,
			DoneFiles:   doneFiles,
			TotalFiles:  totalFiles,
			DoneBytes:   doneBytes,
			TotalBytes:  totalBytes,
			Groups:      groups,
			Skipped:     skipN,
			CurrentPath: path,
			Phase:       phase,
			DoneImages:  doneImages,
			TotalImages: totalImages,
		})
	}
	dupLog(jobID, "progress totals files=%d bytes=%d", totalFiles, totalBytes)
	progress(root)
	for _, sk := range skipped {
		s.emit("dup:error", domain.DupErrorPayload{
			JobID:   jobID,
			Path:    sk.Path,
			Message: sk.Reason,
			Fatal:   false,
		})
	}

	buckets := map[int64][]dupFile{}
	for _, f := range files {
		buckets[f.Size] = append(buckets[f.Size], f)
	}
	var toHash [][]dupFile
	for _, bucket := range buckets {
		if len(bucket) < 2 {
			for _, f := range bucket {
				doneFiles++
				doneBytes += f.Size
			}
			continue
		}
		toHash = append(toHash, bucket)
	}
	lastEmit := time.Time{}
	tick := func(path string, force bool) {
		now := time.Now()
		if !force && !lastEmit.IsZero() && now.Sub(lastEmit) < 100*time.Millisecond {
			return
		}
		lastEmit = now
		progress(path)
	}
	phase = "exact"
	tick(root, true)

	for _, bucket := range toHash {
		if err := ctx.Err(); err != nil {
			finish(root, err)
			return
		}
		byHash := map[string][]dupFile{}
		for _, f := range bucket {
			if err := ctx.Err(); err != nil {
				finish(f.Path, err)
				return
			}
			sum, herr := s.hashPath(ctx, jobID, f.Path)
			if herr != nil {
				if ctx.Err() != nil || isDupFatalErr(herr) {
					finish(f.Path, herr)
					return
				}
				skipN++
				s.emit("dup:error", domain.DupErrorPayload{
					JobID:   jobID,
					Path:    f.Path,
					Message: herr.Error(),
					Fatal:   false,
				})
				doneFiles++
				doneBytes += f.Size
				tick(f.Path, false)
				continue
			}
			byHash[sum] = append(byHash[sum], f)
			doneFiles++
			doneBytes += f.Size
			tick(f.Path, false)
		}
		for h, members := range byHash {
			if len(members) < 2 {
				continue
			}
			sort.Slice(members, func(i, j int) bool { return members[i].Path < members[j].Path })
			g := domain.DuplicateGroup{Hash: h, Size: members[0].Size, Kind: "exact"}
			for _, m := range members {
				g.Files = append(g.Files, domain.DuplicateFile{
					Path:     m.Path,
					Name:     m.Name,
					Size:     m.Size,
					ModTime:  m.ModTime,
					Protocol: proto,
				})
			}
			groups++
			collected = append(collected, g)
			tick(members[0].Path, false)
		}
	}
	tick(root, true)
	if similarImages {
		phase = "visual"
		if err := s.runVisualPass(ctx, jobID, proto, files, &collected, &groups, &skipN, &doneImages, &totalImages, similarityPct, tick); err != nil {
			finish(root, err)
			return
		}
		tick(root, true)
	}
	if ocrOn {
		phase = "ocr"
		if err := s.runOCRPass(ctx, jobID, proto, files, &collected, &groups, &skipN, &doneImages, &totalImages, similarityPct, tick); err != nil {
			finish(root, err)
			return
		}
		tick(root, true)
	}
	finish("", nil)
}
