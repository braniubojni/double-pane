import Button from '@mui/material/Button';
import Dialog from '@mui/material/Dialog';
import DialogActions from '@mui/material/DialogActions';
import DialogContent from '@mui/material/DialogContent';
import DialogContentText from '@mui/material/DialogContentText';
import DialogTitle from '@mui/material/DialogTitle';
import TextField from '@mui/material/TextField';
import Typography from '@mui/material/Typography';
import { type FC, type FormEvent, useState } from 'react';
import { useStartRemoveVault } from '../../entities/file/queries';
import { usePaneJobStore } from '../../features/jobs/paneJobStore';
import { useVaultDialogStore } from '../../features/vault/vaultDialogStore';
import { errMessage } from '../../shared/lib/format';

export const RemoveDialog: FC = () => {
  const open = useVaultDialogStore((s) => s.mode === 'remove');
  const path = useVaultDialogStore((s) => s.path);
  const paneId = useVaultDialogStore((s) => s.paneId);
  const close = useVaultDialogStore((s) => s.close);
  const startJob = usePaneJobStore((s) => s.start);
  const removeVault = useStartRemoveVault();

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
    if (!password || removeVault.isPending) return;
    setError('');
    removeVault.mutate(
      { path, password },
      {
        onSuccess: (jobId) => {
          startJob(paneId, {
            id: jobId,
            kind: 'vault',
            label: 'Removing password… 0%',
            cancelable: true,
            backendJobId: jobId,
            path,
          });
          reset();
          close();
        },
        onError: (err) => setError(errMessage(err)),
      },
    );
  };

  return (
    <Dialog data-testid="dialog-vault-remove" open={open} onClose={onClose} fullWidth maxWidth="xs">
      <form onSubmit={submit}>
        <DialogTitle>Remove password</DialogTitle>
        <DialogContent>
          <DialogContentText sx={{ mb: 1 }}>
            <strong>{name}</strong> will be decrypted back to plaintext and the vault metadata
            deleted. This cannot be undone.
          </DialogContentText>
          <TextField
            autoFocus
            fullWidth
            margin="dense"
            type="password"
            label="Password"
            data-testid="input-vault-remove-password"
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
            color="error"
            variant="contained"
            data-testid="btn-vault-remove-confirm"
            disabled={!password || removeVault.isPending}
          >
            Remove password
          </Button>
        </DialogActions>
      </form>
    </Dialog>
  );
};
