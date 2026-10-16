#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT/e2e"

# Prepare sandbox + build (idempotent; safe if already done by globalSetup)
bun ./global-setup.ts

BIN="$ROOT/bin/gfm-e2e"
if [[ ! -x "$BIN" ]]; then
  echo "[e2e] server binary missing after prepare" >&2
  exit 1
fi

# paths.ts uses os.tmpdir()/gfm-e2e-workspace — mirror for shell
E2E_ROOT="${TMPDIR:-/tmp}/gfm-e2e-workspace"
# Strip trailing slash from TMPDIR if present
E2E_ROOT="$(python3 -c "import os,tempfile; print(os.path.join(tempfile.gettempdir(), 'gfm-e2e-workspace'))")"

export HOME="${HOME:-$E2E_ROOT/home}"
export GFM_CONFIG_DIR="${GFM_CONFIG_DIR:-$E2E_ROOT/config}"
export WAILS_SERVER_HOST="${WAILS_SERVER_HOST:-127.0.0.1}"
export WAILS_SERVER_PORT="${WAILS_SERVER_PORT:-18080}"

# Prefer env from Playwright webServer (already set)
echo "[e2e] Starting $BIN on $WAILS_SERVER_HOST:$WAILS_SERVER_PORT config=$GFM_CONFIG_DIR"
exec "$BIN"
