import type { PaneId } from '../../entities/file/types';
import { parentOfVirtualPath } from '../../features/connections/helpers';
import type { FileOpsAction } from '../../features/file-ops/types';
import { isTrashPath } from '../../shared/lib/trash';
import type { ToolbarRequestHandlers } from './types';

export type { ToolbarRequestHandlers } from './types';

// --- errors ---

export const isPermissionError = (msg: string): boolean => {
  const m = msg.toLowerCase();
  return m.includes('permission denied') || m.includes('cannot delete');
};

// --- paths ---

/** Parent directory for local or remote virtual paths. */
export const parentPath = (activePath: string): string => {
  if (isTrashPath(activePath)) return '';
  const virtual = parentOfVirtualPath(activePath);
  if (virtual) return virtual;
  const parent = activePath.replace(/\/+$/, '').split(/[/\\]/).slice(0, -1).join('/') || '/';
  const fixed =
    activePath.startsWith('/') && !parent.startsWith('/')
      ? `/${parent}`.replace(/\/+/g, '/')
      : parent;
  return fixed || '/';
};

/** Multi-select for actions: selection if any, else focus; never includes `..`. */
export const resolveActionPaths = (selection: string[], focus: string): string[] => {
  const sel = selection.filter((p) => p.split(/[/\\]/).pop() !== '..');
  if (sel.length) return sel;
  if (focus && focus.split(/[/\\]/).pop() !== '..') return [focus];
  return [];
};

// --- keyboard / menu request map ---

/**
 * Build a complete handlers map for toolbar requests.
 * Missing handlers become no-ops so lookup is always safe.
 */
export const createToolbarRequestHandlers = (
  partial: Partial<ToolbarRequestHandlers>,
): ToolbarRequestHandlers => {
  const noop = () => undefined;
  const all: FileOpsAction[] = [
    'copy',
    'paste',
    'move',
    'delete',
    'deletePermanent',
    'restoreTrash',
    'emptyTrash',
    'rename',
    'mkdir',
    'mkfile',
    'editFile',
    'gitDiff',
    'goTo',
    'refresh',
    'goParent',
    'goHome',
    'goBack',
    'goForward',
    'calcSizes',
    'archive',
    'extract',
  ];
  const handlers = {} as ToolbarRequestHandlers;
  for (const key of all) {
    handlers[key] = partial[key] ?? noop;
  }
  return handlers;
};

/** Run the handler for a request (map lookup — no switch). */
export const runToolbarRequest = (
  request: FileOpsAction,
  handlers: ToolbarRequestHandlers,
): void => {
  handlers[request]();
};

/** Click the per-pane folder-sizes control (lives on pane header). */
export const triggerCalcSizes = (pane: PaneId): void => {
  const btn = document.querySelector(`[data-testid="btn-folder-sizes-${pane}"]`);
  if (btn instanceof HTMLElement) {
    btn?.click();
  }
};
