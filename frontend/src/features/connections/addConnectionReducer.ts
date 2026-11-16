import type { AddConnectionAction, AddConnectionState } from './types';

export type { AddConnectionAction, AddConnectionState } from './types';

export const initialAddConnectionState: AddConnectionState = {
  open: false,
  spec: '',
  password: '',
  askPassword: false,
  save: true,
  busy: false,
  error: '',
  profileId: '',
  totp: '',
  megaAuth: false,
  mode: 'add',
  sshConfigPath: '',
  sshConfigHosts: [],
  sshConfigLoading: false,
  selectedConfigHost: null,
  workdirPaths: [],
  workdirHome: '',
  workdirSessionKey: '',
  workdirChosen: '',
  workdirCustom: '',
  workdirRemember: true,
  workdirProfileId: '',
  smbHost: '',
  smbUser: '',
  smbDomain: '',
  smbPort: '445',
  shares: [],
  shareChosen: '',
  showHiddenShares: false,
  smbRootPath: '',
};

export const addConnectionReducer = (
  state: AddConnectionState,
  action: AddConnectionAction,
): AddConnectionState => {
  switch (action.type) {
    case 'open_add':
      return { ...initialAddConnectionState, open: true, mode: 'add', save: true };
    case 'open_add_smb':
      return { ...initialAddConnectionState, open: true, mode: 'add_smb', save: true };
    case 'open_add_ftp':
      return {
        ...initialAddConnectionState,
        open: true,
        mode: 'add_ftp',
        save: true,
        smbPort: '21',
      };
    case 'open_add_mega':
      return {
        ...initialAddConnectionState,
        open: true,
        mode: 'add_mega',
        save: true,
        megaAuth: true,
      };
    case 'open_smb_confirm':
      return { ...state, mode: 'smb_confirm', error: '', busy: false };
    case 'back_smb_form':
      return { ...state, mode: 'add_smb', busy: false };
    case 'open_password':
      return {
        ...initialAddConnectionState,
        open: true,
        mode: 'password',
        askPassword: true,
        profileId: action.profileId,
        spec: action.label ?? '',
        megaAuth: Boolean(action.mega),
      };
    case 'open_ssh_config':
      return {
        ...initialAddConnectionState,
        open: true,
        mode: 'ssh_config',
        save: true,
        sshConfigPath: '',
      };
    case 'open_workdir': {
      const chosen = action.chosen || action.home;
      return {
        ...state,
        open: true,
        mode: 'workdir',
        busy: false,
        error: '',
        workdirPaths: action.paths,
        workdirHome: action.home,
        workdirSessionKey: action.sessionKey,
        workdirChosen: chosen,
        workdirCustom: '',
        workdirRemember: Boolean(action.profileId),
        workdirProfileId: action.profileId ?? '',
      };
    }
    case 'open_smb_shares': {
      const visible = action.shares.filter((s) => !s.hidden);
      const chosen = action.chosen || visible[0]?.name || action.shares[0]?.name || '';
      return {
        ...state,
        open: true,
        mode: 'smb_shares',
        busy: false,
        error: '',
        shares: action.shares,
        shareChosen: chosen,
        showHiddenShares: false,
        smbRootPath: action.rootPath,
        workdirSessionKey: action.sessionKey,
        workdirRemember: Boolean(action.profileId),
        workdirProfileId: action.profileId ?? '',
      };
    }
    case 'close':
      return { ...initialAddConnectionState };
    case 'set_spec':
      return { ...state, spec: action.spec, error: '' };
    case 'set_password':
      return { ...state, password: action.password, error: '' };
    case 'set_totp':
      return { ...state, totp: action.totp, error: '' };
    case 'set_save':
      return { ...state, save: action.save };
    case 'set_busy':
      return { ...state, busy: action.busy };
    case 'set_error':
      return { ...state, error: action.error, busy: false };
    case 'need_password':
      return { ...state, askPassword: true, busy: false, password: '' };
    case 'set_ssh_config_path':
      return { ...state, sshConfigPath: action.path, sshConfigHosts: [], error: '' };
    case 'set_ssh_config_hosts':
      return { ...state, sshConfigHosts: action.hosts, sshConfigLoading: false, error: '' };
    case 'set_ssh_config_loading':
      return { ...state, sshConfigLoading: action.loading };
    case 'select_config_host':
      return { ...state, selectedConfigHost: action.host };
    case 'set_workdir_chosen':
      return { ...state, workdirChosen: action.path, workdirCustom: '' };
    case 'set_workdir_custom':
      return { ...state, workdirCustom: action.path, workdirChosen: '__custom__' };
    case 'remove_workdir_path':
      return {
        ...state,
        workdirPaths: state.workdirPaths.filter((r) => r.path !== action.path),
        workdirChosen:
          state.workdirChosen === action.path ? state.workdirHome : state.workdirChosen,
      };
    case 'set_workdir_remember':
      return { ...state, workdirRemember: action.remember };
    case 'set_smb_host':
      return { ...state, smbHost: action.host, error: '' };
    case 'set_smb_user':
      return { ...state, smbUser: action.user, error: '' };
    case 'set_smb_domain':
      return { ...state, smbDomain: action.domain, error: '' };
    case 'set_smb_port':
      return { ...state, smbPort: action.port, error: '' };
    case 'set_share_chosen':
      return { ...state, shareChosen: action.name };
    case 'set_show_hidden_shares':
      return { ...state, showHiddenShares: action.show };
    default:
      return state;
  }
};
