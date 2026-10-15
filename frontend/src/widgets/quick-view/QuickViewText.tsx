import { EditorState } from '@codemirror/state';
import { oneDark } from '@codemirror/theme-one-dark';
import { EditorView, lineNumbers } from '@codemirror/view';
import { useTheme } from '@mui/material/styles';
import Box from '@mui/material/Box';
import CodeMirror from '@uiw/react-codemirror';
import { useMemo, type FC } from 'react';
import { languageExtensionForPath } from '../editor/helpers';
import { textHostSx } from './styles';

type Props = {
  path: string;
  text: string;
};

export const QuickViewText: FC<Props> = ({ path, text }) => {
  const theme = useTheme();
  const isDark = theme.palette.mode === 'dark';
  const extensions = useMemo(
    () => [
      lineNumbers(),
      EditorView.editable.of(false),
      EditorState.readOnly.of(true),
      EditorView.lineWrapping,
      ...languageExtensionForPath(path),
      ...(isDark ? [oneDark] : []),
    ],
    [path, isDark],
  );

  return (
    <Box data-testid="quick-view-text" sx={textHostSx}>
      <CodeMirror
        value={text}
        height="100%"
        editable={false}
        readOnly
        theme={isDark ? 'dark' : 'light'}
        extensions={extensions}
        basicSetup={false}
      />
    </Box>
  );
};
