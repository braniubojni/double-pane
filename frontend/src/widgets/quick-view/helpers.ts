import type { Preview } from '../../entities/file/types';

export const MSG_SELECT = 'Select a file';
export const MSG_LOCAL = 'Preview is only available for local files';
const MSG_TOO_LARGE = 'File is too large to preview';
const MSG_UNSUPPORTED = 'This file type cannot be previewed';

export const showOpenAction = (message?: string): boolean =>
  message === MSG_TOO_LARGE || message === MSG_UNSUPPORTED;

export const previewBytes = (data: Preview['data']): Uint8Array | null => {
  if (!data || (typeof data === 'string' && data.length === 0) || data.length === 0) return null;
  if (typeof data === 'string') {
    const bin = atob(data);
    const out = new Uint8Array(bin.length);
    for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i);
    return out;
  }
  return Uint8Array.from(data);
};
