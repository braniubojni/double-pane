import { useEffect, useRef } from 'react';
import { useSettings } from '../../../entities/file/queries';
import { isUpdateCheckDue } from '../helpers';
import { useUpdateActions } from './useUpdateActions';

/** Background update check after app is ready (every N days when enabled). */
export const useAutoUpdateCheck = (ready: boolean) => {
  const { data: settings } = useSettings();
  const { check } = useUpdateActions();
  const ran = useRef(false);

  useEffect(() => {
    if (!ready || !settings || ran.current) return;
    if (!isUpdateCheckDue(settings)) return;
    ran.current = true;

    const t = setTimeout(() => {
      void check();
    }, 2500);

    return () => clearTimeout(t);
  }, [ready, settings, check]);
};
