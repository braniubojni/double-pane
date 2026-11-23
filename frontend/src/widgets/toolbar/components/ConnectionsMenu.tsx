import type { FC } from 'react';
import { useQuery } from '@tanstack/react-query';
import AddIcon from '@mui/icons-material/Add';
import CloudIcon from '@mui/icons-material/Cloud';
import CloudQueueIcon from '@mui/icons-material/CloudQueue';
import DeleteIcon from '@mui/icons-material/Delete';
import FolderOpenIcon from '@mui/icons-material/FolderOpen';
import LinkOffIcon from '@mui/icons-material/LinkOff';
import Box from '@mui/material/Box';
import Divider from '@mui/material/Divider';
import IconButton from '@mui/material/IconButton';
import ListSubheader from '@mui/material/ListSubheader';
import Menu from '@mui/material/Menu';
import MenuItem from '@mui/material/MenuItem';
import Tooltip from '@mui/material/Tooltip';
import Typography from '@mui/material/Typography';
import { sessionKeyFromProfile } from '../../../features/connections/helpers';
import type { ConnectionProfile } from '../../../features/connections/types';
import { usePaneStore } from '../../../features/pane/paneStore';
import { FileService } from '../../../shared/api/bindings';
import { enterPaneTab } from '../../file-pane/helpers';
import { useConnections } from '../hooks/useConnections';
import { ConnectionDialog } from './ConnectionDialog';

