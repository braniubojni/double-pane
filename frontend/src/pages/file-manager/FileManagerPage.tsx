import Box from '@mui/material/Box';
import CircularProgress from '@mui/material/CircularProgress';
import { Suspense, lazy, type FC } from 'react';
import { ArchivePasswordDialog } from '../../features/archive/ArchivePasswordDialog';
import { DmgPasswordDialog } from '../../features/dmg/DmgPasswordDialog';
import { useExternalFileDrop } from '../../features/dnd/useExternalFileDrop';
import { useDuplicateEvents } from '../../features/duplicates/useDuplicateEvents';
import { useEditorStore } from '../../features/editor/editorStore';
import { useFileOpsStore } from '../../features/file-ops/fileOpsStore';
import { useTransferEvents } from '../../features/transfers/useTransferEvents';
import { useDialogStore } from '../../features/ui/dialogStore';
import { useAutoUpdateCheck } from '../../features/updates/hooks/useAutoUpdateCheck';
import { useVolumeEvents } from '../../features/volumes/useVolumeEvents';
import { CommandPaletteHost } from '../../widgets/command-palette/CommandPaletteHost';
import { DuplicatesHost } from '../../widgets/duplicates/DuplicatesHost';
import { FileContextMenu } from '../../widgets/file-pane/FileContextMenu';
import { FilePane } from '../../widgets/file-pane/FilePane';
import { GoToHost } from '../../widgets/go-to/GoToHost';
import { AppMenuBar } from '../../widgets/menu/AppMenuBar';
import { SearchHost } from '../../widgets/search/SearchHost';
import { StatusBar } from '../../widgets/status-bar/StatusBar';
import { Toolbar } from '../../widgets/toolbar/Toolbar';
import { useFileManagerKeyboard } from './hooks/useFileManagerKeyboard';
import { useInitGridPrefs } from './hooks/useInitGridPrefs';
import { useInitPaneTabs } from './hooks/useInitPaneTabs';
import { useMouseNavButtons } from './hooks/useMouseNavButtons';
import { usePersistGridPrefs } from './hooks/usePersistGridPrefs';
import { usePersistPaneTabs } from './hooks/usePersistPaneTabs';
import { loadingSx, pageRootSx, panesRowSx } from './styles';

const SettingsDialog = lazy(() => import('../../features/settings/SettingsDialog'));
const ShortcutsDialog = lazy(() => import('../../features/shortcuts/ShortcutsDialog'));
const EditorWorkspace = lazy(() =>
  import('../../widgets/editor/EditorWorkspace').then((m) => ({ default: m.EditorWorkspace })),
);

export const FileManagerPage: FC = () => {
  const tabsReady = useInitPaneTabs();
  const prefsReady = useInitGridPrefs();
  const ready = tabsReady && prefsReady;
  usePersistPaneTabs(ready);
  usePersistGridPrefs(prefsReady);
  useFileManagerKeyboard();
  useMouseNavButtons();
  useAutoUpdateCheck(ready);
  useExternalFileDrop(ready);
  useTransferEvents();
  useDuplicateEvents();
  useVolumeEvents();

  const settingsOpen = useDialogStore((s) => s.settingsOpen);
  const shortcutsOpen = useDialogStore((s) => s.shortcutsOpen);
  const closeSettings = useDialogStore((s) => s.closeSettings);
  const closeShortcuts = useDialogStore((s) => s.closeShortcuts);
  const trigger = useFileOpsStore((s) => s.trigger);
  const editorOpen = useEditorStore((s) => s.open);

  if (!ready) {
    return (
      <Box sx={loadingSx}>
        <CircularProgress />
      </Box>
    );
  }

  return (
    <Box data-testid="app-ready" sx={pageRootSx}>
      {!editorOpen && (
        <AppMenuBar
          onNewFolder={() => trigger('mkdir')}
          onNewFile={() => trigger('mkfile')}
          onEditFile={() => trigger('editFile')}
          onGitDiff={() => trigger('gitDiff')}
          onRename={() => trigger('rename')}
          onDelete={() => trigger('delete')}
        />
      )}
      {!editorOpen && <Toolbar />}
      {editorOpen ? (
        <Suspense
          fallback={
            <Box sx={loadingSx}>
              <CircularProgress />
            </Box>
          }
        >
          <EditorWorkspace />
        </Suspense>
      ) : (
        <Box sx={panesRowSx}>
          <FilePane id="left" />
          <FilePane id="right" />
          <FileContextMenu />
        </Box>
      )}
      {!editorOpen && <StatusBar />}
      <DmgPasswordDialog />
      <ArchivePasswordDialog />
      <GoToHost />
      <CommandPaletteHost />
      <SearchHost />
      <DuplicatesHost />

      <Suspense fallback={null}>
        {settingsOpen && <SettingsDialog open={settingsOpen} onClose={closeSettings} />}
        {shortcutsOpen && <ShortcutsDialog open={shortcutsOpen} onClose={closeShortcuts} />}
      </Suspense>
    </Box>
  );
};
