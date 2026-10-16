import type { UsageNode } from '../../features/disk-treemap/types';

export const findUsageNode = (node: UsageNode | undefined, path: string): UsageNode | undefined => {
  if (!node || !path) return undefined;
  if (node.path === path) return node;
  for (const kid of node.kids ?? []) {
    const found = findUsageNode(kid, path);
    if (found) return found;
  }
  return undefined;
};

/** Root through the zoomed node. Unknown zoom paths fall back to the root. */
export const zoomChain = (root: UsageNode, zoomPath: string): UsageNode[] => {
  if (!zoomPath || zoomPath === root.path) return [root];
  const chain: UsageNode[] = [];
  const walk = (node: UsageNode): boolean => {
    chain.push(node);
    if (node.path === zoomPath) return true;
    for (const kid of node.kids ?? []) {
      if (walk(kid)) return true;
    }
    chain.pop();
    return false;
  };
  return walk(root) ? chain : [root];
};
