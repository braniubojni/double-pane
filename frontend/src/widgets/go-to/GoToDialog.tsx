import FolderIcon from '@mui/icons-material/Folder';
import InsertDriveFileIcon from '@mui/icons-material/InsertDriveFile';
import Box from '@mui/material/Box';
import Dialog from '@mui/material/Dialog';
import DialogContent from '@mui/material/DialogContent';
import TextField from '@mui/material/TextField';
import Typography from '@mui/material/Typography';
import { useEffect, useMemo, useState, type FC, type KeyboardEvent } from 'react';
import { useSearchTree, useSettings } from '../../entities/file/queries';
import type { SearchHit } from '../../entities/file/types';
import { useEditorStore } from '../../features/editor/editorStore';
import { usePaneStore } from '../../features/pane/paneStore';
import { openDocument } from '../../shared/lib/openDocument';
import { useSnack } from '../../shared/ui/SnackbarHost';
import { enterPaneTab } from '../file-pane/helpers';
import { listSx, paperSx, rowSx } from './styles';

type Props = {
  open: boolean;
  onClose: () => void;
};

export const GoToDialog: FC<Props> = ({ open, onClose }) => {
  const activePane = usePaneStore((s) => s.activePane);
  const path = usePaneStore((s) => s.getPath(s.activePane));
  const navigate = usePaneStore((s) => s.navigate);
  const { data: settings } = useSettings();
  const show = useSnack((s) => s.show);
  const openWorkspace = useEditorStore((s) => s.openWorkspace);
  const [query, setQuery] = useState('');
  const [debounced, setDebounced] = useState('');
  const [index, setIndex] = useState(0);

  const search = useSearchTree(path || undefined, debounced, settings?.showHidden ?? false, open);
  const hits = useMemo(() => search.data ?? [], [search.data]);

  const select = (hit: SearchHit) => {
    onClose();
    if (hit.isDir) {
      enterPaneTab(activePane, hit.path);
      navigate(activePane, hit.path);
      return;
    }
    openDocument({
      path: hit.path,
      name: hit.name,
      useBuiltInEditor: settings?.useBuiltInEditor !== false,
      openWorkspace,
      show,
    });
  };

  const onKeyDown = (e: KeyboardEvent) => {
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      setIndex((i) => Math.min(hits.length - 1, i + 1));
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      setIndex((i) => Math.max(0, i - 1));
    } else if (e.key === 'Enter' && hits[index]) {
      e.preventDefault();
      select(hits[index]);
    }
  };

  useEffect(() => {
    if (!open) {
      setQuery('');
      setDebounced('');
      setIndex(0);
      return;
    }
    const t = setTimeout(() => setDebounced(query), 180);
    return () => clearTimeout(t);
  }, [query, open]);

  useEffect(() => {
    setIndex(0);
  }, [debounced, hits.length]);

  return (
    <Dialog
      open={open}
      onClose={onClose}
      fullWidth
      maxWidth="sm"
      data-testid="dialog-goto"
      slotProps={{ paper: { sx: paperSx } }}
      disableRestoreFocus
      autoCorrect=""
    >
      <DialogContent sx={{ p: 0 }}>
        <TextField
          autoFocus
          fullWidth
          placeholder="Go to file or folder…"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          onKeyDown={onKeyDown}
          data-testid="input-goto"
          variant="outlined"
          size="small"
          spellCheck={false}
          autoCorrect="off"
          autoCapitalize="off"
          autoComplete="off"
          sx={{ px: 1.5, pt: 1.5, pb: 1 }}
        />
        <Box sx={listSx}>
          {hits.length === 0 ? (
            <Typography variant="body2" color="text.secondary" sx={{ px: 2, py: 1 }}>
              {search.isFetching ? 'Searching…' : 'No matches'}
            </Typography>
          ) : (
            hits.map((hit, i) => (
              <Box
                key={hit.path}
                sx={rowSx(i === index)}
                onClick={() => select(hit)}
                onMouseEnter={() => setIndex(i)}
                data-testid={`goto-row-${hit.name}`}
              >
                {hit.isDir ? (
                  <FolderIcon fontSize="small" color="warning" />
                ) : (
                  <InsertDriveFileIcon fontSize="small" />
                )}
                <Box sx={{ minWidth: 0, flex: 1 }}>
                  <Typography variant="body2" noWrap sx={{ fontWeight: 600 }}>
                    {hit.name}
                  </Typography>
                  <Typography variant="caption" color="text.secondary" noWrap>
                    {hit.relPath}
                  </Typography>
                </Box>
              </Box>
            ))
          )}
        </Box>
      </DialogContent>
    </Dialog>
  );
};
