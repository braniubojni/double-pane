import type { FC } from 'react';
import ArchiveIcon from '@mui/icons-material/Archive';
import ArrowBackIcon from '@mui/icons-material/ArrowBack';
import ArrowForwardIcon from '@mui/icons-material/ArrowForward';
import CreateNewFolderIcon from '@mui/icons-material/CreateNewFolder';
import DarkModeIcon from '@mui/icons-material/DarkMode';
import RestoreFromTrashIcon from '@mui/icons-material/RestoreFromTrash';
import DifferenceIcon from '@mui/icons-material/Difference';
import EditIcon from '@mui/icons-material/Edit';
import KeyboardDoubleArrowLeftIcon from '@mui/icons-material/KeyboardDoubleArrowLeft';
import KeyboardDoubleArrowRightIcon from '@mui/icons-material/KeyboardDoubleArrowRight';
import NoteAddIcon from '@mui/icons-material/NoteAdd';
import LightModeIcon from '@mui/icons-material/LightMode';
import RefreshIcon from '@mui/icons-material/Refresh';
import SettingsBrightnessIcon from '@mui/icons-material/SettingsBrightness';
import SettingsIcon from '@mui/icons-material/Settings';
import SwapHorizIcon from '@mui/icons-material/SwapHoriz';
import UnarchiveIcon from '@mui/icons-material/Unarchive';
import AppBar from '@mui/material/AppBar';
import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import Divider from '@mui/material/Divider';
import IconButton from '@mui/material/IconButton';
import MuiToolbar from '@mui/material/Toolbar';
import Tooltip from '@mui/material/Tooltip';
import type { ThemePreference } from '../../../entities/file/types';
import { AIUsageMenu } from '../../ai-usage/AIUsageMenu';
import { PortKillerMenu } from '../../port-killer/PortKillerMenu';
import type { ToolbarBarProps } from '../types';
import { BookmarksSelect } from './BookmarksSelect';
import { ConnectionsMenu } from './ConnectionsMenu';
import { DrivesMenu } from './DrivesMenu';
import { FileActionsMenu } from './FileActionsMenu';

export type { ToolbarBarProps } from '../types';

const themeIcon = (theme: ThemePreference) => {
  if (theme === 'system') return <SettingsBrightnessIcon />;
  if (theme === 'dark') return <DarkModeIcon />;
  return <LightModeIcon />;
};

export const ToolbarBar: FC<ToolbarBarProps> = (p) => {
  return (
    <AppBar
      position="static"
      color="default"
      elevation={0}
      sx={{ borderBottom: 1, borderColor: 'divider' }}
    >
      <MuiToolbar variant="dense" sx={{ gap: 0.5, minHeight: 48 }}>
        <Tooltip title="Back (Backspace)">
          <span>
            <IconButton data-testid="btn-back" disabled={!p.canBack} onClick={p.onBack}>
              <ArrowBackIcon />
            </IconButton>
          </span>
        </Tooltip>
        <Tooltip title="Forward">
          <span>
            <IconButton data-testid="btn-forward" disabled={!p.canForward} onClick={p.onForward}>
              <ArrowForwardIcon />
            </IconButton>
          </span>
        </Tooltip>

        <Divider orientation="vertical" flexItem sx={{ mx: 0.5 }} />

        <Tooltip title="Swap left and right panes">
          <IconButton data-testid="btn-swap-panes" onClick={p.onSwapPanes}>
            <SwapHorizIcon />
          </IconButton>
        </Tooltip>
        <Tooltip
          title={
            p.activePane === 'left'
              ? 'Open current folder in right pane (Ctrl+→)'
              : 'Open current folder in left pane (Ctrl+←)'
          }
        >
          <IconButton data-testid="btn-same-dir" onClick={p.onSameDir}>
            {p.activePane === 'left' ? (
              <KeyboardDoubleArrowRightIcon />
            ) : (
              <KeyboardDoubleArrowLeftIcon />
            )}
          </IconButton>
        </Tooltip>

        <Divider orientation="vertical" flexItem sx={{ mx: 0.5 }} />

        <FileActionsMenu
          otherPaneLabel={p.otherPaneLabel}
          inTrash={p.trashActive}
          onCopy={p.onCopy}
          onMove={p.onMove}
          onRename={p.onRename}
          onDelete={p.onDelete}
          onDeletePermanent={p.onDeletePermanent}
          onRestoreTrash={p.onRestoreTrash}
          onEmptyTrash={p.onEmptyTrash}
        />
        <Button
          data-testid="btn-mkdir"
          startIcon={<CreateNewFolderIcon />}
          onClick={p.onMkdir}
          disabled={p.trashActive}
        >
          New folder
        </Button>
        <Button
          data-testid="btn-mkfile"
          startIcon={<NoteAddIcon />}
          onClick={p.onMkfile}
          disabled={p.trashActive}
        >
          New file
        </Button>
        <Button data-testid="btn-edit" startIcon={<EditIcon />} onClick={p.onEditFile}>
          Edit
        </Button>
        <Tooltip title="Git diff (HEAD vs working tree)">
          <Button data-testid="btn-git-diff" startIcon={<DifferenceIcon />} onClick={p.onGitDiff}>
            Diff
          </Button>
        </Tooltip>

        <Divider orientation="vertical" flexItem sx={{ mx: 0.5 }} />

        <Tooltip title="Create archive from selection">
          <IconButton data-testid="btn-archive" onClick={p.onArchive}>
            <ArchiveIcon />
          </IconButton>
        </Tooltip>
        <Tooltip title="Extract selected archive(s)">
          <IconButton data-testid="btn-extract" onClick={p.onExtract}>
            <UnarchiveIcon />
          </IconButton>
        </Tooltip>

        <Divider orientation="vertical" flexItem sx={{ mx: 0.5 }} />
        <ConnectionsMenu />
        <DrivesMenu />
        <Tooltip title="Trash">
          <IconButton
            data-testid="btn-trash"
            onClick={p.onTrash}
            color={p.trashActive ? 'primary' : 'default'}
          >
            <RestoreFromTrashIcon />
          </IconButton>
        </Tooltip>

        <BookmarksSelect activePane={p.activePane} onAddCurrent={p.onBookmark} />
        <Box sx={{ flex: 1 }} />

        <Tooltip title="Refresh both panes (F5)">
          <IconButton data-testid="btn-refresh" onClick={p.onRefresh}>
            <RefreshIcon />
          </IconButton>
        </Tooltip>
        <Tooltip title={`Theme: ${p.theme} (click to cycle)`}>
          <IconButton data-testid="btn-theme" onClick={p.onCycleTheme}>
            {themeIcon(p.theme)}
          </IconButton>
        </Tooltip>
        <AIUsageMenu />
        <PortKillerMenu />
        <Tooltip title="Settings">
          <IconButton data-testid="btn-settings" onClick={p.onSettings}>
            <SettingsIcon />
          </IconButton>
        </Tooltip>
      </MuiToolbar>
    </AppBar>
  );
};
