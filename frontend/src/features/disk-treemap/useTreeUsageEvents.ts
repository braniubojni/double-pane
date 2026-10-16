import { Events } from '@wailsio/runtime';
import { useEffect } from 'react';
import { useTreemapStore } from './treemapStore';
import type { UsageDone, UsageProgress } from './types';

const unwrap = <T>(ev: { data?: T } | T): T => {
  if (ev && typeof ev === 'object' && 'data' in ev && ev.data) return ev.data;
  return ev as T;
};

/** Subscribe to usage:* for the dialog's current job. */
export const useTreeUsageEvents = (): void => {
  useEffect(() => {
    const unsubProgress = Events.On('usage:progress', (ev: { data?: UsageProgress }) => {
      const payload = unwrap<UsageProgress>(ev);
      if (!payload?.jobId) return;
      useTreemapStore.getState().applyProgress(payload);
    });
    const unsubDone = Events.On('usage:done', (ev: { data?: UsageDone }) => {
      const payload = unwrap<UsageDone>(ev);
      if (!payload?.jobId) return;
      useTreemapStore.getState().applyDone(payload);
    });
    return () => {
      unsubProgress();
      unsubDone();
    };
  }, []);
};
