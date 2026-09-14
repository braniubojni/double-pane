import Checkbox from '@mui/material/Checkbox';
import Radio from '@mui/material/Radio';
import RadioGroup from '@mui/material/RadioGroup';
import Typography from '@mui/material/Typography';
import Box from '@mui/material/Box';
import type { FC } from 'react';
import type { DuplicateGroup } from '../../features/duplicates/types';
import { formatModTime, formatSize } from '../../shared/lib/format';
import { hashPrefix } from './helpers';
import { fileRowSx, groupCardSx } from './styles';

type Props = {
  group: DuplicateGroup;
  keep: string;
  checked: string[];
  onKeep: (path: string) => void;
  onToggle: (path: string) => void;
};

export const DupGroupCard: FC<Props> = ({ group, keep, checked, onKeep, onToggle }) => {
  const set = new Set(checked);
  return (
    <Box sx={groupCardSx} data-testid="dup-group">
      <Typography variant="subtitle2">
        {group.files.length} files · {formatSize(group.size, false)} · {hashPrefix(group.hash)}
        {group.kind === 'visual' && group.similarity ? ` · ~${group.similarity}%` : ''}
        {group.kind === 'ocr' && group.snippet ? ` · “${group.snippet}”` : ''}
      </Typography>
      <RadioGroup value={keep} onChange={(e) => onKeep(e.target.value)}>
        {group.files.map((f) => (
          <Box key={f.path} sx={fileRowSx}>
            <Radio value={f.path} size="small" data-testid={`dup-keep-${f.path}`} />
            <Checkbox
              size="small"
              disabled={keep === f.path}
              checked={set.has(f.path)}
              onChange={() => onToggle(f.path)}
              data-testid={`dup-del-${f.path}`}
            />
            <Box sx={{ minWidth: 0 }}>
              <Typography variant="body2" noWrap>
                {f.name}
              </Typography>
              <Typography variant="caption" color="text.secondary" noWrap sx={{ display: 'block' }}>
                {f.path}
              </Typography>
              <Typography variant="caption" color="text.secondary">
                {formatSize(f.size, false)} · {formatModTime(f.modTime)} · {f.protocol}
              </Typography>
            </Box>
          </Box>
        ))}
      </RadioGroup>
    </Box>
  );
};
