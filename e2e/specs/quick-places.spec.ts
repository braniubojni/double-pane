import { test, expect } from "@playwright/test";
import { waitAppReady } from "../fixtures/app";
import { HOME_DIR } from "../paths";

test.describe("quick places", () => {
  test.beforeEach(async ({ page }) => {
    await waitAppReady(page);
  });

  test("opens a new tab at Home in the same pane", async ({ page }) => {
    const tabs = page.getByTestId("pane-left-tabs");
    await expect(tabs.getByRole("tab")).toHaveCount(1);

    await page.getByTestId("pane-left-quick-places").click();
    await page.getByTestId("pane-left-quick-place-Home").click();

    await expect(tabs.getByRole("tab")).toHaveCount(2);
    await expect(page.getByTestId("status-path")).toHaveText(HOME_DIR);
    await expect(page.getByTestId("pane-right-tabs").getByRole("tab")).toHaveCount(1);

    // Leave pane state as found for tests that run after this one.
    const homeTab = tabs.getByRole("tab").last();
    await homeTab.locator('[data-testid^="pane-left-tab-close-"]').click();
    await expect(tabs.getByRole("tab")).toHaveCount(1);
  });
});
