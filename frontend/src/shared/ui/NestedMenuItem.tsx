import ChevronRightIcon from '@mui/icons-material/ChevronRight';
import Divider from '@mui/material/Divider';
import ListItemIcon from '@mui/material/ListItemIcon';
import ListItemText from '@mui/material/ListItemText';
import MenuItem from '@mui/material/MenuItem';
import MenuList from '@mui/material/MenuList';
import Paper from '@mui/material/Paper';
import Popper from '@mui/material/Popper';
import { useId, useState, type FC, type MouseEvent, type ReactNode } from 'react';

export type NestedMenuData = {
  key?: string;
  label: string;
  onClick?: () => void;
  items?: NestedMenuData[];
  leftIcon?: ReactNode;
  disabled?: boolean;
  testId?: string;
  dividerBefore?: boolean;
};

type Props = {
  item: NestedMenuData;
  /** False when an ancestor menu has closed. Submenus unmount with it. */
  parentOpen: boolean;
  onCloseAll?: () => void;
};

const entered = (target: EventTarget | null, attr: string, value: string) => {
  const el =
    target instanceof Element ? target : target instanceof Node ? target.parentElement : null;
  return Boolean(el?.closest(`[${attr}="${value}"]`));
};

/**
 * One row in an existing MUI Menu, with an optional submenu.
 * In-app stand-in for a nested-dropdown library. The parent Menu already
 * closes on Escape and on an outside click; this component only tracks its
 * own anchor.
 */
export const NestedMenuItem: FC<Props> = ({ item, parentOpen, onCloseAll }) => {
  const marker = useId();
  const [anchorEl, setAnchorEl] = useState<HTMLElement | null>(null);
  const children = item.items ?? [];
  const branch = children.length > 0;
  const open = parentOpen && Boolean(anchorEl);

  const closeSub = () => setAnchorEl(null);

  const openSub = (e: MouseEvent<HTMLElement>) => {
    if (!branch) return;
    e.stopPropagation();
    setAnchorEl(e.currentTarget);
  };

  const onItemLeave = (e: MouseEvent<HTMLElement>) => {
    if (entered(e.relatedTarget, 'data-nested-paper', marker)) return;
    closeSub();
  };

  const onPaperLeave = (e: MouseEvent<HTMLDivElement>) => {
    if (entered(e.relatedTarget, 'data-nested-item', marker)) return;
    closeSub();
  };

  const onLeafClick = (e: MouseEvent<HTMLElement>) => {
    e.stopPropagation();
    onCloseAll?.();
    item.onClick?.();
  };

  return (
    <>
      {item.dividerBefore ? <Divider /> : null}
      <MenuItem
        dense
        disabled={item.disabled}
        data-testid={item.testId}
        data-nested-item={marker}
        onMouseEnter={openSub}
        onMouseLeave={branch ? onItemLeave : undefined}
        onClick={branch ? openSub : onLeafClick}
      >
        {item.leftIcon ? <ListItemIcon>{item.leftIcon}</ListItemIcon> : null}
        <ListItemText primary={item.label} />
        {branch ? <ChevronRightIcon fontSize="small" /> : null}
      </MenuItem>
      {branch ? (
        <Popper
          open={open}
          anchorEl={anchorEl}
          placement="right-start"
          sx={{ zIndex: (t) => t.zIndex.modal + 1 }}
        >
          <Paper
            elevation={8}
            data-nested-paper={marker}
            onMouseLeave={onPaperLeave}
            sx={{ minWidth: 180, ml: '-8px' }}
          >
            <MenuList dense>
              {children.map((child) => (
                <NestedMenuItem
                  key={child.key ?? child.testId ?? child.label}
                  item={child}
                  parentOpen={open}
                  onCloseAll={onCloseAll}
                />
              ))}
            </MenuList>
          </Paper>
        </Popper>
      ) : null}
    </>
  );
};
