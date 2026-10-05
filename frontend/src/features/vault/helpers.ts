import type { FileEntry } from '../../entities/file/types';
import { isRemotePath } from '../connections/helpers';
import { isLocalArchivePath } from '../duplicates/helpers';
import { isTrashPath } from '../../shared/lib/trash';

/** True for any path inside a vault's own `.dpvault` metadata directory. */
const isVaultInternalPath = (path: string): boolean => /[/\\]\.dpvault(?:[/\\]|$)/.test(path);

/** True when path is (or is inside) one of the currently-unlocked vault roots. */
const isInsideUnlockedVault = (path: string, roots: string[]): boolean =>
  roots.some((root) => path === root || path.startsWith(`${root.replace(/[/\\]+$/, '')}/`));

const isVaultEligiblePath = (path: string): boolean =>
  Boolean(path) &&
  !isRemotePath(path) &&
  !isTrashPath(path) &&
  !isLocalArchivePath(path) &&
  !isVaultInternalPath(path);

export type VaultMenuState = {
  lock: boolean;
  unlock: boolean;
  lockNow: boolean;
  removePassword: boolean;
};

const disabledState: VaultMenuState = {
  lock: false,
  unlock: false,
  lockNow: false,
  removePassword: false,
};

/**
 * Spec "Entry" enablement rules, shared by the File menu (selection-derived
 * entry) and the row context menu. `entry` is the single focused/selected row
 * — null when nothing is focused (File-menu empty selection, or a context
 * menu right-click on empty pane space), in which case only "is panePath
 * itself inside an unlocked vault" is knowable without another round trip.
 */
export const vaultMenuState = (
  entry: FileEntry | null,
  panePath: string,
  selectionCount: number,
  unlockedRoots: string[],
): VaultMenuState => {
  const path = entry?.path ?? panePath;
  if (selectionCount > 1 || !isVaultEligiblePath(path)) return disabledState;

  if (entry) {
    const isLockedVault = Boolean(entry.isVault) && Boolean(entry.vaultLocked);
    const isUnlockedVault = Boolean(entry.isVault) && !entry.vaultLocked;
    return {
      lock: !entry.isVault,
      unlock: isLockedVault,
      lockNow: isUnlockedVault,
      removePassword: isUnlockedVault,
    };
  }

  const inside = isInsideUnlockedVault(path, unlockedRoots);
  return { lock: false, unlock: false, lockNow: inside, removePassword: inside };
};
