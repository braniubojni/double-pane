import { fireEvent, render, screen } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import ShortcutsDialog from './ShortcutsDialog';

const q = vi.hoisted(() => ({
  data: undefined as unknown,
  mutate: vi.fn(),
}));

vi.mock('../../entities/file/queries', () => ({
  useShortcutDefs: () => ({ data: q.data }),
  useSaveShortcuts: () => ({ mutate: q.mutate, isPending: false }),
}));
vi.mock('../../shared/api/bindings', () => ({ SettingsService: {} }));

const defs = (copy = 'F5') => [
  { id: 'copy', label: 'Copy', description: '', binding: copy },
  { id: 'move', label: 'Move', description: '', binding: 'F6' },
];

const boxes = () => screen.getAllByRole('textbox') as HTMLInputElement[];

beforeEach(() => {
  q.mutate.mockClear();
  q.data = defs();
});

describe('ShortcutsDialog bindings', () => {
  it('fills bindings from defs when opened', () => {
    render(<ShortcutsDialog open onClose={vi.fn()} />);
    expect(boxes().map((b) => b.value)).toEqual(['F5', 'F6']);
  });

  it('saves edited bindings', () => {
    render(<ShortcutsDialog open onClose={vi.fn()} />);
    fireEvent.change(boxes()[0], { target: { value: 'F9' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));
    expect(q.mutate.mock.calls[0][0]).toEqual({ copy: 'F9', move: 'F6' });
  });

  it('discards edits when closed and reopened', () => {
    const view = render(<ShortcutsDialog open onClose={vi.fn()} />);
    fireEvent.change(boxes()[0], { target: { value: 'F9' } });
    view.rerender(<ShortcutsDialog open={false} onClose={vi.fn()} />);
    view.rerender(<ShortcutsDialog open onClose={vi.fn()} />);
    expect(boxes()[0].value).toBe('F5');
  });

  it('resets bindings when fresh defs arrive while open', () => {
    const view = render(<ShortcutsDialog open onClose={vi.fn()} />);
    fireEvent.change(boxes()[0], { target: { value: 'F9' } });
    q.data = defs('F7');
    view.rerender(<ShortcutsDialog open onClose={vi.fn()} />);
    expect(boxes()[0].value).toBe('F7');
  });

  it('keeps edits across unrelated rerenders', () => {
    const view = render(<ShortcutsDialog open onClose={vi.fn()} />);
    fireEvent.change(boxes()[0], { target: { value: 'F9' } });
    view.rerender(<ShortcutsDialog open onClose={vi.fn()} />);
    expect(boxes()[0].value).toBe('F9');
  });
});
