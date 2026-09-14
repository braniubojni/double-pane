package domain

// FileEntry is a single directory listing row.
type FileEntry struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	IsDir     bool   `json:"isDir"`
	Size      int64  `json:"size"`
	ModTime   int64  `json:"modTime"` // unix milliseconds
	Ext       string `json:"ext"`
	IsSymlink bool   `json:"isSymlink"`
	// Access is "full" | "readonly" | "partial" | "none", or "" when unknown.
	// Remote (SFTP) entries are always "": Windows OpenSSH reports a constant
	// mode for everything, so the bits would be a lie. Remote access is instead
	// discovered by operations that actually get denied — see DirSizes.Denied.
	Access string `json:"access"`
}

// OpenWithApp is one OS application that can open a local file.
type OpenWithApp struct {
	ID   string `json:"id"` // bundle path, .desktop id, or executable
	Name string `json:"name"`
}

// DirSizes is the result of a recursive child-size calculation. Denied lists the
// child directories the walk could not fully read, so the UI can mark them
// rather than silently reporting an undercount.
type DirSizes struct {
	Sizes  map[string]int64 `json:"sizes"`
	Denied []string         `json:"denied"`
}

// DiskUsage is capacity of the volume that contains Path.
type DiskUsage struct {
	Path  string `json:"path"`
	Total int64  `json:"total"`
	Free  int64  `json:"free"` // available to the user (bavail)
	Used  int64  `json:"used"`
}

// GitStatusEntry is one child name with a compact git working-tree status.
// Status is one of: M (modified), A (added), D (deleted), U (unmerged), ? (untracked).
type GitStatusEntry struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

// GitDirStatus is git status for immediate children of one listed directory.
type GitDirStatus struct {
	RepoRoot string           `json:"repoRoot"` // empty if not in a repo
	Entries  []GitStatusEntry `json:"entries"`
}

// GitFileDiff is HEAD vs working-tree content for one local file (read-only viewer).
type GitFileDiff struct {
	Path      string `json:"path"`
	RepoRoot  string `json:"repoRoot"`
	RelPath   string `json:"relPath"`
	Status    string `json:"status"` // M|A|D|U|? or empty
	OldText   string `json:"oldText"`
	NewText   string `json:"newText"`
	Binary    bool   `json:"binary"`
	Truncated bool   `json:"truncated"`
	Message   string `json:"message"` // soft-fail reason when no usable content
}

// Bookmark is a saved directory shortcut.
type Bookmark struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Path      string `json:"path"`
	SortOrder int    `json:"sortOrder"`
	CreatedAt string `json:"createdAt"`
}

// PanePaths holds last-used left/right directory paths.
type PanePaths struct {
	Left  string `json:"left"`
	Right string `json:"right"`
}

// TabState is one open tab in a pane (path only; history is not persisted).
type TabState struct {
	Path string `json:"path"`
}

// PaneTabs holds each pane's open tabs and which one is active.
type PaneTabs struct {
	Left        []TabState `json:"left"`
	LeftActive  int        `json:"leftActive"`
	Right       []TabState `json:"right"`
	RightActive int        `json:"rightActive"`
}

// PaneGridPrefs is one pane's file-grid sort, hidden columns, and column order.
type PaneGridPrefs struct {
	SortField string   `json:"sortField"` // default "displayName"
	SortDir   string   `json:"sortDir"`   // "asc" | "desc"
	Hidden    []string `json:"hidden"`    // fields hidden; icon+displayName cannot hide
	Order     []string `json:"order"`     // fields left-to-right; unknown fields appended
}

// GridPrefs holds independent left/right file-grid column preferences.
type GridPrefs struct {
	Left  PaneGridPrefs `json:"left"`
	Right PaneGridPrefs `json:"right"`
}

// Default main-window size: two panes with every file-grid column visible.
const (
	DefaultWindowWidth  = 1680
	DefaultWindowHeight = 900
	MinWindowWidth      = 900
	MinWindowHeight     = 500
	// MaxWindowWidth/Height are a sanity ceiling on a persisted size, not a
	// real display limit — guards against a corrupted blob or a size saved
	// on a much larger monitor than the one the app next launches on.
	MaxWindowWidth  = 8000
	MaxWindowHeight = 6000
)

