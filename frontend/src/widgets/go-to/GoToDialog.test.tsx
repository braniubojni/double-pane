import { act, fireEvent, render, screen } from '@testing-library/react';
import type { ReactNode } from 'react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { usePaneStore } from '../../features/pane/paneStore';
import { GoToDialog } from './GoToDialog';

const search = vi.hoisted(() =>
  vi.fn((_root?: string, _query?: string, _hidden?: boolean, _enabled?: boolean) => ({
    data: [] as never[],
    isFetching: false,
  })),
);

vi.mock('../../entities/file/queries', () => ({
  useSettings: () => ({ data: { showHidden: false, useBuiltInEditor: true } }),
  useSearchTree: (root: string | undefined, query: string, showHidden: boolean, enabled: boolean) =>
    search(root, query, showHidden, enabled),
}));

vi.mock('@mui/material/Dialog', () => ({
  default: ({ children, open }: { children: ReactNode; open: boolean }) =>
    open ? <div>{children}</div> : null,
}));

beforeEach(() => {
  vi.useFakeTimers();
  search.mockClear();
  usePaneStore.getState().setPath('left', '/work');
  usePaneStore.setState({ activePane: 'left' });
});

afterEach(() => {
  vi.useRealTimers();
});

describe('GoToDialog query debounce', () => {
  it('searches with the typed query after 180ms and not before', () => {
    render(<GoToDialog open onClose={vi.fn()} />);
    search.mockClear();
    fireEvent.change(screen.getByRole('textbox'), { target: { value: 'main' } });

    act(() => {
      vi.advanceTimersByTime(179);
    });
    expect(search).not.toHaveBeenCalledWith('/work', 'main', false, true);

    act(() => {
      vi.advanceTimersByTime(1);
    });
    expect(search).toHaveBeenCalledWith('/work', 'main', false, true);
  });

  it('restarts the timer when the query changes again', () => {
    render(<GoToDialog open onClose={vi.fn()} />);
    const field = screen.getByRole('textbox');
    fireEvent.change(field, { target: { value: 'm' } });
    act(() => {
      vi.advanceTimersByTime(100);
    });
    fireEvent.change(field, { target: { value: 'ma' } });
    act(() => {
      vi.advanceTimersByTime(179);
    });
    expect(search).not.toHaveBeenCalledWith('/work', 'ma', false, true);
    act(() => {
      vi.advanceTimersByTime(1);
    });
    expect(search).toHaveBeenCalledWith('/work', 'ma', false, true);
  });

  it('clears the query immediately when the dialog closes', () => {
    const view = render(<GoToDialog open onClose={vi.fn()} />);
    fireEvent.change(screen.getByRole('textbox'), { target: { value: 'main' } });
    act(() => {
      vi.advanceTimersByTime(180);
    });
    search.mockClear();
    view.rerender(<GoToDialog open={false} onClose={vi.fn()} />);
    expect(search).toHaveBeenCalledWith('/work', '', false, false);
  });
});

const hit = (name: string) => ({ path: `/work/${name}`, name, relPath: name, isDir: true });
const rootHits = [hit('a'), hit('b'), hit('c')];
const xHits = [hit('x1'), hit('x2')];

describe('GoToDialog selection index', () => {
  beforeEach(() => {
    search.mockImplementation((_r, query) => ({
      data: (query === 'x' ? xHits : rootHits) as never[],
      isFetching: false,
    }));
  });

  const enter = (onClose = vi.fn()) => {
    fireEvent.keyDown(screen.getByRole('textbox'), { key: 'Enter' });
    expect(onClose).toHaveBeenCalled();
    return usePaneStore.getState().getPath('left');
  };

  it('arrows move selection, clamped to the list, Enter opens it', () => {
    const onClose = vi.fn();
    render(<GoToDialog open onClose={onClose} />);
    const field = screen.getByRole('textbox');
    fireEvent.keyDown(field, { key: 'ArrowUp' });
    for (let i = 0; i < 5; i++) fireEvent.keyDown(field, { key: 'ArrowDown' });
    expect(enter(onClose)).toBe('/work/c');
  });

  it('resets selection to first row when debounced query changes hits', () => {
    const onClose = vi.fn();
    render(<GoToDialog open onClose={onClose} />);
    const field = screen.getByRole('textbox');
    fireEvent.keyDown(field, { key: 'ArrowDown' });
    fireEvent.change(field, { target: { value: 'x' } });
    act(() => {
      vi.advanceTimersByTime(180);
    });
    expect(enter(onClose)).toBe('/work/x1');
  });

  it('reopening starts with empty query and first row selected', () => {
    const onClose = vi.fn();
    const view = render(<GoToDialog open onClose={onClose} />);
    fireEvent.change(screen.getByRole('textbox'), { target: { value: 'x' } });
    act(() => {
      vi.advanceTimersByTime(180);
    });
    fireEvent.keyDown(screen.getByRole('textbox'), { key: 'ArrowDown' });
    view.rerender(<GoToDialog open={false} onClose={onClose} />);
    view.rerender(<GoToDialog open onClose={onClose} />);
    expect((screen.getByRole('textbox') as HTMLInputElement).value).toBe('');
    expect(enter(onClose)).toBe('/work/a');
  });
});
