import { renderHook } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { AppSettings } from '../../../entities/file/types';
import { useAutoUpdateCheck } from './useAutoUpdateCheck';

const check = vi.hoisted(() => vi.fn());
const state = vi.hoisted(() => ({
  settings: undefined as AppSettings | undefined,
}));

vi.mock('./useUpdateActions', () => ({
  useUpdateActions: () => ({ check }),
}));

vi.mock('../../../entities/file/queries', () => ({
  useSettings: () => ({ data: state.settings }),
}));

const settings = (patch: Partial<AppSettings>): AppSettings =>
  ({
    autoCheckUpdates: true,
    updateCheckIntervalDays: 10,
    lastUpdateCheckAt: '',
    ...patch,
  }) as AppSettings;

beforeEach(() => {
  vi.useFakeTimers();
  check.mockClear();
  state.settings = undefined;
});

afterEach(() => {
  vi.useRealTimers();
});

describe('useAutoUpdateCheck', () => {
  it('does not check before the app is ready or settings exist', () => {
    const view = renderHook(({ ready }) => useAutoUpdateCheck(ready), {
      initialProps: { ready: false },
    });
    vi.advanceTimersByTime(3000);
    expect(check).not.toHaveBeenCalled();

    state.settings = settings({});
    view.rerender({ ready: false });
    vi.advanceTimersByTime(3000);
    expect(check).not.toHaveBeenCalled();
  });

  it('does not check when the interval has not elapsed', () => {
    state.settings = settings({ lastUpdateCheckAt: new Date().toISOString() });
    renderHook(() => useAutoUpdateCheck(true));
    vi.advanceTimersByTime(3000);
    expect(check).not.toHaveBeenCalled();
  });

  it('does not check when auto-check is off', () => {
    state.settings = settings({ autoCheckUpdates: false, lastUpdateCheckAt: '' });
    renderHook(() => useAutoUpdateCheck(true));
    vi.advanceTimersByTime(3000);
    expect(check).not.toHaveBeenCalled();
  });

  it('checks once, 2.5s after a due settings load', () => {
    state.settings = settings({ lastUpdateCheckAt: '' });
    const view = renderHook(() => useAutoUpdateCheck(true));
    vi.advanceTimersByTime(2499);
    expect(check).not.toHaveBeenCalled();
    vi.advanceTimersByTime(1);
    expect(check).toHaveBeenCalledOnce();

    state.settings = settings({ lastUpdateCheckAt: '' });
    view.rerender();
    vi.advanceTimersByTime(5000);
    expect(check).toHaveBeenCalledOnce();
  });

  it('cancels the delayed check on unmount', () => {
    state.settings = settings({ lastUpdateCheckAt: '' });
    const view = renderHook(() => useAutoUpdateCheck(true));
    view.unmount();
    vi.advanceTimersByTime(3000);
    expect(check).not.toHaveBeenCalled();
  });
});
