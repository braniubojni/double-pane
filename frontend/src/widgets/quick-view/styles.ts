import type { SxProps, Theme } from '@mui/material/styles';

export const paneBodyPreviewSx: SxProps<Theme> = {
  flex: 1,
  minWidth: 0,
  minHeight: 0,
  display: 'flex',
  flexDirection: 'row',
};

export const panelSx: SxProps<Theme> = {
  width: '40%',
  minWidth: 280,
  flexShrink: 0,
  minHeight: 0,
  display: 'flex',
  flexDirection: 'column',
  borderLeft: 1,
  borderColor: 'divider',
  bgcolor: 'background.paper',
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

export const textSx: SxProps<Theme> = {
  m: 0,
  whiteSpace: 'pre-wrap',
  wordBreak: 'break-word',
  fontFamily: 'monospace',
  fontSize: 13,
};

export const pdfFrameSx: SxProps<Theme> = {
  flex: 1,
  width: '100%',
  minHeight: 0,
  border: 0,
};
