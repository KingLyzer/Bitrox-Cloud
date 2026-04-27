"use client";

import { useEffect, useMemo, useState } from "react";
import {
  ApiError,
  createAdminUser,
  fetchAdminSettings,
  fetchMe,
  listAdminUsers,
  type AdminSettings,
  type AdminUser,
  updateAdminSettingsSection,
  updateAdminUser,
  updateAdminUserPassword
} from "@/lib/api";
import { ThemeToggle } from "@/components/theme-toggle";
import { AdminUsersPanel } from "@/components/file-manager/admin-users-panel";

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
    maintenance_mode: false,
  },
  sharing: {
    public_sharing_enabled: true,
    allow_password_protected_links: true,
    allow_expiration: true,
    default_expiration_days: 7,
    maximum_expiration_days: 90,
    allow_public_downloads: true,
    allow_folder_sharing: false,
    require_password_for_public_links: false,
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
    trusted_proxy_note: "",
  },
  other: {
    storage_cleanup_interval_seconds: 60,
    upload_max_file_size_bytes: 107374182400,
    upload_max_chunk_size_bytes: 16777216,
    preview_generation_enabled: false,
    logging_level: "info",
    requires_restart_note: "",
  },
};

type Tab = "general" | "users" | "sharing" | "security" | "other";

