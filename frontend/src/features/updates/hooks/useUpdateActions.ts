import { useCallback, useState } from 'react';
import { usePatchSettings } from '../../../entities/file/queries';
import { UpdateService } from '../../../shared/api/bindings';
import { errMessage } from '../../../shared/lib/format';
import { hostOS } from '../../../shared/lib/hostOS';
import { useSnack } from '../../../shared/ui/SnackbarHost';

/** Triggers Wails app.Updater (builtin window) and open-releases helpers. */
export const useUpdateActions = () => {
  const patch = usePatchSettings();
  const show = useSnack((s) => s.show);
  const [busy, setBusy] = useState(false);

  const markChecked = useCallback(() => {
    patch.mutate({ lastUpdateCheckAt: new Date().toISOString() });
  }, [patch]);

  const check = useCallback(async () => {
    setBusy(true);
    try {
      if (hostOS() === 'darwin') {
        const version = await UpdateService.CheckForUpdate();
        if (version) {
          await UpdateService.OpenReleasesPage();
          show(`Update v${version} available — opened the releases page`, 'info');
        } else {
          show('Already up to date', 'info');
        }
      } else {
        await UpdateService.CheckAndInstall();
      }
      markChecked();
    } catch (e) {
      show(errMessage(e), 'error');
    } finally {
      setBusy(false);
    }
  }, [markChecked, show]);

  const openReleases = useCallback(() => {
    void UpdateService.OpenReleasesPage().catch((e) => show(errMessage(e), 'error'));
  }, [show]);

  return { check, openReleases, busy };
};
