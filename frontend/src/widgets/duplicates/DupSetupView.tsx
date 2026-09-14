import Alert from '@mui/material/Alert';
import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import Checkbox from '@mui/material/Checkbox';
import DialogActions from '@mui/material/DialogActions';
import FormControlLabel from '@mui/material/FormControlLabel';
import Stack from '@mui/material/Stack';
import TextField from '@mui/material/TextField';
import Typography from '@mui/material/Typography';
import { useMutation, useQuery } from '@tanstack/react-query';
import { useEffect, type FC } from 'react';
import { useDuplicatesStore } from '../../features/duplicates/duplicatesStore';
import { FileService } from '../../shared/api/bindings';
import { errMessage, formatSize } from '../../shared/lib/format';
import { useSnack } from '../../shared/ui/SnackbarHost';
import { DupSimilarSetup } from './DupSimilarSetup';
import { formatEta, megaDiskWarning } from './helpers';
import { viewSx } from './styles';

export const DupSetupView: FC = () => {
  const show = useSnack((s) => s.show);
  const setup = useDuplicatesStore((s) => s.setup);
  const estimate = useDuplicatesStore((s) => s.estimate);
  const patchSetup = useDuplicatesStore((s) => s.patchSetup);
  const setEstimate = useDuplicatesStore((s) => s.setEstimate);
  const closeDialog = useDuplicatesStore((s) => s.closeDialog);

  const ocrAvailQ = useQuery({
    queryKey: ['ocrAvailable'],
    queryFn: () => FileService.OCRAvailable(),
    staleTime: Infinity,
  });
  const ocrAvailable = Boolean(ocrAvailQ.data);

  const estimateMut = useMutation({
    mutationFn: (vars: {
      root: string;
      includeHidden: boolean;
      minSize: number;
      exclude: string;
      similarImages: boolean;
      similarityPct: number;
      ocr: boolean;
    }) =>
      FileService.EstimateDuplicateScan(
        vars.root,
        vars.includeHidden,
        vars.minSize,
        vars.exclude,
        vars.similarImages,
        vars.similarityPct,
        vars.ocr,
      ),
    onSuccess: (est, vars) => {
      const cur = useDuplicatesStore.getState().setup;
      if (
        cur.root !== vars.root ||
        cur.includeHidden !== vars.includeHidden ||
        cur.minSize !== vars.minSize ||
        cur.exclude !== vars.exclude ||
        cur.similarImages !== vars.similarImages ||
        cur.ocr !== vars.ocr
      ) {
        return;
      }
      setEstimate(est);
    },
    onError: (e) => {
      setEstimate(null);
      show(errMessage(e), 'error');
    },
  });

  const startMut = useMutation({
    mutationFn: async () => {
      const jobId = await FileService.NewJobID();
      console.info(
        `[dup] job=${jobId} start root=${setup.root} hidden=${setup.includeHidden} minSize=${setup.minSize} exclude=${setup.exclude} similar=${setup.similarImages} pct=${setup.similarityPct} ocr=${setup.ocr}`,
      );
      if (!jobId) throw new Error('empty jobID');
      useDuplicatesStore.getState().beginScan(jobId);
      await FileService.StartDuplicateScan(
        jobId,
        setup.root,
        setup.includeHidden,
        setup.minSize,
        setup.exclude,
        setup.similarImages,
        setup.similarityPct,
        setup.ocr,
      );
      return jobId;
    },
    onError: (e) => {
      show(errMessage(e), 'error');
      const s = useDuplicatesStore.getState();
      if (s.phase === 'running') s.fail(errMessage(e));
    },
  });

  useEffect(() => {
    if (!setup.root.trim() || estimate || estimateMut.isPending) return;
    estimateMut.mutate(setup);
    // one-shot when setup opens with a root and no estimate yet
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const mega = estimate?.protocol === 'mega' || Boolean(estimate?.megaDownload);
  const warnBytes =
    setup.similarImages || setup.ocr
      ? (estimate?.megaDownloadBytes ?? 0)
      : (estimate?.byteCount ?? 0);
  const canStart = Boolean(estimate) && !estimateMut.isPending && !startMut.isPending;

  return (
    <>
      <Stack sx={viewSx} data-testid="dup-setup">
        <TextField
          label="Folder"
          size="small"
          fullWidth
          value={setup.root}
          onChange={(e) => patchSetup({ root: e.target.value })}
          data-testid="input-dup-root"
        />
        <FormControlLabel
          control={
            <Checkbox
              checked={setup.includeHidden}
              onChange={(e) => patchSetup({ includeHidden: e.target.checked })}
              data-testid="chk-dup-hidden"
            />
          }
          label="Include hidden"
        />
        <TextField
          label="Min size (bytes)"
          size="small"
          type="number"
          value={setup.minSize}
          onChange={(e) => patchSetup({ minSize: Math.max(0, Number(e.target.value) || 0) })}
          data-testid="input-dup-minsize"
        />
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
          <Typography variant="caption" color="text.secondary" sx={{ minWidth: 88 }}>
            files to exclude
          </Typography>
          <TextField
            size="small"
            fullWidth
            value={setup.exclude}
            onChange={(e) => patchSetup({ exclude: e.target.value })}
            placeholder="e.g. build, *lock.json, *.md"
            spellCheck={false}
            data-testid="input-dup-exclude"
          />
        </Box>
        <Typography variant="body2" color="text.secondary">
          Algorithm: SHA-256 (exact content)
        </Typography>
        <DupSimilarSetup />
        <FormControlLabel
          disabled={!ocrAvailable}
          control={
            <Checkbox
              checked={setup.ocr && ocrAvailable}
              onChange={(e) => patchSetup({ ocr: e.target.checked })}
              data-testid="chk-dup-ocr"
            />
          }
          label={ocrAvailable ? 'OCR text' : 'OCR text (install tesseract)'}
        />
        {estimate ? (
          <Typography variant="body2" data-testid="dup-estimate">
            {estimate.fileCount} files · {formatSize(estimate.byteCount, false)} · ETA{' '}
            {formatEta(estimate.etaExactSeconds || estimate.etaSeconds)}
            {setup.similarImages
              ? ` · ${estimate.imageCount ?? 0} images · visual ${formatEta(estimate.etaVisualSeconds ?? 0)}`
              : ''}
            {setup.ocr
              ? ` · OCR ${formatEta(estimate.etaOcrSeconds ?? 0)}`
              : ''}
          </Typography>
        ) : null}
        {estimate && mega ? (
          <Alert severity="warning" data-testid="dup-mega-warning">
            {megaDiskWarning(warnBytes)}
          </Alert>
        ) : null}
      </Stack>
      <DialogActions>
        <Button onClick={closeDialog} data-testid="btn-dup-close">
          Close
        </Button>
        <Button
          onClick={() => estimateMut.mutate(setup)}
          disabled={!setup.root.trim() || estimateMut.isPending}
          data-testid="btn-dup-estimate"
        >
          Estimate
        </Button>
        <Button
          variant="contained"
          disabled={!canStart}
          onClick={() => startMut.mutate()}
          data-testid="btn-dup-start"
        >
          Start
        </Button>
      </DialogActions>
    </>
  );
};
