import type { Dispatch, RefObject } from 'react';
import type { PaneId, ThemePreference } from '../../entities/file/types';
import type {
  ArchiveAction,
  ArchiveState,
  ExtractAction,
  ExtractState,
} from '../../features/archive/types';
import type {
  AddConnectionAction,
  AddConnectionState,
  SSHConfigHost,
} from '../../features/connections/types';
import type {
  DeleteDialogAction,
  DeleteDialogState,
  FileOpsAction,
  NameDialogAction,
  NameDialogState,
} from '../../features/file-ops/types';

/** One handler per file-ops store action (shortcuts, menu triggers). */
export type ToolbarRequestHandlers = Record<FileOpsAction, () => void>;

export type ToolbarBarProps = {
  activePane: PaneId;
  canBack: boolean;
  canForward: boolean;
  theme: ThemePreference;
  otherPaneLabel: PaneId;
  onBack: () => void;
  onForward: () => void;
  onSwapPanes: () => void;
  onSameDir: () => void;
  onCopy: () => void;
  onMove: () => void;
  onMkdir: () => void;
  onMkfile: () => void;
  onEditFile: () => void;
  onGitDiff: () => void;
  onRename: () => void;
  onDelete: () => void;
  onDeletePermanent: () => void;
  onRestoreTrash: () => void;
  onEmptyTrash: () => void;
  onArchive: () => void;
  onExtract: () => void;
  onBookmark: () => void;
  onRefresh: () => void;
  onCycleTheme: () => void;
  onSettings: () => void;
  onTrash: () => void;
  trashActive: boolean;
};

export type FileActionsMenuProps = {
  otherPaneLabel: PaneId;
  inTrash: boolean;
  onCopy: () => void;
  onMove: () => void;
  onRename: () => void;
  onDelete: () => void;
  onDeletePermanent: () => void;
  onRestoreTrash: () => void;
  onEmptyTrash: () => void;
};

export type BookmarksSelectProps = {
  activePane: PaneId;
  onAddCurrent: () => void;
};

export type NameDialogProps = {
  testId: string;
  title: string;
  label: string;
  inputTestId: string;
  confirmTestId: string;
  confirmLabel: string;
  state: NameDialogState;
  dispatch: Dispatch<NameDialogAction>;
  onConfirm: () => void;
  selectStem?: boolean;
};

export type DeleteDialogsProps = {
  del: DeleteDialogState;
  dispatch: Dispatch<DeleteDialogAction>;
  paths: string[];
  remote: boolean;
  deleteBtnRef: RefObject<HTMLButtonElement | null>;
  onConfirm: () => void;
  onEmpty: () => void;
};

export type ArchiveDialogProps = {
  archive: ArchiveState;
  dispatch: Dispatch<ArchiveAction>;
  selectionCount: number;
  activePath: string;
  onConfirm: () => void;
};

export type ExtractDialogProps = {
  extract: ExtractState;
  dispatch: Dispatch<ExtractAction>;
  selectionCount: number;
  onConfirm: () => void;
};

export type ConnectionDialogProps = {
  dialog: AddConnectionState;
  dispatch: Dispatch<AddConnectionAction>;
  onSubmit: () => void;
  onLoadSSHConfig: (path: string) => void;
  onConnectFromConfig: (host: SSHConfigHost) => void;
  onForgetRecent: (path: string) => void;
};

export type FileOpDialogsArgs = {
  activePath: string;
  realSelection: string[];
  destPath: string;
  clearSelection: () => void;
};

export type ArchiveExtractArgs = {
  activePane: PaneId;
  activePath: string;
  realSelection: string[];
  clearSelection: () => void;
};
