import { useEffect } from 'react';
import { usePatchSettings, useSettings, useShortcutDefs } from '../../../entities/file/queries';
import {
  runShortcutAction,
  shortcutToggles,
} from '../../../features/command-palette/runShortcutAction';
import { useEditorStore } from '../../../features/editor/editorStore';
import { useGoToStore } from '../../../features/go-to/goToStore';
import { usePaneStore } from '../../../features/pane/paneStore';
import { useSearchStore } from '../../../features/search/searchStore';
import { useTerminalStore } from '../../../features/terminal/terminalStore';
import { findMatchingAction, isCtrlBackquote } from '../../../shared/lib/shortcuts';
import { historyNav } from '../../../widgets/file-pane/helpers';
import { buildShortcutMap, getPaneGrid, isEditableTarget } from '../helpers';

const isXtermTarget = (target: EventTarget | null): boolean =>
  Boolean((target as HTMLElement | null)?.closest?.('.xterm'));

export const useFileManagerKeyboard = () => {
  const patchSettings = usePatchSettings();
  const { data: settings } = useSettings();
  const { data: shortcutDefs } = useShortcutDefs();
  const openGoTo = useGoToStore((s) => s.openGoTo);
  const openSearch = useSearchStore((s) => s.openSearch);
  const toggleTerminal = useTerminalStore((s) => s.toggleActive);

  useEffect(() => {
    const map = buildShortcutMap(shortcutDefs);
    const toggles = shortcutToggles(patchSettings, settings);

    const onKey = (e: KeyboardEvent) => {
      const editorOpen = useEditorStore.getState().open;
      const matched = findMatchingAction(e, map);

      // Palette: before isEditableTarget so Mod+K works in the path bar.
      if (matched === 'commandPalette') {
        if (isXtermTarget(e.target)) return;
        if ((e.target as HTMLElement | null)?.closest?.('[data-testid="popover-ports"]')) return;
        e.preventDefault();
        runShortcutAction('commandPalette', toggles);
        return;
      }

      // Go-to: Mod+P — disabled while the editor workspace is open
      if (!editorOpen && matched === 'goTo') {
        e.preventDefault();
        openGoTo();
        return;
      }

      // Find in files: Mod+Shift+F (works with editor open too)
      if (matched === 'openSearch') {
        e.preventDefault();
        openSearch();
        return;
      }

      if (isCtrlBackquote(e) || matched === 'toggleTerminal') {
        if (editorOpen) return;
        if (isCtrlBackquote(e) || map.toggleTerminal) {
          const action = findMatchingAction(e, map);
          if (action === 'toggleTerminal' || isCtrlBackquote(e)) {
            e.preventDefault();
            toggleTerminal(usePaneStore.getState().activePane);
            return;
          }
        }
      }

      // F3 toggles quick view even when the dialog (or CodeMirror) has focus.
      if (!editorOpen && matched === 'quickView') {
        e.preventDefault();
        runShortcutAction('quickView', toggles);
        return;
      }

      // While editor open, only allow a few globals (settings/shortcuts/search)
      if (editorOpen) {
        if (isEditableTarget(e.target)) return;
        if (matched === 'openSettings' || matched === 'openShortcuts' || matched === 'openSearch') {
          e.preventDefault();
          runShortcutAction(matched, toggles);
        }
        return;
      }

      if (isEditableTarget(e.target)) return;

      // Mod+A: select all in the active pane (skip inputs; editor is handled above).
      if (
        (e.metaKey || e.ctrlKey) &&
        !e.altKey &&
        !e.shiftKey &&
        (e.key === 'a' || e.key === 'A') &&
        (!map.selectAll || matched === 'selectAll')
      ) {
        e.preventDefault();
        runShortcutAction('selectAll', toggles);
        return;
      }

      if (
        (e.metaKey || e.ctrlKey) &&
        !e.altKey &&
        !e.shiftKey &&
        (e.key === 'v' || e.key === 'V') &&
        (!map.paste || matched === 'paste')
      ) {
        e.preventDefault();
        runShortcutAction('paste', toggles);
        return;
      }

      if (e.key === 'Escape' && !e.metaKey && !e.ctrlKey && !e.altKey && !e.shiftKey) {
        const pane = usePaneStore.getState().activePane;
        const sel = usePaneStore.getState().getSelection(pane);
        if (sel.length > 0) {
          e.preventDefault();
          usePaneStore.getState().clearSelection(pane);
          return;
        }
      }

      if (e.key === 'Backspace' && !e.metaKey && !e.ctrlKey && !e.altKey) {
        e.preventDefault();
        historyNav(usePaneStore.getState().activePane, 'back');
        return;
      }

      if (e.altKey && !e.metaKey && !e.ctrlKey && e.key === 'ArrowLeft') {
        e.preventDefault();
        historyNav(usePaneStore.getState().activePane, 'back');
        return;
      }
      if (e.altKey && !e.metaKey && !e.ctrlKey && e.key === 'ArrowRight') {
        e.preventDefault();
        historyNav(usePaneStore.getState().activePane, 'forward');
        return;
      }

      // Ctrl+← / Ctrl+→: inactive pane opens the active pane's folder.
      // Handled here (like Alt+arrow history) so it works before shortcut
      // defs load and so DataGrid does not steal the keys.
      if (e.ctrlKey && !e.metaKey && !e.altKey && !e.shiftKey) {
        if (e.key === 'ArrowLeft' && (!map.sameDirLeft || matched === 'sameDirLeft')) {
          e.preventDefault();
          runShortcutAction('sameDirLeft', toggles);
          return;
        }
        if (e.key === 'ArrowRight' && (!map.sameDirRight || matched === 'sameDirRight')) {
          e.preventDefault();
          runShortcutAction('sameDirRight', toggles);
          return;
        }
      }

      if (!e.metaKey && !e.ctrlKey && !e.altKey) {
        const pane = usePaneStore.getState().activePane;
        const grid = getPaneGrid(pane);

        if (e.key === 'ArrowDown') {
          e.preventDefault();
          grid?.__gfmMoveFocus?.(1, e.shiftKey);
          return;
        }
        if (e.key === 'ArrowUp') {
          e.preventDefault();
          grid?.__gfmMoveFocus?.(-1, e.shiftKey);
          return;
        }
        if (e.key === 'ArrowLeft') {
          e.preventDefault();
          historyNav(pane, 'back');
          return;
        }
        if (e.key === 'ArrowRight') {
          e.preventDefault();
          grid?.__gfmOpenDir?.();
          return;
        }
        if (e.key === 'Home') {
          e.preventDefault();
          grid?.__gfmFocusHome?.();
          return;
        }
        if (e.key === 'End') {
          e.preventDefault();
          grid?.__gfmFocusEnd?.();
          return;
        }
        if (e.key === 'Enter') {
          const tag = (e.target as HTMLElement | null)?.tagName;
          if (tag !== 'BUTTON' && tag !== 'A') {
            e.preventDefault();
            grid?.__gfmOpenFocused?.();
            return;
          }
        }
        if (e.key === ' ' || e.code === 'Space') {
          const tag = (e.target as HTMLElement | null)?.tagName;
          if (tag !== 'BUTTON' && tag !== 'A') {
            e.preventDefault();
            grid?.__gfmToggleMulti?.();
            return;
          }
        }
      }

      if (!matched) return;
      e.preventDefault();
      runShortcutAction(matched, toggles);
    };

    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [shortcutDefs, patchSettings, settings, toggleTerminal, openGoTo, openSearch]);
};
