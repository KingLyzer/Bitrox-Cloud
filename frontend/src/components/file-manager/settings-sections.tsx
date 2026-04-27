"use client";

import { AdminUsersPanel } from "@/components/file-manager/admin-users-panel";
import type { AdminUser } from "@/lib/api";

export type ProfilePrefs = {
  customDisplayName: string;
  avatarColor: string;
};

type Props = {
  activeSection: "settings" | "profile";
  settingsTab: "users" | "sharing" | "security" | "other";
  setSettingsTab: (tab: "users" | "sharing" | "security" | "other") => void;
  canAccessAdmin: boolean;
  adminUsers: AdminUser[];
  adminLoading: boolean;
  adminBusy: boolean;
  onRefreshAdminUsers: () => void;
  onCreateUser: (payload: {
    email: string;
    displayName: string;
    role: "owner" | "admin" | "user";
    password: string;
    quotaBytes: number | null;
    isActive: boolean;
  }) => Promise<void>;
  onUpdateUser: (payload: {
    userID: string;
    displayName: string;
    role: "owner" | "admin" | "user";
    isActive: boolean;
    quotaBytes?: number;
    useDefaultQuota?: boolean;
  }) => Promise<void>;
  onUpdatePassword: (userID: string, newPassword: string) => Promise<void>;
  profilePrefs: ProfilePrefs;
  saveProfilePrefs: (prefs: ProfilePrefs) => void;
  defaultProfilePrefs: ProfilePrefs;
  avatarColorOptions: string[];
  effectiveAvatarInitials: string;
  userDisplayName: string;
};

