import { act, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { setDropValidity } from '../../features/dnd/dragState';
import { FileDragLayer } from './FileDragLayer';

const monitor = vi.hoisted(() => ({
  dragging: false,
  item: null as {
    paths: string[];
    sourcePane: 'left';
    primary: { name: string; isDir: boolean };
  } | null,
  offset: null as { x: number; y: number } | null,
}));

vi.mock('react-dnd', () => ({
  useDragLayer: (collect: (m: object) => unknown) =>
    collect({
      isDragging: () => monitor.dragging,
      getItemType: () => (monitor.dragging ? 'FILE_ROWS' : null),
      getItem: () => monitor.item,
      getClientOffset: () => monitor.offset,
    }),
}));

const removeAllRanges = vi.fn();

const payload = {
  paths: ['/tmp/a.txt'],
  sourcePane: 'left' as const,
  primary: { name: 'a.txt', isDir: false },
};

afterEach(() => {
  monitor.dragging = false;
  monitor.item = null;
  monitor.offset = null;
  document.body.className = '';
  setDropValidity(true);
  window.dispatchEvent(new Event('blur'));
  removeAllRanges.mockClear();
});

describe('FileDragLayer drag classes', () => {
  it('adds the dragging class and clears the native selection while a drag is active', () => {
    vi.spyOn(window, 'getSelection').mockReturnValue({ removeAllRanges } as unknown as Selection);
    monitor.dragging = true;
    const view = render(<FileDragLayer />);
    expect(document.body.classList.contains('gfm-file-dragging')).toBe(true);
    expect(document.body.classList.contains('gfm-drop-invalid')).toBe(true);
    expect(removeAllRanges).toHaveBeenCalled();

    act(() => {
      setDropValidity(true);
    });
    expect(document.body.classList.contains('gfm-file-dragging')).toBe(true);
    expect(document.body.classList.contains('gfm-drop-invalid')).toBe(false);

    const calls = removeAllRanges.mock.calls.length;
    monitor.dragging = false;
    view.rerender(<FileDragLayer />);
    expect(document.body.classList.contains('gfm-file-dragging')).toBe(false);
    expect(document.body.classList.contains('gfm-drop-invalid')).toBe(false);
    expect(removeAllRanges.mock.calls.length).toBeGreaterThan(calls);
  });

  it('shows copy, move, and invalid badges', () => {
    Object.defineProperty(navigator, 'userAgent', {
      value: 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)',
      configurable: true,
    });
    monitor.dragging = true;
    monitor.item = payload;
    monitor.offset = { x: 10, y: 12 };
    render(<FileDragLayer />);
    act(() => {
      setDropValidity(true);
    });
    expect(screen.getByText('+')).toBeTruthy();

    act(() => {
      window.dispatchEvent(new KeyboardEvent('keydown', { metaKey: true, key: 'Meta' }));
    });
    expect(screen.getByText('−')).toBeTruthy();

    act(() => {
      setDropValidity(false);
    });
    expect(screen.getByText('⊘')).toBeTruthy();
  });

  it('renders nothing and leaves the body alone when nothing is dragging', () => {
    render(<FileDragLayer />);
    expect(document.body.className).toBe('');
    expect(screen.queryByText('+')).toBeNull();
  });
});
