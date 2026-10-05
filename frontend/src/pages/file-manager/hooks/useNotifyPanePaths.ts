import { useEffect } from 'react';
import { useNotifyPanePaths as useNotifyPanePathsMutation } from '../../../entities/file/queries';
import { usePaneStore } from '../../../features/pane/paneStore';

/** Tells the backend both panes' current directory after every navigation, so
 * auto-lock-on-leave (spec: vault leaves both panes → lock) has something to
 * compare vault roots against. Debounced like tab persistence — navigation
 * can fire in quick bursts (back/forward, tab switch). */
export const useNotifyPanePaths = (ready: boolean): void => {
  const left = usePaneStore((s) => s.getPath('left'));
  const right = usePaneStore((s) => s.getPath('right'));
  const notify = useNotifyPanePathsMutation();

  useEffect(() => {
    if (!ready) return;
    const t = setTimeout(() => {
      notify.mutate({ left, right });
    }, 300);
    return () => clearTimeout(t);
    // eslint-disable-next-line react-hooks/exhaustive-deps -- only re-fire on path/ready change
  }, [left, right, ready]);
};
