import AddIcon from '@mui/icons-material/Add';
import CloseIcon from '@mui/icons-material/Close';
import Autocomplete from '@mui/material/Autocomplete';
import Box from '@mui/material/Box';
import IconButton from '@mui/material/IconButton';
import TextField from '@mui/material/TextField';
import Tooltip from '@mui/material/Tooltip';
import Typography from '@mui/material/Typography';
import type { FC } from 'react';
import { useBookmarks, useFileOps } from '../../../entities/file/queries';
import { isRemotePath } from '../../../features/connections/helpers';
import { ensureSessionThenNavigate } from '../../../features/connections/navigate';
import { errMessage } from '../../../shared/lib/format';
import { useSnack } from '../../../shared/ui/SnackbarHost';
import { shortenPath } from '../../file-pane/helpers';
import {
  bookmarkAddIconSx,
  bookmarkListboxSx,
  bookmarkPaperSx,
  bookmarkRemoveBtnSx,
} from '../styles';
import type { BookmarksSelectProps } from '../types';

type BookmarkGroup = 'Add' | 'Local' | 'Remote';

type Option =
  | { kind: 'add'; group: BookmarkGroup; label: string }
  | { kind: 'bookmark'; group: BookmarkGroup; label: string; id: number; path: string };

export const BookmarksSelect: FC<BookmarksSelectProps> = ({ activePane, onAddCurrent }) => {
  const { data: bookmarks = [], isError, error } = useBookmarks();
  const listError = isError ? errMessage(error) : '';
  const ops = useFileOps();
  const show = useSnack((s) => s.show);

  const toOption = (b: (typeof bookmarks)[number], group: BookmarkGroup): Option => ({
    kind: 'bookmark',
    group,
    label: b.name || b.path,
    id: b.id,
    path: b.path,
  });

  const options: Option[] = [
    { kind: 'add', group: 'Add', label: 'Add current path' },
    ...bookmarks.filter((b) => !isRemotePath(b.path)).map((b) => toOption(b, 'Local')),
    ...bookmarks.filter((b) => isRemotePath(b.path)).map((b) => toOption(b, 'Remote')),
  ];

  return (
    <Autocomplete<Option, false, false, false>
      data-testid="select-bookmarks"
      size="small"
      disabled={isError}
      sx={{ minWidth: 180, ml: 0.5 }}
      options={options}
      value={null}
      blurOnSelect
      clearOnBlur
      groupBy={(o) => o.group}
      getOptionLabel={(o) => o.label}
      isOptionEqualToValue={(a, b) => a.kind === b.kind && a.label === b.label}
      slotProps={{
        paper: { sx: bookmarkPaperSx },
        listbox: { sx: bookmarkListboxSx },
      }}
      onChange={(_, option) => {
        if (!option) return;
        if (option.kind === 'add') {
          onAddCurrent();
          return;
        }
        void ensureSessionThenNavigate(activePane, option.path);
      }}
      renderInput={(params) => (
        <TextField
          {...params}
          placeholder="Bookmarks"
          error={isError}
          helperText={listError || undefined}
          data-testid="input-bookmarks"
        />
      )}
      renderOption={(props, option) => {
        const { key, ...rest } = props;
        if (option.kind === 'add') {
          return (
            <Box component="li" key={key} {...rest} data-testid="option-bookmark-add">
              <AddIcon fontSize="small" sx={bookmarkAddIconSx} />
              <Typography variant="body2" noWrap>
                {option.label}
              </Typography>
            </Box>
          );
        }
        return (
          <Box component="li" key={key} {...rest}>
            <Tooltip title={option.path} placement="right">
              <Typography variant="body2" noWrap sx={{ flex: 1, minWidth: 0 }}>
                {option.label}
                <Typography
                  component="span"
                  variant="caption"
                  color="text.secondary"
                  sx={{ ml: 1 }}
                >
                  {shortenPath(option.path, 30)}
                </Typography>
              </Typography>
            </Tooltip>
            <IconButton
              size="small"
              aria-label={`remove-bookmark-${option.label}`}
              sx={bookmarkRemoveBtnSx}
              onClick={(ev) => {
                ev.stopPropagation();
                ops.removeBookmark.mutate(option.id, {
                  onError: (e) => show(errMessage(e), 'error'),
                });
              }}
            >
              <CloseIcon sx={{ fontSize: 14 }} />
            </IconButton>
          </Box>
        );
      }}
    />
  );
};
