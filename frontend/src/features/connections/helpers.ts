/**
 * Session key for a remote virtual path — mirrors remote.Spec.SessionKey.
 * Null for local paths.
 */
export const REMOTE_ORIGIN_RE = /^((?:ssh|smb|mega|ftp):\/\/[^/]+)(\/.*)?$/i;

export const isRemotePath = (p?: string | null): boolean =>
  Boolean(p && /^(ssh|smb|mega|ftp):\/\//i.test(p.trim()));

export const isSMBPath = (p?: string | null): boolean => Boolean(p && /^smb:\/\//i.test(p.trim()));

export const isMEGAPath = (p?: string | null): boolean =>
  Boolean(p && /^mega:\/\//i.test(p.trim()));

export const sessionKeyFromPath = (path: string): string | null => {
  const raw = path.trim();
  if (!isRemotePath(raw)) return null;
  try {
    const u = new URL(raw);
    const scheme = u.protocol.replace(/:$/, '').toLowerCase();
    if (scheme !== 'ssh' && scheme !== 'smb' && scheme !== 'mega' && scheme !== 'ftp') return null;
    // JS URL.hostname keeps IPv6 brackets; Go url.Hostname / Spec.Host do not.
    let host = u.hostname;
    if (host.startsWith('[') && host.endsWith(']')) host = host.slice(1, -1);
    if (!host) return null;
    const user = decodeURIComponent(u.username || '');
    if ((scheme === 'ssh' || scheme === 'mega' || scheme === 'ftp') && !user) return null;
    if (scheme === 'mega') return `mega:${user}@${host}`;
    const port = u.port || (scheme === 'smb' ? '445' : scheme === 'ftp' ? '21' : '22');
    if (scheme === 'smb') return `smb:${user}@${host}:${port}`;
    if (scheme === 'ftp') return `ftp:${user}@${host}:${port}`;
    return `${user}@${host}:${port}`;
  } catch {
    return null;
  }
};

export const sessionKeyFromProfile = (p: {
  protocol?: string;
  user: string;
  host: string;
  port: number;
}): string => {
  if (p.protocol === 'mega') {
    return `mega:${p.user}@${p.host}`;
  }
  if (p.protocol === 'ftp') {
    return `ftp:${p.user}@${p.host}:${p.port || 21}`;
  }
  if (p.protocol === 'smb') {
    return `smb:${p.user}@${p.host}:${p.port || 445}`;
  }
  return `${p.user}@${p.host}:${p.port || 22}`;
};

/** Parent directory for ssh://, smb://, or mega:// virtual paths. */
export const parentOfVirtualPath = (path: string): string | null => {
  const stripped = path.replace(/\?[^/]*$/, '');
  const m = stripped.match(REMOTE_ORIGIN_RE);
  if (!m) return null;
  const base = m[1];
  const p = m[2] || '/';
  const parent = p.replace(/\/+$/, '').split('/').slice(0, -1).join('/') || '/';
  return `${base}${parent === '/' ? '/' : parent}`;
};

/** True when the pane error means the remote session is gone / never dialled. */
export const isNotConnectedMessage = (msg: string): boolean =>
  /not connected to .*connect first/i.test(msg);

export const isLocalNetworkErrorMessage = (msg: string): boolean => {
  const m = msg.toLowerCase();
  return (
    m.includes('cannot reach') ||
    m.includes('local network') ||
    m.includes('no route to host') ||
    m.includes('no such host') ||
    m.includes('network is unreachable') ||
    m.includes('connection refused') ||
    m.includes('i/o timeout') ||
    m.includes('operation timed out') ||
    m.includes('host is down') ||
    m.includes('unreachable host') ||
    m.includes('unreachable network') ||
    m.includes('connection attempt failed') ||
    m.includes('actively refused') ||
    m.includes('forcibly closed') ||
    m.includes('connectex') ||
    m.includes('firewall')
  );
};

export const isAuthErrorMessage = (msg: string): boolean => {
  if (isLocalNetworkErrorMessage(msg)) return false;
  const m = msg.toLowerCase();
  return (
    m.includes('authentication required') ||
    m.includes('unable to authenticate') ||
    m.includes('no authentication methods') ||
    m.includes('public key auth failed') ||
    m.includes('passphrase') ||
    m.includes('permission denied') ||
    m.includes('logon failure') ||
    m.includes('logon is invalid') ||
    m.includes('bad username') ||
    m.includes('2fa') ||
    m.includes('two-factor') ||
    m.includes('multi-factor') ||
    m.includes('mega login failed') ||
    m.includes('ftp login failed')
  );
};

/** Host field only — strip a pasted smb:// URL or UNC \\server\share. */
export const normalizeSMBHost = (raw: string): string => {
  let h = raw.trim().replace(/^['"]|['"]$/g, '');
  const unc = h.match(/^[/\\]{2}([^/\\]+)(?:[/\\].*)?$/);
  if (unc) return unc[1];
  if (/^smb:\/\//i.test(h)) {
    try {
      return new URL(h).hostname;
    } catch {
      h = h.replace(/^smb:\/\//i, '');
    }
  }
  return h.replace(/\/+$/, '').split('/')[0] ?? h;
};

/** Build a virtual path from session home/root + absolute or relative remote path. */
export const resolveRemoteWorkdirInput = (homePath: string, input: string): string | null => {
  const raw = input.trim();
  if (!raw) return null;
  if (isRemotePath(raw)) return raw;
  const m = homePath.match(REMOTE_ORIGIN_RE);
  if (!m) return null;
  const origin = m[1];
  if (raw.startsWith('/')) return `${origin}${raw}`;
  const homeAbs = (m[2] || '/').replace(/\/$/, '') || '';
  return `${origin}${homeAbs}/${raw}`.replace(/([^:]\/)\/+/g, '$1');
};

/** Bracket IPv6 literals for URL authority (rfc3986); leave hostnames/IPv4 alone. */
const formatSMBHostForURL = (host: string): string => {
  const h = host.trim();
  if (!h) return h;
  if (h.startsWith('[') && h.endsWith(']')) return h;
  // Two or more colons ⇒ IPv6 (or IPv4-mapped); single colon is host:port, already split.
  if ((h.match(/:/g) ?? []).length >= 2) return `[${h}]`;
  return h;
};

export const buildSMBSpec = (host: string, user: string, domain: string, port: number): string => {
  const h = formatSMBHostForURL(normalizeSMBHost(host));
  const u = user.trim();
  const d = domain.trim();
  const p = port > 0 ? port : 445;
  const origin = u ? `smb://${u}@${h}:${p}/` : `smb://${h}:${p}/`;
  if (!d) return origin;
  return `${origin}?domain=${encodeURIComponent(d)}`;
};
