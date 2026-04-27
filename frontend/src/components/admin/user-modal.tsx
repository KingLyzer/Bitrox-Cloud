"use client";

import { useEffect, useState } from "react";
import type { AdminRole, AdminUserFormValues } from "@/components/admin/types";
import { useAppContext } from "@/components/app-context-provider";

type Props = {
  mode: "create" | "edit";
  open: boolean;
  title: string;
  emailLocked?: boolean;
  initialValues: AdminUserFormValues;
  busy: boolean;
  onClose: () => void;
  onSubmit: (values: AdminUserFormValues) => Promise<void>;
};

const roleOptions: AdminRole[] = ["owner", "admin", "user"];

export function UserModal({
  mode,
  open,
  title,
  emailLocked = false,
  initialValues,
  busy,
  onClose,
  onSubmit
}: Props) {
  const { t } = useAppContext();
  const [values, setValues] = useState<AdminUserFormValues>(initialValues);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    setValues(initialValues);
    setError(null);
  }, [initialValues, open]);

  if (!open) {
    return null;
  }

  return (
    <div className="modal-overlay fixed inset-0 z-50 grid place-items-center p-4">
      <div className="surface-card w-full max-w-2xl rounded-2xl p-5">
        <div className="mb-4 flex items-center justify-between">
          <h2 className="text-lg font-semibold text-[var(--text-main)]">{title}</h2>
          <button type="button" onClick={onClose} className="focus-ring rounded-lg px-2 py-1 text-sm hover:bg-[var(--bg-soft)]">
            <i className="fa-solid fa-xmark" />
          </button>
        </div>

        <form
          className="grid gap-3 md:grid-cols-2"
          onSubmit={(event) => {
            event.preventDefault();
            setError(null);
            void onSubmit(values).catch((err: unknown) => {
              const message = err instanceof Error ? err.message : t("admin.userModal.error", "Operation failed.");
              setError(message);
            });
          }}
        >
          <label className="block md:col-span-2">
            <span className="mb-1 block text-xs uppercase tracking-wide text-[var(--text-muted)]">{t("admin.userModal.displayName", "Display Name")}</span>
            <input
              required
              value={values.displayName}
              onChange={(event) => setValues((prev) => ({ ...prev, displayName: event.target.value }))}
              className="focus-ring w-full rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-3 py-2 text-sm outline-none"
            />
          </label>

          <label className="block">
            <span className="mb-1 block text-xs uppercase tracking-wide text-[var(--text-muted)]">{t("admin.userModal.email", "Email")}</span>
            <input
              required
              type="email"
              value={values.email}
              disabled={emailLocked}
              onChange={(event) => setValues((prev) => ({ ...prev, email: event.target.value }))}
              className="focus-ring w-full rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-3 py-2 text-sm outline-none disabled:opacity-70"
            />
          </label>

          <label className="block">
            <span className="mb-1 block text-xs uppercase tracking-wide text-[var(--text-muted)]">{t("admin.userModal.role", "Role")}</span>
            <select
              value={values.role}
              onChange={(event) => setValues((prev) => ({ ...prev, role: event.target.value as AdminRole }))}
              className="focus-ring w-full rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-3 py-2 text-sm outline-none"
            >
              {roleOptions.map((role) => (
                <option key={role} value={role}>
                    {t(`admin.role.${role}`, role)}
                </option>
              ))}
            </select>
          </label>

          {mode === "create" ? (
            <label className="block md:col-span-2">
              <span className="mb-1 block text-xs uppercase tracking-wide text-[var(--text-muted)]">{t("admin.userModal.tempPassword", "Temporary password (min 12)")}</span>
              <input
                required
                minLength={12}
                type="password"
                value={values.password}
                onChange={(event) => setValues((prev) => ({ ...prev, password: event.target.value }))}
                className="focus-ring w-full rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-3 py-2 text-sm outline-none"
              />
            </label>
          ) : null}

          <label className="inline-flex items-center gap-2">
            <input
              type="checkbox"
              checked={values.useDefaultQuota}
              onChange={(event) => setValues((prev) => ({ ...prev, useDefaultQuota: event.target.checked }))}
              className="h-4 w-4 rounded border-[var(--line)]"
            />
            <span className="text-sm text-[var(--text-main)]">{t("admin.userModal.defaultQuota", "Use default quota")}</span>
          </label>

          <label className="block">
            <span className="mb-1 block text-xs uppercase tracking-wide text-[var(--text-muted)]">{t("admin.userModal.quotaGb", "Quota (GB)")}</span>
            <input
              type="number"
              step="0.1"
              min="0.1"
              disabled={values.useDefaultQuota}
              value={values.quotaGb}
              onChange={(event) => setValues((prev) => ({ ...prev, quotaGb: event.target.value }))}
              className="focus-ring w-full rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-3 py-2 text-sm outline-none disabled:opacity-70"
            />
          </label>

          <label className="inline-flex items-center gap-2 md:col-span-2">
            <input
              type="checkbox"
              checked={values.isActive}
              onChange={(event) => setValues((prev) => ({ ...prev, isActive: event.target.checked }))}
              className="h-4 w-4 rounded border-[var(--line)]"
            />
            <span className="text-sm text-[var(--text-main)]">{t("admin.userModal.active", "Account active")}</span>
          </label>

          {error ? <p className="text-sm text-red-500 md:col-span-2">{error}</p> : null}

          <div className="mt-2 flex items-center justify-end gap-2 md:col-span-2">
            <button
              type="button"
              onClick={onClose}
              className="focus-ring rounded-lg border border-[var(--line)] px-3 py-2 text-sm hover:bg-[var(--bg-soft)]"
            >
              {t("common.cancel", "Cancel")}
            </button>
            <button
              type="submit"
              disabled={busy}
              className="focus-ring rounded-lg bg-[var(--brand)] px-4 py-2 text-sm font-semibold text-white hover:brightness-110 disabled:opacity-60"
            >
              {mode === "create" ? t("admin.userModal.create", "Create User") : t("admin.userModal.saveChanges", "Save Changes")}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
