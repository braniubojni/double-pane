import CheckIcon from '@mui/icons-material/Check';
import AppBar from '@mui/material/AppBar';
import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import Divider from '@mui/material/Divider';
import ListItemIcon from '@mui/material/ListItemIcon';
import ListItemText from '@mui/material/ListItemText';
import Menu from '@mui/material/Menu';
import MenuItem from '@mui/material/MenuItem';
import Toolbar from '@mui/material/Toolbar';
import { useQueryClient } from '@tanstack/react-query';
import { useState, type FC, type MouseEvent } from 'react';
import { useShallow } from 'zustand/react/shallow';
import {
  useDirListing,
  useLockVault,
  usePatchSettings,
  useSettings,
  useUnlockedVaults,
} from '../../entities/file/queries';
import { useDuplicatesStore } from '../../features/duplicates/duplicatesStore';
import { useFileOpsStore } from '../../features/file-ops/fileOpsStore';
import { isLocalArchivePath } from '../../features/duplicates/helpers';
import { usePaneJobStore } from '../../features/jobs/paneJobStore';
import { usePaneStore } from '../../features/pane/paneStore';
import { useSearchStore } from '../../features/search/searchStore';
import { useDialogStore } from '../../features/ui/dialogStore';
import { vaultMenuState } from '../../features/vault/helpers';
import { useVaultDialogStore } from '../../features/vault/vaultDialogStore';
import { isTrashPath } from '../../shared/lib/trash';
import { errMessage } from '../../shared/lib/format';
import { useSnack } from '../../shared/ui/SnackbarHost';
import { appBarSx, checkPlaceholderSx, listItemIconSx, toolbarSx } from './styles';
import type { AppMenuBarProps } from './types';

