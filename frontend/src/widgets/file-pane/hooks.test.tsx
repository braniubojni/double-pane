import type { GridRowParams } from '@mui/x-data-grid/models';
import { act, renderHook } from '@testing-library/react';
import type { KeyboardEvent } from 'react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { FileEntry } from '../../entities/file/types';
import type { FileTableProps } from './types';
import { useFileTable } from './hooks';

vi.mock('react-dnd', () => ({
  useDrop: () => [{ isOver: false, isOverShallow: false, canDropPane: false }, vi.fn()],
}));

vi.mock('../../features/transfers/useDestRowUpdates', () => ({
  useDestRowUpdates: () => undefined,
}));

const file = (name: string): FileEntry => ({
  name,
  path: `/work/${name}`,
  isDir: false,
  size: 1,
  modTime: 0,
  ext: '',
  isSymlink: false,
  access: '',
});

const onFocus = vi.fn();
const onOpen = vi.fn();

const props = (entries: FileEntry[], patch: Partial<FileTableProps> = {}): FileTableProps => ({
  paneId: 'left',
  panePath: '/work',
  entries,
  isLoading: false,
  isError: false,
  selected: [],
  focused: '',
  active: true,
  showExtensions: true,
  gitByName: undefined,
  folderSizes: {},
  deniedPaths: new Set<string>(),
  onSelect: vi.fn(),
  onFocus,
  onToggleMulti: vi.fn(),
  onSelectRange: vi.fn(),
  onActivate: vi.fn(),
  onOpen,
  onDropPaths: vi.fn(),
  ...patch,
});

const keyEvent = (key: string, target: EventTarget = document.body): KeyboardEvent =>
  ({
    key,
    code: key === ' ' ? 'Space' : '',
    shiftKey: false,
    ctrlKey: false,
    metaKey: false,
    altKey: false,
    target,
    preventDefault: vi.fn(),
    stopPropagation: vi.fn(),
  }) as unknown as KeyboardEvent;

beforeEach(() => {
  vi.useFakeTimers();
  onFocus.mockClear();
  onOpen.mockClear();
});

afterEach(() => {
  document.body.replaceChildren();
  vi.useRealTimers();
  vi.restoreAllMocks();
});

describe('useFileTable type-ahead', () => {
  const rows = [file('..'), file('alpha.txt'), file('alpine.txt'), file('beta.txt')];

  it('jumps to the prefix, cycles the same letter, and appends the next letter', () => {
    const { result, rerender } = renderHook((p: FileTableProps) => useFileTable(p), {
      initialProps: props(rows),
    });

    act(() => {
      result.current.handleKeys(keyEvent('a'));
    });
    expect(onFocus).toHaveBeenCalledWith('/work/alpha.txt');

    rerender(props(rows, { focused: '/work/alpha.txt' }));
    act(() => {
      result.current.handleKeys(keyEvent('a'));
    });
    expect(onFocus).toHaveBeenCalledWith('/work/alpine.txt');

    rerender(props(rows, { focused: '/work/alpine.txt' }));
    onFocus.mockClear();
    act(() => {
      result.current.handleKeys(keyEvent('l'));
    });
    expect(onFocus).toHaveBeenCalledWith('/work/alpha.txt');
  });

  it('resets the buffer after 800ms', () => {
    const { result } = renderHook(() => useFileTable(props(rows)));
    act(() => {
      result.current.handleKeys(keyEvent('a'));
    });
    act(() => {
      vi.advanceTimersByTime(800);
    });
    onFocus.mockClear();
    act(() => {
      result.current.handleKeys(keyEvent('b'));
    });
    expect(onFocus).toHaveBeenCalledWith('/work/beta.txt');
  });

  it('restarts the reset timer when another letter arrives', () => {
    const { result, rerender } = renderHook((p: FileTableProps) => useFileTable(p), {
      initialProps: props(rows),
    });
    act(() => {
      result.current.handleKeys(keyEvent('a'));
    });
    act(() => {
      vi.advanceTimersByTime(799);
    });
    rerender(props(rows, { focused: '/work/alpha.txt' }));
    act(() => {
      result.current.handleKeys(keyEvent('l'));
    });
    act(() => {
      vi.advanceTimersByTime(799);
    });
    onFocus.mockClear();
    act(() => {
      result.current.handleKeys(keyEvent('p'));
    });
    expect(onFocus).toHaveBeenCalledWith('/work/alpha.txt');
  });

  it('does not move when nothing matches or the key is a shortcut', () => {
    const { result } = renderHook(() => useFileTable(props(rows)));
    act(() => {
      result.current.handleKeys(keyEvent('z'));
    });
    act(() => {
      result.current.handleKeys({ ...keyEvent('a'), ctrlKey: true } as KeyboardEvent);
    });
    act(() => {
      result.current.handleKeys({ ...keyEvent('a'), metaKey: true } as KeyboardEvent);
    });
    act(() => {
      result.current.handleKeys({ ...keyEvent('a'), altKey: true } as KeyboardEvent);
    });
    expect(onFocus).not.toHaveBeenCalled();
  });

  it('ignores keys while an input inside the pane is focused', () => {
    const input = document.createElement('input');
    const { result } = renderHook(() => useFileTable(props(rows)));
    act(() => {
      result.current.handleKeys(keyEvent('a', input));
    });
    expect(onFocus).not.toHaveBeenCalled();
  });
});

