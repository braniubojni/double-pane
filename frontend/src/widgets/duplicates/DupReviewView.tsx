import Button from '@mui/material/Button';
import DialogActions from '@mui/material/DialogActions';
import Stack from '@mui/material/Stack';
import Typography from '@mui/material/Typography';
import type { FC } from 'react';
import { useDuplicatesStore } from '../../features/duplicates/duplicatesStore';
import {
  checkedPaths,
  isExactGroup,
  isOcrGroup,
  isVisualGroup,
} from '../../features/duplicates/helpers';
import type { DuplicateGroup } from '../../features/duplicates/types';
import { DupGroupCard } from './DupGroupCard';
import { groupsSx, skippedSx, viewSx } from './styles';

export const DupReviewView: FC = () => {
  const groups = useDuplicatesStore((s) => s.groups);
  const skipped = useDuplicatesStore((s) => s.skipped);
  const similar = useDuplicatesStore((s) => s.setup.similarImages);
  const ocr = useDuplicatesStore((s) => s.setup.ocr);
  const pct = useDuplicatesStore((s) => s.setup.similarityPct);
  const keepByHash = useDuplicatesStore((s) => s.keepByHash);
  const checkedByHash = useDuplicatesStore((s) => s.checkedByHash);
  const setKeep = useDuplicatesStore((s) => s.setKeep);
  const toggleChecked = useDuplicatesStore((s) => s.toggleChecked);
  const setPhase = useDuplicatesStore((s) => s.setPhase);
  const backToSetup = useDuplicatesStore((s) => s.backToSetup);
  const closeDialog = useDuplicatesStore((s) => s.closeDialog);
  const paths = checkedPaths(checkedByHash);
  const exact = groups.filter(isExactGroup);
  const visual = groups.filter(isVisualGroup);
  const ocrGroups = groups.filter(isOcrGroup);
  const sectioned = similar || ocr;

  const card = (g: DuplicateGroup) => (
    <DupGroupCard
      key={g.hash}
      group={g}
      keep={keepByHash[g.hash] ?? ''}
      checked={checkedByHash[g.hash] ?? []}
      onKeep={(p) => setKeep(g.hash, p)}
      onToggle={(p) => toggleChecked(g.hash, p)}
    />
  );

  return (
    <>
      <Stack sx={viewSx} data-testid="dup-review">
        {sectioned ? (
          <>
            <Typography variant="subtitle1" data-testid="dup-section-exact">
              Identical
            </Typography>
            {exact.length === 0 ? (
              <Typography data-testid="dup-empty">No exact duplicates.</Typography>
            ) : (
              <Stack sx={groupsSx}>{exact.map(card)}</Stack>
            )}
            {similar ? (
              <>
                <Typography variant="subtitle1" data-testid="dup-section-visual">
                  Similar photos (~{pct}%)
                </Typography>
                {visual.length === 0 ? (
                  <Typography data-testid="dup-visual-empty">No similar photos.</Typography>
                ) : (
                  <Stack sx={groupsSx}>{visual.map(card)}</Stack>
                )}
              </>
            ) : null}
            {ocr ? (
              <>
                <Typography variant="subtitle1" data-testid="dup-section-ocr">
                  Similar text
                </Typography>
                {ocrGroups.length === 0 ? (
                  <Typography data-testid="dup-ocr-empty">No similar text.</Typography>
                ) : (
                  <Stack sx={groupsSx}>{ocrGroups.map(card)}</Stack>
                )}
              </>
            ) : null}
          </>
        ) : groups.length === 0 ? (
          <Typography data-testid="dup-empty">No exact duplicates.</Typography>
        ) : (
          <Stack sx={groupsSx}>{groups.map(card)}</Stack>
        )}
        {skipped.length > 0 ? (
          <>
            <Typography variant="subtitle2">Skipped ({skipped.length})</Typography>
            <Stack sx={skippedSx}>
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
        <Button onClick={backToSetup} data-testid="btn-dup-back">
          Back
        </Button>
        <Button onClick={closeDialog} data-testid="btn-dup-close">
          Close
        </Button>
        <Button
          variant="contained"
          disabled={paths.length === 0}
          onClick={() => setPhase('merging')}
          data-testid="btn-dup-merge"
        >
          Delete {paths.length} files · Merge
        </Button>
      </DialogActions>
    </>
  );
};
