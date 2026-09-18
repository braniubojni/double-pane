export type NameDialogState = {
  open: boolean;
  name: string;
};

export type NameDialogAction =
  | { type: 'open'; name: string }
  | { type: 'close' }
  | { type: 'set_name'; name: string };

export type DeleteDialogState = {
  confirmOpen: boolean;
  permissionOpen: boolean;
  emptyOpen: boolean;
  permissionMessage: string;
  paths: string[];
  permanent: boolean;
};

export type DeleteDialogAction =
  | { type: 'open_confirm'; paths: string[]; permanent?: boolean }
  | { type: 'close_confirm' }
  | { type: 'open_empty' }
  | { type: 'close_empty' }
  | { type: 'open_permission'; message: string }
  | { type: 'close_permission' }
  | { type: 'reset' };

export type FileOpsRequest =
  | 'copy'
  | 'paste'
  | 'move'
  | 'delete'
  | 'deletePermanent'
  | 'restoreTrash'
  | 'emptyTrash'
  | 'rename'
  | 'mkdir'
  | 'mkfile'
  | 'editFile'
  | 'gitDiff'
  | 'goTo'
  | 'refresh'
  | 'goParent'
  | 'goHome'
  | 'goBack'
  | 'goForward'
  | 'calcSizes'
  | 'archive'
  | 'extract'
  | null;

export type FileOpsAction = Exclude<FileOpsRequest, null>;

export type FileOpsState = {
  request: FileOpsRequest;
  nonce: number;
  trigger: (request: FileOpsAction) => void;
  consume: () => void;
};
