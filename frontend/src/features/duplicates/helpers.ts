import { isRemotePath } from '../connections/helpers';
import { isArchivePanePath } from '../../shared/lib/archives';
import type { DuplicateFile, DuplicateGroup } from './types';

export const isLocalArchivePath = (path?: string | null): boolean => {
  if (!path || isRemotePath(path)) return false;
  return isArchivePanePath(path);
};

export const unwrapDupEvent = <T>(ev: { data?: T | T[] } | T): T => {
  const raw = ev && typeof ev === 'object' && 'data' in ev ? (ev.data ?? ev) : ev;
  if (Array.isArray(raw)) return raw[0] as T;
  return raw as T;
};

const sortedFiles = (files: DuplicateFile[]): DuplicateFile[] =>
  [...files].sort((a, b) => a.path.localeCompare(b.path));

export const defaultKeepAndChecked = (
  files: DuplicateFile[],
): { keep: string; checked: string[] } => {
  const sorted = sortedFiles(files);
  return { keep: sorted[0]?.path ?? '', checked: sorted.slice(1).map((f) => f.path) };
};

export const protocolOf = (groups: DuplicateGroup[], fallback = 'local'): string =>
  groups[0]?.files[0]?.protocol || fallback;

export const checkedPaths = (checkedByHash: Record<string, string[]>): string[] =>
  Object.values(checkedByHash).flat();

export const bytesOfPaths = (groups: DuplicateGroup[], paths: string[]): number => {
  const want = new Set(paths);
  let n = 0;
  for (const g of groups) {
    for (const f of g.files) {
      if (want.has(f.path)) n += f.size;
    }
  }
  return n;
};

export const isExactGroup = (g: DuplicateGroup): boolean => !g.kind || g.kind === 'exact';

export const isVisualGroup = (g: DuplicateGroup): boolean => g.kind === 'visual';

export const isOcrGroup = (g: DuplicateGroup): boolean => g.kind === 'ocr';
