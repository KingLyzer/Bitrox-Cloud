"use client";

import type { AdminAuditEvent } from "@/lib/api";
import { formatDateTime } from "@/components/file-manager/helpers";
import { useAppContext } from "@/components/app-context-provider";

type Props = {
  events: AdminAuditEvent[];
  loading: boolean;
  page: number;
  totalPages: number;
  totalItems: number;
  eventType: string;
  search: string;
  onEventTypeChange: (value: string) => void;
  onSearchChange: (value: string) => void;
  onPageChange: (value: number) => void;
};

function eventLabel(eventType: string, t: (key: string, fallback?: string) => string): string {
  switch (eventType) {
    case "auth.login":
      return t("admin.audit.event.authLogin", "Login");
    case "files.upload.finalized":
      return t("admin.audit.event.uploadFinalized", "Upload");
    case "files.node.deleted":
      return t("admin.audit.event.fileDeleted", "Delete");
    case "admin.user.soft_deleted":
      return t("admin.audit.event.userDisabled", "User Disabled");
    case "admin.user.updated":
      return t("admin.audit.event.userUpdated", "User Updated");
    case "admin.user.created":
      return t("admin.audit.event.userCreated", "User Created");
    case "admin.user.password_updated":
      return t("admin.audit.event.userPasswordUpdated", "Password Updated");
    case "admin.user.permanently_deleted":
      return t("admin.audit.event.userPermanentlyDeleted", "User Permanently Deleted");
    case "auth.logout":
      return t("admin.audit.event.authLogout", "Logout");
    default:
      return eventType;
  }
}

export function AuditList({
  events,
  loading,
  page,
  totalPages,
  totalItems,
  eventType,
  search,
  onEventTypeChange,
  onSearchChange,
  onPageChange
}: Props) {
  const { t } = useAppContext();
  const eventTypeOptions: Array<{ value: string; label: string }> = [
    { value: "all", label: t("admin.audit.filter.allEvents", "All events") },
    { value: "auth.login", label: t("admin.audit.event.authLogin", "Login") },
    { value: "auth.logout", label: t("admin.audit.event.authLogout", "Logout") },
    { value: "files.upload.finalized", label: t("admin.audit.event.uploadFinalized", "Upload") },
    { value: "files.node.deleted", label: t("admin.audit.event.fileDeleted", "File Delete") },
    { value: "admin.user.created", label: t("admin.audit.event.userCreated", "User Created") },
    { value: "admin.user.updated", label: t("admin.audit.event.userUpdated", "User Updated") },
    { value: "admin.user.password_updated", label: t("admin.audit.event.userPasswordUpdated", "Password Updated") },
    { value: "admin.user.soft_deleted", label: t("admin.audit.event.userDisabled", "User Disabled") },
    { value: "admin.user.permanently_deleted", label: t("admin.audit.event.userPermanentlyDeleted", "User Permanently Deleted") }
  ];

  return (
    <div className="surface-card rounded-2xl p-4">
      <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
        <h3 className="text-base font-semibold text-[var(--text-main)]">{t("admin.tabs.audit", "Recent Activity")}</h3>
        <span className="text-xs text-[var(--text-muted)]">{t("admin.audit.total", "Total records: {count}", { count: totalItems })}</span>
      </div>

      <div className="mb-3 grid gap-2 md:grid-cols-[220px_minmax(0,1fr)_auto_auto]">
        <select
          value={eventType}
          onChange={(event) => onEventTypeChange(event.target.value)}
          className="focus-ring rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-3 py-2 text-sm outline-none"
        >
          {eventTypeOptions.map((option) => (
            <option key={option.value} value={option.value}>
              {option.label}
            </option>
          ))}
        </select>
        <input
          value={search}
          onChange={(event) => onSearchChange(event.target.value)}
          placeholder={t("admin.audit.searchPlaceholder", "Search user ID or email")}
          className="focus-ring rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-3 py-2 text-sm outline-none"
        />
        <button
          type="button"
          disabled={page <= 1}
          onClick={() => onPageChange(page - 1)}
          className="focus-ring rounded-lg border border-[var(--line)] px-3 py-2 text-sm disabled:opacity-60"
        >
          {t("common.previous", "Previous")}
        </button>
        <button
          type="button"
          disabled={totalPages <= 0 || page >= totalPages}
          onClick={() => onPageChange(page + 1)}
          className="focus-ring rounded-lg border border-[var(--line)] px-3 py-2 text-sm disabled:opacity-60"
        >
          {t("common.next", "Next")}
        </button>
      </div>

      <p className="mb-2 text-xs text-[var(--text-muted)]">
        {t("common.page", "Page")} {page}
        {totalPages > 0 ? ` / ${totalPages}` : ""}
      </p>

      <div className="custom-scrollbar max-h-80 space-y-2 overflow-y-auto pr-1">
        {loading ? (
          <p className="text-sm text-[var(--text-muted)]">{t("admin.audit.loading", "Loading audit records...")}</p>
        ) : events.length === 0 ? (
          <p className="text-sm text-[var(--text-muted)]">{t("admin.audit.empty", "No records found.")}</p>
        ) : (
          events.map((event, index) => (
            <div key={`${event.event_type}-${event.created_at}-${index}`} className="rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] p-3">
              <div className="flex items-center justify-between gap-2">
                <p className="text-sm font-semibold text-[var(--text-main)]">{eventLabel(event.event_type, t)}</p>
                <span className="text-[11px] uppercase text-[var(--text-muted)]">{event.severity}</span>
              </div>
              <p className="mt-1 text-xs text-[var(--text-muted)]">{event.created_at_human ?? formatDateTime(event.created_at)}</p>
              {event.user_id ? <p className="mt-1 text-xs text-[var(--text-muted)]">{t("admin.audit.userId", "User ID")}: {event.user_id}</p> : null}
              {event.user_email ? <p className="mt-1 text-xs text-[var(--text-muted)]">{t("admin.audit.email", "Email")}: {event.user_email}</p> : null}
              <div className="mt-2 grid gap-1 text-xs text-[var(--text-muted)] md:grid-cols-3">
                <p data-testid="audit-ip">IP: {event.ip || t("common.unknown", "Unknown")}</p>
                <p data-testid="audit-device">{t("admin.audit.device", "Device")}: {event.device_type || t("common.unknown", "Unknown")}</p>
                <p data-testid="audit-browser">{t("admin.audit.browser", "Browser")}: {event.browser || t("common.unknown", "Unknown")}</p>
              </div>
              {event.user_agent_short ? <p className="mt-1 text-[11px] text-[var(--text-muted)]">UA: {event.user_agent_short}</p> : null}
            </div>
          ))
        )}
      </div>
    </div>
  );
}
