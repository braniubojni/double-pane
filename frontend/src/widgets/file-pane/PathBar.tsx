import ArrowUpwardIcon from '@mui/icons-material/ArrowUpward';
import ContentCopyIcon from '@mui/icons-material/ContentCopy';
import HomeIcon from '@mui/icons-material/Home';
import Autocomplete from '@mui/material/Autocomplete';
import Box from '@mui/material/Box';
import IconButton from '@mui/material/IconButton';
import ListItem from '@mui/material/ListItem';
import TextField from '@mui/material/TextField';
import Tooltip from '@mui/material/Tooltip';
import { useRef, useState, type FC } from 'react';
import { usePathCompletions } from '../../entities/file/queries';
import { copyText } from '../../shared/lib/clipboard';
import { errMessage } from '../../shared/lib/format';
import { useSnack } from '../../shared/ui/SnackbarHost';
import { PathBreadcrumbs } from './PathBreadcrumbs';
import { pathFieldWrapSx, pathInputSx, pathTooltipSlotSx } from './styles';
import type { PathBarProps } from './types';

/** Normalize path for navigation: trim, strip trailing slashes (except root). */
const normalizeNavPath = (value: string): string => {
  const trimmed = value.trim();
  if (!trimmed) return '/';
  if (/^[a-zA-Z]:\\?$/.test(trimmed)) {
    return trimmed.endsWith('\\') ? trimmed : `${trimmed}\\`;
  }
  const stripped = trimmed.replace(/[/\\]+$/, '');
  return stripped || '/';
};

export const PathBar: FC<PathBarProps> = ({
  paneId,
  path,
  onNavigate,
  onUp,
  onHome,
  onFocusPane,
}) => {
  const highlightedRef = useRef<string | null>(null);
  const inputRef = useRef<HTMLInputElement | null>(null);
  const pickingRef = useRef(false);
  const [draft, setDraft] = useState(path);
  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState(false);
  const show = useSnack((s) => s.show);
  const completions = usePathCompletions(draft, open || draft !== path);
  const options = completions.data ?? [];

  const [prevPath, setPrevPath] = useState(path);
  if (path !== prevPath) {
    setPrevPath(path);
    setDraft(path);
    setEditing(false);
  }

  const beginEdit = () => {
    setEditing(true);
    onFocusPane();
    requestAnimationFrame(() => inputRef.current?.focus());
  };

  const endEdit = () => {
    pickingRef.current = false;
    setDraft(path);
    setOpen(false);
    highlightedRef.current = null;
    setEditing(false);
  };

  const copyPath = () => {
    if (!path) return;
    void copyText(path)
      .then(() => show('Path copied', 'success'))
      .catch((e) => show(errMessage(e), 'error'));
  };

  const submit = (value: string) => {
    pickingRef.current = false;
    const next = normalizeNavPath(value);
    if (next) onNavigate(next);
    setOpen(false);
    highlightedRef.current = null;
    setEditing(false);
  };

  /** Pick best option for Enter: exact path match, else first ranked suggestion. */
  const pickFromOptions = (input: string): string | null => {
    if (!options.length) return null;
    const norm = normalizeNavPath(input).toLowerCase();
    const exact = options.find((o) => normalizeNavPath(o).toLowerCase() === norm);
    if (exact) return exact;
    const tail = input.split(/[/\\]/).pop()?.toLowerCase() ?? '';
    if (tail) {
      const starts = options.find((o) => {
        const base = normalizeNavPath(o).split(/[/\\]/).pop()?.toLowerCase() ?? '';
        return base.startsWith(tail);
      });
      if (starts) return starts;
    }
    return options[0];
  };

  return (
    <Box
      onMouseDown={(e) => {
        if ((e.target as HTMLElement).closest('input,button,textarea,a')) {
          onFocusPane();
          return;
        }
        onFocusPane();
      }}
      sx={{ display: 'flex', alignItems: 'center', gap: 0.5, px: 0.5, py: 0.5 }}
    >
      <Tooltip title="Parent folder">
        <IconButton data-testid={`btn-parent-${paneId}`} onClick={onUp} size="small">
          <ArrowUpwardIcon fontSize="small" />
        </IconButton>
      </Tooltip>
      <Tooltip title="Home">
        <IconButton data-testid={`btn-home-${paneId}`} onClick={onHome} size="small">
          <HomeIcon fontSize="small" />
        </IconButton>
      </Tooltip>
      <Tooltip title="Copy path">
        <IconButton data-testid={`btn-copy-path-${paneId}`} onClick={copyPath} size="small">
          <ContentCopyIcon fontSize="small" />
        </IconButton>
      </Tooltip>
      <Tooltip
        title={path}
        placement="bottom-start"
        slotProps={{ tooltip: { sx: pathTooltipSlotSx } }}
      >
        <Box sx={pathFieldWrapSx(editing)}>
          {!editing && (
            <PathBreadcrumbs
              paneId={paneId}
              path={path}
              onNavigate={onNavigate}
              onEdit={beginEdit}
            />
          )}
          <Autocomplete
            freeSolo
            fullWidth
            size="small"
            open={open}
            onOpen={() => setOpen(true)}
            onClose={() => {
              setOpen(false);
              highlightedRef.current = null;
            }}
            options={options}
            inputValue={draft}
            autoHighlight
            filterOptions={(x) => x}
            slotProps={{
              listbox: {
                onMouseDown: () => {
                  pickingRef.current = true;
                },
              },
            }}
            onHighlightChange={(_, option) => {
              highlightedRef.current = typeof option === 'string' ? option : null;
            }}
            onInputChange={(_, v, reason) => {
              if (reason === 'reset') return;
              setDraft(v);
              if (v !== path) setOpen(true);
            }}
            onChange={(_, v) => {
              if (typeof v === 'string' && v) {
                setDraft(v);
                submit(v);
              }
            }}
            renderInput={(params) => (
              <TextField
                {...params}
                data-testid={`path-input-${paneId}`}
                inputRef={inputRef}
                onFocus={() => {
                  onFocusPane();
                  setEditing(true);
                }}
                onBlur={() => {
                  if (pickingRef.current) return;
                  endEdit();
                }}
                onMouseDown={() => onFocusPane()}
                onKeyDown={(e) => {
                  if (e.key === 'Escape') {
                    endEdit();
                    (e.target as HTMLInputElement).blur();
                    return;
                  }
                  if (e.key !== 'Enter') return;

                  e.preventDefault();
                  e.stopPropagation();
                  if (open && highlightedRef.current) {
                    const pick = highlightedRef.current;
                    setDraft(pick);
                    submit(pick);
                    return;
                  }
                  if (options.length > 0 && draft.trim() !== path) {
                    const pick = pickFromOptions(draft);
                    if (pick) {
                      setDraft(pick);
                      submit(pick);
                      return;
                    }
                  }
                  submit(draft);
                }}
                sx={pathInputSx}
              />
            )}
            renderOption={(props, option) => (
              <ListItem {...props} key={option} aria-label={`path-option-${option}`}>
                {option}
              </ListItem>
            )}
          />
        </Box>
      </Tooltip>
    </Box>
  );
};
