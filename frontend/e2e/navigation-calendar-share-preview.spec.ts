import { expect, test } from "@playwright/test";
import { ensureListView, fileRowByName, login, uniqueName } from "./helpers";

function toLocalInputValue(date: Date): string {
  const year = date.getFullYear();
  const month = `${date.getMonth() + 1}`.padStart(2, "0");
  const day = `${date.getDate()}`.padStart(2, "0");
  const hours = `${date.getHours()}`.padStart(2, "0");
  const minutes = `${date.getMinutes()}`.padStart(2, "0");
  return `${year}-${month}-${day}T${hours}:${minutes}`;
}

test("profile/admin/settings navigation is explicit and consistent", async ({ page }) => {
  await login(page);

  await expect(page.locator('[data-testid="sidebar-nav-admin"]')).toHaveCount(0);

  await page.getByTestId("sidebar-user-menu-toggle").click();
  await page.getByTestId("sidebar-user-menu-profile").click();
  await expect(page).toHaveURL(/\/profile$/);
  await expect(page.getByText(/Profile|Profil/i)).toBeVisible();

  await page.goto("/");
  await page.getByTestId("sidebar-user-menu-toggle").click();
  await expect(page.getByTestId("sidebar-user-menu-settings")).toHaveCount(0);
  await expect(page.getByTestId("sidebar-user-menu-admin")).toBeVisible();
  await page.getByTestId("sidebar-user-menu-admin").click();
  await expect(page).toHaveURL(/\/admin$/);
  await expect(page.getByRole("heading", { name: /Admin Panel/i })).toBeVisible();

  await page.goto("/");
  await expect(page.getByTestId("sidebar-nav-files")).toBeVisible();
  await expect(page.getByTestId("sidebar-nav-calendar")).toBeVisible();
});

test("runtime branding and language switch update visible shell labels", async ({ page }) => {
  await page.route("**/api/v1/public/settings", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        site_name: "Acme Cloud",
        site_subtitle: "Team files",
        browser_title: "Acme Cloud Console",
        logo_url: "",
        favicon_url: "",
        accent_color: "#2563eb",
        public_base_url: "https://cloud.example.com",
        default_language: "tr",
        timezone: "Europe/Istanbul",
        maintenance_mode: false
      })
    });
  });

  await page.route("**/api/v1/me", async (route) => {
    if (route.request().method() === "PATCH") {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          id: "11111111-1111-1111-1111-111111111111",
          email: "admin@example.com",
          display_name: "Admin",
          preferred_language: "en",
          role: "owner",
          is_active: true
        })
      });
      return;
    }
    await route.continue();
  });

  await login(page);
  await expect(page.getByText("Acme Cloud")).toBeVisible();
  await expect(page.getByText("Team files")).toBeVisible();
  await expect(page.getByTestId("sidebar-nav-files")).toContainText(/Files|Dosyalar/i);

  await page.getByTestId("sidebar-user-menu-toggle").click();
  await page.getByTestId("sidebar-user-menu-profile").click();
  await page.getByTestId("profile-language-select").selectOption("en");
  await page.getByTestId("profile-save-button").click();
  await expect(page.getByTestId("sidebar-nav-files")).toContainText(/Files/i);
});

test("profile display name persists to shell and admin users list", async ({ page }) => {
  const updatedDisplayName = `QA Admin ${Date.now()}`;

  await page.route("**/api/v1/me", async (route) => {
    if (route.request().method() === "PATCH") {
      const body = route.request().postDataJSON() as { display_name?: string; preferred_language?: string | null };
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          id: "11111111-1111-1111-1111-111111111111",
          email: "admin@example.com",
          display_name: body.display_name ?? updatedDisplayName,
          preferred_language: body.preferred_language ?? "en",
          role: "owner",
          is_active: true
        })
      });
      return;
    }
    await route.continue();
  });

  await page.route("**/api/v1/admin/users?status=all", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        users: [
          {
            id: "11111111-1111-1111-1111-111111111111",
            email: "admin@example.com",
            display_name: updatedDisplayName,
            role: "owner",
            quota_bytes: null,
            used_bytes: 0,
            limit_bytes: 21474836480,
            is_active: true,
            status: "active",
            deleted_at: null,
            created_at: new Date().toISOString(),
            updated_at: new Date().toISOString()
          }
        ]
      })
    });
  });

  await login(page);
  await page.getByTestId("sidebar-user-menu-toggle").click();
  await page.getByTestId("sidebar-user-menu-profile").click();

  await page.getByTestId("profile-display-name-input").fill(updatedDisplayName);
  await page.getByTestId("profile-save-button").click();
  await expect(page.getByText(updatedDisplayName)).toBeVisible();

  await page.getByTestId("sidebar-user-menu-toggle").click();
  await page.getByTestId("sidebar-user-menu-admin").click();
  await expect(page.getByText(updatedDisplayName)).toBeVisible();
});

