import type { PaneId } from '../../entities/file/types';

export type PaneJobKind = 'archive' | 'extract' | 'sizes' | 'copy' | 'move' | 'vault' | 'other';

export type PaneJob = {
  id: string;
  kind: PaneJobKind;
  label: string;
  cancelable: boolean;
  /** Backend job id for CancelJob when set */
  backendJobId?: string;
  /** Target path this job is working on (vault lock/remove) — lets menus
   * disable actions on that exact path while it's mid-flight. */
  path?: string;
};

export type PaneJobState = {
  left: PaneJob | null;
  right: PaneJob | null;
  getJob: (id: PaneId) => PaneJob | null;
  start: (pane: PaneId, job: PaneJob) => void;
  clear: (pane: PaneId, jobId?: string) => void;
  /** Clears only if the job id still matches (ignore stale completions). */
  finish: (pane: PaneId, jobId: string) => void;
  /** Replaces the label of whichever pane currently holds jobId (no-op if neither does). */
  updateLabel: (jobId: string, label: string) => void;
  swap: () => void;
};
