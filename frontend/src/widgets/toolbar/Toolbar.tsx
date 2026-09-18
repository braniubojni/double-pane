import type { FC } from 'react';
import { swapPanes } from '../../pages/file-manager/helpers';
import { openActiveFolderInOtherPane } from '../file-pane/helpers';
import { ArchiveDialog } from './components/ArchiveDialog';
import { DeleteDialogs } from './components/DeleteDialogs';
import { ExtractDialog } from './components/ExtractDialog';
import { NameDialog } from './components/NameDialog';
import { ToolbarBar } from './components/ToolbarBar';
import { useToolbarActions } from './hooks/useToolbarActions';

export const Toolbar: FC = () => {
  const a = useToolbarActions();

  return (
    <>
      <ToolbarBar
        activePane={a.activePane}
        canBack={a.canBack}
        canForward={a.canForward}
        theme={a.theme}
        otherPaneLabel={a.otherPane(a.activePane)}
        onBack={a.goBack}
        onForward={a.goForward}
        onSwapPanes={swapPanes}
        onSameDir={openActiveFolderInOtherPane}
        onCopy={a.onCopy}
        onMove={a.onMove}
        onMkdir={a.onMkdir}
        onMkfile={a.onMkfile}
        onEditFile={a.onEditFile}
        onGitDiff={a.onGitDiff}
        onRename={a.onRename}
        onDelete={a.onDelete}
        onDeletePermanent={a.onDeletePermanent}
        onRestoreTrash={a.onRestoreTrash}
        onEmptyTrash={a.onEmptyTrash}
        onArchive={() => void a.openArchiveDialog()}
        onExtract={a.openExtractDialog}
        onBookmark={a.onBookmark}
        onRefresh={a.refreshAll}
        onCycleTheme={a.cycleTheme}
        onSettings={a.openSettings}
        onTrash={a.onTrash}
        trashActive={a.trashActive}
      />

      <NameDialog
        testId="dialog-mkdir"
        title="Create folder"
        label="Folder name"
        inputTestId="input-mkdir-name"
        confirmTestId="btn-mkdir-confirm"
        confirmLabel="Create"
        state={a.mkdir}
        dispatch={a.dispatchMkdir}
        onConfirm={a.confirmMkdir}
      />

      <NameDialog
        testId="dialog-mkfile"
        title="Create file"
        label="File name"
        inputTestId="input-mkfile-name"
        confirmTestId="btn-mkfile-confirm"
        confirmLabel="Create"
        state={a.mkfile}
        dispatch={a.dispatchMkfile}
        onConfirm={a.confirmMkfile}
      />

      <NameDialog
        testId="dialog-rename"
        title="Rename"
        label="New name"
        inputTestId="input-rename-name"
        confirmTestId="btn-rename-confirm"
        confirmLabel="Rename"
        state={a.rename}
        dispatch={a.dispatchRename}
        onConfirm={a.confirmRename}
        selectStem
      />

      <DeleteDialogs
        del={a.del}
        dispatch={a.dispatchDelete}
        paths={a.realSelection}
        remote={a.remote}
        deleteBtnRef={a.deleteBtnRef}
        onConfirm={a.confirmDelete}
        onEmpty={a.confirmEmptyTrash}
      />

      <ArchiveDialog
        archive={a.archive}
        dispatch={a.dispatchArchive}
        selectionCount={a.realSelection.length}
        activePath={a.activePath}
        onConfirm={() => void a.confirmArchive()}
      />

      <ExtractDialog
        extract={a.extract}
        dispatch={a.dispatchExtract}
        selectionCount={a.realSelection.length}
        onConfirm={() => void a.confirmExtract()}
      />
    </>
  );
};
