import { expect, test } from "@playwright/test";
import { createFolder, ensureListView, fileRowByName, goRootFromBreadcrumb, login, uniqueName } from "./helpers";

test.describe("Sidebar + Shares + Sticky Layout", () => {
  test("share modal create/copy/revoke flow works", async ({ page }) => {
    await login(page);
    await ensureListView(page);

    const fileName = `${uniqueName("e2e-share-flow")}.txt`;
    const fileInput = page.getByTestId("file-browser-toolbar").locator('input[type="file"]');
    await fileInput.setInputFiles({
      name: fileName,
      mimeType: "text/plain",
      buffer: Buffer.from("share modal flow"),
    });
    await expect(fileRowByName(page, fileName)).toBeVisible({ timeout: 20_000 });

    await fileRowByName(page, fileName).getByText(fileName, { exact: true }).click();
    await page.getByTestId("share-create-button").click();
    await expect(page.getByTestId("share-modal")).toBeVisible();

    const createResponse = page.waitForResponse(
      (response) => response.url().includes("/api/v1/shares") && response.request().method() === "POST" && response.status() === 201
    );
    await page.getByTestId("share-create-submit").click();
    await createResponse;

    await expect(page.getByTestId("share-item").first()).toBeVisible();
    const linkInput = page.getByTestId("share-link-input").first();
    await expect(linkInput).toBeVisible();
    const shareLink = (await linkInput.inputValue()).trim();
    expect(shareLink).toMatch(/\/s\/[A-Za-z0-9_-]+$/);
    expect(shareLink).not.toContain("//s/");
    await page.getByTestId("share-copy-button").first().click();
    await page.goto(shareLink);
    await expect(page.getByRole("heading", { name: /Shared File|Paylasilan Dosya/i })).toBeVisible();
    await page.goto("/");
    await ensureListView(page);

    page.once("dialog", (dialog) => dialog.accept());
    await fileRowByName(page, fileName).getByText(fileName, { exact: true }).click();
    await page.getByTestId("share-create-button").click();
    await page.getByTestId("share-revoke-button").first().click();
    await expect(page.getByTestId("share-item")).toHaveCount(0);
  });

  test("shares page shows real node name, not node_id", async ({ page }) => {
    await login(page);
    await ensureListView(page);

    const fileName = `${uniqueName("e2e-shares-name")}.txt`;
    const fileInput = page.getByTestId("file-browser-toolbar").locator('input[type="file"]');
    await fileInput.setInputFiles({
      name: fileName,
      mimeType: "text/plain",
      buffer: Buffer.from("shares page metadata"),
    });
    await expect(fileRowByName(page, fileName)).toBeVisible({ timeout: 20_000 });

    await fileRowByName(page, fileName).getByText(fileName, { exact: true }).click();
    await page.getByTestId("share-create-button").click();
    await expect(page.getByTestId("share-modal")).toBeVisible();
    await page.getByTestId("share-create-submit").click();
    await expect(page.getByTestId("share-item").first()).toBeVisible();
    await page.keyboard.press("Escape");

    await page.getByTestId("sidebar-nav-shares").click();
    await expect(page).toHaveURL(/\/shares$/);
    await expect(page.getByTestId("share-row-node-name").first()).toContainText(fileName);
    await expect(page.getByTestId("share-row-node-name").first()).not.toHaveText(/[0-9a-fA-F-]{36}/);
  });

  test("nested sidebar tree expand/collapse lazy loads children", async ({ page }) => {
    await login(page);
    await ensureListView(page);

    const parentName = uniqueName("e2e-tree-parent");
    const childName = uniqueName("e2e-tree-child");

    await createFolder(page, parentName);
    await fileRowByName(page, parentName).getByText(parentName, { exact: true }).dblclick();
    await createFolder(page, childName);
    await goRootFromBreadcrumb(page);

    const parentRow = page.getByTestId("sidebar-tree-node").filter({ hasText: parentName }).first();
    await expect(parentRow).toBeVisible();
    await parentRow.getByTestId("sidebar-tree-toggle").click();

    const childRow = page.getByTestId("sidebar-tree-node").filter({ hasText: childName }).first();
    await expect(childRow).toBeVisible({ timeout: 10_000 });

    await parentRow.getByTestId("sidebar-tree-toggle").click();
    await expect(childRow).toHaveCount(0);
  });

  test("sticky sidebar and details remain visible while scrolling", async ({ page }) => {
    await login(page);
    await ensureListView(page);

    const baseName = uniqueName("e2e-sticky");
    for (let i = 0; i < 18; i += 1) {
      await createFolder(page, `${baseName}-${i}`);
    }

    const rows = page.getByTestId("file-row");
    const rowCount = await rows.count();
    await expect(rowCount).toBeGreaterThan(0);
    await rows.nth(rowCount - 1).getByTestId("row-action-menu-toggle").scrollIntoViewIfNeeded();
    await rows.nth(rowCount - 1).click();
    await expect(page.getByTestId("details-panel")).toBeVisible();

    const sidebarTopBefore = (await page.getByTestId("sidebar-panel").boundingBox())?.y ?? 0;
    const detailsTopBefore = (await page.getByTestId("details-panel").boundingBox())?.y ?? 0;

    const listScrollContainer = page.getByTestId("file-table-scroll-container");
    await listScrollContainer.evaluate((el) => {
      el.scrollTop = el.scrollHeight;
    });
    await page.waitForTimeout(250);

    const sidebarTopAfter = (await page.getByTestId("sidebar-panel").boundingBox())?.y ?? 0;
    const detailsTopAfter = (await page.getByTestId("details-panel").boundingBox())?.y ?? 0;

    expect(Math.abs(sidebarTopAfter - sidebarTopBefore)).toBeLessThan(4);
    expect(Math.abs(detailsTopAfter - detailsTopBefore)).toBeLessThan(4);
  });
});
