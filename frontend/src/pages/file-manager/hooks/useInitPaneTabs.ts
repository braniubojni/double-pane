import { useEffect } from 'react';
import { useHomeDir, usePaneTabs } from '../../../entities/file/queries';
import { usePaneStore, type PaneTab } from '../../../features/pane/paneStore';
import { isRemotePath } from '../../../features/connections/helpers';
import { isTrashPath } from '../../../shared/lib/trash';
import { FileService } from '../../../shared/api/bindings';

let tabSeq = 0;
const makeTab = (path: string): PaneTab => ({ id: `boot${++tabSeq}`, path, back: [], forward: [] });

/** Validate one saved tab path. Remote tabs are restored dormant (no dial),
 * so they always pass — connecting is a manual action via the cloud menu. */
const isValidTab = async (path: string): Promise<boolean> => {
  if (!path) return false;
  if (isRemotePath(path)) return true;
  if (isTrashPath(path)) return true;
  try {
    return await FileService.Exists(path);
  } catch {
    return false;
  }
};

/** Load saved tabs (or home) for both panes once, then mark the store ready. */
export const useInitPaneTabs = () => {
  const ready = usePaneStore((s) => s.ready);
  const hydrateTabs = usePaneStore((s) => s.hydrateTabs);
  const { data: home, isLoading: homeLoading } = useHomeDir();
  const { data: tabs, isLoading: tabsLoading } = usePaneTabs();

  useEffect(() => {
    if (ready || homeLoading || tabsLoading || !home) return;

    const init = async () => {
      const buildPane = async (saved: { path: string }[], activeIdx: number) => {
        const paths = saved.map((t) => t.path);
        const valid = await Promise.all(paths.map(isValidTab));
        const kept = paths.filter((_, i) => valid[i]);
        if (!kept.length) kept.push(home);
        const paneTabs = kept.map(makeTab);
        const active = Math.min(Math.max(activeIdx, 0), paneTabs.length - 1);
        return { paneTabs, active };
      };

      const [left, right] = await Promise.all([
        buildPane(tabs?.left ?? [], tabs?.leftActive ?? 0),
        buildPane(tabs?.right ?? [], tabs?.rightActive ?? 0),
      ]);
      hydrateTabs(left.paneTabs, left.active, right.paneTabs, right.active);
    };
    void init();
  }, [ready, home, tabs, homeLoading, tabsLoading, hydrateTabs]);

  return ready;
};
