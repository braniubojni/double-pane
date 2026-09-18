import { useQueryClient } from '@tanstack/react-query';
import { useCallback, useEffect, useReducer, useRef } from 'react';
import { useFileOps } from '../../../entities/file/queries';
import {
  deleteDialogReducer,
  initialDeleteDialogState,
} from '../../../features/file-ops/deleteDialogReducer';
import {
  initialNameDialogState,
  nameDialogReducer,
} from '../../../features/file-ops/nameDialogReducer';
import { startTransfer } from '../../../features/transfers/startTransfer';
import { FileService } from '../../../shared/api/bindings';
import { errMessage } from '../../../shared/lib/format';
import { useSnack } from '../../../shared/ui/SnackbarHost';
import { isRemotePath } from '../../../features/connections/helpers';
import { isArchivePanePath } from '../../../shared/lib/archives';
import { isPermissionError } from '../helpers';
import { isTrashPath } from '../../../shared/lib/trash';
import type { FileOpDialogsArgs } from '../types';

/** How long the Undo action stays on screen after a delete. */
const UNDO_WINDOW_MS = 10_000;

export const useFileOpDialogs = ({
  activePath,
  realSelection,
  destPath,
  clearSelection,
}: FileOpDialogsArgs) => {
  const ops = useFileOps();
  const show = useSnack((s) => s.show);
  const qc = useQueryClient();
  const [mkdir, dispatchMkdir] = useReducer(nameDialogReducer, initialNameDialogState);
  const [mkfile, dispatchMkfile] = useReducer(nameDialogReducer, initialNameDialogState);
  const [rename, dispatchRename] = useReducer(nameDialogReducer, initialNameDialogState);
  const [del, dispatchDelete] = useReducer(deleteDialogReducer, initialDeleteDialogState);
  const deleteBtnRef = useRef<HTMLButtonElement | null>(null);

  const onOpError = useCallback(
    (e: unknown) => {
      const msg = errMessage(e);
      if (isPermissionError(msg)) {
        dispatchDelete({ type: 'open_permission', message: msg });
      } else {
        show(msg, 'error');
      }
    },
    [show],
  );

  const onOpSuccess = useCallback(
    (label: string) => {
      show(`${label} completed`, 'success');
      clearSelection();
    },
    [show, clearSelection],
  );

  const rejectTrashEdit = (): boolean => {
    if (!isTrashPath(activePath)) return false;
    show('Restore items or empty Trash instead', 'warning');
    return true;
  };

  const onCopy = () => {
    if (rejectTrashEdit()) return;
    if (!realSelection.length) return show('Select files to copy', 'warning');
    startTransfer({
      kind: 'copy',
      sources: realSelection,
      destDir: destPath,
      show,
      onSuccess: () => clearSelection(),
      onSettled: () => {
        void qc.invalidateQueries({ queryKey: ['dir'] });
        void qc.invalidateQueries({ queryKey: ['gitStatus'] });
      },
    });
  };

  const onPaste = () => {
    if (rejectTrashEdit()) return;
    if (isRemotePath(activePath) || isArchivePanePath(activePath)) {
      return show('Paste is not available here', 'warning');
    }
    void FileService.PasteClipboard(activePath)
      .then(() => {
        void qc.invalidateQueries({ queryKey: ['dir'] });
        void qc.invalidateQueries({ queryKey: ['gitStatus'] });
        show('Pasted', 'success');
      })
      .catch((e) => {
        const msg = errMessage(e);
        if (/nothing to paste/i.test(msg)) {
          show('Nothing to paste', 'info');
          return;
        }
        show(msg, 'error');
      });
  };

  const onMove = () => {
    if (rejectTrashEdit()) return;
    if (!realSelection.length) return show('Select files to move', 'warning');
    startTransfer({
      kind: 'move',
      sources: realSelection,
      destDir: destPath,
      show,
      onSuccess: () => clearSelection(),
      onSettled: () => {
        void qc.invalidateQueries({ queryKey: ['dir'] });
        void qc.invalidateQueries({ queryKey: ['gitStatus'] });
      },
    });
  };

  const onDelete = () => {
    if (!realSelection.length) return show('Select files to delete', 'warning');
    dispatchDelete({
      type: 'open_confirm',
      paths: realSelection,
      permanent: isTrashPath(activePath),
    });
  };

  const onDeletePermanent = () => {
    if (!realSelection.length) return show('Select files to delete', 'warning');
    dispatchDelete({ type: 'open_confirm', paths: realSelection, permanent: true });
  };

  const undoDelete = (batchID: string) => {
    void FileService.RestoreDeleted(batchID)
      .then(() => {
        void qc.invalidateQueries({ queryKey: ['dir'] });
        void qc.invalidateQueries({ queryKey: ['gitStatus'] });
        show('Delete undone', 'success');
      })
      .catch((e) => show(errMessage(e), 'error'));
  };

  const confirmDelete = () => {
    if (!del.confirmOpen) return;
    const paths = del.paths.length ? del.paths : realSelection;
    if (!paths.length) return;
    const permanent = del.permanent;
    dispatchDelete({ type: 'close_confirm' });
    const onOk = (batchID?: string) => {
      clearSelection();
      void qc.invalidateQueries({ queryKey: ['dir'] });
      void qc.invalidateQueries({ queryKey: ['gitStatus'] });
      show(
        'Delete completed',
        'success',
        batchID
          ? {
              duration: UNDO_WINDOW_MS,
              action: {
                label: 'Undo',
                testId: 'btn-undo-delete',
                onClick: () => undoDelete(batchID),
              },
            }
          : undefined,
      );
    };
    if (permanent || isRemotePath(activePath)) {
      ops.delPermanent.mutate(paths, { onSuccess: () => onOk(), onError: onOpError });
      return;
    }
    ops.del.mutate(paths, {
      onSuccess: (batchID) => onOk(batchID || undefined),
      onError: onOpError,
    });
  };

  const onRestoreTrash = () => {
    if (!isTrashPath(activePath)) return;
    if (!realSelection.length) return show('Select items to restore', 'warning');
    void FileService.RestoreTrash(realSelection)
      .then(() => {
        clearSelection();
        void qc.invalidateQueries({ queryKey: ['dir'] });
        void qc.invalidateQueries({ queryKey: ['gitStatus'] });
        show('Restored', 'success');
      })
      .catch((e) => show(errMessage(e), 'error'));
  };

  const onEmptyTrash = () => dispatchDelete({ type: 'open_empty' });

  const confirmEmptyTrash = () => {
    if (!del.emptyOpen) return;
    dispatchDelete({ type: 'close_empty' });
    void FileService.EmptyTrash()
      .then(() => {
        clearSelection();
        void qc.invalidateQueries({ queryKey: ['dir'] });
        show('Trash emptied', 'success');
      })
      .catch((e) => show(errMessage(e), 'error'));
  };

  const onMkdir = () => {
    if (rejectTrashEdit()) return;
    dispatchMkdir({ type: 'open', name: 'New Folder' });
  };

  const confirmMkdir = () => {
    if (!mkdir.open) return;
    const name = mkdir.name.trim();
    if (!name) return;
    dispatchMkdir({ type: 'close' });
    ops.mkdir.mutate(
      { parent: activePath, name },
      { onSuccess: () => onOpSuccess('Create folder'), onError: onOpError },
    );
  };

  const onMkfile = () => {
    if (rejectTrashEdit()) return;
    dispatchMkfile({ type: 'open', name: 'untitled.txt' });
  };

  const confirmMkfile = () => {
    if (!mkfile.open) return;
    const name = mkfile.name.trim();
    if (!name) return;
    dispatchMkfile({ type: 'close' });
    ops.mkfile.mutate(
      { parent: activePath, name },
      { onSuccess: () => onOpSuccess('Create file'), onError: onOpError },
    );
  };

  const onRename = () => {
    if (rejectTrashEdit()) return;
    if (realSelection.length !== 1) return show('Select exactly one item to rename', 'warning');
    const base = realSelection[0].split(/[/\\]/).pop() || '';
    dispatchRename({ type: 'open', name: base });
  };

  const confirmRename = () => {
    if (!rename.open) return;
    const newName = rename.name.trim();
    const oldPath = realSelection[0];
    if (!newName || !oldPath) return;
    dispatchRename({ type: 'close' });
    ops.rename.mutate(
      { oldPath, newName },
      { onSuccess: () => onOpSuccess('Rename'), onError: onOpError },
    );
  };

  const onBookmark = () => {
    ops.addBookmark.mutate(
      { name: '', path: activePath },
      { onSuccess: () => onOpSuccess('Bookmark'), onError: onOpError },
    );
  };

  useEffect(() => {
    if (!del.confirmOpen) return;
    const t = window.setTimeout(() => deleteBtnRef.current?.focus(), 50);
    return () => window.clearTimeout(t);
  }, [del.confirmOpen]);

  return {
    mkdir,
    mkfile,
    rename,
    del,
    remote: isRemotePath(activePath),
    deleteBtnRef,
    dispatchMkdir,
    dispatchMkfile,
    dispatchRename,
    dispatchDelete,
    onCopy,
    onPaste,
    onMove,
    onDelete,
    onDeletePermanent,
    onRestoreTrash,
    onEmptyTrash,
    onMkdir,
    onMkfile,
    onRename,
    onBookmark,
    confirmMkdir,
    confirmMkfile,
    confirmRename,
    confirmDelete,
    confirmEmptyTrash,
  };
};
