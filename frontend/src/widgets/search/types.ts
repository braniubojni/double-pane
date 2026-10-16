export type SearchMode = 'content' | 'folders' | 'files';

export type ContentHit = {
  path: string;
  relPath: string;
  line: number;
  column: number;
  lineText: string;
  matchStart: number;
  matchEnd: number;
};

type FolderHit = {
  name: string;
  path: string;
  isDir: boolean;
  relPath: string;
};

export type SearchResult =
  | { kind: 'content'; hit: ContentHit }
  | { kind: 'folder'; hit: FolderHit };

export type SearchPrefs = {
  query: string;
  replace: string;
  include: string;
  exclude: string;
  mode: SearchMode;
  replaceOpen: boolean;
  caseSensitive: boolean;
};

export const defaultSearchPrefs = (): SearchPrefs => ({
  query: '',
  replace: '',
  include: '',
  exclude: '',
  mode: 'content',
  replaceOpen: false,
  caseSensitive: false,
});

export type HistoryField = 'query' | 'replace' | 'include' | 'exclude';
