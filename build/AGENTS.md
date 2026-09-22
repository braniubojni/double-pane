# build — agent memory

Parent: root `AGENTS.md`. Wails packaging assets + platform Taskfiles.

## Icon (single source)

| File                    | Role                                    |
| ----------------------- | --------------------------------------- |
| **`build/appicon.png`** | **Only** brand source (1024²)           |
| `windows/icon.ico`      | Generated                               |
| `darwin/icons.icns`     | Generated                               |
| `ios/icon.png`          | Keep in sync with appicon when branding |

```bash
# from build/
wails3 generate icons -input appicon.png \
  -macfilename darwin/icons.icns -windowsfilename windows/icon.ico
```

- Task `common:generate:icons` in `build/Taskfile.yml` (sources: `appicon.png` only).
- **Do not** keep product icons under `frontend/public`.
- No `appicon.icon` composer package required for desktop (Assets.car optional/stale OK if icns present).
- macOS Dock/Finder icon only appears on **`.app`** (`darwin:package` / `wails3 package`), not bare `bin/double-pane`.

## macOS package vs build

| Command                                  | Output                                           |
| ---------------------------------------- | ------------------------------------------------ |
| `wails3 build` / `darwin:build`          | Bare Mach-O → Terminal icon if double-clicked    |
| `wails3 package` / `task package:darwin` | `bin/double-pane.app` (drag to Applications) |

Brand metadata: `build/config.yml` + `darwin/Info.plist` (`Double Pane`, `com.braniubojni.double-pane`).

## Multi-platform local dist

```bash
task setup:docker          # once (Linux cross) — host arch image
task dist VERSION=0.1.0    # → dist/* named for the in-app updater
```

`task dist` starts Docker Desktop on macOS when the daemon is down, and **quits it at the end of that run** (including on failure) only if this dist started it. It does not quit Docker that was already running, and does not auto-quit after `build:linux` / `setup:docker`.

Defaults: darwin/arm64 `.app` zip, windows/amd64 zip, **linux/\<host arch\>** tar.gz
(so Apple Silicon produces `linux_arm64` and matches host `wails-cross`).

```bash
# CI-like linux/amd64 from Apple Silicon (QEMU image — slow):
task setup:docker ARCH=amd64
task dist VERSION=0.1.0 LINUX_ARCH=amd64
```

## Linux cross-compile (macOS/Windows hosts)

- Image: `wails-cross` (`task setup:docker` / `wails3 task setup:docker`).
- **Single-arch:** Linux target arch must match image arch (`--platform linux/<arch>`).
- Image has **GTK 4.8**; Wails default needs **GtkFileDialog (GTK 4.10+)**.
- **Fix:** Docker Linux builds use **`EXTRA_TAGS=gtk3`** (Taskfile default for docker path).
- **Native Ubuntu 24.04 CI/release:** full **GTK4** (no gtk3 tag).

```bash
wails3 build GOOS=linux GOARCH=arm64   # uses docker + gtk3 on Apple Silicon
task build:linux ARCH=arm64 VERSION=0.1.0
```

## Layout

```
build/Taskfile.yml          # common: frontend, icons, bindings
build/config.yml            # Wails app metadata
build/darwin|linux|windows/ # platform package tasks
build/docker/               # Dockerfile.cross, Dockerfile.server
build/ios|android/          # mobile scaffolding (golangci-excluded)
```

## Version

- Pass `VERSION` into Task/build for ldflags → `internal/version.Version`.
- Keep `config.yml` version in sync on local release (`task dist` / `task release:local`).

## Don’t

- “Fix” Linux docker by forcing GTK4 APIs without upgrading the cross image.
- Commit machine-local mobile overlays (`build/ios/xcode/`, `build/android/gen/` — gitignored).
- Expect a custom Dock icon from a bare `wails3 build` binary on macOS.