export const ConnectionsMenu: FC = () => {
  const {
    anchor,
    setAnchor,
    dialog,
    dispatch,
    sshProfiles,
    smbProfiles,
    ftpProfiles,
    megaProfiles,
    sessionKeys,
    onMenuConnect,
    onDisconnect,
    onRemove,
    onForgetRecent,
    submitDialog,
    openSSHConfigMode,
    loadSSHConfig,
    connectFromConfig,
  } = useConnections();
  const navigate = usePaneStore((s) => s.navigate);
  const activePane = usePaneStore((s) => s.activePane);
  const { data: iCloudPath = '' } = useQuery({
    queryKey: ['icloudDrive'],
    queryFn: async () => ((await FileService.ICloudDrivePath()) ?? '') as string,
    staleTime: 60_000,
  });
  const { data: drivePaths = [] } = useQuery({
    queryKey: ['googleDrive'],
    queryFn: async () => (await FileService.GoogleDrivePaths()) ?? [],
    staleTime: 60_000,
  });

  const openPath = (path: string) => {
    if (!path) return;
    enterPaneTab(activePane, path);
    navigate(activePane, path);
    setAnchor(null);
  };

  const driveLabel = (path: string) => {
    const account = path.split('GoogleDrive-')[1];
    return account ? `Google Drive (${account})` : 'Google Drive';
  };

  const renderProfile = (p: ConnectionProfile) => {
    const key = sessionKeyFromProfile(p);
    const live = sessionKeys.has(key);
    return (
      <MenuItem
        key={p.id}
        data-testid={`conn-profile-${p.id}`}
        dense
        onClick={() => onMenuConnect(p)}
        sx={{ display: 'flex', gap: 1, pr: 0.5 }}
      >
        <Box sx={{ flex: 1, minWidth: 0 }}>
          <Typography variant="body2" noWrap sx={{ fontWeight: live ? 700 : 400 }}>
            {p.label || key}
          </Typography>
          {live && (
            <Typography variant="caption" color="success.main">
              connected
            </Typography>
          )}
        </Box>
        {live && (
          <IconButton
            size="small"
            aria-label="Disconnect"
            data-testid={`btn-disconnect-${p.id}`}
            onClick={(e) => {
              e.stopPropagation();
              void onDisconnect(key);
            }}
          >
            <LinkOffIcon fontSize="small" />
          </IconButton>
        )}
        <IconButton
          size="small"
          aria-label="Remove"
          data-testid={`btn-remove-conn-${p.id}`}
          onClick={(e) => {
            e.stopPropagation();
            void onRemove(p.id);
          }}
        >
          <DeleteIcon fontSize="small" />
        </IconButton>
      </MenuItem>
    );
  };

  return (
    <>
      <Tooltip title="Remote connections (SSH/SFTP, SMB)">
        <IconButton
          data-testid="btn-connections"
          onClick={(e) => setAnchor(e.currentTarget)}
          color={sessionKeys.size ? 'primary' : 'default'}
        >
          <CloudIcon />
        </IconButton>
      </Tooltip>

      <Menu
        data-testid="menu-connections"
        anchorEl={anchor}
        open={Boolean(anchor)}
        onClose={() => setAnchor(null)}
        slotProps={{ paper: { sx: { minWidth: 260 } } }}
      >
        {iCloudPath || drivePaths.length ? (
          <>
            <ListSubheader sx={{ lineHeight: '32px', bgcolor: 'background.paper' }}>
              Cloud
            </ListSubheader>
            {iCloudPath ? (
              <MenuItem data-testid="menu-conn-icloud" dense onClick={() => openPath(iCloudPath)}>
                <CloudQueueIcon fontSize="small" sx={{ mr: 1 }} />
                iCloud Drive
              </MenuItem>
            ) : null}
            {drivePaths.map((path, i) => (
              <MenuItem
                key={path}
                data-testid={`menu-conn-gdrive-${i}`}
                dense
                onClick={() => openPath(path)}
              >
                <CloudQueueIcon fontSize="small" sx={{ mr: 1 }} />
                {driveLabel(path)}
              </MenuItem>
            ))}
            <Divider />
          </>
        ) : null}
        <ListSubheader sx={{ lineHeight: '32px', bgcolor: 'background.paper' }}>
          SSH/SFTP
        </ListSubheader>
        {sshProfiles.length === 0 && (
          <MenuItem disabled dense>
            <Typography variant="body2" color="text.secondary">
              No saved SSH connections
            </Typography>
          </MenuItem>
        )}
        {sshProfiles.map(renderProfile)}
        <MenuItem
          data-testid="menu-conn-add"
          dense
          onClick={() => {
            setAnchor(null);
            dispatch({ type: 'open_add' });
          }}
        >
          <AddIcon fontSize="small" sx={{ mr: 1 }} />
          Add new…
        </MenuItem>
        <MenuItem
          data-testid="menu-conn-ssh-config"
          dense
          onClick={() => {
            setAnchor(null);
            void openSSHConfigMode();
          }}
        >
          <FolderOpenIcon fontSize="small" sx={{ mr: 1 }} />
          From SSH config…
        </MenuItem>

        <Divider />
        <ListSubheader sx={{ lineHeight: '32px', bgcolor: 'background.paper' }}>SMB</ListSubheader>
        {smbProfiles.length === 0 && (
          <MenuItem disabled dense>
            <Typography variant="body2" color="text.secondary">
              No saved SMB connections
            </Typography>
          </MenuItem>
        )}
        {smbProfiles.map(renderProfile)}
        <MenuItem
          data-testid="menu-conn-add-smb"
          dense
          onClick={() => {
            setAnchor(null);
            dispatch({ type: 'open_add_smb' });
          }}
        >
          <AddIcon fontSize="small" sx={{ mr: 1 }} />
          Add SMB…
        </MenuItem>

        <Divider />
        <ListSubheader sx={{ lineHeight: '32px', bgcolor: 'background.paper' }}>MEGA</ListSubheader>
        {megaProfiles.length === 0 && (
          <MenuItem disabled dense>
            <Typography variant="body2" color="text.secondary">
              No saved MEGA connections
            </Typography>
          </MenuItem>
        )}
        {megaProfiles.map(renderProfile)}
        <MenuItem
          data-testid="menu-conn-add-mega"
          dense
          onClick={() => {
            setAnchor(null);
            dispatch({ type: 'open_add_mega' });
          }}
        >
          <AddIcon fontSize="small" sx={{ mr: 1 }} />
          Add MEGA…
        </MenuItem>

        <Divider />
        <ListSubheader sx={{ lineHeight: '32px', bgcolor: 'background.paper' }}>FTP</ListSubheader>
        {ftpProfiles.length === 0 && (
          <MenuItem disabled dense>
            <Typography variant="body2" color="text.secondary">
              No saved FTP connections
            </Typography>
          </MenuItem>
        )}
        {ftpProfiles.map(renderProfile)}
        <MenuItem
          data-testid="menu-conn-ftp"
          dense
          onClick={() => {
            setAnchor(null);
            dispatch({ type: 'open_add_ftp' });
          }}
        >
          <AddIcon fontSize="small" sx={{ mr: 1 }} />
          FTP…
        </MenuItem>
      </Menu>

      <ConnectionDialog
        dialog={dialog}
        dispatch={dispatch}
        onSubmit={() => void submitDialog()}
        onLoadSSHConfig={(path) => void loadSSHConfig(path)}
        onConnectFromConfig={(host) => void connectFromConfig(host)}
        onForgetRecent={onForgetRecent}
      />
    </>
  );
};
