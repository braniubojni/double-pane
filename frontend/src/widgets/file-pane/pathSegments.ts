import { REMOTE_ORIGIN_RE } from '../../features/connections/helpers';
import { isTrashPath, TRASH_URI } from '../../shared/lib/trash';

export type PathCrumb = { label: string; path: string };

export function pathCrumbs(path: string): PathCrumb[] {
  const raw = path.trim() || '/';
  if (isTrashPath(raw)) return [{ label: 'Trash', path: TRASH_URI }];
  const remote = raw.replace(/\?[^/]*$/, '').match(REMOTE_ORIGIN_RE);
  if (remote) return remoteCrumbs(remote[1], remote[2] || '/', /^smb:/i.test(remote[1]));
  if (/^[a-zA-Z]:/.test(raw)) return windowsCrumbs(raw);
  return posixCrumbs(raw);
}

function posixCrumbs(path: string): PathCrumb[] {
  if (path === '/') return [{ label: '/', path: '/' }];
  const segs = path.split('/').filter(Boolean);
  const crumbs: PathCrumb[] = [{ label: '/', path: '/' }];
  let acc = '';
  for (const s of segs) {
    acc += `/${s}`;
    crumbs.push({ label: s, path: acc });
  }
  return crumbs;
}

function windowsCrumbs(path: string): PathCrumb[] {
  const drive = path.slice(0, 2);
  const root = `${drive}\\`;
  const rest = path.slice(2).replace(/^[\\/]+/, '');
  const crumbs: PathCrumb[] = [{ label: root, path: root }];
  const segs = rest.split(/[\\/]/).filter(Boolean);
  let acc = drive;
  for (const s of segs) {
    acc += `\\${s}`;
    crumbs.push({ label: s, path: acc });
  }
  return crumbs;
}

function remoteCrumbs(origin: string, rest: string, smb: boolean): PathCrumb[] {
  const segs = rest.split('/').filter(Boolean);
  if (smb) {
    if (!segs.length) {
      const root = `${origin}/`;
      return [{ label: root, path: root }];
    }
    const sharePath = `${origin}/${segs[0]}`;
    const crumbs: PathCrumb[] = [{ label: sharePath, path: sharePath }];
    let acc = sharePath;
    for (const s of segs.slice(1)) {
      acc += `/${s}`;
      crumbs.push({ label: s, path: acc });
    }
    return crumbs;
  }
  const root = `${origin}/`;
  const crumbs: PathCrumb[] = [{ label: root, path: root }];
  let acc = origin;
  for (const s of segs) {
    acc += `/${s}`;
    crumbs.push({ label: s, path: acc });
  }
  return crumbs;
}
