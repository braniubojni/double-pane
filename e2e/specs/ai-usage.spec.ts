import { test, expect, type Locator } from "@playwright/test";
import { waitAppReady } from "../fixtures/app";

async function assertNoScrollport(el: Locator) {
  const box = await el.evaluate((node) => {
    const style = getComputedStyle(node);
    return {
      scrollHeight: node.scrollHeight,
      clientHeight: node.clientHeight,
      overflowY: style.overflowY,
    };
  });
  expect(box.scrollHeight).toBeLessThanOrEqual(box.clientHeight + 1);
  expect(box.overflowY === "auto" || box.overflowY === "scroll").toBe(false);
}

test.describe("ai usage popover", () => {
  test.beforeEach(async ({ page }) => {
    await waitAppReady(page);
  });

  test("expanded popover has no scrollbar", async ({ page }) => {
    test.setTimeout(90_000);
    await page.setViewportSize({ width: 1280, height: 800 });

    await page.getByTestId("btn-ai-usage").click();
    const popover = page.getByTestId("popover-ai-usage");
    const paper = popover.locator(".MuiPopover-paper");
    await expect(paper).toBeVisible();
    await expect(paper.getByRole("button", { name: "Refresh" })).toBeVisible();

    const rows = paper.locator("[data-testid^='ai-usage-row-']");
    await expect(rows.first()).toBeVisible({ timeout: 45_000 });

    const list = paper.getByTestId("ai-usage-list");
    const count = await rows.count();
    for (let i = 0; i < count; i++) {
      // Production builds omit MUI's ExpandMoreIcon test id; the chevron is the expand affordance.
      const chevron = rows.nth(i).locator("svg");
      if ((await chevron.count()) === 0) continue;
      await chevron.first().click();
      await assertNoScrollport(paper);
      await assertNoScrollport(list);
    }

    await assertNoScrollport(paper);
    await assertNoScrollport(list);

    const bounds = await paper.boundingBox();
    expect(bounds).not.toBeNull();
    expect(bounds!.y).toBeGreaterThanOrEqual(-1);
    expect(bounds!.y + bounds!.height).toBeLessThanOrEqual(801);
  });
});
