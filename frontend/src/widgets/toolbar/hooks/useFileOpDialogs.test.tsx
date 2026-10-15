import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, renderHook } from '@testing-library/react';
import type { ReactNode } from 'react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useFileOpDialogs } from './useFileOpDialogs';

vi.mock('../../../shared/api/bindings', () => ({
  FileService: {},
}));

const wrapper = ({ children }: { children: ReactNode }) => (
  <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
    {children}
  </QueryClientProvider>
);

const renderDialogs = () =>
  renderHook(
    () =>
      useFileOpDialogs({
        activePath: '/tmp',
        realSelection: ['/tmp/notes.txt'],
        destPath: '/other',
        clearSelection: vi.fn(),
      }),
    { wrapper },
  );

beforeEach(() => {
  vi.useFakeTimers();
});

afterEach(() => {
  document.body.replaceChildren();
  vi.useRealTimers();
});

describe('delete confirm focus', () => {
  it('focuses the confirm button 50ms after the dialog opens', () => {
    const { result } = renderDialogs();
    const button = document.createElement('button');
    document.body.append(button);

    act(() => {
      result.current.onDelete();
    });
    result.current.deleteBtnRef.current = button;

    act(() => {
      vi.advanceTimersByTime(49);
    });
    expect(document.activeElement).not.toBe(button);

    act(() => {
      vi.advanceTimersByTime(1);
    });
    expect(document.activeElement).toBe(button);
  });

  it('does not focus while the confirm dialog is closed', () => {
    const { result } = renderDialogs();
    const button = document.createElement('button');
    document.body.append(button);
    result.current.deleteBtnRef.current = button;

    act(() => {
      vi.advanceTimersByTime(50);
    });
    expect(document.activeElement).not.toBe(button);
  });

  it('cancels the focus when the dialog closes before 50ms', () => {
    const { result } = renderDialogs();
    const button = document.createElement('button');
    document.body.append(button);

    act(() => {
      result.current.onDelete();
    });
    result.current.deleteBtnRef.current = button;
    act(() => {
      result.current.dispatchDelete({ type: 'close_confirm' });
    });
    act(() => {
      vi.advanceTimersByTime(50);
    });
    expect(document.activeElement).not.toBe(button);
  });

  it('does not throw when the confirm button is not mounted yet', () => {
    const { result } = renderDialogs();
    act(() => {
      result.current.onDelete();
    });
    expect(() => {
      act(() => {
        vi.advanceTimersByTime(50);
      });
    }).not.toThrow();
  });
});
