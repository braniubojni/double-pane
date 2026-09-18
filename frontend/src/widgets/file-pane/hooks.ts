import { useQueryClient } from '@tanstack/react-query';
import { useGridApiRef, type GridColumnVisibilityModel } from '@mui/x-data-grid';
import type {
  GridColDef,
  GridRowClassNameParams,
  GridRowParams,
  GridSortModel,
} from '@mui/x-data-grid/models';
import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  type KeyboardEvent,
  type MouseEvent,
} from 'react';
import { useDrop } from 'react-dnd';
import {
  useDirListing,
  useGitDirStatus,
  useHomeDir,
  useSettings,
} from '../../entities/file/queries';
import type { FileEntry, PaneId } from '../../entities/file/types';
import { isRemotePath } from '../../features/connections/helpers';
import { useEditorStore } from '../../features/editor/editorStore';
import { useContextMenuStore } from '../../features/file-ops/contextMenuStore';
import { useFolderSizeStore } from '../../features/folder-size/folderSizeStore';
import { newJobId, usePaneJobStore } from '../../features/jobs/paneJobStore';
import { usePaneStore } from '../../features/pane/paneStore';
import { useTerminalStore } from '../../features/terminal/terminalStore';
import { startTransfer } from '../../features/transfers/startTransfer';
import { useDestRowUpdates } from '../../features/transfers/useDestRowUpdates';
import { useColumnStore } from '../../features/ui/columnStore';
import { useGridPrefsStore } from '../../features/ui/gridPrefsStore';
import { useDmgPasswordStore } from '../../features/dmg/dmgPasswordStore';
import { startAttachDmg } from '../../features/dmg/startAttach';
import { isBrowsableArchive } from '../../shared/lib/archives';
import { isTrashPath } from '../../shared/lib/trash';
import { errMessage } from '../../shared/lib/format';
import { openDocument } from '../../shared/lib/openDocument';
import { FileService } from '../../shared/api/bindings';
import { useSnack } from '../../shared/ui/SnackbarHost';
import { getColumns } from './consts';
import { setDropValidity } from '../../features/dnd/dragState';
import { refreshAfterDrop } from '../../features/dnd/refreshAfterDrop';
import { FILE_ROW_ITEM, dropModeForDrag } from './dnd';
import {
  allSameParentAsDest,
  displayName,
  enterPaneTab,
  findTypeAheadPath,
  isCellKeyboardEvent,
  isNestedInSelf,
  isTypeAheadKey,
  lookupFolderSize,
  mapChildSizes,
  parentOfPath,
  sameDirPath,
  sortRowsPinParent,
} from './helpers';
import type { DragPayload, FileTableProps } from './types';

const TYPE_AHEAD_RESET_MS = 800;

type GridKeyboardHelpers = {
  __gfmMoveFocus?: (d: number, extend?: boolean) => void;
  __gfmOpenFocused?: () => void;
  __gfmOpenDir?: () => void;
  __gfmToggleMulti?: () => void;
  __gfmSelectAll?: () => void;
  __gfmFocusHome?: () => void;
  __gfmFocusEnd?: () => void;
};

/** Typing inside an embedded input must not reach grid navigation. */
const isEditingTarget = (target: EventTarget | null): boolean => {
  const el = target as HTMLElement | null;
  if (!el) return false;
  return el.tagName === 'INPUT' || el.tagName === 'TEXTAREA' || Boolean(el.isContentEditable);
};

