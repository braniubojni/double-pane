import Button from '@mui/material/Button';
import DialogActions from '@mui/material/DialogActions';
import Stack from '@mui/material/Stack';
import Typography from '@mui/material/Typography';
import { useMutation } from '@tanstack/react-query';
import type { FC } from 'react';
import { useDuplicatesStore } from '../../features/duplicates/duplicatesStore';
import { FileService } from '../../shared/api/bindings';
import { errMessage } from '../../shared/lib/format';
import { useSnack } from '../../shared/ui/SnackbarHost';
import { viewSx } from './styles';

export const DupErrorView: FC = () => {
  const show = useSnack((s) => s.show);
  const message = useDuplicatesStore((s) => s.errorMessage);
  const jobId = useDuplicatesStore((s) => s.jobId);
  const setup = useDuplicatesStore((s) => s.setup);
  const discard = useDuplicatesStore((s) => s.discard);
  const closeDialog = useDuplicatesStore((s) => s.closeDialog);

  const restartMut = useMutation({
    mutationFn: async () => {
      if (jobId) await FileService.CancelJob(jobId).catch(() => undefined);
      const id = await FileService.NewJobID();
      console.info(
        `[dup] job=${id} start root=${setup.root} hidden=${setup.includeHidden} minSize=${setup.minSize} exclude=${setup.exclude} similar=${setup.similarImages} pct=${setup.similarityPct} ocr=${setup.ocr}`,
      );
      if (!id) throw new Error('empty jobID');
      useDuplicatesStore.getState().beginScan(id);
      await FileService.StartDuplicateScan(
        id,
        setup.root,
        setup.includeHidden,
        setup.minSize,
        setup.exclude,
        setup.similarImages,
        setup.similarityPct,
        setup.ocr,
      );
      return id;
    },
    onError: (e) => {
      show(errMessage(e), 'error');
      const s = useDuplicatesStore.getState();
      if (s.phase === 'running') s.fail(errMessage(e));
    },
  });

  const onDiscard = () => {
    if (jobId) void FileService.CancelJob(jobId).catch(() => undefined);
    discard();
    closeDialog();
  };

  return (
    <>
      <Stack sx={viewSx} data-testid="dup-error">
        <Typography color="error" data-testid="dup-error-message">
          {message}
        </Typography>
      </Stack>
      <DialogActions>
        <Button onClick={onDiscard} data-testid="btn-dup-discard">
          Discard scan
        </Button>
        <Button
          variant="contained"
          disabled={restartMut.isPending}
          onClick={() => restartMut.mutate()}
          data-testid="btn-dup-restart"
        >
          Restart
        </Button>
      </DialogActions>
    </>
  );
};
