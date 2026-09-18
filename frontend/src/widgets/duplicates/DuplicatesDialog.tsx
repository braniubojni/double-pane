import Dialog from '@mui/material/Dialog';
import DialogContent from '@mui/material/DialogContent';
import DialogTitle from '@mui/material/DialogTitle';
import type { FC } from 'react';
import { useDuplicatesStore } from '../../features/duplicates/duplicatesStore';
import { DupErrorView } from './DupErrorView';
import { DupMergeView } from './DupMergeView';
import { DupReviewView } from './DupReviewView';
import { DupRunningView } from './DupRunningView';
import { DupSetupView } from './DupSetupView';
import { paperSx } from './styles';

export const DuplicatesDialog: FC = () => {
  const open = useDuplicatesStore((s) => s.dialogOpen);
  const phase = useDuplicatesStore((s) => s.phase);
  const closeDialog = useDuplicatesStore((s) => s.closeDialog);

  return (
    <Dialog
      open={open}
      onClose={closeDialog}
      fullWidth
      maxWidth="md"
      data-testid="dialog-duplicates"
      slotProps={{ paper: { sx: paperSx } }}
    >
      <DialogTitle>Find duplicates</DialogTitle>
      <DialogContent>
        {open && phase === 'setup' ? <DupSetupView /> : null}
        {open && phase === 'running' ? <DupRunningView /> : null}
        {open && phase === 'error' ? <DupErrorView /> : null}
        {open && phase === 'review' ? <DupReviewView /> : null}
        {open && (phase === 'merging' || phase === 'done') ? <DupMergeView /> : null}
      </DialogContent>
    </Dialog>
  );
};
