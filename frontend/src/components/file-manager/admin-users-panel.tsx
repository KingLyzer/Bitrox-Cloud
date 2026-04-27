"use client";

import { useEffect, useMemo, useState } from "react";
import type { AdminUser } from "@/lib/api";
import { formatDateTime } from "@/components/file-manager/helpers";

type Role = "owner" | "admin" | "user";

type UserDraft = {
  displayName: string;
  role: Role;
  isActive: boolean;
  quotaGB: string;
  useDefaultQuota: boolean;
  newPassword: string;
};

type CreatePayload = {
  email: string;
  displayName: string;
  role: Role;
  password: string;
  quotaBytes: number | null;
  isActive: boolean;
};

type UpdatePayload = {
  userID: string;
  displayName: string;
  role: Role;
  isActive: boolean;
  quotaBytes?: number;
  useDefaultQuota?: boolean;
};

type Props = {
  users: AdminUser[];
  loading: boolean;
  busy: boolean;
  onRefresh: () => void;
  onCreateUser: (payload: CreatePayload) => Promise<void>;
  onUpdateUser: (payload: UpdatePayload) => Promise<void>;
  onUpdatePassword: (userID: string, newPassword: string) => Promise<void>;
};

const roleOptions: Role[] = ["owner", "admin", "user"];

function toDraft(user: AdminUser): UserDraft {
  const normalizedRole: Role = user.role === "owner" || user.role === "admin" ? user.role : "user";
  return {
    displayName: user.display_name,
    role: normalizedRole,
    isActive: user.is_active,
    quotaGB: user.quota_bytes ? (user.quota_bytes / 1024 / 1024 / 1024).toFixed(2) : "",
    useDefaultQuota: user.quota_bytes === null,
    newPassword: ""
  };
}

function usernameFromEmail(email: string): string {
  const idx = email.indexOf("@");
  if (idx <= 0) {
    return email;
  }
  return email.slice(0, idx);
}

function normalizeQuotaGB(raw: string): number | null {
  const normalized = Number(raw.trim());
  if (!Number.isFinite(normalized) || normalized <= 0) {
    return null;
  }
  return Math.round(normalized * 1024 * 1024 * 1024);
}

