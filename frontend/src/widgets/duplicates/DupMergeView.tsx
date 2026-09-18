import Button from '@mui/material/Button';
import DialogActions from '@mui/material/DialogActions';
import Stack from '@mui/material/Stack';
import Typography from '@mui/material/Typography';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import type { FC } from 'react';
import { useDuplicatesStore } from '../../features/duplicates/duplicatesStore';
import { bytesOfPaths, checkedPaths, protocolOf } from '../../features/duplicates/helpers';
import { FileService } from '../../shared/api/bindings';
import { errMessage, formatSize } from '../../shared/lib/format';
import { useSnack } from '../../shared/ui/SnackbarHost';
import { mergeConfirmCopy } from './helpers';
import { viewSx } from './styles';

export const DupMergeView: FC = () => {
  const phase = useDuplicatesStore((s) => s.phase);
  const groups = useDuplicatesStore((s) => s.groups);
  const checkedByHash = useDuplicatesStore((s) => s.checkedByHash);
  const batchId = useDuplicatesStore((s) => s.batchId);
  const estimate = useDuplicatesStore((s) => s.estimate);
  const setPhase = useDuplicatesStore((s) => s.setPhase);
  const setBatchId = useDuplicatesStore((s) => s.setBatchId);
  const discard = useDuplicatesStore((s) => s.discard);
  const closeDialog = useDuplicatesStore((s) => s.closeDialog);
  const show = useSnack((s) => s.show);
  const qc = useQueryClient();
  const paths = checkedPaths(checkedByHash);
  const bytes = bytesOfPaths(groups, paths);
  const protocol = protocolOf(groups, estimate?.protocol || 'local');

  const mergeMut = useMutation({
    mutationFn: (p: string[]) => FileService.Delete(p),
    onSuccess: (id, deleted) => {
      void qc.invalidateQueries({ queryKey: ['dir'] });
      void qc.invalidateQueries({ queryKey: ['gitStatus'] });
      setBatchId(id ?? '');
      show(`Deleted ${deleted.length} file(s)`, 'success');
    },
    onError: (e) => {
      setPhase('review');
      show(errMessage(e), 'error');
    },
  });

  const undoMut = useMutation({
    mutationFn: (id: string) => FileService.RestoreDeleted(id),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['dir'] });
      void qc.invalidateQueries({ queryKey: ['gitStatus'] });
      show('Delete undone', 'success');
      discard();
      closeDialog();
    },
    onError: (e) => show(errMessage(e), 'error'),
  });

  if (phase === 'done') {
    return (
      <>
        <Stack sx={viewSx} data-testid="dup-done">
          <Typography>Merge complete.</Typography>
          {protocol === 'mega' && !batchId ? (
            <Typography variant="body2" color="text.secondary">
              Restore from MEGA, not from this app.
            </Typography>
          ) : null}
        </Stack>
        <DialogActions>
          {batchId ? (
            <Button
              onClick={() => undoMut.mutate(batchId)}
              disabled={undoMut.isPending}
              data-testid="btn-dup-undo"
            >
              Undo
            </Button>
          ) : null}
          <Button
            variant="contained"
            onClick={() => {
              discard();
              closeDialog();
            }}
            data-testid="btn-dup-done-close"
          >
            Close
          </Button>
        </DialogActions>
      </>
    );
  }

  return (
    <>
      <Stack sx={viewSx} data-testid="dup-merging">
        <Typography>
          Delete {paths.length} files ({formatSize(bytes, false)})?
        </Typography>
        <Typography data-testid="dup-confirm-copy">{mergeConfirmCopy(protocol)}</Typography>
      </Stack>
      <DialogActions>
        <Button onClick={() => setPhase('review')} data-testid="btn-dup-merge-cancel">
          Cancel
        </Button>
        <Button
          color="error"
          variant="contained"
          disabled={mergeMut.isPending || paths.length === 0}
          onClick={() => mergeMut.mutate(paths)}
          data-testid="btn-dup-merge-confirm"
        >
          Merge
        </Button>
      </DialogActions>
    </>
  );
};
