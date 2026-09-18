import ArchiveIcon from '@mui/icons-material/Archive';
import StorageIcon from '@mui/icons-material/Storage';
import UnarchiveIcon from '@mui/icons-material/Unarchive';
import type { GridSortModel } from '@mui/x-data-grid';
import type { KeyboardEvent, ReactElement } from 'react';
import { createElement } from 'react';
import type { FileEntry, PaneId } from '../../entities/file/types';
import { isRemotePath, parentOfVirtualPath } from '../../features/connections/helpers';
import { isTrashPath } from '../../shared/lib/trash';
import { useFolderSizeStore } from '../../features/folder-size/folderSizeStore';
import type { PaneJobKind } from '../../features/jobs/types';
import { usePaneStore } from '../../features/pane/paneStore';
import { useTerminalStore } from '../../features/terminal/terminalStore';
import { jobKindIconSx } from './styles';
import { FileTableRow } from './types';

/** Side effects that must run whenever a pane's active tab lands on a new
 * directory, from any entry point (in-tab navigate, history, tab switch,
 * new/close tab, go-to, keyboard). Kept as one function so they can't drift. */
export const enterPaneTab = (id: PaneId, path: string): void => {
  useFolderSizeStore.getState().clear(id);
  if (isRemotePath(path) && useTerminalStore.getState().isOpen(id)) {
    useTerminalStore.getState().toggle(id);
  }
};

/** Navigate `to` to the same directory as `from` (inactive pane follows source). */
const copyPaneDir = (from: PaneId, to: PaneId): void => {
  if (from === to) return;
  const s = usePaneStore.getState();
  const path = s.getPath(from);
  if (!path) return;
  enterPaneTab(to, path);
  s.navigate(to, path);
};

/** Inactive pane opens the active pane's current folder. */
export const openActiveFolderInOtherPane = (): void => {
  const s = usePaneStore.getState();
  copyPaneDir(s.activePane, s.otherPane(s.activePane));
};

/** History back/forward + enterPaneTab side effects. Returns false if stack empty. */
export const historyNav = (id: PaneId, dir: 'back' | 'forward'): boolean => {
  const store = usePaneStore.getState();
  const ok = dir === 'back' ? store.goBack(id) : store.goForward(id);
  if (ok) enterPaneTab(id, store.getPath(id));
  return ok;
};

export const jobKindIcon = (kind: PaneJobKind): ReactElement => {
  const props = { sx: jobKindIconSx };
  switch (kind) {
    case 'archive':
      return createElement(ArchiveIcon, props);
    case 'extract':
      return createElement(UnarchiveIcon, props);
    case 'sizes':
    default:
      return createElement(StorageIcon, props);
  }
};

/** Parent directory for local or remote virtual paths. */
export const parentOfPath = (path: string): string => {
  if (isTrashPath(path)) return '';
  const virtual = parentOfVirtualPath(path);
  if (virtual) return virtual;
  const parent = path.replace(/\/+$/, '').split(/[/\\]/).slice(0, -1).join('/') || '/';
  const fixed =
    path.startsWith('/') && !parent.startsWith('/') ? `/${parent}`.replace(/\/+/g, '/') : parent;
  return fixed || '/';
};

export const isNestedInSelf = (paths: string[], dest: string): boolean =>
  paths.some((p) => p === dest || dest.startsWith(p + '/') || dest.startsWith(p + '\\'));

export const sameDirPath = (a: string, b: string): boolean =>
  a.replace(/[/\\]+$/, '') === b.replace(/[/\\]+$/, '');

export const allSameParentAsDest = (paths: string[], dest: string): boolean => {
  const destNorm = dest.replace(/\/+$/, '');
  return paths.every((p) => {
    const parent = p.replace(/\/+$/, '').split(/[/\\]/).slice(0, -1).join('/') || '/';
    return parent === dest || parent === destNorm;
  });
};

