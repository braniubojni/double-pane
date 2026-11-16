import type { Dispatch, FC } from 'react';
import Checkbox from '@mui/material/Checkbox';
import FormControlLabel from '@mui/material/FormControlLabel';
import TextField from '@mui/material/TextField';
import { parseFTPForm, type FTPFieldErrors } from '../../../features/connections/ftpValidate';
import type { AddConnectionAction, AddConnectionState } from '../../../features/connections/types';
import { handleDialogEnter } from '../../../shared/lib/dialogSubmit';

type Props = {
  dialog: AddConnectionState;
  dispatch: Dispatch<AddConnectionAction>;
  onSubmit: () => void;
  errors: FTPFieldErrors;
};

export const ftpFormFromDialog = (dialog: AddConnectionState) =>
  parseFTPForm({
    host: dialog.smbHost,
    user: dialog.smbUser,
    port: dialog.smbPort,
  });

export const FTPConnectFields: FC<Props> = ({ dialog, dispatch, onSubmit, errors }) => {
  const show = (key: keyof FTPFieldErrors, value: string) => Boolean(errors[key] && value.trim());

  return (
    <>
      <TextField
        autoFocus
        label="Server"
        placeholder="ftp.example.com or 192.168.1.10"
        value={dialog.smbHost}
        onChange={(e) => dispatch({ type: 'set_smb_host', host: e.target.value })}
        onKeyDown={(e) => handleDialogEnter(e, onSubmit)}
        data-testid="input-ftp-host"
        fullWidth
        disabled={dialog.busy}
        error={show('host', dialog.smbHost)}
        helperText={show('host', dialog.smbHost) ? errors.host : 'Host name or IP'}
        sx={{ mt: 1 }}
        spellCheck={false}
      />
      <TextField
        label="Username"
        placeholder="anonymous"
        value={dialog.smbUser}
        onChange={(e) => dispatch({ type: 'set_smb_user', user: e.target.value })}
        onKeyDown={(e) => handleDialogEnter(e, onSubmit)}
        data-testid="input-ftp-user"
        fullWidth
        disabled={dialog.busy}
        error={show('user', dialog.smbUser)}
        helperText={
          show('user', dialog.smbUser)
            ? errors.user
            : 'Use anonymous and leave the password empty for anonymous FTP'
        }
        spellCheck={false}
      />
      <TextField
        label="Password"
        type="password"
        value={dialog.password}
        onChange={(e) => dispatch({ type: 'set_password', password: e.target.value })}
        onKeyDown={(e) => handleDialogEnter(e, onSubmit)}
        data-testid="input-ftp-password"
        fullWidth
        disabled={dialog.busy}
        helperText="Blank is fine for anonymous. Some servers ask for an email here."
      />
      <TextField
        label="Port"
        value={dialog.smbPort}
        onChange={(e) => dispatch({ type: 'set_smb_port', port: e.target.value })}
        onKeyDown={(e) => handleDialogEnter(e, onSubmit)}
        data-testid="input-ftp-port"
        fullWidth
        disabled={dialog.busy}
        error={Boolean(errors.port)}
        helperText={errors.port ?? 'Default 21'}
        spellCheck={false}
      />
      <FormControlLabel
        control={
          <Checkbox
            checked={dialog.save}
            onChange={(e) => dispatch({ type: 'set_save', save: e.target.checked })}
            disabled={dialog.busy}
          />
        }
        label="Save connection"
      />
    </>
  );
};
