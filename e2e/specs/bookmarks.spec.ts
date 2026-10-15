import fs from "node:fs";
import path from "node:path";
import { test, expect } from "@playwright/test";
import { waitAppReady, LEFT_DIR } from "../fixtures/app";

test.describe("bookmarks", () => {
  test.beforeEach(async ({ page }) => {
    await waitAppReady(page);
  });

  test("bookmarks current path and can jump to it", async ({ page }) => {
    // Unique folder so UNIQUE(path) never collides across runs
    const folder = `bm-${Date.now()}`;
    const full = path.join(LEFT_DIR, folder);
    fs.mkdirSync(full, { recursive: true });
    fs.writeFileSync(path.join(full, "marker.txt"), "x");

    await page.getByTestId("btn-refresh").click();
    await page.getByTestId("pane-left").click();
    const input = page.getByTestId("path-input-left").locator("input");
    await input.fill(full);
    await input.press("Enter");
    await expect(page.getByTestId("status-path")).toContainText(folder);

    await page.getByTestId("input-bookmarks").locator("input").click();
    await page.getByTestId("option-bookmark-add").click();
    await expect(page.getByTestId("snackbar")).toContainText("completed", { timeout: 10_000 });

    // Navigate back to left root
    await input.fill(LEFT_DIR);
    await input.press("Enter");
    await expect(page.getByTestId("status-path")).toContainText(LEFT_DIR);

    await page.getByTestId("input-bookmarks").locator("input").click();
    await expect(page.getByText("Local", { exact: true })).toBeVisible();
    await page
      .getByRole("option", { name: new RegExp(folder) })
      .first()
      .click();
    await expect(page.getByTestId("status-path")).toContainText(folder);
  });

  test("bookmark path tooltip does not cover the remove button", async ({ page }) => {
    const folder = `tip-${Date.now()}`;
    const full = path.join(LEFT_DIR, folder);
    fs.mkdirSync(full, { recursive: true });

    await page.getByTestId("btn-refresh").click();
    await page.getByTestId("pane-left").click();
    const input = page.getByTestId("path-input-left").locator("input");
    await input.fill(full);
    await input.press("Enter");
    await expect(page.getByTestId("status-path")).toContainText(folder);

    await page.getByTestId("input-bookmarks").locator("input").click();
    await page.getByTestId("option-bookmark-add").click();
    await expect(page.getByTestId("snackbar")).toContainText("completed", { timeout: 10_000 });

    await page.getByTestId("input-bookmarks").locator("input").click();
    const option = page.getByRole("option", { name: new RegExp(folder) }).first();
    await option.locator("p").first().hover();
    const tip = page.getByRole("tooltip");
    await expect(tip).toBeVisible();
    await expect(tip).toContainText(full);

    const remove = option.getByRole("button", { name: /^remove-bookmark-/ });
    const tipBox = await tip.boundingBox();
    const removeBox = await remove.boundingBox();
    expect(tipBox).toBeTruthy();
    expect(removeBox).toBeTruthy();
    const overlaps =
      tipBox!.x < removeBox!.x + removeBox!.width &&
      tipBox!.x + tipBox!.width > removeBox!.x &&
      tipBox!.y < removeBox!.y + removeBox!.height &&
      tipBox!.y + tipBox!.height > removeBox!.y;
    expect(overlaps).toBe(false);

    await remove.click();
    await expect(option).toHaveCount(0);
  });
});