// WindowState is the last used main-window size (persisted separately from Settings).
type WindowState struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// ThemeMode values for Settings.Theme.
const (
	ThemeSystem = "system"
	ThemeDark   = "dark"
	ThemeLight  = "light"
)

// Settings is the contents of settings.json.
type Settings struct {
	Theme                   string `json:"theme"`
	ShowHidden              bool   `json:"showHidden"`
	ShowExtensions          bool   `json:"showExtensions"`
	ShowGitStatus           bool   `json:"showGitStatus"`
	UseBuiltInEditor        bool   `json:"useBuiltInEditor"`
	AutoCheckUpdates        bool   `json:"autoCheckUpdates"`
	UpdateCheckIntervalDays int    `json:"updateCheckIntervalDays"`
	LastUpdateCheckAt       string `json:"lastUpdateCheckAt"`    // RFC3339; empty if never
	SkippedUpdateVersion    string `json:"skippedUpdateVersion"` // remote version user skipped
	LeftPath                string `json:"leftPath"`
	RightPath               string `json:"rightPath"`
}

// SearchHit is one result from nested file/folder search (Go-to).
type SearchHit struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	IsDir   bool   `json:"isDir"`
	RelPath string `json:"relPath"`
}

// Search mode values for Find-in-files dialog.
const (
	SearchModeContent = "content"
	SearchModeFolders = "folders"
)

// ContentSearchHit is one text match inside a file.
type ContentSearchHit struct {
	Path       string `json:"path"`
	RelPath    string `json:"relPath"`
	Line       int    `json:"line"`       // 1-based
	Column     int    `json:"column"`     // 1-based byte index of match on the full line
	LineText   string `json:"lineText"`   // windowed single-line preview (may include …)
	MatchStart int    `json:"matchStart"` // 0-based byte index into LineText
	MatchEnd   int    `json:"matchEnd"`   // exclusive byte index into LineText
}

// SearchPrefs is the last Find-in-files dialog state (persisted).
type SearchPrefs struct {
	Query         string `json:"query"`
	Replace       string `json:"replace"`
	Include       string `json:"include"`
	Exclude       string `json:"exclude"`
	Mode          string `json:"mode"` // content | folders
	ReplaceOpen   bool   `json:"replaceOpen"`
	CaseSensitive bool   `json:"caseSensitive"`
}

// SearchDonePayload is emitted when a streaming search finishes.
type SearchDonePayload struct {
	JobID       string `json:"jobId"`
	Truncated   bool   `json:"truncated"`
	HitCount    int    `json:"hitCount"`
	DeniedCount int    `json:"deniedCount"`
}

// SearchHitPayload wraps a streaming search hit for the UI.
type SearchHitPayload struct {
	JobID   string            `json:"jobId"`
	Mode    string            `json:"mode"` // content | folders
	Content *ContentSearchHit `json:"content,omitempty"`
	Folder  *SearchHit        `json:"folder,omitempty"`
}

// SearchDeniedPayload is one path the walker could not open.
type SearchDeniedPayload struct {
	JobID string `json:"jobId"`
	Path  string `json:"path"`
	Error string `json:"error"`
}

// ScanEstimate is a pre-hash count for the duplicate-finder setup dialog.
type ScanEstimate struct {
	FileCount         int64  `json:"fileCount"`
	ByteCount         int64  `json:"byteCount"`
	EtaSeconds        int    `json:"etaSeconds"`
	Protocol          string `json:"protocol"` // local | ssh | smb | mega
	MegaDownload      bool   `json:"megaDownload"`
	ImageCount        int64  `json:"imageCount"`
	EtaExactSeconds   int    `json:"etaExactSeconds"`
	EtaVisualSeconds  int    `json:"etaVisualSeconds"`
	EtaOcrSeconds     int    `json:"etaOcrSeconds"`
	MegaDownloadBytes int64  `json:"megaDownloadBytes"`
}

// DuplicateFile is one member of a same-hash group.
type DuplicateFile struct {
	Path     string `json:"path"`
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	ModTime  int64  `json:"modTime"`
	Protocol string `json:"protocol"` // local | ssh | smb | mega
}

// DuplicateGroup is files that share one SHA-256 (kind=exact), dHash, or OCR text.
type DuplicateGroup struct {
	Hash       string          `json:"hash"`
	Size       int64           `json:"size"`
	Files      []DuplicateFile `json:"files"`
	Kind       string          `json:"kind"`                 // exact | visual | ocr
	Similarity int             `json:"similarity,omitempty"` // visual Hamming % or OCR Jaccard %
	Snippet    string          `json:"snippet,omitempty"`    // OCR text preview (≤80 chars)
}

