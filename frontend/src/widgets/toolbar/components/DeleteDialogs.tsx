import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import Dialog from '@mui/material/Dialog';
import DialogActions from '@mui/material/DialogActions';
import DialogContent from '@mui/material/DialogContent';
import DialogContentText from '@mui/material/DialogContentText';
import DialogTitle from '@mui/material/DialogTitle';
import Typography from '@mui/material/Typography';
import type { FC, RefObject } from 'react';
import { handleDialogEnter, handleDialogFormSubmit } from '../../../shared/lib/dialogSubmit';
import type { DeleteDialogsProps } from '../types';

const deleteCopy = (remote: boolean, permanent: boolean): string => {
  if (remote) return 'This cannot be undone.';
  if (permanent) return 'Permanently delete these items? This cannot be undone.';
  return 'Items are moved to the system trash. Use Undo in the notification, or open Trash to restore.';
};

export const DeleteDialogs: FC<DeleteDialogsProps> = ({
  del,
  dispatch,
  paths,
  remote,
  deleteBtnRef,
  onConfirm,
  onEmpty,
}) => {
  const listed = del.paths.length ? del.paths : paths;
  const closePermission = () => dispatch({ type: 'close_permission' });
  const permanent = del.permanent || remote;

  return (
    <>
      <Dialog
        data-testid="dialog-delete"
        open={del.confirmOpen}
        onClose={() => dispatch({ type: 'close_confirm' })}
        onKeyDown={(e) => handleDialogEnter(e, onConfirm)}
      >
        <form onSubmit={(e) => handleDialogFormSubmit(e, onConfirm)}>
          <DialogTitle>
            {permanent ? 'Permanently delete' : 'Delete'} {listed.length} item(s)?
          </DialogTitle>
          <DialogContent>
            <Typography variant="body2" color="text.secondary">
              {deleteCopy(remote, permanent)}
            </Typography>
            <Box component="ul" sx={{ pl: 2, maxHeight: 160, overflow: 'auto' }}>
              {listed.map((p) => (
                <li key={p}>
                  <Typography variant="caption" sx={{ fontFamily: 'monospace' }}>
                    {p}
                  </Typography>
                </li>
              ))}
            </Box>
          </DialogContent>
          <DialogActions>
            <Button type="button" onClick={() => dispatch({ type: 'close_confirm' })}>
              Cancel
            </Button>
            <Button
              ref={deleteBtnRef as RefObject<HTMLButtonElement>}
              data-testid="btn-delete-confirm"
              type="submit"
              color="error"
              variant="contained"
              autoFocus
            >
              Delete
            </Button>
          </DialogActions>
        </form>
      </Dialog>

      <Dialog
        data-testid="dialog-empty-trash"
        open={del.emptyOpen}
        onClose={() => dispatch({ type: 'close_empty' })}
        onKeyDown={(e) => handleDialogEnter(e, onEmpty)}
      >
        <form onSubmit={(e) => handleDialogFormSubmit(e, onEmpty)}>
          <DialogTitle>Empty Trash?</DialogTitle>
          <DialogContent>
            <DialogContentText>
              This empties the system trash, including items deleted from other apps. This cannot be
              undone.
            </DialogContentText>
          </DialogContent>
          <DialogActions>
            <Button type="button" onClick={() => dispatch({ type: 'close_empty' })}>
              Cancel
            </Button>
            <Button
              data-testid="btn-empty-trash-confirm"
              type="submit"
              color="error"
              variant="contained"
            >
              Empty Trash
            </Button>
          </DialogActions>
        </form>
      </Dialog>

      <Dialog
        data-testid="dialog-permission"
        open={del.permissionOpen}
        onClose={closePermission}
        onKeyDown={(e) => handleDialogEnter(e, closePermission)}
      >
        <form onSubmit={(e) => handleDialogFormSubmit(e, closePermission)}>
          <DialogTitle>Permission denied</DialogTitle>
          <DialogContent>
            <DialogContentText sx={{ mb: 1 }}>{del.permissionMessage}</DialogContentText>
            <DialogContentText variant="body2">
              The system blocked this delete. On macOS, grant the app access under System Settings →
              Privacy &amp; Security → Files and Folders (or Full Disk Access), then try again.
            </DialogContentText>
          </DialogContent>
          <DialogActions>
            <Button data-testid="btn-permission-ok" type="submit" variant="outlined" autoFocus>
              OK
            </Button>
          </DialogActions>
        </form>
      </Dialog>
    </>
  );
};
