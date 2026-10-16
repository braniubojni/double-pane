import { act, renderHook } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { FileService, SettingsService } from '../../../shared/api/bindings';
import { useSnack } from '../../../shared/ui/SnackbarHost';
import type { ContentHit, SearchPrefs } from '../types';
import { useSearchSession } from './useSearchSession';

const handlers = vi.hoisted(() => ({
  map: new Map<string, (ev: unknown) => void>(),
}));

vi.mock('@wailsio/runtime', () => ({
  Events: {
    On: (name: string, fn: (ev: unknown) => void) => {
      handlers.map.set(name, fn);
      return () => {
        handlers.map.delete(name);
      };
    },
  },
}));

vi.mock('../../../shared/api/bindings', () => ({
  FileService: {
    CancelJob: vi.fn().mockResolvedValue(undefined),
    NewJobID: vi.fn().mockResolvedValue('job-1'),
    StartSearch: vi.fn().mockResolvedValue(undefined),
  },
  SettingsService: {
    AddSearchHistory: vi.fn().mockResolvedValue(undefined),
  },
}));

const prefs = (patch: Partial<SearchPrefs> = {}): SearchPrefs => ({
  query: 'alpha',
  replace: '',
  include: '*.ts',
  exclude: 'dist',
  mode: 'content',
  replaceOpen: false,
  caseSensitive: true,
  ...patch,
});

const content = (path: string): ContentHit => ({
  path,
  relPath: path,
  line: 1,
  column: 1,
  lineText: 'alpha',
  matchStart: 0,
  matchEnd: 5,
});

const emit = (name: string, data: unknown) => {
  act(() => {
    handlers.map.get(name)?.({ data });
  });
};

beforeEach(() => {
  vi.useFakeTimers();
  handlers.map.clear();
  vi.mocked(FileService.NewJobID).mockResolvedValue('job-1');
  vi.mocked(FileService.StartSearch).mockClear();
  vi.mocked(FileService.CancelJob).mockClear();
  useSnack.setState({ open: false, message: '', severity: 'info' });
});

afterEach(() => {
  vi.useRealTimers();
});

describe('useSearchSession hit batching', () => {
  it('flushes hits together after 32ms and ignores other jobs', async () => {
    const { result } = renderHook(() => useSearchSession(true, '/work', false));
    act(() => result.current.setPrefsRef(prefs()));
    await act(async () => {
      await result.current.runSearch();
    });

    emit('search:hit', { jobId: 'other', content: content('/nope.ts') });
    emit('search:hit', { jobId: 'job-1', content: content('/a.ts') });
    emit('search:hit', { jobId: 'job-1', content: content('/b.ts') });
    expect(result.current.results).toEqual([]);

    act(() => {
      vi.advanceTimersByTime(31);
    });
    expect(result.current.results).toEqual([]);
    act(() => {
      vi.advanceTimersByTime(1);
    });
    expect(result.current.results).toEqual([
      { kind: 'content', hit: content('/a.ts') },
      { kind: 'content', hit: content('/b.ts') },
    ]);
  });

  it('records folder hits in folder mode', async () => {
    const { result } = renderHook(() => useSearchSession(true, '/work', true));
    act(() => result.current.setPrefsRef(prefs({ mode: 'folders', query: 'src' })));
    await act(async () => {
      await result.current.runSearch();
    });
    const folder = { name: 'src', path: '/work/src', isDir: true, relPath: 'src' };
    emit('search:hit', { jobId: 'job-1', mode: 'folders', folder });
    act(() => {
      vi.advanceTimersByTime(32);
    });
    expect(result.current.results).toEqual([{ kind: 'folder', hit: folder }]);
  });

  it('searches file names with the folder limit and records the hit', async () => {
    const { result } = renderHook(() => useSearchSession(true, '/work', false));
    act(() => result.current.setPrefsRef(prefs({ mode: 'files', query: 'readme' })));
    await act(async () => {
      await result.current.runSearch();
    });
    expect(FileService.StartSearch).toHaveBeenCalledWith(
      'job-1',
      '/work',
      'readme',
      'files',
      '*.ts',
      'dist',
      true,
      false,
      500,
    );
    const file = {
      name: 'readme.md',
      path: '/work/a/readme.md',
      isDir: false,
      relPath: 'a/readme.md',
    };
    emit('search:hit', { jobId: 'job-1', mode: 'files', folder: file });
    act(() => {
      vi.advanceTimersByTime(32);
    });
    expect(result.current.results).toEqual([{ kind: 'folder', hit: file }]);
  });

  it('flushes a pending batch immediately when the job finishes or fails', async () => {
    const { result } = renderHook(() => useSearchSession(true, '/work', false));
    act(() => result.current.setPrefsRef(prefs()));
    await act(async () => {
      await result.current.runSearch();
    });
    emit('search:hit', { jobId: 'job-1', content: content('/a.ts') });
    emit('search:done', { jobId: 'job-1', truncated: true });
    expect(result.current.searching).toBe(false);
    expect(result.current.truncated).toBe(true);
    expect(result.current.results).toHaveLength(1);

    emit('search:done', { jobId: 'other', truncated: true });
    expect(result.current.truncated).toBe(true);
  });

  it('shows the search error and still flushes hits', async () => {
    const { result } = renderHook(() => useSearchSession(true, '/work', false));
    act(() => result.current.setPrefsRef(prefs()));
    await act(async () => {
      await result.current.runSearch();
    });
    emit('search:hit', { jobId: 'job-1', content: content('/a.ts') });
    emit('search:error', { jobId: 'job-1', error: 'permission denied' });
    expect(result.current.searching).toBe(false);
    expect(result.current.results).toHaveLength(1);
    expect(useSnack.getState().message).toBe('permission denied');
  });

  it('uses a fallback message when the error payload is empty', async () => {
    const { result } = renderHook(() => useSearchSession(true, '/work', false));
    act(() => result.current.setPrefsRef(prefs()));
    await act(async () => {
      await result.current.runSearch();
    });
    emit('search:error', { jobId: 'job-1' });
    expect(useSnack.getState().message).toBe('Search failed');
  });

  it('records denied paths once and ignores a denied event with no path', async () => {
    const { result } = renderHook(() => useSearchSession(true, '/work', false));
    act(() => result.current.setPrefsRef(prefs()));
    await act(async () => {
      await result.current.runSearch();
    });
    emit('search:denied', { jobId: 'job-1' });
    emit('search:denied', { jobId: 'job-1', path: '/secret' });
    emit('search:denied', { jobId: 'job-1', path: '/secret' });
    emit('search:denied', { jobId: 'other', path: '/nope' });
    expect(result.current.denied).toEqual(['/secret']);
  });

  it('does not flush hits that were still pending when the dialog closes', async () => {
    const { result, rerender } = renderHook(({ open }) => useSearchSession(open, '/work', false), {
      initialProps: { open: true },
    });
    act(() => result.current.setPrefsRef(prefs()));
    await act(async () => {
      await result.current.runSearch();
    });
    emit('search:hit', { jobId: 'job-1', content: content('/a.ts') });
    rerender({ open: false });
    act(() => {
      vi.advanceTimersByTime(32);
    });
    expect(result.current.results).toEqual([]);
    expect(FileService.CancelJob).toHaveBeenCalledWith('job-1');
  });
});