export const useFilePane = (id: PaneId) => {
  const path = usePaneStore((s) => s.getPath(id));
  const tabs = usePaneStore((s) => s.getTabs(id));
  const tabIndex = usePaneStore((s) => s.getTabIndex(id));
  const selection = usePaneStore((s) => (id === 'left' ? s.leftSelection : s.rightSelection));
  const focused = usePaneStore((s) => (id === 'left' ? s.leftFocus : s.rightFocus));
  const active = usePaneStore((s) => s.activePane === id);
  const navigateStore = usePaneStore((s) => s.navigate);
  const setActivePane = usePaneStore((s) => s.setActivePane);
  const setSelection = usePaneStore((s) => s.setSelection);
  const setFocus = usePaneStore((s) => s.setFocus);
  const toggleMultiSelect = usePaneStore((s) => s.toggleMultiSelect);
  const selectRange = usePaneStore((s) => s.selectRange);
  const clearSelection = usePaneStore((s) => s.clearSelection);
  const addTab = usePaneStore((s) => s.addTab);
  const closeTab = usePaneStore((s) => s.closeTab);
  const selectTab = usePaneStore((s) => s.selectTab);
  const { data: home } = useHomeDir();
  const { data: settings } = useSettings();
  const showHidden = settings?.showHidden ?? false;
  const showExtensions = settings?.showExtensions ?? true;
  const showGitStatus = settings?.showGitStatus !== false;
  const listing = useDirListing(path || undefined, showHidden);
  const gitStatus = useGitDirStatus(path || undefined, showGitStatus);
  const gitByName = useMemo(() => {
    const m = new Map<string, string>();
    for (const e of gitStatus.data?.entries ?? []) {
      if (e.name) m.set(e.name, e.status);
    }
    return m;
  }, [gitStatus.data]);
  const show = useSnack((s) => s.show);
  const qc = useQueryClient();

  const terminalOpen = useTerminalStore((s) => s.isOpen(id));
  const terminalHeight = useTerminalStore((s) => s.height);
  const toggleTerminal = useTerminalStore((s) => s.toggle);

  const folderSizes = useFolderSizeStore((s) => s.getSizes(id));
  const deniedPaths = useFolderSizeStore((s) => s.getDenied(id));
  const beginSizes = useFolderSizeStore((s) => s.begin);
  const finishSizes = useFolderSizeStore((s) => s.finish);
  const failSizes = useFolderSizeStore((s) => s.fail);

  const job = usePaneJobStore((s) => s.getJob(id));
  const startJob = usePaneJobStore((s) => s.start);
  const finishJob = usePaneJobStore((s) => s.finish);
  const clearJob = usePaneJobStore((s) => s.clear);

  const navigate = (next: string) => {
    void FileService.Exists(next)
      .then((ok) => {
        if (!ok) {
          show(`Path not found: ${next}`, 'error');
          return;
        }
        enterPaneTab(id, next);
        navigateStore(id, next);
      })
      .catch((e) => show(errMessage(e), 'error'));
  };

  const onSelectTab = (tabId: string) => {
    const tab = tabs.find((t) => t.id === tabId);
    if (tab) enterPaneTab(id, tab.path);
    selectTab(id, tabId);
  };

  const onAddTab = () => {
    enterPaneTab(id, path);
    addTab(id, path);
  };

  const onCloseTab = (tabId: string) => {
    // Closing the active tab changes this pane's active directory — run the
    // same side effects navigate/selectTab do, for the tab that becomes active.
    if (tabId === tabs[tabIndex]?.id && tabs.length > 1) {
      const remaining = tabs.filter((t) => t.id !== tabId);
      const nextIdx = Math.min(Math.max(tabIndex - 1, 0), remaining.length - 1);
      enterPaneTab(id, remaining[nextIdx].path);
    }
    closeTab(id, tabId);
  };

  const goUp = () => {
    if (!path) return;
    if (isTrashPath(path)) {
      goHome();
      return;
    }
    const parent = parentOfPath(path);
    if (parent) navigate(parent);
    else goHome();
  };

  const goHome = () => {
    if (home) navigate(home);
  };

  const openWorkspace = useEditorStore((s) => s.openWorkspace);

  const openEntry = (entry: FileEntry) => {
    if (entry.isDir) {
      if (isTrashPath(path) && entry.name !== '..') return;
      navigate(entry.path);
      return;
    }
    const ext = (entry.ext || entry.name.split('.').pop() || '').toLowerCase();
    if (!isRemotePath(entry.path) && isBrowsableArchive(entry.name, ext)) {
      navigate(entry.path);
      return;
    }
    if (ext === 'dmg' && !isRemotePath(entry.path)) {
      const prompt = useDmgPasswordStore.getState().prompt;
      void FileService.IsEncryptedDiskImage(entry.path)
        .then((encrypted) => {
          if (encrypted) {
            prompt(entry.path, id);
            return;
          }
          startAttachDmg({
            path: entry.path,
            show,
            onMounted: (mp) => navigate(mp),
            onNeedPassword: (p, err) => prompt(p, id, err),
          });
        })
        .catch((e) => show(errMessage(e), 'error'));
      return;
    }
    openDocument({
      path: entry.path,
      name: entry.name,
      ext,
      useBuiltInEditor: settings?.useBuiltInEditor !== false,
      openWorkspace,
      show,
    });
  };

  const onDropPaths = (
    paths: string[],
    destDir: string,
    _sourcePane: PaneId,
    mode: 'copy' | 'move',
  ) => {
    const dest = destDir || path;
    if (!dest || !paths.length) return;
    if (isNestedInSelf(paths, dest)) {
      show(`Cannot ${mode} a folder into itself`, 'warning');
      return;
    }
    // Dropping back into the same parent dir = cancel (user regretted the drag).
    // Applies to both copy and move so we never create "name (1)" self-duplicates.
    if (allSameParentAsDest(paths, dest)) return;
    startTransfer({
      kind: mode,
      sources: paths,
      destDir: dest,
      show,
      onSuccess: () => {
        clearSelection();
        setActivePane(id);
        if (!sameDirPath(dest, path)) {
          navigate(dest);
        }
      },
      onSettled: () => refreshAfterDrop(qc, dest, paths),
    });
  };

  const cancelJob = () => {
    if (!job) return;
    if (job.backendJobId) {
      void FileService.CancelJob(job.backendJobId).catch(() => undefined);
    }
    clearJob(id, job.id);
    show('Cancelled', 'info');
  };

  const onRefresh = () => {
    setActivePane(id);
    if (!path) return;
    void qc.invalidateQueries({ queryKey: ['dir', path] });
    void qc.invalidateQueries({ queryKey: ['gitStatus', path] });
  };

  const onCalcSizes = () => {
    if (!path) return;
    setActivePane(id);
    const gen = beginSizes(id);
    const uiJobId = newJobId('sizes');
    void FileService.NewJobID()
      .catch(() => '')
      .then((backendJobId: string) => {
        startJob(id, {
          id: uiJobId,
          kind: 'sizes',
          label: 'Calculating folder sizes…',
          cancelable: true,
          backendJobId: backendJobId || undefined,
        });
        return FileService.DirChildSizes(backendJobId || '', path)
          .then((res) => {
            const denied = res.denied ?? [];
            const sizes = mapChildSizes(res.sizes);
            // Dev diagnostics: path keys must match ListDir entry.path.
            console.debug('[gfm] DirChildSizes', {
              path,
              sizeKeys: Object.keys(sizes).slice(0, 8),
              sizeCount: Object.keys(sizes).length,
              deniedCount: denied.length,
              sample: res,
            });
            finishSizes(id, gen, sizes, denied);
            finishJob(id, uiJobId);
            const n = Object.keys(sizes).length;
            if (n === 0 && denied.length === 0) {
              show('Folder sizes calculated — no subfolders found', 'info');
            } else {
              show(
                denied.length
                  ? `Folder sizes calculated — ${denied.length} folder(s) not fully readable`
                  : `Folder sizes calculated (${n})`,
                denied.length ? 'warning' : 'success',
              );
            }
          })
          .catch((e) => {
            failSizes(id, gen);
            finishJob(id, uiJobId);
            const msg = errMessage(e);
            if (msg.toLowerCase().includes('cancel') || msg.includes('context canceled')) {
              show('Cancelled', 'info');
              return;
            }
            show(msg, 'error');
          });
      });
  };

  const activatePane = () => setActivePane(id);

  return {
    path,
    tabs,
    activeTabId: tabs[tabIndex]?.id ?? '',
    onSelectTab,
    onAddTab,
    onCloseTab,
    selection,
    focused,
    active,
    showExtensions,
    gitByName,
    listing,
    terminalOpen,
    terminalHeight,
    folderSizes,
    deniedPaths,
    job,
    navigate,
    goUp,
    goHome,
    openEntry,
    onDropPaths,
    cancelJob,
    onCalcSizes,
    onRefresh,
    activatePane,
    setSelection,
    setFocus,
    toggleMultiSelect,
    selectRange,
    setActivePane,
    toggleTerminal,
  };
};

