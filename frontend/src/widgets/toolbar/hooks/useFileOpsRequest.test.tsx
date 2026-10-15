import { act, renderHook } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { useFileOpsStore } from '../../../features/file-ops/fileOpsStore';
import { useFileOpsRequest } from './useFileOpsRequest';

const run = vi.hoisted(() => vi.fn());
vi.mock('../helpers', () => ({ runToolbarRequest: run }));

const handlers = (tag: string) => ({ tag }) as never;

beforeEach(() => {
  run.mockClear();
  useFileOpsStore.setState({ request: null, nonce: 0 });
});

describe('useFileOpsRequest', () => {
  it('runs the handler for a fired request and consumes it', () => {
    const h = handlers('a');
    renderHook(() => useFileOpsRequest(h));
    act(() => useFileOpsStore.getState().trigger('copy' as never));
    expect(run).toHaveBeenCalledWith('copy', h);
    expect(useFileOpsStore.getState().request).toBeNull();
  });

  it('uses the latest handlers without re-running', () => {
    const { rerender } = renderHook(({ h }) => useFileOpsRequest(h), {
      initialProps: { h: handlers('a') },
    });
    const b = handlers('b');
    rerender({ h: b });
    expect(run).not.toHaveBeenCalled();
    act(() => useFileOpsStore.getState().trigger('copy' as never));
    expect(run).toHaveBeenCalledWith('copy', b);
  });

  it('ignores nonce changes with no request', () => {
    renderHook(() => useFileOpsRequest(handlers('a')));
    act(() => useFileOpsStore.setState({ nonce: 5 }));
    expect(run).not.toHaveBeenCalled();
  });
});
