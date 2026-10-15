import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useReducer, useRef, useState } from 'react';
import {
  addConnectionReducer,
  initialAddConnectionState,
} from '../../../features/connections/addConnectionReducer';
import {
  useConnectRequestStore,
  type ConnectRequest,
} from '../../../features/connections/connectRequestStore';
import { ensureSessionThenNavigate } from '../../../features/connections/navigate';
import {
  buildSMBSpec,
  isAuthErrorMessage,
  isLocalNetworkErrorMessage,
  isMEGAPath,
  isSMBPath,
  resolveRemoteWorkdirInput,
} from '../../../features/connections/helpers';
import type {
  ActiveSession,
  ConnectionProfile,
  RemoteRecent,
  SMBShare,
  SSHConfigHost,
} from '../../../features/connections/types';
import { usePaneStore } from '../../../features/pane/paneStore';
import { smbAuthFailMessage } from '../../../features/connections/smbCopy';
import { parseSMBForm, smbShareError } from '../../../features/connections/smbValidate';
import { megaEmailOk } from '../components/MEGAConnectFields';
import { ConnectionService } from '../../../shared/api/bindings';
import { errMessage } from '../../../shared/lib/format';
import { useSnack } from '../../../shared/ui/SnackbarHost';
import { enterPaneTab } from '../../file-pane/helpers';

type ConnectRes = {
  rootPath: string;
  homePath: string;
  key: string;
  profileId?: string;
  defaultWorkDir?: string;
  shares?: SMBShare[] | null;
  localMount?: boolean;
};