describe('useFileTable focus', () => {
  const mount = (entries = [file('alpha.txt')]) => {
    const view = renderHook((p: FileTableProps) => useFileTable(p), {
      initialProps: props(entries),
    });
    const wrap = document.createElement('div');
    wrap.tabIndex = -1;
    document.body.append(wrap);
    act(() => {
      view.result.current.setWrapRef(wrap);
    });
    view.rerender(props([...entries, file('refocus.txt')]));
    return { ...view, wrap };
  };

  it('focuses the grid when the pane is active and nothing editable is focused', () => {
    const { wrap, rerender } = mount();
    rerender(props([file('alpha.txt'), file('beta.txt')]));
    expect(document.activeElement).toBe(wrap);
  });

  it('does not steal focus from the path bar, a dialog, a menu, or the terminal', () => {
    const cases = [
      () => {
        const bar = document.createElement('div');
        bar.dataset.testid = 'path-input-left';
        const input = document.createElement('input');
        bar.append(input);
        document.body.append(bar);
        input.focus();
      },
      () => {
        const dialog = document.createElement('div');
        dialog.setAttribute('role', 'dialog');
        dialog.tabIndex = -1;
        document.body.append(dialog);
        dialog.focus();
      },
      () => {
        const menu = document.createElement('div');
        menu.setAttribute('role', 'menu');
        menu.tabIndex = -1;
        document.body.append(menu);
        menu.focus();
      },
      () => {
        const term = document.createElement('div');
        term.className = 'xterm';
        term.tabIndex = -1;
        document.body.append(term);
        term.focus();
      },
    ];

    for (const focusOf of cases) {
      document.body.replaceChildren();
      focusOf();
      const held = document.activeElement;
      const { wrap } = mount();
      expect(document.activeElement).toBe(held);
      expect(document.activeElement).not.toBe(wrap);
    }
  });

  it('does not steal focus from an input, textarea, select, or contenteditable', () => {
    const elements: HTMLElement[] = [
      document.createElement('input'),
      document.createElement('textarea'),
      document.createElement('select'),
    ];
    const rich = document.createElement('div');
    Object.defineProperty(rich, 'isContentEditable', { value: true });
    rich.tabIndex = -1;
    elements.push(rich);

    for (const el of elements) {
      document.body.replaceChildren();
      document.body.append(el);
      const { wrap, rerender } = mount();
      el.focus();
      rerender(props([file('alpha.txt'), file('beta.txt'), file('gamma.txt')]));
      expect(document.activeElement).not.toBe(wrap);
    }
  });

  it('blurs a child of the grid and focuses the grid itself', () => {
    const { wrap, rerender } = mount();
    const child = document.createElement('button');
    wrap.append(child);
    child.focus();
    rerender(props([file('beta.txt')]));
    expect(document.activeElement).toBe(wrap);
  });

  it('focuses again on the next tick, and skips that tick after unmount', () => {
    const { wrap, unmount } = mount();
    document.body.focus();
    act(() => {
      vi.advanceTimersByTime(0);
    });
    expect(document.activeElement).toBe(wrap);

    const other = document.createElement('button');
    document.body.append(other);
    other.focus();
    unmount();
    act(() => {
      vi.advanceTimersByTime(0);
    });
    expect(document.activeElement).toBe(other);
  });
});

describe('useFileTable double click', () => {
  it('opens the row and clears the native selection', () => {
    const removeAllRanges = vi.fn();
    vi.spyOn(window, 'getSelection').mockReturnValue({ removeAllRanges } as unknown as Selection);
    const entry = file('alpha.txt');
    const { result } = renderHook(() => useFileTable(props([entry])));
    act(() => {
      result.current.onRowDoubleClick({ row: entry } as GridRowParams);
    });
    expect(onOpen).toHaveBeenCalledWith(entry);
    expect(removeAllRanges).toHaveBeenCalledOnce();
  });

  it('still opens the row when there is no selection', () => {
    vi.spyOn(window, 'getSelection').mockReturnValue(null);
    const entry = file('alpha.txt');
    const { result } = renderHook(() => useFileTable(props([entry])));
    expect(() => {
      act(() => {
        result.current.onRowDoubleClick({ row: entry } as GridRowParams);
      });
    }).not.toThrow();
    expect(onOpen).toHaveBeenCalledWith(entry);
  });
});
