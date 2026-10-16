import { execSync } from "node:child_process";
import fs from "node:fs";
import path from "node:path";
import {
  CONFIG_DIR,
  E2E_ROOT,
  HOME_DIR,
  LEFT_DIR,
  REPO_ROOT,
  RIGHT_DIR,
  SERVER_BIN,
} from "./paths";

/** Ensure sandbox dirs, seed files, and settings exist (no full tree wipe — avoids races with the server). */
export function prepareSandbox() {
  fs.mkdirSync(LEFT_DIR, { recursive: true });
  fs.mkdirSync(RIGHT_DIR, { recursive: true });
  fs.mkdirSync(HOME_DIR, { recursive: true });
  fs.mkdirSync(CONFIG_DIR, { recursive: true });

  fs.mkdirSync(path.join(LEFT_DIR, "docs"), { recursive: true });
  if (!fs.existsSync(path.join(LEFT_DIR, "note.txt"))) {
    fs.writeFileSync(path.join(LEFT_DIR, "note.txt"), "hello from e2e\n");
  }
  if (!fs.existsSync(path.join(LEFT_DIR, "report.pdf"))) {
    fs.writeFileSync(path.join(LEFT_DIR, "report.pdf"), "pdf-bytes");
  }
  if (!fs.existsSync(path.join(LEFT_DIR, ".secret"))) {
    fs.writeFileSync(path.join(LEFT_DIR, ".secret"), "hidden");
  }
  if (!fs.existsSync(path.join(LEFT_DIR, "docs", "readme.md"))) {
    fs.writeFileSync(path.join(LEFT_DIR, "docs", "readme.md"), "# docs\n");
  }
  if (!fs.existsSync(path.join(RIGHT_DIR, ".keep"))) {
    fs.writeFileSync(path.join(RIGHT_DIR, ".keep"), "");
  }

  // Do NOT delete app.db/app.key while a previous server may hold the file open
  // (Playwright can start webServer before/around globalSetup). Write settings.json;
  // SettingsService re-imports it into encrypted app.db on next GetSettings.
  fs.writeFileSync(
    path.join(CONFIG_DIR, "settings.json"),
    JSON.stringify(
      {
        theme: "dark",
        showHidden: false,
        showExtensions: true,
        leftPath: LEFT_DIR,
        rightPath: RIGHT_DIR,
      },
      null,
      2,
    ) + "\n",
  );
}

export function buildApp() {
  // Always rebuild so e2e picks up latest FE/BE changes (dist/bin caches go stale).
  console.log("[e2e] Building frontend…");
  execSync("bun run build", {
    cwd: path.join(REPO_ROOT, "frontend"),
    stdio: "inherit",
  });

  console.log("[e2e] Building server binary…");
  execSync(`go build -tags server -o ${SERVER_BIN} .`, {
    cwd: REPO_ROOT,
    stdio: "inherit",
  });
}

/**
 * Prepare sandbox + build before webServer starts.
 * Does not delete the workspace (server process may already hold DB locks).
 */
export default async function globalSetup() {
  console.log("[e2e] Preparing workspace at", E2E_ROOT);
  prepareSandbox();
  buildApp();
  console.log("[e2e] Ready");
}

if (process.argv[1]?.endsWith("global-setup.ts")) {
  globalSetup().catch((err) => {
    console.error(err);
    process.exit(1);
  });
}
