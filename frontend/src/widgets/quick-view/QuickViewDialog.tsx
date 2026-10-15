import CircularProgress from '@mui/material/CircularProgress';
import Dialog from '@mui/material/Dialog';
import DialogContent from '@mui/material/DialogContent';
import DialogTitle from '@mui/material/DialogTitle';
import Typography from '@mui/material/Typography';
import type { FC } from 'react';
import { useDirListing, usePreview, useSettings } from '../../entities/file/queries';
import { isRemotePath } from '../../features/connections/helpers';
import { isLocalArchivePath } from '../../features/duplicates/helpers';
import { useEditorStore } from '../../features/editor/editorStore';
import { usePaneStore } from '../../features/pane/paneStore';
import { useQuickViewStore } from '../../features/quick-view/quickViewStore';
import { errMessage } from '../../shared/lib/format';
import { openDocument } from '../../shared/lib/openDocument';
import { isTrashPath } from '../../shared/lib/trash';
import { useSnack } from '../../shared/ui/SnackbarHost';
import { MSG_LOCAL, MSG_SELECT } from './helpers';
import { QuickViewBody } from './QuickViewBody';
import { bodySx, dialogContentSx, dialogPaperSx } from './styles';

const fileName = (path: string): string => path.split(/[/\\]/).pop() || path;

export const QuickViewDialog: FC = () => {
  const close = useQuickViewStore((s) => s.close);
  const activePane = usePaneStore((s) => s.activePane);
  const focus = usePaneStore((s) => (activePane === 'left' ? s.leftFocus : s.rightFocus));
  const panePath = usePaneStore((s) => {
    const tabs = activePane === 'left' ? s.leftTabs : s.rightTabs;
    const idx = activePane === 'left' ? s.leftIndex : s.rightIndex;
    return tabs[idx]?.path ?? '';
  });
  const { data: settings } = useSettings();
  const listing = useDirListing(panePath, settings?.showHidden ?? false);
  const isDir = listing.data?.find((row) => row.path === focus)?.isDir === true;
  const blocked = isRemotePath(focus) || isTrashPath(focus) || isLocalArchivePath(focus);
  const canQuery = Boolean(focus) && !isDir && !blocked;
  const preview = usePreview(focus, canQuery);
  const openWorkspace = useEditorStore((s) => s.openWorkspace);
  const show = useSnack((s) => s.show);

  const onOpen = () => {
    const name = fileName(focus);
    const dot = name.lastIndexOf('.');
    openDocument({
      path: focus,
      name,
      ext: dot > 0 ? name.slice(dot + 1).toLowerCase() : '',
      useBuiltInEditor: settings?.useBuiltInEditor !== false,
      openWorkspace,
      show,
    });
  };

  let message = '';
  if (!focus || isDir) message = MSG_SELECT;
  else if (blocked) message = MSG_LOCAL;
  else if (preview.isError) message = errMessage(preview.error);

  return (
    <Dialog
      open
      onClose={close}
      fullWidth
      maxWidth="md"
      data-testid="dialog-quick-view"
      slotProps={{ paper: { sx: dialogPaperSx } }}
    >
      <DialogTitle>{focus ? fileName(focus) : 'Quick view'}</DialogTitle>
      <DialogContent sx={dialogContentSx}>
        {message ? (
          <Typography variant="body2" data-testid="quick-view-message" sx={bodySx}>
            {message}
          </Typography>
        ) : preview.isPending || !preview.data ? (
          <CircularProgress size={22} sx={{ m: 2 }} />
        ) : (
          <QuickViewBody path={focus} preview={preview.data} onOpen={onOpen} />
        )}
      </DialogContent>
    </Dialog>
  );
};