export function SettingsSections({
  activeSection,
  settingsTab,
  setSettingsTab,
  canAccessAdmin,
  adminUsers,
  adminLoading,
  adminBusy,
  onRefreshAdminUsers,
  onCreateUser,
  onUpdateUser,
  onUpdatePassword,
  profilePrefs,
  saveProfilePrefs,
  defaultProfilePrefs,
  avatarColorOptions,
  effectiveAvatarInitials,
  userDisplayName
}: Props) {
  if (activeSection === "settings") {
    return (
      <section className="grid gap-4 lg:grid-cols-[220px_minmax(0,1fr)]">
        <aside className="surface-card rounded-2xl p-3">
          <p className="px-2 pb-2 text-xs uppercase tracking-wide text-[var(--text-muted)]">Ayar Menusu</p>
          <nav className="space-y-1">
            <button
              type="button"
              onClick={() => setSettingsTab("users")}
              className={`focus-ring flex w-full items-center gap-2 rounded-lg px-3 py-2 text-left text-sm ${
                settingsTab === "users" ? "bg-[var(--brand-soft)] text-[var(--brand)]" : "hover:bg-[var(--bg-soft)]"
              }`}
            >
              <i className="fa-solid fa-users" />
              Kullanicilar
            </button>
            <button
              type="button"
              onClick={() => setSettingsTab("sharing")}
              className={`focus-ring flex w-full items-center gap-2 rounded-lg px-3 py-2 text-left text-sm ${
                settingsTab === "sharing" ? "bg-[var(--brand-soft)] text-[var(--brand)]" : "hover:bg-[var(--bg-soft)]"
              }`}
            >
              <i className="fa-solid fa-share-nodes" />
              Paylasim
            </button>
            <button
              type="button"
              onClick={() => setSettingsTab("security")}
              className={`focus-ring flex w-full items-center gap-2 rounded-lg px-3 py-2 text-left text-sm ${
                settingsTab === "security" ? "bg-[var(--brand-soft)] text-[var(--brand)]" : "hover:bg-[var(--bg-soft)]"
              }`}
            >
              <i className="fa-solid fa-shield-halved" />
              Guvenlik
            </button>
            <button
              type="button"
              onClick={() => setSettingsTab("other")}
              className={`focus-ring flex w-full items-center gap-2 rounded-lg px-3 py-2 text-left text-sm ${
                settingsTab === "other" ? "bg-[var(--brand-soft)] text-[var(--brand)]" : "hover:bg-[var(--bg-soft)]"
              }`}
            >
              <i className="fa-solid fa-sliders" />
              Diger
            </button>
          </nav>
        </aside>

        <div>
          {settingsTab === "users" ? (
            canAccessAdmin ? (
              <AdminUsersPanel
                users={adminUsers}
                loading={adminLoading}
                busy={adminBusy}
                onRefresh={onRefreshAdminUsers}
                onCreateUser={onCreateUser}
                onUpdateUser={onUpdateUser}
                onUpdatePassword={onUpdatePassword}
              />
            ) : (
              <div className="surface-card rounded-2xl p-5">
                <h2 className="text-lg font-semibold">Kullanicilar</h2>
                <p className="mt-2 text-sm text-[var(--text-muted)]">Bu bolum yalnizca owner veya admin rolundeki hesaplara aciktir.</p>
              </div>
            )
          ) : settingsTab === "sharing" ? (
            <div className="surface-card rounded-2xl p-5">
              <h2 className="text-lg font-semibold">Paylasim</h2>
              <p className="mt-2 text-sm text-[var(--text-muted)]">Paylasim izinleri, herkese acik linkler ve ekip klasor politikalarini bu bolumde yonetebilecegiz.</p>
            </div>
          ) : settingsTab === "security" ? (
            <div className="surface-card rounded-2xl p-5">
              <h2 className="text-lg font-semibold">Guvenlik</h2>
              <p className="mt-2 text-sm text-[var(--text-muted)]">Oturum sureleri, parola politikasi ve ekstra guvenlik ayarlari bu bolumde yer alacak.</p>
            </div>
          ) : (
            <div className="surface-card rounded-2xl p-5">
              <h2 className="text-lg font-semibold">Diger</h2>
              <p className="mt-2 text-sm text-[var(--text-muted)]">Bildirimler, arayuz tercihleri ve diger genel ayarlar burada toplanacak.</p>
            </div>
          )}
        </div>
      </section>
    );
  }

  return (
    <section className="space-y-4">
      <div className="surface-card rounded-2xl p-5">
        <h2 className="text-lg font-semibold">Kullanici Profili</h2>
        <p className="mt-1 text-sm text-[var(--text-muted)]">Bu ayarlar sadece bu tarayicida saklanir.</p>
        <div className="mt-5 grid gap-5 lg:grid-cols-[120px_minmax(0,1fr)]">
          <div className="flex flex-col items-center gap-3">
            <div className="grid h-20 w-20 place-items-center rounded-full text-2xl font-semibold text-white" style={{ backgroundColor: profilePrefs.avatarColor }}>
              {effectiveAvatarInitials}
            </div>
            <p className="text-xs text-[var(--text-muted)]">Onizleme</p>
          </div>
          <div className="space-y-4">
            <label className="block">
              <span className="mb-1 block text-xs uppercase tracking-wide text-[var(--text-muted)]">Gorunen Ad</span>
              <input
                value={userDisplayName}
                readOnly
                placeholder={userDisplayName}
                className="focus-ring w-full rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-3 py-2 text-sm outline-none"
              />
            </label>
            <div>
              <p className="mb-2 text-xs uppercase tracking-wide text-[var(--text-muted)]">Avatar Rengi</p>
              <div className="flex flex-wrap items-center gap-2">
                {avatarColorOptions.map((color) => (
                  <button
                    key={color}
                    type="button"
                    onClick={() =>
                      saveProfilePrefs({
                        ...profilePrefs,
                        avatarColor: color
                      })
                    }
                    className={`focus-ring h-8 w-8 rounded-full border-2 ${profilePrefs.avatarColor === color ? "border-[var(--text-main)]" : "border-transparent"}`}
                    style={{ backgroundColor: color }}
                    aria-label={`Avatar rengi ${color}`}
                  />
                ))}
                <button
                  type="button"
                  onClick={() => saveProfilePrefs(defaultProfilePrefs)}
                  className="focus-ring rounded-lg border border-[var(--line)] px-2.5 py-1.5 text-xs text-[var(--text-main)] hover:bg-[var(--bg-soft)]"
                >
                  Varsayilan
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}
