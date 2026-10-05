export type PaneId = 'left' | 'right';

export type ThemePreference = 'system' | 'dark' | 'light';

export interface FileEntry {
  name: string;
  path: string;
  isDir: boolean;
  size: number;
  modTime: number;
  ext: string;
  isSymlink: boolean;
  /** 'full' | 'readonly' | 'partial' | 'none', or '' when unknown (remote). */
  access: string;
  /** Put Back path for trash:// rows. */
  origin?: string;
  /** Folder-vault root (locked or unlocked) or a .dpenc file. */
  isVault?: boolean;
  /** True when isVault is true and no session is unlocked for it. */
  vaultLocked?: boolean;
  /** Single-file vault (.dpenc) rather than a folder vault. */
  vaultFile?: boolean;
}

/** One folder/file password vault (see internal/vault, spec: folder-vault/v1). */
export interface VaultInfo {
  root: string;
  locked: boolean;
  isFile: boolean;
  hint?: string;
  version: number;
}

/** Streamed while a create/create-file/remove vault job runs (see "vault:progress"). */
export interface VaultJobEvent {
  jobId: string;
  kind: 'create' | 'create-file' | 'remove';
  path: string;
  filesDone: number;
  filesTotal: number;
  bytesDone: number;
  bytesTotal: number;
  current: string;
  err?: string;
  done: boolean;
}

export interface PortListener {
  port: number;
  pid: number;
  process: string;
  proto: string;
}

export interface ProcessInfo {
  pid: number;
  name: string;
  cmd: string;
  cwd?: string;
}

export interface AIUsageLimit {
  label: string;
  percent: number;
  resetAt?: string;
}

export interface AIUsageDetail {
  label: string;
  value: string;
  depth: number;
}

export interface AIUsage {
  id: string;
  name: string;
  /** 'ok' | 'not-installed' | 'unsupported' | 'error' */
  status: string;
  error?: string;
  estimate: boolean;
  limits: AIUsageLimit[];
  details: AIUsageDetail[];
}

export interface Volume {
  path: string;
  name: string;
  kind: string;
  unmountable: boolean;
  sourcePath?: string;
  device?: string;
}

export interface QuickPlace {
  name: string;
  path: string;
}

export interface DiskUsage {
  path: string;
  total: number;
  free: number;
  used: number;
}

interface TabState {
  path: string;
}

export interface PaneTabsState {
  left: TabState[];
  leftActive: number;
  right: TabState[];
  rightActive: number;
}

export interface AppSettings {
  theme: ThemePreference;
  showHidden: boolean;
  showExtensions: boolean;
  showGitStatus: boolean;
  useBuiltInEditor: boolean;
  autoCheckUpdates: boolean;
  updateCheckIntervalDays: number;
  lastUpdateCheckAt: string;
  skippedUpdateVersion: string;
  leftPath: string;
  rightPath: string;
  /** Auto-lock unlocked vaults after this many idle minutes. 0 disables it. */
  vaultIdleLockMinutes: number;
}

export interface GitDirStatus {
  repoRoot: string;
  entries: { name: string; status: string }[];
}

export interface SearchHit {
  name: string;
  path: string;
  isDir: boolean;
  relPath: string;
}

export const defaultSettings: AppSettings = {
  theme: 'system',
  showHidden: false,
  showExtensions: true,
  showGitStatus: true,
  useBuiltInEditor: true,
  autoCheckUpdates: true,
  updateCheckIntervalDays: 10,
  lastUpdateCheckAt: '',
  skippedUpdateVersion: '',
  leftPath: '',
  rightPath: '',
  vaultIdleLockMinutes: 5,
};
