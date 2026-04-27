"use client";

import { type FormEvent, type ReactNode, useCallback, useEffect, useMemo, useState } from "react";
import {
  ApiError,
  createAdminUser,
  deleteAdminUser,
  fetchAdminSettings,
  fetchMe,
  listAdminAudit,
  listAdminUsers,
  login,
  logout,
  permanentlyDeleteAdminUser,
  resolveDisplayName,
  updateAdminSettingsSection,
  updateAdminUser,
  updateAdminUserPassword,
  type AdminAuditEvent,
  type AdminAuditPagination,
  type AdminSettings,
  type AdminUser,
  type AuthUser
} from "@/lib/api";
import { type AdminUserFormValues, bytesToQuotaGb, quotaGbToBytes } from "@/components/admin/types";
import { AuditList } from "@/components/admin/audit-list";
import { UserModal } from "@/components/admin/user-modal";
import { UsersTable } from "@/components/admin/users-table";
import { useAppContext } from "@/components/app-context-provider";
import { AuthView } from "@/components/file-manager/auth-view";
import { formatBytes } from "@/components/file-manager/helpers";
import { ThemeToggle } from "@/components/theme-toggle";
import { useToast } from "@/components/toast-provider";

type ModalState =
  | { open: false }
  | {
      open: true;
      mode: "create" | "edit";
      title: string;
      userID: string | null;
      initialValues: AdminUserFormValues;
    };

type AdminTab = "general" | "users" | "sharing" | "security" | "other" | "audit";

const defaultSettings: AdminSettings = {
  general: {
    site_name: "BitroxCloud",
    site_subtitle: "Private storage",
    browser_title: "BitroxCloud | Private Cloud",
    site_logo_url: "https://pb.dashboardicons.com/api/files/community_gallery/myyy4r7vdmreido/bitrocloud_ameyfihhth.png",
    favicon_url: "",
    brand_color: "#2563eb",
    default_storage_quota_bytes: 20 * 1024 * 1024 * 1024,
    default_language: "en",
    default_timezone: "Europe/Istanbul",
    public_base_url: "",
    maintenance_mode: false
  },
  sharing: {
    public_sharing_enabled: true,
    allow_password_protected_links: true,
    allow_expiration: true,
    default_expiration_days: 7,
    maximum_expiration_days: 90,
    allow_public_downloads: true,
    allow_folder_sharing: false,
    require_password_for_public_links: false
  },
  security: {
    minimum_password_length: 12,
    require_uppercase: true,
    require_lowercase: true,
    require_number: true,
    require_symbol: false,
    session_lifetime_minutes: 15,
    refresh_token_lifetime_minutes: 43200,
    two_factor_required: false,
    login_rate_limit_per_window: 10,
    login_rate_window_seconds: 60,
    trusted_proxy_note: ""
  },
  other: {
    storage_cleanup_interval_seconds: 60,
    upload_max_file_size_bytes: 107374182400,
    upload_max_chunk_size_bytes: 16777216,
    preview_generation_enabled: false,
    logging_level: "info",
    requires_restart_note: ""
  }
};

const emptyCreateForm: AdminUserFormValues = {
  email: "",
  displayName: "",
  role: "user",
  password: "",
  useDefaultQuota: true,
  quotaGb: "",
  isActive: true
};

const BYTES_IN_MB = 1024 * 1024;
const BYTES_IN_GB = 1024 * 1024 * 1024;

function toUnitInput(bytes: number, unitDivisor: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) {
    return "";
  }
  const value = bytes / unitDivisor;
  const rounded = Number.isInteger(value) ? String(value) : value.toFixed(2).replace(/\.?0+$/, "");
  return rounded;
}

function parsePositiveNumber(raw: string): number | null {
  const parsed = Number.parseFloat(raw);
  if (!Number.isFinite(parsed) || parsed <= 0) {
    return null;
  }
  return parsed;
}

type SettingsFieldProps = {
  label: string;
  helper: string;
  children: ReactNode;
  testID?: string;
};

function SettingsField({ label, helper, children, testID }: SettingsFieldProps) {
  return (
    <label className="space-y-1.5" data-testid={testID}>
      <span className="block text-sm font-medium text-[var(--text-main)]">{label}</span>
      {children}
      <p className="text-xs text-[var(--text-muted)]">{helper}</p>
    </label>
  );
}

function readCookie(name: string): string | null {
  if (typeof document === "undefined") {
    return null;
  }
  const token = `${name}=`;
  const parts = document.cookie.split(";");
  for (const rawPart of parts) {
    const part = rawPart.trim();
    if (part.startsWith(token)) {
      return decodeURIComponent(part.slice(token.length));
    }
  }
  return null;
}

function readCSRFToken(): string | null {
  return readCookie("__Host-cloud_csrf_token") ?? readCookie("cloud_csrf_token");
}

function mapUserToForm(user: AdminUser): AdminUserFormValues {
  return {
    email: user.email,
    displayName: resolveDisplayName(user),
    role: user.role === "owner" || user.role === "admin" ? user.role : "user",
    password: "",
    useDefaultQuota: user.quota_bytes === null,
    quotaGb: bytesToQuotaGb(user.quota_bytes),
    isActive: user.is_active
  };
}

function isAdminRole(role: string): boolean {
  return role === "owner" || role === "admin";
}

