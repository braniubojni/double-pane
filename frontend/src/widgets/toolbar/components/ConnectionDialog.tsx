import type { FC } from 'react';
import CloseIcon from '@mui/icons-material/Close';
import Autocomplete from '@mui/material/Autocomplete';
import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import Checkbox from '@mui/material/Checkbox';
import Dialog from '@mui/material/Dialog';
import DialogActions from '@mui/material/DialogActions';
import DialogContent from '@mui/material/DialogContent';
import DialogTitle from '@mui/material/DialogTitle';
import FormControlLabel from '@mui/material/FormControlLabel';
import IconButton from '@mui/material/IconButton';
import List from '@mui/material/List';
import ListItemButton from '@mui/material/ListItemButton';
import ListItemText from '@mui/material/ListItemText';
import Radio from '@mui/material/Radio';
import RadioGroup from '@mui/material/RadioGroup';
import TextField from '@mui/material/TextField';
import Tooltip from '@mui/material/Tooltip';
import Typography from '@mui/material/Typography';
import { usePathCompletions } from '../../../entities/file/queries';
import {
  isLocalNetworkErrorMessage,
  resolveRemoteWorkdirInput,
} from '../../../features/connections/helpers';
import { smbNetworkSettingsLabel } from '../../../features/connections/smbCopy';
import { smbShareError } from '../../../features/connections/smbValidate';
import { FileService } from '../../../shared/api/bindings';
import { errMessage } from '../../../shared/lib/format';
import { useSnack } from '../../../shared/ui/SnackbarHost';
import { handleDialogEnter, handleDialogFormSubmit } from '../../../shared/lib/dialogSubmit';
import type { ConnectionDialogProps } from '../types';
import { FTPConnectFields, ftpFormFromDialog } from './FTPConnectFields';
import { SMBConnectFields, smbFormFromDialog } from './SMBConnectFields';
import { SMBSharePicker } from './SMBSharePicker';
import { MEGAConnectFields, megaEmailOk } from './MEGAConnectFields';

const DIALOG_TITLES: Record<string, string> = {
  add: 'Add SSH connection',
  add_smb: 'Add SMB connection',
  add_ftp: 'Add FTP connection',
  add_mega: 'Add MEGA connection',
  password: 'Password',
  ssh_config: 'Connect from SSH config',
  workdir: 'Choose working directory',
  smb_shares: 'Choose a share',
  smb_confirm: 'Connect to Server',
};

