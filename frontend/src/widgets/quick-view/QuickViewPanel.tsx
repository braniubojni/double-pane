import Box from '@mui/material/Box';
import CircularProgress from '@mui/material/CircularProgress';
import Typography from '@mui/material/Typography';
import type { FC } from 'react';
import { usePreview } from '../../entities/file/queries';
import { isRemotePath } from '../../features/connections/helpers';
import { isLocalArchivePath } from '../../features/duplicates/helpers';
import { errMessage } from '../../shared/lib/format';
import { isTrashPath } from '../../shared/lib/trash';
import { MSG_LOCAL, MSG_SELECT } from './helpers';
import { bodySx, panelSx } from './styles';
import { QuickViewBody } from './QuickViewBody';

type Props = {
  path: string;
  isDir: boolean;
  onOpen: () => void;
};

export const QuickViewPanel: FC<Props> = ({ path, isDir, onOpen }) => {
  const blocked = isRemotePath(path) || isTrashPath(path) || isLocalArchivePath(path);
  const canQuery = Boolean(path) && !isDir && !blocked;
  const preview = usePreview(path, canQuery);

  let message = '';
  if (!path || isDir) message = MSG_SELECT;
  else if (blocked) message = MSG_LOCAL;
  else if (preview.isError) message = errMessage(preview.error);

  return (
    <Box data-testid="panel-quick-view" sx={panelSx}>
      {message ? (
        <Box sx={bodySx} data-testid="quick-view-message">
          <Typography variant="body2">{message}</Typography>
        </Box>
      ) : preview.isPending || !preview.data ? (
        <Box sx={bodySx}>
          <CircularProgress size={22} />
        </Box>
      ) : (
        <QuickViewBody preview={preview.data} onOpen={onOpen} />
      )}
    </Box>
  );
};
