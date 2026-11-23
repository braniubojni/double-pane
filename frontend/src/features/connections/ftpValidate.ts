import { parseSMBForm } from './smbValidate';

export type FTPFieldErrors = {
  host?: string;
  user?: string;
  port?: string;
};

export const parseFTPForm = (input: { host: string; user: string; port: string }) => {
  const port = input.port.trim() || '21';
  const parsed = parseSMBForm({ host: input.host, user: input.user, domain: '', port });
  const user = input.user.trim();
  const errors: FTPFieldErrors = {
    host: parsed.errors.host,
    user: user ? parsed.errors.user : 'Enter a username, or anonymous',
    port: parsed.errors.port,
  };
  return {
    ok: !errors.host && !errors.user && !errors.port,
    host: parsed.host,
    user,
    port: Number(port) || 21,
    errors,
  };
};

export const buildFTPSpec = (host: string, user: string, port: number): string => {
  const hostPart = host.includes(':') ? `[${host}]` : host;
  return `ftp://${encodeURIComponent(user)}@${hostPart}:${port}/`;
};
