import { create } from 'zustand';
import type { PaneJobState } from './types';

export const usePaneJobStore = create<PaneJobState>((set, get) => ({
  left: null,
  right: null,

  getJob: (id) => (id === 'left' ? get().left : get().right),

  start: (pane, job) => {
    set(pane === 'left' ? { left: job } : { right: job });
  },

  clear: (pane, jobId) => {
    const key = pane === 'left' ? 'left' : 'right';
    const cur = get()[key];
    if (jobId && cur && cur.id !== jobId) return;
    set({ [key]: null });
  },

  finish: (pane, jobId) => {
    const key = pane === 'left' ? 'left' : 'right';
    const cur = get()[key];
    if (!cur || cur.id !== jobId) return;
    set({ [key]: null });
  },

  updateLabel: (jobId, label) => {
    const { left, right } = get();
    if (left?.id === jobId) set({ left: { ...left, label } });
    else if (right?.id === jobId) set({ right: { ...right, label } });
  },

  swap: () => set((s) => ({ left: s.right, right: s.left })),
}));

let jobSeq = 0;
export const newJobId = (prefix = 'job'): string => {
  jobSeq += 1;
  return `${prefix}-${Date.now()}-${jobSeq}`;
};
