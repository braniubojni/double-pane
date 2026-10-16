export type UsageNode = {
  name: string;
  path: string;
  size: number;
  isDir: boolean;
  kids?: UsageNode[];
};

export type UsageProgress = {
  jobId: string;
  current: string;
  files: number;
  bytes: number;
};

export type UsageDone = {
  jobId: string;
  root: string;
  total: number;
  files: number;
  truncated: boolean;
  denied: string[] | null;
  deniedCount: number;
  error?: string;
  tree?: UsageNode;
};
