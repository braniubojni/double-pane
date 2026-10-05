import type { SxProps, Theme } from '@mui/material/styles';

export const paperSx: SxProps<Theme> = {
  width: 340,
  maxWidth: 'calc(100vw - 32px)',
  height: 'auto',
  // MUI Popover paper defaults to maxHeight + overflowY auto.
  maxHeight: 'none',
  overflow: 'visible',
  mt: 0.5,
  borderRadius: 1.5,
  boxShadow: 8,
};

export const contentSx: SxProps<Theme> = {
  p: 0,
};

export const listSx: SxProps<Theme> = {
  overflow: 'visible',
  py: 0.5,
};

export const headerRowSx: SxProps<Theme> = {
  display: 'flex',
  alignItems: 'center',
  gap: 1,
  px: 1.5,
  pt: 1,
  pb: 0.25,
  cursor: 'pointer',
  '&:hover': { opacity: 0.8 },
};

export const headerRowDisabledSx: SxProps<Theme> = {
  ...headerRowSx,
  cursor: 'default',
  '&:hover': {},
};

export const rowMainSx: SxProps<Theme> = {
  display: 'flex',
  flexDirection: 'column',
  flex: '1 1 auto',
  minWidth: 0,
};

export const resetCaptionSx: SxProps<Theme> = {
  color: 'text.secondary',
  fontSize: 12,
  px: 1.5,
  pb: 0.5,
};

export const limitBlockSx: SxProps<Theme> = {
  px: 1.5,
  pt: 0.5,
  pb: 1,
};

export const limitHeaderRowSx: SxProps<Theme> = {
  display: 'flex',
  alignItems: 'baseline',
  justifyContent: 'space-between',
  gap: 1,
  mb: 0.5,
};

export const limitLabelSx: SxProps<Theme> = {
  fontSize: 13,
  fontWeight: 500,
};

export const limitValueSx: SxProps<Theme> = {
  fontSize: 13,
  color: 'text.secondary',
  whiteSpace: 'nowrap',
};

export const progressSx: SxProps<Theme> = {
  height: 6,
  borderRadius: 1,
};

export const detailSx: SxProps<Theme> = {
  px: 1.5,
  py: 0.25,
  fontSize: 12,
  color: 'text.secondary',
};

export const skeletonRowSx: SxProps<Theme> = {
  px: 1.5,
  py: 0.75,
};

export const actionsSx: SxProps<Theme> = {
  borderTop: 1,
  borderColor: 'divider',
  py: 0.5,
  px: 1.5,
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'space-between',
  gap: 1,
};

export const actionRowSx: SxProps<Theme> = {
  textTransform: 'none',
  color: 'text.primary',
};

export const errorSx: SxProps<Theme> = {
  px: 1.5,
  py: 1.5,
  color: 'error.main',
  fontSize: 13,
};
