import type { ScanEstimate } from '../../shared/api/bindings';

type DupPhase = 'setup' | 'running' | 'error' | 'review' | 'merging' | 'done';

export type DuplicateFile = {
  path: string;
  name: string;
  size: number;
  modTime: number;
  protocol: string;
};

export type DuplicateGroup = {
  hash: string;
  size: number;
  files: DuplicateFile[];
  kind?: 'exact' | 'visual' | 'ocr';
  similarity?: number;
  snippet?: string;
};

type DupSkipped = { path: string; message: string };

type DupProgress = {
  doneFiles: number;
  totalFiles: number;
  doneBytes: number;
  totalBytes: number;
  groups: number;
  skipped: number;
  currentPath: string;
  phase?: string;
  doneImages?: number;
  totalImages?: number;
};

export type DupProgressPayload = DupProgress & { jobId: string };
export type DupGroupPayload = { jobId: string; group: DuplicateGroup };
export type DupErrorPayload = { jobId: string; path: string; message: string; fatal: boolean };
export type DupDonePayload = { jobId: string; error?: string; groups?: DuplicateGroup[] };

export type DupSetup = {
  root: string;
  includeHidden: boolean;
  minSize: number;
  exclude: string;
  similarImages: boolean;
  similarityPct: number;
  ocr: boolean;
};

export type DuplicatesState = {
  dialogOpen: boolean;
  phase: DupPhase;
  jobId: string;
  setup: DupSetup;
  estimate: ScanEstimate | null;
  groups: DuplicateGroup[];
  skipped: DupSkipped[];
  progress: DupProgress | null;
  errorMessage: string;
  keepByHash: Record<string, string>;
  checkedByHash: Record<string, string[]>;
  batchId: string;

  openDialog: (cwd: string) => void;
  closeDialog: () => void;
  patchSetup: (p: Partial<DupSetup>) => void;
  setEstimate: (estimate: ScanEstimate | null) => void;
  beginScan: (jobId: string) => void;
  applyProgress: (p: DupProgressPayload) => void;
  addGroup: (g: DuplicateGroup) => void;
  addSkipped: (path: string, message: string) => void;
  fail: (message: string) => void;
  finish: (error?: string, groups?: DuplicateGroup[]) => void;
  setKeep: (hash: string, path: string) => void;
  toggleChecked: (hash: string, path: string) => void;
  setPhase: (phase: DupPhase) => void;
  setBatchId: (batchId: string) => void;
  discard: () => void;
  backToSetup: () => void;
};
