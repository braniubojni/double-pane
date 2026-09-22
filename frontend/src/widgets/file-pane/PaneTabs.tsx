import AddIcon from '@mui/icons-material/Add';
import ArrowDropDownIcon from '@mui/icons-material/ArrowDropDown';
import CloseIcon from '@mui/icons-material/Close';
import CloudIcon from '@mui/icons-material/Cloud';
import DescriptionIcon from '@mui/icons-material/Description';
import DesktopWindowsIcon from '@mui/icons-material/DesktopWindows';
import DownloadIcon from '@mui/icons-material/Download';
import FolderIcon from '@mui/icons-material/Folder';
import HomeIcon from '@mui/icons-material/Home';
import ImageIcon from '@mui/icons-material/Image';
import MovieIcon from '@mui/icons-material/Movie';
import MusicNoteIcon from '@mui/icons-material/MusicNote';
import Box from '@mui/material/Box';
import IconButton from '@mui/material/IconButton';
import Menu from '@mui/material/Menu';
import MenuItem from '@mui/material/MenuItem';
import Tab from '@mui/material/Tab';
import Tabs from '@mui/material/Tabs';
import Tooltip from '@mui/material/Tooltip';
import type { FC } from 'react';
import { useState } from 'react';
import { useQuickPlaces } from '../../entities/file/queries';
import type { PaneId } from '../../entities/file/types';
import { isRemotePath } from '../../features/connections/helpers';
import type { PaneTab } from '../../features/pane/paneStore';
import { tabLabel } from './helpers';
import {
  addTabBtnSx,
  tabCloseBtnSx,
  tabCloudIconSx,
  tabLabelRowSx,
  tabSx,
  tabsRowSx,
  tabsSx,
} from './styles';

type Props = {
  id: PaneId;
  tabs: PaneTab[];
  activeTabId: string;
  onSelectTab: (tabId: string) => void;
  onCloseTab: (tabId: string) => void;
  onAddTab: () => void;
  onAddTabAt: (path: string) => void;
};

const placeIcon = (name: string) => {
  if (name === 'Home') return <HomeIcon fontSize="small" />;
  if (name === 'Desktop') return <DesktopWindowsIcon fontSize="small" />;
  if (name === 'Documents') return <DescriptionIcon fontSize="small" />;
  if (name === 'Downloads') return <DownloadIcon fontSize="small" />;
  if (name === 'Pictures') return <ImageIcon fontSize="small" />;
  if (name === 'Music') return <MusicNoteIcon fontSize="small" />;
  if (name === 'Videos') return <MovieIcon fontSize="small" />;
  return <FolderIcon fontSize="small" />;
};

export const PaneTabs: FC<Props> = ({
  id,
  tabs,
  activeTabId,
  onSelectTab,
  onCloseTab,
  onAddTab,
  onAddTabAt,
}) => {
  const [quickPlacesAnchor, setQuickPlacesAnchor] = useState<null | HTMLElement>(null);
  const { data: quickPlaces = [] } = useQuickPlaces();

  return (
    <Box sx={tabsRowSx} data-testid={`pane-${id}-tabs`}>
      <Tabs
        value={activeTabId}
        onChange={(_e, val: string) => onSelectTab(val)}
        variant="scrollable"
        scrollButtons="auto"
        sx={tabsSx}
      >
        {tabs.map((tab) => (
          <Tab
            key={tab.id}
            value={tab.id}
            data-testid={`pane-${id}-tab-${tab.id}`}
            sx={tabSx(tab.id === activeTabId)}
            onAuxClick={(e) => {
              if (e.button === 1) {
                e.preventDefault();
                onCloseTab(tab.id);
              }
            }}
            label={
              <Box sx={tabLabelRowSx}>
                {isRemotePath(tab.path) && <CloudIcon sx={tabCloudIconSx} />}
                <span title={tab.path}>{tabLabel(tab.path)}</span>
                {tabs.length > 1 && (
                  <IconButton
                    size="small"
                    sx={tabCloseBtnSx}
                    data-testid={`pane-${id}-tab-close-${tab.id}`}
                    onClick={(e) => {
                      e.stopPropagation();
                      onCloseTab(tab.id);
                    }}
                  >
                    <CloseIcon sx={tabCloudIconSx} />
                  </IconButton>
                )}
              </Box>
            }
          />
        ))}
      </Tabs>
      <Tooltip title="New tab">
        <IconButton
          size="small"
          sx={addTabBtnSx}
          data-testid={`pane-${id}-tab-add`}
          onClick={onAddTab}
        >
          <AddIcon fontSize="small" />
        </IconButton>
      </Tooltip>
      {quickPlaces.length > 0 && (
        <>
          <Tooltip title="Quick places">
            <IconButton
              size="small"
              sx={addTabBtnSx}
              data-testid={`pane-${id}-quick-places`}
              onClick={(e) => setQuickPlacesAnchor(e.currentTarget)}
            >
              <ArrowDropDownIcon fontSize="small" />
            </IconButton>
          </Tooltip>
          <Menu
            anchorEl={quickPlacesAnchor}
            open={Boolean(quickPlacesAnchor)}
            onClose={() => setQuickPlacesAnchor(null)}
          >
            {quickPlaces.map((place) => (
              <MenuItem
                key={place.name}
                data-testid={`pane-${id}-quick-place-${place.name}`}
                dense
                onClick={() => {
                  onAddTabAt(place.path);
                  setQuickPlacesAnchor(null);
                }}
                sx={{ display: 'flex', gap: 1 }}
              >
                {placeIcon(place.name)}
                {place.name}
              </MenuItem>
            ))}
          </Menu>
        </>
      )}
    </Box>
  );
};
