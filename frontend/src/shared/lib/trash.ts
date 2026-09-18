export const TRASH_URI = 'trash://';

export const isTrashPath = (p?: string | null): boolean => {
  if (!p) return false;
  const s = p.trim().toLowerCase();
  return s === 'trash:' || s === 'trash://' || s.startsWith('trash://');
};
