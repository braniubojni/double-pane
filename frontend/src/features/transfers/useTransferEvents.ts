import { Events } from '@wailsio/runtime';
import { useEffect } from 'react';
import { useTransferStore } from './transferStore';
import type { TransferDonePayload, TransferProgressPayload } from './types';

/** Subscribe once to backend transfer progress/done events. */
export const useTransferEvents = (): void => {
  useEffect(() => {
    const updateProgress = useTransferStore.getState().updateProgress;
    const remove = useTransferStore.getState().remove;

    const unsubProgress = Events.On(
      'transfer:progress',
      (ev: { data?: TransferProgressPayload }) => {
        const payload = (ev?.data ?? ev) as TransferProgressPayload;
        if (!payload?.jobId) return;
        updateProgress(payload);
      },
    );

    const unsubDone = Events.On('transfer:done', (ev: { data?: TransferDonePayload }) => {
      const payload = (ev?.data ?? ev) as TransferDonePayload;
      if (!payload?.jobId) return;
      // Keep a brief final frame then clear; startTransfer also removes on settle.
      // remove() itself flips any still-active files to 'canceled' first, so
      // a job that stopped from a real error (not just user cancel) still
      // gets its ghost dest rows cleaned up.
      setTimeout(() => remove(payload.jobId!), 150);
    });

    return () => {
      unsubProgress();
      unsubDone();
    };
  }, []);
};
