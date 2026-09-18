import { Events } from '@wailsio/runtime';
import { useEffect } from 'react';
import { useDuplicatesStore } from './duplicatesStore';
import { unwrapDupEvent } from './helpers';
import type { DupDonePayload, DupErrorPayload, DupGroupPayload, DupProgressPayload } from './types';

const jobMatches = (jobId?: string): boolean =>
  Boolean(jobId) && jobId === useDuplicatesStore.getState().jobId;

/** Subscribe once to dup:* events; closing the dialog does not unsubscribe. */
export const useDuplicateEvents = (): void => {
  useEffect(() => {
    const unsubProgress = Events.On('dup:progress', (ev: { data?: DupProgressPayload }) => {
      const payload = unwrapDupEvent<DupProgressPayload>(ev);
      if (!jobMatches(payload?.jobId)) return;
      useDuplicatesStore.getState().applyProgress(payload);
    });
    const unsubGroup = Events.On('dup:group', (ev: { data?: DupGroupPayload }) => {
      const payload = unwrapDupEvent<DupGroupPayload>(ev);
      if (!jobMatches(payload?.jobId) || !payload.group) return;
      useDuplicatesStore.getState().addGroup(payload.group);
    });
    const unsubError = Events.On('dup:error', (ev: { data?: DupErrorPayload }) => {
      const payload = unwrapDupEvent<DupErrorPayload>(ev);
      if (!jobMatches(payload?.jobId)) return;
      if (payload.fatal) {
        useDuplicatesStore.getState().fail(payload.message);
        return;
      }
      if (payload.path) {
        useDuplicatesStore.getState().addSkipped(payload.path, payload.message || 'skipped');
      }
    });
    const unsubDone = Events.On('dup:done', (ev: { data?: DupDonePayload }) => {
      const payload = unwrapDupEvent<DupDonePayload>(ev);
      if (!jobMatches(payload?.jobId)) return;
      useDuplicatesStore.getState().finish(payload.error, payload.groups);
    });

    return () => {
      if (typeof unsubProgress === 'function') unsubProgress();
      if (typeof unsubGroup === 'function') unsubGroup();
      if (typeof unsubError === 'function') unsubError();
      if (typeof unsubDone === 'function') unsubDone();
    };
  }, []);
};
