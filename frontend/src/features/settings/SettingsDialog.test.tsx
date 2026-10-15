import { fireEvent, render, screen } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { AppSettings } from '../../entities/file/types';
import SettingsDialog from './SettingsDialog';

const q = vi.hoisted(() => ({
  data: undefined as unknown,
  mutate: vi.fn(),
}));

vi.mock('../../entities/file/queries', () => ({
  useSettings: () => ({ data: q.data }),
  useSaveSettings: () => ({ mutate: q.mutate, isPending: false }),
}));
vi.mock('../../shared/api/bindings', () => ({ SettingsService: {} }));
vi.mock('./UpdatesSection', () => ({ UpdatesSection: () => null }));

const settings = (over: Partial<AppSettings> = {}) =>
  ({
    theme: 'system',
    showHidden: false,
    showExtensions: true,
    showGitStatus: true,
    useBuiltInEditor: true,
    vaultIdleLockMinutes: 5,
    ...over,
  }) as AppSettings;

const hidden = () =>
  screen.getByTestId('settings-show-hidden').querySelector('input') as HTMLInputElement;

beforeEach(() => {
  q.mutate.mockClear();
  q.data = settings();
});

describe('SettingsDialog draft', () => {
  it('renders nothing until open with data', () => {
    render(<SettingsDialog open={false} onClose={vi.fn()} />);
    expect(screen.queryByTestId('dialog-settings')).toBeNull();
  });

  it('shows loaded settings when opened', () => {
    render(<SettingsDialog open onClose={vi.fn()} />);
    expect(hidden().checked).toBe(false);
  });

  it('saves the edited draft', () => {
    render(<SettingsDialog open onClose={vi.fn()} />);
    fireEvent.click(hidden());
    fireEvent.click(screen.getByTestId('settings-save'));
    expect(q.mutate.mock.calls[0][0]).toMatchObject({ showHidden: true, theme: 'system' });
  });

  it('discards edits when closed and reopened', () => {
    const view = render(<SettingsDialog open onClose={vi.fn()} />);
    fireEvent.click(hidden());
    expect(hidden().checked).toBe(true);
    view.rerender(<SettingsDialog open={false} onClose={vi.fn()} />);
    view.rerender(<SettingsDialog open onClose={vi.fn()} />);
    expect(hidden().checked).toBe(false);
  });

  it('resets the draft when fresh data arrives while open', () => {
    const view = render(<SettingsDialog open onClose={vi.fn()} />);
    fireEvent.click(hidden());
    q.data = settings({ showHidden: false, theme: 'dark' });
    view.rerender(<SettingsDialog open onClose={vi.fn()} />);
    expect(hidden().checked).toBe(false);
    fireEvent.click(screen.getByTestId('settings-save'));
    expect(q.mutate.mock.calls[0][0]).toMatchObject({ theme: 'dark' });
  });

  it('keeps edits across unrelated rerenders', () => {
    const view = render(<SettingsDialog open onClose={vi.fn()} />);
    fireEvent.click(hidden());
    view.rerender(<SettingsDialog open onClose={vi.fn()} />);
    expect(hidden().checked).toBe(true);
  });
});
