import { render } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { runShortcutAction } from '../../../features/command-palette/runShortcutAction';
import { useEditorStore } from '../../../features/editor/editorStore';
import { useGoToStore } from '../../../features/go-to/goToStore';
import { usePaneStore } from '../../../features/pane/paneStore';
import { useSearchStore } from '../../../features/search/searchStore';
import { useTerminalStore } from '../../../features/terminal/terminalStore';
import { historyNav } from '../../../widgets/file-pane/helpers';
import type { PaneGridApi } from '../helpers';
import { useFileManagerKeyboard } from './useFileManagerKeyboard';

vi.mock('../../../entities/file/queries', () => ({
  useSettings: () => ({ data: { showHidden: false } }),
  useShortcutDefs: () => ({
    data: [
      { id: 'commandPalette', binding: 'Mod+K' },
      { id: 'goTo', binding: 'Mod+P' },
      { id: 'openSearch', binding: 'Mod+Shift+F' },
      { id: 'toggleTerminal', binding: 'Ctrl+`' },
      { id: 'openSettings', binding: 'Mod+,' },
      { id: 'openShortcuts', binding: 'Mod+/' },
      { id: 'selectAll', binding: 'Mod+A' },
      { id: 'paste', binding: 'Mod+V' },
      { id: 'sameDirLeft', binding: 'Ctrl+ArrowLeft' },
      { id: 'sameDirRight', binding: 'Ctrl+ArrowRight' },
      { id: 'refresh', binding: 'F5' },
    ],
  }),
  usePatchSettings: () => ({ mutate: vi.fn() }),
}));

vi.mock('../../../features/command-palette/runShortcutAction', () => ({
  runShortcutAction: vi.fn(),
  shortcutToggles: () => ({}),
}));

vi.mock('../../../widgets/file-pane/helpers', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../../widgets/file-pane/helpers')>();
  return { ...actual, historyNav: vi.fn(() => true) };
});

const Harness = () => {
  useFileManagerKeyboard();
  return null;
};

const press = (key: string, init: KeyboardEventInit = {}, target: EventTarget = window) => {
  const event = new KeyboardEvent('keydown', {
    key,
    bubbles: true,
    cancelable: true,
    ...init,
  });
  target.dispatchEvent(event);
  return event;
};

const grid = () => document.querySelector('[data-pane-grid="left"]') as PaneGridApi;

beforeEach(() => {
  document.body.replaceChildren();
  vi.mocked(runShortcutAction).mockClear();
  vi.mocked(historyNav).mockClear();
  useEditorStore.getState().closeWorkspace();
  useGoToStore.getState().closeGoTo();
  useSearchStore.getState().closeSearch();
  usePaneStore.setState({ activePane: 'left', leftSelection: [] });
  const el = document.createElement('div') as PaneGridApi;
  el.setAttribute('data-pane-grid', 'left');
  el.__gfmMoveFocus = vi.fn();
  el.__gfmOpenFocused = vi.fn();
  el.__gfmOpenDir = vi.fn();
  el.__gfmToggleMulti = vi.fn();
  el.__gfmFocusHome = vi.fn();
  el.__gfmFocusEnd = vi.fn();
  document.body.append(el);
});

afterEach(() => {
  document.body.replaceChildren();
});

