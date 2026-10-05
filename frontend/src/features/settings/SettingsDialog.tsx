import FolderOpenIcon from '@mui/icons-material/FolderOpen';
import OpenInNewIcon from '@mui/icons-material/OpenInNew';
import Button from '@mui/material/Button';
import Dialog from '@mui/material/Dialog';
import DialogActions from '@mui/material/DialogActions';
import DialogContent from '@mui/material/DialogContent';
import DialogTitle from '@mui/material/DialogTitle';
import FormControl from '@mui/material/FormControl';
import FormControlLabel from '@mui/material/FormControlLabel';
import InputLabel from '@mui/material/InputLabel';
import MenuItem from '@mui/material/MenuItem';
import Select from '@mui/material/Select';
import Stack from '@mui/material/Stack';
import Switch from '@mui/material/Switch';
import TextField from '@mui/material/TextField';
import Tooltip from '@mui/material/Tooltip';
import Typography from '@mui/material/Typography';
import { useEffect, useState, type FC } from 'react';
import { useSaveSettings, useSettings } from '../../entities/file/queries';
import type { AppSettings } from '../../entities/file/types';
import { SettingsService } from '../../shared/api/bindings';
import { errMessage } from '../../shared/lib/format';
import { useSnack } from '../../shared/ui/SnackbarHost';
import { UpdatesSection } from './UpdatesSection';

interface Props {
  open: boolean;
  onClose: () => void;
}

const SettingsDialog: FC<Props> = ({ open, onClose }) => {
  const { data } = useSettings();
  const save = useSaveSettings();
  const show = useSnack((s) => s.show);
  const [draft, setDraft] = useState<AppSettings | null>(null);

  useEffect(() => {
    if (open && data) setDraft({ ...data });
  }, [open, data]);

  if (!draft) return null;

  const onSave = () => {
    save.mutate(draft, {
      onSuccess: () => {
        show('Settings saved', 'success');
        onClose();
      },
      onError: (e) => show(errMessage(e), 'error'),
    });
  };

  const reveal = () => {
    void SettingsService.GetSettingsPath()
      .then((p) => SettingsService.RevealInOS(p))
      .catch((e) => show(errMessage(e), 'error'));
  };

  const openFile = () => {
    void SettingsService.GetSettingsPath()
      .then((p) => SettingsService.OpenInOS(p))
      .catch((e) => show(errMessage(e), 'error'));
  };

  return (
    <Dialog data-testid="dialog-settings" open={open} onClose={onClose} fullWidth maxWidth="sm">
      <DialogTitle>Settings</DialogTitle>
      <DialogContent>
        <Stack spacing={2} sx={{ mt: 1 }}>
          <Tooltip title="Color scheme. System follows the OS appearance.">
            <FormControl fullWidth size="small">
              <InputLabel id="theme-label">Theme</InputLabel>
              <Select
                data-testid="settings-theme"
                labelId="theme-label"
                label="Theme"
                value={draft.theme}
                onChange={(e) => setDraft({ ...draft, theme: e.target.value })}
              >
                <MenuItem value="system">System</MenuItem>
                <MenuItem value="dark">Dark</MenuItem>
                <MenuItem value="light">Light</MenuItem>
              </Select>
            </FormControl>
          </Tooltip>

          <Tooltip title="When enabled, files and folders whose names start with a dot are shown.">
            <FormControlLabel
              control={
                <Switch
                  data-testid="settings-show-hidden"
                  checked={draft.showHidden}
                  onChange={(_, v) => setDraft({ ...draft, showHidden: v })}
                />
              }
              label="Show hidden files"
            />
          </Tooltip>

          <Tooltip title="When enabled, file extensions are shown in the Name column.">
            <FormControlLabel
              control={
                <Switch
                  data-testid="settings-show-extensions"
                  checked={draft.showExtensions}
                  onChange={(_, v) => setDraft({ ...draft, showExtensions: v })}
                />
              }
              label="Show file extensions"
            />
          </Tooltip>

          <Tooltip title="Color file names by git status when the folder is inside a local repository. Uses one cheap git call per directory (no disk-wide .git search).">
            <FormControlLabel
              control={
                <Switch
                  data-testid="settings-show-git-status"
                  checked={draft.showGitStatus}
                  onChange={(_, v) => setDraft({ ...draft, showGitStatus: v })}
                />
              }
              label="Show git status colors"
            />
          </Tooltip>

          <Tooltip title="Open text and code in the built-in editor. Images, PDFs, Office files, and other binary types always use the system app.">
            <FormControlLabel
              control={
                <Switch
                  data-testid="settings-use-builtin-editor"
                  checked={draft.useBuiltInEditor}
                  onChange={(_, v) => setDraft({ ...draft, useBuiltInEditor: v })}
                />
              }
              label="Use built-in editor"
            />
          </Tooltip>

          <UpdatesSection draft={draft} onChange={setDraft} />

          <Typography variant="overline" color="text.secondary" sx={{ mt: 1 }}>
            Vaults
          </Typography>
          <Tooltip title="Auto-lock an unlocked folder/file vault after this many minutes with no vault activity. 0 disables idle locking (vaults still lock when you leave them in both panes, or on quit).">
            <TextField
              size="small"
              type="number"
              label="Idle lock (minutes)"
              data-testid="settings-vault-idle-minutes"
              value={draft.vaultIdleLockMinutes}
              slotProps={{ htmlInput: { min: 0 } }}
              onChange={(e) =>
                setDraft({
                  ...draft,
                  vaultIdleLockMinutes: Math.max(0, Number(e.target.value) || 0),
                })
              }
            />
          </Tooltip>

          <Typography variant="caption" color="text.secondary">
            Pane paths are saved automatically when you navigate. Bookmarks use SQLite separately.
          </Typography>
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
        <Button
          data-testid="settings-save"
          variant="contained"
          onClick={onSave}
          disabled={save.isPending}
        >
          Save
        </Button>
      </DialogActions>
    </Dialog>
  );
};

export default SettingsDialog;
