import Box from '@mui/material/Box';
import Typography from '@mui/material/Typography';
import { memo, useDeferredValue, useTransition, type FC } from 'react';
import { contentHitKey } from './helpers';
import { SearchResultRow } from './SearchResultRow';
import { listSx } from './styles';
import type { SearchResult } from './types';

type Props = {
  remote: boolean;
  searching: boolean;
  results: SearchResult[];
  index: number;
  onIndex: (i: number) => void;
  onSelect: (i: number) => void;
};

const SearchResultsListInner: FC<Props> = ({
  remote,
  searching,
  results,
  index,
  onIndex,
  onSelect,
}) => {
  // Keep typing/search UI snappy while a large result list catches up (React deferred).
  const deferredResults = useDeferredValue(results);
  const deferredIndex = useDeferredValue(index);
  const isStale = deferredResults !== results;
  const [, startHoverTransition] = useTransition();

  const handleHover = (i: number) => {
    // Hover highlight is non-urgent; don't block input/scroll with full re-renders.
    startHoverTransition(() => {
      onIndex(i);
    });
  };

  return (
    <Box sx={listSx} data-testid="search-results" style={isStale ? { opacity: 0.85 } : undefined}>
      {remote ? (
        <Typography variant="body2" color="text.secondary" sx={{ px: 2, py: 1 }}>
          Search is not available on MEGA connections yet
        </Typography>
      ) : deferredResults.length === 0 ? (
        <Typography variant="body2" color="text.secondary" sx={{ px: 2, py: 1 }}>
          {searching ? 'Searching…' : 'No matches'}
        </Typography>
      ) : (
        deferredResults.map((r, i) => (
          <SearchResultRow
            key={r.kind === 'folder' ? r.hit.path : contentHitKey(r.hit)}
            result={r}
            index={i}
            active={i === deferredIndex}
            onHover={handleHover}
            onSelect={onSelect}
          />
        ))
      )}
    </Box>
  );
};

export const SearchResultsList = memo(SearchResultsListInner);
