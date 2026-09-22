import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  AIUsageService,
  BookmarkService,
  FileService,
  GitService,
  PortService,
  SettingsService,
} from '../../shared/api/bindings';
import { isRemotePath } from '../../features/connections/helpers';
import { isTrashPath } from '../../shared/lib/trash';
import type {
  AIUsage,
  AppSettings,
  DiskUsage,
  GitDirStatus,
  PaneTabsState,
  ThemePreference,
  PortListener,
  ProcessInfo,
  QuickPlace,
  Volume,
} from './types';
import { defaultSettings } from './types';

const queryKeys = {
  dir: (path: string, showHidden: boolean) => ['dir', path, showHidden] as const,
  gitStatus: (path: string) => ['gitStatus', path] as const,
  home: ['home'] as const,
  quickPlaces: ['quickPlaces'] as const,
  settings: ['settings'] as const,
  shortcuts: ['shortcuts'] as const,
  shortcutDefs: ['shortcutDefs'] as const,
  bookmarks: ['bookmarks'] as const,
  pathCompletions: (partial: string) => ['pathCompletions', partial] as const,
  paneTabs: ['paneTabs'] as const,
  volumes: ['volumes'] as const,
  ports: ['ports'] as const,
  processes: ['processes'] as const,
  diskUsage: (path: string) => ['diskUsage', path] as const,
  aiUsage: ['aiUsage'] as const,
};

export const useHomeDir = () => {
  return useQuery({
    queryKey: queryKeys.home,
    queryFn: () => FileService.GetHomeDir() as Promise<string>,
    staleTime: Infinity,
  });
};

export const useQuickPlaces = () => {
  return useQuery({
    queryKey: queryKeys.quickPlaces,
    queryFn: () => FileService.GetQuickPlaces() as Promise<QuickPlace[]>,
    staleTime: Infinity, // same rationale as useHomeDir — OS folders don't move mid-session
  });
};

export const useSettings = () => {
  return useQuery({
    queryKey: queryKeys.settings,
    queryFn: async () => {
      // Bindings type theme as string; normalize to our ThemePreference union.
      const s = await SettingsService.GetSettings();
      return normalizeSettings(s);
    },
  });
};

/** Accept loose binding payloads (generated types use string theme, etc.). */
const normalizeSettings = (
  s:
    | {
        theme?: string;
        showHidden?: boolean;
        showExtensions?: boolean;
        showGitStatus?: boolean;
        useBuiltInEditor?: boolean;
        autoCheckUpdates?: boolean;
        updateCheckIntervalDays?: number;
        lastUpdateCheckAt?: string;
        skippedUpdateVersion?: string;
        leftPath?: string;
        rightPath?: string;
      }
    | null
    | undefined,
): AppSettings => {
  const theme = s?.theme;
  const interval = s?.updateCheckIntervalDays;
  return {
    theme:
      theme === 'dark' || theme === 'light' || theme === 'system' ? theme : defaultSettings.theme,
    showHidden: Boolean(s?.showHidden),
    showExtensions: s?.showExtensions !== false,
    showGitStatus: s?.showGitStatus !== false,
    useBuiltInEditor: s?.useBuiltInEditor !== false,
    autoCheckUpdates: s?.autoCheckUpdates !== false,
    updateCheckIntervalDays:
      typeof interval === 'number' && interval > 0
        ? interval
        : defaultSettings.updateCheckIntervalDays,
    lastUpdateCheckAt: s?.lastUpdateCheckAt ?? '',
    skippedUpdateVersion: s?.skippedUpdateVersion ?? '',
    leftPath: s?.leftPath ?? '',
    rightPath: s?.rightPath ?? '',
  };
};

const settingsPayload = (settings: AppSettings) => ({
  theme: settings.theme,
  showHidden: settings.showHidden,
  showExtensions: settings.showExtensions,
  showGitStatus: settings.showGitStatus,
  useBuiltInEditor: settings.useBuiltInEditor,
  autoCheckUpdates: settings.autoCheckUpdates,
  updateCheckIntervalDays: settings.updateCheckIntervalDays,
  lastUpdateCheckAt: settings.lastUpdateCheckAt,
  skippedUpdateVersion: settings.skippedUpdateVersion,
  leftPath: settings.leftPath,
  rightPath: settings.rightPath,
});