export function AdminSettingsPage() {
  const [authorized, setAuthorized] = useState<boolean | null>(null);
  const [tab, setTab] = useState<Tab>("general");
  const [settings, setSettings] = useState<AdminSettings>(defaultSettings);
  const [busy, setBusy] = useState(false);
  const [users, setUsers] = useState<AdminUser[]>([]);
  const [usersLoading, setUsersLoading] = useState(false);
  const [flash, setFlash] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const canShowUsers = useMemo(() => tab === "users", [tab]);

  async function loadSettingsAndUsers() {
    setBusy(true);
    setError(null);
    try {
      const [loadedSettings, loadedUsers] = await Promise.all([fetchAdminSettings(), listAdminUsers()]);
      setSettings({
        ...defaultSettings,
        ...loadedSettings,
        general: { ...defaultSettings.general, ...loadedSettings.general },
        sharing: { ...defaultSettings.sharing, ...loadedSettings.sharing },
        security: { ...defaultSettings.security, ...loadedSettings.security },
        other: { ...defaultSettings.other, ...loadedSettings.other }
      });
      setUsers(loadedUsers);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Ayarlar yuklenemedi.");
    } finally {
      setBusy(false);
    }
  }

  async function loadUsers() {
    setUsersLoading(true);
    try {
      setUsers(await listAdminUsers());
    } catch (err) {
      setError(err instanceof Error ? err.message : "Kullanicilar yuklenemedi.");
    } finally {
      setUsersLoading(false);
    }
  }

  useEffect(() => {
    let active = true;
    void (async () => {
      try {
        const me = await fetchMe();
        const allowed = me.role === "owner" || me.role === "admin";
        if (!active) {
          return;
        }
        setAuthorized(allowed);
        if (allowed) {
          await loadSettingsAndUsers();
        }
      } catch {
        if (active) {
          setAuthorized(false);
        }
      }
    })();

    return () => {
      active = false;
    };
  }, []);

  useEffect(() => {
    if (!canShowUsers) {
      return;
    }
    void loadUsers();
  }, [canShowUsers]);

  function showSaved(message: string) {
    setFlash(message);
    window.setTimeout(() => setFlash(null), 3200);
  }

  async function saveSection<K extends Exclude<Tab, "users">>(section: K, value: AdminSettings[K]) {
    setBusy(true);
    setError(null);
    try {
      const saved = await updateAdminSettingsSection(section, value);
      setSettings((prev) => ({ ...prev, [section]: saved }));
      showSaved("Ayarlar kaydedildi.");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Ayar kaydedilemedi.");
    } finally {
      setBusy(false);
    }
  }

  if (authorized === null) {
    return <main className="grid min-h-screen place-items-center">Yukleniyor...</main>;
  }

  if (!authorized) {
    return (
      <main className="min-h-screen bg-[var(--bg-main)] p-6">
        <div className="mx-auto max-w-3xl rounded-2xl border border-[var(--line)] bg-[var(--bg-card)] p-6">
          <h1 className="text-xl font-semibold text-[var(--text-main)]">Yetkisiz Erisim</h1>
          <p className="mt-2 text-sm text-[var(--text-muted)]">Bu sayfayi acmak icin owner/admin yetkisi gerekir.</p>
          <a href="/" className="focus-ring mt-4 inline-flex rounded-lg border border-[var(--line)] px-3 py-2 text-sm hover:bg-[var(--bg-soft)]">
            Dosyalara don
          </a>
        </div>
      </main>
    );
  }

  return (
    <main className="min-h-screen bg-[var(--bg-main)] p-4 lg:p-6">
      <div className="mx-auto max-w-7xl space-y-4">
        <header className="surface-card flex items-center justify-between rounded-2xl px-4 py-3">
          <div>
            <p className="text-xs uppercase tracking-wide text-[var(--text-muted)]">Admin Settings</p>
            <h1 className="text-xl font-semibold text-[var(--text-main)]">Yonetim Ayarlari</h1>
          </div>
          <div className="flex items-center gap-2">
            <a href="/admin" className="focus-ring rounded-lg border border-[var(--line)] px-3 py-2 text-sm hover:bg-[var(--bg-soft)]">Admin Panel</a>
            <a href="/" className="focus-ring rounded-lg border border-[var(--line)] px-3 py-2 text-sm hover:bg-[var(--bg-soft)]">Dosyalar</a>
            <ThemeToggle />
          </div>
        </header>

        {flash ? <div className="rounded-xl border border-emerald-500/40 bg-emerald-500/10 px-4 py-2 text-sm text-emerald-600">{flash}</div> : null}
        {error ? <div className="rounded-xl border border-red-500/40 bg-red-500/10 px-4 py-2 text-sm text-red-500">{error}</div> : null}

        <div className="grid gap-4 lg:grid-cols-[220px_minmax(0,1fr)]">
          <aside className="surface-card rounded-2xl p-3">
            <p className="px-2 pb-2 text-xs uppercase tracking-wide text-[var(--text-muted)]">Ayar Menusu</p>
            {([
              ["general", "Genel"],
              ["users", "Kullanicilar"],
              ["sharing", "Paylasim"],
              ["security", "Guvenlik"],
              ["other", "Diger"]
            ] as Array<[Tab, string]>).map(([value, label]) => (
              <button
                key={value}
                type="button"
                onClick={() => setTab(value)}
                data-testid={`admin-settings-tab-${value}`}
                className={`focus-ring mb-1 flex w-full items-center gap-2 rounded-lg px-3 py-2 text-left text-sm ${
                  tab === value ? "bg-[var(--brand-soft)] text-[var(--brand)]" : "hover:bg-[var(--bg-soft)]"
                }`}
              >
                {label}
              </button>
            ))}
          </aside>

          <section className="space-y-4">
            {tab === "users" ? (
              <AdminUsersPanel
                users={users}
                loading={usersLoading}
                busy={busy}
                onRefresh={() => void loadUsers()}
                onCreateUser={async (payload) => {
                  await createAdminUser(payload);
                  await loadUsers();
                  showSaved("Kullanici olusturuldu.");
                }}
                onUpdateUser={async (payload) => {
                  await updateAdminUser(payload);
                  await loadUsers();
                  showSaved("Kullanici guncellendi.");
                }}
                onUpdatePassword={async (userID, newPassword) => {
                  await updateAdminUserPassword(userID, newPassword);
                  await loadUsers();
                  showSaved("Parola guncellendi.");
                }}
              />
            ) : null}

            {tab === "general" ? (
              <div className="surface-card rounded-2xl p-5" data-testid="admin-settings-section-general">
                <h2 className="text-lg font-semibold">Genel</h2>
                <div className="mt-4 grid gap-3 md:grid-cols-2">
                  <input value={settings.general.site_name} onChange={(e) => setSettings((p) => ({ ...p, general: { ...p.general, site_name: e.target.value } }))} className="focus-ring rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-3 py-2 text-sm outline-none" placeholder="Site adi" />
                  <input value={settings.general.public_base_url} onChange={(e) => setSettings((p) => ({ ...p, general: { ...p.general, public_base_url: e.target.value } }))} className="focus-ring rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-3 py-2 text-sm outline-none" placeholder="Public base URL" />
                  <input type="number" value={settings.general.default_storage_quota_bytes} onChange={(e) => setSettings((p) => ({ ...p, general: { ...p.general, default_storage_quota_bytes: Number(e.target.value) } }))} className="focus-ring rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-3 py-2 text-sm outline-none" placeholder="Varsayilan kota (bayt)" />
                  <input value={settings.general.default_timezone} onChange={(e) => setSettings((p) => ({ ...p, general: { ...p.general, default_timezone: e.target.value } }))} className="focus-ring rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-3 py-2 text-sm outline-none" placeholder="Timezone" />
                </div>
                <label className="mt-3 inline-flex items-center gap-2 text-sm"><input type="checkbox" checked={settings.general.maintenance_mode} onChange={(e) => setSettings((p) => ({ ...p, general: { ...p.general, maintenance_mode: e.target.checked } }))} /> Maintenance mode</label>
                <button disabled={busy} onClick={() => void saveSection("general", settings.general)} className="focus-ring mt-4 rounded-lg bg-[var(--brand)] px-4 py-2 text-sm font-semibold text-white">Kaydet</button>
              </div>
            ) : null}

            {tab === "sharing" ? (
              <div className="surface-card rounded-2xl p-5" data-testid="admin-settings-section-sharing">
                <h2 className="text-lg font-semibold">Paylasim</h2>
                <div className="mt-4 grid gap-2 md:grid-cols-2">
                  <label className="inline-flex items-center gap-2 text-sm"><input type="checkbox" checked={settings.sharing.public_sharing_enabled} onChange={(e) => setSettings((p) => ({ ...p, sharing: { ...p.sharing, public_sharing_enabled: e.target.checked } }))} /> Public paylasim aktif</label>
                  <label className="inline-flex items-center gap-2 text-sm"><input type="checkbox" checked={settings.sharing.allow_password_protected_links} onChange={(e) => setSettings((p) => ({ ...p, sharing: { ...p.sharing, allow_password_protected_links: e.target.checked } }))} /> Parolali linklere izin ver</label>
                  <label className="inline-flex items-center gap-2 text-sm"><input type="checkbox" checked={settings.sharing.require_password_for_public_links} onChange={(e) => setSettings((p) => ({ ...p, sharing: { ...p.sharing, require_password_for_public_links: e.target.checked } }))} /> Public linkte parola zorunlu</label>
                  <label className="inline-flex items-center gap-2 text-sm"><input type="checkbox" checked={settings.sharing.allow_public_downloads} onChange={(e) => setSettings((p) => ({ ...p, sharing: { ...p.sharing, allow_public_downloads: e.target.checked } }))} /> Public indirmeye izin ver</label>
                  <input type="number" value={settings.sharing.default_expiration_days} onChange={(e) => setSettings((p) => ({ ...p, sharing: { ...p.sharing, default_expiration_days: Number(e.target.value) } }))} className="focus-ring rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-3 py-2 text-sm outline-none" placeholder="Varsayilan sure (gun)" />
                  <input type="number" value={settings.sharing.maximum_expiration_days} onChange={(e) => setSettings((p) => ({ ...p, sharing: { ...p.sharing, maximum_expiration_days: Number(e.target.value) } }))} className="focus-ring rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-3 py-2 text-sm outline-none" placeholder="Maksimum sure (gun)" />
                </div>
                <button disabled={busy} onClick={() => void saveSection("sharing", settings.sharing)} className="focus-ring mt-4 rounded-lg bg-[var(--brand)] px-4 py-2 text-sm font-semibold text-white">Kaydet</button>
              </div>
            ) : null}

            {tab === "security" ? (
              <div className="surface-card rounded-2xl p-5" data-testid="admin-settings-section-security">
                <h2 className="text-lg font-semibold">Guvenlik</h2>
                <div className="mt-4 grid gap-3 md:grid-cols-2">
                  <input type="number" value={settings.security.minimum_password_length} onChange={(e) => setSettings((p) => ({ ...p, security: { ...p.security, minimum_password_length: Number(e.target.value) } }))} className="focus-ring rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-3 py-2 text-sm outline-none" placeholder="Minimum parola uzunlugu" />
                  <input type="number" value={settings.security.session_lifetime_minutes} onChange={(e) => setSettings((p) => ({ ...p, security: { ...p.security, session_lifetime_minutes: Number(e.target.value) } }))} className="focus-ring rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-3 py-2 text-sm outline-none" placeholder="Session suresi (dk)" />
                  <input type="number" value={settings.security.refresh_token_lifetime_minutes} onChange={(e) => setSettings((p) => ({ ...p, security: { ...p.security, refresh_token_lifetime_minutes: Number(e.target.value) } }))} className="focus-ring rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-3 py-2 text-sm outline-none" placeholder="Refresh token suresi (dk)" />
                  <input type="number" value={settings.security.login_rate_limit_per_window} onChange={(e) => setSettings((p) => ({ ...p, security: { ...p.security, login_rate_limit_per_window: Number(e.target.value) } }))} className="focus-ring rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-3 py-2 text-sm outline-none" placeholder="Login limit" />
                </div>
                <div className="mt-3 grid gap-2 md:grid-cols-2">
                  <label className="inline-flex items-center gap-2 text-sm"><input type="checkbox" checked={settings.security.require_uppercase} onChange={(e) => setSettings((p) => ({ ...p, security: { ...p.security, require_uppercase: e.target.checked } }))} /> Buyuk harf zorunlu</label>
                  <label className="inline-flex items-center gap-2 text-sm"><input type="checkbox" checked={settings.security.require_lowercase} onChange={(e) => setSettings((p) => ({ ...p, security: { ...p.security, require_lowercase: e.target.checked } }))} /> Kucuk harf zorunlu</label>
                  <label className="inline-flex items-center gap-2 text-sm"><input type="checkbox" checked={settings.security.require_number} onChange={(e) => setSettings((p) => ({ ...p, security: { ...p.security, require_number: e.target.checked } }))} /> Rakam zorunlu</label>
                  <label className="inline-flex items-center gap-2 text-sm"><input type="checkbox" checked={settings.security.require_symbol} onChange={(e) => setSettings((p) => ({ ...p, security: { ...p.security, require_symbol: e.target.checked } }))} /> Sembol zorunlu</label>
                  <label className="inline-flex items-center gap-2 text-sm"><input type="checkbox" checked={settings.security.two_factor_required} onChange={(e) => setSettings((p) => ({ ...p, security: { ...p.security, two_factor_required: e.target.checked } }))} /> 2FA zorunlu (foundation)</label>
                </div>
                <button disabled={busy} onClick={() => void saveSection("security", settings.security)} className="focus-ring mt-4 rounded-lg bg-[var(--brand)] px-4 py-2 text-sm font-semibold text-white">Kaydet</button>
              </div>
            ) : null}

            {tab === "other" ? (
              <div className="surface-card rounded-2xl p-5" data-testid="admin-settings-section-other">
                <h2 className="text-lg font-semibold">Diger</h2>
                <div className="mt-4 grid gap-3 md:grid-cols-2">
                  <input type="number" value={settings.other.storage_cleanup_interval_seconds} onChange={(e) => setSettings((p) => ({ ...p, other: { ...p.other, storage_cleanup_interval_seconds: Number(e.target.value) } }))} className="focus-ring rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-3 py-2 text-sm outline-none" placeholder="Cleanup interval (sn)" />
                  <input type="number" value={settings.other.upload_max_file_size_bytes} onChange={(e) => setSettings((p) => ({ ...p, other: { ...p.other, upload_max_file_size_bytes: Number(e.target.value) } }))} className="focus-ring rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-3 py-2 text-sm outline-none" placeholder="Maks dosya boyutu (bayt)" />
                  <input type="number" value={settings.other.upload_max_chunk_size_bytes} onChange={(e) => setSettings((p) => ({ ...p, other: { ...p.other, upload_max_chunk_size_bytes: Number(e.target.value) } }))} className="focus-ring rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-3 py-2 text-sm outline-none" placeholder="Maks chunk (bayt)" />
                  <input value={settings.other.logging_level} onChange={(e) => setSettings((p) => ({ ...p, other: { ...p.other, logging_level: e.target.value } }))} className="focus-ring rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-3 py-2 text-sm outline-none" placeholder="Log seviyesi" />
                </div>
                <label className="mt-3 inline-flex items-center gap-2 text-sm"><input type="checkbox" checked={settings.other.preview_generation_enabled} onChange={(e) => setSettings((p) => ({ ...p, other: { ...p.other, preview_generation_enabled: e.target.checked } }))} /> Preview generation (foundation)</label>
                <button disabled={busy} onClick={() => void saveSection("other", settings.other)} className="focus-ring mt-4 rounded-lg bg-[var(--brand)] px-4 py-2 text-sm font-semibold text-white">Kaydet</button>
                <p className="mt-3 text-xs text-[var(--text-muted)]">{settings.other.requires_restart_note || "Bazi ayarlar restart gerektirir."}</p>
              </div>
            ) : null}
          </section>
        </div>
      </div>
    </main>
  );
}