// DupProgressPayload is emitted while a duplicate scan runs.
type DupProgressPayload struct {
	JobID       string `json:"jobId"`
	DoneFiles   int64  `json:"doneFiles"`
	TotalFiles  int64  `json:"totalFiles"`
	DoneBytes   int64  `json:"doneBytes"`
	TotalBytes  int64  `json:"totalBytes"`
	Groups      int    `json:"groups"`
	Skipped     int    `json:"skipped"`
	CurrentPath string `json:"currentPath"`
	Phase       string `json:"phase,omitempty"` // walk | exact | visual | ocr
	DoneImages  int64  `json:"doneImages,omitempty"`
	TotalImages int64  `json:"totalImages,omitempty"`
}

// DupGroupPayload streams one closed duplicate group.
type DupGroupPayload struct {
	JobID string         `json:"jobId"`
	Group DuplicateGroup `json:"group"`
}

// DupErrorPayload is a per-path skip (fatal=false) or a job-ending failure.
type DupErrorPayload struct {
	JobID   string `json:"jobId"`
	Path    string `json:"path"`
	Message string `json:"message"`
	Fatal   bool   `json:"fatal"`
}

// DupDonePayload is emitted when a duplicate scan finishes or is cancelled.
// Groups are attached here (not streamed as dup:group) so a fast scan cannot
// flood Wails ExecJS and drop this event — the running dialog would hang.
type DupDonePayload struct {
	JobID  string           `json:"jobId"`
	Error  string           `json:"error,omitempty"`
	Groups []DuplicateGroup `json:"groups,omitempty"`
}

// SearchErrorPayload is a fatal search failure.
type SearchErrorPayload struct {
	JobID string `json:"jobId"`
	Error string `json:"error"`
}

// TransferProgressPayload is emitted while a copy/move/attach job runs.
type TransferProgressPayload struct {
	JobID       string                 `json:"jobId"`
	Kind        string                 `json:"kind"` // copy | move | attach
	BytesDone   int64                  `json:"bytesDone"`
	BytesTotal  int64                  `json:"bytesTotal"`
	CurrentPath string                 `json:"currentPath"`
	Label       string                 `json:"label"`
	DestDir     string                 `json:"destDir"`
	DestPath    string                 `json:"destPath,omitempty"`
	DestSize    int64                  `json:"destSize"`
	DestIsDir   bool                   `json:"destIsDir"`
	Files       []TransferFileProgress `json:"files,omitempty"`
}

// TransferFileProgress is one top-level source's own progress within a
// TransferProgressPayload's job — lets the UI show/cancel it individually.
type TransferFileProgress struct {
	Path   string `json:"path"` // FileService.CancelTransferFile's path arg
	Dest   string `json:"dest"` // dest row id this file was optimistically added under
	Done   int64  `json:"done"`
	Total  int64  `json:"total"`
	Status string `json:"status"` // active | done | canceled
}

// PortListener is a local TCP socket in LISTEN state.
type PortListener struct {
	Port    int    `json:"port"`
	PID     int    `json:"pid"`
	Process string `json:"process"`
	Proto   string `json:"proto"` // "tcp"
}

// ProcessInfo is a user-owned OS process (port killer Processes tab).
type ProcessInfo struct {
	PID  int    `json:"pid"`
	Name string `json:"name"`
	Cmd  string `json:"cmd"`
	Cwd  string `json:"cwd,omitempty"`
}

// AIUsage is one AI coding agent's quota snapshot.
type AIUsage struct {
	ID       string          `json:"id"` // claude | grok | cursor
	Name     string          `json:"name"`
	Status   string          `json:"status"` // ok | not-installed | unsupported | error
	Error    string          `json:"error,omitempty"`
	Estimate bool            `json:"estimate"` // derived locally, not authoritative
	Limits   []AIUsageLimit  `json:"limits"`
	Details  []AIUsageDetail `json:"details"`
}

// AIUsageLimit is one quota window (e.g. "current session").
type AIUsageLimit struct {
	Label   string `json:"label"`
	Percent int    `json:"percent"` // 0..100, -1 = unknown
	ResetAt string `json:"resetAt,omitempty"`
}

