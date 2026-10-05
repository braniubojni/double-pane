import fs from "node:fs";
import path from "node:path";
import { test, expect } from "@playwright/test";
import {
  waitAppReady,
  selectRow,
  expectRowVisible,
  doubleClickRow,
  openFileMenu,
  refresh,
  LEFT_DIR,
} from "../fixtures/app";

const PASSWORD = "correct horse battery staple";

test.describe("folder vault", () => {
  test.beforeEach(async ({ page }) => {
    await waitAppReady(page);
    await page.getByTestId("pane-left").click();
  });

  test("locks a folder, re-prompts on entry, unlocks, and re-locks", async ({ page }) => {
    const name = `vault-${Date.now()}`;
    const dir = path.join(LEFT_DIR, name);
    fs.mkdirSync(dir, { recursive: true });
    fs.writeFileSync(path.join(dir, "secret.txt"), "top secret contents\n");
    await refresh(page);
    await expectRowVisible(page, "left", name);

    // Lock with password…
    await selectRow(page, "left", name);
    await openFileMenu(page);
    await page.getByTestId("menu-file-vault-lock").click();
    await expect(page.getByTestId("dialog-vault-lock")).toBeVisible();
    await page.getByTestId("input-vault-lock-password").locator("input").fill(PASSWORD);
    await page.getByTestId("input-vault-lock-confirm").locator("input").fill(PASSWORD);
    await page.getByTestId("btn-vault-lock-confirm").click();
    await expect(page.getByTestId("dialog-vault-lock")).toBeHidden({ timeout: 10_000 });
    await expect(page.getByTestId("snackbar")).toContainText("Locked with password", {
      timeout: 20_000,
    });

    // On disk: ciphertext only, no plaintext sibling.
    expect(fs.existsSync(path.join(dir, ".dpvault", "manifest.json"))).toBeTruthy();
    expect(fs.existsSync(path.join(dir, "secret.txt"))).toBeFalsy();

    // Entering a locked vault row prompts Unlock instead of listing ciphertext.
    await refresh(page);
    await expectRowVisible(page, "left", name);
    await doubleClickRow(page, "left", name);
    await expect(page.getByTestId("dialog-vault-unlock")).toBeVisible();
    await page.getByTestId("input-vault-unlock-password").locator("input").fill(PASSWORD);
    await page.getByTestId("btn-vault-unlock-confirm").click();
    await expect(page.getByTestId("dialog-vault-unlock")).toBeHidden({ timeout: 10_000 });
    await expect(page.getByTestId("status-path")).toContainText(dir, { timeout: 10_000 });
    await expectRowVisible(page, "left", "secret.txt");

    // Lock now (no password needed while unlocked) — back at the parent listing.
    const parentInput = page.getByTestId("path-input-left").locator("input");
    await parentInput.fill(LEFT_DIR);
    await parentInput.press("Enter");
    await expect(page.getByTestId("status-path")).toContainText(LEFT_DIR, { timeout: 10_000 });
    await refresh(page);
    await selectRow(page, "left", name);
    await openFileMenu(page);
    await page.getByTestId("menu-file-vault-lock-now").click();
    await expect(page.getByTestId("snackbar")).toContainText("Locked", { timeout: 10_000 });

    // Re-entering after re-lock prompts Unlock again.
    await refresh(page);
    await doubleClickRow(page, "left", name);
    await expect(page.getByTestId("dialog-vault-unlock")).toBeVisible();
  });
});
