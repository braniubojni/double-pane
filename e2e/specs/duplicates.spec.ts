import fs from "node:fs";
import path from "node:path";
import { test, expect, type Page } from "@playwright/test";
import { waitAppReady, openFileMenu, refresh, expectRowVisible, LEFT_DIR } from "../fixtures/app";

const TWIN = "identical-bytes\n";
const OTHER = "different-bytes\n";

function writeDupDir(dir: string, withSkipMd: boolean) {
  fs.mkdirSync(dir, { recursive: true });
  fs.writeFileSync(path.join(dir, "same-a.txt"), TWIN);
  fs.writeFileSync(path.join(dir, "same-b.txt"), TWIN);
  fs.writeFileSync(path.join(dir, "other.txt"), OTHER);
  if (withSkipMd) {
    fs.writeFileSync(path.join(dir, "skip.md"), TWIN);
  }
}

async function goToLeft(page: Page, dir: string) {
  const input = page.getByTestId("path-input-left").locator("input");
  await input.fill(dir);
  await input.press("Enter");
  await expect(page.getByTestId("status-path")).toContainText(dir, { timeout: 10_000 });
}

async function openFindDuplicates(page: Page) {
  await openFileMenu(page);
  await page.getByTestId("menu-file-duplicates").click();
  await expect(page.getByTestId("dialog-duplicates")).toBeVisible();
  await expect(page.getByTestId("dup-setup")).toBeVisible();
}

async function startScan(page: Page) {
  await expect(page.getByTestId("dup-estimate")).toBeVisible({ timeout: 15_000 });
  await page.getByTestId("btn-dup-start").click();
  await expect(page.getByTestId("dup-review")).toBeVisible({ timeout: 20_000 });
}

test.describe("duplicates", () => {
  test.beforeEach(async ({ page }) => {
    await waitAppReady(page);
    await page.getByTestId("pane-left").click();
  });

  test("finds exact twins, merges extras, and undoes when available", async ({ page }) => {
    const dir = path.join(LEFT_DIR, `dups-merge-${Date.now()}`);
    writeDupDir(dir, false);
    await goToLeft(page, dir);
    await refresh(page);
    await expectRowVisible(page, "left", "same-a.txt");

    await openFindDuplicates(page);
    await startScan(page);

    await expect(page.getByTestId("dup-group")).toHaveCount(1);
    const review = page.getByTestId("dup-review");
    await expect(review).toContainText("same-a.txt");
    await expect(review).toContainText("same-b.txt");
    await expect(review).not.toContainText("other.txt");

    await page.getByTestId("btn-dup-merge").click();
    await expect(page.getByTestId("dup-merging")).toBeVisible();
    await page.getByTestId("btn-dup-merge-confirm").click();
    await expect(page.getByTestId("dup-done")).toBeVisible({ timeout: 15_000 });

    expect(fs.existsSync(path.join(dir, "same-a.txt"))).toBeTruthy();
    expect(fs.existsSync(path.join(dir, "same-b.txt"))).toBeFalsy();
    expect(fs.existsSync(path.join(dir, "other.txt"))).toBeTruthy();

    const undo = page.getByTestId("btn-dup-undo");
    if ((await undo.count()) > 0) {
      await undo.click();
      await expect(page.getByTestId("snackbar")).toContainText("Delete undone", { timeout: 10_000 });
      expect(fs.existsSync(path.join(dir, "same-b.txt"))).toBeTruthy();
      expect(fs.readFileSync(path.join(dir, "same-b.txt"), "utf8")).toBe(TWIN);
    }
  });

  test("exclude glob drops skip.md from estimate and review", async ({ page }) => {
    const dir = path.join(LEFT_DIR, `dups-exclude-${Date.now()}`);
    writeDupDir(dir, true);
    await goToLeft(page, dir);
    await refresh(page);
    await expectRowVisible(page, "left", "skip.md");

    await openFindDuplicates(page);
    await page.getByTestId("input-dup-exclude").locator("input").fill("*.md");
    await page.getByTestId("btn-dup-estimate").click();
    await expect(page.getByTestId("dup-estimate")).toContainText("3 files", { timeout: 15_000 });
    await expect(page.getByTestId("dup-estimate")).not.toContainText("skip.md");

    await page.getByTestId("btn-dup-start").click();
    await expect(page.getByTestId("dup-review")).toBeVisible({ timeout: 20_000 });

    await expect(page.getByTestId("dup-group")).toHaveCount(1);
    const review = page.getByTestId("dup-review");
    await expect(review).toContainText("same-a.txt");
    await expect(review).toContainText("same-b.txt");
    await expect(review).not.toContainText("skip.md");
  });
});
