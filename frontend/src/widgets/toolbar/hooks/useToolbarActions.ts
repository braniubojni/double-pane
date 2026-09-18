import { useQueryClient } from '@tanstack/react-query';
import { useMemo } from 'react';
import { useSetTheme, useSettings } from '../../../entities/file/queries';
import type { ThemePreference } from '../../../entities/file/types';
import { parentDirOf, useEditorStore } from '../../../features/editor/editorStore';
import { openDocument } from '../../../shared/lib/openDocument';
import { useGoToStore } from '../../../features/go-to/goToStore';
import { isRemotePath } from '../../../features/connections/helpers';
import { isTrashPath, TRASH_URI } from '../../../shared/lib/trash';
import { usePaneStore } from '../../../features/pane/paneStore';
import { useDialogStore } from '../../../features/ui/dialogStore';
import { FileService } from '../../../shared/api/bindings';
import { errMessage } from '../../../shared/lib/format';
import { useSnack } from '../../../shared/ui/SnackbarHost';
import { enterPaneTab, historyNav } from '../../file-pane/helpers';
import {
  createToolbarRequestHandlers,
  parentPath,
  resolveActionPaths,
  triggerCalcSizes,
} from '../helpers';
import { useArchiveExtract } from './useArchiveExtract';
import { useFileOpDialogs } from './useFileOpDialogs';
import { useFileOpsRequest } from './useFileOpsRequest';

export const useToolbarActions = () => {
  const activePane = usePaneStore((s) => s.activePane);
  const leftPath = usePaneStore((s) => s.getPath('left'));
  const rightPath = usePaneStore((s) => s.getPath('right'));
  const leftSelection = usePaneStore((s) => s.leftSelection);
  const rightSelection = usePaneStore((s) => s.rightSelection);
  const leftFocus = usePaneStore((s) => s.leftFocus);
  const rightFocus = usePaneStore((s) => s.rightFocus);
  const navigateStore = usePaneStore((s) => s.navigate);
  const canBack = usePaneStore((s) => s.canGoBack(s.activePane));
  const canForward = usePaneStore((s) => s.canGoForward(s.activePane));
  const clearSelection = usePaneStore((s) => s.clearSelection);
  const otherPane = usePaneStore((s) => s.otherPane);
  const openSettings = useDialogStore((s) => s.openSettings);
  const openWorkspace = useEditorStore((s) => s.openWorkspace);
  const openDiff = useEditorStore((s) => s.openDiff);
  const openGoTo = useGoToStore((s) => s.openGoTo);
  const editorOpen = useEditorStore((s) => s.open);

  const { data: settings } = useSettings();
  const setTheme = useSetTheme();
  const show = useSnack((s) => s.show);
  const qc = useQueryClient();

  const activePath = activePane === 'left' ? leftPath : rightPath;
  const destPath = activePane === 'left' ? rightPath : leftPath;
  const realSelection = useMemo(
    () =>
      resolveActionPaths(
        activePane === 'left' ? leftSelection : rightSelection,
        activePane === 'left' ? leftFocus : rightFocus,
      ),
    [activePane, leftSelection, rightSelection, leftFocus, rightFocus],
  );

  const theme = settings?.theme ?? 'system';
  const cycleTheme = () => {
    const order: ThemePreference[] = ['system', 'dark', 'light'];
    setTheme.mutate(order[(order.indexOf(theme) + 1) % order.length], {
      onError: (e) => show(errMessage(e), 'error'),
    });
  };
  const refreshAll = () => {
    void qc.invalidateQueries({ queryKey: ['dir'] });
    void qc.invalidateQueries({ queryKey: ['gitStatus'] });
  };

  const fileOps = useFileOpDialogs({ activePath, realSelection, destPath, clearSelection });
  const archiveOps = useArchiveExtract({ activePane, activePath, realSelection, clearSelection });

  const goParent = async () => {
    if (!activePath) return;
    if (isTrashPath(activePath)) {
      await goHome();
      return;
    }
    try {
      const next = parentPath(activePath);
      if (!next) {
        await goHome();
        return;
      }
      if (await FileService.Exists(next)) {
        enterPaneTab(activePane, next);
        navigateStore(activePane, next);
      }
    } catch (e) {
      show(errMessage(e), 'error');
    }
  };

  const goHome = async () => {
    try {
      const home = await FileService.GetHomeDir();
      enterPaneTab(activePane, home);
      navigateStore(activePane, home);
    } catch (e) {
      show(errMessage(e), 'error');
    }
  };

  const onEditFile = () => {
    if (realSelection.length !== 1) {
      show('Select exactly one file to edit', 'warning');
      return;
    }
    const path = realSelection[0];
    openDocument({
      path,
      useBuiltInEditor: settings?.useBuiltInEditor !== false,
      openWorkspace,
      show,
    });
  };

  const onGitDiff = () => {
    if (realSelection.length !== 1) {
      show('Select exactly one file to diff', 'warning');
      return;
    }
    const path = realSelection[0];
    if (isRemotePath(path)) {
      show('Git diff is not available on remote connections', 'warning');
      return;
    }
    openDiff(parentDirOf(path), path);
  };

  const onGoTo = () => {
    if (editorOpen) return;
    openGoTo();
  };

  const onTrash = () => {
    enterPaneTab(activePane, TRASH_URI);
    navigateStore(activePane, TRASH_URI);
  };

  useFileOpsRequest(
    createToolbarRequestHandlers({
      copy: fileOps.onCopy,
      paste: fileOps.onPaste,
      move: fileOps.onMove,
      delete: fileOps.onDelete,
      deletePermanent: fileOps.onDeletePermanent,
      restoreTrash: fileOps.onRestoreTrash,
      emptyTrash: fileOps.onEmptyTrash,
      rename: fileOps.onRename,
      mkdir: fileOps.onMkdir,
      mkfile: fileOps.onMkfile,
      editFile: onEditFile,
      gitDiff: onGitDiff,
      goTo: onGoTo,
      refresh: refreshAll,
      goParent: () => void goParent(),
      goHome: () => void goHome(),
      goBack: () => historyNav(activePane, 'back'),
      goForward: () => historyNav(activePane, 'forward'),
      calcSizes: () => triggerCalcSizes(activePane),
      archive: () => void archiveOps.openArchiveDialog(),
      extract: archiveOps.openExtractDialog,
    }),
  );

  return {
    activePane,
    activePath,
    canBack,
    canForward,
    theme,
    realSelection,
    goBack: () => historyNav(activePane, 'back'),
    goForward: () => historyNav(activePane, 'forward'),
    otherPane,
    openSettings,
    cycleTheme,
    refreshAll,
    onEditFile,
    onGitDiff,
    onGoTo,
    onTrash,
    trashActive: isTrashPath(activePath),
    ...fileOps,
    ...archiveOps,
  };
};
