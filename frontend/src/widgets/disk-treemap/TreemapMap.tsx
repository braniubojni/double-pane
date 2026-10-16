import Box from '@mui/material/Box';
import Tooltip from '@mui/material/Tooltip';
import { useTheme } from '@mui/material/styles';
import { useEffect, useRef, useState, type FC } from 'react';
import type { UsageNode } from '../../features/disk-treemap/types';
import { formatSize } from '../../shared/lib/format';
import { layoutTreemap, type LayoutRect } from './layout';
import { mapSx } from './styles';

const percentLabel = (size: number, total: number): string => {
  if (total <= 0) return '0%';
  const pct = (size / total) * 100;
  return pct < 10 ? `${pct.toFixed(1)}%` : `${Math.round(pct)}%`;
};

type Props = {
  kids: UsageNode[];
  total: number;
  frame: string;
  onReset: () => void;
  onOpenDir: (path: string) => void;
  onOpenFile: (path: string) => void;
};

const fillFor = (name: string, depth: number, other: boolean): { bg: string; fg: string } => {
  if (other) return { bg: '#9aa0a6', fg: '#1c1c1c' };
  let hash = 0;
  for (const ch of name) hash = (hash * 33 + ch.charCodeAt(0)) >>> 0;
  const hue = hash % 360;
  const light = 36 + (depth % 4) * 8;
  return { bg: `hsl(${hue} 46% ${light}%)`, fg: light > 58 ? '#1c1c1c' : '#fff' };
};

export const TreemapMap: FC<Props> = ({ kids, total, frame, onReset, onOpenDir, onOpenFile }) => {
  const theme = useTheme();
  const host = useRef<HTMLDivElement>(null);
  const [box, setBox] = useState({ w: 0, h: 0 });

  useEffect(() => {
    const el = host.current;
    if (!el) return;
    const observer = new ResizeObserver((entries) => {
      const rect = entries[0]?.contentRect;
      if (!rect) return;
      setBox({ w: rect.width, h: rect.height });
    });
    observer.observe(el);
    return () => observer.disconnect();
  }, []);

  const rects = layoutTreemap(kids, box.w, box.h);

  const onClick = (rect: LayoutRect) => {
    if (!rect.path) return;
    if (rect.isDir) onOpenDir(rect.path);
    else onOpenFile(rect.path);
  };

  return (
    <Box sx={{ flex: 1, minHeight: 0, display: 'flex', flexDirection: 'column' }}>
      {frame ? (
        <Box
          component="button"
          type="button"
          data-testid="btn-treemap-zoom-reset"
          onClick={onReset}
          sx={{
            border: 0,
            cursor: 'pointer',
            textAlign: 'left',
            px: 1,
            py: 0.5,
            bgcolor: 'action.selected',
            color: 'text.primary',
          }}
        >
          {frame} — click again for the whole folder
        </Box>
      ) : null}
      <Box ref={host} sx={mapSx}>
        {rects.map((rect) => {
          const { bg, fg } = fillFor(rect.name, rect.depth, rect.colorIndex < 0);
          const showLabel = rect.w >= 48 && rect.h >= 16;
          const sizeLabel = formatSize(rect.size, false);
          return (
            <Tooltip
              key={`${rect.path}:${rect.name}:${rect.x}:${rect.y}`}
              followCursor
              placement="top"
              slotProps={{ popper: { sx: { zIndex: theme.zIndex.tooltip } } }}
              title={
                <Box>
                  <div>{rect.name}</div>
                  <div>
                    {sizeLabel} · {percentLabel(rect.size, total)}
                  </div>
                </Box>
              }
            >
              <Box
                onClick={() => onClick(rect)}
                sx={{
                  position: 'absolute',
                  left: `${rect.x}px`,
                  top: `${rect.y}px`,
                  width: `${Math.max(0, rect.w)}px`,
                  height: `${Math.max(0, rect.h)}px`,
                  bgcolor: bg,
                  color: fg,
                  overflow: 'hidden',
                  fontSize: 12,
                  lineHeight: '16px',
                  px: showLabel ? 0.5 : 0,
                  cursor: rect.path ? 'pointer' : 'default',
                  boxSizing: 'border-box',
                  border: '1px solid rgba(0,0,0,0.45)',
                }}
              >
                {showLabel ? (
                  <Box
                    component="span"
                    sx={{
                      display: 'block',
                      overflow: 'hidden',
                      textOverflow: 'ellipsis',
                      whiteSpace: 'nowrap',
                    }}
                  >
                    {rect.name}
                  </Box>
                ) : null}
              </Box>
            </Tooltip>
          );
        })}
      </Box>
    </Box>
  );
};
