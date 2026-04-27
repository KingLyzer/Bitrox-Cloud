"use client";

import type { AdminUser } from "@/lib/api";
import { resolveDisplayName } from "@/lib/api";
import { formatBytes, formatDateTime } from "@/components/file-manager/helpers";
import { useAppContext } from "@/components/app-context-provider";

type Props = {
  users: AdminUser[];
  loading: boolean;
  busyUserID: string | null;
  statusFilter: "all" | "active" | "inactive" | "deleted";
  onChangeStatusFilter: (status: "all" | "active" | "inactive" | "deleted") => void;
  onEdit: (user: AdminUser) => void;
  onResetPassword: (user: AdminUser) => void;
  onToggleActive: (user: AdminUser) => void;
  onReactivate: (user: AdminUser) => void;
  onDelete: (user: AdminUser) => void;
  onPermanentDelete: (user: AdminUser) => void;
};

function roleBadge(role: AdminUser["role"]): string {
  if (role === "owner") {
    return "OWNER";
  }
  if (role === "admin") {
    return "ADMIN";
  }
  return "USER";
}

function quotaState(percent: number): { barClass: string; textClass: string; label: string } {
  if (percent >= 95) {
    return {
      barClass: "bg-red-500",
      textClass: "text-red-500",
      label: "critical"
    };
  }
  if (percent >= 80) {
    return {
      barClass: "bg-amber-500",
      textClass: "text-amber-500",
      label: "warning"
    };
  }
  return {
    barClass: "bg-[var(--brand)]",
    textClass: "text-[var(--text-muted)]",
    label: "normal"
  };
}