// AIUsageDetail is one free-form expanded line (e.g. request counts).
type AIUsageDetail struct {
	Label string `json:"label"`
	Value string `json:"value"`
	Depth int    `json:"depth"` // 0 = header, 1 = indented bullet
}

// Volume is an OS-mounted drive, network share, or disk image.
type Volume struct {
	Path        string `json:"path"`
	Name        string `json:"name"`
	Kind        string `json:"kind"` // internal | external | network | disk-image
	Unmountable bool   `json:"unmountable"`
	SourcePath  string `json:"sourcePath,omitempty"` // .dmg path when known
	Device      string `json:"device,omitempty"`
}

// TransferDonePayload is emitted when a copy/move/attach job finishes (success or error).
type TransferDonePayload struct {
	JobID string `json:"jobId"`
	Kind  string `json:"kind"`  // copy | move | attach
	Error string `json:"error"` // empty on success
}

// ReplaceAllRequest replaces find with replace across paths (content mode).
type ReplaceAllRequest struct {
	Paths         []string `json:"paths"`
	Find          string   `json:"find"`
	Replace       string   `json:"replace"`
	CaseSensitive bool     `json:"caseSensitive"`
}

// ReplaceAllResult summarizes a batch replace.
type ReplaceAllResult struct {
	FilesChanged int `json:"filesChanged"`
	Replacements int `json:"replacements"`
}

// ShortcutDef describes one bindable action (for UI + docs).
type ShortcutDef struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Binding     string `json:"binding"`
}

// ConnectionProfile is a saved remote connection (SSH, SMB, or MEGA).
type ConnectionProfile struct {
	ID             string   `json:"id"`
	Protocol       string   `json:"protocol"` // "ssh", "smb", or "mega"
	User           string   `json:"user"`
	Host           string   `json:"host"`
	Port           int      `json:"port"`
	Label          string   `json:"label"`                    // display name; default user@host
	ConfigAlias    string   `json:"configAlias,omitempty"`    // ~/.ssh/config Host alias (re-resolved on connect)
	IdentityFiles  []string `json:"identityFiles,omitempty"`  // private key paths (also re-merged from config)
	DefaultWorkDir string   `json:"defaultWorkDir,omitempty"` // preferred start path
	Domain         string   `json:"domain,omitempty"`         // SMB NTLM domain
}

// SMBShare is one share exported by an SMB host.
type SMBShare struct {
	Name      string `json:"name"`
	Hidden    bool   `json:"hidden"`
	LocalPath string `json:"localPath,omitempty"` // set when the OS mounted the volume
}

// ActiveSession is a live remote connection for UI.
type ActiveSession struct {
	Key      string `json:"key"` // user@host:port
	Protocol string `json:"protocol"`
	User     string `json:"user"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	RootPath string `json:"rootPath"` // ssh://user@host:port/
}

// ConnectResult is returned after a successful SSH dial.
type ConnectResult struct {
	RootPath       string     `json:"rootPath"`
	HomePath       string     `json:"homePath"`                 // remote home or root
	Key            string     `json:"key"`                      // session key user@host:port
	ProfileID      string     `json:"profileId,omitempty"`      // saved profile id when known
	DefaultWorkDir string     `json:"defaultWorkDir,omitempty"` // profile default start path if set
	Shares         []SMBShare `json:"shares,omitempty"`         // SMB share list after connect
	LocalMount     bool       `json:"localMount,omitempty"`     // OS/Finder mounted; paths are local
}

// SSHConfigHost is a host entry from an OpenSSH client config file.
type SSHConfigHost struct {
	Alias         string   `json:"alias"`
	HostName      string   `json:"hostName"`
	User          string   `json:"user"`
	Port          int      `json:"port"`
	IdentityFiles []string `json:"identityFiles"`
	ConfigPath    string   `json:"configPath,omitempty"` // absolute path of the file this entry was read from
}

// RemoteRecent is a recently visited remote directory path.
type RemoteRecent struct {
	SessionKey  string `json:"sessionKey"`
	Path        string `json:"path"`        // ssh:// virtual path
	Label       string `json:"label"`       // remote abs path for display
	LastVisited string `json:"lastVisited"` // RFC3339
}

// AppName is used for config/data directories.
const AppName = "go-file-manager"
