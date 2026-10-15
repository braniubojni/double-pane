import { useCallback, useEffect, useState } from 'react';
import { useSettings } from '../../../entities/file/queries';
import type { FileEntry } from '../../../entities/file/types';
import { FileService } from '../../../shared/api/bindings';
import { errMessage } from '../../../shared/lib/format';
import { useSnack } from '../../../shared/ui/SnackbarHost';

export type TreeChildren = Record<string, FileEntry[] | undefined>;

export const useFolderTree = (rootPath: string) => {
  const { data: settings } = useSettings();
  const showHidden = settings?.showHidden ?? false;
  const show = useSnack((s) => s.show);
  const [children, setChildren] = useState<TreeChildren>({});
  const [expanded, setExpanded] = useState<Record<string, boolean>>({});

  const loadDir = useCallback(
    (dir: string) => {
      void FileService.ListDir(dir, showHidden)
        .then((rows) => {
          const list = ((rows ?? []) as FileEntry[]).filter((e) => e.name !== '..');
          setChildren((c) => ({ ...c, [dir]: list }));
        })
        .catch((e) => show(errMessage(e), 'error'));
    },
    [showHidden, show],
  );

  const [synced, setSynced] = useState<{ rootPath: string; loadDir?: typeof loadDir }>({
    rootPath: '',
  });
  if (rootPath && (synced.rootPath !== rootPath || synced.loadDir !== loadDir)) {
    setSynced({ rootPath, loadDir });
    setChildren({});
    setExpanded({ [rootPath]: true });
  }

  useEffect(() => {
    if (rootPath) loadDir(rootPath);
  }, [rootPath, loadDir]);

  const toggle = (dir: string) => {
    setExpanded((e) => {
      const next = !e[dir];
      if (next && children[dir] === undefined) loadDir(dir);
      return { ...e, [dir]: next };
    });
  };

  return { children, expanded, toggle, loadDir };
};
