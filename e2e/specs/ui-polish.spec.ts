import path from "node:path";
import { test, expect } from "@playwright/test";
import {
  waitAppReady,
  doubleClickRow,
  expectRowVisible,
  selectRow,
  refresh,
  LEFT_DIR,
} from "../fixtures/app";

test.describe("ui polish", () => {
  test.beforeEach(async ({ page }) => {
    await waitAppReady(page);
  });

  test("path breadcrumbs navigate to parent segment", async ({ page }) => {
    await doubleClickRow(page, "left", "docs");
    await expect(page.getByTestId("status-path")).toContainText("docs");
    await expectRowVisible(page, "left", "readme.md");

    const crumbs = page.getByTestId("path-crumbs-left");
    await expect(crumbs).toBeVisible();
    const expand = crumbs.getByLabel("Show path");
    if ((await expand.count()) > 0) {
      await expand.click();
    }

    const parentIdx = path.join(LEFT_DIR, "docs").split(/[/\\]/).filter(Boolean).length - 1;
    await crumbs.getByTestId(`path-crumb-left-${parentIdx}`).click();

    await expect(page.getByTestId("status-path")).toContainText(LEFT_DIR);
    await expectRowVisible(page, "left", "docs");
  });

  test("status bar shows selection size and free disk space", async ({ page }) => {
    await selectRow(page, "left", "note.txt");
    const selected = page.getByTestId("status-selected");
    await expect(selected).toContainText("Selected: 1");
    await expect(selected).toHaveText(/Selected: 1 \([^)]*(B|KB)\)/);
    await expect(page.getByTestId("status-free")).toBeVisible();
    await expect(page.getByTestId("status-free")).toContainText("Free:");
  });

  test("hide Type on left via column menu; right still shows it after refresh", async ({
    page,
  }) => {
    const left = page.getByTestId("file-grid-left");
    const right = page.getByTestId("file-grid-right");
    const typeHeader = (grid: ReturnType<typeof page.getByTestId>) =>
      grid.locator('[role="columnheader"][data-field="ext"]');

    await expect(typeHeader(left)).toBeVisible();
    await expect(typeHeader(right)).toBeVisible();

    try {
      const header = typeHeader(left);
      await header.hover();
      await header.getByRole("button", { name: /menu/i }).click();
      await page.getByRole("menuitem", { name: "Hide column" }).click();

      await expect(typeHeader(left)).toHaveCount(0);
      await expect(typeHeader(right)).toBeVisible();

      await refresh(page);
      await expect(typeHeader(left)).toHaveCount(0);
      await expect(typeHeader(right)).toBeVisible();
    } finally {
      if ((await typeHeader(left).count()) === 0) {
        const nameHeader = left.locator('[role="columnheader"][data-field="displayName"]');
        await nameHeader.hover();
        await nameHeader.getByRole("button", { name: /menu/i }).click();
        await page.getByRole("menuitem", { name: "Manage columns" }).click();
        await page.getByRole("checkbox", { name: "Type" }).check();
        await page.keyboard.press("Escape");
        await expect(typeHeader(left)).toBeVisible();
        await page.waitForTimeout(400);
      }
    }
  });

  test("context menu shows Open with for a local file", async ({ page }) => {
    await page.getByTestId("file-grid-left").getByText("note.txt", { exact: true }).click({
      button: "right",
    });
    await expect(page.getByTestId("file-context-menu")).toBeVisible();
    await expect(page.getByTestId("ctx-open-with")).toBeVisible();
  });

  test("Open with closes on Escape and the row accepts the next click", async ({ page }) => {
    const grid = page.getByTestId("file-grid-left");
    const row = grid.locator(".MuiDataGrid-row").filter({ hasText: /\bnote\b/ }).first();
    await row.click({ button: "right" });
    await expect(page.getByTestId("file-context-menu")).toBeVisible();
    await page.getByTestId("ctx-open-with").click();
    await page.keyboard.press("Escape");
    await expect(page.getByTestId("file-context-menu")).toBeHidden();
    await expect(page.locator(".MuiModal-root")).toHaveCount(0);
    await row.click();
    await expect(page.getByTestId("file-context-menu")).toBeHidden();
    await expect(row).toHaveClass(/row-selected/);
  });

  test("command palette opens with Mod+K and runs delete", async ({ page }) => {
    await selectRow(page, "left", "note.txt");
    const palette = page.getByTestId("dialog-command-palette");
    const pathInput = page.getByTestId("path-input-left").locator("input");
    await pathInput.click();
    await page.keyboard.press("Meta+k");
    if (!(await palette.isVisible())) {
      await page.keyboard.press("Control+k");
    }
    await expect(palette).toBeVisible();
    await page.getByTestId("input-command-palette").locator("input").fill("delete");
    await page.keyboard.press("Enter");
    await expect(page.getByTestId("dialog-delete")).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(page.getByTestId("dialog-delete")).toBeHidden();
  });
});
