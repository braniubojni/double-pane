import { useTheme } from '@mui/material/styles';
import { act, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { ThemePreference } from '../entities/file/types';
import { AppThemeProvider } from './theme';

const settings = vi.hoisted(() => ({ theme: 'system' as ThemePreference }));

vi.mock('../entities/file/queries', () => ({
  useSettings: () => ({ data: { theme: settings.theme } }),
}));

const Probe = () => {
  const theme = useTheme();
  return <span data-testid="mode">{theme.palette.mode}</span>;
};

type Media = {
  matches: boolean;
  addEventListener: (type: string, cb: (event: { matches: boolean }) => void) => void;
  removeEventListener: (type: string, cb: (event: { matches: boolean }) => void) => void;
  emit: (matches: boolean) => void;
};

const installMedia = (matches: boolean): Media => {
  const listeners = new Set<(event: { matches: boolean }) => void>();
  const media: Media = {
    matches,
    addEventListener: (_type, cb) => {
      listeners.add(cb);
    },
    removeEventListener: (_type, cb) => {
      listeners.delete(cb);
    },
    emit: (next) => {
      media.matches = next;
      listeners.forEach((cb) => cb({ matches: next }));
    },
  };
  vi.stubGlobal(
    'matchMedia',
    vi.fn(() => media),
  );
  return media;
};

afterEach(() => {
  settings.theme = 'system';
  vi.unstubAllGlobals();
});

describe('AppThemeProvider system theme', () => {
  it('follows prefers-color-scheme when the preference is system', () => {
    const media = installMedia(true);
    const view = render(
      <AppThemeProvider>
        <Probe />
      </AppThemeProvider>,
    );
    expect(screen.getByTestId('mode').textContent).toBe('dark');

    act(() => {
      media.emit(false);
    });
    expect(screen.getByTestId('mode').textContent).toBe('light');

    view.unmount();
    media.emit(true);
    expect(screen.queryByTestId('mode')).toBeNull();
  });

  it('starts light when the system scheme is light', () => {
    installMedia(false);
    render(
      <AppThemeProvider>
        <Probe />
      </AppThemeProvider>,
    );
    expect(screen.getByTestId('mode').textContent).toBe('light');
  });

  it('keeps an explicit light or dark preference', () => {
    const media = installMedia(true);
    settings.theme = 'light';
    const view = render(
      <AppThemeProvider>
        <Probe />
      </AppThemeProvider>,
    );
    expect(screen.getByTestId('mode').textContent).toBe('light');
    act(() => {
      media.emit(false);
    });
    expect(screen.getByTestId('mode').textContent).toBe('light');

    settings.theme = 'dark';
    view.rerender(
      <AppThemeProvider>
        <Probe />
      </AppThemeProvider>,
    );
    expect(screen.getByTestId('mode').textContent).toBe('dark');
    act(() => {
      media.emit(false);
    });
    expect(screen.getByTestId('mode').textContent).toBe('dark');
  });
});
