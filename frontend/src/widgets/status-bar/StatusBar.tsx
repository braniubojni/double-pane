import type { FC } from 'react';
import Box from '@mui/material/Box';
import Typography from '@mui/material/Typography';
import { useDirListing, useDiskUsage, useSettings } from '../../entities/file/queries';
import { isRemotePath } from '../../features/connections/helpers';
import { useFolderSizeStore } from '../../features/folder-size/folderSizeStore';
import { usePaneStore } from '../../features/pane/paneStore';
import { formatSize } from '../../shared/lib/format';
import { formatSelectionCaption, selectedEntryPaths } from './helpers';
import { DuplicateStatusChip } from './DuplicateStatusChip';
import { TransferStatusSegment } from './TransferStatusSegment';

export const StatusBar: FC = () => {
  const activePane = usePaneStore((s) => s.activePane);
  const path = usePaneStore((s) => s.getPath(s.activePane));
  const selection = usePaneStore((s) =>
    s.activePane === 'left' ? s.leftSelection : s.rightSelection,
  );
  const focus = usePaneStore((s) => (s.activePane === 'left' ? s.leftFocus : s.rightFocus));
  const folderSizes = useFolderSizeStore((s) => s.getSizes(activePane));
  const { data: settings } = useSettings();
  const listing = useDirListing(path || undefined, settings?.showHidden ?? false);
  const disk = useDiskUsage(path || undefined);
  const count = listing.data?.filter((e) => e.name !== '..').length ?? 0;
  const selectedPaths = selectedEntryPaths(selection, focus);
  const selectedCaption = formatSelectionCaption(selectedPaths, listing.data, folderSizes);
  const freeBytes = disk.data?.free;

  return (
    <Box
      data-testid="status-bar"
      sx={{
        display: 'flex',
        gap: 2,
        px: 1.5,
        py: 0.5,
        borderTop: 1,
        borderColor: 'divider',
        bgcolor: 'background.paper',
        alignItems: 'center',
      }}
    >
      <Typography data-testid="status-active-pane" variant="caption" color="text.secondary">
        Active: <strong>{activePane}</strong>
      </Typography>
      <Typography
        data-testid="status-path"
        variant="caption"
        color="text.secondary"
        noWrap
        sx={{ flex: 1, fontFamily: 'monospace', minWidth: 0 }}
      >
        {path}
      </Typography>
      <DuplicateStatusChip />
      <TransferStatusSegment />
      <Typography data-testid="status-items" variant="caption" color="text.secondary">
        Items: {count}
      </Typography>
      <Typography data-testid="status-selected" variant="caption" color="text.secondary">
        {selectedCaption}
      </Typography>
      {path && !isRemotePath(path) && freeBytes != null ? (
        <Typography data-testid="status-free" variant="caption" color="text.secondary">
          Free: {formatSize(freeBytes, false)}
        </Typography>
      ) : null}
    </Box>
  );
};
