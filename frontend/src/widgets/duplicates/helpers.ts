import { formatSize } from '../../shared/lib/format';

export const formatEta = (seconds: number): string => {
  if (seconds < 1) return '<1s';
  if (seconds < 60) return `${seconds}s`;
  const m = Math.floor(seconds / 60);
  const s = seconds % 60;
  if (m < 60) return s ? `${m}m ${s}s` : `${m}m`;
  const h = Math.floor(m / 60);
  const rm = m % 60;
  return rm ? `${h}h ${rm}m` : `${h}h`;
};

export const mergeConfirmCopy = (protocol: string): string => {
  if (protocol === 'mega') {
    return 'Moved to MEGA trash. Restore from MEGA, not from this app.';
  }
  if (protocol === 'ssh' || protocol === 'smb') {
    return 'Permanent delete. App Undo will not restore these.';
  }
  return 'Moved to the system trash. Undo from the notification or open Trash.';
};

export const megaDiskWarning = (byteCount: number): string =>
  `Files will be downloaded to a temp cache once, then hashed. Need about ${formatSize(byteCount, false)} free disk in the config dir.`;

export const hashPrefix = (hash: string): string => hash.slice(0, 12);
