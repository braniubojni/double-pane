package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	gopath "path"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/erikharutyunyan/go-file-manager/internal/clipboard"
	"github.com/erikharutyunyan/go-file-manager/internal/config"
	"github.com/erikharutyunyan/go-file-manager/internal/domain"
	"github.com/erikharutyunyan/go-file-manager/internal/filesystem"
	"github.com/erikharutyunyan/go-file-manager/internal/remote"
	"github.com/erikharutyunyan/go-file-manager/internal/volumes"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// TrashMaxAge is how long duplicate-scan temps stay on disk.
const TrashMaxAge = 24 * time.Hour

// maxConcurrentTransfers caps overlapping copy/move/attach jobs.
const maxConcurrentTransfers = 2

// FileService exposes filesystem operations to the frontend.
type FileService struct {
	jobs         sync.Map // jobID -> *jobHandle
	jobSeq       atomic.Uint64
	transferGate chan struct{}
	remote       *remote.Manager
	smb          *remote.SMBManager
	mega         *remote.MEGAManager
	trash        *filesystem.Trash
	osTrash      filesystem.TrashBackend
	app          *application.App
	vols         *volumes.Manager
	archivePw    sync.Map // archive abs path -> password, cached for the session
	dupMu        sync.Mutex
	dupJob       string
	dupCacheDir  string
	onEvent      func(name string, data any)
	hashFile     func(context.Context, string) (string, error)
	dhashFile    func(context.Context, string) (uint64, error)
	ocrFile      func(context.Context, string) (string, error)
	listDup      listDirFunc
}

type remoteBackend interface {
	ListDir(path string, showHidden bool) ([]domain.FileEntry, error)
	ListPathCompletions(partial string) ([]string, error)
	Exists(path string) (bool, error)
	CopyWithin(sources []string, destDir string) error
	MoveWithin(sources []string, destDir string) error
	Download(sources []string, destDir string) error
	Upload(sources []string, destDir string) error
	Delete(paths []string) error
	Rename(oldPath, newName string) (string, error)
	Mkdir(parent, name string) (string, error)
	ReadTextFile(path string) (string, error)
	WriteTextFile(path, content string) error
	DirChildSizesCtx(ctx context.Context, dir string) (domain.DirSizes, error)
	OpenRead(path string) (io.ReadCloser, error)
}

func NewFileService(remoteMgr *remote.Manager, smbMgr *remote.SMBManager, megaMgr *remote.MEGAManager, trashDir string) *FileService {
	// dup-cache lives next to trash/ under the config dir, not inside trash/.
	cfgDir := filepath.Dir(trashDir)
	dupCache := filepath.Join(cfgDir, "dup-cache")
	migrateDupCache(filepath.Join(trashDir, "dup-cache"), dupCache)
	return &FileService{
		transferGate: make(chan struct{}, maxConcurrentTransfers),
		remote:       remoteMgr,
		smb:          smbMgr,
		mega:         megaMgr,
		trash:        filesystem.NewTrash(trashDir),
		osTrash:      filesystem.NewXDGTrash(filepath.Join(cfgDir, "Trash")),
		vols:         volumes.NewManager(),
		dupCacheDir:  dupCache,
	}
}

// SetTrashBackend replaces the OS trash implementation (production vs isolated XDG).
//
//wails:ignore
func (s *FileService) SetTrashBackend(b filesystem.TrashBackend) {
	if s != nil && b != nil {
		s.osTrash = b
	}
}

