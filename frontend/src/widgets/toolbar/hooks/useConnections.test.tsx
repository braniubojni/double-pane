import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, renderHook } from '@testing-library/react';
import type { ReactNode } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { useConnectRequestStore } from '../../../features/connections/connectRequestStore';
import { useConnections } from './useConnections';

vi.mock('../../../shared/api/bindings', () => ({
  ConnectionService: {
    ListProfiles: async () => [],
    ListSessions: async () => [],
  },
}));
const show = vi.hoisted(() => vi.fn());
vi.mock('../../../shared/ui/SnackbarHost', () => ({
  useSnack: (sel: (s: { show: () => void }) => unknown) => sel({ show }),
}));

const wrapper = ({ children }: { children: ReactNode }) => (
  <QueryClientProvider client={new QueryClient()}>{children}</QueryClientProvider>
);

const ask = (label: string, extra = {}) =>
  act(() =>
    useConnectRequestStore
      .getState()
      .askPassword({ paneId: 'left', targetPath: '/x', profileId: 'p1', label, ...extra }),
  );

beforeEach(() => {
  useConnectRequestStore.setState({ request: null, nonce: 0 });
});

describe('useConnections pending connect request', () => {
  it('opens the password dialog for a request and closes the menu', () => {
    const { result } = renderHook(() => useConnections(), { wrapper });
    act(() => result.current.setAnchor(document.createElement('div')));
    ask('host-a');
    expect(result.current.anchor).toBeNull();
    expect(result.current.dialog).toMatchObject({
      open: true,
      mode: 'password',
      profileId: 'p1',
      spec: 'host-a',
      megaAuth: false,
    });
  });

  it('flags MEGA requests', () => {
    const { result } = renderHook(() => useConnections(), { wrapper });
    ask('m', { spec: 'mega:a@b.c' });
    expect(result.current.dialog.megaAuth).toBe(true);
  });

  it('re-opens for a repeat of the same request (nonce)', () => {
    const { result } = renderHook(() => useConnections(), { wrapper });
    ask('host-a');
    act(() => result.current.dispatch({ type: 'close' } as never));
    expect(result.current.dialog.open).toBe(false);
    ask('host-a');
    expect(result.current.dialog.open).toBe(true);
  });

  it('opens at mount when a request is already pending', () => {
    useConnectRequestStore.getState().askPassword({
      paneId: 'left',
      targetPath: '/x',
      label: 'early',
    });
    const { result } = renderHook(() => useConnections(), { wrapper });
    expect(result.current.dialog).toMatchObject({ open: true, mode: 'password' });
  });

  it('consumes the request once the password dialog goes away', () => {
    const { result } = renderHook(() => useConnections(), { wrapper });
    ask('host-a');
    expect(useConnectRequestStore.getState().request).not.toBeNull();
    act(() => result.current.dispatch({ type: 'close' } as never));
    expect(useConnectRequestStore.getState().request).toBeNull();
  });

  it('does nothing without a request', () => {
    const { result } = renderHook(() => useConnections(), { wrapper });
    expect(result.current.dialog.open).toBe(false);
  });
});
