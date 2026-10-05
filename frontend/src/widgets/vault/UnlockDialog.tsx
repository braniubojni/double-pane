import Button from '@mui/material/Button';
import Dialog from '@mui/material/Dialog';
import DialogActions from '@mui/material/DialogActions';
import DialogContent from '@mui/material/DialogContent';
import DialogContentText from '@mui/material/DialogContentText';
import DialogTitle from '@mui/material/DialogTitle';
import TextField from '@mui/material/TextField';
import Typography from '@mui/material/Typography';
import { type FC, type FormEvent, useState } from 'react';
import { useDetectVault, useUnlockVault } from '../../entities/file/queries';
import { usePaneStore } from '../../features/pane/paneStore';
import { useVaultDialogStore } from '../../features/vault/vaultDialogStore';
import { enterPaneTab } from '../file-pane/helpers';
import { errMessage } from '../../shared/lib/format';
import { useSnack } from '../../shared/ui/SnackbarHost';

export const UnlockDialog: FC = () => {
  const open = useVaultDialogStore((s) => s.mode === 'unlock');
  const path = useVaultDialogStore((s) => s.path);
  const paneId = useVaultDialogStore((s) => s.paneId);
  const isFile = useVaultDialogStore((s) => s.isFile);
  const close = useVaultDialogStore((s) => s.close);
  const navigate = usePaneStore((s) => s.navigate);
  const show = useSnack((s) => s.show);
  const unlock = useUnlockVault();
  const { data: info } = useDetectVault(path, open);

  const [password, setPassword] = useState('');
  const [error, setError] = useState('');

  const name = path.split(/[/\\]/).pop() || path;

  const reset = () => {
    setPassword('');
    setError('');
  };

  const onClose = () => {
    reset();
    close();
  };

  const submit = (e?: FormEvent) => {
    e?.preventDefault();
    if (!password || unlock.isPending) return;
    setError('');
    unlock.mutate(
      { path, password },
      {
        onSuccess: () => {
          reset();
          close();
          show('Unlocked', 'success');
          // .dpenc is a file — unlocking it never "enters" it (spec V1).
          if (!isFile) {
            enterPaneTab(paneId, path);
            navigate(paneId, path);
          }
        },
        onError: (err) => setError(errMessage(err) || 'Invalid password'),
      },
    );
  };

  return (
    <Dialog data-testid="dialog-vault-unlock" open={open} onClose={onClose} fullWidth maxWidth="xs">
      <form onSubmit={submit}>
        <DialogTitle>Unlock vault</DialogTitle>
        <DialogContent>
          <DialogContentText sx={{ mb: 1 }}>
            Enter the password for <strong>{name}</strong>.
          </DialogContentText>
          {info?.hint ? (
            <Typography variant="body2" color="text.secondary" sx={{ mb: 1 }}>
              Hint: {info.hint}
            </Typography>
          ) : null}
          <TextField
            autoFocus
            fullWidth
            margin="dense"
            type="password"
            label="Password"
            data-testid="input-vault-unlock-password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
          />
          {error ? (
            <Typography variant="body2" color="error" sx={{ mt: 1 }}>
              {error}
            </Typography>
          ) : null}
        </DialogContent>
        <DialogActions>
          <Button type="button" onClick={onClose}>
            Cancel
          </Button>
          <Button
            type="submit"
            variant="contained"
            data-testid="btn-vault-unlock-confirm"
            disabled={!password || unlock.isPending}
          >
            Unlock
          </Button>
        </DialogActions>
      </form>
    </Dialog>
  );
};
