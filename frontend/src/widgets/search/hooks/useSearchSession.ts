import { Events } from '@wailsio/runtime';
import { startTransition, useCallback, useEffect, useRef, useState } from 'react';
import { isMEGAPath } from '../../../features/connections/helpers';
import { FileService, SettingsService } from '../../../shared/api/bindings';
import { errMessage } from '../../../shared/lib/format';
import { useSnack } from '../../../shared/ui/SnackbarHost';
import { includePatternsForSearch } from '../helpers';
import type { ContentHit, SearchPrefs, SearchResult } from '../types';

type HitPayload = {
  jobId?: string;
  mode?: string;
  content?: ContentHit | null;
  folder?: { name: string; path: string; isDir: boolean; relPath: string } | null;
};

type DonePayload = {
  jobId?: string;
  truncated?: boolean;
};

type DeniedPayload = { jobId?: string; path?: string };
type ErrorPayload = { jobId?: string; error?: string };

export const useSearchSession = (open: boolean, root: string, showHidden: boolean) => {
  const show = useSnack((s) => s.show);
  const [results, setResults] = useState<SearchResult[]>([]);
  const [searching, setSearching] = useState(false);
  const [truncated, setTruncated] = useState(false);
  const [denied, setDenied] = useState<string[]>([]);
  const jobIdRef = useRef<string | null>(null);
  const prefsRef = useRef<SearchPrefs | null>(null);

  const setPrefsRef = useCallback((p: SearchPrefs) => {
    prefsRef.current = p;
  }, []);

  const cancelJob = useCallback(() => {
    const id = jobIdRef.current;
    if (id) {
      void FileService.CancelJob(id).catch(() => undefined);
      jobIdRef.current = null;
    }
  }, []);

  const runSearch = useCallback(async () => {
    const p = prefsRef.current;
    if (!p) return;
    if (isMEGAPath(root) && p.mode !== 'folders') {
      show('Content search is not available on MEGA connections yet', 'error');
      return;
    }
    if (p.mode === 'content' && !p.query.trim()) {
      setResults([]);
      return;
    }
    cancelJob();
    setResults([]);
    setDenied([]);
    setTruncated(false);
    setSearching(true);

    const includeGlobs = includePatternsForSearch(p.include, root);
    void SettingsService.AddSearchHistory('query', p.query);
    void SettingsService.AddSearchHistory('include', includeGlobs || p.include);
    void SettingsService.AddSearchHistory('exclude', p.exclude);

    try {
      const jobId = await FileService.NewJobID();
      jobIdRef.current = jobId;
      await FileService.StartSearch(
        jobId,
        root,
        p.query,
        p.mode,
        includeGlobs,
        p.exclude,
        p.caseSensitive,
        showHidden,
        p.mode === 'folders' ? 500 : 2000,
      );
    } catch (e) {
      setSearching(false);
      show(errMessage(e), 'error');
    }
  }, [cancelJob, root, show, showHidden]);

  useEffect(() => {
    if (!open) return;

    // Coalesce rapid search:hit events so React is not re-rendering per hit.
    let pending: SearchResult[] = [];
    let flushTimer = 0;
    const flushHits = () => {
      flushTimer = 0;
      if (pending.length === 0) return;
      const batch = pending;
      pending = [];
      startTransition(() => {
        setResults((r) => (r.length === 0 ? batch : r.concat(batch)));
      });
    };
    const enqueueHit = (hit: SearchResult) => {
      pending.push(hit);
      if (!flushTimer) {
        flushTimer = window.setTimeout(flushHits, 32);
      }
    };

    const unsubHit = Events.On('search:hit', (ev: { data?: HitPayload }) => {
      const payload = (ev?.data ?? ev) as HitPayload;
      if (!payload?.jobId || payload.jobId !== jobIdRef.current) return;
      if (payload.mode === 'folders' && payload.folder) {
        enqueueHit({ kind: 'folder', hit: payload.folder });
      } else if (payload.content) {
        enqueueHit({ kind: 'content', hit: payload.content as ContentHit });
      }
    });
    const unsubDenied = Events.On('search:denied', (ev: { data?: DeniedPayload }) => {
      const payload = (ev?.data ?? ev) as DeniedPayload;
      if (!payload?.jobId || payload.jobId !== jobIdRef.current || !payload.path) return;
      startTransition(() => {
        setDenied((d) => (d.includes(payload.path!) ? d : [...d, payload.path!]));
      });
    });
    const unsubDone = Events.On('search:done', (ev: { data?: DonePayload }) => {
      const payload = (ev?.data ?? ev) as DonePayload;
      if (!payload?.jobId || payload.jobId !== jobIdRef.current) return;
      if (flushTimer) {
        window.clearTimeout(flushTimer);
        flushHits();
      }
      setSearching(false);
      setTruncated(Boolean(payload.truncated));
      jobIdRef.current = null;
    });
    const unsubErr = Events.On('search:error', (ev: { data?: ErrorPayload }) => {
      const payload = (ev?.data ?? ev) as ErrorPayload;
      if (!payload?.jobId || payload.jobId !== jobIdRef.current) return;
      if (flushTimer) {
        window.clearTimeout(flushTimer);
        flushHits();
      }
      setSearching(false);
      jobIdRef.current = null;
      show(payload.error || 'Search failed', 'error');
    });

    return () => {
      if (flushTimer) window.clearTimeout(flushTimer);
      if (typeof unsubHit === 'function') unsubHit();
      if (typeof unsubDenied === 'function') unsubDenied();
      if (typeof unsubDone === 'function') unsubDone();
      if (typeof unsubErr === 'function') unsubErr();
    };
  }, [open, show]);

  useEffect(() => {
    if (!open) {
      cancelJob();
      setResults([]);
      setSearching(false);
      setDenied([]);
    }
  }, [open, cancelJob]);

  return {
    results,
    setResults,
    searching,
    truncated,
    denied,
    setDenied,
    runSearch,
    cancelJob,
    setPrefsRef,
  };
};
