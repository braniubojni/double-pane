import Box from '@mui/material/Box';
import Checkbox from '@mui/material/Checkbox';
import FormControlLabel from '@mui/material/FormControlLabel';
import Slider from '@mui/material/Slider';
import Typography from '@mui/material/Typography';
import type { FC } from 'react';
import { useDuplicatesStore } from '../../features/duplicates/duplicatesStore';

export const DupSimilarSetup: FC = () => {
  const similarImages = useDuplicatesStore((s) => s.setup.similarImages);
  const ocr = useDuplicatesStore((s) => s.setup.ocr);
  const similarityPct = useDuplicatesStore((s) => s.setup.similarityPct);
  const patchSetup = useDuplicatesStore((s) => s.patchSetup);

  return (
    <>
      <FormControlLabel
        control={
          <Checkbox
            checked={similarImages}
            onChange={(e) => patchSetup({ similarImages: e.target.checked })}
            data-testid="chk-dup-similar"
          />
        }
        label="Similar images"
      />
      <Box sx={{ px: 1 }}>
        <Typography variant="caption" color="text.secondary">
          Similarity {similarityPct}%
        </Typography>
        <Slider
          min={70}
          max={100}
          value={similarityPct}
          disabled={!similarImages && !ocr}
          onChange={(_, v) => patchSetup({ similarityPct: Array.isArray(v) ? v[0] : v })}
          valueLabelDisplay="auto"
          data-testid="slider-dup-similar"
        />
      </Box>
    </>
  );
};