export function AdminPanel() {
  const { settings: publicSettings, refreshSettings, setLocale, t } = useAppContext();
  const { showToast } = useToast();
  const [authBusy, setAuthBusy] = useState(false);
  const [authChecked, setAuthChecked] = useState(false);
  const [currentUser, setCurrentUser] = useState<AuthUser | null>(null);
  const [email, setEmail] = useState("admin@example.com");
  const [password, setPassword] = useState("");

  const [usersLoading, setUsersLoading] = useState(false);
  const [settingsLoading, setSettingsLoading] = useState(false);
  const [auditLoading, setAuditLoading] = useState(false);
  const [settingsSaving, setSettingsSaving] = useState(false);

  const [users, setUsers] = useState<AdminUser[]>([]);
  const [userStatusFilter, setUserStatusFilter] = useState<"all" | "active" | "inactive" | "deleted">("all");
  const [settings, setSettings] = useState<AdminSettings>(defaultSettings);
  const [activeTab, setActiveTab] = useState<AdminTab>("users");
  const [auditEvents, setAuditEvents] = useState<AdminAuditEvent[]>([]);
  const [auditPage, setAuditPage] = useState(1);
  const [auditEventType, setAuditEventType] = useState("all");
  const [auditSearch, setAuditSearch] = useState("");
  const [auditPagination, setAuditPagination] = useState<AdminAuditPagination>({
    page: 1,
    limit: 20,
    total: 0,
    total_pages: 0
  });

  const [busyUserID, setBusyUserID] = useState<string | null>(null);
  const [modalBusy, setModalBusy] = useState(false);
  const [modalState, setModalState] = useState<ModalState>({ open: false });

  const [error, setError] = useState<string | null>(null);

  const canAccessAdmin = useMemo(() => (currentUser ? isAdminRole(currentUser.role) : false), [currentUser]);
  const adminBrandName = settings.general.site_name.trim() || publicSettings.site_name.trim() || "BitroxCloud";
  const tabItems = useMemo<Array<{ value: AdminTab; label: string; testID: string }>>(
    () => [
      { value: "general", label: t("admin.tabs.general", "General"), testID: "admin-tab-general" },
      { value: "users", label: t("admin.tabs.users", "Users"), testID: "admin-tab-users" },
      { value: "sharing", label: t("admin.tabs.sharing", "Sharing"), testID: "admin-tab-sharing" },
      { value: "security", label: t("admin.tabs.security", "Security"), testID: "admin-tab-security" },
      { value: "other", label: t("admin.tabs.other", "Other"), testID: "admin-tab-other" },
      { value: "audit", label: t("admin.tabs.audit", "Recent Activity"), testID: "admin-tab-audit" }
    ],
    [t]
  );

  const loadUsers = useCallback(async (status = userStatusFilter) => {
    setUsersLoading(true);
    try {
      setUsers(await listAdminUsers(status));
    } catch (err) {
      setError(err instanceof Error ? err.message : t("admin.users.loadFailed", "Users could not be loaded."));
    } finally {
      setUsersLoading(false);
    }
  }, [t, userStatusFilter]);

  const loadSettings = useCallback(async () => {
    setSettingsLoading(true);
    try {
      const loaded = await fetchAdminSettings();
      setSettings({
        ...defaultSettings,
        ...loaded,
        general: { ...defaultSettings.general, ...loaded.general },
        sharing: { ...defaultSettings.sharing, ...loaded.sharing },
        security: { ...defaultSettings.security, ...loaded.security },
        other: { ...defaultSettings.other, ...loaded.other }
      });
    } catch (err) {
      setError(err instanceof Error ? err.message : t("admin.settings.loadFailed", "Settings could not be loaded."));
    } finally {
      setSettingsLoading(false);
    }
  }, [t]);

  const loadAudit = useCallback(
    async (input?: { page?: number; eventType?: string; search?: string }) => {
      setAuditLoading(true);
      try {
        const response = await listAdminAudit({
          limit: auditPagination.limit,
          page: input?.page ?? auditPage,
          eventType: input?.eventType ?? auditEventType,
          search: input?.search ?? auditSearch
        });
        setAuditEvents(response.events);
        setAuditPagination(response.pagination);
      } catch (err) {
        setError(err instanceof Error ? err.message : t("admin.audit.loadFailed", "Audit records could not be loaded."));
      } finally {
        setAuditLoading(false);
      }
    },
    [auditEventType, auditPage, auditPagination.limit, auditSearch, t]
  );

  const loadAdminData = useCallback(async () => {
    setError(null);
    await Promise.all([loadUsers(), loadSettings(), loadAudit()]);
  }, [loadAudit, loadSettings, loadUsers]);

  useEffect(() => {
    let active = true;

    void (async () => {
      try {
        const me = await fetchMe();
        if (!active) {
          return;
        }
        setCurrentUser(me);
        if (isAdminRole(me.role)) {
          await loadAdminData();
        }
      } catch {
        if (active) {
          setCurrentUser(null);
        }
      } finally {
        if (active) {
          setAuthChecked(true);
        }
      }
    })();

    return () => {
      active = false;
    };
  }, [loadAdminData]);

  useEffect(() => {
    if (!canAccessAdmin) {
      return;
    }
    if (activeTab !== "users") {
      return;
    }
    void loadUsers(userStatusFilter);
  }, [activeTab, canAccessAdmin, loadUsers, userStatusFilter]);

  const handleLogin = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setAuthBusy(true);
    setError(null);
    try {
      const me = await login(email, password);
      setCurrentUser(me);
      setPassword("");
      if (!isAdminRole(me.role)) {
        setError(t("admin.accessRequiredDesc", "This account cannot access the admin panel."));
        return;
      }
      await loadAdminData();
    } catch (err) {
      setError(err instanceof Error ? err.message : t("auth.loginFailed", "Login failed."));
    } finally {
      setAuthBusy(false);
      setAuthChecked(true);
    }
  };

  const handleLogout = async () => {
    const csrf = readCSRFToken();
    if (!csrf) {
      setCurrentUser(null);
      return;
    }
    try {
      await logout(csrf);
    } catch {
      // no-op
    }
    setCurrentUser(null);
    setUsers([]);
    setAuditEvents([]);
  };

  const saveSettingsSection = async <K extends keyof AdminSettings>(section: K, value: AdminSettings[K]) => {
    setSettingsSaving(true);
    setError(null);
    try {
      const saved = await updateAdminSettingsSection(section, value);
      setSettings((prev) => ({ ...prev, [section]: saved }));
      showToast("success", t("admin.settings.saved", "Settings saved."));
      await refreshSettings();
      if (section === "general" && !window.localStorage.getItem("bitrox_locale")) {
        const nextLocale = (saved as AdminSettings["general"]).default_language === "tr" ? "tr" : "en";
        setLocale(nextLocale, false);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : t("admin.settings.saveFailed", "Settings could not be saved."));
    } finally {
      setSettingsSaving(false);
    }
  };

  if (!authChecked || !currentUser) {
    return (
      <>
        <AuthView
          email={email}
          password={password}
          loading={authBusy}
          onEmailChange={setEmail}
          onPasswordChange={setPassword}
          onSubmit={handleLogin}
        />
        {error ? <p className="-mt-6 px-4 pb-4 text-center text-sm text-red-500">{error}</p> : null}
      </>
    );
  }

  if (!canAccessAdmin) {
    return (
      <main className="min-h-screen bg-[var(--bg-main)] p-6">
        <div className="mx-auto max-w-3xl rounded-2xl border border-[var(--line)] bg-[var(--bg-card)] p-6">
          <h1 className="text-xl font-semibold text-[var(--text-main)]">{t("admin.accessRequired", "Admin Access Required")}</h1>
          <p className="mt-2 text-sm text-[var(--text-muted)]">{t("admin.accessRequiredDesc", "This account cannot access the admin panel.")}</p>
          <div className="mt-4 flex items-center gap-2">
            <a href="/" className="focus-ring rounded-lg border border-[var(--line)] px-3 py-2 text-sm hover:bg-[var(--bg-soft)]">
              {t("sidebar.files", "Files")}
            </a>
            <button
              type="button"
              onClick={() => void handleLogout()}
              className="focus-ring rounded-lg border border-red-500/40 px-3 py-2 text-sm text-red-500 hover:bg-red-500/10"
            >
              {t("user.logout", "Sign Out")}
            </button>
          </div>
        </div>
      </main>
    );
  }

  return (
    <main className="min-h-screen bg-[var(--bg-main)] p-4 lg:p-6">
      <div className="mx-auto max-w-7xl space-y-4">
        <header className="surface-card flex flex-wrap items-center justify-between gap-3 rounded-2xl px-4 py-3">
          <div>
            <p className="text-xs uppercase tracking-wide text-[var(--text-muted)]">{adminBrandName}</p>
            <h1 className="text-xl font-semibold text-[var(--text-main)]">{t("user.adminPanel", "Admin Panel")}</h1>
          </div>
          <div className="flex items-center gap-2">
            <a href="/" className="focus-ring rounded-lg border border-[var(--line)] px-3 py-2 text-sm hover:bg-[var(--bg-soft)]">
              {t("sidebar.files", "Files")}
            </a>
            <button
              type="button"
              onClick={() => void handleLogout()}
              className="focus-ring rounded-lg border border-red-500/40 px-3 py-2 text-sm text-red-500 hover:bg-red-500/10"
            >
              {t("user.logout", "Sign Out")}
            </button>
            <ThemeToggle />
          </div>
        </header>

        <nav className="surface-card flex flex-wrap items-center gap-2 rounded-2xl p-2">
          {tabItems.map((item) => (
            <button
              key={item.value}
              type="button"
              data-testid={item.testID}
              onClick={() => setActiveTab(item.value)}
              className={`focus-ring rounded-lg px-3 py-2 text-sm ${
                activeTab === item.value ? "bg-[var(--brand-soft)] text-[var(--brand)]" : "hover:bg-[var(--bg-soft)]"
              }`}
            >
              {item.label}
            </button>
          ))}
        </nav>

        {error ? <div className="rounded-xl border border-red-500/40 bg-red-500/10 px-4 py-2 text-sm text-red-500">{error}</div> : null}

        {activeTab === "users" ? (
          <section className="surface-card rounded-2xl p-4" data-testid="admin-section-users">
            <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
              <h2 className="text-base font-semibold text-[var(--text-main)]">{t("admin.users.title", "User Management")}</h2>
              <div className="flex items-center gap-2">
                <button
                  type="button"
                  onClick={() => void loadAdminData()}
                  className="focus-ring rounded-lg border border-[var(--line)] px-3 py-2 text-sm hover:bg-[var(--bg-soft)]"
                >
                  {t("common.refresh", "Refresh")}
                </button>
                <button
                  type="button"
                  onClick={() =>
                    setModalState({
                      open: true,
                      mode: "create",
                      title: t("admin.users.createUser", "Create User"),
                      userID: null,
                      initialValues: emptyCreateForm
                    })
                  }
                  className="focus-ring rounded-lg bg-[var(--brand)] px-3 py-2 text-sm font-semibold text-white hover:brightness-110"
                >
                  {t("admin.users.createUser", "Create User")}
                </button>
              </div>
            </div>
            <UsersTable
              users={users}
              loading={usersLoading}
              busyUserID={busyUserID}
              statusFilter={userStatusFilter}
              onChangeStatusFilter={(status) => {
                setUserStatusFilter(status);
                void loadUsers(status);
              }}
              onEdit={(user) =>
                setModalState({
                  open: true,
                  mode: "edit",
                  title: t("admin.users.editUser", "Edit User"),
                  userID: user.id,
                  initialValues: mapUserToForm(user)
                })
              }
              onResetPassword={(user) => {
                const suggestedPassword = `${Math.random().toString(36).slice(2, 8)}A1!${Date.now().toString().slice(-4)}`;
                const nextPassword = window.prompt(
                  t("admin.users.resetPasswordPrompt", "Enter a new temporary password (min 12 characters):"),
                  suggestedPassword
                );
                if (nextPassword === null) {
                  return;
                }
                if (nextPassword.trim().length < 12) {
                  setError(t("admin.validation.passwordMinLength", "Password must be at least 12 characters."));
                  return;
                }
                setBusyUserID(user.id);
                setError(null);
                void updateAdminUserPassword(user.id, nextPassword.trim())
                  .then(() => loadAudit())
                  .then(() => showToast("success", t("admin.users.passwordReset", "Password reset successfully.")))
                  .catch((err: unknown) => {
                    setError(err instanceof Error ? err.message : t("admin.users.passwordResetFailed", "Password could not be reset."));
                  })
                  .finally(() => setBusyUserID(null));
              }}
              onToggleActive={(user) => {
                const nextIsActive = !user.is_active;
                const previousUsers = users;
                setUsers((current) => current.map((item) => (item.id === user.id ? { ...item, is_active: nextIsActive } : item)));
                setBusyUserID(user.id);
                setError(null);
                void updateAdminUser({
                  userID: user.id,
                  isActive: nextIsActive
                })
                  .then(() => Promise.all([loadUsers(userStatusFilter), loadAudit()]))
                  .then(() => showToast("success", t("admin.users.statusUpdated", "User status updated.")))
                  .catch((err: unknown) => {
                    setUsers(previousUsers);
                    setError(err instanceof Error ? err.message : t("admin.users.statusUpdateFailed", "Status could not be updated."));
                  })
                  .finally(() => setBusyUserID(null));
              }}
              onReactivate={(user) => {
                const previousUsers = users;
                setUsers((current) => current.map((item) => (item.id === user.id ? { ...item, is_active: true, status: "active", deleted_at: null } : item)));
                setBusyUserID(user.id);
                setError(null);
                void updateAdminUser({
                  userID: user.id,
                  isActive: true
                })
                  .then(() => Promise.all([loadUsers(userStatusFilter), loadAudit()]))
                  .then(() => showToast("success", t("admin.users.reactivated", "User reactivated.")))
                  .catch((err: unknown) => {
                    setUsers(previousUsers);
                    setError(err instanceof Error ? err.message : t("admin.users.reactivateFailed", "User could not be reactivated."));
                  })
                  .finally(() => setBusyUserID(null));
              }}
              onDelete={(user) => {
                const approved = window.confirm(
                  t(
                    "admin.users.disableConfirm",
                    "{email} account will be disabled and cannot sign in.\nDo you want to continue?",
                    { email: user.email }
                  )
                );
                if (!approved) {
                  return;
                }
                const previousUsers = users;
                setUsers((current) =>
                  current.map((item) =>
                    item.id === user.id ? { ...item, is_active: false, status: "deleted", deleted_at: new Date().toISOString() } : item
                  )
                );
                setBusyUserID(user.id);
                setError(null);
                void deleteAdminUser(user.id)
                  .then(() => Promise.all([loadUsers(userStatusFilter), loadAudit()]))
                  .then(() => showToast("success", t("admin.users.disabled", "User disabled.")))
                  .catch((err: unknown) => {
                    setUsers(previousUsers);
                    setError(err instanceof Error ? err.message : t("admin.users.disableFailed", "User could not be disabled."));
                  })
                  .finally(() => setBusyUserID(null));
              }}
              onPermanentDelete={(user) => {
                const approved = window.confirm(
                  t(
                    "admin.users.permanentDeleteConfirm",
                    "This will permanently delete {email} and all owned files. This action cannot be undone.\nDo you want to continue?",
                    { email: user.email }
                  )
                );
                if (!approved) {
                  return;
                }
                const previousUsers = users;
                setUsers((current) => current.filter((item) => item.id !== user.id));
                setBusyUserID(user.id);
                setError(null);
                void permanentlyDeleteAdminUser(user.id)
                  .then(() => Promise.all([loadUsers(userStatusFilter), loadAudit()]))
                  .then(() => showToast("success", t("admin.users.permanentlyDeleted", "User permanently deleted.")))
                  .catch((err: unknown) => {
                    setUsers(previousUsers);
                    setError(err instanceof Error ? err.message : t("admin.users.permanentDeleteFailed", "User could not be permanently deleted."));
                  })
                  .finally(() => setBusyUserID(null));
              }}
            />
          </section>
        ) : null}

        {activeTab === "audit" ? (
          <section data-testid="admin-section-audit">
            <AuditList
              events={auditEvents}
              loading={auditLoading}
              page={auditPagination.page}
              totalPages={auditPagination.total_pages}
              totalItems={auditPagination.total}
              eventType={auditEventType}
              search={auditSearch}
              onEventTypeChange={(value) => {
                setAuditEventType(value);
                setAuditPage(1);
                void loadAudit({ page: 1, eventType: value });
              }}
              onSearchChange={(value) => {
                setAuditSearch(value);
                setAuditPage(1);
                void loadAudit({ page: 1, search: value });
              }}
              onPageChange={(value) => {
                setAuditPage(value);
                void loadAudit({ page: value });
              }}
            />
          </section>
        ) : null}

        {activeTab === "general" ? (
          <section className="surface-card rounded-2xl p-5" data-testid="admin-section-general">
            <h2 className="text-lg font-semibold">{t("admin.tabs.general", "General")}</h2>
            <div className="mt-4 grid gap-4 md:grid-cols-2">
              <SettingsField label={t("admin.settings.general.siteName", "Site name")} helper={t("admin.settings.general.siteNameHelp", "Brand name shown in sidebar/header/admin screens.")} testID="admin-settings-field-site-name">
                <input
                  value={settings.general.site_name}
                  onChange={(event) =>
                    setSettings((prev) => ({ ...prev, general: { ...prev.general, site_name: event.target.value } }))
                  }
                  className="focus-ring w-full rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-3 py-2 text-sm outline-none"
                  placeholder={t("admin.settings.general.siteNamePlaceholder", "Example: BitroxCloud")}
                />
              </SettingsField>

              <SettingsField
                label={t("admin.settings.general.siteSubtitle", "Site subtitle")}
                helper={t("admin.settings.general.siteSubtitleHelp", "Short description text shown under logo.")}
                testID="admin-settings-field-site-subtitle"
              >
                <input
                  value={settings.general.site_subtitle}
                  onChange={(event) =>
                    setSettings((prev) => ({ ...prev, general: { ...prev.general, site_subtitle: event.target.value } }))
                  }
                  className="focus-ring w-full rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-3 py-2 text-sm outline-none"
                  placeholder="Private storage"
                />
              </SettingsField>

              <SettingsField
                label={t("admin.settings.general.browserTitle", "Browser title")}
                helper={t("admin.settings.general.browserTitleHelp", "Title shown on browser tab.")}
                testID="admin-settings-field-browser-title"
              >
                <input
                  value={settings.general.browser_title}
                  onChange={(event) =>
                    setSettings((prev) => ({ ...prev, general: { ...prev.general, browser_title: event.target.value } }))
                  }
                  className="focus-ring w-full rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-3 py-2 text-sm outline-none"
                  placeholder="BitroxCloud"
                />
              </SettingsField>

              <SettingsField
                label={t("admin.settings.general.publicBaseUrl", "Public base URL")}
                helper={t("admin.settings.general.publicBaseUrlHelp", "Base URL for share links and callback URLs. Example: https://cloud.company.com")}
                testID="admin-settings-field-public-base-url"
              >
                <input
                  value={settings.general.public_base_url}
                  onChange={(event) =>
                    setSettings((prev) => ({ ...prev, general: { ...prev.general, public_base_url: event.target.value } }))
                  }
                  className="focus-ring w-full rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-3 py-2 text-sm outline-none"
                  placeholder="https://cloud.example.com"
                />
              </SettingsField>

              <SettingsField
                label={t("admin.settings.general.defaultQuotaGb", "Default user quota (GB)")}
                helper={t("admin.settings.general.defaultQuotaHelp", "Default quota assigned to new users. Current: {size}", { size: formatBytes(settings.general.default_storage_quota_bytes) })}
                testID="admin-settings-field-default-quota"
              >
                <input
                  type="number"
                  min={1}
                  step="0.5"
                  value={toUnitInput(settings.general.default_storage_quota_bytes, BYTES_IN_GB)}
                  onChange={(event) => {
                    const parsed = parsePositiveNumber(event.target.value);
                    if (parsed === null) {
                      return;
                    }
                    setSettings((prev) => ({
                      ...prev,
                      general: { ...prev.general, default_storage_quota_bytes: Math.round(parsed * BYTES_IN_GB) }
                    }));
                  }}
                  className="focus-ring w-full rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-3 py-2 text-sm outline-none"
                  placeholder="20"
                />
              </SettingsField>

              <SettingsField
                label={t("admin.settings.general.timezone", "Timezone")}
                helper={t("admin.settings.general.timezoneHelp", "Used for date/time formatting and scheduling defaults.")}
                testID="admin-settings-field-timezone"
              >
                <input
                  value={settings.general.default_timezone}
                  onChange={(event) =>
                    setSettings((prev) => ({ ...prev, general: { ...prev.general, default_timezone: event.target.value } }))
                  }
                  className="focus-ring w-full rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-3 py-2 text-sm outline-none"
                  placeholder="Europe/Istanbul"
                />
              </SettingsField>

              <SettingsField
                label={t("admin.settings.general.defaultLanguage", "Default language")}
                helper={t("admin.settings.general.defaultLanguageHelp", "Default interface language for users without personal preference.")}
                testID="admin-settings-field-default-language"
              >
                <select
                  value={settings.general.default_language}
                  onChange={(event) =>
                    setSettings((prev) => ({ ...prev, general: { ...prev.general, default_language: event.target.value === "tr" ? "tr" : "en" } }))
                  }
                  className="focus-ring w-full rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-3 py-2 text-sm outline-none"
                >
                  <option value="en">{t("language.english", "English")}</option>
                  <option value="tr">{t("language.turkish", "Turkish")}</option>
                </select>
              </SettingsField>

              <SettingsField
                label={t("admin.settings.general.siteLogoUrl", "Site logo URL")}
                helper={t("admin.settings.general.siteLogoUrlHelp", "Logo URL shown in sidebar/header. Use a trusted https URL.")}
                testID="admin-settings-field-site-logo"
              >
                <input
                  value={settings.general.site_logo_url}
                  onChange={(event) =>
                    setSettings((prev) => ({ ...prev, general: { ...prev.general, site_logo_url: event.target.value } }))
                  }
                  className="focus-ring w-full rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-3 py-2 text-sm outline-none"
                  placeholder="https://cdn.example.com/logo.png"
                />
              </SettingsField>

              <SettingsField
                label={t("admin.settings.general.faviconUrl", "Favicon URL")}
                helper={t("admin.settings.general.faviconUrlHelp", "Browser tab icon URL. Browser cache refresh may be required after changes.")}
                testID="admin-settings-field-favicon-url"
              >
                <input
                  value={settings.general.favicon_url}
                  onChange={(event) =>
                    setSettings((prev) => ({ ...prev, general: { ...prev.general, favicon_url: event.target.value } }))
                  }
                  className="focus-ring w-full rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-3 py-2 text-sm outline-none"
                  placeholder="https://cdn.example.com/favicon.ico"
                />
              </SettingsField>

              <SettingsField
                label={t("admin.settings.general.brandColor", "Brand accent color")}
                helper={t("admin.settings.general.brandColorHelp", "Primary accent color used by buttons/highlights. Saved in hex format.")}
                testID="admin-settings-field-brand-color"
              >
                <div className="flex items-center gap-2">
                  <input
                    type="color"
                    value={settings.general.brand_color || "#2563eb"}
                    onChange={(event) =>
                      setSettings((prev) => ({ ...prev, general: { ...prev.general, brand_color: event.target.value } }))
                    }
                    className="h-10 w-12 cursor-pointer rounded-md border border-[var(--line)] bg-transparent"
                  />
                  <input
                    value={settings.general.brand_color}
                    onChange={(event) =>
                      setSettings((prev) => ({ ...prev, general: { ...prev.general, brand_color: event.target.value } }))
                    }
                    className="focus-ring w-full rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-3 py-2 text-sm outline-none"
                    placeholder="#2563eb"
                  />
                </div>
              </SettingsField>
            </div>

            <div className="mt-4 grid gap-4 lg:grid-cols-[minmax(0,1fr)_300px]">
              <SettingsField
                label={t("admin.settings.general.maintenanceMode", "Maintenance mode")}
                helper={t("admin.settings.general.maintenanceModeHelp", "When enabled, write actions may be restricted depending on deployment/runtime settings.")}
                testID="admin-settings-field-maintenance"
              >
                <label className="inline-flex items-center gap-2 text-sm">
                  <input
                    type="checkbox"
                    checked={settings.general.maintenance_mode}
                    onChange={(event) =>
                      setSettings((prev) => ({ ...prev, general: { ...prev.general, maintenance_mode: event.target.checked } }))
                    }
                  />
                  {t("admin.settings.general.maintenanceModeEnabled", "Enable maintenance mode")}
                </label>
              </SettingsField>

              <div className="rounded-xl border border-[var(--line)] bg-[var(--bg-soft)] p-3" data-testid="admin-settings-preview-card">
                <p className="text-xs uppercase tracking-wide text-[var(--text-muted)]">{t("admin.settings.general.livePreview", "Live preview")}</p>
                <div className="mt-2 flex items-center gap-3">
                  {settings.general.site_logo_url ? (
                    /* eslint-disable-next-line @next/next/no-img-element */
                    <img src={settings.general.site_logo_url} alt={t("admin.settings.general.siteLogoAlt", "Site logo")} className="h-10 w-10 rounded-lg border border-[var(--line)] object-cover" />
                  ) : (
                    <div className="grid h-10 w-10 place-items-center rounded-lg text-white" style={{ backgroundColor: settings.general.brand_color || "#2563eb" }}>
                      <i className="fa-solid fa-cloud" />
                    </div>
                  )}
                  <div>
                    <p className="text-sm font-semibold">{adminBrandName}</p>
                    <p className="text-xs text-[var(--text-muted)]">{settings.general.site_subtitle || t("admin.settings.general.previewSubtitleDefault", "Private storage")}</p>
                    <p className="text-xs text-[var(--text-muted)]">{settings.general.public_base_url || t("admin.settings.general.publicUrlMissing", "Public URL is not configured.")}</p>
                  </div>
                </div>
              </div>
            </div>

            <button disabled={settingsSaving || settingsLoading} onClick={() => void saveSettingsSection("general", settings.general)} className="focus-ring mt-4 rounded-lg bg-[var(--brand)] px-4 py-2 text-sm font-semibold text-white">
              {t("common.save", "Save")}
            </button>
          </section>
        ) : null}

        {activeTab === "sharing" ? (
          <section className="surface-card rounded-2xl p-5" data-testid="admin-section-sharing">
            <h2 className="text-lg font-semibold">{t("admin.tabs.sharing", "Sharing")}</h2>
            <div className="mt-4 grid gap-4 md:grid-cols-2">
              <SettingsField label={t("admin.settings.sharing.publicSharingEnabled", "Public sharing enabled")} helper={t("admin.settings.sharing.publicSharingEnabledHelp", "Users can create publicly accessible sharing links.")} testID="admin-settings-field-sharing-enabled">
                <label className="inline-flex items-center gap-2 text-sm">
                  <input
                    type="checkbox"
                    checked={settings.sharing.public_sharing_enabled}
                    onChange={(event) =>
                      setSettings((prev) => ({ ...prev, sharing: { ...prev.sharing, public_sharing_enabled: event.target.checked } }))
                    }
                  />
                  {t("common.enabled", "Enabled")}
                </label>
              </SettingsField>

              <SettingsField
                label={t("admin.settings.sharing.requirePasswordForPublicLinks", "Require password for public links")}
                helper={t("admin.settings.sharing.requirePasswordForPublicLinksHelp", "If enabled, public links cannot be created without password.")}
                testID="admin-settings-field-sharing-require-password"
              >
                <label className="inline-flex items-center gap-2 text-sm">
                  <input
                    type="checkbox"
                    checked={settings.sharing.require_password_for_public_links}
                    onChange={(event) =>
                      setSettings((prev) => ({
                        ...prev,
                        sharing: { ...prev.sharing, require_password_for_public_links: event.target.checked }
                      }))
                    }
                  />
                  {t("common.required", "Required")}
                </label>
              </SettingsField>

              <SettingsField
                label={t("admin.settings.sharing.allowPasswordProtectedLinks", "Allow password-protected links")}
                helper={t("admin.settings.sharing.allowPasswordProtectedLinksHelp", "Users can optionally set password on public links.")}
                testID="admin-settings-field-sharing-allow-password"
              >
                <label className="inline-flex items-center gap-2 text-sm">
                  <input
                    type="checkbox"
                    checked={settings.sharing.allow_password_protected_links}
                    onChange={(event) =>
                      setSettings((prev) => ({
                        ...prev,
                        sharing: { ...prev.sharing, allow_password_protected_links: event.target.checked }
                      }))
                    }
                  />
                  {t("common.allow", "Allow")}
                </label>
              </SettingsField>

              <SettingsField
                label={t("admin.settings.sharing.allowExpiration", "Allow expiration date")}
                helper={t("admin.settings.sharing.allowExpirationHelp", "Users can set expiration date for shared links.")}
                testID="admin-settings-field-sharing-allow-expiration"
              >
                <label className="inline-flex items-center gap-2 text-sm">
                  <input
                    type="checkbox"
                    checked={settings.sharing.allow_expiration}
                    onChange={(event) =>
                      setSettings((prev) => ({ ...prev, sharing: { ...prev.sharing, allow_expiration: event.target.checked } }))
                    }
                  />
                  {t("common.allow", "Allow")}
                </label>
              </SettingsField>

              <SettingsField
                label={t("admin.settings.sharing.allowPublicDownloads", "Allow public downloads")}
                helper={t("admin.settings.sharing.allowPublicDownloadsHelp", "If disabled, public links can be preview-only and downloads are blocked.")}
                testID="admin-settings-field-sharing-public-download"
              >
                <label className="inline-flex items-center gap-2 text-sm">
                  <input
                    type="checkbox"
                    checked={settings.sharing.allow_public_downloads}
                    onChange={(event) =>
                      setSettings((prev) => ({ ...prev, sharing: { ...prev.sharing, allow_public_downloads: event.target.checked } }))
                    }
                  />
                  {t("common.allow", "Allow")}
                </label>
              </SettingsField>

              <SettingsField
                label={t("admin.settings.sharing.defaultExpirationDays", "Default link expiration (days)")}
                helper={t("admin.settings.sharing.defaultExpirationDaysHelp", "Default validity period for newly created public links.")}
                testID="admin-settings-field-sharing-default-expiration"
              >
                <input
                  type="number"
                  min={0}
                  value={settings.sharing.default_expiration_days}
                  onChange={(event) =>
                    setSettings((prev) => ({
                      ...prev,
                      sharing: { ...prev.sharing, default_expiration_days: Number(event.target.value) }
                    }))
                  }
                  className="focus-ring w-full rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-3 py-2 text-sm outline-none"
                  placeholder="7"
                />
              </SettingsField>

              <SettingsField
                label={t("admin.settings.sharing.maximumExpirationDays", "Maximum link expiration (days)")}
                helper={t("admin.settings.sharing.maximumExpirationDaysHelp", "Upper limit for how long public links can remain valid.")}
                testID="admin-settings-field-sharing-max-expiration"
              >
                <input
                  type="number"
                  min={0}
                  value={settings.sharing.maximum_expiration_days}
                  onChange={(event) =>
                    setSettings((prev) => ({
                      ...prev,
                      sharing: { ...prev.sharing, maximum_expiration_days: Number(event.target.value) }
                    }))
                  }
                  className="focus-ring w-full rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-3 py-2 text-sm outline-none"
                  placeholder="90"
                />
              </SettingsField>

              <SettingsField
                label={t("admin.settings.sharing.allowFolderSharing", "Allow folder sharing")}
                helper={t("admin.settings.sharing.allowFolderSharingHelp", "Allow creating public shares for folders.")}
                testID="admin-settings-field-sharing-folder"
              >
                <label className="inline-flex items-center gap-2 text-sm">
                  <input
                    type="checkbox"
                    checked={settings.sharing.allow_folder_sharing}
                    onChange={(event) =>
                      setSettings((prev) => ({ ...prev, sharing: { ...prev.sharing, allow_folder_sharing: event.target.checked } }))
                    }
                  />
                  {t("common.allow", "Allow")}
                </label>
              </SettingsField>
            </div>
            <button disabled={settingsSaving || settingsLoading} onClick={() => void saveSettingsSection("sharing", settings.sharing)} className="focus-ring mt-4 rounded-lg bg-[var(--brand)] px-4 py-2 text-sm font-semibold text-white">
              {t("common.save", "Save")}
            </button>
          </section>
        ) : null}

        {activeTab === "security" ? (
          <section className="surface-card rounded-2xl p-5" data-testid="admin-section-security">
            <h2 className="text-lg font-semibold">{t("admin.tabs.security", "Security")}</h2>
            <div className="mt-4 grid gap-4 md:grid-cols-2">
              <SettingsField
                label={t("admin.settings.security.minimumPasswordLength", "Minimum password length")}
                helper={t("admin.settings.security.minimumPasswordLengthHelp", "Minimum required length when creating/updating passwords.")}
                testID="admin-settings-field-security-min-password"
              >
                <input
                  type="number"
                  min={8}
                  value={settings.security.minimum_password_length}
                  onChange={(event) =>
                    setSettings((prev) => ({
                      ...prev,
                      security: { ...prev.security, minimum_password_length: Number(event.target.value) }
                    }))
                  }
                  className="focus-ring w-full rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-3 py-2 text-sm outline-none"
                  placeholder="12"
                />
              </SettingsField>

              <SettingsField
                label={t("admin.settings.security.sessionLifetimeMinutes", "Session lifetime (minutes)")}
                helper={t("admin.settings.security.sessionLifetimeMinutesHelp", "Access token lifetime. Shorter values improve security.")}
                testID="admin-settings-field-security-session-minutes"
              >
                <input
                  type="number"
                  min={1}
                  value={settings.security.session_lifetime_minutes}
                  onChange={(event) =>
                    setSettings((prev) => ({
                      ...prev,
                      security: { ...prev.security, session_lifetime_minutes: Number(event.target.value) }
                    }))
                  }
                  className="focus-ring w-full rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-3 py-2 text-sm outline-none"
                  placeholder="15"
                />
              </SettingsField>

              <SettingsField
                label={t("admin.settings.security.refreshTokenLifetimeMinutes", "Refresh token lifetime (minutes)")}
                helper={t("admin.settings.security.refreshTokenLifetimeMinutesHelp", "Long-term session renewal window. Example: 43200 minutes = 30 days.")}
                testID="admin-settings-field-security-refresh-minutes"
              >
                <input
                  type="number"
                  min={1}
                  value={settings.security.refresh_token_lifetime_minutes}
                  onChange={(event) =>
                    setSettings((prev) => ({
                      ...prev,
                      security: { ...prev.security, refresh_token_lifetime_minutes: Number(event.target.value) }
                    }))
                  }
                  className="focus-ring w-full rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-3 py-2 text-sm outline-none"
                  placeholder="43200"
                />
              </SettingsField>

              <SettingsField
                label={t("admin.settings.security.loginAttemptLimit", "Login attempt limit")}
                helper={t("admin.settings.security.loginAttemptLimitHelp", "Max failed login attempts allowed within rate-limit window.")}
                testID="admin-settings-field-security-login-limit"
              >
                <input
                  type="number"
                  min={1}
                  value={settings.security.login_rate_limit_per_window}
                  onChange={(event) =>
                    setSettings((prev) => ({
                      ...prev,
                      security: { ...prev.security, login_rate_limit_per_window: Number(event.target.value) }
                    }))
                  }
                  className="focus-ring w-full rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-3 py-2 text-sm outline-none"
                  placeholder="10"
                />
              </SettingsField>

              <SettingsField
                label={t("admin.settings.security.rateLimitWindowSeconds", "Rate-limit window (seconds)")}
                helper={t("admin.settings.security.rateLimitWindowSecondsHelp", "Time window where login attempt limits are enforced.")}
                testID="admin-settings-field-security-rate-window"
              >
                <input
                  type="number"
                  min={1}
                  value={settings.security.login_rate_window_seconds}
                  onChange={(event) =>
                    setSettings((prev) => ({
                      ...prev,
                      security: { ...prev.security, login_rate_window_seconds: Number(event.target.value) }
                    }))
                  }
                  className="focus-ring w-full rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-3 py-2 text-sm outline-none"
                  placeholder="60"
                />
              </SettingsField>
            </div>
            <div className="mt-4 grid gap-3 md:grid-cols-2">
              <SettingsField label={t("admin.settings.security.requireUppercase", "Require uppercase")} helper={t("admin.settings.security.requireUppercaseHelp", "Passwords must include at least one uppercase letter.")} testID="admin-settings-field-security-upper">
                <label className="inline-flex items-center gap-2 text-sm">
                  <input
                    type="checkbox"
                    checked={settings.security.require_uppercase}
                    onChange={(event) =>
                      setSettings((prev) => ({ ...prev, security: { ...prev.security, require_uppercase: event.target.checked } }))
                    }
                  />
                  {t("common.enabled", "Enabled")}
                </label>
              </SettingsField>
              <SettingsField label={t("admin.settings.security.requireLowercase", "Require lowercase")} helper={t("admin.settings.security.requireLowercaseHelp", "Passwords must include at least one lowercase letter.")} testID="admin-settings-field-security-lower">
                <label className="inline-flex items-center gap-2 text-sm">
                  <input
                    type="checkbox"
                    checked={settings.security.require_lowercase}
                    onChange={(event) =>
                      setSettings((prev) => ({ ...prev, security: { ...prev.security, require_lowercase: event.target.checked } }))
                    }
                  />
                  {t("common.enabled", "Enabled")}
                </label>
              </SettingsField>
              <SettingsField label={t("admin.settings.security.requireNumber", "Require number")} helper={t("admin.settings.security.requireNumberHelp", "Passwords must include at least one number.")} testID="admin-settings-field-security-number">
                <label className="inline-flex items-center gap-2 text-sm">
                  <input
                    type="checkbox"
                    checked={settings.security.require_number}
                    onChange={(event) =>
                      setSettings((prev) => ({ ...prev, security: { ...prev.security, require_number: event.target.checked } }))
                    }
                  />
                  {t("common.enabled", "Enabled")}
                </label>
              </SettingsField>
              <SettingsField label={t("admin.settings.security.requireSymbol", "Require symbol")} helper={t("admin.settings.security.requireSymbolHelp", "Passwords must include at least one special character.")} testID="admin-settings-field-security-symbol">
                <label className="inline-flex items-center gap-2 text-sm">
                  <input
                    type="checkbox"
                    checked={settings.security.require_symbol}
                    onChange={(event) =>
                      setSettings((prev) => ({ ...prev, security: { ...prev.security, require_symbol: event.target.checked } }))
                    }
                  />
                  {t("common.enabled", "Enabled")}
                </label>
              </SettingsField>
            </div>
            <p className="mt-3 text-xs text-[var(--text-muted)]">{t("admin.settings.security.restartNote", "Note: Session/refresh and rate-limit settings may require service restart in some deployments.")}</p>
            <button disabled={settingsSaving || settingsLoading} onClick={() => void saveSettingsSection("security", settings.security)} className="focus-ring mt-4 rounded-lg bg-[var(--brand)] px-4 py-2 text-sm font-semibold text-white">
              {t("common.save", "Save")}
            </button>
          </section>
        ) : null}

        {activeTab === "other" ? (
          <section className="surface-card rounded-2xl p-5" data-testid="admin-section-other">
            <h2 className="text-lg font-semibold">{t("admin.tabs.other", "Other")}</h2>
            <div className="mt-4 grid gap-4 md:grid-cols-2">
              <SettingsField
                label={t("admin.settings.other.uploadSessionCleanupSeconds", "Upload session cleanup interval (seconds)")}
                helper={t("admin.settings.other.uploadSessionCleanupSecondsHelp", "How often stale/incomplete upload sessions are cleaned up.")}
                testID="admin-settings-field-other-upload-session-ttl"
              >
                <input
                  type="number"
                  min={1}
                  value={settings.other.storage_cleanup_interval_seconds}
                  onChange={(event) =>
                    setSettings((prev) => ({
                      ...prev,
                      other: { ...prev.other, storage_cleanup_interval_seconds: Number(event.target.value) }
                    }))
                  }
                  className="focus-ring w-full rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-3 py-2 text-sm outline-none"
                  placeholder="60"
                />
              </SettingsField>

              <SettingsField
                label={t("admin.settings.other.maxFileSizeGb", "Maximum file size (GB)")}
                helper={t("admin.settings.other.maxFileSizeGbHelp", "Upper limit for a single file. Current: {size}", { size: formatBytes(settings.other.upload_max_file_size_bytes) })}
                testID="admin-settings-field-other-max-file-size"
              >
                <input
                  type="number"
                  min={1}
                  step="0.5"
                  value={toUnitInput(settings.other.upload_max_file_size_bytes, BYTES_IN_GB)}
                  onChange={(event) => {
                    const parsed = parsePositiveNumber(event.target.value);
                    if (parsed === null) {
                      return;
                    }
                    setSettings((prev) => ({
                      ...prev,
                      other: { ...prev.other, upload_max_file_size_bytes: Math.round(parsed * BYTES_IN_GB) }
                    }));
                  }}
                  className="focus-ring w-full rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-3 py-2 text-sm outline-none"
                  placeholder="100"
                />
              </SettingsField>

              <SettingsField
                label={t("admin.settings.other.maxChunkSizeMb", "Maximum chunk size (MB)")}
                helper={t("admin.settings.other.maxChunkSizeMbHelp", "Max allowed size for each upload chunk. Current: {size}", { size: formatBytes(settings.other.upload_max_chunk_size_bytes) })}
                testID="admin-settings-field-other-max-chunk-size"
              >
                <input
                  type="number"
                  min={1}
                  step="1"
                  value={toUnitInput(settings.other.upload_max_chunk_size_bytes, BYTES_IN_MB)}
                  onChange={(event) => {
                    const parsed = parsePositiveNumber(event.target.value);
                    if (parsed === null) {
                      return;
                    }
                    setSettings((prev) => ({
                      ...prev,
                      other: { ...prev.other, upload_max_chunk_size_bytes: Math.round(parsed * BYTES_IN_MB) }
                    }));
                  }}
                  className="focus-ring w-full rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-3 py-2 text-sm outline-none"
                  placeholder="16"
                />
              </SettingsField>

              <SettingsField
                label={t("admin.settings.other.loggingLevel", "Logging level")}
                helper={t("admin.settings.other.loggingLevelHelp", "Application log verbosity (example: debug, info, warn, error).")}
                testID="admin-settings-field-other-logging-level"
              >
                <input
                  value={settings.other.logging_level}
                  onChange={(event) =>
                    setSettings((prev) => ({ ...prev, other: { ...prev.other, logging_level: event.target.value } }))
                  }
                  className="focus-ring w-full rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-3 py-2 text-sm outline-none"
                  placeholder="info"
                />
              </SettingsField>
            </div>
            <div className="mt-4">
              <SettingsField
                label={t("admin.settings.other.previewGeneration", "Preview generation")}
                helper={t("admin.settings.other.previewGenerationHelp", "Enable/disable preview worker pipeline. Worker restart may be required in some environments.")}
                testID="admin-settings-field-other-preview-generation"
              >
                <label className="inline-flex items-center gap-2 text-sm">
                  <input
                    type="checkbox"
                    checked={settings.other.preview_generation_enabled}
                    onChange={(event) =>
                      setSettings((prev) => ({
                        ...prev,
                        other: { ...prev.other, preview_generation_enabled: event.target.checked }
                      }))
                    }
                  />
                  {t("common.enabled", "Enabled")}
                </label>
              </SettingsField>
            </div>
            <button disabled={settingsSaving || settingsLoading} onClick={() => void saveSettingsSection("other", settings.other)} className="focus-ring mt-4 rounded-lg bg-[var(--brand)] px-4 py-2 text-sm font-semibold text-white">
              {t("common.save", "Save")}
            </button>
            <p className="mt-3 text-xs text-[var(--text-muted)]">{settings.other.requires_restart_note || t("admin.settings.restartNote", "Some settings may require a restart.")}</p>
          </section>
        ) : null}
      </div>

      {modalState.open ? (
        <UserModal
          mode={modalState.mode}
          open={modalState.open}
          title={modalState.title}
          initialValues={modalState.initialValues}
          busy={modalBusy}
          emailLocked={modalState.mode === "edit"}
          onClose={() => setModalState({ open: false })}
          onSubmit={async (values) => {
            setModalBusy(true);
            setError(null);
            try {
              if (modalState.mode === "create") {
                let reactivated = false;
                const quotaBytes = values.useDefaultQuota ? null : quotaGbToBytes(values.quotaGb);
                if (!values.useDefaultQuota && quotaBytes === null) {
                  throw new Error(t("admin.validation.validQuota", "Enter a valid quota (GB)."));
                }
                if (values.password.trim().length < 12) {
                  throw new Error(t("admin.validation.passwordMinLength", "Password must be at least 12 characters."));
                }
                try {
                  await createAdminUser({
                    email: values.email.trim(),
                    displayName: values.displayName.trim(),
                    role: values.role,
                    password: values.password.trim(),
                    quotaBytes,
                    isActive: values.isActive
                  });
                } catch (err) {
                  if (err instanceof ApiError && err.code === "existing_inactive_user") {
                    const approved = window.confirm(
                      t(
                        "admin.users.existingInactivePrompt",
                        "A disabled/inactive user already exists with this email. Do you want to reactivate it?"
                      )
                    );
                    if (approved) {
                      await createAdminUser({
                        email: values.email.trim(),
                        displayName: values.displayName.trim(),
                        role: values.role,
                        password: values.password.trim(),
                        quotaBytes,
                        isActive: true,
                        reactivateExisting: true
                      });
                      reactivated = true;
                      showToast("success", t("admin.users.existingReactivated", "Existing user reactivated."));
                    } else {
                      throw err;
                    }
                  } else {
                    throw err;
                  }
                }
                if (!reactivated) {
                  showToast("success", t("admin.users.created", "User created."));
                }
              } else if (modalState.userID) {
                const payload: Parameters<typeof updateAdminUser>[0] = {
                  userID: modalState.userID,
                  displayName: values.displayName.trim(),
                  role: values.role,
                  isActive: values.isActive
                };
                if (values.useDefaultQuota) {
                  payload.useDefaultQuota = true;
                } else {
                  const quotaBytes = quotaGbToBytes(values.quotaGb);
                  if (quotaBytes === null) {
                    throw new Error(t("admin.validation.validQuota", "Enter a valid quota (GB)."));
                  }
                  payload.quotaBytes = quotaBytes;
                  payload.useDefaultQuota = false;
                }
                await updateAdminUser(payload);
                showToast("success", t("admin.users.updated", "User updated."));
              }
              setModalState({ open: false });
              await Promise.all([loadUsers(userStatusFilter), loadAudit()]);
            } finally {
              setModalBusy(false);
            }
          }}
        />
      ) : null}
    </main>
  );
}