export const useSaveSettings = () => {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (settings: AppSettings) => SettingsService.SaveSettings(settingsPayload(settings)),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: queryKeys.settings });
    },
  });
};

export const usePatchSettings = () => {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (patch: Partial<AppSettings>) => {
      const current =
        qc.getQueryData<AppSettings>(queryKeys.settings) ??
        normalizeSettings(await SettingsService.GetSettings());
      const next = { ...current, ...patch };
      await SettingsService.SaveSettings(settingsPayload(next));
      return next;
    },
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: queryKeys.settings });
    },
  });
};

export const useSearchTree = (
  root: string | undefined,
  query: string,
  showHidden: boolean,
  enabled: boolean,
) => {
  return useQuery({
    queryKey: ['searchTree', root ?? '', query, showHidden] as const,
    queryFn: async () => {
      const rows = await FileService.SearchTree(root!, query, showHidden, 80);
      return rows ?? [];
    },
    enabled: Boolean(enabled && root),
    staleTime: 2_000,
  });
};

export const useVolumes = () => {
  return useQuery({
    queryKey: queryKeys.volumes,
    queryFn: async () => ((await FileService.ListVolumes()) ?? []) as Volume[],
  });
};

export const usePorts = (enabled: boolean) => {
  return useQuery({
    queryKey: queryKeys.ports,
    queryFn: async () => ((await PortService.List()) ?? []) as PortListener[],
    enabled,
  });
};

export const useProcesses = (enabled: boolean) => {
  return useQuery({
    queryKey: queryKeys.processes,
    queryFn: async () => ((await PortService.ListProcesses()) ?? []) as ProcessInfo[],
    enabled,
  });
};

export const useKillPort = () => {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (pid: number) => PortService.Kill(pid),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: queryKeys.ports });
      void qc.invalidateQueries({ queryKey: queryKeys.processes });
    },
  });
};

export const useKillAllPorts = () => {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (pids: number[]) => PortService.KillAll(pids),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: queryKeys.ports });
      void qc.invalidateQueries({ queryKey: queryKeys.processes });
    },
  });
};

export const useAIUsage = (enabled: boolean) => {
  return useQuery({
    queryKey: queryKeys.aiUsage,
    queryFn: async () => ((await AIUsageService.List()) ?? []) as AIUsage[],
    enabled,
    retry: 0, // each retry re-spawns a multi-second CLI
    staleTime: 5 * 60_000, // reopening the popover shouldn't re-run claude
  });
};

export const useDirListing = (path: string | undefined, showHidden: boolean) => {
  return useQuery({
    queryKey: queryKeys.dir(path ?? '', showHidden),
    queryFn: async () => {
      const rows = await FileService.ListDir(path!, showHidden);
      return rows ?? [];
    },
    enabled: Boolean(path),
  });
};

export const useDiskUsage = (path: string | undefined) => {
  const remote = isRemotePath(path) || isTrashPath(path);
  return useQuery({
    queryKey: queryKeys.diskUsage(path ?? ''),
    queryFn: async (): Promise<DiskUsage> => {
      const u = await FileService.DiskUsage(path!);
      return {
        path: u.path ?? '',
        total: u.total ?? 0,
        free: u.free ?? 0,
        used: u.used ?? 0,
      };
    },
    enabled: Boolean(path && !remote),
    staleTime: 10_000,
  });
};

/** Git working-tree status for one local directory (parallel to ListDir). */
export const useGitDirStatus = (path: string | undefined, enabled: boolean) => {
  const remote = isRemotePath(path) || isTrashPath(path);
  return useQuery({
    queryKey: queryKeys.gitStatus(path ?? ''),
    queryFn: async (): Promise<GitDirStatus> => {
      const st = await GitService.StatusForDir(path!);
      return {
        repoRoot: st?.repoRoot ?? '',
        entries: (st?.entries ?? []).map((e) => ({
          name: e.name,
          status: e.status,
        })),
      };
    },
    enabled: Boolean(enabled && path && !remote),
    staleTime: 8_000,
  });
};

