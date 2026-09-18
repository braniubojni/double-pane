import type { AppSettings } from '../../entities/file/types';
import { getPaneGrid } from '../../pages/file-manager/helpers';
import { enterPaneTab, openActiveFolderInOtherPane } from '../../widgets/file-pane/helpers';
import { useFileOpsStore } from '../file-ops/fileOpsStore';
import type { FileOpsAction } from '../file-ops/types';
import { useGoToStore } from '../go-to/goToStore';
import { usePaneStore } from '../pane/paneStore';
import { useSearchStore } from '../search/searchStore';
import { useTerminalStore } from '../terminal/terminalStore';
import { useDialogStore } from '../ui/dialogStore';
import { useCommandPaletteStore } from './commandPaletteStore';

const FILE_OPS_IDS = new Set<string>([
  'copy',
  'paste',
  'move',
  'delete',
  'deletePermanent',
  'restoreTrash',
  'emptyTrash',
  'rename',
  'mkdir',
  'mkfile',
  'editFile',
  'gitDiff',
  'refresh',
  'goParent',
  'goHome',
  'goBack',
  'goForward',
  'calcSizes',
  'archive',
  'extract',
]);

export type ShortcutToggleFns = {
  toggleHidden: () => void;
  toggleExtensions: () => void;
};

export const shortcutToggles = (
  patchSettings: { mutate: (p: Partial<AppSettings>) => void },
  settings: Pick<AppSettings, 'showHidden' | 'showExtensions'> | undefined,
): ShortcutToggleFns => ({
  toggleHidden: () => patchSettings.mutate({ showHidden: !settings?.showHidden }),
  toggleExtensions: () => patchSettings.mutate({ showExtensions: !settings?.showExtensions }),
});

const runTabAction = (id: 'tabNew' | 'tabClose' | 'tabNext' | 'tabPrev'): void => {
  const s = usePaneStore.getState();
  const pane = s.activePane;
  if (id === 'tabNew') {
    const curPath = s.getPath(pane);
    enterPaneTab(pane, curPath);
    s.addTab(pane, curPath);
    return;
  }
  const tabs = s.getTabs(pane);
  const idx = s.getTabIndex(pane);
  if (id === 'tabClose') {
    const closingId = tabs[idx]?.id;
    if (!closingId || tabs.length < 2) return;
    const nextIdx = Math.min(Math.max(idx - 1, 0), tabs.length - 2);
    enterPaneTab(pane, tabs.filter((t) => t.id !== closingId)[nextIdx].path);
    s.closeTab(pane, closingId);
    return;
  }
  if (tabs.length < 2) return;
  const delta = id === 'tabNext' ? 1 : -1;
  const nextIdx = (idx + delta + tabs.length) % tabs.length;
  enterPaneTab(pane, tabs[nextIdx].path);
  s.selectTab(pane, tabs[nextIdx].id);
};

export const runShortcutAction = (id: string, toggles?: ShortcutToggleFns): void => {
  if (id === 'commandPalette') {
    useCommandPaletteStore.getState().openPalette();
    return;
  }
  if (id === 'goTo') {
    useGoToStore.getState().openGoTo();
    return;
  }
  if (id === 'openSearch') {
    useSearchStore.getState().openSearch();
    return;
  }
  if (id === 'openSettings') {
    useDialogStore.getState().openSettings();
    return;
  }
  if (id === 'openShortcuts') {
    useDialogStore.getState().openShortcuts();
    return;
  }
  if (id === 'toggleHidden') {
    toggles?.toggleHidden();
    return;
  }
  if (id === 'toggleExtensions') {
    toggles?.toggleExtensions();
    return;
  }
  if (id === 'toggleTerminal') {
    useTerminalStore.getState().toggleActive(usePaneStore.getState().activePane);
    return;
  }
  if (id === 'switchPane') {
    const s = usePaneStore.getState();
    s.setActivePane(s.activePane === 'left' ? 'right' : 'left');
    return;
  }
  if (id === 'selectAll') {
    getPaneGrid(usePaneStore.getState().activePane)?.__gfmSelectAll?.();
    return;
  }
  if (id === 'sameDirLeft' || id === 'sameDirRight') {
    // Always active → inactive, regardless of which arrow/id fired.
    openActiveFolderInOtherPane();
    return;
  }
  if (id === 'tabNew' || id === 'tabClose' || id === 'tabNext' || id === 'tabPrev') {
    runTabAction(id);
    return;
  }
  if (FILE_OPS_IDS.has(id)) {
    useFileOpsStore.getState().trigger(id as FileOpsAction);
  }
};