describe('useSearchSession close reset', () => {
  it('clears results, denied paths and searching state when the dialog closes', async () => {
    const { result, rerender } = renderHook(({ open }) => useSearchSession(open, '/work', false), {
      initialProps: { open: true },
    });
    act(() => result.current.setPrefsRef(prefs()));
    await act(async () => {
      await result.current.runSearch();
    });
    emit('search:hit', { jobId: 'job-1', content: content('/a.ts') });
    act(() => {
      vi.advanceTimersByTime(32);
    });
    emit('search:denied', { jobId: 'job-1', path: '/secret' });
    expect(result.current.results).toHaveLength(1);
    expect(result.current.denied).toEqual(['/secret']);
    expect(result.current.searching).toBe(true);

    rerender({ open: false });
    expect(result.current.results).toEqual([]);
    expect(result.current.denied).toEqual([]);
    expect(result.current.searching).toBe(false);
    expect(FileService.CancelJob).toHaveBeenCalledWith('job-1');
  });

  it('does not cancel anything when closing with no job running', () => {
    const { rerender } = renderHook(({ open }) => useSearchSession(open, '/work', false), {
      initialProps: { open: true },
    });
    rerender({ open: false });
    expect(FileService.CancelJob).not.toHaveBeenCalled();
  });
});

describe('useSearchSession runSearch', () => {
  it('does not start a content search with an empty query', async () => {
    const { result } = renderHook(() => useSearchSession(true, '/work', false));
    act(() => result.current.setPrefsRef(prefs({ query: '  ' })));
    await act(async () => {
      await result.current.runSearch();
    });
    expect(FileService.StartSearch).not.toHaveBeenCalled();
    expect(result.current.results).toEqual([]);
  });

  it('blocks content search on MEGA and allows folder search', async () => {
    const { result, rerender } = renderHook(({ root }) => useSearchSession(true, root, false), {
      initialProps: { root: 'mega://drive/docs' },
    });
    act(() => result.current.setPrefsRef(prefs()));
    await act(async () => {
      await result.current.runSearch();
    });
    expect(FileService.StartSearch).not.toHaveBeenCalled();
    expect(useSnack.getState().message).toBe(
      'Content search is not available on MEGA connections yet',
    );

    act(() => result.current.setPrefsRef(prefs({ mode: 'folders' })));
    await act(async () => {
      await result.current.runSearch();
    });
    expect(FileService.StartSearch).toHaveBeenCalled();

    vi.mocked(FileService.StartSearch).mockClear();
    act(() => result.current.setPrefsRef(prefs({ mode: 'files', query: 'readme' })));
    await act(async () => {
      await result.current.runSearch();
    });
    expect(FileService.StartSearch).toHaveBeenCalled();
    rerender({ root: 'mega://drive/docs' });
  });

  it('reports a start failure and stops searching', async () => {
    vi.mocked(FileService.NewJobID).mockRejectedValueOnce(new Error('offline'));
    const { result } = renderHook(() => useSearchSession(true, '/work', false));
    act(() => result.current.setPrefsRef(prefs()));
    await act(async () => {
      await result.current.runSearch();
    });
    expect(result.current.searching).toBe(false);
    expect(useSnack.getState().message).toBe('offline');
    expect(SettingsService.AddSearchHistory).toHaveBeenCalled();
  });
});
