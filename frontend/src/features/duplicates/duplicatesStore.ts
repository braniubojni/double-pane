import { create } from 'zustand';
import { isMEGAPath } from '../connections/helpers';
import { defaultKeepAndChecked } from './helpers';
import type { DuplicatesState, DupSetup } from './types';

const emptySetup = (): DupSetup => ({
  root: '',
  includeHidden: false,
  minSize: 0,
  exclude: '',
  similarImages: true,
  similarityPct: 90,
  ocr: false,
});

const idle: Pick<
  DuplicatesState,
  | 'dialogOpen'
  | 'phase'
  | 'jobId'
  | 'estimate'
  | 'groups'
  | 'skipped'
  | 'progress'
  | 'errorMessage'
  | 'keepByHash'
  | 'checkedByHash'
  | 'batchId'
> = {
  dialogOpen: false,
  phase: 'setup',
  jobId: '',
  estimate: null,
  groups: [],
  skipped: [],
  progress: null,
  errorMessage: '',
  keepByHash: {},
  checkedByHash: {},
  batchId: '',
};

export const useDuplicatesStore = create<DuplicatesState>((set, get) => ({
  ...idle,
  setup: emptySetup(),

  openDialog: (cwd) => {
    const s = get();
    const resume =
      s.phase === 'running' ||
      s.phase === 'review' ||
      s.phase === 'error' ||
      s.phase === 'merging' ||
      s.phase === 'done';
    if (resume) {
      set({ dialogOpen: true });
      return;
    }
    set({
      ...idle,
      dialogOpen: true,
      phase: 'setup',
      setup: {
        ...s.setup,
        root: cwd,
        similarImages: !isMEGAPath(cwd),
        similarityPct: s.setup.similarityPct || 90,
        ocr: false,
      },
    });
  },

  closeDialog: () => set({ dialogOpen: false }),

  patchSetup: (p) =>
    set((s) => {
      const setup = { ...s.setup, ...p };
      const cost =
        p.root !== undefined ||
        p.includeHidden !== undefined ||
        p.minSize !== undefined ||
        p.exclude !== undefined ||
        p.similarImages !== undefined ||
        p.ocr !== undefined;
      return { setup, estimate: cost ? null : s.estimate };
    }),

  setEstimate: (estimate) => set({ estimate }),

  beginScan: (jobId) => {
    const s = get();
    const est = s.estimate;
    set({
      jobId,
      phase: 'running',
      groups: [],
      skipped: [],
      progress: est
        ? {
            doneFiles: 0,
            totalFiles: est.fileCount,
            doneBytes: 0,
            totalBytes: est.byteCount,
            groups: 0,
            skipped: 0,
            currentPath: s.setup.root,
          }
        : {
            doneFiles: 0,
            totalFiles: 0,
            doneBytes: 0,
            totalBytes: 0,
            groups: 0,
            skipped: 0,
            currentPath: s.setup.root,
          },
      errorMessage: '',
      keepByHash: {},
      checkedByHash: {},
      batchId: '',
    });
  },

  applyProgress: (p) => {
    if (p.jobId !== get().jobId) return;
    set({
      progress: {
        doneFiles: p.doneFiles,
        totalFiles: p.totalFiles,
        doneBytes: p.doneBytes,
        totalBytes: p.totalBytes,
        groups: p.groups,
        skipped: p.skipped,
        currentPath: p.currentPath,
        phase: p.phase,
        doneImages: p.doneImages,
        totalImages: p.totalImages,
      },
    });
  },

  addGroup: (g) => {
    if (!g?.hash) return;
    const { keep, checked } = defaultKeepAndChecked(g.files ?? []);
    set((s) => {
      if (s.groups.some((x) => x.hash === g.hash)) return s;
      return {
        groups: [...s.groups, g],
        keepByHash: { ...s.keepByHash, [g.hash]: keep },
        checkedByHash: { ...s.checkedByHash, [g.hash]: checked },
      };
    });
  },

  addSkipped: (path, message) =>
    set((s) => {
      if (s.skipped.some((x) => x.path === path && x.message === message)) return s;
      return { skipped: [...s.skipped, { path, message }] };
    }),

  fail: (message) => set({ phase: 'error', errorMessage: message || 'Duplicate scan failed' }),

  finish: (error, incoming) => {
    if (get().phase === 'error') return;
    const s = get();
    let groups = s.groups;
    let keepByHash = s.keepByHash;
    let checkedByHash = s.checkedByHash;
    if (incoming?.length) {
      const seen = new Set(s.groups.map((g) => g.hash));
      groups = [...s.groups];
      keepByHash = { ...keepByHash };
      checkedByHash = { ...checkedByHash };
      for (const g of incoming) {
        if (!g?.hash || seen.has(g.hash)) continue;
        seen.add(g.hash);
        const { keep, checked } = defaultKeepAndChecked(g.files ?? []);
        groups.push(g);
        keepByHash[g.hash] = keep;
        checkedByHash[g.hash] = checked;
      }
    }
    if (error) {
      set({ groups, keepByHash, checkedByHash, phase: 'error', errorMessage: error });
      return;
    }
    set({ groups, keepByHash, checkedByHash, phase: 'review' });
  },

  setKeep: (hash, path) =>
    set((s) => {
      const prev = s.keepByHash[hash];
      const cur = new Set(s.checkedByHash[hash] ?? []);
      cur.delete(path);
      if (prev && prev !== path) cur.add(prev);
      return {
        keepByHash: { ...s.keepByHash, [hash]: path },
        checkedByHash: { ...s.checkedByHash, [hash]: [...cur] },
      };
    }),

  toggleChecked: (hash, path) =>
    set((s) => {
      if (s.keepByHash[hash] === path) return s;
      const cur = new Set(s.checkedByHash[hash] ?? []);
      if (cur.has(path)) cur.delete(path);
      else cur.add(path);
      return { checkedByHash: { ...s.checkedByHash, [hash]: [...cur] } };
    }),

  setPhase: (phase) => set({ phase }),
  setBatchId: (batchId) => set({ batchId, phase: 'done' }),

  discard: () => set({ ...idle, setup: get().setup }),

  backToSetup: () =>
    set({
      ...idle,
      dialogOpen: true,
      phase: 'setup',
      setup: get().setup,
    }),
}));
