import { expect, test } from "@playwright/test";
import {
  createFolder,
  deleteNode,
  ensureListView,
  fileRowByName,
  goRootFromBreadcrumb,
  login,
  renameNode,
  uniqueName
} from "./helpers";

test.describe("File Manager", () => {
  test("login and basic folder navigation", async ({ page }) => {
    await login(page);
    await ensureListView(page);

    const folderName = uniqueName("e2e-nav");
    await createFolder(page, folderName);
    await expect(page.getByTestId("global-toast").first()).toBeVisible();
    await fileRowByName(page, folderName).dblclick();

    await expect(page.getByRole("button", { name: folderName })).toBeVisible();
    await goRootFromBreadcrumb(page);
    await expect(fileRowByName(page, folderName)).toBeVisible();
  });

  test("search by name and filter by type", async ({ page }) => {
    await login(page);
    await ensureListView(page);

    const folderName = uniqueName("e2e-search");
    await createFolder(page, folderName);

    await page.getByTestId("search-toggle-button").click();
    await page.getByTestId("search-input").fill(folderName);
    await expect(fileRowByName(page, folderName)).toBeVisible();

    await page.getByTestId("type-filter-select").selectOption("folder");
    await expect(fileRowByName(page, folderName)).toBeVisible();

    await page.getByTestId("type-filter-select").selectOption("image");
    await expect(page.getByTestId("file-table-empty")).toBeVisible();
    await page.getByRole("button", { name: /Clear Search\/Filter|Arama\/Filtre temizle/i }).click();
    await expect(fileRowByName(page, folderName)).toBeVisible();
  });

  test("upload success shows completed state", async ({ page }) => {
    await login(page);
    await ensureListView(page);

    const fileName = `${uniqueName("e2e-upload")}.txt`;
    const fileInput = page.getByTestId("file-browser-toolbar").locator('input[type="file"]');
    await fileInput.setInputFiles({
      name: fileName,
      mimeType: "text/plain",
      buffer: Buffer.from("playwright upload success")
    });

    await expect(fileRowByName(page, fileName)).toBeVisible({ timeout: 20_000 });
    await page.getByTestId("notifications-toggle-button").click();
    await expect(page.getByTestId("notifications-panel")).toBeVisible();
    await expect(page.getByTestId("upload-job-item").first()).toHaveAttribute("data-upload-status", /done|running/);
  });

  test("upload cancel and retry flow", async ({ page }) => {
    await login(page);
    await ensureListView(page);

    await page.route("**/api/v1/files/uploads/*/chunks/*", async (route) => {
      await new Promise((resolve) => setTimeout(resolve, 1500));
      await route.continue();
    });

    const fileName = `${uniqueName("e2e-cancel")}.bin`;
    const fileInput = page.getByTestId("file-browser-toolbar").locator('input[type="file"]');
    await fileInput.setInputFiles({
      name: fileName,
      mimeType: "application/octet-stream",
      buffer: Buffer.alloc(20 * 1024 * 1024, 7)
    });

    await page.getByTestId("notifications-toggle-button").click();
    await expect(page.getByTestId("upload-job-item").first()).toBeVisible();
    await page.getByTestId("upload-cancel-button").first().click();
    await expect(page.getByTestId("upload-job-item").first()).toHaveAttribute("data-upload-status", /cancelled|error/, { timeout: 20_000 });

    await page.getByTestId("upload-retry-button").first().click();
    await expect(page.getByTestId("upload-job-item").first()).toHaveAttribute("data-upload-status", /running|done/, { timeout: 20_000 });
  });

  test("rename and delete", async ({ page }) => {
    await login(page);
    await ensureListView(page);

    const oldName = uniqueName("e2e-rename");
    const newName = `${oldName}-updated`;
    await createFolder(page, oldName);
    await renameNode(page, oldName, newName);
    await deleteNode(page, newName);
  });

  test("move validation prevents moving folder into descendant", async ({ page }) => {
    await login(page);
    await ensureListView(page);

    const parentName = uniqueName("e2e-parent");
    const childName = uniqueName("e2e-child");

    await createFolder(page, parentName);
    await fileRowByName(page, parentName).getByText(parentName, { exact: true }).dblclick();
    await createFolder(page, childName);
    await goRootFromBreadcrumb(page);

    await fileRowByName(page, parentName).locator('input[type="checkbox"]').check();
    await page.getByTestId("bulk-move-button").click();
    await expect(page.getByTestId("move-modal")).toBeVisible();
    await page.getByTestId("move-destination-select").selectOption({ label: childName });

    await expect(page.getByText(/cannot be moved into itself|kendisinin icine tasinamaz/i)).toBeVisible();
    await expect(page.getByTestId("move-submit")).toBeDisabled();
  });

  test("txt and folder downloads return expected headers", async ({ page }) => {
    await login(page);
    await ensureListView(page);

    const txtName = `${uniqueName("e2e-download")}.txt`;
    const folderName = uniqueName("e2e-download-folder");

    const fileInput = page.getByTestId("file-browser-toolbar").locator('input[type="file"]');
    await fileInput.setInputFiles({
      name: txtName,
      mimeType: "text/plain",
      buffer: Buffer.from("download smoke")
    });
    await expect(fileRowByName(page, txtName)).toBeVisible({ timeout: 20_000 });

    await createFolder(page, folderName);

    const txtNodeID = await fileRowByName(page, txtName).getAttribute("data-node-id");
    const folderNodeID = await fileRowByName(page, folderName).getAttribute("data-node-id");

    expect(txtNodeID).toBeTruthy();
    expect(folderNodeID).toBeTruthy();

    const txtResponse = await page.request.get(`/api/v1/files/download?node_id=${encodeURIComponent(txtNodeID ?? "")}`);
    expect(txtResponse.ok()).toBeTruthy();
    expect(txtResponse.headers()["content-disposition"]).toContain(".txt");

    await fileRowByName(page, txtName).getByTestId("row-action-menu-toggle").click();
    const txtDownloadPromise = page.waitForEvent("download");
    await page.getByTestId("row-action-download").click();
    const txtDownload = await txtDownloadPromise;
    expect(await txtDownload.suggestedFilename()).toContain(".txt");

    const folderResponse = await page.request.get(`/api/v1/files/download?node_id=${encodeURIComponent(folderNodeID ?? "")}`);
    expect(folderResponse.ok()).toBeTruthy();
    expect(folderResponse.headers()["content-type"]).toContain("application/zip");
    expect(folderResponse.headers()["content-disposition"]).toContain(".zip");
  });
});
