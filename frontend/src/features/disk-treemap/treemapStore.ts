import { create } from 'zustand';
import { FileService } from '../../shared/api/bindings';
import { errMessage } from '../../shared/lib/format';
import type { UsageDone, UsageProgress } from './types';

type TreemapState = {
  open: boolean;
  root: string;
  jobId: string;
  crumbs: string[];
  cache: Record<string, UsageDone>;
  progress: UsageProgress | null;
  scanning: boolean;
  callError: string;
  zoomPath: string;
  openDialog: (root: string) => void;
  closeDialog: () => void;
  drill: (path: string) => void;
  setZoom: (path: string) => void;
  applyProgress: (payload: UsageProgress) => void;
  applyDone: (payload: UsageDone) => void;
};

let generation = 0;

const remember = (crumbs: string[], path: string): string[] =>
  crumbs.includes(path) ? crumbs : [...crumbs, path];

const cancelInFlight = (): void => {
  const { scanning, jobId } = useTreemapStore.getState();
  if (scanning && jobId) void FileService.CancelJob(jobId);
};

const begin = (root: string, crumbs: string[]): void => {
  const mine = ++generation;
  cancelInFlight();
  useTreemapStore.setState({
    open: true,
    root,
    crumbs,
    jobId: '',
    scanning: true,
    progress: null,
    callError: '',
    zoomPath: '',
  });
  void FileService.NewJobID().then(
    (jobId) => {
      if (mine !== generation || !useTreemapStore.getState().open) {
        void FileService.CancelJob(jobId);
        return;
      }
      useTreemapStore.setState({ jobId });
      void FileService.TreeUsage(jobId, root, 0).then(
        () => undefined,
        (err: unknown) => {
          if (mine !== generation) return;
          useTreemapStore.setState({
            scanning: false,
            progress: null,
            callError: errMessage(err),
          });
        },
      );
    },
    (err: unknown) => {
      if (mine !== generation) return;
      useTreemapStore.setState({ scanning: false, callError: errMessage(err) });
    },
  );
};

export const useTreemapStore = create<TreemapState>((set, get) => ({
  open: false,
  root: '',
  jobId: '',
  crumbs: [],
  cache: {},
  progress: null,
  scanning: false,
  callError: '',
  zoomPath: '',

  openDialog: (root) => {
    set({ cache: {} });
    begin(root, root ? [root] : []);
  },

  closeDialog: () => {
    generation += 1;
    cancelInFlight();
    set({
      open: false,
      root: '',
      jobId: '',
      crumbs: [],
      cache: {},
      progress: null,
      scanning: false,
      callError: '',
      zoomPath: '',
    });
  },

  drill: (path) => {
    if (!path) return;
    const cur = get();
    const crumbs = remember(cur.crumbs, path);
    const cached = cur.cache[path];
    if (cached) {
      generation += 1;
      cancelInFlight();
      set({
        root: path,
        crumbs,
        jobId: '',
        scanning: false,
        progress: null,
        callError: '',
        zoomPath: '',
      });
      return;
    }
    begin(path, crumbs);
  },

  setZoom: (path) => set({ zoomPath: path }),

  applyProgress: (payload) => {
    if (payload.jobId !== get().jobId) return;
    set({ progress: payload });
  },

  applyDone: (payload) => {
    const cur = get();
    if (payload.jobId !== cur.jobId) return;
    const cache = { ...cur.cache, [payload.root]: payload };
    if (cur.root && cur.root !== payload.root) cache[cur.root] = payload;
    const root = payload.root || cur.root;
    set({
      cache,
      root,
      crumbs: cur.crumbs.map((path) => (path === cur.root ? root : path)),
      scanning: false,
      progress: null,
    });
  },
}));
