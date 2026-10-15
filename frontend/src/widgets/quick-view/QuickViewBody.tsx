import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import Typography from '@mui/material/Typography';
import { useEffect, useState, type FC } from 'react';
import type { Preview } from '../../entities/file/types';
import { previewBytes, showOpenAction } from './helpers';
import { QuickViewText } from './QuickViewText';
import { bodySx, imageSx, pdfFrameSx } from './styles';

type Props = {
  path: string;
  preview: Preview;
  onOpen: () => void;
};

export const QuickViewBody: FC<Props> = ({ path, preview, onOpen }) => {
  const url = useObjectUrl(preview);
  if ((preview.kind === 'image' || preview.kind === 'pdf') && !url) {
    return <Box sx={bodySx} />;
  }
  if (preview.kind === 'image' && url) {
    return (
      <Box sx={bodySx}>
        <Box component="img" data-testid="quick-view-image" alt="" src={url} sx={imageSx} />
      </Box>
    );
  }
  if (preview.kind === 'text') {
    return <QuickViewText path={path} text={preview.text ?? ''} />;
  }
  if (preview.kind === 'pdf' && url) {
    return (
      <Box sx={{ ...bodySx, overflow: 'hidden' }}>
        <Button size="small" variant="outlined" onClick={onOpen}>
          Open
        </Button>
        <Box
          component="iframe"
          data-testid="quick-view-pdf"
          title="PDF preview"
          src={url}
          sx={pdfFrameSx}
        />
      </Box>
    );
  }
  return (
    <Box sx={bodySx} data-testid="quick-view-message">
      <Typography variant="body2">{preview.message}</Typography>
      {showOpenAction(preview.message) && (
        <Button size="small" variant="outlined" onClick={onOpen}>
          Open
        </Button>
      )}
    </Box>
  );
};

/** Blob URL for image/pdf bytes. Revoked when the payload changes. */
const useObjectUrl = (preview: Preview): string => {
  const [url, setUrl] = useState('');
  const { kind, mime, data } = preview;
  useEffect(() => {
    if (kind !== 'image' && kind !== 'pdf') return;
    const bytes = previewBytes(data);
    if (!bytes) return;
    const type = mime || (kind === 'pdf' ? 'application/pdf' : 'application/octet-stream');
    const next = URL.createObjectURL(new Blob([bytes as BlobPart], { type }));
    setUrl(next);
    return () => URL.revokeObjectURL(next);
  }, [kind, mime, data]);
  return kind === 'image' || kind === 'pdf' ? url : '';
};
