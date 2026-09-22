import Box from '@mui/material/Box';
import Dialog from '@mui/material/Dialog';
import DialogContent from '@mui/material/DialogContent';
import { useCallback, useEffect, useState, type FC } from 'react';
import { useSettings } from '../../entities/file/queries';
import { isMEGAPath } from '../../features/connections/helpers';
import { useEditorStore } from '../../features/editor/editorStore';
import { openDocument } from '../../shared/lib/openDocument';
import { usePaneStore } from '../../features/pane/paneStore';
import { FileService, SettingsService } from '../../shared/api/bindings';
import { errMessage } from '../../shared/lib/format';
import { useSnack } from '../../shared/ui/SnackbarHost';
import { enterPaneTab } from '../file-pane/helpers';
import { AccessDeniedPanel } from './AccessDeniedPanel';
import { includeIsOnlyRoot, uniquePathsFromResults } from './helpers';
import { useSearchSession } from './hooks/useSearchSession';
import { SearchForm } from './SearchForm';
import { SearchResultsList } from './SearchResultsList';
import { contentSx, dialogRootSx, formSectionSx, paperSx } from './styles';
import { defaultSearchPrefs, type SearchPrefs } from './types';

type Props = {
  open: boolean;
  onClose: () => void;
};

export const SearchDialog: FC<Props> = ({ open, onClose }) => {
  const activePane = usePaneStore((s) => s.activePane);
  const root = usePaneStore((s) => s.getPath(s.activePane));
  const navigate = usePaneStore((s) => s.navigate);
  const { data: settings } = useSettings();
  const show = useSnack((s) => s.show);
  const openWorkspace = useEditorStore((s) => s.openWorkspace);

  const [prefs, setPrefs] = useState<SearchPrefs>(defaultSearchPrefs);
  const [index, setIndex] = useState(0);
  const [loaded, setLoaded] = useState(false);

  const session = useSearchSession(open, root, settings?.showHidden ?? false);
  const { results, setResults, searching, denied, setDenied, runSearch, setPrefsRef } = session;

  const patch = useCallback((p: Partial<SearchPrefs>) => {
    setPrefs((prev) => ({ ...prev, ...p }));
  }, []);

  useEffect(() => {
    setPrefsRef(prefs);
  }, [prefs, setPrefsRef]);

  useEffect(() => {
    if (!open) {
      setLoaded(false);
      return;
    }
    const paneRoot = usePaneStore.getState().getPath(usePaneStore.getState().activePane);
    void SettingsService.GetSearchPrefs()
      .then((p) => {
        const savedInclude = p.include ?? '';
        // Show active pane path in "files to include" (replaces the old status line).
        // Keep non-path globs from prefs; absolute path tokens are treated as the current root.
        let include = paneRoot;
        if (savedInclude && !includeIsOnlyRoot(savedInclude, paneRoot)) {
          const globs = savedInclude
            .split(',')
            .map((s) => s.trim())
            .filter(Boolean)
            .filter((s) => !s.startsWith('/') && !/^[A-Za-z]:[\\/]/.test(s));
          include = [paneRoot, ...globs].filter(Boolean).join(', ');
        }
        setPrefs({
          query: p.query ?? '',
          replace: p.replace ?? '',
          include,
          exclude: p.exclude ?? '',
          mode: p.mode === 'folders' ? 'folders' : 'content',
          replaceOpen: Boolean(p.replaceOpen),
          caseSensitive: Boolean(p.caseSensitive),
        });
        setLoaded(true);
      })
      .catch(() => {
        setPrefs({ ...defaultSearchPrefs(), include: paneRoot });
        setLoaded(true);
      });
  }, [open]);

  // Keep include's root path segment in sync when the active pane changes while open.
  useEffect(() => {
    if (!open || !loaded || !root) return;
    setPrefs((prev) => {
      const parts = prev.include
        .split(',')
        .map((s) => s.trim())
        .filter(Boolean);
      const globs = parts.filter((s) => !s.startsWith('/') && !/^[A-Za-z]:[\\/]/.test(s));
      const next = [root, ...globs].join(', ');
      return next === prev.include ? prev : { ...prev, include: next };
    });
  }, [root, open, loaded]);

  useEffect(() => {
    if (!open || !loaded) return;
    const t = window.setTimeout(() => {
      void SettingsService.SaveSearchPrefs(prefs).catch(() => undefined);
    }, 400);
    return () => window.clearTimeout(t);
  }, [prefs, open, loaded]);

  useEffect(() => {
    if (!open || !loaded) return;
    if (prefs.mode === 'content' && !prefs.query.trim()) {
      setResults([]);
      return;
    }
    const t = window.setTimeout(() => {
      void runSearch();
    }, 320);
    return () => window.clearTimeout(t);
    // eslint-disable-next-line react-hooks/exhaustive-deps -- field-driven debounce
  }, [
    prefs.query,
    prefs.include,
    prefs.exclude,
    prefs.mode,
    prefs.caseSensitive,
    open,
    loaded,
    root,
  ]);

  useEffect(() => {
    setIndex(0);
  }, [results.length, prefs.query]);

  const selectResult = useCallback(
    (i: number) => {
      const r = results[i];
      if (!r) return;
      if (r.kind === 'folder') {
        onClose();
        enterPaneTab(activePane, r.hit.path);
        navigate(activePane, r.hit.path);
        return;
      }
      onClose();
      openDocument({
        path: r.hit.path,
        useBuiltInEditor: settings?.useBuiltInEditor !== false,
        openWorkspace,
        show,
      });
    },
    [activePane, navigate, onClose, openWorkspace, results, settings?.useBuiltInEditor, show],
  );

  const setIndexStable = useCallback((i: number) => {
    setIndex(i);
  }, []);

  const replaceOne = async () => {
    const r = results[index];
    if (!r || r.kind !== 'content' || !prefs.query) return;
    try {
      await FileService.ReplaceOccurrence(
        r.hit.path,
        prefs.query,
        prefs.replace,
        r.hit.line,
        r.hit.column,
        prefs.caseSensitive,
      );
      void SettingsService.AddSearchHistory('replace', prefs.replace);
      setResults((list) => list.filter((_, i) => i !== index));
      setIndex((i) => Math.min(i, Math.max(0, results.length - 2)));
      show('Replaced 1 occurrence', 'success');
    } catch (e) {
      show(errMessage(e), 'error');
    }
  };

  const replaceAll = async () => {
    if (prefs.mode !== 'content' || !prefs.query) return;
    const paths = uniquePathsFromResults(results);
    if (paths.length === 0) return;
    if (
      results.length > 50 &&
      !window.confirm(`Replace all ${results.length} matches in ${paths.length} files?`)
    ) {
      return;
    }
    try {
      const res = await FileService.ReplaceAllInPaths(
        paths,
        prefs.query,
        prefs.replace,
        prefs.caseSensitive,
      );
      void SettingsService.AddSearchHistory('replace', prefs.replace);
      show(`Replaced ${res.replacements} in ${res.filesChanged} file(s)`, 'success');
      void runSearch();
    } catch (e) {
      show(errMessage(e), 'error');
    }
  };

  return (
    <Dialog
      open={open}
      onClose={onClose}
      fullWidth
      maxWidth="md"
      data-testid="dialog-search"
      sx={dialogRootSx}
      slotProps={{ paper: { sx: paperSx } }}
      disableRestoreFocus
      scroll="paper"
    >
      <DialogContent sx={contentSx}>
        <Box sx={formSectionSx}>
          <SearchForm
            prefs={prefs}
            patch={patch}
            searching={searching}
            resultCount={results.length}
            contentSearchDisabled={isMEGAPath(root)}
            onSearch={() => void runSearch()}
            onReplaceOne={() => void replaceOne()}
            onReplaceAll={() => void replaceAll()}
          />
          {denied.length > 0 ? (
            <AccessDeniedPanel paths={denied} onDismiss={() => setDenied([])} />
          ) : null}
        </Box>
        <SearchResultsList
          remote={isMEGAPath(root) && prefs.mode !== 'folders'}
          searching={searching}
          results={results}
          index={index}
          onIndex={setIndexStable}
          onSelect={selectResult}
        />
      </DialogContent>
    </Dialog>
  );
};