export function UsersTable({
  users,
  loading,
  busyUserID,
  statusFilter,
  onChangeStatusFilter,
  onEdit,
  onResetPassword,
  onToggleActive,
  onReactivate,
  onDelete,
  onPermanentDelete
}: Props) {
  const { t } = useAppContext();

  const statusMeta = (user: AdminUser): { label: string; className: string; isDeleted: boolean } => {
    const current = user.status ?? (user.deleted_at ? "deleted" : user.is_active ? "active" : "inactive");
    if (current === "disabled") {
      return {
        label: t("status.disabled", "Disabled"),
        className: "bg-slate-500/15 text-slate-500",
        isDeleted: true
      };
    }
    if (current === "deleted") {
      return {
        label: t("status.deleted", "Deleted"),
        className: "bg-slate-500/15 text-slate-500",
        isDeleted: true
      };
    }
    if (current === "inactive") {
      return {
        label: t("status.inactive", "Inactive"),
        className: "bg-amber-500/15 text-amber-600",
        isDeleted: false
      };
    }
    return {
      label: t("status.active", "Active"),
      className: "bg-emerald-500/15 text-emerald-500",
      isDeleted: false
    };
  };

  return (
    <div className="surface-card custom-scrollbar overflow-x-auto rounded-2xl">
      <div className="flex flex-wrap items-center gap-2 border-b border-[var(--line)] bg-[var(--bg-soft)] px-4 py-3" data-testid="admin-user-status-filters">
        {[
          { value: "all" as const, label: t("admin.filter.all", "All") },
          { value: "active" as const, label: t("admin.filter.active", "Active") },
          { value: "inactive" as const, label: t("admin.filter.inactive", "Inactive") },
          { value: "deleted" as const, label: t("admin.filter.deleted", "Deleted") }
        ].map((item) => (
          <button
            key={item.value}
            type="button"
            onClick={() => onChangeStatusFilter(item.value)}
            className={`focus-ring rounded-lg px-2.5 py-1.5 text-xs ${
              statusFilter === item.value ? "bg-[var(--brand-soft)] font-semibold text-[var(--brand)]" : "border border-[var(--line)] hover:bg-[var(--bg-card)]"
            }`}
            data-testid={`admin-users-filter-${item.value}`}
          >
            {item.label}
          </button>
        ))}
      </div>
      <table className="w-full min-w-[980px]">
        <thead>
          <tr className="border-b border-[var(--line)] bg-[var(--bg-soft)]">
            <th className="px-4 py-3 text-left text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)]">{t("admin.users.columns.user", "User")}</th>
            <th className="px-4 py-3 text-left text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)]">{t("admin.users.columns.role", "Role")}</th>
            <th className="px-4 py-3 text-left text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)]">{t("admin.users.columns.quota", "Quota")}</th>
            <th className="px-4 py-3 text-left text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)]">{t("admin.users.columns.status", "Status")}</th>
            <th className="px-4 py-3 text-left text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)]">{t("admin.users.columns.created", "Created")}</th>
            <th className="px-4 py-3 text-right text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)]">{t("admin.users.columns.action", "Action")}</th>
          </tr>
        </thead>
        <tbody>
          {loading ? (
            <tr>
              <td colSpan={6} className="px-4 py-6 text-sm text-[var(--text-muted)]">
                {t("admin.users.loading", "Loading users...")}
              </td>
            </tr>
          ) : users.length === 0 ? (
            <tr>
              <td colSpan={6} className="px-4 py-6 text-sm text-[var(--text-muted)]">
                {t("admin.users.empty", "No users found.")}
              </td>
            </tr>
          ) : (
            users.map((user) => {
              const used = user.used_bytes ?? 0;
              const limit = user.limit_bytes ?? user.quota_bytes ?? 0;
              const quotaPercent = limit > 0 ? Math.min(100, Math.round((used / limit) * 100)) : 0;
              const quotaTone = quotaState(quotaPercent);
              const isBusy = busyUserID === user.id;
              const status = statusMeta(user);
              return (
                <tr key={user.id} className="border-b border-[var(--line)]">
                  <td className="px-4 py-3">
                    <p className="text-sm font-semibold text-[var(--text-main)]">{resolveDisplayName(user)}</p>
                    <p className="mt-1 text-xs text-[var(--text-muted)]">{user.email}</p>
                  </td>
                  <td className="px-4 py-3">
                    <span className="rounded-full bg-[var(--brand-soft)] px-2 py-1 text-xs font-semibold text-[var(--brand)]">
                      {roleBadge(user.role)}
                    </span>
                  </td>
                  <td className="px-4 py-3">
                    <p className="text-xs text-[var(--text-main)]">
                       {formatBytes(used)} / {limit > 0 ? formatBytes(limit) : t("admin.users.defaultQuota", "Default")}
                    </p>
                    <div className="mt-1 h-1.5 rounded-full bg-black/10 dark:bg-white/10">
                      <div className={`h-1.5 rounded-full ${quotaTone.barClass}`} style={{ width: `${quotaPercent}%` }} />
                    </div>
                    <p className={`mt-1 text-[11px] ${quotaTone.textClass}`}>
                      {quotaPercent}% ({t(`admin.quota.${quotaTone.label}`, quotaTone.label)})
                    </p>
                  </td>
                  <td className="px-4 py-3">
                    <span className={`rounded-full px-2 py-1 text-xs font-semibold ${status.className}`}>{status.label}</span>
                  </td>
                  <td className="px-4 py-3 text-xs text-[var(--text-muted)]">{formatDateTime(user.created_at)}</td>
                  <td className="px-4 py-3 text-right">
                    <div className="inline-flex items-center gap-2">
                      <button
                        type="button"
                        onClick={() => onEdit(user)}
                        className="focus-ring rounded-lg border border-[var(--line)] px-2.5 py-1.5 text-xs hover:bg-[var(--bg-soft)]"
                      >
                        {t("common.edit", "Edit")}
                      </button>
                      <button
                        type="button"
                        onClick={() => onResetPassword(user)}
                        disabled={isBusy}
                        className="focus-ring rounded-lg border border-[var(--line)] px-2.5 py-1.5 text-xs hover:bg-[var(--bg-soft)] disabled:opacity-60"
                        data-testid="admin-user-reset-password"
                      >
                        {t("admin.users.resetPassword", "Reset Password")}
                      </button>
                      <button
                        type="button"
                        onClick={() => (status.isDeleted ? onReactivate(user) : onToggleActive(user))}
                        disabled={isBusy}
                        className="focus-ring rounded-lg border border-[var(--line)] px-2.5 py-1.5 text-xs hover:bg-[var(--bg-soft)] disabled:opacity-60"
                        data-testid={status.isDeleted ? "admin-user-reactivate" : "admin-user-toggle-active"}
                      >
                        {status.isDeleted ? t("admin.reactivate", "Reactivate") : user.is_active ? t("status.inactive", "Inactive") : t("admin.reactivate", "Reactivate")}
                      </button>
                      {status.isDeleted ? (
                        <button
                          type="button"
                          onClick={() => onPermanentDelete(user)}
                          disabled={isBusy}
                          className="focus-ring rounded-lg border border-red-500/40 px-2.5 py-1.5 text-xs text-red-500 hover:bg-red-500/10 disabled:opacity-60"
                          data-testid="admin-user-permanent-delete"
                        >
                          {t("admin.users.permanentDelete", "Delete Permanently")}
                        </button>
                      ) : (
                        <button
                          type="button"
                          onClick={() => onDelete(user)}
                          disabled={isBusy}
                          className="focus-ring rounded-lg border border-red-500/40 px-2.5 py-1.5 text-xs text-red-500 hover:bg-red-500/10 disabled:opacity-60"
                        >
                          {t("admin.users.disable", "Disable")}
                        </button>
                      )}
                    </div>
                  </td>
                </tr>
              );
            })
          )}
        </tbody>
      </table>
    </div>
  );
}
