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
  gap: 1,
};

export const mapSx: SxProps<Theme> = {
  position: 'relative',
  flex: 1,
  minHeight: 0,
};

export const crumbRowSx: SxProps<Theme> = {
  display: 'flex',
  flexWrap: 'wrap',
  gap: 0.5,
};
