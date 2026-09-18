import type { GridColDef } from '@mui/x-data-grid/models';
import { FileEntry } from '../../entities/file/types';
import { orderColumns } from '../../features/ui/gridPrefsStore';
import { FileTypeIcon } from '../../shared/ui/FileTypeIcon';
import Tooltip from '@mui/material/Tooltip';
import Typography from '@mui/material/Typography';
import { formatModTime, formatSize } from '../../shared/lib/format';
import { accessLabel, lookupFolderSize, sizeValue, typeValue } from './helpers';

/** Effective access for a row: a size walk that got denied overrides the
 * listing's value, and is the only source of truth for remote entries. */
const rowAccess = (e: FileEntry, denied: Set<string> | undefined): string =>
  denied?.has(e.path) ? 'none' : e.access;

export const getColumns = (
  widths: Record<string, number>,
  selected: string[],
  folderSizes: Record<string, number> | undefined,
  deniedPaths?: Set<string>,
  order: string[] = [],
  showOrigin = false,
): GridColDef[] =>
  orderColumns<GridColDef>(
    [
      {
        field: 'icon',
        headerName: '',
        width: widths.icon ?? 44,
        sortable: false,
        resizable: false,
        disableColumnMenu: true,
        cellClassName: 'no-select-cell',
        renderCell: (params) => <FileTypeIcon entry={params.row as FileEntry} />,
      },
      {
        field: 'displayName',
        headerName: 'Name',
        hideable: false,
        width: widths.displayName ?? 220,
        flex: widths.displayName ? undefined : 1,
        minWidth: 120,
        cellClassName: 'name-cell',
        renderCell: (params) => {
          const e = params.row as FileEntry & { displayName: string };
          const isSelected = selected.includes(e.path);
          return (
            <Typography
              variant="body2"
              noWrap
              sx={{
                fontWeight: e.isDir ? 600 : 400,
                userSelect: 'text',
                lineHeight: 1.2,
                width: '100%',
                color: isSelected ? 'error.main' : 'inherit',
              }}
            >
              {e.displayName}
              {e.isSymlink ? ' ↗' : ''}
            </Typography>
          );
        },
      },
      {
        field: 'size',
        headerName: 'Size',
        width: widths.size ?? 90,
        cellClassName: 'no-select-cell',
        // Prefer row.folderSizeBytes (stamped when sizes finish) so DataGrid always
        // re-renders; fall back to live folderSizes map for sorting/valueGetter.
        valueGetter: (_v, row) => {
          const e = row as FileEntry & { folderSizeBytes?: number };
          if (e.isDir && e.folderSizeBytes != null) return e.folderSizeBytes;
          return sizeValue(e, folderSizes);
        },
        valueFormatter: (value, row) => {
          const e = row as FileEntry & { folderSizeBytes?: number };
          if (e.isDir) {
            const n = e.folderSizeBytes ?? lookupFolderSize(folderSizes, e.path, e.name);
            if (n != null) return formatSize(n, false);
            return '<DIR>';
          }
          return formatSize(Number(value) || 0, false);
        },
      },
      {
        field: 'modTime',
        headerName: 'Modified',
        width: widths.modTime ?? 150,
        cellClassName: 'no-select-cell',
        valueFormatter: (value) => formatModTime(Number(value) || 0),
      },
      ...(showOrigin
        ? [
            {
              field: 'origin',
              headerName: 'Original path',
              width: widths.origin ?? 280,
              flex: widths.origin ? undefined : 1,
              minWidth: 140,
              cellClassName: 'no-select-cell',
              valueGetter: (_v: unknown, row: FileEntry) => row.origin || '',
            } satisfies GridColDef,
          ]
        : []),
      {
        field: 'ext',
        headerName: 'Type',
        width: widths.ext ?? 70,
        cellClassName: 'no-select-cell',
        valueGetter: (_v, row) => typeValue(row as FileEntry),
      },
      {
        field: 'access',
        headerName: 'Permission',
        width: widths.access ?? 90,
        cellClassName: 'no-select-cell',
        valueGetter: (_v, row) => rowAccess(row as FileEntry, deniedPaths),
        renderCell: (params) => {
          const e = params.row as FileEntry;
          if (e.name === '..') return null;
          const { text, color, title } = accessLabel(rowAccess(e, deniedPaths));
          return (
            <Tooltip title={title}>
              <Typography variant="body2" noWrap sx={{ color, lineHeight: 1.2 }}>
                {text}
              </Typography>
            </Tooltip>
          );
        },
      },
    ],
    order,
  );
