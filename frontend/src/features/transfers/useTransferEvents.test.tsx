import { renderHook } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useTransferStore } from './transferStore';
import { useTransferEvents } from './useTransferEvents';

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

beforeEach(() => {
  vi.useFakeTimers();
  handlers.map.clear();
  useTransferStore.setState({ byId: {} });
});

afterEach(() => {
  vi.useRealTimers();
});

describe('useTransferEvents', () => {
  it('writes progress for a job and ignores a payload without one', () => {
    const update = vi.spyOn(useTransferStore.getState(), 'updateProgress');
    renderHook(() => useTransferEvents());

    handlers.map.get('transfer:progress')?.({ data: {} });
    handlers.map.get('transfer:progress')?.({
      data: { jobId: 'job-1', bytesDone: 1, bytesTotal: 4 },
    });

    expect(update).toHaveBeenCalledOnce();
    expect(update).toHaveBeenCalledWith({ jobId: 'job-1', bytesDone: 1, bytesTotal: 4 });
  });

  it('removes a finished job after 150ms and ignores a done event without a job', () => {
    const remove = vi.spyOn(useTransferStore.getState(), 'remove');
    renderHook(() => useTransferEvents());

    handlers.map.get('transfer:done')?.({ data: {} });
    handlers.map.get('transfer:done')?.({ data: { jobId: 'job-9' } });
    vi.advanceTimersByTime(149);
    expect(remove).not.toHaveBeenCalled();
    vi.advanceTimersByTime(1);
    expect(remove).toHaveBeenCalledOnce();
    expect(remove).toHaveBeenCalledWith('job-9');
  });

  it('drops the subscriptions on unmount', () => {
    const update = vi.spyOn(useTransferStore.getState(), 'updateProgress');
    const view = renderHook(() => useTransferEvents());
    view.unmount();
    expect(handlers.map.size).toBe(0);
    handlers.map.get('transfer:progress')?.({ data: { jobId: 'late' } });
    expect(update).not.toHaveBeenCalled();
  });
});
