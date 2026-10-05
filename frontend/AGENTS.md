# frontend — agent memory

Parent: root `AGENTS.md`. React UI for Wails (Vite 8).

## Tree

```
src/
  main.tsx, App.tsx
  app/           # Providers, MUI theme (system/dark/light)
  pages/file-manager/   # Shell: panes, keyboard, path persist
  widgets/       # file-pane, toolbar, editor, terminal, menu, go-to, command-palette, status-bar, port-killer
  features/      # Zustand stores + small dialogs (settings, shortcuts, updates…)
  entities/file/ # TanStack Query hooks + domain TS types
  shared/        # api/bindings re-export, format, shortcuts, ErrorBoundary
bindings/        # GENERATED Wails TS — do not edit
public/          # usually empty; no brand icons here
```

## Patterns

| Do                                  | Don’t                  |
| ----------------------------------- | ---------------------- |
| MUI path imports                    | `@mui/material` barrel |
| `mutate` + onSuccess/onError        | `mutateAsync` chains   |
| `FC` components                     | class components       |
| Colocate `styles.ts` / `helpers.ts` | Dump into global utils |
| Keep files ~100–150 lines           | Mega-components        |
| Derive in render / event handlers   | `useEffect` for UI state |
| CodeMirror 6 editor                 | Monaco / workers       |
| `useShallow` for array/object zustand selectors | Bare selector returning `.filter/.map/[x]/[]` |

- Zustand selector returning a new array/object each call (e.g. `s.getActionPaths(id)`, `.filter(...)`, `[x]`) breaks React's snapshot equality check → infinite re-render loop ("getSnapshot should be cached"). Wrap in `useShallow` from `zustand/react/shallow`, or select primitives and derive with `useMemo`. Primitive selectors (string/number/bool/stable ref) are fine as-is.

## Data flow

- **Server state:** TanStack Query in `entities/file/queries.ts` → `FileService` / settings / bookmarks via `shared/api/bindings`.
- **UI state:** Zustand — `features/pane`, `editor`, `terminal`, `file-ops`, `jobs`, `updates`, `ui/*` (grid prefs: `gridPrefsStore`, KV via `SettingsService`).
- **Command palette:** `widgets/command-palette` + `features/command-palette` (Mod+K); runs catalog ids through `runShortcutAction`.
- **Jobs:** pane-level busy (copy/archive/sizes) → `features/jobs/paneJobStore` + toolbar `runPaneJob`.
- **Transfers:** status bar + dest-row `apiRef.updateRows` (`useDestRowUpdates`); do not refetch `['dir']` on progress ticks.
- **Volumes:** `useVolumes` + `useVolumeEvents`; toolbar `DrivesMenu`.

## Editor

- Workspace: `widgets/editor/EditorWorkspace.tsx` + tree + CodeMirror pane.
- Lang extensions: `widgets/editor/helpers.ts` (`languageExtensionForPath`).
- Go-to file: `widgets/go-to` + `features/go-to/goToStore` (Cmd/Ctrl+P style).

## Updates UI

- `features/updates/*` + Settings `UpdatesSection`.
- Check / auto-check → `UpdateService.CheckAndInstall` (Wails builtin update window).

## Tooling

```bash
cd frontend
npm run lint          # eslint src
npm run knip          # knip.json ignores bindings/**
npm run format        # oxfmt src
npm run build
```

- Vite: production minify default (do **not** force esbuild — not installed by default).
- Alias: `@` → `src/`.
- Bindings regen: Wails build/dev (`wails3 generate bindings` via Task).

## Don’t reintroduce

- Template assets: `wails.png`, `react.svg`, Inter font in public, bg-desktop/mobile.
- Empty feature folders without exports.
- Unused exports (knip + prefer unexported helpers).
- Staging `frontend/bindings/**` in agent commits — regenerate locally if needed; the human adds generated files.