/** Short tab label: basename, "host:/basename" for remotes, or "/" for a root. */
export const tabLabel = (path: string): string => {
  if (!path) return '';
  if (isRemotePath(path)) {
    const m = path.match(/^(?:ssh|smb|mega):\/\/(?:[^@/]+@)?([^/:]+)(?::\d+)?(\/.*)?$/i);
    const host = m?.[1] ?? 'ssh';
    const rest = (m?.[2] ?? '/').replace(/\/+$/, '');
    const base = rest.split('/').filter(Boolean).pop();
    return `${host}:/${base ?? ''}`;
  }
  const trimmed = path.replace(/[/\\]+$/, '');
  const base = trimmed.split(/[/\\]/).pop();
  return base || '/';
};

/** Human wording for the backend's access codes. '' means "not known". */
export const accessLabel = (access: string): { text: string; color: string; title: string } => {
  switch (access) {
    case 'full':
      return { text: 'Full', color: 'text.primary', title: 'Read and write' };
    case 'readonly':
      return { text: 'Read-only', color: 'text.secondary', title: 'Readable, not writable' };
    case 'partial':
      return {
        text: 'Partial',
        color: 'warning.main',
        title: 'Listable but not enterable, or writable without read',
      };
    case 'none':
      return { text: 'No access', color: 'error.main', title: 'Permission denied' };
    default:
      return {
        text: '—',
        color: 'text.disabled',
        title: 'Not known for remote paths — calculate folder sizes to find out',
      };
  }
};

/** Sort order for the permission column: most access first. */
const accessRank = (access: string): number =>
  ({ full: 0, readonly: 1, partial: 2, none: 3 })[access] ?? 4;

const SHORTEN_MAX = 48;

/**
 * Middle-ellipsis a path for display: keeps the first and last two segments,
 * and for remotes drops the scheme and a default `:22`/`:445` so the host stays
 * readable. Returns the input unchanged when it already fits.
 * ponytail: no `$HOME` -> `~` collapse; that needs an async GetHomeDir lookup.
 */
export const shortenPath = (path: string, max = SHORTEN_MAX): string => {
  if (!path || path.length <= max) return path;
  let prefix = '';
  let rest = path;
  const m = path.match(/^(?:ssh|smb|mega):\/\/([^/]+)(\/.*)?$/i);
  if (m) {
    prefix = `${m[1].replace(/:22$/, '').replace(/:445$/, '')}:`;
    rest = m[2] ?? '/';
  }
  const sep = rest.includes('\\') && !rest.includes('/') ? '\\' : '/';
  const lead = /^[/\\]/.test(rest) ? sep : '';
  const segs = rest.split(/[/\\]/).filter(Boolean);
  if (segs.length <= 3) return prefix + rest;
  const tail = segs.slice(-2);
  const withHead = `${prefix}${lead}${[segs[0], '…', ...tail].join(sep)}`;
  if (withHead.length <= max) return withHead;
  return `${prefix}${lead}${['…', ...tail].join(sep)}`;
};

/**
 * First row whose display name starts with the typed buffer (type-ahead).
 * When `fromPath` is set and already matches the buffer, returns the *next*
 * match after that row (wraps), so repeated same-letter presses cycle.
 */
export const findTypeAheadPath = (
  rows: FileTableRow[],
  buffer: string,
  fromPath?: string | null,
): string | null => {
  if (!buffer) return null;
  const q = buffer.toLowerCase();
  const matches = rows.filter((r) => r.name !== '..' && r.displayName.toLowerCase().startsWith(q));
  if (!matches.length) return null;
  if (!fromPath) return matches[0].path;

  const curIdx = matches.findIndex((r) => r.path === fromPath);
  if (curIdx < 0) return matches[0].path;
  return matches[(curIdx + 1) % matches.length].path;
};

/** A bare character key that should feed type-ahead (not a shortcut). */
export const isTypeAheadKey = (e: {
  key: string;
  ctrlKey: boolean;
  metaKey: boolean;
  altKey: boolean;
}): boolean => e.key.length === 1 && e.key !== ' ' && !e.ctrlKey && !e.metaKey && !e.altKey;

/** Coerce Wails/JSON map values (number | numeric string | bigint) into a plain Record. */
export const mapChildSizes = (
  map: { [key: string]: number | string | undefined } | null | undefined,
): Record<string, number> => {
  const sizes: Record<string, number> = {};
  if (!map) return sizes;
  for (const [k, v] of Object.entries(map)) {
    if (typeof v === 'number' && Number.isFinite(v)) {
      sizes[k] = v;
      continue;
    }
    if (typeof v === 'string' && v.trim() !== '') {
      const n = Number(v);
      if (Number.isFinite(n)) sizes[k] = n;
      continue;
    }
    // bigint (some bridges) — use loose check so we don't need ES2020 lib target
    if (typeof v === 'bigint') {
      sizes[k] = Number(v);
    }
  }
  return sizes;
};

