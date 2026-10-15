import { useEffect, useEffectEvent } from 'react';
import { useFileOpsStore, type FileOpsRequest } from '../../../features/file-ops/fileOpsStore';
import { runToolbarRequest } from '../helpers';
import type { ToolbarRequestHandlers } from '../types';

/**
 * When the file-ops store fires a request (keyboard shortcut / menu),
 * invoke the matching handler and consume the request.
 *
 * Latest handlers are read via an effect event so the effect only re-runs on `nonce`.
 */
export const useFileOpsRequest = (handlers: ToolbarRequestHandlers): void => {
  const nonce = useFileOpsStore((s) => s.nonce);
  const run = useEffectEvent((request: NonNullable<FileOpsRequest>) =>
    runToolbarRequest(request, handlers),
  );

  useEffect(() => {
    const { request, consume } = useFileOpsStore.getState();
    if (!request) return;
    run(request);
    consume();
  }, [nonce]);
};
