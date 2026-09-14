import Button from '@mui/material/Button';
import DialogActions from '@mui/material/DialogActions';
import LinearProgress from '@mui/material/LinearProgress';
import Stack from '@mui/material/Stack';
import Typography from '@mui/material/Typography';
import type { FC } from 'react';
import { useDuplicatesStore } from '../../features/duplicates/duplicatesStore';
import { FileService } from '../../shared/api/bindings';
import { errMessage, formatSize } from '../../shared/lib/format';
import { useSnack } from '../../shared/ui/SnackbarHost';
import { skippedSx, viewSx } from './styles';

export const DupRunningView: FC = () => {
  const show = useSnack((s) => s.show);
  const progress = useDuplicatesStore((s) => s.progress);
  const skipped = useDuplicatesStore((s) => s.skipped);
  const jobId = useDuplicatesStore((s) => s.jobId);
  const closeDialog = useDuplicatesStore((s) => s.closeDialog);
  const fail = useDuplicatesStore((s) => s.fail);

  const done = progress?.doneFiles ?? 0;
  const total = progress?.totalFiles ?? 0;
  const imagePhase = progress?.phase === 'visual' || progress?.phase === 'ocr';
  const imgDone = progress?.doneImages ?? 0;
  const imgTotal = progress?.totalImages ?? 0;
  const numer = imagePhase ? imgDone : done;
  const denom = imagePhase ? imgTotal : total;
  const pct = denom > 0 ? Math.min(100, Math.round((numer / denom) * 100)) : 0;

  const onCancel = () => {
    console.info(`[dup] job=${jobId || '-'} cancel`);
    if (!jobId) {
      show('Duplicate scan has no job id', 'error');
      fail('Duplicate scan has no job id');
      return;
    }
    void FileService.CancelJob(jobId).catch((e) => show(errMessage(e), 'error'));
  };

  return (
    <>
      <Stack sx={viewSx} data-testid="dup-running">
        <Typography variant="body2">
          {progress?.phase === 'ocr'
            ? `OCR: ${imgDone}/${imgTotal}`
            : progress?.phase === 'visual'
              ? `Similar photos: ${imgDone}/${imgTotal}`
              : `Duplicates: ${done}/${total}`}
          {!imagePhase && progress && progress.totalBytes > 0
            ? ` · ${formatSize(progress.doneBytes, false)} / ${formatSize(progress.totalBytes, false)}`
            : ''}
          {progress ? ` · ${progress.groups} groups` : ''}
        </Typography>
        <LinearProgress
          variant={(imagePhase ? imgTotal : total) > 0 ? 'determinate' : 'indeterminate'}
          value={pct}
          data-testid="dup-progress"
        />
        {progress?.currentPath ? (
          <Typography variant="caption" color="text.secondary" noWrap title={progress.currentPath}>
            {progress.currentPath}
          </Typography>
        ) : null}
        {skipped.length > 0 ? (
          <>
            <Typography variant="subtitle2">Skipped ({skipped.length})</Typography>
            <Stack sx={skippedSx} data-testid="dup-skipped">
              {skipped.map((s) => (
                <Typography key={`${s.path}:${s.message}`} variant="caption">
                  {s.path}: {s.message}
                </Typography>
              ))}
            </Stack>
          </>
        ) : null}
      </Stack>
      <DialogActions>
        <Button onClick={closeDialog} data-testid="btn-dup-close">
          Close
        </Button>
        <Button color="error" onClick={onCancel} data-testid="btn-dup-cancel">
          Cancel
        </Button>
      </DialogActions>
    </>
  );
};
