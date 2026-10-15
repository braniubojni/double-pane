import Box from '@mui/material/Box';
import CircularProgress from '@mui/material/CircularProgress';
import Typography from '@mui/material/Typography';
import { DataGrid } from '@mui/x-data-grid/DataGrid';
import type { FC } from 'react';
import { isNotConnectedMessage, isRemotePath } from '../../features/connections/helpers';
import { isPermissionDeniedMessage } from '../../shared/lib/format';
import { FileGridRow, FileRowProvider } from './dnd';
import { FileColumnMenu } from './FileColumnMenu';
import { useFileTable } from './hooks';
import { PermissionDeniedNotice } from './PermissionDeniedNotice';
import { ReconnectNotice } from './ReconnectNotice';
import { dataGridSx, getBoxWrapperSx } from './styles';
import type { FileTableProps } from './types';

export const FileTable: FC<FileTableProps> = (props) => {
  const { apiRef, setWrapRef, ...t } = useFileTable(props);

  return (
    <Box
      ref={setWrapRef}
      tabIndex={0}
      id={`file-drop-pane-${t.paneId}`}
      data-testid={`file-grid-${t.paneId}`}
      data-pane-grid={t.paneId}
      data-file-drop-target=""
      data-drop-kind="pane"
      data-pane-id={t.paneId}
      onClick={t.onActivate}
      onContextMenu={t.onContextMenu}
      onKeyDown={t.handleKeys}
      sx={getBoxWrapperSx(t.active, t.dropActive)}
    >
      {t.isLoading && !t.entries ? (
        <Box sx={{ display: 'flex', justifyContent: 'center', p: 4 }}>
          <CircularProgress size={28} />
        </Box>
      ) : t.isError ? (
        isNotConnectedMessage(t.errorMessage ?? '') ? (
          <ReconnectNotice paneId={t.paneId} path={props.panePath} />
        ) : !isRemotePath(props.panePath) && isPermissionDeniedMessage(t.errorMessage ?? '') ? (
          <PermissionDeniedNotice paneId={t.paneId} />
        ) : (
          <Box sx={{ p: 2 }}>
            <Typography color="error" variant="body2">
              {t.errorMessage || 'Failed to load directory'}
            </Typography>
          </Box>
        )
      ) : (
        <FileRowProvider
          value={{
            paneId: t.paneId,
            panePath: props.panePath,
            selected: t.selected,
            onDropPaths: t.onDropPaths,
          }}
        >
          <DataGrid
            apiRef={apiRef}
            rows={t.rows}
            columns={t.columns}
            density="compact"
            disableColumnFilter
            hideFooter
            checkboxSelection={false}
            disableRowSelectionOnClick
            // Row drag/drop lives in FileGridRow (dnd.tsx), backed by react-dnd.
            slots={{
              row: FileGridRow,
              columnMenu: (menuProps) => <FileColumnMenu {...menuProps} paneId={t.paneId} />,
            }}
            // Free DataGrid only allows single selection — multi-select is custom (CSS + Zustand)
            sortingMode="server"
            sortModel={t.sortModel}
            onSortModelChange={t.onSortModelChange}
            columnVisibilityModel={t.columnVisibilityModel}
            onColumnVisibilityModelChange={t.onColumnVisibilityModelChange}
            getRowClassName={t.getRowClassName}
            onColumnWidthChange={t.onColumnWidthChange}
            onCellKeyDown={t.onCellKeyDown}
            onRowClick={t.onRowClick}
            onRowDoubleClick={t.onRowDoubleClick}
            getRowId={(row) => row.path}
            loading={t.isLoading}
            sx={dataGridSx}
          />
        </FileRowProvider>
      )}
    </Box>
  );
};