export const useConnections = () => {
  const [anchor, setAnchor] = useState<null | HTMLElement>(null);
  const [dialog, dispatch] = useReducer(addConnectionReducer, initialAddConnectionState);
  const show = useSnack((s) => s.show);
  const qc = useQueryClient();
  const navigate = usePaneStore((s) => s.navigate);
  const activePane = usePaneStore((s) => s.activePane);

  const profilesQ = useQuery({
    queryKey: ['connections', 'profiles'],
    queryFn: async () => ((await ConnectionService.ListProfiles()) ?? []) as ConnectionProfile[],
  });

  const sessionsQ = useQuery({
    queryKey: ['connections', 'sessions'],
    queryFn: async () => ((await ConnectionService.ListSessions()) ?? []) as ActiveSession[],
  });

  const profiles = profilesQ.data ?? [];
  const sessions = sessionsQ.data ?? [];
  const sessionKeys = new Set(sessions.map((s) => s.key));
  const sshProfiles = profiles.filter((p) => p.protocol === 'ssh' || !p.protocol);
  const smbProfiles = profiles.filter((p) => p.protocol === 'smb');
  const megaProfiles = profiles.filter((p) => p.protocol === 'mega');

  const refresh = () => {
    void qc.invalidateQueries({ queryKey: ['connections'] });
    void qc.invalidateQueries({ queryKey: ['dir'] });
    void qc.invalidateQueries({ queryKey: ['gitStatus'] });
  };

  const landConnected = (path: string) => {
    enterPaneTab(activePane, path);
    navigate(activePane, path);
    refresh();
    show('Connected', 'success');
    dispatch({ type: 'close' });
    setAnchor(null);
  };

  /** After SSH: workdir picker. After SMB: share picker (skip if default share saved). */
  const afterConnect = async (res: ConnectRes) => {
    if (res.localMount) {
      const locals = (res.shares ?? [])
        .map((s) => s.localPath)
        .filter((p): p is string => Boolean(p));
      if (locals.length === 1) {
        landConnected(locals[0]);
        return;
      }
      if (locals.length > 1) {
        dispatch({
          type: 'open_smb_shares',
          shares: res.shares ?? [],
          rootPath: res.rootPath,
          sessionKey: res.key,
          profileId: res.profileId,
        });
        return;
      }
      landConnected(res.rootPath);
      return;
    }
    if (isMEGAPath(res.rootPath) || res.rootPath.startsWith('mega:')) {
      const dest = res.defaultWorkDir || res.homePath || res.rootPath;
      void ConnectionService.AddRecentPath(dest);
      landConnected(dest);
      return;
    }
    if (isSMBPath(res.rootPath) || (res.shares && res.shares.length > 0)) {
      if (res.defaultWorkDir) {
        void ConnectionService.AddRecentPath(res.defaultWorkDir);
        landConnected(res.defaultWorkDir);
        return;
      }
      dispatch({
        type: 'open_smb_shares',
        shares: res.shares ?? [],
        rootPath: res.rootPath,
        sessionKey: res.key,
        profileId: res.profileId,
      });
      return;
    }
    const homePath = res.homePath || res.rootPath;
    const recents = ((await ConnectionService.GetRecentPaths(res.key).catch(() => null)) ??
      []) as RemoteRecent[];
    const chosen = res.defaultWorkDir || homePath;
    dispatch({
      type: 'open_workdir',
      paths: recents,
      home: homePath,
      sessionKey: res.key,
      chosen,
      profileId: res.profileId,
    });
  };

  const confirmWorkdir = async () => {
    let path = dialog.workdirChosen;
    if (path === '__custom__' || dialog.workdirCustom.trim()) {
      const resolved = resolveRemoteWorkdirInput(dialog.workdirHome, dialog.workdirCustom);
      if (!resolved) {
        dispatch({ type: 'set_error', error: 'Enter a remote path (e.g. /home/user/project)' });
        return;
      }
      path = resolved;
    }
    if (!path) path = dialog.workdirHome;
    void ConnectionService.AddRecentPath(path);
    if (dialog.workdirRemember && dialog.workdirProfileId) {
      try {
        await ConnectionService.SetProfileDefaultWorkDir(dialog.workdirProfileId, path);
        void qc.invalidateQueries({ queryKey: ['connections', 'profiles'] });
      } catch {
        // non-fatal
      }
    }
    landConnected(path);
  };

  const confirmSMBShare = async () => {
    const name = dialog.shareChosen.trim();
    const shareErr = smbShareError(name);
    if (shareErr) {
      dispatch({ type: 'set_error', error: shareErr });
      return;
    }
    const picked = dialog.shares.find((s) => s.name === name);
    const root = dialog.smbRootPath.replace(/\/+$/, '');
    const virtualPath = `${root}/${name}/`;
    const navPath = picked?.localPath || virtualPath;
    void ConnectionService.AddRecentPath(navPath);
    if (dialog.workdirRemember && dialog.workdirProfileId) {
      try {
        await ConnectionService.SetProfileDefaultWorkDir(dialog.workdirProfileId, virtualPath);
        void qc.invalidateQueries({ queryKey: ['connections', 'profiles'] });
      } catch {
        // non-fatal
      }
    }
    landConnected(navPath);
  };

  const connectMEGA = async (email: string, password: string, totp: string, save: boolean) => {
    const res = await ConnectionService.ConnectMEGA(email, password, totp, save);
    await afterConnect(res);
  };

  const connectProfile = async (id: string, password = '', totp = '') => {
    try {
      const p = profiles.find((x) => x.id === id);
      if (p?.protocol === 'mega') {
        const res = await ConnectionService.ConnectMEGA(
          `${p.user}@${p.host}`,
          password,
          totp,
          false,
        );
        await afterConnect({
          ...res,
          profileId: p.id,
          defaultWorkDir: p.defaultWorkDir,
        });
        return;
      }
      const res = await ConnectionService.ConnectProfile(id, password);
      await afterConnect(res);
    } catch (e) {
      const msg = errMessage(e);
      if (isAuthErrorMessage(msg) && !password) {
        const p = profiles.find((x) => x.id === id);
        dispatch({
          type: 'open_password',
          profileId: id,
          label: p?.label,
          mega: p?.protocol === 'mega',
        });
        return;
      }
      show(msg, 'error');
      dispatch({ type: 'set_error', error: msg });
    }
  };

  const connectFromConfig = async (host: SSHConfigHost, password = '') => {
    dispatch({ type: 'select_config_host', host });
    dispatch({ type: 'set_busy', busy: true });
    try {
      const res = await ConnectionService.ConnectFromConfig(host, password, dialog.save);
      await afterConnect(res);
    } catch (e) {
      const msg = errMessage(e);
      if (isAuthErrorMessage(msg) && !password) {
        dispatch({ type: 'need_password' });
        return;
      }
      dispatch({ type: 'set_error', error: msg });
      show(msg, 'error');
    } finally {
      dispatch({ type: 'set_busy', busy: false });
    }
  };

  const openSSHConfigMode = async () => {
    dispatch({ type: 'open_ssh_config' });
    try {
      const paths = await ConnectionService.DefaultSSHConfigPaths();
      const first = paths?.[0] ?? '';
      if (first) dispatch({ type: 'set_ssh_config_path', path: first });
    } catch {
      // non-fatal — user can type the path manually
    }
  };

  const loadSSHConfig = async (configPath: string) => {
    dispatch({ type: 'set_ssh_config_loading', loading: true });
    try {
      const hosts = await ConnectionService.ListSSHConfigHosts(configPath);
      dispatch({ type: 'set_ssh_config_hosts', hosts: (hosts ?? []) as SSHConfigHost[] });
    } catch (e) {
      dispatch({ type: 'set_error', error: errMessage(e) });
    }
  };

  const onForgetRecent = (path: string) => {
    dispatch({ type: 'remove_workdir_path', path });
    void ConnectionService.RemoveRecentPath(path).catch((e) => show(errMessage(e), 'error'));
  };

  const onMenuConnect = (p: ConnectionProfile) => {
    setAnchor(null);
    void connectProfile(p.id);
  };

  const onDisconnect = async (key: string) => {
    try {
      await ConnectionService.Disconnect(key);
      refresh();
      show('Disconnected', 'info');
      setAnchor(null);
    } catch (e) {
      show(errMessage(e), 'error');
    }
  };

  const onRemove = async (id: string) => {
    try {
      await ConnectionService.RemoveProfile(id);
      refresh();
    } catch (e) {
      show(errMessage(e), 'error');
    }
  };

  /**
   * A pane (bookmark jump / reconnect prompt) hit an auth wall. Reuse this
   * dialog for the password instead of owning a second one, then send the pane
   * to the path it originally asked for — no workdir picker, the target is known.
   */
  const pendingConnect = useConnectRequestStore((s) => s.request);
  const pendingNonce = useConnectRequestStore((s) => s.nonce);
  // nonce re-opens the dialog even for a repeat of the same request
  const [seen, setSeen] = useState<{ request?: ConnectRequest | null; nonce?: number }>({});
  if (seen.request !== pendingConnect || seen.nonce !== pendingNonce) {
    setSeen({ request: pendingConnect, nonce: pendingNonce });
    if (pendingConnect) {
      setAnchor(null);
      dispatch({
        type: 'open_password',
        profileId: pendingConnect.profileId ?? '',
        label: pendingConnect.label,
        mega:
          pendingConnect.spec?.startsWith('mega:') ||
          isMEGAPath(pendingConnect.targetPath) ||
          pendingConnect.spec?.startsWith('mega://'),
      });
    }
  }

  // Drop the pending request when the password dialog goes away (cancel or
  // success), so a later unrelated password prompt can't inherit it.
  const inPasswordDialog = useRef(false);
  useEffect(() => {
    if (dialog.open && dialog.mode === 'password') {
      inPasswordDialog.current = true;
      return;
    }
    if (inPasswordDialog.current) {
      inPasswordDialog.current = false;
      useConnectRequestStore.getState().consume();
    }
  }, [dialog.open, dialog.mode]);

  const submitPendingConnect = async (req: ConnectRequest) => {
    try {
      const mega =
        req.spec?.startsWith('mega:') ||
        isMEGAPath(req.targetPath) ||
        Boolean(req.spec?.startsWith('mega://'));
      if (mega) {
        const email = req.spec?.startsWith('mega:')
          ? req.spec.slice(5)
          : req.spec && isMEGAPath(req.spec)
            ? `${new URL(req.spec).username}@${new URL(req.spec).hostname}`
            : '';
        const fromProfile = profiles.find((x) => x.id === req.profileId);
        await ConnectionService.ConnectMEGA(
          fromProfile ? `${fromProfile.user}@${fromProfile.host}` : email,
          dialog.password,
          dialog.totp,
          false,
        );
      } else if (req.profileId)
        await ConnectionService.ConnectProfile(req.profileId, dialog.password);
      else await ConnectionService.ConnectSpec(req.spec ?? '', dialog.password, false);
    } catch (e) {
      const msg = errMessage(e);
      dispatch({ type: 'set_error', error: msg });
      return;
    }
    useConnectRequestStore.getState().consume();
    dispatch({ type: 'close' });
    refresh();
    await ensureSessionThenNavigate(req.paneId, req.targetPath);
  };

  const submitDialog = async () => {
    if (dialog.mode === 'workdir') {
      await confirmWorkdir();
      return;
    }
    if (dialog.mode === 'smb_shares') {
      await confirmSMBShare();
      return;
    }
    if (dialog.mode === 'add_mega') {
      if (!megaEmailOk(dialog.spec)) {
        dispatch({ type: 'set_error', error: 'Enter a MEGA email (you@example.com)' });
        return;
      }
      dispatch({ type: 'set_busy', busy: true });
      try {
        await connectMEGA(dialog.spec.trim(), dialog.password, dialog.totp, dialog.save);
      } catch (e) {
        dispatch({ type: 'set_error', error: errMessage(e) });
      } finally {
        dispatch({ type: 'set_busy', busy: false });
      }
      return;
    }
    if (dialog.mode === 'add_smb') {
      const parsed = parseSMBForm({
        host: dialog.smbHost,
        user: dialog.smbUser,
        domain: dialog.smbDomain,
        port: dialog.smbPort,
      });
      if (!parsed.ok) {
        dispatch({
          type: 'set_error',
          error:
            parsed.errors.host ||
            parsed.errors.port ||
            parsed.errors.user ||
            parsed.errors.domain ||
            'Fix the highlighted fields before connecting',
        });
        return;
      }
      dispatch({ type: 'open_smb_confirm' });
      return;
    }
    dispatch({ type: 'set_busy', busy: true });
    try {
      if (dialog.mode === 'password' && pendingConnect) {
        await submitPendingConnect(pendingConnect);
        return;
      }
      if (dialog.mode === 'ssh_config') {
        if (dialog.selectedConfigHost) {
          await connectFromConfig(dialog.selectedConfigHost, dialog.password);
        }
        return;
      }
      if (dialog.mode === 'password' && dialog.profileId) {
        await connectProfile(dialog.profileId, dialog.password, dialog.totp);
        return;
      }
      if (dialog.mode === 'smb_confirm') {
        const parsed = parseSMBForm({
          host: dialog.smbHost,
          user: dialog.smbUser,
          domain: dialog.smbDomain,
          port: dialog.smbPort,
        });
        if (!parsed.ok) {
          dispatch({ type: 'set_error', error: 'Fix the connection fields first' });
          return;
        }
        const spec = buildSMBSpec(parsed.host, parsed.user, parsed.domain, parsed.port);
        try {
          const res = await ConnectionService.ConnectSpec(spec, dialog.password, dialog.save);
          await afterConnect(res);
        } catch (e) {
          const msg = errMessage(e);
          if (/canceled|cancelled/i.test(msg)) {
            dispatch({ type: 'set_error', error: 'Connection canceled' });
            return;
          }
          if (isLocalNetworkErrorMessage(msg)) {
            dispatch({ type: 'set_error', error: msg });
            return;
          }
          if (isAuthErrorMessage(msg)) {
            dispatch({ type: 'back_smb_form' });
            dispatch({
              type: 'set_error',
              error: smbAuthFailMessage(),
            });
            return;
          }
          throw e;
        }
        return;
      }
      // mode === 'add'
      const spec = dialog.spec.trim();
      if (!spec) {
        dispatch({
          type: 'set_error',
          error: 'Enter ssh user@host, or an SSH config Host alias (e.g. pahestain)',
        });
        return;
      }
      await ConnectionService.ParseSpec(spec);
      try {
        const res = await ConnectionService.ConnectSpec(spec, dialog.password, dialog.save);
        await afterConnect(res);
      } catch (e) {
        const msg = errMessage(e);
        if (isAuthErrorMessage(msg) && !dialog.password) {
          dispatch({ type: 'need_password' });
          return;
        }
        throw e;
      }
    } catch (e) {
      dispatch({ type: 'set_error', error: errMessage(e) });
    } finally {
      dispatch({ type: 'set_busy', busy: false });
    }
  };

  return {
    anchor,
    setAnchor,
    dialog,
    dispatch,
    sshProfiles,
    smbProfiles,
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
  };
};
