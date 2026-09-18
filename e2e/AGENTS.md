# e2e — agent memory

Parent: root `AGENTS.md`. Playwright tests against Wails **server** mode.

## Layout

```
e2e/
  playwright.config.ts
  global-setup.ts
  fixtures/app.ts
  specs/*.spec.ts     # bookmarks, file-ops, duplicates, keyboard-terminal, navigation, view-settings
  scripts/prepare-and-start.sh
  paths.ts
```

## Run

```bash
task test:e2e
# or: cd e2e && npm test
```

Separate `package.json` / lockfile from frontend. Own `node_modules`.

## Conventions

- Prefer stable `data-testid` hooks already used in specs.
- Don’t assume production `frontend/dist` layout from Vite hash names.
- Global setup starts app via project scripts — read `prepare-and-start.sh` before changing ports.

## Don’t

- Commit `test-results/` or `playwright-report/` (gitignored).
- Mix e2e deps into `frontend/package.json`.
