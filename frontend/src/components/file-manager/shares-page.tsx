"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import {
  downloadUrlForNode,
  listShares,
  revokeShare,
  type ShareRecord,
} from "@/lib/api";
import { formatBytes, formatDateTime, normalizeErrorMessage } from "@/components/file-manager/helpers";
import { useAppContext } from "@/components/app-context-provider";
import { useToast } from "@/components/toast-provider";
import { resolveNodeTypeLabel, resolveNodeTypeMeta } from "@/components/file-manager/file-type";

type ShareWithNode = ShareRecord & {
  node_id: string;
};

function isShareActive(share: ShareRecord): boolean {
  if (share.revoked_at) {
    return false;
  }
  if (!share.expires_at) {
    return true;
  }
  return new Date(share.expires_at).getTime() > Date.now();
}

export function SharesPage() {
  const { t, settings } = useAppContext();
  const { showToast } = useToast();
  const [loading, setLoading] = useState(true);
  const [shares, setShares] = useState<ShareWithNode[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [filter, setFilter] = useState<"active" | "expired" | "revoked">("active");
  const [copiedID, setCopiedID] = useState<string | null>(null);
  const [revokingID, setRevokingID] = useState<string | null>(null);
  const [selectedShareID, setSelectedShareID] = useState<string | null>(null);

  const triggerDownload = useCallback(
    async (share: ShareWithNode) => {
      try {
        const response = await fetch(downloadUrlForNode(share.node_id), {
          method: "GET",
          credentials: "include"
        });
        if (!response.ok) {
          throw new Error(t("file.downloadFailedHttp", "Download failed (HTTP {status}).", { status: response.status }));
        }
        const blob = await response.blob();
        const disposition = response.headers.get("content-disposition");
        const fallbackName = (share.node_name ?? "").trim() || (share.node_deleted ? t("share.deletedNode", "Deleted item") : t("share.unknownNode", "Unknown item"));
        const fileName =
          disposition?.match(/filename\*?=(?:UTF-8''|\"?)([^\";]+)/i)?.[1]?.replace(/^UTF-8''/, "")?.trim() ||
          fallbackName ||
          "download";
        const objectURL = URL.createObjectURL(blob);
        const link = document.createElement("a");
        link.href = objectURL;
        link.download = decodeURIComponent(fileName).replace(/[\\/:*?"<>|]+/g, "_");
        link.style.display = "none";
        document.body.appendChild(link);
        link.click();
        document.body.removeChild(link);
        window.setTimeout(() => URL.revokeObjectURL(objectURL), 1500);
      } catch (error) {
        showToast("error", normalizeErrorMessage(error));
      }
    },
    [showToast, t]
  );

  const loadShares = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const items = await listShares();
      const normalized: ShareWithNode[] = items.map((share) => ({
        ...share,
        node_id: share.node_id,
      }));
      setShares(normalized);
      setSelectedShareID((prev) => (prev && normalized.some((item) => item.id === prev) ? prev : normalized[0]?.id ?? null));
    } catch (err) {
      setError(normalizeErrorMessage(err));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void loadShares();
  }, [loadShares]);

  function buildPublicURL(token: string): string {
    const encodedToken = encodeURIComponent(token);
    const base = settings.public_base_url?.trim().replace(/\/+$/g, "");
    if (base) {
      return `${base}/s/${encodedToken}`;
    }
    if (typeof window !== "undefined") {
      return `${window.location.origin}/s/${encodedToken}`;
    }
    return `/s/${encodedToken}`;
  }

  async function copyShareLink(shareID: string, token: string) {
    const url = buildPublicURL(token);
    try {
      if (typeof navigator !== "undefined" && navigator.clipboard && window.isSecureContext) {
        await navigator.clipboard.writeText(url);
        setCopiedID(shareID);
        showToast("success", t("share.linkCopied", "Share link copied."));
      } else {
        throw new Error("clipboard_unavailable");
      }
    } catch {
      const textarea = document.createElement("textarea");
      textarea.value = url;
      textarea.style.position = "fixed";
      textarea.style.opacity = "0";
      document.body.appendChild(textarea);
      textarea.select();
      try {
        const copied = document.execCommand("copy");
        if (!copied) {
          throw new Error("copy_failed");
        }
        setCopiedID(shareID);
        showToast("success", t("share.linkCopied", "Share link copied."));
      } catch {
        showToast("info", t("share.linkCopyManual", "Copy the visible link manually."));
      } finally {
        document.body.removeChild(textarea);
      }
    }
    setTimeout(() => setCopiedID(null), 2000);
  }

  const filteredShares = useMemo(
    () =>
      shares.filter((share) => {
        if (filter === "active") return isShareActive(share);
        if (filter === "revoked") return !!share.revoked_at;
        if (filter === "expired") return !!share.expires_at && new Date(share.expires_at).getTime() <= Date.now() && !share.revoked_at;
        return true;
      }),
    [filter, shares]
  );

  const selectedShare = useMemo(
    () => (selectedShareID ? filteredShares.find((item) => item.id === selectedShareID) ?? null : null),
    [filteredShares, selectedShareID]
  );

  const resolveShareName = (share: ShareWithNode): string => {
    const name = share.node_name?.trim();
    if (name) {
      return name;
    }
    if (share.node_deleted) {
      return t("share.deletedNode", "Deleted item");
    }
    return t("share.unknownNode", "Unknown item");
  };

  const resolveShareType = (share: ShareWithNode): string => {
    if (share.node_type === "file" || share.node_type === "folder") {
      return resolveNodeTypeLabel({
        id: share.node_id,
        owner_user_id: share.owner_user_id,
        parent_id: null,
        type: share.node_type,
        name: resolveShareName(share),
        size_bytes: share.size_bytes ?? share.size ?? 0,
        mime_type: share.mime_type ?? undefined,
        created_at: share.created_at,
        updated_at: share.node_updated_at ?? share.updated_at,
      });
    }
    return t("details.type", "Type");
  };

  const handleRevoke = async (shareID: string) => {
    const confirmed = window.confirm(t("share.revokeConfirm", "Revoke this share link?"));
    if (!confirmed) {
      return;
    }
    setRevokingID(shareID);
    try {
      await revokeShare(shareID);
      setShares((prev) => prev.map((item) => (item.id === shareID ? { ...item, revoked_at: new Date().toISOString() } : item)));
      showToast("info", t("share.revoked", "Share link revoked."));
      window.dispatchEvent(new Event("shares-updated"));
    } catch (error) {
      showToast("error", normalizeErrorMessage(error));
    } finally {
      setRevokingID(null);
    }
  };

  return (
    <section className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_320px]">
      <div className="space-y-4">
        <div className="flex items-center justify-between">
          <h1 className="text-xl font-semibold text-[var(--text-main)]">{t("sidebar.shares", "Shares")}</h1>
          <div className="flex gap-2">
            <button
              type="button"
              onClick={() => setFilter("active")}
              className={`focus-ring rounded-lg px-3 py-1.5 text-sm ${
                filter === "active" ? "bg-[var(--brand)] text-white" : "border border-[var(--line)] hover:bg-[var(--bg-soft)]"
              }`}
            >
              {t("share.statusActive", "Active")}
            </button>
            <button
              type="button"
              onClick={() => setFilter("expired")}
              className={`focus-ring rounded-lg px-3 py-1.5 text-sm ${
                filter === "expired" ? "bg-[var(--brand)] text-white" : "border border-[var(--line)] hover:bg-[var(--bg-soft)]"
              }`}
            >
              {t("share.expired", "Expired")}
            </button>
            <button
              type="button"
              onClick={() => setFilter("revoked")}
              className={`focus-ring rounded-lg px-3 py-1.5 text-sm ${
                filter === "revoked" ? "bg-[var(--brand)] text-white" : "border border-[var(--line)] hover:bg-[var(--bg-soft)]"
              }`}
            >
              {t("share.statusRevoked", "Revoked")}
            </button>
          </div>
        </div>

        {loading ? (
          <div className="surface-card rounded-2xl p-6">
            <p className="text-sm text-[var(--text-muted)]">{t("share.loading", "Loading shares...")}</p>
          </div>
        ) : error ? (
          <div className="surface-card rounded-2xl p-6">
            <p className="text-sm text-red-500">{error}</p>
            <button
              type="button"
              onClick={() => void loadShares()}
              className="focus-ring mt-3 rounded-lg bg-[var(--brand)] px-3 py-2 text-sm font-semibold text-white"
            >
              {t("common.retry", "Retry")}
            </button>
          </div>
        ) : filteredShares.length === 0 ? (
          <div className="surface-card rounded-2xl p-10 text-center">
            <i className="fa-solid fa-share-nodes text-3xl text-[var(--text-muted)]" />
            <p className="mt-3 text-sm text-[var(--text-main)]">
              {filter === "active"
                ? t("shares.emptyActive", "No active shares.")
                : filter === "expired"
                ? t("shares.emptyExpired", "No expired shares.")
                : t("shares.emptyRevoked", "No revoked shares.")}
            </p>
          </div>
        ) : (
          <div className="overflow-x-auto rounded-2xl border border-[var(--line)]">
            <table className="w-full">
              <thead>
                <tr className="border-b border-[var(--line)] bg-[var(--bg-soft)]">
                  <th className="px-4 py-3 text-left text-xs font-semibold uppercase text-[var(--text-muted)]">{t("file.columns.name", "Name")}</th>
                  <th className="px-4 py-3 text-left text-xs font-semibold uppercase text-[var(--text-muted)]">{t("details.type", "Type")}</th>
                  <th className="px-4 py-3 text-left text-xs font-semibold uppercase text-[var(--text-muted)]">{t("share.expires", "Expires")}</th>
                  <th className="px-4 py-3 text-left text-xs font-semibold uppercase text-[var(--text-muted)]">{t("share.downloads", "Downloads")}</th>
                  <th className="px-4 py-3 text-right text-xs font-semibold uppercase text-[var(--text-muted)]">{t("file.columns.action", "Action")}</th>
                </tr>
              </thead>
              <tbody>
                {filteredShares.map((share) => {
                  const isExpired = share.expires_at && new Date(share.expires_at).getTime() <= Date.now();
                  const isRevoked = !!share.revoked_at;
                  const shareName = resolveShareName(share);
                  const typeMeta = resolveNodeTypeMeta({
                    id: share.node_id,
                    owner_user_id: share.owner_user_id,
                    parent_id: null,
                    type: share.node_type === "folder" ? "folder" : "file",
                    name: shareName,
                    size_bytes: share.size_bytes ?? share.size ?? 0,
                    mime_type: share.mime_type ?? undefined,
                    created_at: share.created_at,
                    updated_at: share.node_updated_at ?? share.updated_at,
                  });
                  const selected = selectedShareID === share.id;
                  return (
                    <tr
                      key={share.id}
                      className={`border-b border-[var(--line)] ${selected ? "bg-[var(--bg-soft)] shadow-[inset_3px_0_0_var(--brand)]" : "hover:bg-[var(--bg-soft)]"}`}
                      data-testid="share-row"
                      onClick={() => setSelectedShareID(share.id)}
                    >
                      <td className="px-4 py-3">
                        <div className="flex items-center gap-2">
                          <span className={`inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-lg ${typeMeta.wrapClass}`}>
                            <i className={typeMeta.iconClass} />
                          </span>
                          <span className="text-sm font-medium text-[var(--text-main)]" data-testid="share-row-node-name">
                            {shareName}
                          </span>
                        </div>
                      </td>
                      <td className="px-4 py-3 text-sm text-[var(--text-muted)]">{resolveShareType(share)}</td>
                      <td className="px-4 py-3 text-sm text-[var(--text-muted)]">
                        {isRevoked
                          ? t("share.statusRevoked", "Revoked")
                          : isExpired
                          ? t("share.expired", "Expired")
                          : share.expires_at
                          ? formatDateTime(share.expires_at)
                          : t("share.permanent", "Permanent")}
                      </td>
                      <td className="px-4 py-3 text-sm text-[var(--text-muted)]">{share.download_count}</td>
                      <td className="px-4 py-3 text-right">
                        {!isRevoked ? (
                          <button
                            type="button"
                            onClick={(event) => {
                              event.stopPropagation();
                              void handleRevoke(share.id);
                            }}
                            data-testid="share-revoke-button"
                            disabled={revokingID === share.id}
                            className="focus-ring rounded border border-red-500/40 px-3 py-1.5 text-xs text-red-500 hover:bg-red-500/10 disabled:opacity-60"
                          >
                            {t("share.revoke", "Revoke")}
                          </button>
                        ) : null}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </div>

      <aside className="surface-card hidden rounded-2xl p-4 xl:block">
        <h2 className="text-sm font-semibold text-[var(--text-main)]">{t("details.title", "Details")}</h2>
        {!selectedShare ? (
          <p className="mt-3 text-sm text-[var(--text-muted)]">{t("details.empty", "Select a file or folder to view details.")}</p>
        ) : (
          <div className="mt-4 space-y-3">
            <div className="text-sm">
              <p className="text-xs uppercase tracking-wide text-[var(--text-muted)]">{t("details.name", "Name")}</p>
              <p className="mt-1 break-all text-[var(--text-main)]">{resolveShareName(selectedShare)}</p>
            </div>
            <div className="text-sm">
              <p className="text-xs uppercase tracking-wide text-[var(--text-muted)]">{t("details.type", "Type")}</p>
              <p className="mt-1 text-[var(--text-main)]">{resolveShareType(selectedShare)}</p>
            </div>
            <div className="text-sm">
              <p className="text-xs uppercase tracking-wide text-[var(--text-muted)]">{t("details.size", "Size")}</p>
              <p className="mt-1 text-[var(--text-main)]">
                {selectedShare.node_type === "folder" ? "--" : formatBytes(selectedShare.size_bytes ?? selectedShare.size ?? 0)}
              </p>
            </div>
            <div className="text-sm">
              <p className="text-xs uppercase tracking-wide text-[var(--text-muted)]">{t("details.updated", "Updated")}</p>
              <p className="mt-1 text-[var(--text-main)]">{formatDateTime(selectedShare.node_updated_at ?? selectedShare.updated_at)}</p>
            </div>
            <div className="text-sm">
              <p className="text-xs uppercase tracking-wide text-[var(--text-muted)]">ID</p>
              <p className="mt-1 break-all text-[var(--text-main)]">{selectedShare.node_id}</p>
            </div>
            {selectedShare.token ? (
              <>
                <div className="text-sm">
                  <p className="text-xs uppercase tracking-wide text-[var(--text-muted)]">{t("share.link", "Share Link")}</p>
                  <input
                    readOnly
                    value={buildPublicURL(selectedShare.token)}
                    data-testid="share-link-input"
                    className="focus-ring mt-1 w-full rounded border border-[var(--line)] bg-[var(--bg-soft)] px-2 py-1.5 text-xs text-[var(--text-main)]"
                  />
                </div>
                <div className="grid grid-cols-2 gap-2">
                  <button
                    type="button"
                    onClick={() => void copyShareLink(selectedShare.id, selectedShare.token!)}
                    className="focus-ring rounded border border-[var(--line)] px-2 py-2 text-xs hover:bg-[var(--bg-soft)]"
                    data-testid="share-copy-button"
                  >
                    {copiedID === selectedShare.id ? t("share.copied", "Copied") : t("share.copyLink", "Copy link")}
                  </button>
                  {!selectedShare.revoked_at ? (
                    <button
                      type="button"
                      onClick={() => void handleRevoke(selectedShare.id)}
                      disabled={revokingID === selectedShare.id}
                      className="focus-ring rounded border border-red-500/40 px-2 py-2 text-xs text-red-500 hover:bg-red-500/10 disabled:opacity-60"
                    >
                      {t("share.revoke", "Revoke")}
                    </button>
                  ) : null}
                </div>
              </>
            ) : null}
            {selectedShare.node_type === "file" && !selectedShare.node_deleted ? (
              <>
                <button
                  type="button"
                  onClick={() => void triggerDownload(selectedShare)}
                  className="focus-ring w-full rounded-lg border border-[var(--line)] px-2.5 py-2 text-sm hover:bg-[var(--bg-soft)]"
                >
                  {t("common.download", "Download")}
                </button>
                {(selectedShare.mime_type ?? "").startsWith("image/") ? (
                  <div className="rounded-xl border border-[var(--line)] bg-[var(--bg-soft)] p-2">
                    {/* eslint-disable-next-line @next/next/no-img-element */}
                    <img
                      src={downloadUrlForNode(selectedShare.node_id)}
                      alt={resolveShareName(selectedShare)}
                      className="max-h-52 w-full rounded-lg object-contain"
                    />
                  </div>
                ) : null}
              </>
            ) : null}
            {selectedShare.node_deleted ? (
              <p className="text-xs text-[var(--text-muted)]">{t("share.deletedNode", "Deleted item")}</p>
            ) : null}
          </div>
        )}
      </aside>
    </section>
  );
}
