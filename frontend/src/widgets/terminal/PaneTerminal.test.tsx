import { ThemeProvider, createTheme } from '@mui/material/styles';
import { fireEvent, render, screen } from '@testing-library/react';
import type { ReactNode } from 'react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { useTerminalStore } from '../../features/terminal/terminalStore';
import { PaneTerminal } from './PaneTerminal';

vi.mock('./hooks/useXterm', () => ({
  useXterm: () => ({ hostRef: { current: null } }),
}));

const onActivate = vi.fn();

const shell = (height: number, node: ReactNode = null) => (
  <ThemeProvider theme={createTheme()}>
    <PaneTerminal paneId="left" cwd="/tmp" height={height} onActivate={onActivate} />
    {node}
  </ThemeProvider>
);

afterEach(() => {
  document.body.style.cursor = '';
  document.body.style.userSelect = '';
  useTerminalStore.setState({ height: 200 });
  onActivate.mockClear();
});

describe('PaneTerminal resize', () => {
  it('sets the row-resize cursor while dragging and clears it on mouseup', () => {
    render(shell(200));
    fireEvent.mouseDown(screen.getByTestId('terminal-resize-left'), { clientY: 100 });
    expect(onActivate).toHaveBeenCalledTimes(2);
    expect(document.body.style.cursor).toBe('row-resize');
    expect(document.body.style.userSelect).toBe('none');

    fireEvent.mouseMove(window, { clientY: 40 });
    expect(useTerminalStore.getState().height).toBe(260);

    fireEvent.mouseUp(window);
    expect(document.body.style.cursor).toBe('');
    expect(document.body.style.userSelect).toBe('');

    fireEvent.mouseMove(window, { clientY: 0 });
    expect(useTerminalStore.getState().height).toBe(260);
  });

  it('clamps the height to the terminal minimum and maximum', () => {
    render(shell(200));
    fireEvent.mouseDown(screen.getByTestId('terminal-resize-left'), { clientY: 100 });
    fireEvent.mouseMove(window, { clientY: -1000 });
    expect(useTerminalStore.getState().height).toBe(600);
    fireEvent.mouseUp(window);

    fireEvent.mouseDown(screen.getByTestId('terminal-resize-left'), { clientY: 100 });
    fireEvent.mouseMove(window, { clientY: 5000 });
    expect(useTerminalStore.getState().height).toBe(100);
  });
});
