import ExpandLessIcon from '@mui/icons-material/ExpandLess';
import ExpandMoreIcon from '@mui/icons-material/ExpandMore';
import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import FormControlLabel from '@mui/material/FormControlLabel';
import IconButton from '@mui/material/IconButton';
import Radio from '@mui/material/Radio';
import RadioGroup from '@mui/material/RadioGroup';
import Typography from '@mui/material/Typography';
import { memo, type FC } from 'react';
import { HistoryTextField } from './HistoryTextField';
import { fieldRowSx, headerRowSx, replaceRowSx } from './styles';
import type { SearchMode, SearchPrefs } from './types';

type Props = {
  prefs: SearchPrefs;
  patch: (p: Partial<SearchPrefs>) => void;
  searching: boolean;
  resultCount: number;
  contentSearchDisabled?: boolean;
  onSearch: () => void;
  onReplaceOne: () => void;
  onReplaceAll: () => void;
};

const SearchFormInner: FC<Props> = ({
  prefs,
  patch,
  searching,
  resultCount,
  contentSearchDisabled,
  onSearch,
  onReplaceOne,
  onReplaceAll,
}) => (
  <>
    <Box sx={headerRowSx}>
      <IconButton
        size="small"
        onClick={() => patch({ replaceOpen: !prefs.replaceOpen })}
        data-testid="search-toggle-replace"
        aria-label={prefs.replaceOpen ? 'Collapse replace' : 'Expand replace'}
        disabled={prefs.mode === 'folders'}
      >
        {prefs.replaceOpen && prefs.mode === 'content' ? <ExpandLessIcon /> : <ExpandMoreIcon />}
      </IconButton>
      <HistoryTextField
        field="query"
        value={prefs.query}
        onChange={(v) => patch({ query: v })}
        onEnter={onSearch}
        placeholder={prefs.mode === 'folders' ? 'Folder name…' : 'Search…'}
        autoFocus
        testId="input-search-query"
      />
    </Box>

    {prefs.replaceOpen && prefs.mode === 'content' ? (
      <Box sx={replaceRowSx}>
        <HistoryTextField
          field="replace"
          value={prefs.replace}
          onChange={(v) => patch({ replace: v })}
          onEnter={onReplaceOne}
          placeholder="Replace…"
          testId="input-search-replace"
        />
        <Button
          size="small"
          variant="outlined"
          onClick={onReplaceAll}
          data-testid="btn-search-replace-all"
          disabled={resultCount === 0 || searching}
        >
          Replace All
        </Button>
      </Box>
    ) : null}

    <Box sx={fieldRowSx}>
      <Typography variant="caption" color="text.secondary" sx={{ minWidth: 88 }}>
        files to include
      </Typography>
      <HistoryTextField
        field="include"
        value={prefs.include}
        onChange={(v) => patch({ include: v })}
        onEnter={onSearch}
        placeholder="Active folder path, or e.g. *.ts, *.go"
        testId="input-search-include"
      />
    </Box>
    <Box sx={fieldRowSx}>
      <Typography variant="caption" color="text.secondary" sx={{ minWidth: 88 }}>
        files to exclude
      </Typography>
      <HistoryTextField
        field="exclude"
        value={prefs.exclude}
        onChange={(v) => patch({ exclude: v })}
        onEnter={onSearch}
        placeholder="e.g. build, *lock.json, *.md"
        testId="input-search-exclude"
      />
    </Box>

    <Box sx={{ px: 1.5, pt: 1 }}>
      <RadioGroup
        row
        value={prefs.mode}
        onChange={(_, v) => patch({ mode: v as SearchMode })}
        data-testid="search-mode"
      >
        <FormControlLabel
          value="content"
          control={<Radio size="small" />}
          label="Text in files"
          disabled={contentSearchDisabled}
          title={contentSearchDisabled ? 'Not available on MEGA connections yet' : undefined}
        />
        <FormControlLabel value="folders" control={<Radio size="small" />} label="Folder names" />
      </RadioGroup>
    </Box>
  </>
);

export const SearchForm = memo(SearchFormInner);
