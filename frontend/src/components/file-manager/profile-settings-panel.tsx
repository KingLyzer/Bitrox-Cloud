"use client";

import { useMemo } from "react";
import { useAppContext } from "@/components/app-context-provider";

export type ProfilePrefs = {
  avatarColor: string;
};

type Props = {
  displayName: string;
  preferredLanguage: "en" | "tr";
  onDisplayNameChange: (value: string) => void;
  onPreferredLanguageChange: (value: "en" | "tr") => void;
  onSaveProfile: () => void;
  currentPassword: string;
  newPassword: string;
  confirmPassword: string;
  onCurrentPasswordChange: (value: string) => void;
  onNewPasswordChange: (value: string) => void;
  onConfirmPasswordChange: (value: string) => void;
  onChangePassword: () => void;
  passwordSaving: boolean;
  saving: boolean;
  profilePrefs: ProfilePrefs;
  saveProfilePrefs: (prefs: ProfilePrefs) => void;
  defaultProfilePrefs: ProfilePrefs;
  avatarColorOptions: string[];
  effectiveAvatarInitials: string;
  userEmail: string;
};

export function ProfileSettingsPanel({
  displayName,
  preferredLanguage,
  onDisplayNameChange,
  onPreferredLanguageChange,
  onSaveProfile,
  currentPassword,
  newPassword,
  confirmPassword,
  onCurrentPasswordChange,
  onNewPasswordChange,
  onConfirmPasswordChange,
  onChangePassword,
  passwordSaving,
  saving,
  profilePrefs,
  saveProfilePrefs,
  defaultProfilePrefs,
  avatarColorOptions,
  effectiveAvatarInitials,
  userEmail
}: Props) {
  const { t } = useAppContext();
  const languageOptions = useMemo(
    () => [
      { value: "en" as const, label: t("language.english", "English") },
      { value: "tr" as const, label: t("language.turkish", "Turkish") }
    ],
    [t]
  );

  return (
    <section className="space-y-4">
      <div className="surface-card rounded-2xl p-5">
        <h2 className="text-lg font-semibold">{t("profile.title", "Profile")}</h2>
        <p className="mt-1 text-sm text-[var(--text-muted)]">{t("profile.subtitle", "Profile settings are saved in your account.")}</p>

        <div className="mt-5 grid gap-5 lg:grid-cols-[120px_minmax(0,1fr)]">
          <div className="flex flex-col items-center gap-3">
            <div className="grid h-20 w-20 place-items-center rounded-full text-2xl font-semibold text-white" style={{ backgroundColor: profilePrefs.avatarColor }}>
              {effectiveAvatarInitials}
            </div>
            <p className="text-xs text-[var(--text-muted)]">{userEmail}</p>
          </div>

          <div className="space-y-4">
            <label className="block">
              <span className="mb-1 block text-xs uppercase tracking-wide text-[var(--text-muted)]">{t("profile.displayName", "Display Name")}</span>
              <input
                value={displayName}
                onChange={(event) => onDisplayNameChange(event.target.value)}
                className="focus-ring w-full rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-3 py-2 text-sm outline-none"
                data-testid="profile-display-name-input"
              />
            </label>

            <label className="block">
              <span className="mb-1 block text-xs uppercase tracking-wide text-[var(--text-muted)]">{t("profile.language", "Language")}</span>
              <select
                value={preferredLanguage}
                onChange={(event) => onPreferredLanguageChange(event.target.value === "tr" ? "tr" : "en")}
                className="focus-ring w-full rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-3 py-2 text-sm outline-none"
                data-testid="profile-language-select"
              >
                {languageOptions.map((option) => (
                  <option key={option.value} value={option.value}>
                    {option.label}
                  </option>
                ))}
              </select>
            </label>

            <div>
              <p className="mb-2 text-xs uppercase tracking-wide text-[var(--text-muted)]">{t("profile.avatarColor", "Avatar Color")}</p>
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
                    aria-label={t("profile.avatarColorOption", "Avatar color {color}", { color })}
                  />
                ))}
                <button
                  type="button"
                  onClick={() => saveProfilePrefs(defaultProfilePrefs)}
                  className="focus-ring rounded-lg border border-[var(--line)] px-2.5 py-1.5 text-xs text-[var(--text-main)] hover:bg-[var(--bg-soft)]"
                >
                  {t("profile.resetAvatarColor", "Default")}
                </button>
              </div>
            </div>

            <div className="pt-2">
              <button
                type="button"
                onClick={onSaveProfile}
                disabled={saving}
                className="focus-ring rounded-lg bg-[var(--brand)] px-4 py-2 text-sm font-semibold text-white hover:brightness-110 disabled:opacity-60"
                data-testid="profile-save-button"
              >
                {saving ? t("common.saving", "Saving...") : t("profile.save", "Save Profile")}
              </button>
            </div>
          </div>
        </div>
      </div>

      <div className="surface-card rounded-2xl p-5">
        <h3 className="text-base font-semibold">{t("profile.passwordSection", "Change Password")}</h3>
        <p className="mt-1 text-sm text-[var(--text-muted)]">{t("profile.passwordSectionDesc", "Use at least 12 characters.")}</p>
        <div className="mt-4 grid gap-3 md:grid-cols-3">
          <label className="block">
            <span className="mb-1 block text-xs uppercase tracking-wide text-[var(--text-muted)]">{t("profile.currentPassword", "Current Password")}</span>
            <input
              type="password"
              value={currentPassword}
              onChange={(event) => onCurrentPasswordChange(event.target.value)}
              className="focus-ring w-full rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-3 py-2 text-sm outline-none"
              data-testid="profile-current-password-input"
            />
          </label>
          <label className="block">
            <span className="mb-1 block text-xs uppercase tracking-wide text-[var(--text-muted)]">{t("profile.newPassword", "New Password")}</span>
            <input
              type="password"
              value={newPassword}
              onChange={(event) => onNewPasswordChange(event.target.value)}
              className="focus-ring w-full rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-3 py-2 text-sm outline-none"
              data-testid="profile-new-password-input"
            />
          </label>
          <label className="block">
            <span className="mb-1 block text-xs uppercase tracking-wide text-[var(--text-muted)]">{t("profile.confirmPassword", "Confirm Password")}</span>
            <input
              type="password"
              value={confirmPassword}
              onChange={(event) => onConfirmPasswordChange(event.target.value)}
              className="focus-ring w-full rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-3 py-2 text-sm outline-none"
              data-testid="profile-confirm-password-input"
            />
          </label>
        </div>
        <button
          type="button"
          onClick={onChangePassword}
          disabled={passwordSaving}
          className="focus-ring mt-4 rounded-lg bg-[var(--brand)] px-4 py-2 text-sm font-semibold text-white hover:brightness-110 disabled:opacity-60"
          data-testid="profile-change-password-button"
        >
          {passwordSaving ? t("common.saving", "Saving...") : t("profile.changePassword", "Change Password")}
        </button>
      </div>
    </section>
  );
}
