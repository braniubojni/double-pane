import { Events } from '@wailsio/runtime';
import { useEffect } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import type { VaultJobEvent } from '../../entities/file/types';
import { usePaneJobStore } from '../jobs/paneJobStore';
import { useSnack } from '../../shared/ui/SnackbarHost';

const verbForKind = (kind: VaultJobEvent['kind']): string => {
  if (kind === 'create-file') return 'Locking file';
  if (kind === 'remove') return 'Removing password';
  return 'Locking';
};

const percentOf = (done: number, total: number): number =>
  total > 0 ? Math.min(100, Math.round((done / total) * 100)) : 0;

/** Subscribe once to backend vault job progress + auto-lock notifications. */
export const useVaultEvents = (): void => {
  const qc = useQueryClient();
  const show = useSnack((s) => s.show);

  useEffect(() => {
    const unsubProgress = Events.On('vault:progress', (ev: { data?: VaultJobEvent }) => {
      const e = (ev?.data ?? ev) as VaultJobEvent;
      if (!e?.jobId) return;

      if (e.done) {
        // Whichever pane started this job holds it; finish() no-ops on the other.
        usePaneJobStore.getState().finish('left', e.jobId);
        usePaneJobStore.getState().finish('right', e.jobId);
        void qc.invalidateQueries({ queryKey: ['unlockedVaults'] });
        void qc.invalidateQueries({ queryKey: ['dir'] });
        if (e.err) {
          show(e.err, 'error');
        } else {
          show(e.kind === 'remove' ? 'Password removed' : 'Locked with password', 'success');
        }
        return;
      }

      const pct = percentOf(e.bytesDone || e.filesDone, e.bytesTotal || e.filesTotal);
      usePaneJobStore.getState().updateLabel(e.jobId, `${verbForKind(e.kind)}… ${pct}%`);
    });

    const unsubLocked = Events.On('vault:locked', (ev: { data?: string[] } | string[]) => {
      const paths = (Array.isArray(ev) ? ev : (ev?.data ?? [])) as string[];
      if (!paths.length) return;
      void qc.invalidateQueries({ queryKey: ['unlockedVaults'] });
      void qc.invalidateQueries({ queryKey: ['dir'] });
      show(
        paths.length === 1 ? 'Vault locked (left both panes)' : `${paths.length} vaults locked`,
        'info',
      );
    });

    return () => {
      unsubProgress();
      unsubLocked();
    };
  }, [qc, show]);
};
