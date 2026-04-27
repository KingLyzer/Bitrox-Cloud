"use client";

import { useEffect, useRef, useState, type ReactNode } from "react";
import type { NotificationRecord } from "@/lib/api";
import { formatDateTime, type UploadJob } from "@/components/file-manager/helpers";
import { useAppContext } from "@/components/app-context-provider";

type Props = {
  appName: string;
  pageLabel: string;
  runningUploadCount: number;
  uploadJobs: UploadJob[];
  onRetryUpload: (job: UploadJob) => void;
  onCancelUpload: (job: UploadJob) => void;
  notifications?: NotificationRecord[];
  onMarkNotificationRead?: (notificationID: string) => void;
  rightSlot?: ReactNode;
};

export function WorkspaceHeader({
  appName,
  pageLabel,
  runningUploadCount,
  uploadJobs,
  onRetryUpload,
  onCancelUpload,
  notifications = [],
  onMarkNotificationRead,
  rightSlot
}: Props) {
  const { t } = useAppContext();
  const [open, setOpen] = useState(false);
  const panelRef = useRef<HTMLDivElement | null>(null);
  const previousRunningCountRef = useRef(0);

  const unreadNotificationCount = notifications.filter((item) => !item.is_read).length;
  const bellBadgeCount = runningUploadCount + unreadNotificationCount;

  useEffect(() => {
    if (runningUploadCount > previousRunningCountRef.current) {
      setOpen(true);
    }
    previousRunningCountRef.current = runningUploadCount;
  }, [runningUploadCount]);

  useEffect(() => {
    if (!open) {
      return;
    }
    const handlePointerDown = (event: MouseEvent) => {
      const target = event.target as Node | null;
      if (!target) {
        return;
      }
      if (panelRef.current?.contains(target)) {
        return;
      }
      setOpen(false);
    };
    window.addEventListener("pointerdown", handlePointerDown);
    return () => window.removeEventListener("pointerdown", handlePointerDown);
  }, [open]);

  return (
    <header className="mb-4 rounded-xl border border-[var(--line)] bg-[var(--bg-card)] px-4 py-3 shadow-sm">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <p className="text-lg font-semibold tracking-wide text-[var(--text-main)]">{appName}</p>
          <p className="text-xs uppercase tracking-wide text-[var(--text-muted)]">{pageLabel}</p>
        </div>
        <div className="flex items-center gap-2">
          <div ref={panelRef} className="relative">
            <button
              type="button"
              onClick={() => setOpen((prev) => !prev)}
              data-testid="notifications-toggle-button"
              className="focus-ring relative rounded-lg border border-[var(--line)] px-3 py-2 text-sm hover:bg-[var(--bg-soft)]"
              aria-label={t("notifications.title", "Notifications")}
            >
              <i className="fa-regular fa-bell" />
              {bellBadgeCount > 0 ? (
                <span className="absolute -right-1 -top-1 grid h-5 min-w-5 place-items-center rounded-full bg-[var(--brand)] px-1 text-[10px] font-semibold text-white">
                  {bellBadgeCount}
                </span>
              ) : null}
            </button>

            {open ? (
              <div className="absolute right-0 top-full z-40 mt-2 w-[340px] rounded-xl border border-[var(--line)] bg-[var(--bg-card)] p-2 shadow-2xl" data-testid="notifications-panel">
                <p className="px-2 pb-2 text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)]">{t("notifications.center", "Notification Center")}</p>
                <div className="custom-scrollbar max-h-[340px] space-y-2 overflow-y-auto pr-1">
                  {notifications.length > 0 ? (
                    <div className="space-y-2 rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] p-2">
                      <p className="text-[11px] font-semibold uppercase tracking-wide text-[var(--text-muted)]">{t("notifications.calendarReminders", "Calendar Reminders")}</p>
                      {notifications.map((notification) => (
                        <article
                          key={notification.id}
                          className="rounded-md border border-[var(--line)] bg-[var(--bg-card)] p-2"
                          data-testid="calendar-notification-item"
                        >
                          <p className="text-sm font-medium text-[var(--text-main)]">{notification.title}</p>
                          <p className="mt-0.5 text-xs text-[var(--text-muted)]">{notification.message}</p>
                          <div className="mt-1 flex items-center justify-between gap-2">
                            <span className="text-[11px] text-[var(--text-muted)]">{formatDateTime(notification.created_at)}</span>
                            {!notification.is_read && onMarkNotificationRead ? (
                              <button
                                type="button"
                                onClick={() => onMarkNotificationRead(notification.id)}
                                data-testid="notification-mark-read-button"
                                className="focus-ring rounded border border-[var(--line)] px-2 py-0.5 text-[11px] hover:bg-[var(--bg-soft)]"
                              >
                                {t("notifications.markRead", "Mark Read")}
                              </button>
                            ) : (
                              <span className="text-[11px] text-[var(--text-muted)]">{t("notifications.read", "Read")}</span>
                            )}
                          </div>
                        </article>
                      ))}
                    </div>
                  ) : null}

                  {uploadJobs.length > 0 ? (
                    <div className="space-y-2 rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] p-2">
                      <p className="text-[11px] font-semibold uppercase tracking-wide text-[var(--text-muted)]">{t("upload.queue", "Upload Queue")}</p>
                      {uploadJobs.map((job) => (
                        <div key={job.id} className="rounded-lg border border-[var(--line)] bg-[var(--bg-card)] p-2" data-testid="upload-job-item" data-upload-status={job.status}>
                          <div className="flex items-center justify-between gap-2">
                            <p className="truncate text-sm font-medium text-[var(--text-main)]">{job.fileName}</p>
                            <span className="text-[11px] uppercase text-[var(--text-muted)]">
                              {job.status === "running" ? t("status.running", "RUNNING") : job.status === "done" ? t("status.done", "DONE") : job.status === "cancelled" ? t("status.cancelled", "CANCELLED") : t("status.error", "ERROR")}
                            </span>
                          </div>
                          <div className="mt-2 h-1.5 rounded-full bg-black/10 dark:bg-white/10">
                            <div
                              className={`h-1.5 rounded-full ${job.status === "error" || job.status === "cancelled" ? "bg-red-500" : "bg-[var(--brand)]"}`}
                              style={{ width: `${job.progress}%` }}
                            />
                          </div>
                          <p className="mt-1 text-xs text-[var(--text-muted)]">
                            {job.status === "running" ? `${job.detail} ${t("upload.progressSuffix", "uploaded")}` : job.status === "done" ? t("status.completed", "Completed") : job.detail}
                          </p>
                          {job.status === "running" ? (
                            <button
                              type="button"
                              onClick={() => onCancelUpload(job)}
                              data-testid="upload-cancel-button"
                              className="focus-ring mt-2 rounded-md border border-red-500/30 px-2 py-1 text-xs text-red-500 hover:bg-red-500/10"
                            >
                              {t("common.cancel", "Cancel")}
                            </button>
                          ) : null}
                          {job.status === "error" || job.status === "cancelled" ? (
                            <button
                              type="button"
                              onClick={() => onRetryUpload(job)}
                              data-testid="upload-retry-button"
                              className="focus-ring mt-2 rounded-md border border-[var(--line)] px-2 py-1 text-xs hover:bg-[var(--bg-soft)]"
                            >
                              {t("common.retry", "Retry")}
                            </button>
                          ) : null}
                        </div>
                      ))}
                    </div>
                  ) : null}

                  {notifications.length === 0 && uploadJobs.length === 0 ? (
                    <p className="px-2 py-3 text-sm text-[var(--text-muted)]">{t("notifications.empty", "No notifications yet.")}</p>
                  ) : null}
                </div>
              </div>
            ) : null}
          </div>
          {rightSlot}
        </div>
      </div>
    </header>
  );
}
