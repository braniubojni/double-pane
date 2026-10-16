import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import Dialog from '@mui/material/Dialog';
import DialogContent from '@mui/material/DialogContent';
import DialogTitle from '@mui/material/DialogTitle';
import LinearProgress from '@mui/material/LinearProgress';
import Typography from '@mui/material/Typography';
import type { FC } from 'react';
import { useTreeUsageEvents } from '../../features/disk-treemap/useTreeUsageEvents';
import { useTreemapStore } from '../../features/disk-treemap/treemapStore';
import { usePaneStore } from '../../features/pane/paneStore';
import { FileService } from '../../shared/api/bindings';
import { formatSize } from '../../shared/lib/format';
import { parentPath } from '../toolbar/helpers';
import { TreemapMap } from './TreemapMap';
import { crumbRowSx, dialogContentSx, dialogPaperSx } from './styles';
import { findUsageNode, zoomChain } from './tree';

const baseName = (path: string): string => path.split(/[/\\]/).pop() || path;

const openFile = (path: string): void => {
  const pane = usePaneStore.getState();
  const id = pane.activePane;
  const parent = parentPath(path);
  if (pane.getPath(id) !== parent) pane.navigate(id, parent);
  pane.setFocus(id, path);
  pane.setSelection(id, [path]);
  useTreemapStore.getState().closeDialog();
};

export const TreemapDialog: FC = () => {
  useTreeUsageEvents();
  const close = useTreemapStore((s) => s.closeDialog);
  const drill = useTreemapStore((s) => s.drill);
  const root = useTreemapStore((s) => s.root);
  const scanning = useTreemapStore((s) => s.scanning);
  const progress = useTreemapStore((s) => s.progress);
  const callError = useTreemapStore((s) => s.callError);
  const jobId = useTreemapStore((s) => s.jobId);
  const zoomPath = useTreemapStore((s) => s.zoomPath);
  const setZoom = useTreemapStore((s) => s.setZoom);
  const payload = useTreemapStore((s) => s.cache[s.root]);

  const errorText = payload?.error || callError;
  const tree = !scanning && !errorText ? payload?.tree : undefined;
  const empty = tree != null && payload != null && payload.total === 0;
  const view = tree && zoomPath ? (findUsageNode(tree, zoomPath) ?? tree) : tree;
  const chain = tree ? zoomChain(tree, view && view !== tree ? view.path : '') : [];

  const onOpenDir = (path: string) => {
    if (!tree || !path) return;
    if (path === zoomPath) {
      setZoom('');
      return;
    }
    const node = findUsageNode(tree, path);
    if (node?.kids?.length) {
      setZoom(path);
      return;
    }
    drill(path);
  };

  return (
    <Dialog
      open
      onClose={close}
      fullWidth
      maxWidth="lg"
      data-testid="dialog-treemap"
      slotProps={{ paper: { sx: dialogPaperSx } }}
    >
      <DialogTitle>Disk usage — {baseName(root)}</DialogTitle>
      <DialogContent sx={dialogContentSx}>
        {tree && payload && (
          <Typography variant="body2">
            {formatSize(payload.total, false)} · {payload.files} files
          </Typography>
        )}
        {payload?.truncated && (
          <Typography variant="body2">
            Map is abbreviated. Open a folder for more detail.
          </Typography>
        )}
        {(payload?.deniedCount ?? 0) > 0 && (
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, flexWrap: 'wrap' }}>
            <Typography variant="body2">
              {payload?.deniedCount} folders could not be read, so their size is missing. Other is
              everything under 1% of this folder, grouped together.
            </Typography>
            <Button
              size="small"
              data-testid="btn-treemap-privacy"
              onClick={() => {
                void FileService.OpenPrivacySettings();
              }}
            >
              Privacy settings
            </Button>
          </Box>
        )}
        {chain.length > 0 && (
          <Box sx={crumbRowSx}>
            {chain.map((node) => (
              <Button
                key={node.path || node.name}
                size="small"
                onClick={() => {
                  if (!tree || node.path === tree.path || node.path === zoomPath) setZoom('');
                  else setZoom(node.path);
                }}
              >
                {node.name}
              </Button>
            ))}
          </Box>
        )}
        {scanning && (
          <>
            <LinearProgress />
            <Typography variant="body2">
              {progress
                ? `${baseName(progress.current)} · ${formatSize(progress.bytes, false)}`
                : 'Scanning…'}
            </Typography>
            <Button
              data-testid="btn-treemap-cancel"
              onClick={() => {
                if (jobId) void FileService.CancelJob(jobId);
              }}
            >
              Cancel
            </Button>
          </>
        )}
        {errorText && !scanning && <Typography variant="body2">{errorText}</Typography>}
        {empty && <Typography variant="body2">This folder is empty.</Typography>}
        {tree && payload && view && !empty && (
          <TreemapMap
            kids={view.kids ?? []}
            total={payload.total}
            frame={zoomPath && view !== tree ? view.name : ''}
            onReset={() => setZoom('')}
            onOpenDir={onOpenDir}
            onOpenFile={openFile}
          />
        )}
      </DialogContent>
    </Dialog>
  );
};
