import { create } from 'zustand';
import type { PaneId } from '../../entities/file/types';

type SortDir = 'asc' | 'desc';

export type PaneGridPrefs = {
  sortField: string;
  sortDir: SortDir;
  hidden: string[];
  order: string[];
};

const DEFAULT_COLUMN_ORDER = ['icon', 'displayName', 'size', 'modTime', 'ext', 'access'] as const;

const NEVER_HIDE = new Set(['icon', 'displayName']);

export const defaultPaneGridPrefs = (): PaneGridPrefs => ({
  sortField: 'displayName',
  sortDir: 'asc',
  hidden: [],
  order: [],
});

type PaneGridPrefsInput = {
  sortField?: string;
  sortDir?: string;
  hidden?: string[] | null;
  order?: string[] | null;
};

const normalizePaneGridPrefs = (p?: PaneGridPrefsInput | null): PaneGridPrefs => ({
  sortField: p?.sortField || 'displayName',
  sortDir: p?.sortDir === 'desc' ? 'desc' : 'asc',
  hidden: (p?.hidden ?? []).filter((f) => f && !NEVER_HIDE.has(f)),
  order: p?.order ?? [],
});

/** Reorder column defs from prefs.order. Empty order keeps `cols` as given.
 * Icon stays first unless `order` names it. Unknown fields are appended. */
export const orderColumns = <T extends { field: string }>(cols: T[], order: string[]): T[] => {
  if (!order.length) return cols;
  const byField = new Map(cols.map((c) => [c.field, c]));
  const out: T[] = [];
  const used = new Set<string>();
  if (!order.includes('icon')) {
    const icon = byField.get('icon');
    if (icon) {
      out.push(icon);
      used.add('icon');
    }
  }
  for (const f of order) {
    const col = byField.get(f);
    if (col && !used.has(f)) {
      out.push(col);
      used.add(f);
    }
  }
  for (const col of cols) {
    if (!used.has(col.field)) out.push(col);
  }
  return out;
};

export const columnVisualOrder = (order: string[]): string[] =>
  orderColumns(
    DEFAULT_COLUMN_ORDER.map((field) => ({ field })),
    order,
  ).map((c) => c.field);

interface GridPrefsState {
  left: PaneGridPrefs;
  right: PaneGridPrefs;
  loaded: boolean;
  hydrate: (prefs: { left?: PaneGridPrefsInput | null; right?: PaneGridPrefsInput | null }) => void;
  setSort: (pane: PaneId, field: string, dir: SortDir) => void;
  setHidden: (pane: PaneId, hidden: string[]) => void;
  moveColumn: (pane: PaneId, field: string, dir: -1 | 1) => void;
}

export const useGridPrefsStore = create<GridPrefsState>((set) => ({
  left: defaultPaneGridPrefs(),
  right: defaultPaneGridPrefs(),
  loaded: false,
  hydrate: (prefs) =>
    set({
      left: normalizePaneGridPrefs(prefs.left),
      right: normalizePaneGridPrefs(prefs.right),
      loaded: true,
    }),
  setSort: (pane, field, dir) =>
    set((s) => {
      const cur = s[pane];
      if (cur.sortField === field && cur.sortDir === dir) return s;
      return { ...s, [pane]: { ...cur, sortField: field, sortDir: dir } };
    }),
  setHidden: (pane, hidden) =>
    set((s) => {
      const next = hidden.filter((f) => f && !NEVER_HIDE.has(f));
      const cur = s[pane];
      if (cur.hidden.length === next.length && cur.hidden.every((f, i) => f === next[i])) return s;
      return { ...s, [pane]: { ...cur, hidden: next } };
    }),
  moveColumn: (pane, field, dir) =>
    set((s) => {
      const cur = s[pane];
      const order = columnVisualOrder(cur.order);
      const i = order.indexOf(field);
      const j = i + dir;
      if (i < 0 || j < 0 || j >= order.length) return s;
      const next = [...order];
      [next[i], next[j]] = [next[j], next[i]];
      return { ...s, [pane]: { ...cur, order: next } };
    }),
}));
