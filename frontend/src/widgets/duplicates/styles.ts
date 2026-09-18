import { type SxProps, type Theme } from '@mui/material/styles';

export const paperSx: SxProps<Theme> = {
  width: '100%',
  maxWidth: 720,
  maxHeight: 'calc(100vh - 48px)',
};

export const viewSx: SxProps<Theme> = {
  display: 'flex',
  flexDirection: 'column',
  gap: 1.5,
  pt: 0.5,
};

export const groupCardSx: SxProps<Theme> = {
  border: 1,
  borderColor: 'divider',
  borderRadius: 1,
  p: 1,
  mb: 1,
};

export const fileRowSx: SxProps<Theme> = {
  display: 'flex',
  alignItems: 'flex-start',
  gap: 0.5,
  py: 0.25,
  minWidth: 0,
};

export const skippedSx: SxProps<Theme> = {
  maxHeight: 120,
  overflow: 'auto',
  fontFamily: 'monospace',
};

export const groupsSx: SxProps<Theme> = {
  maxHeight: 380,
  overflow: 'auto',
  mt: 1,
};
