import { expect, type Locator, type Page } from "@playwright/test";

export const E2E_EMAIL = process.env.E2E_EMAIL ?? "admin@example.com";
export const E2E_PASSWORD = process.env.E2E_PASSWORD ?? "Passw0rd";

export function uniqueName(prefix: string): string {
  return `${prefix}-${Date.now()}-${Math.floor(Math.random() * 10000)}`;
}

export async function login(page: Page): Promise<void> {
  await page.goto("/");
  const authForm = page.getByTestId("auth-form");
  if (await authForm.isVisible().catch(() => false)) {
    await page.getByTestId("login-email-input").fill(E2E_EMAIL);
    await page.getByTestId("login-password-input").fill(E2E_PASSWORD);
    await page.getByTestId("login-submit-button").click();
  }
  await expect(page.getByTestId("file-browser-toolbar")).toBeVisible();
}

export async function ensureListView(page: Page): Promise<void> {
  await page.getByTestId("view-list-button").click();
  await expect(page.getByTestId("file-table")).toBeVisible();
}

export async function createFolder(page: Page, folderName: string): Promise<void> {
  await page.getByTestId("quick-actions-toggle").click();
  await page.getByTestId("quick-action-create-folder").click();
  await expect(page.getByTestId("create-folder-modal")).toBeVisible();
  await page.getByTestId("create-folder-input").fill(folderName);
  await page.getByTestId("create-folder-submit").click();
  await expect(page.getByTestId("create-folder-modal")).toBeHidden();
  await expect(fileRowByName(page, folderName)).toBeVisible();
}

export async function openFolderFromList(page: Page, folderName: string): Promise<void> {
  const row = fileRowByName(page, folderName);
  await expect(row).toBeVisible();
  await row.getByText(folderName, { exact: true }).dblclick();
}

export async function goRootFromBreadcrumb(page: Page): Promise<void> {
  await page.getByRole("button", { name: /All Files|Tum Dosyalar|Tüm Dosyalar|T?m Dosyalar/i }).first().click();
}

export function fileRowByName(page: Page, nodeName: string): Locator {
  return page.locator(`[data-testid="file-row"][data-node-name="${nodeName}"]`);
}

export async function openRowMenu(page: Page, nodeName: string): Promise<void> {
  const row = fileRowByName(page, nodeName);
  await expect(row).toBeVisible();
  await row.getByTestId("row-action-menu-toggle").click();
}

export async function renameNode(page: Page, oldName: string, newName: string): Promise<void> {
  await openRowMenu(page, oldName);
  await page.getByTestId("row-action-rename").click();
  await expect(page.getByTestId("rename-modal")).toBeVisible();
  await page.getByTestId("rename-input").fill(newName);
  await page.getByTestId("rename-submit").click();
  await expect(page.getByTestId("rename-modal")).toBeHidden();
  await expect(fileRowByName(page, newName)).toBeVisible();
}

export async function deleteNode(page: Page, nodeName: string): Promise<void> {
  await openRowMenu(page, nodeName);
  await page.getByTestId("row-action-delete").click();
  await expect(page.getByTestId("delete-modal")).toBeVisible();
  await page.getByTestId("delete-submit").click();
  await expect(page.getByTestId("delete-modal")).toBeHidden();
  await expect(fileRowByName(page, nodeName)).toHaveCount(0);
}
