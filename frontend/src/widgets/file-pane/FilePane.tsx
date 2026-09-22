import Box from '@mui/material/Box';
import LinearProgress from '@mui/material/LinearProgress';
import type { FC } from 'react';
import { useConnectRequestStore } from '../../features/connections/connectRequestStore';
import { errMessage } from '../../shared/lib/format';
import { PaneTerminal } from '../terminal/PaneTerminal';
import { FileTable } from './FileTable';
import { useFilePane } from './hooks';
import { PaneHeader } from './PaneHeader';
import { PaneTabs } from './PaneTabs';
import { PathBar } from './PathBar';
import { paneBodySx, paneRootSx } from './styles';
import type { FilePaneProps } from './types';

export const FilePane: FC<FilePaneProps> = ({ id }) => {
  const p = useFilePane(id);
  const connecting = useConnectRequestStore((s) => s.connecting[id]);

  return (
    <Box data-testid={`pane-${id}`} sx={paneRootSx(p.active)}>
      <PaneHeader
        id={id}
        active={p.active}
        job={p.job}
        terminalOpen={p.terminalOpen}
        onActivate={p.activatePane}
        onCancelJob={p.cancelJob}
        onCalcSizes={p.onCalcSizes}
        onRefresh={p.onRefresh}
        onToggleTerminal={() => {
          p.setActivePane(id);
          p.toggleTerminal(id);
        }}
      />
      <PaneTabs
        id={id}
        tabs={p.tabs}
        activeTabId={p.activeTabId}
        onSelectTab={p.onSelectTab}
        onCloseTab={p.onCloseTab}
        onAddTab={p.onAddTab}
        onAddTabAt={p.onAddTabAt}
      />
      <PathBar
        paneId={id}
        path={p.path}
        onNavigate={p.navigate}
        onUp={p.goUp}
        onHome={p.goHome}
        onFocusPane={p.activatePane}
      />
      {connecting && <LinearProgress data-testid={`pane-connecting-${id}`} />}
      <Box sx={paneBodySx}>
        <FileTable
          paneId={id}
          panePath={p.path}
          entries={p.listing.data}
          isLoading={p.listing.isLoading || p.listing.isFetching}
          isError={p.listing.isError}
          errorMessage={p.listing.error ? errMessage(p.listing.error) : undefined}
          selected={p.selection}
          focused={p.focused}
          active={p.active}
          showExtensions={p.showExtensions}
          gitByName={p.gitByName}
          folderSizes={p.folderSizes}
          deniedPaths={p.deniedPaths}
          onSelect={(paths) => p.setSelection(id, paths)}
          onFocus={(path, opts) => p.setFocus(id, path, opts)}
          onToggleMulti={(path) => p.toggleMultiSelect(id, path)}
          onSelectRange={(ordered, to) => p.selectRange(id, ordered, to)}
          onActivate={p.activatePane}
          onOpen={p.openEntry}
          onDropPaths={p.onDropPaths}
        />
      </Box>
      {p.terminalOpen && (
        <PaneTerminal
          paneId={id}
          cwd={p.path}
          height={p.terminalHeight}
          onActivate={p.activatePane}
        />
      )}
    </Box>
  );
};
