import Button from '@mui/material/Button';
import Dialog from '@mui/material/Dialog';
import DialogActions from '@mui/material/DialogActions';
import DialogContent from '@mui/material/DialogContent';
import DialogContentText from '@mui/material/DialogContentText';
import DialogTitle from '@mui/material/DialogTitle';
import TextField from '@mui/material/TextField';
import Typography from '@mui/material/Typography';
import { type FC, type FormEvent, useState } from 'react';
import { useStartCreateFileVault, useStartCreateVault } from '../../entities/file/queries';
import { usePaneJobStore } from '../../features/jobs/paneJobStore';
import { useVaultDialogStore } from '../../features/vault/vaultDialogStore';
import { errMessage } from '../../shared/lib/format';

const MIN_PASSWORD_LEN = 8;

export const LockDialog: FC = () => {
  const open = useVaultDialogStore((s) => s.mode === 'lock');
  const path = useVaultDialogStore((s) => s.path);
  const paneId = useVaultDialogStore((s) => s.paneId);
  const isFile = useVaultDialogStore((s) => s.isFile);
  const close = useVaultDialogStore((s) => s.close);
  const startJob = usePaneJobStore((s) => s.start);

  const createFolderVault = useStartCreateVault();
  const createFileVault = useStartCreateFileVault();
  const pending = createFolderVault.isPending || createFileVault.isPending;

  const [password, setPassword] = useState('');
  const [confirm, setConfirm] = useState('');
  const [hint, setHint] = useState('');
  const [error, setError] = useState('');

  const name = path.split(/[/\\]/).pop() || path;
  const mismatch = confirm.length > 0 && password !== confirm;
  const tooShort = password.length > 0 && password.length < MIN_PASSWORD_LEN;
  const canSubmit = password.length >= MIN_PASSWORD_LEN && password === confirm;

  const reset = () => {
    setPassword('');
    setConfirm('');
    setHint('');
    setError('');
  };

  const onClose = () => {
    reset();
    close();
  };

  const submit = (e?: FormEvent) => {
    e?.preventDefault();
    if (!canSubmit || pending) return;
    setError('');
    const mutation = isFile ? createFileVault : createFolderVault;
    mutation.mutate(
      { path, password, hint },
      {
        onSuccess: (jobId) => {
          startJob(paneId, {
            id: jobId,
            kind: 'vault',
            label: `Locking… 0%`,
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
    <Dialog data-testid="dialog-vault-lock" open={open} onClose={onClose} fullWidth maxWidth="xs">
      <form onSubmit={submit}>
        <DialogTitle>Lock with password</DialogTitle>
        <DialogContent>
          <DialogContentText sx={{ mb: 1.5 }}>
            {isFile ? 'File' : 'Folder'}: <strong>{name}</strong>
          </DialogContentText>
          <TextField
            fullWidth
            margin="dense"
            type="password"
            label="Password"
            data-testid="input-vault-lock-password"
            value={password}
            error={tooShort}
            helperText={tooShort ? `At least ${MIN_PASSWORD_LEN} characters` : ' '}
            onChange={(e) => setPassword(e.target.value)}
            autoFocus
          />
          <TextField
            fullWidth
            margin="dense"
            type="password"
            label="Confirm password"
            data-testid="input-vault-lock-confirm"
            value={confirm}
            error={mismatch}
            helperText={mismatch ? 'Passwords do not match' : ' '}
            onChange={(e) => setConfirm(e.target.value)}
          />
          <TextField
            fullWidth
            margin="dense"
            label="Hint (optional)"
            data-testid="input-vault-lock-hint"
            value={hint}
            helperText="Stored next to the vault, not secret."
            onChange={(e) => setHint(e.target.value)}
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
            data-testid="btn-vault-lock-confirm"
            disabled={!canSubmit || pending}
          >
            Lock
          </Button>
        </DialogActions>
      </form>
    </Dialog>
  );
};
