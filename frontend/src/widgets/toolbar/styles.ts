import type { SxProps, Theme } from '@mui/material/styles';

export const bookmarkPaperSx: SxProps<Theme> = {
  minWidth: 340,
};

/**
 * Compact bookmark list: one line per entry, tight group headers and a slim
 * scrollbar. The MUI defaults produce a very tall list with a chunky native
 * scrollbar, which is unusable once you have more than a handful of bookmarks.
 */
export const bookmarkListboxSx: SxProps<Theme> = {
  maxHeight: 340,
  py: 0,
  '& .MuiAutocomplete-groupLabel': {
    lineHeight: '22px',
    fontSize: 11,
    py: 0,
    textTransform: 'uppercase',
    letterSpacing: 0.5,
  },
  '& .MuiAutocomplete-option': {
    minHeight: 30,
    py: 0.25,
    px: 1,
    gap: 0.5,
  },
  '&::-webkit-scrollbar': { width: 8 },
  '&::-webkit-scrollbar-thumb': {
    borderRadius: 4,
    backgroundColor: 'action.disabled',
  },
  '&::-webkit-scrollbar-track': { backgroundColor: 'transparent' },
};

export const bookmarkAddIconSx: SxProps<Theme> = { mr: 0.75, fontSize: 16 };

export const bookmarkLabelSx: SxProps<Theme> = { flex: 1, minWidth: 0 };

/** Full path sits above the label. A right-placed tip covers the remove button. */
export const bookmarkTooltipSx: SxProps<Theme> = { maxWidth: 360 };

export const bookmarkRemoveBtnSx: SxProps<Theme> = { p: 0.25, ml: 'auto', flexShrink: 0 };