export function AdminUsersPanel({
  users,
  loading,
  busy,
  onRefresh,
  onCreateUser,
  onUpdateUser,
  onUpdatePassword
}: Props) {
  const [drafts, setDrafts] = useState<Record<string, UserDraft>>({});

  const [newEmail, setNewEmail] = useState("");
  const [newDisplayName, setNewDisplayName] = useState("");
  const [newRole, setNewRole] = useState<Role>("user");
  const [newPassword, setNewPassword] = useState("");

  useEffect(() => {
    const next: Record<string, UserDraft> = {};
    for (const user of users) {
      next[user.id] = toDraft(user);
    }
    setDrafts(next);
  }, [users]);

  const userCount = useMemo(() => users.length, [users]);

  return (
    <section className="space-y-4">
      <div className="surface-card rounded-2xl p-4">
        <div className="mb-2 flex items-center justify-between">
          <h2 className="text-base font-semibold text-[var(--text-main)]">Kullanici Yonetimi</h2>
          <button
            type="button"
            onClick={onRefresh}
            disabled={loading || busy}
            className="focus-ring rounded-lg border border-[var(--line)] px-3 py-1.5 text-sm text-[var(--text-main)] hover:bg-[var(--bg-soft)] disabled:opacity-60"
          >
            Yenile
          </button>
        </div>
        <p className="text-sm text-[var(--text-muted)]">
          Toplam kullanici: <span className="font-semibold text-[var(--text-main)]">{userCount}</span>
        </p>
      </div>

      <div className="surface-card rounded-2xl p-4">
        <h3 className="text-sm font-semibold text-[var(--text-main)]">Kullanici ekle</h3>
        <div className="mt-3 grid gap-2 md:grid-cols-[1fr_1fr_160px_1fr_auto]">
          <input
            value={newDisplayName}
            onChange={(event) => setNewDisplayName(event.target.value)}
            placeholder="Kullanici Adi"
            className="focus-ring rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-3 py-2 text-sm text-[var(--text-main)] outline-none"
          />
          <input
            value={newEmail}
            onChange={(event) => setNewEmail(event.target.value)}
            placeholder="E-Posta"
            type="email"
            className="focus-ring rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-3 py-2 text-sm text-[var(--text-main)] outline-none"
          />
          <select
            value={newRole}
            onChange={(event) => setNewRole(event.target.value as Role)}
            className="focus-ring rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-3 py-2 text-sm text-[var(--text-main)] outline-none"
          >
            {roleOptions.map((role) => (
              <option key={role} value={role}>
                {role}
              </option>
            ))}
          </select>
          <input
            value={newPassword}
            onChange={(event) => setNewPassword(event.target.value)}
            placeholder="Parola (min 12)"
            type="password"
            className="focus-ring rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-3 py-2 text-sm text-[var(--text-main)] outline-none"
          />
          <button
            type="button"
            disabled={busy || loading || newEmail.trim() === "" || newDisplayName.trim() === "" || newPassword.trim().length < 12}
            onClick={() => {
              void onCreateUser({
                email: newEmail.trim(),
                displayName: newDisplayName.trim(),
                role: newRole,
                password: newPassword.trim(),
                quotaBytes: null,
                isActive: true
              }).then(() => {
                setNewEmail("");
                setNewDisplayName("");
                setNewPassword("");
                setNewRole("user");
              });
            }}
            className="focus-ring rounded-lg bg-[var(--brand)] px-4 py-2 text-sm font-semibold text-white hover:brightness-110 disabled:opacity-60"
          >
            Olustur
          </button>
        </div>
      </div>

      <div className="surface-card custom-scrollbar overflow-x-auto rounded-2xl">
        <table className="w-full min-w-[1160px]">
          <thead>
            <tr className="border-b border-[var(--line)] bg-[var(--bg-soft)]">
              <th className="px-4 py-3 text-left text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)]">Kullanici Adi</th>
              <th className="px-4 py-3 text-left text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)]">Tam Adi</th>
              <th className="px-4 py-3 text-left text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)]">Parola</th>
              <th className="px-4 py-3 text-left text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)]">Gruplar</th>
              <th className="px-4 py-3 text-left text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)]">Grup Yoneticisi</th>
              <th className="px-4 py-3 text-left text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)]">Kota</th>
              <th className="px-4 py-3 text-left text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)]">Aksiyon</th>
            </tr>
          </thead>
          <tbody>
            {loading ? (
              <tr>
                <td colSpan={7} className="px-4 py-6 text-sm text-[var(--text-muted)]">
                  Yukleniyor...
                </td>
              </tr>
            ) : users.length === 0 ? (
              <tr>
                <td colSpan={7} className="px-4 py-6 text-sm text-[var(--text-muted)]">
                  Kullanici bulunamadi.
                </td>
              </tr>
            ) : (
              users.map((user) => {
                const draft = drafts[user.id] ?? toDraft(user);
                const quotaMode = draft.useDefaultQuota ? "default" : "custom";

                return (
                  <tr key={user.id} className="border-b border-[var(--line)]">
                    <td className="px-4 py-3">
                      <p className="text-base font-semibold text-[var(--text-main)]">{usernameFromEmail(user.email)}</p>
                      <p className="mt-1 text-sm text-[var(--text-muted)]">{user.email}</p>
                      <p className="mt-1 text-xs text-[var(--text-muted)]">Olusturma: {formatDateTime(user.created_at)}</p>
                    </td>
                    <td className="px-4 py-3">
                      <input
                        value={draft.displayName}
                        onChange={(event) =>
                          setDrafts((prev) => ({ ...prev, [user.id]: { ...draft, displayName: event.target.value } }))
                        }
                        className="focus-ring w-full rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-3 py-2 text-sm text-[var(--text-main)] outline-none"
                      />
                    </td>
                    <td className="px-4 py-3">
                      <p className="mb-2 text-xs tracking-[0.15em] text-[var(--text-muted)]">********</p>
                      <div className="flex items-center gap-2">
                        <input
                          value={draft.newPassword}
                          onChange={(event) =>
                            setDrafts((prev) => ({ ...prev, [user.id]: { ...draft, newPassword: event.target.value } }))
                          }
                          type="password"
                          placeholder="Yeni sifre"
                          className="focus-ring w-44 rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-2 py-1.5 text-xs text-[var(--text-main)] outline-none"
                        />
                        <button
                          type="button"
                          disabled={busy || draft.newPassword.trim().length < 12}
                          onClick={() =>
                            void onUpdatePassword(user.id, draft.newPassword).then(() => {
                              setDrafts((prev) => ({ ...prev, [user.id]: { ...draft, newPassword: "" } }));
                            })
                          }
                          className="focus-ring rounded-lg border border-[var(--line)] px-2 py-1.5 text-xs text-[var(--text-main)] hover:bg-[var(--bg-soft)] disabled:opacity-60"
                        >
                          Guncelle
                        </button>
                      </div>
                    </td>
                    <td className="px-4 py-3">
                      <select
                        value={draft.role}
                        onChange={(event) =>
                          setDrafts((prev) => ({ ...prev, [user.id]: { ...draft, role: event.target.value as Role } }))
                        }
                        className="focus-ring rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-2 py-1.5 text-sm text-[var(--text-main)] outline-none"
                      >
                        {roleOptions.map((role) => (
                          <option key={role} value={role}>
                            {role}
                          </option>
                        ))}
                      </select>
                    </td>
                    <td className="px-4 py-3">
                      <select
                        disabled
                        value="none"
                        className="focus-ring rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-2 py-1.5 text-sm text-[var(--text-main)] outline-none disabled:opacity-70"
                      >
                        <option value="none">grup yok</option>
                      </select>
                    </td>
                    <td className="px-4 py-3">
                      <div className="space-y-2">
                        <select
                          value={quotaMode}
                          onChange={(event) =>
                            setDrafts((prev) => ({
                              ...prev,
                              [user.id]: {
                                ...draft,
                                useDefaultQuota: event.target.value === "default"
                              }
                            }))
                          }
                          className="focus-ring rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-2 py-1.5 text-sm text-[var(--text-main)] outline-none"
                        >
                          <option value="default">Varsayilan</option>
                          <option value="custom">Ozel (GB)</option>
                        </select>
                        {!draft.useDefaultQuota ? (
                          <input
                            value={draft.quotaGB}
                            onChange={(event) =>
                              setDrafts((prev) => ({ ...prev, [user.id]: { ...draft, quotaGB: event.target.value } }))
                            }
                            placeholder="GB"
                            className="focus-ring w-24 rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-2 py-1.5 text-xs text-[var(--text-main)] outline-none"
                          />
                        ) : null}
                      </div>
                    </td>
                    <td className="px-4 py-3">
                      <button
                        type="button"
                        disabled={busy}
                        onClick={() => {
                          const payload: UpdatePayload = {
                            userID: user.id,
                            displayName: draft.displayName.trim(),
                            role: draft.role,
                            isActive: draft.isActive,
                            useDefaultQuota: draft.useDefaultQuota
                          };
                          if (!draft.useDefaultQuota) {
                            const quotaBytes = normalizeQuotaGB(draft.quotaGB);
                            if (quotaBytes) {
                              payload.quotaBytes = quotaBytes;
                            }
                          }
                          void onUpdateUser(payload);
                        }}
                        className="focus-ring rounded-lg bg-[var(--brand)] px-3 py-1.5 text-xs font-semibold text-white hover:brightness-110 disabled:opacity-60"
                      >
                        Kaydet
                      </button>
                    </td>
                  </tr>
                );
              })
            )}
          </tbody>
        </table>
      </div>
    </section>
  );
}
