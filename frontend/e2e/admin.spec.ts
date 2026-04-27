import { expect, test } from "@playwright/test";
import { login } from "./helpers";

test("admin users page smoke", async ({ page }) => {
  await login(page);
  await expect(page.getByTestId("sidebar-nav-admin")).toHaveCount(0);
  await page.getByTestId("sidebar-user-menu-toggle").click();
  await expect(page.getByTestId("sidebar-user-menu-admin")).toBeVisible();
  await page.getByTestId("sidebar-user-menu-admin").click();

  await expect(page.getByRole("heading", { name: "Admin Panel" })).toBeVisible();
  await expect(page.getByText(/Kullanici Yonetimi|User Management/i)).toBeVisible();
  await expect(page.getByRole("button", { name: /Create User|Kullanici Olustur|New User/i })).toBeVisible();
});

test("admin audit list shows ip and client fields when present", async ({ page }) => {
  await page.route("**/api/v1/admin/audit**", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        events: [
          {
            event_type: "auth.login",
            severity: "info",
            user_id: "11111111-1111-1111-1111-111111111111",
            user_email: "admin@example.com",
            session_id: "22222222-2222-2222-2222-222222222222",
            ip: "128.1.1.199",
            user_agent: "Mozilla/5.0 Chrome/124.0.0.0 Safari/537.36",
            user_agent_short: "Chrome/124",
            device_type: "Desktop",
            browser: "Chrome",
            metadata: {},
            created_at: new Date().toISOString(),
            created_at_human: "24 Nis 2026 16:30:00"
          }
        ],
        pagination: {
          page: 1,
          limit: 20,
          total: 1,
          total_pages: 1
        }
      })
    });
  });

  await login(page);
  await page.goto("/admin");
  await page.getByTestId("admin-tab-audit").click();
  await expect(page.getByTestId("audit-ip")).toContainText("128.1.1.199");
  await expect(page.getByTestId("audit-device")).toContainText("Desktop");
  await expect(page.getByTestId("audit-browser")).toContainText("Chrome");
});

test("disabled user stays visible in filters and can be reactivated", async ({ page }) => {
  await page.route("**/api/v1/admin/users?status=all", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        users: [
          {
            id: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
            email: "pasif@example.com",
            display_name: "Pasif Kullanici",
            role: "user",
            quota_bytes: null,
            used_bytes: 0,
            limit_bytes: 21474836480,
            is_active: false,
            status: "deleted",
            deleted_at: new Date().toISOString(),
            created_at: new Date().toISOString(),
            updated_at: new Date().toISOString()
          }
        ]
      })
    });
  });

  await page.route("**/api/v1/admin/users?status=deleted", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        users: [
          {
            id: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
            email: "pasif@example.com",
            display_name: "Pasif Kullanici",
            role: "user",
            quota_bytes: null,
            used_bytes: 0,
            limit_bytes: 21474836480,
            is_active: false,
            status: "deleted",
            deleted_at: new Date().toISOString(),
            created_at: new Date().toISOString(),
            updated_at: new Date().toISOString()
          }
        ]
      })
    });
  });

  await page.route("**/api/v1/admin/users/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", async (route) => {
    if (route.request().method() === "PATCH") {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          user: {
            id: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
            email: "pasif@example.com",
            display_name: "Pasif Kullanici",
            role: "user",
            quota_bytes: null,
            used_bytes: 0,
            limit_bytes: 21474836480,
            is_active: true,
            status: "active",
            deleted_at: null,
            created_at: new Date().toISOString(),
            updated_at: new Date().toISOString()
          }
        })
      });
      return;
    }
    await route.continue();
  });

  await login(page);
  await page.goto("/admin");
  await page.getByTestId("admin-tab-users").click();
  await page.getByTestId("admin-users-filter-deleted").click();
  await expect(page.getByText("pasif@example.com")).toBeVisible();
  await expect(page.getByTestId("admin-user-reactivate")).toBeVisible();
  await expect(page.getByTestId("admin-user-permanent-delete")).toBeVisible();
});

test("admin general default language uses dropdown with EN/TR options", async ({ page }) => {
  await login(page);
  await page.goto("/admin");
  await page.getByTestId("admin-tab-general").click();
  const languageField = page.getByTestId("admin-settings-field-default-language");
  await expect(languageField).toBeVisible();
  const languageSelect = languageField.locator("select");
  await expect(languageSelect).toBeVisible();
  await expect(languageSelect.locator('option[value=\"en\"]')).toBeVisible();
  await expect(languageSelect.locator('option[value=\"tr\"]')).toBeVisible();
});