export const ConnectionDialog: FC<ConnectionDialogProps> = ({
  dialog,
  dispatch,
  onSubmit,
  onLoadSSHConfig,
  onConnectFromConfig,
  onForgetRecent,
}) => {
  const workdirCustomOpen = dialog.mode === 'workdir' && Boolean(dialog.workdirCustom);
  const workdirPartial = resolveRemoteWorkdirInput(dialog.workdirHome, dialog.workdirCustom) ?? '';
  const workdirCompletions = usePathCompletions(workdirPartial, workdirCustomOpen);
  const workdirOptions = workdirCompletions.data ?? [];
  const show = useSnack((s) => s.show);
  const localNetError = isLocalNetworkErrorMessage(dialog.error);
  const smbForm = smbFormFromDialog(dialog);
  const ftpForm = ftpFormFromDialog(dialog);
  const shareInvalid = Boolean(smbShareError(dialog.shareChosen));
  const connectDisabled =
    dialog.busy ||
    (dialog.mode === 'add_smb' && !smbForm.ok) ||
    (dialog.mode === 'add_ftp' && !ftpForm.ok) ||
    (dialog.mode === 'add_mega' && !megaEmailOk(dialog.spec)) ||
    (dialog.mode === 'smb_shares' && shareInvalid);

  const submit = () => {
    if (dialog.busy) return;
    if (dialog.mode === 'add_smb' && !smbForm.ok) return;
    if (dialog.mode === 'add_ftp' && !ftpForm.ok) return;
    if (dialog.mode === 'smb_shares' && shareInvalid) return;
    // Password prompt (including ssh_config after an auth failure) must connect,
    // not reload the host list.
    if (dialog.askPassword || dialog.mode === 'password') {
      onSubmit();
      return;
    }
    if (dialog.mode === 'ssh_config') {
      onLoadSSHConfig(dialog.sshConfigPath);
      return;
    }
    onSubmit();
  };

  return (
    <Dialog
      data-testid="dialog-connection"
      open={dialog.open}
      onClose={() => !dialog.busy && dispatch({ type: 'close' })}
      fullWidth
      disableRestoreFocus
      maxWidth="xs"
    >
      <form onSubmit={(e) => handleDialogFormSubmit(e, submit)}>
        <DialogTitle>{DIALOG_TITLES[dialog.mode] ?? 'Remote'}</DialogTitle>
        <DialogContent sx={{ display: 'flex', flexDirection: 'column', gap: 1.5, pt: 1 }}>
          {dialog.mode === 'add_smb' && (
            <SMBConnectFields
              dialog={dialog}
              dispatch={dispatch}
              onSubmit={submit}
              errors={smbForm.errors}
            />
          )}
          {dialog.mode === 'add_ftp' && (
            <FTPConnectFields
              dialog={dialog}
              dispatch={dispatch}
              onSubmit={submit}
              errors={ftpForm.errors}
            />
          )}
          {dialog.mode === 'add_mega' && (
            <MEGAConnectFields dialog={dialog} dispatch={dispatch} onSubmit={submit} />
          )}
          {dialog.mode === 'smb_shares' && <SMBSharePicker dialog={dialog} dispatch={dispatch} />}
          {dialog.mode === 'smb_confirm' && (
            <Typography variant="body2" sx={{ mt: 1 }} data-testid="smb-confirm-copy">
              You are attempting to connect to the server “{smbForm.host || dialog.smbHost}”. Click
              Connect to continue. On a Mac this may also open Finder’s volume picker if guest
              access is not enough.
            </Typography>
          )}
          {/* ── mode: add ─────────────────────────────────────────────────── */}
          {dialog.mode === 'add' && (
            <>
              <TextField
                autoFocus
                label="Connection"
                placeholder="ssh user@host or config alias"
                helperText="Examples: ssh user@192.168.1.10 · user@host:2222 · pahestain (from ~/.ssh/config)"
                value={dialog.spec}
                onChange={(e) => dispatch({ type: 'set_spec', spec: e.target.value })}
                onKeyDown={(e) => handleDialogEnter(e, submit)}
                data-testid="input-conn-spec"
                fullWidth
                disabled={dialog.busy}
                sx={{ mt: 1 }}
                spellCheck={false}
                autoCorrect="off"
                autoCapitalize="off"
                autoComplete="off"
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
          )}

          {/* ── mode: ssh_config ──────────────────────────────────────────── */}
          {dialog.mode === 'ssh_config' && (
            <>
              <Box sx={{ display: 'flex', gap: 1, alignItems: 'flex-start', mt: 1 }}>
                <TextField
                  label="Config file"
                  value={dialog.sshConfigPath}
                  onChange={(e) => dispatch({ type: 'set_ssh_config_path', path: e.target.value })}
                  onKeyDown={(e) => handleDialogEnter(e, submit)}
                  size="small"
                  fullWidth
                  disabled={dialog.busy || dialog.sshConfigLoading}
                  data-testid="input-ssh-config-path"
                />
                <Button
                  variant="outlined"
                  size="small"
                  onClick={() => onLoadSSHConfig(dialog.sshConfigPath)}
                  disabled={dialog.busy || dialog.sshConfigLoading}
                  sx={{ mt: 0.25, whiteSpace: 'nowrap' }}
                  data-testid="btn-load-ssh-config"
                >
                  {dialog.sshConfigLoading ? 'Loading…' : 'Load'}
                </Button>
              </Box>
              <FormControlLabel
                control={
                  <Checkbox
                    checked={dialog.save}
                    onChange={(e) => dispatch({ type: 'set_save', save: e.target.checked })}
                    disabled={dialog.busy}
                  />
                }
                label="Save as profile"
              />
              {dialog.sshConfigHosts.length === 0 && !dialog.sshConfigLoading && (
                <Typography variant="body2" color="text.secondary">
                  {dialog.sshConfigPath
                    ? 'Click Load to read hosts from the config file.'
                    : 'Enter a config file path above.'}
                </Typography>
              )}
              {dialog.sshConfigHosts.length > 0 && (
                <List
                  dense
                  disablePadding
                  sx={{ border: 1, borderColor: 'divider', borderRadius: 1 }}
                >
                  {dialog.sshConfigHosts.map((h) => (
                    <ListItemButton
                      key={h.alias}
                      selected={dialog.selectedConfigHost?.alias === h.alias}
                      onClick={() => onConnectFromConfig(h)}
                      disabled={dialog.busy}
                      data-testid={`ssh-config-host-${h.alias}`}
                    >
                      <ListItemText
                        primary={h.alias}
                        secondary={`${h.user}@${h.hostName}:${h.port}`}
                      />
                    </ListItemButton>
                  ))}
                </List>
              )}
            </>
          )}

          {/* ── mode: workdir ─────────────────────────────────────────────── */}
          {dialog.mode === 'workdir' && (
            <>
              <Typography variant="body2" color="text.secondary">
                Connected to {dialog.workdirSessionKey}. Choose a folder to open (not the whole
                tree).
              </Typography>
              <RadioGroup
                value={dialog.workdirChosen}
                onChange={(e) => dispatch({ type: 'set_workdir_chosen', path: e.target.value })}
              >
                <FormControlLabel
                  value={dialog.workdirHome}
                  control={<Radio size="small" />}
                  label={
                    <Box>
                      <Typography variant="body2">Home</Typography>
                      <Typography
                        variant="caption"
                        color="text.secondary"
                        sx={{ wordBreak: 'break-all' }}
                      >
                        {dialog.workdirHome}
                      </Typography>
                    </Box>
                  }
                />
                {dialog.workdirPaths
                  .filter((r) => r.path !== dialog.workdirHome)
                  .map((r) => (
                    <Box key={r.path} sx={{ display: 'flex', alignItems: 'center' }}>
                      <FormControlLabel
                        sx={{ flex: 1, minWidth: 0 }}
                        value={r.path}
                        control={<Radio size="small" />}
                        label={
                          <Box>
                            <Typography variant="body2" sx={{ wordBreak: 'break-all' }}>
                              {r.label}
                            </Typography>
                            <Typography variant="caption" color="text.secondary">
                              Recent · {r.lastVisited}
                            </Typography>
                          </Box>
                        }
                      />
                      <Tooltip title="Forget this path">
                        <IconButton
                          size="small"
                          aria-label={`forget-recent-${r.path}`}
                          onClick={(e) => {
                            e.stopPropagation();
                            onForgetRecent(r.path);
                          }}
                        >
                          <CloseIcon fontSize="small" />
                        </IconButton>
                      </Tooltip>
                    </Box>
                  ))}
                <FormControlLabel
                  value="__custom__"
                  control={<Radio size="small" />}
                  label={<Typography variant="body2">Other path…</Typography>}
                />
              </RadioGroup>
              {(dialog.workdirChosen === '__custom__' || dialog.workdirCustom) && (
                <Autocomplete
                  freeSolo
                  fullWidth
                  size="small"
                  options={workdirOptions}
                  inputValue={dialog.workdirCustom}
                  filterOptions={(x) => x}
                  onInputChange={(_, v, reason) => {
                    if (reason === 'reset') return;
                    dispatch({ type: 'set_workdir_custom', path: v });
                  }}
                  onChange={(_, v) => {
                    if (typeof v === 'string' && v)
                      dispatch({ type: 'set_workdir_custom', path: v });
                  }}
                  renderInput={(params) => (
                    <TextField
                      {...params}
                      autoFocus
                      label="Remote path"
                      placeholder="/home/user/project"
                      helperText="Absolute path on the remote, or ssh:// / smb:// / ftp:// / mega://…"
                      onKeyDown={(e) => handleDialogEnter(e, submit)}
                      data-testid="input-workdir-custom"
                    />
                  )}
                />
              )}
              {dialog.workdirProfileId && (
                <FormControlLabel
                  control={
                    <Checkbox
                      checked={dialog.workdirRemember}
                      onChange={(e) =>
                        dispatch({ type: 'set_workdir_remember', remember: e.target.checked })
                      }
                    />
                  }
                  label="Remember as default for this connection"
                />
              )}
            </>
          )}

          {/* ── password / key passphrase prompt ─────────────────────────── */}
          {(dialog.askPassword || dialog.mode === 'password') &&
            dialog.mode !== 'workdir' &&
            dialog.mode !== 'smb_shares' &&
            dialog.mode !== 'add_smb' &&
            dialog.mode !== 'add_ftp' &&
            dialog.mode !== 'add_mega' && (
              <>
                <TextField
                  autoFocus={dialog.mode === 'password' || dialog.askPassword}
                  type="password"
                  label="Password or key passphrase"
                  value={dialog.password}
                  onChange={(e) => dispatch({ type: 'set_password', password: e.target.value })}
                  onKeyDown={(e) => handleDialogEnter(e, submit)}
                  data-testid="input-conn-password"
                  fullWidth
                  disabled={dialog.busy}
                  helperText={
                    dialog.megaAuth
                      ? `MEGA account ${dialog.spec || ''}`.trim()
                      : dialog.mode === 'password'
                        ? `Auth for ${dialog.spec || 'SSH'} (server password or encrypted key passphrase)`
                        : 'Public key auth failed — try passphrase, password, or set IdentityFile in ~/.ssh/config'
                  }
                  sx={{ mt: 1 }}
                />
                {dialog.megaAuth && (
                  <TextField
                    label="2FA code"
                    placeholder="optional"
                    value={dialog.totp}
                    onChange={(e) => dispatch({ type: 'set_totp', totp: e.target.value })}
                    onKeyDown={(e) => handleDialogEnter(e, submit)}
                    data-testid="input-mega-totp-prompt"
                    fullWidth
                    disabled={dialog.busy}
                    autoComplete="one-time-code"
                  />
                )}
              </>
            )}

          {dialog.error && (
            <>
              <Typography color="error" variant="body2" data-testid="conn-error">
                {dialog.error}
              </Typography>
              {localNetError && (
                <Button
                  size="small"
                  variant="outlined"
                  data-testid="btn-open-local-network"
                  onClick={() => {
                    void FileService.OpenLocalNetworkSettings().catch((e) =>
                      show(errMessage(e), 'error'),
                    );
                  }}
                >
                  {smbNetworkSettingsLabel()}
                </Button>
              )}
            </>
          )}
        </DialogContent>

        <DialogActions>
          <Button type="button" disabled={dialog.busy} onClick={() => dispatch({ type: 'close' })}>
            Cancel
          </Button>
          {(dialog.mode !== 'ssh_config' || dialog.askPassword) && (
            <Button
              data-testid="btn-conn-connect"
              type="submit"
              variant="contained"
              disabled={connectDisabled}
            >
              {dialog.mode === 'workdir' || dialog.mode === 'smb_shares'
                ? 'Open'
                : dialog.busy
                  ? 'Connecting…'
                  : 'Connect'}
            </Button>
          )}
        </DialogActions>
      </form>
    </Dialog>
  );
};
