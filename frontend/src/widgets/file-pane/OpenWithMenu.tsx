import AppsIcon from '@mui/icons-material/Apps';
import ListItemIcon from '@mui/material/ListItemIcon';
import ListItemText from '@mui/material/ListItemText';
import MenuItem from '@mui/material/MenuItem';
import { useEffect, useState, type FC } from 'react';
import { flushSync } from 'react-dom';
import { FileService } from '../../shared/api/bindings';
import { errMessage } from '../../shared/lib/format';
import { NestedMenuItem, type NestedMenuData } from '../../shared/ui/NestedMenuItem';
import { useSnack } from '../../shared/ui/SnackbarHost';

const APP_CAP = 12;

type App = { id: string; name: string };

type Props = {
  path: string;
  parentOpen: boolean;
  onDone: () => void;
};

const pickerOnly = /Win/i.test(navigator.platform);

/** Nested “Open with” items (or a single picker row on Windows). */
export const OpenWithMenu: FC<Props> = ({ path, parentOpen, onDone }) => {
  const show = useSnack((s) => s.show);
  const [apps, setApps] = useState<App[]>([]);

  useEffect(() => {
    if (pickerOnly) return;
    let cancelled = false;
    void FileService.ListOpenWithApps(path)
      .then((list) => {
        if (!cancelled) setApps(list ?? []);
      })
      .catch((e) => {
        if (!cancelled) show(errMessage(e), 'error');
      });
    return () => {
      cancelled = true;
    };
  }, [path, show]);

  const fail = (e: unknown) => show(errMessage(e), 'error');

  const runAfterClose = (fn: () => void) => {
    flushSync(() => onDone());
    fn();
  };

  const pickOther = () => {
    runAfterClose(() => {
      void FileService.OpenWithPicker(path).catch(fail);
    });
  };

  if (pickerOnly) {
    return (
      <MenuItem dense data-testid="ctx-open-with" onClick={pickOther}>
        <ListItemIcon>
          <AppsIcon fontSize="small" />
        </ListItemIcon>
        <ListItemText primary="Open with…" />
      </MenuItem>
    );
  }

  const items: NestedMenuData[] = [
    ...apps.slice(0, APP_CAP).map((app) => ({
      key: app.id,
      label: app.name,
      onClick: () =>
        runAfterClose(() => {
          void FileService.OpenWith(path, app.id).catch(fail);
        }),
    })),
    {
      key: 'other',
      label: 'Other…',
      testId: 'ctx-open-with-other',
      dividerBefore: apps.length > 0,
      onClick: pickOther,
    },
  ];

  return (
    <NestedMenuItem
      parentOpen={parentOpen}
      onCloseAll={onDone}
      item={{
        label: 'Open with',
        testId: 'ctx-open-with',
        leftIcon: <AppsIcon fontSize="small" />,
        items,
      }}
    />
  );
};