export const useFileTable = ({
  paneId,
  panePath,
  entries,
  isLoading,
  isError,
  errorMessage,
  selected,
  focused,
  active,
  showExtensions,
  gitByName,
  folderSizes,
  deniedPaths,
  onSelect,
  onFocus,
  onToggleMulti,
  onSelectRange,
  onActivate,
  onOpen,
  onDropPaths,
  onSortedPathsChange,
}: FileTableProps) => {
  const apiRef = useGridApiRef();
  const wrapRef = useRef<HTMLDivElement | null>(null);
  const showContextMenu = useContextMenuStore((s) => s.show);
  const widths = useColumnStore((s) => s.widths);
  const setWidth = useColumnStore((s) => s.setWidth);
  const prefs = useGridPrefsStore((s) => s[paneId]);
  const setSort = useGridPrefsStore((s) => s.setSort);
  const setHidden = useGridPrefsStore((s) => s.setHidden);
  const sortModel = useMemo<GridSortModel>(
    () => [{ field: prefs.sortField, sort: prefs.sortDir }],
    [prefs.sortField, prefs.sortDir],
  );
  const columnVisibilityModel = useMemo<GridColumnVisibilityModel>(() => {
    const m: GridColumnVisibilityModel = {};
    for (const f of prefs.hidden) m[f] = false;
    return m;
  }, [prefs.hidden]);
  const onSortModelChange = useCallback(
    (model: GridSortModel) => {
      const s = model[0];
      setSort(paneId, s?.field || 'displayName', s?.sort === 'desc' ? 'desc' : 'asc');
    },
    [paneId, setSort],
  );
  const onColumnVisibilityModelChange = useCallback(
    (model: GridColumnVisibilityModel) => {
      setHidden(
        paneId,
        Object.entries(model)
          .filter(([, visible]) => visible === false)
          .map(([field]) => field),
      );
    },
    [paneId, setHidden],
  );

  // Fallback drop target: dropping on empty pane space (not on a folder row)
  // lands in the pane's current directory. Row-level drops are handled by
  // FileGridRow (see dnd.tsx); monitor.didDrop() here skips this when a row
  // above already consumed the drop.
  const [{ isOver: dropActive, isOverShallow, canDropPane }, dropRef] = useDrop<
    DragPayload,
    void,
    { isOver: boolean; isOverShallow: boolean; canDropPane: boolean }
  >(
    () => ({
      accept: FILE_ROW_ITEM,
      canDrop: (item) =>
        Boolean(panePath) &&
        !isNestedInSelf(item.paths, panePath) &&
        !allSameParentAsDest(item.paths, panePath),
      drop: (item, monitor) => {
        if (monitor.didDrop()) return;
        onDropPaths(item.paths, panePath, item.sourcePane, dropModeForDrag());
      },
      collect: (monitor) => ({
        isOver: monitor.isOver({ shallow: false }) && monitor.canDrop(),
        isOverShallow: monitor.isOver({ shallow: true }),
        canDropPane: monitor.canDrop(),
      }),
    }),
    [panePath, onDropPaths],
  );

  // Empty-space / chrome hover: show drop-allowed for valid cross-pane drops
  // (e.g. remote → local) instead of stuck no-drop from a prior file-row hover.
  useEffect(() => {
    if (isOverShallow) setDropValidity(canDropPane);
  }, [isOverShallow, canDropPane]);

  const setWrapRef = useCallback(
    (el: HTMLDivElement | null) => {
      wrapRef.current = el;
      dropRef(el);
    },
    [dropRef],
  );

  const baseRows = useMemo(
    () =>
      (entries ?? []).map((e) => {
        const folderSizeBytes =
          e.isDir && e.name !== '..' ? lookupFolderSize(folderSizes, e.path, e.name) : undefined;
        return {
          ...e,
          id: e.path,
          displayName: displayName(e, showExtensions),
          // Stamp size onto the row so DataGrid re-renders when calculation finishes.
          ...(folderSizeBytes != null ? { folderSizeBytes } : {}),
        };
      }),
    [entries, showExtensions, folderSizes],
  );

  const rows = useMemo(
    () => sortRowsPinParent(baseRows, sortModel, folderSizes),
    [baseRows, sortModel, folderSizes],
  );
  useDestRowUpdates(apiRef, panePath, rows.length);

  const orderedPaths = useMemo(() => rows.map((r) => r.path), [rows]);

  const columns = useMemo<GridColDef[]>(
    () =>
      getColumns(widths, selected, folderSizes, deniedPaths, prefs.order, isTrashPath(panePath)),
    [widths, folderSizes, selected, deniedPaths, prefs.order, panePath],
  );

  const moveFocus = useCallback(
    (delta: number, extend = false) => {
      if (!rows.length) return;
      const ids = rows.map((r) => r.path);
      let idx = focused ? ids.indexOf(focused) : -1;
      if (idx < 0) {
        idx = delta > 0 ? 0 : ids.length - 1;
      } else {
        idx = Math.max(0, Math.min(ids.length - 1, idx + delta));
      }
      const next = ids[idx];
      if (extend) {
        onFocus(next, { keepAnchor: true });
        onSelectRange(ids, next);
      } else {
        onFocus(next);
      }
      try {
        apiRef.current?.scrollToIndexes?.({ rowIndex: idx });
      } catch {
        /* ignore */
      }
    },
    [rows, focused, onFocus, onSelectRange, apiRef],
  );

  // Type-ahead: bare letters move the cursor (highlight only, never selection).
  const typeAhead = useRef({ buffer: '', timer: 0 });
  useEffect(() => () => window.clearTimeout(typeAhead.current.timer), []);

  const typeAheadJump = useCallback(
    (key: string) => {
      const ta = typeAhead.current;
      window.clearTimeout(ta.timer);
      const ch = key.toLowerCase();
      // Same letter again while buffer is that single char → cycle matches
      // (Double Commander / Explorer style). Otherwise append for multi-char prefix.
      const cycling = ta.buffer === ch;
      if (!cycling) ta.buffer += ch;
      ta.timer = window.setTimeout(() => {
        ta.buffer = '';
      }, TYPE_AHEAD_RESET_MS);
      const path = findTypeAheadPath(rows, ta.buffer, cycling ? focused : null);
      if (!path) return;
      onFocus(path);
      try {
        apiRef.current?.scrollToIndexes?.({ rowIndex: rows.findIndex((r) => r.path === path) });
      } catch {
        /* ignore */
      }
    },
    [rows, onFocus, apiRef, focused],
  );

  const openFocused = useCallback(() => {
    if (!focused) return;
    const entry = rows.find((r) => r.path === focused);
    if (entry) onOpen(entry);
  }, [focused, rows, onOpen]);

  const openFocusedDirOnly = useCallback(() => {
    if (!focused) return;
    const entry = rows.find((r) => r.path === focused);
    if (entry?.isDir) onOpen(entry);
  }, [focused, rows, onOpen]);

  const shouldStealFocus = useCallback(() => {
    const ae = document.activeElement as HTMLElement | null;
    if (!ae) return true;
    // Never steal from path bar, dialogs, menus, or any editable field
    if (ae.closest?.(`[data-testid="path-input-${paneId}"]`)) return false;
    if (ae.closest?.('[role="dialog"]')) return false;
    if (ae.closest?.('[role="menu"]')) return false;
    if (ae.closest?.('.xterm')) return false;
    const tag = ae.tagName;
    if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return false;
    if (ae.isContentEditable) return false;
    return true;
  }, [paneId]);

  const focusGrid = useCallback(() => {
    if (!shouldStealFocus()) return;
    const el = wrapRef.current;
    if (!el) return;
    const activeEl = document.activeElement as HTMLElement | null;
    if (activeEl && el.contains(activeEl) && activeEl !== el) {
      activeEl.blur();
    }
    el.focus({ preventScroll: true });
  }, [shouldStealFocus]);

  const handleKeys = useCallback(
    (e: KeyboardEvent) => {
      if (isEditingTarget(e.target)) return;
      if (e.key === 'ArrowDown') {
        e.preventDefault();
        e.stopPropagation();
        onActivate();
        moveFocus(1, e.shiftKey);
      } else if (e.key === 'ArrowUp') {
        e.preventDefault();
        e.stopPropagation();
        onActivate();
        moveFocus(-1, e.shiftKey);
      } else if (e.key === 'ArrowLeft') {
        // Handled at window level for history back; prevent scroll
        e.preventDefault();
      } else if (e.key === 'ArrowRight') {
        e.preventDefault();
        e.stopPropagation();
        onActivate();
        openFocusedDirOnly();
      } else if (e.key === 'Home') {
        e.preventDefault();
        if (rows.length) onFocus(rows[0].path);
      } else if (e.key === 'End') {
        e.preventDefault();
        if (rows.length) onFocus(rows[rows.length - 1].path);
      } else if (e.key === 'Enter') {
        e.preventDefault();
        openFocused();
      } else if (e.key === ' ' || e.code === 'Space') {
        e.preventDefault();
        e.stopPropagation();
        if (focused) onToggleMulti(focused);
      } else if (isTypeAheadKey(e)) {
        e.preventDefault();
        e.stopPropagation();
        onActivate();
        typeAheadJump(e.key);
      }
    },
    [
      onActivate,
      moveFocus,
      openFocusedDirOnly,
      openFocused,
      rows,
      onFocus,
      focused,
      onToggleMulti,
      typeAheadJump,
    ],
  );

  /**
   * Right-click anywhere in the pane. One handler on the wrapper rather than a
   * per-row prop: the clicked row is recovered from the DataGrid's `data-id`.
   * A row that is not already selected becomes the selection first, so the
   * menu's actions target what was actually clicked.
   */
  const onContextMenu = useCallback(
    (e: MouseEvent) => {
      e.preventDefault();
      e.stopPropagation();
      onActivate();
      const rowEl = (e.target as HTMLElement).closest?.('.MuiDataGrid-row');
      const rowPath = rowEl?.getAttribute('data-id') ?? '';
      const row = rows.find((r) => r.path === rowPath && r.name !== '..') ?? null;
      if (row) {
        onFocus(row.path);
        if (!selected.includes(row.path)) onSelect([row.path]);
      }
      showContextMenu({
        x: e.clientX,
        y: e.clientY,
        paneId,
        panePath,
        entry: row,
      });
    },
    [onActivate, rows, onFocus, onSelect, selected, showContextMenu, paneId, panePath],
  );

  const getRowClassName = useCallback(
    (params: GridRowClassNameParams) => {
      const classes: string[] = [];
      if (params.id === focused) classes.push('row-focused');
      if (selected.includes(String(params.id))) classes.push('row-selected');
      const name = (params.row as { name?: string }).name;
      const st = name && gitByName?.get(name);
      if (st === 'M') classes.push('git-M');
      else if (st === 'A') classes.push('git-A');
      else if (st === 'D') classes.push('git-D');
      else if (st === 'U') classes.push('git-U');
      else if (st === '?') classes.push('git-untracked');
      return classes.join(' ');
    },
    [focused, selected, gitByName],
  );

  const onColumnWidthChange = useCallback(
    (params: { colDef: { field?: string }; width: number }) => {
      if (params.colDef.field) setWidth(params.colDef.field, params.width);
    },
    [setWidth],
  );

  const onCellKeyDown = useCallback(
    (
      _params: unknown,
      event: {
        key: string;
        code: string;
        shiftKey: boolean;
        ctrlKey: boolean;
        metaKey: boolean;
        altKey: boolean;
        target?: EventTarget | null;
        defaultMuiPrevented?: boolean;
        preventDefault: () => void;
        stopPropagation: () => void;
      },
    ) => {
      if (isEditingTarget(event.target ?? null)) return;
      if (!isCellKeyboardEvent(event.key, event.code)) {
        if (isTypeAheadKey(event)) {
          event.defaultMuiPrevented = true;
          event.preventDefault();
          event.stopPropagation();
          typeAheadJump(event.key);
        }
        return;
      }
      event.defaultMuiPrevented = true;
      event.preventDefault();
      event.stopPropagation();
      if (event.key === 'ArrowDown') moveFocus(1, event.shiftKey);
      else if (event.key === 'ArrowUp') moveFocus(-1, event.shiftKey);
      else if (event.key === 'ArrowRight') openFocusedDirOnly();
      else if (event.key === 'Enter') openFocused();
      else if (event.key === 'Home' && rows.length) onFocus(rows[0].path);
      else if (event.key === 'End' && rows.length) onFocus(rows[rows.length - 1].path);
      else if (event.key === ' ' || event.code === 'Space') {
        if (focused) onToggleMulti(focused);
      }
      // ArrowLeft: let window handler do history back
      focusGrid();
    },
    [
      moveFocus,
      openFocusedDirOnly,
      openFocused,
      rows,
      onFocus,
      focused,
      onToggleMulti,
      focusGrid,
      typeAheadJump,
    ],
  );

  const onRowClick = useCallback(
    (params: { id: string | number }, event: MouseEvent) => {
      onActivate();
      const path = String(params.id);
      if (event.shiftKey) {
        onFocus(path, { keepAnchor: true });
        onSelectRange(orderedPaths, path);
        return;
      }
      if (event.metaKey || event.ctrlKey) {
        onFocus(path);
        onToggleMulti(path);
        return;
      }
      onFocus(path);
      onSelect([path]);
    },
    [onActivate, onFocus, onSelectRange, orderedPaths, onToggleMulti, onSelect],
  );

  const onRowDoubleClick = useCallback(
    (params: GridRowParams) => {
      onOpen(params.row as FileEntry);
      window.getSelection()?.removeAllRanges();
    },
    [onOpen],
  );

  useEffect(() => {
    onSortedPathsChange?.(orderedPaths);
  }, [orderedPaths, onSortedPathsChange]);

  // Focus grid when this pane becomes active — but never while typing in path/dialogs
  useEffect(() => {
    if (!active) return;
    if (!shouldStealFocus()) return;
    focusGrid();
  }, [active, focusGrid, shouldStealFocus]);

  // After listing loads, restore grid focus only if nothing editable is focused
  useEffect(() => {
    if (!active || isLoading || !entries) return;
    if (!shouldStealFocus()) return;
    let cancelled = false;
    const run = () => {
      if (!cancelled && shouldStealFocus()) focusGrid();
    };
    run();
    const t1 = window.setTimeout(run, 0);
    return () => {
      cancelled = true;
      window.clearTimeout(t1);
    };
  }, [active, isLoading, entries, focusGrid, shouldStealFocus]);

  // Expose keyboard helpers on the wrapper for window-level nav
  useEffect(() => {
    const el = wrapRef.current as (HTMLDivElement & GridKeyboardHelpers) | null;
    if (!el) return;
    el.__gfmMoveFocus = moveFocus;
    el.__gfmOpenFocused = openFocused;
    el.__gfmOpenDir = openFocusedDirOnly;
    el.__gfmToggleMulti = () => {
      if (focused) onToggleMulti(focused);
    };
    el.__gfmSelectAll = () => {
      onSelect(rows.filter((r) => r.name !== '..').map((r) => r.path));
    };
    el.__gfmFocusHome = () => {
      if (rows.length) onFocus(rows[0].path);
    };
    el.__gfmFocusEnd = () => {
      if (rows.length) onFocus(rows[rows.length - 1].path);
    };
  }, [moveFocus, openFocused, openFocusedDirOnly, focused, onToggleMulti, onSelect, rows, onFocus]);

  return {
    paneId,
    active,
    dropActive,
    isLoading,
    isError,
    errorMessage,
    entries,
    selected,
    onActivate,
    onDropPaths,
    setWrapRef,
    handleKeys,
    apiRef,
    rows,
    columns,
    sortModel,
    onSortModelChange,
    columnVisibilityModel,
    onColumnVisibilityModelChange,
    getRowClassName,
    onColumnWidthChange,
    onCellKeyDown,
    onRowClick,
    onRowDoubleClick,
    onContextMenu,
  };
};
