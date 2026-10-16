import { describe, expect, it } from 'vitest';
import { layoutTreemap } from './layout';

const node = (name: string, size: number) => ({
  name,
  path: `/${name}`,
  size,
  isDir: false,
});

describe('layoutTreemap', () => {
  it('one child fills the box', () => {
    const [rect] = layoutTreemap([node('a', 10)], 200, 80);
    expect(rect.x).toBeCloseTo(0);
    expect(rect.y).toBeCloseTo(0);
    expect(rect.w).toBeCloseTo(200);
    expect(rect.h).toBeCloseTo(80);
  });

  it('two equal children split the shorter side', () => {
    const rects = layoutTreemap([node('a', 10), node('b', 10)], 200, 100);
    expect(rects).toHaveLength(2);
    for (const rect of rects) {
      expect(rect.h).toBeCloseTo(100);
      expect(rect.w).toBeCloseTo(100);
    }
    expect(rects[0].x).toBeCloseTo(0);
    expect(rects[1].x).toBeCloseTo(100);
  });

  it('zero total yields no rects', () => {
    expect(layoutTreemap([node('a', 0)], 100, 100)).toEqual([]);
    expect(layoutTreemap([], 100, 100)).toEqual([]);
  });

  it('lays children under the folder title', () => {
    const rects = layoutTreemap(
      [
        {
          name: 'dir',
          path: '/dir',
          size: 10,
          isDir: true,
          kids: [node('a', 10)],
        },
      ],
      200,
      80,
    );
    const parent = rects.find((rect) => rect.path === '/dir');
    const child = rects.find((rect) => rect.path === '/a');
    expect(parent?.hasKids).toBe(true);
    expect(parent?.h).toBeCloseTo(80);
    expect(child).toBeDefined();
    expect(child!.y).toBeGreaterThan(14);
    expect(child!.y + child!.h).toBeLessThanOrEqual(80.1);
  });
});
