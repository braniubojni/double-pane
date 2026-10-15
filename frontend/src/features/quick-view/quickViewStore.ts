import { create } from 'zustand';

interface QuickViewState {
  open: boolean;
  toggle: () => void;
  close: () => void;
}

export const useQuickViewStore = create<QuickViewState>((set) => ({
  open: false,
  toggle: () => set((s) => ({ open: !s.open })),
  close: () => set({ open: false }),
}));
