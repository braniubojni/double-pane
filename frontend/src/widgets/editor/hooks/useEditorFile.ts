import { useCallback, useEffect, useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { useArchivePasswordStore } from '../../../features/archive/archivePasswordStore';
import { isRemotePath } from '../../../features/connections/helpers';
import { useEditorStore } from '../../../features/editor/editorStore';
import { FileService, SettingsService } from '../../../shared/api/bindings';
import { errMessage } from '../../../shared/lib/format';
import { splitArchivePanePath } from '../../../shared/lib/archives';
import { useSnack } from '../../../shared/ui/SnackbarHost';

/** Matches the backend messages from filesystem/text.go and remote/ssh.go. */
const BINARY_RE = /binary or unsupported encoding/i;
const EXECUTABLE_RE = /executable file/i;
/** Matches filesystem.ErrPasswordRequired ("password required"). */
const PASSWORD_RE = /password required/i;

export const useEditorFile = () => {
  const filePath = useEditorStore((s) => s.filePath);
  const dirty = useEditorStore((s) => s.dirty);
  const setDirty = useEditorStore((s) => s.setDirty);
  const show = useSnack((s) => s.show);
  const qc = useQueryClient();
  const [content, setContent] = useState('');
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);

  if (!filePath && (content !== '' || loadError !== null)) {
    setContent('');
    setLoadError(null);
  }

  useEffect(() => {
    if (!filePath) return;
    let cancelled = false;
    const load = () => {
      setLoading(true);
      setLoadError(null);
      void FileService.ReadTextFile(filePath)
        .then((text) => {
          if (!cancelled) {
            setContent(text);
            setDirty(false);
          }
        })
        .catch((e) => {
          if (cancelled) return;
          const msg = errMessage(e);
          setContent('');
          // Executables are NEVER handed to the OS: "open" would run them.
          if (EXECUTABLE_RE.test(msg)) {
            setLoadError('Executable file — it cannot be edited.');
            show(msg, 'warning');
            return;
          }
          // The editor is text-only. Anything else it cannot decode (PDFs, images,
          // documents) goes to the OS default app instead of a dead-end error.
          // Remote files have no local copy to hand over, so they keep the error.
          if (BINARY_RE.test(msg) && !isRemotePath(filePath)) {
            setLoadError('Binary file — opened in your default app.');
            void SettingsService.OpenInOS(filePath).catch((openErr) =>
              show(errMessage(openErr), 'error'),
            );
            return;
          }
          // A password-protected archive member: prompt for the password and
          // retry once it's cached backend-side (SetArchivePassword).
          if (PASSWORD_RE.test(msg)) {
            const split = splitArchivePanePath(filePath);
            if (split) {
              setLoadError('Password required to open this archive.');
              useArchivePasswordStore.getState().prompt(split.archivePath, load);
              return;
            }
          }
          setLoadError(msg);
          show(msg, 'error');
        })
        .finally(() => {
          if (!cancelled) setLoading(false);
        });
    };
    load();
    return () => {
      cancelled = true;
    };
  }, [filePath, setDirty, show]);

  const onChange = useCallback(
    (value: string | undefined) => {
      setContent(value ?? '');
      setDirty(true);
    },
    [setDirty],
  );

  const save = useCallback(() => {
    if (!filePath || !dirty) return;
    void FileService.WriteTextFile(filePath, content)
      .then(() => {
        setDirty(false);
        show('Saved', 'success');
        const parent = filePath.replace(/\/+$/, '').split(/[/\\]/).slice(0, -1).join('/') || '/';
        void qc.invalidateQueries({ queryKey: ['dir', parent] });
        void qc.invalidateQueries({ queryKey: ['gitStatus', parent] });
      })
      .catch((e) => show(errMessage(e), 'error'));
  }, [filePath, dirty, content, setDirty, show, qc]);

  return { content, loading, loadError, onChange, save, dirty, filePath };
};
