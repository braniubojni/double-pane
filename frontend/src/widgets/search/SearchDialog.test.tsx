import { act, fireEvent, render, screen } from '@testing-library/react';
import type { ReactNode } from 'react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { usePaneStore } from '../../features/pane/paneStore';
import { FileService, SettingsService } from '../../shared/api/bindings';
import type { SearchResult } from './types';
import { SearchDialog } from './SearchDialog';

const session = vi.hoisted(() => ({
  results: [] as SearchResult[],
  setResults: vi.fn(),
  searching: false,
  denied: [] as string[],
  setDenied: vi.fn(),
  runSearch: vi.fn(),
  setPrefsRef: vi.fn(),
}));

vi.mock('./hooks/useSearchSession', () => ({
  useSearchSession: () => session,
}));

vi.mock('../../entities/file/queries', () => ({
  useSettings: () => ({ data: { showHidden: false, useBuiltInEditor: true } }),
}));

vi.mock('../../shared/api/bindings', () => ({
  SettingsService: {
    GetSearchPrefs: vi.fn(),
    SaveSearchPrefs: vi.fn().mockResolvedValue(undefined),
    ListSearchHistory: vi.fn().mockResolvedValue([]),
    AddSearchHistory: vi.fn().mockResolvedValue(undefined),
  },
  FileService: {
    ReplaceAllInPaths: vi.fn().mockResolvedValue({ replacements: 1, filesChanged: 1 }),
    ReplaceOccurrence: vi.fn(),
  },
}));

vi.mock('@mui/material/Dialog', () => ({
  default: ({ children, open }: { children: ReactNode; open: boolean }) =>
    open ? <div>{children}</div> : null,
}));

const hit = (path: string, line: number): SearchResult => ({
  kind: 'content',
  hit: {
    path,
    relPath: path,
    line,
    column: 1,
    lineText: 'match',
    matchStart: 0,
    matchEnd: 5,
  },
});

const prefs = (patch: Record<string, unknown> = {}) => ({
  query: 'find',
  replace: 'repl',
  include: '/work',
  exclude: '',
  mode: 'content',
  replaceOpen: true,
  caseSensitive: false,
  ...patch,
});

beforeEach(() => {
  vi.useFakeTimers();
  session.results = [];
  session.searching = false;
  session.setResults.mockClear();
  session.runSearch.mockClear();
  vi.mocked(SettingsService.GetSearchPrefs).mockResolvedValue(prefs());
  vi.mocked(SettingsService.SaveSearchPrefs).mockClear();
  vi.mocked(FileService.ReplaceAllInPaths).mockClear();
  usePaneStore.getState().setPath('left', '/work');
  usePaneStore.setState({ activePane: 'left' });
});

afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

const open = async () => {
  render(<SearchDialog open onClose={vi.fn()} />);
  await act(async () => {
    await Promise.resolve();
  });
};

describe('SearchDialog timers', () => {
  it('saves prefs 400ms after they load and resets the timer on a later edit', async () => {
    await open();
    await act(async () => {
      vi.advanceTimersByTime(399);
    });
    expect(SettingsService.SaveSearchPrefs).not.toHaveBeenCalled();
    await act(async () => {
      vi.advanceTimersByTime(1);
    });
    expect(SettingsService.SaveSearchPrefs).toHaveBeenCalledOnce();

    fireEvent.change(screen.getByTestId('input-search-query'), { target: { value: 'next' } });
    await act(async () => {
      vi.advanceTimersByTime(399);
    });
    expect(SettingsService.SaveSearchPrefs).toHaveBeenCalledOnce();
    await act(async () => {
      vi.advanceTimersByTime(1);
    });
    expect(SettingsService.SaveSearchPrefs).toHaveBeenCalledTimes(2);
  });

  it('runs the search 320ms after a non-empty query loads', async () => {
    await open();
    await act(async () => {
      vi.advanceTimersByTime(319);
    });
    expect(session.runSearch).not.toHaveBeenCalled();
    await act(async () => {
      vi.advanceTimersByTime(1);
    });
    expect(session.runSearch).toHaveBeenCalledOnce();
  });

  it('clears results and does not search when content query is empty', async () => {
    vi.mocked(SettingsService.GetSearchPrefs).mockResolvedValue(prefs({ query: '   ' }));
    await open();
    await act(async () => {
      vi.advanceTimersByTime(400);
    });
    expect(session.setResults).toHaveBeenCalledWith([]);
    expect(session.runSearch).not.toHaveBeenCalled();
  });

  it('does not save or search after the dialog closes before the timers fire', async () => {
    const view = render(<SearchDialog open onClose={vi.fn()} />);
    await act(async () => {
      await Promise.resolve();
    });
    view.rerender(<SearchDialog open={false} onClose={vi.fn()} />);
    await act(async () => {
      vi.advanceTimersByTime(500);
    });
    expect(SettingsService.SaveSearchPrefs).not.toHaveBeenCalled();
    expect(session.runSearch).not.toHaveBeenCalled();
  });
});