describe('useFileManagerKeyboard', () => {
  it('opens the palette from an input, and skips the terminal and port popover', () => {
    const input = document.createElement('input');
    const xterm = document.createElement('div');
    xterm.className = 'xterm';
    const ports = document.createElement('div');
    ports.dataset.testid = 'popover-ports';
    document.body.append(input, xterm, ports);
    render(<Harness />);

    press('k', { metaKey: true }, input);
    expect(runShortcutAction).toHaveBeenCalledWith('commandPalette', {});

    press('k', { metaKey: true }, xterm);
    press('k', { metaKey: true }, ports);
    expect(runShortcutAction).toHaveBeenCalledOnce();
  });

  it('opens go-to and search, and skips go-to while the editor is open', () => {
    render(<Harness />);
    press('p', { metaKey: true });
    expect(useGoToStore.getState().open).toBe(true);

    press('F', { metaKey: true, shiftKey: true });
    expect(useSearchStore.getState().open).toBe(true);

    useEditorStore.setState({ open: true });
    useGoToStore.getState().closeGoTo();
    press('p', { metaKey: true });
    expect(useGoToStore.getState().open).toBe(false);
  });

  it('toggles the terminal for the active pane and skips it while the editor is open', () => {
    const toggle = vi.fn();
    useTerminalStore.setState({ toggleActive: toggle });
    usePaneStore.setState({ activePane: 'right' });
    render(<Harness />);

    press('`', { ctrlKey: true, code: 'Backquote' });
    expect(toggle).toHaveBeenCalledWith('right');

    useEditorStore.setState({ open: true });
    press('`', { ctrlKey: true, code: 'Backquote' });
    expect(toggle).toHaveBeenCalledOnce();
  });

  it('while the editor is open, runs settings and shortcuts and ignores pane keys', () => {
    useEditorStore.setState({ open: true });
    render(<Harness />);
    press(',', { metaKey: true });
    press('/', { metaKey: true });
    press('ArrowDown');
    expect(runShortcutAction).toHaveBeenCalledWith('openSettings', {});
    expect(runShortcutAction).toHaveBeenCalledWith('openShortcuts', {});
    expect(grid().__gfmMoveFocus).not.toHaveBeenCalled();
  });

  it('selects all and pastes from the window, not from an input', () => {
    const input = document.createElement('input');
    document.body.append(input);
    render(<Harness />);
    press('a', { metaKey: true });
    press('v', { ctrlKey: true });
    press('a', { metaKey: true }, input);
    expect(runShortcutAction).toHaveBeenCalledWith('selectAll', {});
    expect(runShortcutAction).toHaveBeenCalledWith('paste', {});
    expect(runShortcutAction).toHaveBeenCalledTimes(2);
  });

  it('clears a selection on Escape and leaves an empty selection alone', () => {
    usePaneStore.setState({ leftSelection: ['/tmp/a'] });
    render(<Harness />);
    const escape = press('Escape');
    expect(escape.defaultPrevented).toBe(true);
    expect(usePaneStore.getState().leftSelection).toEqual([]);

    const again = press('Escape');
    expect(again.defaultPrevented).toBe(false);
  });

  it('navigates history on Backspace, Alt+arrows, and a plain left arrow', () => {
    render(<Harness />);
    press('Backspace');
    press('ArrowLeft', { altKey: true });
    press('ArrowRight', { altKey: true });
    press('ArrowLeft');
    expect(historyNav).toHaveBeenNthCalledWith(1, 'left', 'back');
    expect(historyNav).toHaveBeenNthCalledWith(2, 'left', 'back');
    expect(historyNav).toHaveBeenNthCalledWith(3, 'left', 'forward');
    expect(historyNav).toHaveBeenNthCalledWith(4, 'left', 'back');
  });

  it('opens the same directory in the other pane on Ctrl+arrows', () => {
    render(<Harness />);
    press('ArrowLeft', { ctrlKey: true });
    press('ArrowRight', { ctrlKey: true });
    expect(runShortcutAction).toHaveBeenCalledWith('sameDirLeft', {});
    expect(runShortcutAction).toHaveBeenCalledWith('sameDirRight', {});
  });

  it('drives the active pane grid for arrows, home, end, enter, and space', () => {
    render(<Harness />);
    press('ArrowDown');
    press('ArrowUp', { shiftKey: true });
    press('ArrowRight');
    press('Home');
    press('End');
    press('Enter');
    press(' ', { code: 'Space' });

    expect(grid().__gfmMoveFocus).toHaveBeenNthCalledWith(1, 1, false);
    expect(grid().__gfmMoveFocus).toHaveBeenNthCalledWith(2, -1, true);
    expect(grid().__gfmOpenDir).toHaveBeenCalledOnce();
    expect(grid().__gfmFocusHome).toHaveBeenCalledOnce();
    expect(grid().__gfmFocusEnd).toHaveBeenCalledOnce();
    expect(grid().__gfmOpenFocused).toHaveBeenCalledOnce();
    expect(grid().__gfmToggleMulti).toHaveBeenCalledOnce();
  });

  it('lets Enter and Space activate a button or link', () => {
    const button = document.createElement('button');
    const link = document.createElement('a');
    document.body.append(button, link);
    render(<Harness />);
    press('Enter', {}, button);
    press(' ', { code: 'Space' }, link);
    expect(grid().__gfmOpenFocused).not.toHaveBeenCalled();
    expect(grid().__gfmToggleMulti).not.toHaveBeenCalled();
  });

  it('ignores pane keys while typing, and runs a matched shortcut otherwise', () => {
    const input = document.createElement('input');
    document.body.append(input);
    render(<Harness />);
    press('ArrowDown', {}, input);
    press('F5');
    expect(grid().__gfmMoveFocus).not.toHaveBeenCalled();
    expect(runShortcutAction).toHaveBeenCalledWith('refresh', {});
  });

  it('does nothing for an unbound key and stops listening after unmount', () => {
    const view = render(<Harness />);
    const unbound = press('F6');
    expect(unbound.defaultPrevented).toBe(false);
    view.unmount();
    press('ArrowDown');
    expect(grid().__gfmMoveFocus).not.toHaveBeenCalled();
  });
});
