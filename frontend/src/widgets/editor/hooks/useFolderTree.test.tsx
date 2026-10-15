import { act, renderHook, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { useFolderTree } from './useFolderTree';

const m = vi.hoisted(() => ({
  listDir: vi.fn(),
  showHidden: false,
  show: vi.fn(),
}));

vi.mock('../../../entities/file/queries', () => ({
  useSettings: () => ({ data: { showHidden: m.showHidden } }),
}));
vi.mock('../../../shared/api/bindings', () => ({
  FileService: { ListDir: m.listDir },
}));
vi.mock('../../../shared/ui/SnackbarHost', () => ({
  useSnack: (sel: (s: { show: () => void }) => unknown) => sel({ show: m.show }),
}));

const entry = (name: string) => ({ name, path: `/r/${name}`, isDir: true });

beforeEach(() => {
  m.showHidden = false;
  m.listDir.mockReset();
  m.listDir.mockImplementation(async (dir: string) => [entry('..'), entry(`${dir}-kid`)]);
});

describe('useFolderTree', () => {
  it('expands and loads the root, dropping the ".." entry', async () => {
    const { result } = renderHook(() => useFolderTree('/r'));
    await waitFor(() => expect(result.current.children['/r']).toBeDefined());
    expect(result.current.expanded).toEqual({ '/r': true });
    expect(result.current.children['/r']?.map((e) => e.name)).toEqual(['/r-kid']);
    expect(m.listDir).toHaveBeenCalledWith('/r', false);
  });

  it('toggle loads a folder once and flips expansion', async () => {
    const { result } = renderHook(() => useFolderTree('/r'));
    await waitFor(() => expect(result.current.children['/r']).toBeDefined());
    act(() => result.current.toggle('/r/a'));
    await waitFor(() => expect(result.current.children['/r/a']).toBeDefined());
    expect(result.current.expanded['/r/a']).toBe(true);
    act(() => result.current.toggle('/r/a'));
    act(() => result.current.toggle('/r/a'));
    expect(m.listDir.mock.calls.filter((c) => c[0] === '/r/a')).toHaveLength(1);
  });

  it('resets children and expansion when the root changes', async () => {
    const { result, rerender } = renderHook(({ root }) => useFolderTree(root), {
      initialProps: { root: '/r' },
    });
    await waitFor(() => expect(result.current.children['/r']).toBeDefined());
    act(() => result.current.toggle('/r/a'));
    rerender({ root: '/s' });
    expect(result.current.children['/r']).toBeUndefined();
    expect(result.current.expanded).toEqual({ '/s': true });
    await waitFor(() => expect(result.current.children['/s']).toBeDefined());
  });

  it('reloads from scratch when showHidden changes', async () => {
    const { result, rerender } = renderHook(() => useFolderTree('/r'));
    await waitFor(() => expect(result.current.children['/r']).toBeDefined());
    m.showHidden = true;
    rerender();
    await waitFor(() => expect(m.listDir).toHaveBeenCalledWith('/r', true));
    expect(result.current.expanded).toEqual({ '/r': true });
  });

  it('does nothing without a root', () => {
    const { result } = renderHook(() => useFolderTree(''));
    expect(m.listDir).not.toHaveBeenCalled();
    expect(result.current.children).toEqual({});
  });
});
