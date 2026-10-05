import Button from '@mui/material/Button';
import type { FC } from 'react';
import { useLockVault, useUnlockedVaults } from '../../entities/file/queries';
import { errMessage } from '../../shared/lib/format';
import { useSnack } from '../../shared/ui/SnackbarHost';

/** "Vault · N open" — click locks every currently-unlocked vault session. */
export const VaultStatusChip: FC = () => {
  const { data: vaults } = useUnlockedVaults();
  const lockVault = useLockVault();
  const show = useSnack((s) => s.show);
  const n = vaults?.length ?? 0;
  if (!n) return null;

  const lockAll = () => {
    for (const v of vaults ?? []) {
      lockVault.mutate(v.root, { onError: (e) => show(errMessage(e), 'error') });
    }
    show(n === 1 ? 'Vault locked' : `${n} vaults locked`, 'success');
  };

  return (
    <Button
      size="small"
      data-testid="status-vault-chip"
      onClick={lockAll}
      sx={{ textTransform: 'none', minWidth: 0, px: 1 }}
    >
      Vault · {n} open
    </Button>
  );
};
