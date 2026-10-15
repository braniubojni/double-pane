import FolderOpenIcon from '@mui/icons-material/FolderOpen';
import OpenInNewIcon from '@mui/icons-material/OpenInNew';
import Button from '@mui/material/Button';
import Dialog from '@mui/material/Dialog';
import DialogActions from '@mui/material/DialogActions';
import DialogContent from '@mui/material/DialogContent';
import DialogTitle from '@mui/material/DialogTitle';
import Stack from '@mui/material/Stack';
import Table from '@mui/material/Table';
import TableBody from '@mui/material/TableBody';
import TableCell from '@mui/material/TableCell';
import TableHead from '@mui/material/TableHead';
import TableRow from '@mui/material/TableRow';
import TextField from '@mui/material/TextField';
import Typography from '@mui/material/Typography';
import { useState, type FC } from 'react';
import { useSaveShortcuts, useShortcutDefs } from '../../entities/file/queries';
import { SettingsService } from '../../shared/api/bindings';
import { errMessage } from '../../shared/lib/format';
import { useSnack } from '../../shared/ui/SnackbarHost';

interface Props {
  open: boolean;
  onClose: () => void;
}

const ShortcutsDialog: FC<Props> = ({ open, onClose }) => {
  const { data } = useShortcutDefs();
  const save = useSaveShortcuts();
  const show = useSnack((s) => s.show);
  const [bindings, setBindings] = useState<Record<string, string>>({});

  const [synced, setSynced] = useState<{ open: boolean; data?: typeof data }>({ open: false });
  if (synced.open !== open || synced.data !== data) {
    setSynced({ open, data });
    if (open && data) {
      const map: Record<string, string> = {};
      for (const d of data) map[d.id] = d.binding;
      setBindings(map);
    }
  }

  const onSave = () => {
    save.mutate(bindings, {
      onSuccess: () => {
        show('Shortcuts saved', 'success');
        onClose();
      },
      onError: (e) => show(errMessage(e), 'error'),
    });
  };

  const reveal = () => {
    void SettingsService.GetShortcutsPath()
      .then((p) => SettingsService.RevealInOS(p))
      .catch((e) => show(errMessage(e), 'error'));
  };

  const openFile = () => {
    void SettingsService.GetShortcutsPath()
      .then((p) => SettingsService.OpenInOS(p))
      .catch((e) => show(errMessage(e), 'error'));
  };

  return (
    <Dialog data-testid="dialog-shortcuts" open={open} onClose={onClose} fullWidth maxWidth="md">
      <DialogTitle>Keyboard shortcuts</DialogTitle>
      <DialogContent>
        <Stack spacing={1} sx={{ mt: 1 }}>
          <Typography variant="body2" color="text.secondary">
            Bindings use <code>Mod</code> for Cmd (macOS) / Ctrl (Windows/Linux). Examples:{' '}
            <code>F5</code>, <code>Mod+Shift+C</code>, <code>Alt+ArrowUp</code>.
          </Typography>
          <Table size="small">
            <TableHead>
              <TableRow>
                <TableCell>Action</TableCell>
                <TableCell>Description</TableCell>
                <TableCell width={180}>Shortcut</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {(data ?? []).map((row) => (
                <TableRow key={row.id}>
                  <TableCell>{row.label}</TableCell>
                  <TableCell>
                    <Typography variant="caption" color="text.secondary">
                      {row.description}
                    </Typography>
                  </TableCell>
                  <TableCell>
                    <TextField
                      size="small"
                      fullWidth
                      value={bindings[row.id] ?? ''}
                      onChange={(e) => setBindings((b) => ({ ...b, [row.id]: e.target.value }))}
                      slotProps={{ htmlInput: { spellCheck: false } }}
                    />
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </Stack>
      </DialogContent>
      <DialogActions sx={{ px: 3, pb: 2, flexWrap: 'wrap', gap: 1 }}>
        <Button startIcon={<FolderOpenIcon />} onClick={reveal}>
          Reveal file
        </Button>
        <Button startIcon={<OpenInNewIcon />} onClick={openFile}>
          Open file
        </Button>
        <span style={{ flex: 1 }} />
        <Button onClick={onClose}>Cancel</Button>
        <Button variant="contained" onClick={onSave} disabled={save.isPending}>
          Save
        </Button>
      </DialogActions>
    </Dialog>
  );
};

export default ShortcutsDialog;
