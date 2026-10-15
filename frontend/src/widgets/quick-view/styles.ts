import type { SxProps, Theme } from '@mui/material/styles';

export const dialogPaperSx: SxProps<Theme> = {
  height: '70vh',
  display: 'flex',
  flexDirection: 'column',
};

export const dialogContentSx: SxProps<Theme> = {
  display: 'flex',
  flexDirection: 'column',
  flex: 1,
  minHeight: 0,
  p: 0,
  overflow: 'hidden',
};

export const bodySx: SxProps<Theme> = {
  flex: 1,
  minHeight: 0,
  overflow: 'auto',
  p: 1,
  display: 'flex',
  flexDirection: 'column',
  gap: 1,
};

export const imageSx: SxProps<Theme> = {
  width: '100%',
  height: '100%',
  objectFit: 'contain',
};

export const textHostSx: SxProps<Theme> = {
  // height: 0 + flex gives this box a real height. Otherwise CodeMirror
  // grows with the file, the dialog clips it, and the wheel has nothing to scroll.
  flex: 1,
  height: 0,
  minHeight: 0,
  display: 'flex',
  flexDirection: 'column',
  '& > div': {
    flex: 1,
    minHeight: 0,
    height: '100%',
  },
  '& .cm-editor': { height: '100%' },
  '& .cm-scroller': {
    overflow: 'scroll !important',
    '&::-webkit-scrollbar': { width: 10, height: 10 },
    '&::-webkit-scrollbar-thumb': {
      backgroundColor: 'rgba(127, 127, 127, 0.55)',
      borderRadius: 8,
    },
    '&::-webkit-scrollbar-track': { backgroundColor: 'transparent' },
  },
};

export const pdfFrameSx: SxProps<Theme> = {
  flex: 1,
  width: '100%',
  minHeight: 0,
  border: 0,
};