export const AppMenuBar: FC<AppMenuBarProps> = ({
  onNewFolder,
  onNewFile,
  onEditFile,
  onGitDiff,
  onRename,
  onDelete,
}) => {
  const { data: settings } = useSettings();
  const patch = usePatchSettings();
  const show = useSnack((s) => s.show);
  const openSettings = useDialogStore((s) => s.openSettings);
  const openShortcuts = useDialogStore((s) => s.openShortcuts);
  const openSearch = useSearchStore((s) => s.openSearch);
  const openDuplicates = useDuplicatesStore((s) => s.openDialog);
  const dupRunning = useDuplicatesStore((s) => s.phase === 'running');
  const activePane = usePaneStore((s) => s.activePane);
  const cwd = usePaneStore((s) => s.getPath(s.activePane));
  const actionPaths = usePaneStore(useShallow((s) => s.getActionPaths(s.activePane)));
  const trigger = useFileOpsStore((s) => s.trigger);
  const dupDisabled = dupRunning || isLocalArchivePath(cwd);
  const inTrash = isTrashPath(cwd);
  const qc = useQueryClient();

  const { data: listing } = useDirListing(cwd || undefined, settings?.showHidden ?? false);
  const { data: unlockedVaults } = useUnlockedVaults();
  const paneJob = usePaneJobStore((s) => s.getJob(activePane));
  const openLockDialog = useVaultDialogStore((s) => s.openLock);
  const openUnlockDialog = useVaultDialogStore((s) => s.openUnlock);
  const openRemoveDialog = useVaultDialogStore((s) => s.openRemove);
  const lockVault = useLockVault();

  const focusedEntry =
    actionPaths.length === 1 ? (listing?.find((e) => e.path === actionPaths[0]) ?? null) : null;
  const vaultTarget = focusedEntry?.path ?? cwd;
  const vaultBusy = paneJob?.kind === 'vault' && paneJob.path === vaultTarget;
  const vaultState = vaultMenuState(
    focusedEntry,
    cwd,
    actionPaths.length,
    (unlockedVaults ?? []).map((v) => v.root),
  );

  const [fileAnchor, setFileAnchor] = useState<null | HTMLElement>(null);
  const [viewAnchor, setViewAnchor] = useState<null | HTMLElement>(null);

  const closeAll = () => {
    setFileAnchor(null);
    setViewAnchor(null);
  };

  const toggle = (key: 'showHidden' | 'showExtensions') => {
    patch.mutate({ [key]: !settings?.[key] }, { onError: (e) => show(errMessage(e), 'error') });
  };

  const refresh = () => {
    void qc.invalidateQueries({ queryKey: ['dir'] });
    void qc.invalidateQueries({ queryKey: ['gitStatus'] });
    closeAll();
  };

  const openMenu = (setter: (el: HTMLElement | null) => void) => (e: MouseEvent<HTMLElement>) => {
    setter(e.currentTarget);
  };

  return (
    <AppBar position="static" color="default" elevation={0} sx={appBarSx}>
      <Toolbar variant="dense" sx={toolbarSx}>
        <Button data-testid="menu-file" size="small" onClick={openMenu(setFileAnchor)}>
          File
        </Button>
        <Menu anchorEl={fileAnchor} open={Boolean(fileAnchor)} onClose={closeAll}>
          <MenuItem
            data-testid="menu-file-mkdir"
            disabled={inTrash}
            onClick={() => {
              closeAll();
              onNewFolder();
            }}
          >
            New folder
          </MenuItem>
          <MenuItem
            data-testid="menu-file-mkfile"
            disabled={inTrash}
            onClick={() => {
              closeAll();
              onNewFile();
            }}
          >
            New file
          </MenuItem>
          <MenuItem
            data-testid="menu-file-edit"
            onClick={() => {
              closeAll();
              onEditFile();
            }}
          >
            Edit
          </MenuItem>
          <MenuItem
            data-testid="menu-file-git-diff"
            onClick={() => {
              closeAll();
              onGitDiff();
            }}
          >
            Git diff
          </MenuItem>
          <MenuItem
            data-testid="menu-file-rename"
            disabled={inTrash}
            onClick={() => {
              closeAll();
              onRename();
            }}
          >
            Rename
          </MenuItem>
          <MenuItem
            data-testid="menu-file-delete"
            onClick={() => {
              closeAll();
              onDelete();
            }}
          >
            Delete
          </MenuItem>
          <MenuItem
            data-testid="menu-file-delete-permanent"
            onClick={() => {
              closeAll();
              trigger('deletePermanent');
            }}
          >
            Delete permanently
          </MenuItem>
          <MenuItem
            data-testid="menu-file-empty-trash"
            onClick={() => {
              closeAll();
              trigger('emptyTrash');
            }}
          >
            Empty Trash
          </MenuItem>
          <Divider />
          <MenuItem
            data-testid="menu-file-vault-lock"
            disabled={!vaultState.lock || vaultBusy}
            onClick={() => {
              closeAll();
              if (focusedEntry) openLockDialog(focusedEntry.path, activePane, !focusedEntry.isDir);
            }}
          >
            Lock with password…
          </MenuItem>
          <MenuItem
            data-testid="menu-file-vault-unlock"
            disabled={!vaultState.unlock || vaultBusy}
            onClick={() => {
              closeAll();
              openUnlockDialog(
                vaultTarget,
                activePane,
                Boolean(focusedEntry && !focusedEntry.isDir),
              );
            }}
          >
            Unlock…
          </MenuItem>
          <MenuItem
            data-testid="menu-file-vault-lock-now"
            disabled={!vaultState.lockNow || vaultBusy}
            onClick={() => {
              closeAll();
              lockVault.mutate(vaultTarget, {
                onSuccess: () => show('Locked', 'success'),
                onError: (e) => show(errMessage(e), 'error'),
              });
            }}
          >
            Lock now
          </MenuItem>
          <MenuItem
            data-testid="menu-file-vault-remove"
            disabled={!vaultState.removePassword || vaultBusy}
            onClick={() => {
              closeAll();
              openRemoveDialog(vaultTarget, activePane);
            }}
          >
            Remove password…
          </MenuItem>
          <Divider />
          <MenuItem
            data-testid="menu-file-search"
            onClick={() => {
              closeAll();
              openSearch();
            }}
          >
            Find in files…
          </MenuItem>
          <MenuItem
            data-testid="menu-file-duplicates"
            disabled={dupDisabled}
            onClick={() => {
              closeAll();
              openDuplicates(cwd);
            }}
          >
            Find duplicates…
          </MenuItem>
          <MenuItem
            data-testid="menu-file-settings"
            onClick={() => {
              closeAll();
              openSettings();
            }}
          >
            Settings…
          </MenuItem>
          <MenuItem
            data-testid="menu-file-shortcuts"
            onClick={() => {
              closeAll();
              openShortcuts();
            }}
          >
            Keyboard shortcuts…
          </MenuItem>
        </Menu>

        <Button data-testid="menu-view" size="small" onClick={openMenu(setViewAnchor)}>
          View
        </Button>
        <Menu anchorEl={viewAnchor} open={Boolean(viewAnchor)} onClose={closeAll}>
          <MenuItem data-testid="menu-view-hidden" onClick={() => toggle('showHidden')}>
            <ListItemIcon sx={listItemIconSx}>
              {settings?.showHidden ? (
                <CheckIcon fontSize="small" />
              ) : (
                <Box sx={checkPlaceholderSx} />
              )}
            </ListItemIcon>
            <ListItemText>Show hidden files</ListItemText>
          </MenuItem>
          <MenuItem data-testid="menu-view-extensions" onClick={() => toggle('showExtensions')}>
            <ListItemIcon sx={listItemIconSx}>
              {settings?.showExtensions ? (
                <CheckIcon fontSize="small" />
              ) : (
                <Box sx={checkPlaceholderSx} />
              )}
            </ListItemIcon>
            <ListItemText>Show file extensions</ListItemText>
          </MenuItem>
          <Divider />
          <MenuItem data-testid="menu-view-refresh" onClick={refresh}>
            Refresh
          </MenuItem>
        </Menu>
      </Toolbar>
    </AppBar>
  );
};