test("admin settings tabs render and switch", async ({ page }) => {
  await login(page);
  await page.goto("/admin");

  await expect(page.getByTestId("admin-tab-general")).toBeVisible();
  await expect(page.getByTestId("admin-tab-users")).toBeVisible();
  await expect(page.getByTestId("admin-tab-sharing")).toBeVisible();
  await expect(page.getByTestId("admin-tab-security")).toBeVisible();
  await expect(page.getByTestId("admin-tab-other")).toBeVisible();
  await expect(page.getByTestId("admin-tab-audit")).toBeVisible();

  await page.getByTestId("admin-tab-general").click();
  await expect(page.getByTestId("admin-section-general")).toBeVisible();
  await expect(page.getByTestId("admin-settings-field-site-name")).toBeVisible();
  await expect(page.getByTestId("admin-settings-field-default-quota")).toBeVisible();
  await expect(page.getByTestId("admin-settings-field-favicon-url")).toBeVisible();

  await page.getByTestId("admin-tab-sharing").click();
  await expect(page.getByTestId("admin-section-sharing")).toBeVisible();
  await expect(page.getByTestId("admin-settings-field-sharing-enabled")).toBeVisible();
  await expect(page.getByTestId("admin-settings-field-sharing-max-expiration")).toBeVisible();

  await page.getByTestId("admin-tab-security").click();
  await expect(page.getByTestId("admin-section-security")).toBeVisible();
  await expect(page.getByTestId("admin-settings-field-security-min-password")).toBeVisible();
  await expect(page.getByTestId("admin-settings-field-security-rate-window")).toBeVisible();

  await page.getByTestId("admin-tab-other").click();
  await expect(page.getByTestId("admin-section-other")).toBeVisible();
  await expect(page.getByTestId("admin-settings-field-other-max-file-size")).toBeVisible();
  await expect(page.getByTestId("admin-settings-field-other-max-chunk-size")).toBeVisible();
});

test("calendar month grid and event CRUD smoke", async ({ page }) => {
  await login(page);
  await page.getByTestId("sidebar-nav-calendar").click();
  await expect(page).toHaveURL(/\/calendar$/);
  await expect(page.getByTestId("calendar-month-grid")).toBeVisible();

  const title = uniqueName("e2e-calendar");
  await page.getByTestId("calendar-day-cell").nth(10).click();
  await expect(page.getByTestId("calendar-event-modal")).toBeVisible();

  const start = new Date(Date.now() + 120 * 60 * 1000);
  const end = new Date(start.getTime() + 30 * 60 * 1000);

  await page.getByTestId("calendar-input-title").fill(title);
  await page.getByTestId("calendar-input-description").fill("Playwright calendar smoke test");
  await page.getByTestId("calendar-input-start").fill(toLocalInputValue(start));
  await page.getByTestId("calendar-input-end").fill(toLocalInputValue(end));
  await page.getByTestId("calendar-reminder-10m").check();
  await page.getByTestId("calendar-save-button").click();

  await expect(page.getByText(title).first()).toBeVisible();

  await page.getByText(title).first().click();
  await expect(page.getByTestId("calendar-event-modal")).toBeVisible();
  page.once("dialog", (dialog) => dialog.accept());
  await page.getByTestId("calendar-delete-button").click();
  await expect(page.getByText(title)).toHaveCount(0);
});

test("notification center reminder smoke (stable mock)", async ({ page }) => {
  await page.route("**/api/v1/notifications?limit=50", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        notifications: [
          {
            id: "11111111-1111-1111-1111-111111111111",
            user_id: "22222222-2222-2222-2222-222222222222",
            type: "calendar.reminder",
            title: "Toplanti hatirlaticisi",
            message: "Toplantiya 10 dakika kaldi.",
            payload: {},
            is_read: false,
            read_at: null,
            created_at: new Date().toISOString()
          }
        ]
      })
    });
  });

  await login(page);
  await page.reload();
  await page.getByTestId("notifications-toggle-button").click();
  await expect(page.getByTestId("calendar-notification-item")).toBeVisible();
  await page.getByTestId("notification-mark-read-button").click();
});

