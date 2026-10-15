import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, renderHook, waitFor } from '@testing-library/react';
import type { ReactNode } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { useEditorStore } from '../../../features/editor/editorStore';
import { useEditorFile } from './useEditorFile';

const m = vi.hoisted(() => ({
  read: vi.fn(),
  write: vi.fn(),
  openOS: vi.fn(),
  show: vi.fn(),
}));

vi.mock('../../../shared/api/bindings', () => ({
  FileService: { ReadTextFile: m.read, WriteTextFile: m.write },
  SettingsService: { OpenInOS: m.openOS },
}));
vi.mock('../../../shared/ui/SnackbarHost', () => ({
  useSnack: (sel: (s: { show: typeof m.show }) => unknown) => sel({ show: m.show }),
}));

const wrapper = ({ children }: { children: ReactNode }) => (
  <QueryClientProvider client={new QueryClient()}>{children}</QueryClientProvider>
);

const setFile = (filePath: string | null) =>
  act(() => useEditorStore.setState({ filePath, dirty: false }));

beforeEach(() => {
  Object.values(m).forEach((f) => f.mockReset());
  m.read.mockImplementation(async (p: string) => `text of ${p}`);
  m.openOS.mockResolvedValue(undefined);
  useEditorStore.setState({ filePath: null, dirty: false });
});

describe('useEditorFile', () => {
  it('loads the file when a path is set', async () => {
    const { result } = renderHook(() => useEditorFile(), { wrapper });
    setFile('/a.txt');
    await waitFor(() => expect(result.current.content).toBe('text of /a.txt'));
    expect(result.current.loading).toBe(false);
    expect(result.current.loadError).toBeNull();
  });

  it('clears content and error when the path is unset', async () => {
    const { result } = renderHook(() => useEditorFile(), { wrapper });
    setFile('/a.txt');
    await waitFor(() => expect(result.current.content).toBe('text of /a.txt'));
    setFile(null);
    await waitFor(() => expect(result.current.content).toBe(''));
    expect(result.current.loadError).toBeNull();
  });

  it('replaces content when switching files', async () => {
    const { result } = renderHook(() => useEditorFile(), { wrapper });
    setFile('/a.txt');
    await waitFor(() => expect(result.current.content).toBe('text of /a.txt'));
    setFile('/b.txt');
    await waitFor(() => expect(result.current.content).toBe('text of /b.txt'));
  });

  it('reports load errors', async () => {
    m.read.mockRejectedValue(new Error('boom'));
    const { result } = renderHook(() => useEditorFile(), { wrapper });
    setFile('/a.txt');
    await waitFor(() => expect(result.current.loadError).toBe('boom'));
    expect(result.current.content).toBe('');
    expect(m.show).toHaveBeenCalledWith('boom', 'error');
  });

  it('hands binary files to the OS', async () => {
    m.read.mockRejectedValue(new Error('binary or unsupported encoding'));
    const { result } = renderHook(() => useEditorFile(), { wrapper });
    setFile('/a.png');
    await waitFor(() => expect(result.current.loadError).toMatch(/Binary file/));
    expect(m.openOS).toHaveBeenCalledWith('/a.png');
  });

  it('marks edits dirty and saves them', async () => {
    m.write.mockResolvedValue(undefined);
    const { result } = renderHook(() => useEditorFile(), { wrapper });
    setFile('/a.txt');
    await waitFor(() => expect(result.current.content).toBe('text of /a.txt'));
    act(() => result.current.onChange('new'));
    expect(result.current.dirty).toBe(true);
    act(() => result.current.save());
    await waitFor(() => expect(m.write).toHaveBeenCalledWith('/a.txt', 'new'));
    await waitFor(() => expect(result.current.dirty).toBe(false));
  });
});
