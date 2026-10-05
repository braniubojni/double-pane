import RefreshIcon from '@mui/icons-material/Refresh';
import AutoAwesomeIcon from '@mui/icons-material/AutoAwesome';
import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import Chip from '@mui/material/Chip';
import Skeleton from '@mui/material/Skeleton';
import Typography from '@mui/material/Typography';
import { useState, type FC } from 'react';
import { useAIUsage } from '../../entities/file/queries';
import { errMessage } from '../../shared/lib/format';
import { AIUsageRow } from './AIUsageRow';
import { formatUpdatedAgo } from './helpers';
import { actionRowSx, actionsSx, contentSx, errorSx, listSx, skeletonRowSx } from './styles';

type Props = { open: boolean };

export const AIUsagePanel: FC<Props> = ({ open }) => {
  const { data, dataUpdatedAt, isPending, isFetching, error, refetch } = useAIUsage(open);
  const [openId, setOpenId] = useState<string | null>(null);

  return (
    <Box sx={contentSx}>
      <Box
        sx={{
          display: 'flex',
          alignItems: 'center',
          gap: 0.75,
          px: 1.5,
          py: 1,
          color: 'primary.main',
          fontWeight: 600,
          fontSize: 13,
        }}
      >
        <AutoAwesomeIcon sx={{ fontSize: 16 }} />
        AI usage
      </Box>
      {isPending ? (
        <Box sx={listSx} data-testid="ai-usage-list">
          {[0, 1, 2].map((i) => (
            <Box key={i} sx={skeletonRowSx}>
              <Skeleton variant="rounded" height={36} />
            </Box>
          ))}
        </Box>
      ) : error ? (
        <Typography sx={errorSx}>{errMessage(error)}</Typography>
      ) : (
        <Box sx={listSx} data-testid="ai-usage-list">
          {(data ?? []).map((row) => (
            <AIUsageRow
              key={row.id}
              row={row}
              expanded={openId === row.id}
              onToggle={() => setOpenId((id) => (id === row.id ? null : row.id))}
            />
          ))}
        </Box>
      )}
      <Box sx={actionsSx}>
        <Button
          size="small"
          sx={actionRowSx}
          startIcon={<RefreshIcon fontSize="small" />}
          disabled={isFetching}
          onClick={() => void refetch()}
        >
          Refresh
        </Button>
        {dataUpdatedAt > 0 && <Chip size="small" label={formatUpdatedAgo(dataUpdatedAt)} />}
      </Box>
    </Box>
  );
};