export const usePathCompletions = (partial: string, enabled: boolean) => {
  return useQuery({
    queryKey: queryKeys.pathCompletions(partial),
    queryFn: async () => {
      const rows = await FileService.ListPathCompletions(partial);
      return rows ?? [];
    },
    enabled: enabled && partial.length > 0,
    staleTime: 5_000,
  });
};

export const useBookmarks = () => {
  return useQuery({
    queryKey: queryKeys.bookmarks,
    queryFn: async () => {
      const list = await BookmarkService.List();
      return list ?? [];
    },
  });
};

export const useShortcutDefs = () => {
  return useQuery({
    queryKey: queryKeys.shortcutDefs,
    queryFn: async () => {
      const list = await SettingsService.ListShortcutDefs();
      return list ?? [];
    },
  });
};

export const useSaveShortcuts = () => {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (map: Record<string, string>) => SettingsService.SaveShortcuts(map),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: queryKeys.shortcutDefs });
      void qc.invalidateQueries({ queryKey: queryKeys.shortcuts });
    },
  });
};

export const usePaneTabs = () => {
  return useQuery({
    queryKey: queryKeys.paneTabs,
    queryFn: async () => {
      const t = await SettingsService.GetPaneTabs();
      return {
        left: t.left ?? [],
        leftActive: t.leftActive,
        right: t.right ?? [],
        rightActive: t.rightActive,
      };
    },
  });
};

export const useSavePaneTabs = () => {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (tabs: PaneTabsState) => SettingsService.SavePaneTabs(tabs),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: queryKeys.paneTabs });
    },
  });
};

/** Theme-only settings patch — use mutate / onSuccess / onError. */
export const useSetTheme = () => {
  const patch = usePatchSettings();
  return {
    mutate: (
      theme: ThemePreference,
      options?: {
        onSuccess?: () => void;
        onError?: (e: unknown) => void;
      },
    ) => {
      patch.mutate({ theme }, options);
    },
    isPending: patch.isPending,
  };
};

const useInvalidateDirs = () => {
  const qc = useQueryClient();
  return (...paths: string[]) => {
    for (const p of paths) {
      if (p) {
        void qc.invalidateQueries({ queryKey: ['dir', p] });
        void qc.invalidateQueries({ queryKey: ['gitStatus', p] });
      }
    }
  };
};

export const useFileOps = () => {
  const invalidate = useInvalidateDirs();
  const qc = useQueryClient();

  const del = useMutation({
    mutationFn: (paths: string[]) => FileService.Delete(paths),
    onSuccess: (_d, paths) => invalidate(...parentDirs(paths)),
  });

  const delPermanent = useMutation({
    mutationFn: (paths: string[]) => FileService.DeletePermanent(paths),
    onSuccess: (_d, paths) => invalidate(...parentDirs(paths)),
  });

  const rename = useMutation({
    mutationFn: ({ oldPath, newName }: { oldPath: string; newName: string }) =>
      FileService.Rename(oldPath, newName),
    onSuccess: (_d, v) => invalidate(...parentDirs([v.oldPath])),
  });

  const mkdir = useMutation({
    mutationFn: ({ parent, name }: { parent: string; name: string }) =>
      FileService.Mkdir(parent, name),
    onSuccess: (_d, v) => invalidate(v.parent),
  });

  const mkfile = useMutation({
    mutationFn: ({ parent, name }: { parent: string; name: string }) =>
      FileService.CreateFile(parent, name),
    onSuccess: (_d, v) => invalidate(v.parent),
  });

  const addBookmark = useMutation({
    mutationFn: ({ name, path }: { name: string; path: string }) => BookmarkService.Add(name, path),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: queryKeys.bookmarks });
    },
  });

  const removeBookmark = useMutation({
    mutationFn: (id: number) => BookmarkService.Remove(id),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: queryKeys.bookmarks });
    },
  });

  return { del, delPermanent, rename, mkdir, mkfile, addBookmark, removeBookmark };
};

const parentDirs = (paths: string[]): string[] => {
  const set = new Set<string>();
  for (const p of paths) {
    const idx = Math.max(p.lastIndexOf('/'), p.lastIndexOf('\\'));
    if (idx > 0) set.add(p.slice(0, idx));
  }
  return [...set];
};