/**
 * Look up a calculated folder size for a row. Prefers exact path match (ListDir
 * key), then suffix/basename match when bridges or path normalization drift.
 */
export const lookupFolderSize = (
  folderSizes: Record<string, number> | undefined,
  path: string,
  name?: string,
): number | undefined => {
  if (!folderSizes) return undefined;
  const direct = folderSizes[path];
  if (direct != null) return direct;
  const base = name || path.split(/[/\\]/).filter(Boolean).pop() || '';
  if (!base) return undefined;
  if (folderSizes[base] != null) return folderSizes[base];
  const slashSuffix = `/${base}`;
  const bslashSuffix = `\\${base}`;
  for (const [k, v] of Object.entries(folderSizes)) {
    if (k.endsWith(slashSuffix) || k.endsWith(bslashSuffix) || k === base) return v;
  }
  return undefined;
};

export const displayName = (e: FileEntry, showExtensions: boolean): string => {
  if (e.isDir || showExtensions || e.name === '..') return e.name;
  const i = e.name.lastIndexOf('.');
  if (i <= 0) return e.name;
  return e.name.slice(0, i);
};

export const sizeValue = (e: FileEntry, folderSizes?: Record<string, number>): number => {
  if (e.isDir) {
    const n = lookupFolderSize(folderSizes, e.path, e.name);
    if (n != null) return n;
  }
  return e.size;
};

export const typeValue = (e: FileEntry): string => {
  return e.isDir ? 'dir' : e.ext || 'file';
};

const compareRows = (
  a: FileTableRow,
  b: FileTableRow,
  field: string,
  folderSizes?: Record<string, number>,
): number => {
  switch (field) {
    case 'displayName': {
      const an = a.displayName.toLowerCase();
      const bn = b.displayName.toLowerCase();
      if (an < bn) return -1;
      if (an > bn) return 1;
      return 0;
    }
    case 'size':
      return sizeValue(a, folderSizes) - sizeValue(b, folderSizes);
    case 'modTime':
      return a.modTime - b.modTime;
    case 'ext':
      return typeValue(a).localeCompare(typeValue(b));
    case 'access':
      return accessRank(a.access) - accessRank(b.access);
    default:
      return a.displayName.localeCompare(b.displayName);
  }
};

/**
 * Sort rows with fixed hierarchy:
 * 1. `..` always first
 * 2. folders (sorted by active column)
 * 3. files (sorted by active column)
 */
export const sortRowsPinParent = (
  rows: FileTableRow[],
  sortModel: GridSortModel,
  folderSizes?: Record<string, number>,
): FileTableRow[] => {
  const parent = rows.find((r) => r.name === '..');
  const rest = rows.filter((r) => r.name !== '..');
  const dirs = rest.filter((r) => r.isDir);
  const files = rest.filter((r) => !r.isDir);
  const sort = sortModel[0];
  if (sort?.field) {
    const dir = sort.sort === 'desc' ? -1 : 1;
    const cmp = (a: FileTableRow, b: FileTableRow) =>
      dir * compareRows(a, b, sort.field, folderSizes);
    dirs.sort(cmp);
    files.sort(cmp);
  } else {
    // Stable default: name asc within each group
    const byName = (a: FileTableRow, b: FileTableRow) =>
      compareRows(a, b, 'displayName', folderSizes);
    dirs.sort(byName);
    files.sort(byName);
  }
  return parent ? [parent, ...dirs, ...files] : [...dirs, ...files];
};

export const isCellKeyboardEvent = (key: KeyboardEvent['key'], code: KeyboardEvent['code']) =>
  key === 'ArrowDown' ||
  key === 'ArrowUp' ||
  key === 'ArrowLeft' ||
  key === 'ArrowRight' ||
  key === 'Enter' ||
  key === 'Home' ||
  key === 'End' ||
  key === ' ' ||
  code === 'Space';
