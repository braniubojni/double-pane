import { fireEvent, render, screen } from '@testing-library/react';
import type { ReactNode } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { CommandPalette } from './CommandPalette';

const run = vi.hoisted(() => vi.fn());

vi.mock('../../entities/file/queries', () => ({
  useShortcutDefs: () => ({ data: defsData }),
  usePatchSettings: () => vi.fn(),
  useSettings: () => ({ data: {} }),
}));
vi.mock('../../features/command-palette/runShortcutAction', () => ({
  runShortcutAction: run,
  shortcutToggles: () => ({}),
}));
vi.mock('@mui/material/Dialog', () => ({
  default: ({ children, open }: { children: ReactNode; open: boolean }) =>
    open ? <div>{children}</div> : null,
}));

const cmd = (id: string, label: string) => ({ id, label, description: '', binding: '' });
const defsData = [cmd('a', 'Alpha'), cmd('b', 'Beta'), cmd('c', 'Cherry')];

const field = () => screen.getByRole('textbox') as HTMLInputElement;
const press = (key: string) => fireEvent.keyDown(field(), { key });

beforeEach(() => {
  run.mockClear();
  Element.prototype.scrollIntoView = vi.fn();
});

describe('CommandPalette selection', () => {
  it('arrows move selection, clamped, Enter runs it and closes', () => {
    const onClose = vi.fn();
    render(<CommandPalette open onClose={onClose} />);
    press('ArrowUp');
    for (let i = 0; i < 5; i++) press('ArrowDown');
    press('Enter');
    expect(run).toHaveBeenCalledWith('c', expect.anything());
    expect(onClose).toHaveBeenCalled();
  });

  it('typing filters and resets selection to first match', () => {
    render(<CommandPalette open onClose={vi.fn()} />);
    press('ArrowDown');
    press('ArrowDown');
    fireEvent.change(field(), { target: { value: 'e' } });
    press('Enter');
    expect(run).toHaveBeenCalledWith('b', expect.anything());
  });

  it('closing clears query and selection', () => {
    const view = render(<CommandPalette open onClose={vi.fn()} />);
    fireEvent.change(field(), { target: { value: 'be' } });
    view.rerender(<CommandPalette open={false} onClose={vi.fn()} />);
    view.rerender(<CommandPalette open onClose={vi.fn()} />);
    expect(field().value).toBe('');
    press('Enter');
    expect(run).toHaveBeenCalledWith('a', expect.anything());
  });

  it('shows no matches for unknown query', () => {
    render(<CommandPalette open onClose={vi.fn()} />);
    fireEvent.change(field(), { target: { value: 'zzz' } });
    expect(screen.getByText('No matches')).toBeTruthy();
  });
});
