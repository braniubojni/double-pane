export type SMBShare = {
  name: string;
  hidden: boolean;
  localPath?: string;
};

export type AddConnectionState = {
  open: boolean;
  spec: string;
  password: string;
  askPassword: boolean;
  save: boolean;
  busy: boolean;
  error: string;
  /** When re-prompting password for an existing profile connect */
  profileId: string;
  totp: string;
  megaAuth: boolean;
  mode:
    | 'add'
    | 'add_smb'
    | 'add_ftp'
    | 'add_mega'
    | 'smb_confirm'
    | 'password'
    | 'ssh_config'
    | 'workdir'
    | 'smb_shares';
  // ssh_config mode
  sshConfigPath: string;
  sshConfigHosts: SSHConfigHost[];
  sshConfigLoading: boolean;
  selectedConfigHost: SSHConfigHost | null;
  // workdir mode
  workdirPaths: RemoteRecent[];
  workdirHome: string;
  workdirSessionKey: string;
  workdirChosen: string;
  workdirCustom: string;
  workdirRemember: boolean;
  workdirProfileId: string;
  // SMB add + share picker
  smbHost: string;
  smbUser: string;
  smbDomain: string;
  smbPort: string;
  shares: SMBShare[];
  shareChosen: string;
  showHiddenShares: boolean;
  smbRootPath: string;
};

export type AddConnectionAction =
  | { type: 'open_add' }
  | { type: 'open_add_smb' }
  | { type: 'open_add_ftp' }
  | { type: 'open_add_mega' }
  | { type: 'open_smb_confirm' }
  | { type: 'back_smb_form' }
  | { type: 'open_password'; profileId: string; label?: string; mega?: boolean }
  | { type: 'set_totp'; totp: string }
  | { type: 'open_ssh_config' }
  | {
      type: 'open_workdir';
      paths: RemoteRecent[];
      home: string;
      sessionKey: string;
      chosen?: string;
      profileId?: string;
    }
  | {
      type: 'open_smb_shares';
      shares: SMBShare[];
      rootPath: string;
      sessionKey: string;
      chosen?: string;
      profileId?: string;
    }
  | { type: 'close' }
  | { type: 'set_spec'; spec: string }
  | { type: 'set_password'; password: string }
  | { type: 'set_save'; save: boolean }
  | { type: 'set_busy'; busy: boolean }
  | { type: 'set_error'; error: string }
  | { type: 'need_password' }
  | { type: 'set_ssh_config_path'; path: string }
  | { type: 'set_ssh_config_hosts'; hosts: SSHConfigHost[] }
  | { type: 'set_ssh_config_loading'; loading: boolean }
  | { type: 'select_config_host'; host: SSHConfigHost }
  | { type: 'set_workdir_chosen'; path: string }
  | { type: 'set_workdir_custom'; path: string }
  | { type: 'set_workdir_remember'; remember: boolean }
  | { type: 'remove_workdir_path'; path: string }
  | { type: 'set_smb_host'; host: string }
  | { type: 'set_smb_user'; user: string }
  | { type: 'set_smb_domain'; domain: string }
  | { type: 'set_smb_port'; port: string }
  | { type: 'set_share_chosen'; name: string }
  | { type: 'set_show_hidden_shares'; show: boolean };

/** Saved remote connection profile (from backend). */
export type ConnectionProfile = {
  id: string;
  protocol: string;
  user: string;
  host: string;
  port: number;
  label: string;
  configAlias?: string;
  identityFiles?: string[];
  defaultWorkDir?: string;
  domain?: string;
};

/** Live remote session (from backend). */
export type ActiveSession = {
  key: string;
  protocol: string;
  user: string;
  host: string;
  port: number;
  rootPath: string;
};

/** SSH config file host entry (mirrors domain.SSHConfigHost). */
export type SSHConfigHost = {
  alias: string;
  hostName: string;
  user: string;
  port: number;
  identityFiles: string[];
  /** Absolute path of the config file this entry was loaded from (for ssh -F). */
  configPath?: string;
};

/** Recently visited remote directory (mirrors domain.RemoteRecent). */
export type RemoteRecent = {
  sessionKey: string;
  path: string;
  label: string;
  lastVisited: string;
};
