import type { UsageNode } from '../../features/disk-treemap/types';

export type LayoutRect = {
  path: string;
  name: string;
  size: number;
  isDir: boolean;
  x: number;
  y: number;
  w: number;
  h: number;
  colorIndex: number;
  depth: number;
  hasKids: boolean;
};

const HEADER = 16;
const INSET = 3;

type Box = { x: number; y: number; w: number; h: number };

type Item = UsageNode & { area: number; colorIndex: number };

const worst = (row: Item[], side: number): number => {
  if (row.length === 0 || side <= 0) return Number.POSITIVE_INFINITY;
  let sum = 0;
  let max = 0;
  let min = Number.POSITIVE_INFINITY;
  for (const item of row) {
    sum += item.area;
    if (item.area > max) max = item.area;
    if (item.area < min) min = item.area;
  }
  if (sum === 0 || min === 0) return Number.POSITIVE_INFINITY;
  const sum2 = sum * sum;
  const side2 = side * side;
  return Math.max((side2 * max) / sum2, sum2 / (side2 * min));
};

const pushNode = (item: Item, box: Box, depth: number, out: LayoutRect[]): void => {
  const hasKids = Boolean(item.kids?.length);
  out.push({
    path: item.path,
    name: item.name,
    size: item.size,
    isDir: item.isDir,
    x: box.x,
    y: box.y,
    w: box.w,
    h: box.h,
    colorIndex: item.path === '' ? -1 : item.colorIndex,
    depth,
    hasKids,
  });
  if (!hasKids || !item.kids) return;
  const innerW = box.w - INSET * 2;
  const innerH = box.h - HEADER - INSET;
  if (innerW < 8 || innerH < 8) return;
  layoutInto(
    item.kids,
    { x: box.x + INSET, y: box.y + HEADER, w: innerW, h: innerH },
    item.path === '' ? -1 : item.colorIndex,
    depth + 1,
    out,
  );
};

const placeRow = (row: Item[], box: Box, depth: number, out: LayoutRect[]): Box => {
  const sum = row.reduce((total, item) => total + item.area, 0);
  if (box.w >= box.h) {
    const rowWidth = box.h === 0 ? 0 : sum / box.h;
    let y = box.y;
    for (const item of row) {
      const h = rowWidth === 0 ? 0 : item.area / rowWidth;
      pushNode(item, { x: box.x, y, w: rowWidth, h }, depth, out);
      y += h;
    }
    return { x: box.x + rowWidth, y: box.y, w: Math.max(0, box.w - rowWidth), h: box.h };
  }
  const rowHeight = box.w === 0 ? 0 : sum / box.w;
  let x = box.x;
  for (const item of row) {
    const w = rowHeight === 0 ? 0 : item.area / rowHeight;
    pushNode(item, { x, y: box.y, w, h: rowHeight }, depth, out);
    x += w;
  }
  return { x: box.x, y: box.y + rowHeight, w: box.w, h: Math.max(0, box.h - rowHeight) };
};

const squarify = (items: Item[], row: Item[], box: Box, depth: number, out: LayoutRect[]): void => {
  if (box.w <= 0 || box.h <= 0) return;
  if (items.length === 0) {
    if (row.length > 0) placeRow(row, box, depth, out);
    return;
  }
  const side = Math.min(box.w, box.h);
  const next = items[0];
  const withNext = row.concat(next);
  if (row.length === 0 || worst(row, side) >= worst(withNext, side)) {
    squarify(items.slice(1), withNext, box, depth, out);
    return;
  }
  squarify(items, [], placeRow(row, box, depth, out), depth, out);
};

const layoutInto = (
  nodes: UsageNode[],
  box: Box,
  parentColor: number,
  depth: number,
  out: LayoutRect[],
): void => {
  const total = nodes.reduce((sum, node) => sum + node.size, 0);
  if (total <= 0 || box.w <= 0 || box.h <= 0) return;
  const area = box.w * box.h;
  const items: Item[] = [];
  nodes.forEach((node, index) => {
    if (node.size <= 0) return;
    const topLevel = parentColor < 0;
    let colorIndex = parentColor;
    if (node.path === '') colorIndex = -1;
    else if (topLevel) colorIndex = index;
    items.push({ ...node, area: (node.size / total) * area, colorIndex });
  });
  squarify(items, [], box, depth, out);
};

/** Squarify kids into width × height. A zero total yields no rects. */
export const layoutTreemap = (kids: UsageNode[], width: number, height: number): LayoutRect[] => {
  const out: LayoutRect[] = [];
  layoutInto(kids, { x: 0, y: 0, w: width, h: height }, -1, 0, out);
  return out;
};
