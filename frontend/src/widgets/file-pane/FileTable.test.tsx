import { render, screen } from '@testing-library/react';
import type { ReactNode } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { FileTable } from './FileTable';
import type { FileTableProps } from './types';

const hook = vi.hoisted(() => ({ value: {} as Record<string, unknown> }));

vi.mock('./hooks', () => ({ useFileTable: () => hook.value }));
vi.mock('@mui/x-data-grid/DataGrid', () => ({
  DataGrid: () => <div data-testid="grid" />,
}));
vi.mock('./dnd', () => ({
  FileGridRow: () => null,
  FileRowProvider: ({ children }: { children: ReactNode }) => <>{children}</>,
}));
vi.mock('./FileColumnMenu', () => ({ FileColumnMenu: () => null }));
vi.mock('./ReconnectNotice', () => ({ ReconnectNotice: () => <div data-testid="reconnect" /> }));
vi.mock('./PermissionDeniedNotice', () => ({
  PermissionDeniedNotice: () => <div data-testid="denied" />,
}));

const state = (patch: Record<string, unknown> = {}) => ({
  paneId: 'left',
  active: false,
  dropActive: false,
  isLoading: false,
  isError: false,
  errorMessage: undefined,
  entries: [],
  selected: [],
  onActivate: vi.fn(),
  onDropPaths: vi.fn(),
  setWrapRef: vi.fn(),
  handleKeys: vi.fn(),
  apiRef: { current: null },
  rows: [],
  columns: [],
  onContextMenu: vi.fn(),
  ...patch,
});

const props = (patch: Partial<FileTableProps> = {}) =>
  ({ paneId: 'left', panePath: '/work', ...patch }) as FileTableProps;

beforeEach(() => {
  hook.value = state();
});

describe('FileTable states', () => {
  it('labels the wrapper with the pane id', () => {
    render(<FileTable {...props()} />);
    const wrap = screen.getByTestId('file-grid-left');
    expect(wrap.id).toBe('file-drop-pane-left');
    expect(wrap.getAttribute('data-pane-id')).toBe('left');
  });

  it('shows the grid when loaded', () => {
    render(<FileTable {...props()} />);
    expect(screen.getByTestId('grid')).toBeTruthy();
  });

  it('shows a spinner while loading with no entries', () => {
    hook.value = state({ isLoading: true, entries: undefined });
    render(<FileTable {...props()} />);
    expect(screen.getByRole('progressbar')).toBeTruthy();
    expect(screen.queryByTestId('grid')).toBeNull();
  });

  it('keeps the grid while reloading with existing entries', () => {
    hook.value = state({ isLoading: true });
    render(<FileTable {...props()} />);
    expect(screen.getByTestId('grid')).toBeTruthy();
  });

  it('offers reconnect when the remote is not connected', () => {
    hook.value = state({ isError: true, errorMessage: 'not connected to host, connect first' });
    render(<FileTable {...props({ panePath: 'ssh://h/x' })} />);
    expect(screen.getByTestId('reconnect')).toBeTruthy();
  });

  it('shows the permission notice for local operation-not-permitted errors', () => {
    hook.value = state({ isError: true, errorMessage: 'open: operation not permitted' });
    render(<FileTable {...props()} />);
    expect(screen.getByTestId('denied')).toBeTruthy();
  });

  it('shows the raw message for remote operation-not-permitted errors', () => {
    hook.value = state({ isError: true, errorMessage: 'operation not permitted' });
    render(<FileTable {...props({ panePath: 'ssh://h/x' })} />);
    expect(screen.queryByTestId('denied')).toBeNull();
    expect(screen.getByText('operation not permitted')).toBeTruthy();
  });

  it('shows a fallback message for an empty error', () => {
    hook.value = state({ isError: true, errorMessage: '' });
    render(<FileTable {...props()} />);
    expect(screen.getByText('Failed to load directory')).toBeTruthy();
  });
});
