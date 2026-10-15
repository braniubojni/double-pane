import Box from '@mui/material/Box';
import Dialog from '@mui/material/Dialog';
import DialogContent from '@mui/material/DialogContent';
import TextField from '@mui/material/TextField';
import Typography from '@mui/material/Typography';
import { useMemo, useState, type FC, type KeyboardEvent } from 'react';
import { usePatchSettings, useSettings, useShortcutDefs } from '../../entities/file/queries';
import {
  runShortcutAction,
  shortcutToggles,
} from '../../features/command-palette/runShortcutAction';
import { filterCommands } from './helpers';
import { bindingSx, listSx, paperSx, rowSx } from './styles';

type Props = {
  open: boolean;
  onClose: () => void;
};

export const CommandPalette: FC<Props> = ({ open, onClose }) => {
  const { data: defs } = useShortcutDefs();
  const patchSettings = usePatchSettings();
  const { data: settings } = useSettings();
  const [query, setQuery] = useState('');
  const [index, setIndex] = useState(0);

  const commands = useMemo(() => filterCommands(defs ?? [], query), [defs, query]);

  const run = (id: string) => {
    onClose();
    runShortcutAction(id, shortcutToggles(patchSettings, settings));
  };

  const onKeyDown = (e: KeyboardEvent) => {
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      setIndex((i) => Math.min(commands.length - 1, i + 1));
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      setIndex((i) => Math.max(0, i - 1));
    } else if (e.key === 'Enter' && commands[index]) {
      e.preventDefault();
      run(commands[index].id);
    }
  };

  if (!open && (query !== '' || index !== 0)) {
    setQuery('');
    setIndex(0);
  }

  const [prevQuery, setPrevQuery] = useState(query);
  if (query !== prevQuery) {
    setPrevQuery(query);
    setIndex(0);
  }

  return (
    <Dialog
      open={open}
      onClose={onClose}
      fullWidth
      maxWidth="sm"
      data-testid="dialog-command-palette"
      slotProps={{ paper: { sx: paperSx } }}
      disableRestoreFocus
    >
      <DialogContent sx={{ p: 0 }}>
        <TextField
          autoFocus
          fullWidth
          placeholder="Run a command…"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          onKeyDown={onKeyDown}
          data-testid="input-command-palette"
          variant="outlined"
          size="small"
          spellCheck={false}
          autoCorrect="off"
          autoCapitalize="off"
          autoComplete="off"
          sx={{ px: 1.5, pt: 1.5, pb: 1 }}
        />
        <Box sx={listSx}>
          {commands.length === 0 ? (
            <Typography variant="body2" color="text.secondary" sx={{ px: 2, py: 1 }}>
              No matches
            </Typography>
          ) : (
            commands.map((cmd, i) => (
              <Box
                key={cmd.id}
                sx={rowSx(i === index)}
                onClick={() => run(cmd.id)}
                onMouseEnter={() => setIndex(i)}
                ref={
                  i === index
                    ? (el: HTMLDivElement | null) => el?.scrollIntoView({ block: 'nearest' })
                    : undefined
                }
              >
                <Typography variant="body2" noWrap sx={{ fontWeight: 600, flex: 1, minWidth: 0 }}>
                  {cmd.label}
                </Typography>
                <Typography variant="caption" color="text.secondary" sx={bindingSx}>
                  {cmd.binding}
                </Typography>
              </Box>
            ))
          )}
        </Box>
      </DialogContent>
    </Dialog>
  );
};
