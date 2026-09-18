import Button from '@mui/material/Button';
import type { FC } from 'react';
import { useDuplicatesStore } from '../../features/duplicates/duplicatesStore';

export const DuplicateStatusChip: FC = () => {
  const phase = useDuplicatesStore((s) => s.phase);
  const dialogOpen = useDuplicatesStore((s) => s.dialogOpen);
  const progress = useDuplicatesStore((s) => s.progress);
  const visible = phase === 'running' || (phase === 'review' && !dialogOpen);
  if (!visible) return null;

  const done = progress?.doneFiles ?? 0;
  const total = progress?.totalFiles ?? 0;

  return (
    <Button
      size="small"
      data-testid="status-dup-chip"
      onClick={() => useDuplicatesStore.setState({ dialogOpen: true })}
      sx={{ textTransform: 'none', minWidth: 0, px: 1 }}
    >
      Duplicates: {done}/{total}
    </Button>
  );
};
