import { create } from 'zustand';
import type { PaneId } from '../../entities/file/types';

type VaultDialogMode = 'lock' | 'unlock' | 'remove' | null;

type VaultDialogState = {
  mode: VaultDialogMode;
  path: string;
  paneId: PaneId;
  /** True when path is a regular file (.dpenc target) rather than a folder. */
  isFile: boolean;
  openLock: (path: string, paneId: PaneId, isFile: boolean) => void;
  /** isFile: .dpenc target — unlocking one never "enters" it (spec V1). */
  openUnlock: (path: string, paneId: PaneId, isFile: boolean) => void;
  openRemove: (path: string, paneId: PaneId) => void;
  close: () => void;
};

export const useVaultDialogStore = create<VaultDialogState>((set) => ({
  mode: null,
  path: '',
  paneId: 'left',
  isFile: false,
  openLock: (path, paneId, isFile) => set({ mode: 'lock', path, paneId, isFile }),
  openUnlock: (path, paneId, isFile) => set({ mode: 'unlock', path, paneId, isFile }),
  openRemove: (path, paneId) => set({ mode: 'remove', path, paneId, isFile: false }),
  close: () => set({ mode: null, path: '' }),
}));
