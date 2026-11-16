import ExpandLessIcon from '@mui/icons-material/ExpandLess';
import ExpandMoreIcon from '@mui/icons-material/ExpandMore';
import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import Collapse from '@mui/material/Collapse';
import MenuItem from '@mui/material/MenuItem';
import TextField from '@mui/material/TextField';
import { useState, type FC, type KeyboardEvent } from 'react';
import type { SearchFileType, SearchPrefs } from './types';

type Props = {
  prefs: SearchPrefs;
  patch: (p: Partial<SearchPrefs>) => void;
  onSearch: () => void;
};

const kbText = (bytes: number) => (bytes > 0 ? String(bytes / 1024) : '');

const kbToBytes = (raw: string) => {
  const digits = raw.replace(/\D/g, '');
  return digits ? Number(digits) * 1024 : 0;
};

const dateText = (ms: number) => {
  if (!ms) return '';
  const d = new Date(ms);
  const p = (n: number) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}`;
};

const dayStart = (value: string) => (value ? new Date(`${value}T00:00:00`).getTime() || 0 : 0);

const dayEnd = (value: string) => (value ? new Date(`${value}T23:59:59.999`).getTime() || 0 : 0);

const onEnter = (onSearch: () => void) => (e: KeyboardEvent) => {
  if (e.key === 'Enter') onSearch();
};

export const SearchFilters: FC<Props> = ({ prefs, patch, onSearch }) => {
  const [open, setOpen] = useState(false);
  const enter = onEnter(onSearch);
  return (
    <Box data-testid="search-filters" sx={{ px: 1.5, pt: 1 }}>
      <Button
        size="small"
        onClick={() => setOpen((v) => !v)}
        endIcon={open ? <ExpandLessIcon /> : <ExpandMoreIcon />}
      >
        Filters
      </Button>
      <Collapse in={open}>
        <Box sx={{ display: 'flex', gap: 1, pt: 1, flexWrap: 'wrap' }}>
          <TextField
            size="small"
            label="Min KB"
            value={kbText(prefs.minSize)}
            onChange={(e) => patch({ minSize: kbToBytes(e.target.value) })}
            onKeyDown={enter}
            sx={{ width: 100 }}
          />
          <TextField
            size="small"
            label="Max KB"
            value={kbText(prefs.maxSize)}
            onChange={(e) => patch({ maxSize: kbToBytes(e.target.value) })}
            onKeyDown={enter}
            sx={{ width: 100 }}
          />
          <TextField
            size="small"
            type="date"
            label="Modified after"
            value={dateText(prefs.modifiedAfter)}
            onChange={(e) => patch({ modifiedAfter: dayStart(e.target.value) })}
            onKeyDown={enter}
            slotProps={{ inputLabel: { shrink: true } }}
            sx={{ width: 160 }}
          />
          <TextField
            size="small"
            type="date"
            label="Modified before"
            value={dateText(prefs.modifiedBefore)}
            onChange={(e) => patch({ modifiedBefore: dayEnd(e.target.value) })}
            onKeyDown={enter}
            slotProps={{ inputLabel: { shrink: true } }}
            sx={{ width: 170 }}
          />
          <TextField
            select
            size="small"
            label="Type"
            value={prefs.type}
            onChange={(e) => patch({ type: e.target.value as SearchFileType })}
            sx={{ width: 130 }}
          >
            <MenuItem value="">Any</MenuItem>
            <MenuItem value="file">Files</MenuItem>
            <MenuItem value="dir">Folders</MenuItem>
            <MenuItem value="image">Images</MenuItem>
            <MenuItem value="text">Text</MenuItem>
            <MenuItem value="archive">Archives</MenuItem>
          </TextField>
        </Box>
      </Collapse>
    </Box>
  );
};
