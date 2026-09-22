import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import CircularProgress from '@mui/material/CircularProgress';
import FormControlLabel from '@mui/material/FormControlLabel';
import Stack from '@mui/material/Stack';
import Switch from '@mui/material/Switch';
import Typography from '@mui/material/Typography';
import { useEffect, useState, type FC } from 'react';
import type { AppSettings } from '../../entities/file/types';
import { useUpdateActions } from '../updates/hooks/useUpdateActions';
import { UpdateService } from '../../shared/api/bindings';
import { hostOS } from '../../shared/lib/hostOS';

type Props = {
  draft: AppSettings;
  onChange: (next: AppSettings) => void;
};

export const UpdatesSection: FC<Props> = ({ draft, onChange }) => {
  const [version, setVersion] = useState('…');
  const { check, openReleases, busy } = useUpdateActions();

  useEffect(() => {
    void UpdateService.GetVersion()
      .then(setVersion)
      .catch(() => setVersion('unknown'));
  }, []);

  return (
    <Stack spacing={1.5} data-testid="settings-updates">
      <Typography variant="subtitle2">Updates</Typography>
      <Typography variant="body2" color="text.secondary">
        Version: <strong data-testid="app-version">{version}</strong>
      </Typography>

      <FormControlLabel
        control={
          <Switch
            data-testid="settings-auto-check-updates"
            checked={draft.autoCheckUpdates}
            onChange={(_, v) => onChange({ ...draft, autoCheckUpdates: v })}
          />
        }
        label="Check for updates automatically (every 10 days)"
      />

      <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 1, alignItems: 'center' }}>
        <Button
          size="small"
          variant="outlined"
          data-testid="btn-check-updates"
          disabled={busy}
          onClick={() => void check()}
          startIcon={busy ? <CircularProgress size={14} /> : undefined}
        >
          Check for updates
        </Button>
        <Button size="small" onClick={openReleases}>
          Open releases page
        </Button>
      </Box>

      <Typography variant="caption" color="text.secondary">
        {hostOS() === 'darwin'
          ? 'Checks GitHub and opens the release page in your browser — download the .dmg manually.'
          : 'Checks open the built-in update window (download, verify, restart to apply).'}
      </Typography>
    </Stack>
  );
};