describe('SearchDialog file names', () => {
  it('shows the file-name mode and keeps replace disabled', async () => {
    vi.mocked(SettingsService.GetSearchPrefs).mockResolvedValue(
      prefs({ mode: 'files', replaceOpen: true }),
    );
    await open();
    expect((screen.getByLabelText('File names') as HTMLInputElement).checked).toBe(true);
    expect(screen.getByTestId('input-search-query').getAttribute('placeholder')).toBe('File name…');
    expect((screen.getByTestId('search-toggle-replace') as HTMLButtonElement).disabled).toBe(true);
    expect(screen.queryByTestId('input-search-replace')).toBeNull();
  });
});

describe('SearchDialog replace all', () => {
  it('asks before replacing more than 50 matches and skips the call when declined', async () => {
    session.results = Array.from({ length: 51 }, (_, i) => hit(i < 40 ? '/a.ts' : '/b.ts', i + 1));
    const confirm = vi.fn(() => false);
    vi.stubGlobal('confirm', confirm);
    await open();
    fireEvent.click(screen.getByTestId('btn-search-replace-all'));
    expect(confirm).toHaveBeenCalledWith('Replace all 51 matches in 2 files?');
    expect(FileService.ReplaceAllInPaths).not.toHaveBeenCalled();
  });

  it('replaces every unique path when the confirm is accepted', async () => {
    session.results = Array.from({ length: 51 }, (_, i) => hit(i < 40 ? '/a.ts' : '/b.ts', i + 1));
    vi.stubGlobal('confirm', () => true);
    await open();
    fireEvent.click(screen.getByTestId('btn-search-replace-all'));
    await act(async () => {
      await Promise.resolve();
    });
    expect(FileService.ReplaceAllInPaths).toHaveBeenCalledWith(
      ['/a.ts', '/b.ts'],
      'find',
      'repl',
      false,
    );
  });

  it('replaces 50 or fewer matches without a confirm', async () => {
    session.results = Array.from({ length: 50 }, (_, i) => hit('/a.ts', i + 1));
    const confirm = vi.fn(() => false);
    vi.stubGlobal('confirm', confirm);
    await open();
    fireEvent.click(screen.getByTestId('btn-search-replace-all'));
    await act(async () => {
      await Promise.resolve();
    });
    expect(confirm).not.toHaveBeenCalled();
    expect(FileService.ReplaceAllInPaths).toHaveBeenCalledWith(['/a.ts'], 'find', 'repl', false);
  });

  it('does not replace when the query is blank', async () => {
    vi.mocked(SettingsService.GetSearchPrefs).mockResolvedValue(prefs({ query: '' }));
    session.results = [hit('/a.ts', 1)];
    const confirm = vi.fn(() => true);
    vi.stubGlobal('confirm', confirm);
    await open();
    fireEvent.click(screen.getByTestId('btn-search-replace-all'));
    expect(confirm).not.toHaveBeenCalled();
    expect(FileService.ReplaceAllInPaths).not.toHaveBeenCalled();
  });

  it('hides replace when the mode is folders', async () => {
    vi.mocked(SettingsService.GetSearchPrefs).mockResolvedValue(
      prefs({ mode: 'folders', replaceOpen: true }),
    );
    await open();
    expect(screen.queryByTestId('btn-search-replace-all')).toBeNull();
  });
});