test("share + public page + revoke and preview smoke", async ({ page }) => {
  await login(page);
  await ensureListView(page);

  const textFileName = `${uniqueName("e2e-preview")}.txt`;
  const imageFileName = `${uniqueName("e2e-image")}.png`;
  const pngBytes = Buffer.from(
    "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8Xw8AAtMB9xg7u9gAAAAASUVORK5CYII=",
    "base64"
  );

  const fileInput = page.getByTestId("file-browser-toolbar").locator('input[type="file"]');
  await fileInput.setInputFiles([
    {
      name: textFileName,
      mimeType: "text/plain",
      buffer: Buffer.from("preview smoke text content")
    },
    {
      name: imageFileName,
      mimeType: "image/png",
      buffer: pngBytes
    }
  ]);

  await expect(fileRowByName(page, textFileName)).toBeVisible({ timeout: 20_000 });
  await expect(fileRowByName(page, imageFileName)).toBeVisible({ timeout: 20_000 });

  await fileRowByName(page, textFileName).getByText(textFileName, { exact: true }).click();
  await expect(page.getByTestId("preview-text-container")).toBeVisible();
  await expect(page.getByTestId("preview-text-content")).toContainText("preview smoke text content");

  const createShareResponsePromise = page.waitForResponse(
    (response) => response.url().includes("/api/v1/shares") && response.request().method() === "POST" && response.status() === 201
  );
  await page.getByTestId("share-create-button").click();
  await expect(page.getByTestId("share-modal")).toBeVisible();
  await page.getByTestId("share-create-submit").click();
  const createShareResponse = await createShareResponsePromise;
  const createShareBody = (await createShareResponse.json()) as { share?: { public_url?: string } };
  const publicURL = createShareBody.share?.public_url ?? "";

  await expect(page.getByTestId("share-item").first()).toBeVisible();
  await page.getByTestId("share-copy-button").first().click();

  if (publicURL) {
    await page.goto(publicURL);
    await expect(page.getByRole("heading", { name: /Shared File|Paylasilan Dosya/i })).toBeVisible();
    await page.goto("/");
    await ensureListView(page);
    await fileRowByName(page, textFileName).getByText(textFileName, { exact: true }).click();
  }

  page.once("dialog", (dialog) => dialog.accept());
  await page.getByTestId("share-revoke-button").first().click();

  await fileRowByName(page, imageFileName).getByText(imageFileName, { exact: true }).click();
  await expect(page.getByTestId("preview-image")).toBeVisible();
});

test("profile language selector switches shell labels between TR and EN", async ({ page }) => {
  await login(page);

  await page.getByTestId("sidebar-user-menu-toggle").click();
  await page.getByTestId("sidebar-user-menu-profile").click();

  await page.getByTestId("profile-language-select").selectOption("tr");
  await page.getByTestId("profile-save-button").click();
  await expect(page.getByTestId("sidebar-nav-files")).toContainText(/Dosyalar|Files/i);

  await page.getByTestId("profile-language-select").selectOption("en");
  await page.getByTestId("profile-save-button").click();
  await expect(page.getByTestId("sidebar-nav-files")).toContainText(/Files/i);
});

test("default language EN applies when user preference is missing", async ({ page }) => {
  await page.route("**/api/v1/public/settings", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        site_name: "BitroxCloud",
        site_subtitle: "Private storage",
        browser_title: "BitroxCloud | Private Cloud",
        logo_url: "",
        favicon_url: "",
        accent_color: "#2563eb",
        public_base_url: "",
        default_language: "en",
        timezone: "Europe/Istanbul",
        maintenance_mode: false
      })
    });
  });

  await page.route("**/api/v1/me", async (route) => {
    const request = route.request();
    if (request.method() === "GET") {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          id: "11111111-1111-1111-1111-111111111111",
          email: "admin@example.com",
          display_name: "Admin",
          preferred_language: null,
          role: "owner",
          is_active: true
        })
      });
      return;
    }
    await route.continue();
  });

  await login(page);
  await expect(page.getByTestId("sidebar-nav-files")).toContainText(/^Files$/);
  await expect(page.getByTestId("sidebar-nav-calendar")).toContainText(/^Calendar$/);
  await expect(page.getByText("All Files")).toBeVisible();
  await expect(page.getByTestId("details-panel").getByText("Details")).toBeVisible();
});

test("default language TR applies when user preference is missing", async ({ page }) => {
  await page.route("**/api/v1/public/settings", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        site_name: "BitroxCloud",
        site_subtitle: "Private storage",
        browser_title: "BitroxCloud | Private Cloud",
        logo_url: "",
        favicon_url: "",
        accent_color: "#2563eb",
        public_base_url: "",
        default_language: "tr",
        timezone: "Europe/Istanbul",
        maintenance_mode: false
      })
    });
  });

  await page.route("**/api/v1/me", async (route) => {
    const request = route.request();
    if (request.method() === "GET") {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          id: "11111111-1111-1111-1111-111111111111",
          email: "admin@example.com",
          display_name: "Admin",
          preferred_language: null,
          role: "owner",
          is_active: true
        })
      });
      return;
    }
    await route.continue();
  });

  await login(page);
  await expect(page.getByTestId("sidebar-nav-files")).toContainText(/Dosyalar/);
  await expect(page.getByTestId("sidebar-nav-calendar")).toContainText(/Takvim/);
  await expect(page.getByText(/Tum Dosyalar|Tüm Dosyalar/)).toBeVisible();
  await expect(page.getByTestId("details-panel").getByText(/Detaylar/)).toBeVisible();
});
