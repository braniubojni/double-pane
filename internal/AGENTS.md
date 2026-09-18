# internal — agent memory (Go)

Parent: root `AGENTS.md`. All app logic lives here; `main.go` only wires Wails + services.

## Packages

| Package      | Role                                                                          |
| ------------ | ----------------------------------------------------------------------------- |
| `domain`     | Shared models (settings, files, bookmarks)                       |
| `filesystem` | Local FS: list/copy/move/delete (clonefile/FICLONE then byte copy; workers from CPU + I/O class: HDD 2, SSD `NumCPU()*2` cap 64, network 32; cancel deletes copy dests), **OS trash** + virtual `trash://`, archive/extract, zip/tar virtual folders, search, text R/W, dir sizes, `DiskUsage` |
| `volumes`    | OS mounts list/unmount, DMG attach (darwin), poll watcher                       |
| `ports`      | Local TCP LISTEN sockets (`lsof`/`netstat`) + user processes + force-kill by PID                 |
| `gitstatus`  | Upward-only repo root + one scoped `git status` (no disk-wide `.git` walk)   |
| `remote`     | SSH/SFTP + SMB + MEGA (`path.go`, `ssh.go`, `smb.go`, `mega.go`) |
| `storage`    | SQLite bookmarks + crypto helpers                                             |
| `config`     | Config dir, OS open, Open-with (`openwith_*.go`)                              |
| `service`    | Wails-bound services (thin orchestration over packages above)                 |
| `imghash`    | 64-bit dHash for similar-image duplicate groups (pure Go, no CGo)             |
| `ocr`        | System `tesseract` exec wrapper for optional OCR duplicate groups (no CGo)   |
| `version`    | `Version` string; set via `-ldflags` / Task `VERSION`                         |

## Services (Wails)

Registered in `main.go`:

- `FileService` — FS + remote + jobs cancel; `DiskUsage`, `ListOpenWithApps` / `OpenWith` / `OpenWithPicker`
- `SettingsService` — JSON settings/shortcuts + pane paths + `GetGridPrefs` / `SaveGridPrefs` + `GetWindowState` / `SaveWindowState` KV
- `BookmarkService` — SQLite
- `ConnectionService` — SSH/SMB/MEGA profiles/sessions
- `TerminalService` — PTY per pane (`_unix` / `_windows`); holds `*application.App` for events
- `UpdateService` — thin façade over `app.Updater` (CheckAndInstall / GetVersion / OpenReleases)
- `GitService` — `StatusForDir` (cached root + porcelain; local only)
- `PortService` — list local TCP listeners and user processes; `Kill` / `KillAll` by PID

## Remote paths

- Virtual paths: `ssh://user@host:port/remote/path`, `smb://user@host:port/Share/path`, or `mega://user@domain/path` (see `remote.ParseLocation`). `trash://` lists OS trash (plus leftover app-trash batches).
- Archive browse: pane path is the zip/tar file plus inner members (`/path/to/a.zip/docs`); writes inside archives are rejected (`ErrArchiveReadOnly`).
- `Location` **embeds** `Spec` → use `loc.JoinPath(...)`, not `loc.Spec.JoinPath` (staticcheck QF1008).

## Lint / style

- `gofmt` required (husky + CI).
- **errcheck:** prefer `defer func() { _ = f.Close() }()`; return explicit `Close()` error when it matters.
- golangci config: root `.golangci.yml` (errcheck, govet, staticcheck, ineffassign, unused).
- `build/` excluded from golangci (platform scaffolding).

## Tests

```bash
go test ./internal/...
```

Package tests next to code (`*_test.go`). Prefer table-driven tests for path/parse helpers.

## Linux / CGO note

`TerminalService` (and main) import `wails/v3/pkg/application` → on Linux, **`go vet`/`go test` of those packages need GTK4 + WebKitGTK 6.0**. See `.github/AGENTS.md`. Pure packages (`filesystem`, `remote` parse, `storage`, `config`, `version`) are lighter.

## Do

- Keep FS logic in `filesystem`, not in services.
- Propagate `context.Context` for long ops (sizes, archive) when already patterned.
- Match existing error wrapping `fmt.Errorf("…: %w", err)`.

## Don’t

- Hand-edit generated frontend bindings.
- Add heavy deps without need (prefer stdlib).
- Break dual-pane path model (left/right independent).