func (s *FileService) acquireTransfer(ctx context.Context) error {
	if s == nil || s.transferGate == nil {
		return nil
	}
	select {
	case s.transferGate <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *FileService) releaseTransfer() {
	if s == nil || s.transferGate == nil {
		return
	}
	select {
	case <-s.transferGate:
	default:
	}
}

func (s *FileService) backendFor(path string) (remoteBackend, error) {
	if remote.IsMEGA(path) {
		if s.mega == nil {
			return nil, fmt.Errorf("remote not available")
		}
		return s.mega, nil
	}
	if remote.IsSMB(path) {
		if s.smb == nil {
			return nil, fmt.Errorf("remote not available")
		}
		return s.smb, nil
	}
	if remote.IsRemote(path) {
		if s.remote == nil {
			return nil, fmt.Errorf("remote not available")
		}
		return s.remote, nil
	}
	return nil, fmt.Errorf("not a remote path")
}

// PurgeTrash is a no-op: leftover app-trash batches stay until restore or EmptyTrash.
func (s *FileService) PurgeTrash() error {
	return nil
}

// PurgeDupCache drops stale duplicate-scan temps older than TrashMaxAge.
// Called at startup; does not touch trash/ or app.db.
//
//wails:ignore
func (s *FileService) PurgeDupCache() error {
	return purgeDupCacheDir(s.megaCacheDir(), TrashMaxAge)
}

// SetApp injects the application for search event emission (call after application.New).
// Backend wiring only — not part of the frontend IPC surface.
//
//wails:ignore
func (s *FileService) SetApp(app *application.App) {
	s.app = app
	if s.vols != nil {
		s.vols.StartWatch(func() { s.emit("volumes:changed", map[string]any{}) })
	}
}

func (s *FileService) emit(name string, data any) {
	if s != nil && s.onEvent != nil {
		s.onEvent(name, data)
	}
	if s != nil && s.app != nil {
		s.app.Event.Emit(name, data)
	}
}

type jobHandle struct {
	ctx        context.Context
	cancel     context.CancelFunc
	fileCancel *filesystem.FileCancelRegistry
}

// NewJobID allocates a cancellable job context and returns its id.
func (s *FileService) NewJobID() string {
	id := fmt.Sprintf("job-%d", s.jobSeq.Add(1))
	s.storeJob(id)
	return id
}

// storeJob puts id in the CancelJob map, or returns the existing ctx.
func (s *FileService) storeJob(id string) context.Context {
	if id == "" {
		return context.Background()
	}
	if v, ok := s.jobs.Load(id); ok {
		return v.(*jobHandle).ctx
	}
	reg := filesystem.NewFileCancelRegistry()
	ctx, cancel := context.WithCancel(filesystem.WithFileCancelRegistry(context.Background(), reg))
	s.jobs.Store(id, &jobHandle{ctx: ctx, cancel: cancel, fileCancel: reg})
	return ctx
}

func (s *FileService) jobCtx(jobID string) context.Context {
	if jobID == "" {
		return context.Background()
	}
	if v, ok := s.jobs.Load(jobID); ok {
		return v.(*jobHandle).ctx
	}
	return context.Background()
}

// CancelJob cancels a long-running Archive/Extract/DirChildSizes/Copy/Move/Attach started with NewJobID.
func (s *FileService) CancelJob(jobID string) error {
	if jobID == "" {
		return nil
	}
	if v, ok := s.jobs.LoadAndDelete(jobID); ok {
		v.(*jobHandle).cancel()
	}
	return nil
}

// CancelTransferFile cancels one source path within a running Copy/Move job
// without cancelling the rest of it (the "Cancel All" job-wide cancel is
// CancelJob). Currently only takes effect for local (non-remote) transfers —
// remote SMB/SFTP jobs still support job-wide cancel only.
func (s *FileService) CancelTransferFile(jobID, path string) error {
	if jobID == "" || path == "" {
		return nil
	}
	if v, ok := s.jobs.Load(jobID); ok {
		if abs, err := filesystem.Resolve(path); err == nil {
			v.(*jobHandle).fileCancel.Cancel(abs)
		}
	}
	return nil
}

// FinishJob releases a completed job (no-op if already cancelled).
func (s *FileService) FinishJob(jobID string) error {
	if jobID == "" {
		return nil
	}
	if v, ok := s.jobs.LoadAndDelete(jobID); ok {
		v.(*jobHandle).cancel()
	}
	return nil
}

func (s *FileService) ListDir(path string, showHidden bool) ([]domain.FileEntry, error) {
	if filesystem.IsTrashPath(path) {
		home, _ := filesystem.HomeDir()
		return filesystem.ListTrashEntries(s.osTrash, s.trash, home)
	}
	if remote.IsRemote(path) {
		be, err := s.backendFor(path)
		if err != nil {
			return nil, err
		}
		return be.ListDir(path, showHidden)
	}
	if a, inner, ok := filesystem.SplitArchivePath(path); ok {
		return filesystem.ListArchiveDir(a, inner, showHidden)
	}
	entries, err := filesystem.ListDir(path, showHidden)
	if err != nil {
		return nil, err
	}
	s.rewriteDMGParent(path, entries)
	return entries, nil
}

func (s *FileService) ListPathCompletions(partial string) ([]string, error) {
	if remote.IsRemote(partial) {
		be, err := s.backendFor(partial)
		if err != nil {
			return nil, err
		}
		return be.ListPathCompletions(partial)
	}
	return filesystem.ListPathCompletions(partial)
}

func (s *FileService) GetHomeDir() (string, error) {
	return filesystem.HomeDir()
}

// ICloudDrivePath returns the macOS iCloud Drive folder, or "" if it is not present.
func (s *FileService) ICloudDrivePath() (string, error) {
	return filesystem.ICloudDrivePath()
}

// GoogleDrivePaths returns local Google Drive for desktop folders, or nil if none.
func (s *FileService) GoogleDrivePaths() ([]string, error) {
	return filesystem.GoogleDrivePaths()
}

func (s *FileService) Exists(path string) (bool, error) {
	if filesystem.IsTrashPath(path) {
		return true, nil
	}
	if remote.IsRemote(path) {
		be, err := s.backendFor(path)
		if err != nil {
			return false, err
		}
		return be.Exists(path)
	}
	if a, inner, ok := filesystem.SplitArchivePath(path); ok {
		if inner == "" {
			return filesystem.Exists(a)
		}
		return filesystem.ArchiveMemberExists(a, inner)
	}
	return filesystem.Exists(path)
}

// DiskUsage returns volume capacity for a local path.
func (s *FileService) DiskUsage(path string) (domain.DiskUsage, error) {
	if filesystem.IsTrashPath(path) {
		return domain.DiskUsage{}, fmt.Errorf("disk usage is not available in trash")
	}
	if remote.IsRemote(path) {
		return domain.DiskUsage{}, fmt.Errorf("disk usage is not available on remote paths")
	}
	if a, _, ok := filesystem.SplitArchivePath(path); ok {
		return filesystem.DiskUsage(a)
	}
	return filesystem.DiskUsage(path)
}

// IsArchivePath reports whether path is a browsable archive or a member inside one.
func (s *FileService) IsArchivePath(path string) bool {
	if remote.IsRemote(path) {
		return false
	}
	return filesystem.IsArchivePath(path)
}

// Copy copies sources into destDir.
// jobID from NewJobID enables CancelJob and transfer:progress events; empty is fire-and-forget.
func (s *FileService) Copy(jobID string, sources []string, destDir string) error {
	return s.runTransfer(jobID, "copy", sources, destDir, false)
}

// Move moves sources into destDir.
// jobID from NewJobID enables CancelJob and transfer:progress events; empty is fire-and-forget.
func (s *FileService) Move(jobID string, sources []string, destDir string) error {
	return s.runTransfer(jobID, "move", sources, destDir, true)
}

func (s *FileService) runTransfer(jobID, kind string, sources []string, destDir string, isMove bool) (err error) {
	defer func() { _ = s.FinishJob(jobID) }()
	label := transferLabel(kind, sources, destDir)
	emitDone := func(e error) {
		if jobID == "" {
			return
		}
		msg := ""
		if e != nil {
			msg = e.Error()
		}
		s.emit("transfer:done", domain.TransferDonePayload{JobID: jobID, Kind: kind, Error: msg})
	}
	defer func() { emitDone(err) }()

	onProgress := s.transferProgress(jobID, kind, label, destDir)
	ctx := s.jobCtx(jobID)
	if err := s.acquireTransfer(ctx); err != nil {
		return err
	}
	defer s.releaseTransfer()

	if err := rejectArchiveDest(destDir); err != nil {
		return err
	}
	if err := s.rejectTrashDest(destDir); err != nil {
		return err
	}
	if n := countInsideArchive(sources); n > 0 {
		if n != len(sources) {
			return fmt.Errorf("mixed archive/local selection is not supported")
		}
		if remote.IsRemote(destDir) {
			return fmt.Errorf("copy from archive to remote is not supported yet")
		}
		if isMove {
			return filesystem.ErrArchiveReadOnly
		}
		return s.extractArchiveSources(ctx, sources, destDir)
	}

	xfer, err := transferKind(sources, destDir)
	if err != nil {
		return err
	}
	switch xfer {
	case transferLocal:
		if isMove {
			return filesystem.MoveCtx(ctx, sources, destDir, onProgress)
		}
		return filesystem.CopyCtx(ctx, sources, destDir, onProgress)
	case transferRemoteWithin:
		be, err := s.backendFor(destDir)
		if err != nil {
			return err
		}
		if mgr, ok := be.(*remote.Manager); ok {
			if isMove {
				return mgr.MoveWithinCtx(ctx, sources, destDir, onProgress)
			}
			return mgr.CopyWithinCtx(ctx, sources, destDir, onProgress)
		}
		if smb, ok := be.(*remote.SMBManager); ok {
			if isMove {
				return smb.MoveWithinCtx(ctx, sources, destDir, onProgress)
			}
			return smb.CopyWithinCtx(ctx, sources, destDir, onProgress)
		}
		if mega, ok := be.(*remote.MEGAManager); ok {
			if isMove {
				return mega.MoveWithinCtx(ctx, sources, destDir, onProgress)
			}
			return mega.CopyWithinCtx(ctx, sources, destDir, onProgress)
		}
		if isMove {
			return be.MoveWithin(sources, destDir)
		}
		return be.CopyWithin(sources, destDir)
	case transferDownload:
		be, err := s.backendFor(sources[0])
		if err != nil {
			return err
		}
		if mgr, ok := be.(*remote.Manager); ok {
			if err := mgr.DownloadCtx(ctx, sources, destDir, onProgress); err != nil {
				return err
			}
		} else if smb, ok := be.(*remote.SMBManager); ok {
			if err := smb.DownloadCtx(ctx, sources, destDir, onProgress); err != nil {
				return err
			}
		} else if mega, ok := be.(*remote.MEGAManager); ok {
			if err := mega.DownloadCtx(ctx, sources, destDir, onProgress); err != nil {
				return err
			}
		} else if err := be.Download(sources, destDir); err != nil {
			return err
		}
		if isMove {
			return be.Delete(sources)
		}
		return nil
	case transferUpload:
		be, err := s.backendFor(destDir)
		if err != nil {
			return err
		}
		if mgr, ok := be.(*remote.Manager); ok {
			if err := mgr.UploadCtx(ctx, sources, destDir, onProgress); err != nil {
				return err
			}
		} else if smb, ok := be.(*remote.SMBManager); ok {
			if err := smb.UploadCtx(ctx, sources, destDir, onProgress); err != nil {
				return err
			}
		} else if mega, ok := be.(*remote.MEGAManager); ok {
			if err := mega.UploadCtx(ctx, sources, destDir, onProgress); err != nil {
				return err
			}
		} else if err := be.Upload(sources, destDir); err != nil {
			return err
		}
		if isMove {
			return filesystem.Delete(sources)
		}
		return nil
	default:
		return fmt.Errorf("unsupported %s", kind)
	}
}

func (s *FileService) transferProgress(jobID, kind, label, destDir string) filesystem.ProgressFunc {
	if jobID == "" {
		return nil
	}
	return func(ev filesystem.ProgressEvent) {
		var files []domain.TransferFileProgress
		if len(ev.Files) > 0 {
			files = make([]domain.TransferFileProgress, len(ev.Files))
			for i, f := range ev.Files {
				files[i] = domain.TransferFileProgress{
					Path: f.Path, Dest: f.Dest, Done: f.Done, Total: f.Total, Status: f.Status,
				}
			}
		}
		s.emit("transfer:progress", domain.TransferProgressPayload{
			JobID:       jobID,
			Kind:        kind,
			BytesDone:   ev.Done,
			BytesTotal:  ev.Total,
			CurrentPath: ev.CurrentPath,
			Label:       label,
			DestDir:     destDir,
			DestPath:    ev.DestPath,
			DestSize:    ev.DestSize,
			DestIsDir:   ev.DestIsDir,
			Files:       files,
		})
	}
}

func transferLabel(kind string, sources []string, destDir string) string {
	n := len(sources)
	if n == 0 {
		return kind
	}
	base := sources[0]
	if i := strings.LastIndexAny(base, `/\`); i >= 0 {
		base = base[i+1:]
	}
	verb := "Copy"
	switch kind {
	case "move":
		verb = "Move"
	case "archive":
		verb = "Archive"
	case "extract":
		verb = "Extract"
	}
	if n == 1 {
		return fmt.Sprintf("%s %s → %s", verb, base, destDir)
	}
	return fmt.Sprintf("%s %d items → %s", verb, n, destDir)
}

type xferKind int

const (
	transferLocal xferKind = iota
	transferRemoteWithin
	transferDownload // remote → local
	transferUpload   // local → remote
)

func transferKind(sources []string, destDir string) (xferKind, error) {
	if len(sources) == 0 {
		return 0, fmt.Errorf("no sources")
	}
	srcRemote := allRemote(sources)
	srcAnyRemote := anyRemote(sources)
	if srcAnyRemote && !srcRemote {
		return 0, fmt.Errorf("mixed local/remote selection is not supported")
	}
	if srcRemote {
		scheme := remote.SchemeOf(sources[0])
		for _, src := range sources {
			if remote.SchemeOf(src) != scheme {
				return 0, fmt.Errorf("cross-protocol copy not supported")
			}
		}
	}
	destRemote := remote.IsRemote(destDir)
	if srcRemote && destRemote && remote.SchemeOf(destDir) != remote.SchemeOf(sources[0]) {
		return 0, fmt.Errorf("cross-protocol copy not supported")
	}
	switch {
	case !srcRemote && !destRemote:
		return transferLocal, nil
	case srcRemote && destRemote:
		return transferRemoteWithin, nil
	case srcRemote && !destRemote:
		return transferDownload, nil
	case !srcRemote && destRemote:
		return transferUpload, nil
	default:
		return 0, fmt.Errorf("unsupported transfer")
	}
}

// Delete removes paths and returns an undo token. The token is empty when the
// delete cannot be undone (remote). The frontend only offers Undo for a non-empty token.
func (s *FileService) Delete(paths []string) (string, error) {
	if err := rejectInsideArchive(paths...); err != nil {
		return "", err
	}
	if err := rejectTrashWrite(paths...); err != nil {
		return "", err
	}
	if anyRemote(paths) {
		if !allRemote(paths) {
			return "", fmt.Errorf("mixed local/remote delete not supported")
		}
		scheme := remote.SchemeOf(paths[0])
		for _, p := range paths {
			if remote.SchemeOf(p) != scheme {
				return "", fmt.Errorf("cross-protocol delete not supported")
			}
		}
		be, err := s.backendFor(paths[0])
		if err != nil {
			return "", err
		}
		return "", be.Delete(paths)
	}
	if s.inTrash(paths) {
		return "", s.DeletePermanent(paths)
	}
	origins := make([]string, 0, len(paths))
	for _, p := range paths {
		abs, err := filesystem.Resolve(p)
		if err != nil {
			return "", err
		}
		origins = append(origins, abs)
	}
	if s.osTrash == nil {
		return "", filesystem.Delete(paths)
	}
	if err := s.osTrash.Put(paths); err != nil {
		return "", err
	}
	raw, err := json.Marshal(origins)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// DeletePermanent unlinks paths without sending them to trash.
func (s *FileService) DeletePermanent(paths []string) error {
	if err := rejectInsideArchive(paths...); err != nil {
		return err
	}
	if err := rejectTrashWrite(paths...); err != nil {
		return err
	}
	if anyRemote(paths) {
		if !allRemote(paths) {
			return fmt.Errorf("mixed local/remote delete not supported")
		}
		be, err := s.backendFor(paths[0])
		if err != nil {
			return err
		}
		return be.Delete(paths)
	}
	var first error
	osPaths := make([]string, 0, len(paths))
	leftPaths := make([]string, 0, len(paths))
	plain := make([]string, 0, len(paths))
	for _, p := range paths {
		switch {
		case s.trash != nil && s.trash.Contains(p):
			leftPaths = append(leftPaths, p)
		case s.osTrash != nil && s.osTrash.Contains(p):
			osPaths = append(osPaths, p)
		default:
			plain = append(plain, p)
		}
	}
	if len(osPaths) > 0 {
		if err := s.osTrash.Remove(osPaths); err != nil && first == nil {
			first = err
		}
	}
	if len(leftPaths) > 0 {
		if err := s.trash.Remove(leftPaths); err != nil && first == nil {
			first = err
		}
	}
	if len(plain) > 0 {
		if err := filesystem.Delete(plain); err != nil && first == nil {
			first = err
		}
	}
	return first
}

// RestoreDeleted puts a delete batch back (JSON original paths, or a leftover batch id).
func (s *FileService) RestoreDeleted(token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return fmt.Errorf("nothing to restore")
	}
	if filesystem.IsLegacyBatchID(token) {
		return s.trash.Restore(token)
	}
	var origins []string
	if err := json.Unmarshal([]byte(token), &origins); err != nil {
		return fmt.Errorf("invalid restore token")
	}
	n := 0
	var first error
	if s.osTrash != nil {
		got, err := s.osTrash.RestoreOrigins(origins)
		n += got
		if err != nil && first == nil {
			first = err
		}
	}
	if s.trash != nil {
		got, err := s.trash.RestoreOrigins(origins)
		n += got
		if err != nil && first == nil {
			first = err
		}
	}
	if n == 0 {
		if first != nil {
			return first
		}
		return fmt.Errorf("nothing left to restore")
	}
	return nil
}

// RestoreTrash puts selected trash:// rows back to their original paths.
func (s *FileService) RestoreTrash(paths []string) error {
	var osPaths, leftPaths []string
	for _, p := range paths {
		if s.trash != nil && s.trash.Contains(p) {
			leftPaths = append(leftPaths, p)
			continue
		}
		osPaths = append(osPaths, p)
	}
	var first error
	if len(osPaths) > 0 && s.osTrash != nil {
		if err := s.osTrash.Restore(osPaths); err != nil {
			first = err
		}
	}
	if len(leftPaths) > 0 && s.trash != nil {
		if err := s.trash.RestoreStored(leftPaths); err != nil && first == nil {
			first = err
		}
	}
	return first
}

// EmptyTrash empties the system trash and leftover app-trash batches.
func (s *FileService) EmptyTrash() error {
	var first error
	if s.osTrash != nil {
		first = s.osTrash.Empty()
	}
	if s.trash != nil {
		if err := s.trash.Empty(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func (s *FileService) inTrash(paths []string) bool {
	if len(paths) == 0 {
		return false
	}
	for _, p := range paths {
		if filesystem.IsTrashPath(p) {
			return true
		}
		in := (s.osTrash != nil && s.osTrash.Contains(p)) || (s.trash != nil && s.trash.Contains(p))
		if !in {
			return false
		}
	}
	return true
}

func rejectTrashWrite(paths ...string) error {
	for _, p := range paths {
		if filesystem.IsTrashPath(p) {
			return filesystem.ErrTrashReadOnly
		}
	}
	return nil
}

func (s *FileService) rejectTrashDest(paths ...string) error {
	if err := rejectTrashWrite(paths...); err != nil {
		return err
	}
	if s.inTrash(paths) {
		return filesystem.ErrTrashReadOnly
	}
	return nil
}

func (s *FileService) Rename(oldPath, newName string) (string, error) {
	if err := rejectInsideArchive(oldPath); err != nil {
		return "", err
	}
	if err := s.rejectTrashDest(oldPath); err != nil {
		return "", err
	}
	if remote.IsRemote(oldPath) {
		be, err := s.backendFor(oldPath)
		if err != nil {
			return "", err
		}
		return be.Rename(oldPath, newName)
	}
	return filesystem.Rename(oldPath, newName)
}

func (s *FileService) Mkdir(parent, name string) (string, error) {
	if err := rejectArchiveWrite(parent); err != nil {
		return "", err
	}
	if err := s.rejectTrashDest(parent); err != nil {
		return "", err
	}
	if remote.IsRemote(parent) {
		be, err := s.backendFor(parent)
		if err != nil {
			return "", err
		}
		return be.Mkdir(parent, name)
	}
	return filesystem.Mkdir(parent, name)
}

// CreateFile creates an empty file under parent (local or remote).
func (s *FileService) CreateFile(parent, name string) (string, error) {
	if err := rejectArchiveWrite(parent); err != nil {
		return "", err
	}
	if err := s.rejectTrashDest(parent); err != nil {
		return "", err
	}
	if remote.IsRemote(parent) {
		be, err := s.backendFor(parent)
		if err != nil {
			return "", err
		}
		loc, err := remote.ParseLocation(parent)
		if err != nil {
			return "", err
		}
		p := loc.JoinPath(gopath.Join(loc.RemotePath, name))
		if ok, err := be.Exists(p); err != nil {
			return "", err
		} else if ok {
			return "", fmt.Errorf("%w: %s", filesystem.ErrExists, p)
		}
		return p, be.WriteTextFile(p, "")
	}
	return filesystem.CreateFile(parent, name)
}

// PasteClipboard copies OS clipboard files into dest, or writes a PNG image.
func (s *FileService) PasteClipboard(dest string) error {
	if err := rejectArchiveWrite(dest); err != nil {
		return err
	}
	if err := s.rejectTrashDest(dest); err != nil {
		return err
	}
	if remote.IsRemote(dest) {
		return fmt.Errorf("paste is not available on remote connections")
	}
	log.Printf("PasteClipboard dest=%s", dest)
	err := clipboard.PasteInto(dest)
	if err != nil {
		log.Printf("PasteClipboard: %v", err)
	}
	return err
}

// ReadTextFile reads a text file for the built-in editor (local or remote).
func (s *FileService) ReadTextFile(path string) (string, error) {
	if remote.IsRemote(path) {
		be, err := s.backendFor(path)
		if err != nil {
			return "", err
		}
		return be.ReadTextFile(path)
	}
	if a, inner, ok := filesystem.SplitArchivePath(path); ok && inner != "" {
		return filesystem.ReadArchiveTextFile(a, inner, s.archivePassword(a))
	}
	return filesystem.ReadTextFile(path)
}

// WriteTextFile writes a text file from the built-in editor (local or remote).
func (s *FileService) WriteTextFile(path, content string) error {
	if err := rejectInsideArchive(path); err != nil {
		return err
	}
	if remote.IsRemote(path) {
		be, err := s.backendFor(path)
		if err != nil {
			return err
		}
		return be.WriteTextFile(path, content)
	}
	return filesystem.WriteTextFile(path, content)
}

// SearchTree finds nested files/folders under root (local and remote; Go-to).
func (s *FileService) SearchTree(root, query string, showHidden bool, limit int) ([]domain.SearchHit, error) {
	if filesystem.IsTrashPath(root) {
		return nil, fmt.Errorf("go-to is not available in trash")
	}
	if filesystem.IsArchivePath(root) {
		return nil, fmt.Errorf("go-to is not available inside archives yet")
	}
	if remote.IsRemote(root) {
		be, err := s.backendFor(root)
		if err != nil {
			return nil, err
		}
		return s.searchTreeRemote(be, root, query, showHidden, limit)
	}
	return filesystem.SearchTree(root, query, showHidden, limit)
}

func (s *FileService) searchTreeRemote(be remoteBackend, root, query string, showHidden bool, limit int) ([]domain.SearchHit, error) {
	if limit <= 0 {
		limit = 80
	}
	q := strings.ToLower(strings.TrimSpace(query))
	var hits []domain.SearchHit
	err := walkRemote(context.Background(), be, root, showHidden, func(e domain.FileEntry, rel string, depth int) (descend, stop bool) {
		if q == "" {
			if depth == 0 {
				hits = append(hits, domain.SearchHit{Name: e.Name, Path: e.Path, IsDir: e.IsDir, RelPath: rel})
			}
			// Empty query: immediate children only, matching filesystem.SearchTree.
			return false, false
		}
		if strings.Contains(strings.ToLower(e.Name), q) {
			hits = append(hits, domain.SearchHit{Name: e.Name, Path: e.Path, IsDir: e.IsDir, RelPath: rel})
		}
		return true, len(hits) >= limit*3
	})
	if err != nil {
		return nil, err
	}
	return filesystem.RankSearchHits(hits, q, limit), nil
}

// StartSearch runs a cancellable content or folder-name search and streams
// events: search:hit, search:denied, search:done, search:error.
// jobID should come from NewJobID. Mode is domain.SearchModeContent or SearchModeFolders.
func (s *FileService) StartSearch(
	jobID, root, query, mode, include, exclude string,
	caseSensitive, showHidden bool,
	limit int,
) error {
	if filesystem.IsTrashPath(root) {
		return fmt.Errorf("search is not available in trash")
	}
	if remote.IsRemote(root) && mode != domain.SearchModeFolders {
		return fmt.Errorf("content search is not available on remote connections yet")
	}
	if filesystem.IsArchivePath(root) {
		return fmt.Errorf("search is not available inside archives yet")
	}
	if jobID == "" {
		return fmt.Errorf("jobID required")
	}
	ctx := s.jobCtx(jobID)
	go s.runSearch(ctx, jobID, root, query, mode, include, exclude, caseSensitive, showHidden, limit)
	return nil
}

func (s *FileService) runSearch(
	ctx context.Context,
	jobID, root, query, mode, include, exclude string,
	caseSensitive, showHidden bool,
	limit int,
) {
	defer func() { _ = s.FinishJob(jobID) }()

	if mode == "" {
		mode = domain.SearchModeContent
	}
	var (
		truncated   bool
		err         error
		hitCount    int
		deniedCount int
	)

	onDenied := func(path string, derr error) {
		deniedCount++
		msg := ""
		if derr != nil {
			msg = derr.Error()
		}
		s.emit("search:denied", domain.SearchDeniedPayload{
			JobID: jobID,
			Path:  path,
			Error: msg,
		})
	}

	switch mode {
	case domain.SearchModeFolders:
		onHit := func(h domain.SearchHit) {
			hitCount++
			cp := h
			s.emit("search:hit", domain.SearchHitPayload{
				JobID:  jobID,
				Mode:   domain.SearchModeFolders,
				Folder: &cp,
			})
		}
		if remote.IsRemote(root) {
			var be remoteBackend
			be, err = s.backendFor(root)
			if err == nil {
				truncated, err = s.searchFoldersRemote(ctx, be, root, query, include, exclude, showHidden, limit, onHit, onDenied)
			}
			break
		}
		truncated, err = filesystem.SearchFolders(ctx, root, query, include, exclude, showHidden, limit, filesystem.FolderSearchCallbacks{
			OnHit:    onHit,
			OnDenied: onDenied,
		})
	default:
		truncated, err = filesystem.SearchContent(ctx, root, query, include, exclude, showHidden, caseSensitive, limit, filesystem.ContentSearchCallbacks{
			OnHit: func(h domain.ContentSearchHit) {
				hitCount++
				cp := h
				s.emit("search:hit", domain.SearchHitPayload{
					JobID:   jobID,
					Mode:    domain.SearchModeContent,
					Content: &cp,
				})
			},
			OnDenied: onDenied,
		})
	}

	if err != nil && ctx.Err() == nil {
		s.emit("search:error", domain.SearchErrorPayload{JobID: jobID, Error: err.Error()})
		return
	}
	s.emit("search:done", domain.SearchDonePayload{
		JobID:       jobID,
		Truncated:   truncated,
		HitCount:    hitCount,
		DeniedCount: deniedCount,
	})
}

// ReplaceOccurrence replaces one content match at path:line:column.
func (s *FileService) ReplaceOccurrence(path, find, replace string, line, column int, caseSensitive bool) error {
	if err := rejectInsideArchive(path); err != nil {
		return err
	}
	if remote.IsRemote(path) {
		return fmt.Errorf("replace is not available on remote connections yet")
	}
	return filesystem.ReplaceOccurrence(path, find, replace, line, column, caseSensitive)
}

// ReplaceAllInPaths replaces find with replace in each path (all occurrences per file).
func (s *FileService) ReplaceAllInPaths(paths []string, find, replace string, caseSensitive bool) (domain.ReplaceAllResult, error) {
	if err := rejectInsideArchive(paths...); err != nil {
		return domain.ReplaceAllResult{}, err
	}
	for _, p := range paths {
		if remote.IsRemote(p) {
			return domain.ReplaceAllResult{}, fmt.Errorf("replace is not available on remote connections yet")
		}
	}
	files, reps, err := filesystem.ReplaceAllInPaths(paths, find, replace, caseSensitive)
	if err != nil {
		return domain.ReplaceAllResult{}, err
	}
	return domain.ReplaceAllResult{FilesChanged: files, Replacements: reps}, nil
}

// OpenPrivacySettings opens OS privacy / full-disk access settings when possible.
func (s *FileService) OpenPrivacySettings() error {
	return config.OpenPrivacySettings()
}

// OpenLocalNetworkSettings opens OS Local Network / firewall settings when possible.
func (s *FileService) OpenLocalNetworkSettings() error {
	return config.OpenLocalNetworkSettings()
}

// Open opens a path with the OS default application.
func (s *FileService) Open(path string) error {
	if remote.IsRemote(path) {
		return fmt.Errorf("open is not supported for remote paths yet")
	}
	if filesystem.IsInsideArchive(path) {
		return filesystem.ErrArchiveReadOnly
	}
	return config.OpenInOS(path)
}

func rejectRemoteOpenWith(path string) error {
	if remote.IsRemote(path) {
		return fmt.Errorf("open with is not supported for remote paths")
	}
	if filesystem.IsInsideArchive(path) {
		return filesystem.ErrArchiveReadOnly
	}
	return nil
}

// ListOpenWithApps returns applications that can open a local file.
func (s *FileService) ListOpenWithApps(path string) ([]domain.OpenWithApp, error) {
	if err := rejectRemoteOpenWith(path); err != nil {
		return nil, err
	}
	return config.ListOpenWithApps(path)
}

// OpenWith opens path with the application identified by appID.
func (s *FileService) OpenWith(path, appID string) error {
	if err := rejectRemoteOpenWith(path); err != nil {
		return err
	}
	return config.OpenWith(path, appID)
}

// OpenWithPicker opens the OS application picker for a local file.
func (s *FileService) OpenWithPicker(path string) error {
	if err := rejectRemoteOpenWith(path); err != nil {
		return err
	}
	return config.OpenWithPicker(path)
}

// DirChildSizes returns recursive sizes for immediate child directories, plus
// the children that could not be fully read (permission denied).
// jobID from NewJobID enables CancelJob; empty jobID is non-cancellable.
func (s *FileService) DirChildSizes(jobID string, dir string) (domain.DirSizes, error) {
	defer func() { _ = s.FinishJob(jobID) }()
	if filesystem.IsTrashPath(dir) || filesystem.IsArchivePath(dir) {
		return domain.DirSizes{Sizes: map[string]int64{}, Denied: []string{}}, nil
	}
	if remote.IsRemote(dir) {
		be, err := s.backendFor(dir)
		if err != nil {
			return domain.DirSizes{}, err
		}
		return be.DirChildSizesCtx(s.jobCtx(jobID), dir)
	}
	return filesystem.DirChildSizesCtx(s.jobCtx(jobID), dir)
}

func anyRemote(paths []string) bool {
	for _, p := range paths {
		if remote.IsRemote(p) {
			return true
		}
	}
	return false
}

func allRemote(paths []string) bool {
	if len(paths) == 0 {
		return false
	}
	for _, p := range paths {
		if !remote.IsRemote(p) {
			return false
		}
	}
	return true
}

// archivePassword returns the cached password for archiveAbs, or "".
func (s *FileService) archivePassword(archiveAbs string) string {
	if v, ok := s.archivePw.Load(archiveAbs); ok {
		return v.(string)
	}
	return ""
}

// SetArchivePassword validates password against archivePath's central
// directory and caches it for the session (used by ReadTextFile/Extract on
// subsequent calls into the same archive). Returns ErrBadPassword on mismatch.
func (s *FileService) SetArchivePassword(archivePath, password string) error {
	abs, err := filesystem.Resolve(archivePath)
	if err != nil {
		return err
	}
	if err := filesystem.CheckArchivePassword(abs, password); err != nil {
		return err
	}
	s.archivePw.Store(abs, password)
	return nil
}

// ListArchiveCreateFormats returns formats the create dialog can use.
func (s *FileService) ListArchiveCreateFormats() []string {
	return append([]string(nil), filesystem.CreateFormats...)
}

// Archive packs sources into destPath using format (zip, tar.gz, …).
// password enables traditional zip encryption when format is zip.
// jobID from NewJobID enables CancelJob and transfer:progress events; empty jobID is fire-and-forget.
// Unlike Copy/Move, this does not emit transfer:done — a single Archive call
// is one full job, so the frontend (useArchiveExtract) registers/removes the
// transfer-bar row itself around the call, the same way startTransfer does.
func (s *FileService) Archive(jobID string, sources []string, destPath, format, password string) error {
	defer func() { _ = s.FinishJob(jobID) }()
	if anyRemote(sources) || remote.IsRemote(destPath) {
		return fmt.Errorf("archive is not supported for remote paths yet")
	}
	if err := rejectInsideArchive(append(sources, destPath)...); err != nil {
		return err
	}
	label := transferLabel("archive", sources, destPath)
	onProgress := s.transferProgress(jobID, "archive", label, destPath)
	return filesystem.Archive(s.jobCtx(jobID), sources, destPath, format, password, onProgress)
}

// Extract unpacks archivePath into destDir. password for protected rar/7z/zip when needed.
// Does not finish the job — call FinishJob after multi-extract, or CancelJob.
// No transfer:done either (see Archive) — a multi-extract loop shares one
// jobID across several Extract calls, so only the caller knows when it's done.
func (s *FileService) Extract(jobID string, archivePath, destDir, password string) error {
	if remote.IsRemote(archivePath) || remote.IsRemote(destDir) {
		return fmt.Errorf("extract is not supported for remote paths yet")
	}
	if filesystem.IsInsideArchive(archivePath) || filesystem.IsArchivePath(destDir) {
		return filesystem.ErrArchiveReadOnly
	}
	label := transferLabel("extract", []string{archivePath}, destDir)
	onProgress := s.transferProgress(jobID, "extract", label, destDir)
	return filesystem.Extract(s.jobCtx(jobID), archivePath, destDir, password, onProgress)
}

// ExtractBatch unpacks each archivePaths[i] into destDirs[i] (same length),
// sharing one progress total across the whole batch — see
// filesystem.ExtractBatch — so a multi-select extract's transfer-bar row
// progresses monotonically instead of resetting per archive.
// Does not finish the job — call FinishJob after, or CancelJob.
func (s *FileService) ExtractBatch(jobID string, archivePaths, destDirs []string, password string) error {
	if len(archivePaths) != len(destDirs) {
		return fmt.Errorf("archivePaths and destDirs must be the same length")
	}
	jobs := make([]filesystem.ExtractJob, len(archivePaths))
	for i, a := range archivePaths {
		d := destDirs[i]
		if remote.IsRemote(a) || remote.IsRemote(d) {
			return fmt.Errorf("extract is not supported for remote paths yet")
		}
		if filesystem.IsInsideArchive(a) || filesystem.IsArchivePath(d) {
			return filesystem.ErrArchiveReadOnly
		}
		jobs[i] = filesystem.ExtractJob{ArchivePath: a, DestDir: d}
	}
	label := transferLabel("extract", archivePaths, "")
	onProgress := s.transferProgress(jobID, "extract", label, "")
	return filesystem.ExtractBatch(s.jobCtx(jobID), jobs, password, onProgress)
}

// ArchiveExtension returns the extension for a create format.
func (s *FileService) ArchiveExtension(format string) string {
	return filesystem.ExtensionForFormat(format)
}
