"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import {
  createShare,
  listShares,
  revokeShare,
  type NodeRecord,
  type ShareRecord
} from "@/lib/api";
import { useAppContext } from "@/components/app-context-provider";
import { useToast } from "@/components/toast-provider";
import { formatDateTime } from "@/components/file-manager/helpers";

type Props = {
  node: NodeRecord | null;
  isOpen: boolean;
  onClose: () => void;
};

type ShareDuration = "permanent" | "1h" | "6h" | "1d" | "7d" | "custom";

function mergeSharesByID(items: ShareRecord[]): ShareRecord[] {
  const map = new Map<string, ShareRecord>();
  for (const item of items) {
    map.set(item.id, item);
  }
  return Array.from(map.values()).sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime());
}

function isShareActive(share: ShareRecord): boolean {
  if (share.revoked_at) {
    return false;
  }
  if (share.expires_at) {
    return new Date(share.expires_at).getTime() > Date.now();
  }
  return true;
}

export function ShareModal({ node, isOpen, onClose }: Props) {
  const { t, settings } = useAppContext();
  const { showToast } = useToast();
  
  const [loading, setLoading] = useState(false);
  const [shares, setShares] = useState<ShareRecord[]>([]);
  const [shareError, setShareError] = useState<string | null>(null);
  
  const [duration, setDuration] = useState<ShareDuration>("7d");
  const [customExpiry, setCustomExpiry] = useState("");
  const [password, setPassword] = useState("");
  const [allowDownload, setAllowDownload] = useState(true);
  const [busy, setBusy] = useState(false);
  const [showRevoked, setShowRevoked] = useState(false);
  
  const [copiedID, setCopiedID] = useState<string | null>(null);
  const shareLinkInputRefs = useRef<Record<string, HTMLInputElement | null>>({});

  const loadShares = useCallback(async () => {
    if (!node) return;
    setLoading(true);
    setShareError(null);
    try {
      const items = await listShares(node.id);
      setShares(mergeSharesByID(items));
    } catch (error) {
      setShareError(error instanceof Error ? error.message : t("share.listLoadFailed", "Share list could not be loaded."));
    } finally {
      setLoading(false);
    }
  }, [node, t]);

  useEffect(() => {
    if (!isOpen || !node) {
      return;
    }
    void loadShares();
  }, [isOpen, node, loadShares]);

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
        const input = shareLinkInputRefs.current[shareID];
        if (input) {
          input.focus();
          input.select();
          showToast("info", t("share.linkSelectedForManualCopy", "Link selected. Press Ctrl+C to copy."));
        } else {
          showToast("info", t("share.linkCopyManual", "Copy the visible link manually."));
        }
      } finally {
        document.body.removeChild(textarea);
      }
    }
    setTimeout(() => setCopiedID(null), 2000);
  }

  async function handleCreateShare() {
    if (!node) return;
    setBusy(true);
    setShareError(null);
    
    try {
      let expiresAt: string | undefined;
      if (duration === "custom" && customExpiry.trim()) {
        const parsed = new Date(customExpiry);
        if (Number.isNaN(parsed.getTime())) {
          setShareError(t("share.invalidExpiry", "Invalid expiration date."));
          setBusy(false);
          return;
        }
        expiresAt = parsed.toISOString();
      } else if (duration !== "permanent") {
        const now = new Date();
        switch (duration) {
          case "1h":
            now.setHours(now.getHours() + 1);
            break;
          case "6h":
            now.setHours(now.getHours() + 6);
            break;
          case "1d":
            now.setDate(now.getDate() + 1);
            break;
          case "7d":
            now.setDate(now.getDate() + 7);
            break;
        }
        expiresAt = now.toISOString();
      }

      const created = await createShare({
        node_id: node.id,
        password: password.trim() || undefined,
        expires_at: expiresAt,
        allow_download: allowDownload
      });
      
      setShares((prev) => mergeSharesByID([created, ...prev]));
      setPassword("");
      setDuration("7d");
      setCustomExpiry("");
      setShowRevoked(false);
      window.dispatchEvent(new Event("shares-updated"));
      showToast("success", t("share.created", "Share link created."));
    } catch (error) {
      showToast("error", error instanceof Error ? error.message : t("share.createFailed", "Share could not be created."));
    } finally {
      setBusy(false);
    }
  }

  async function handleRevoke(shareID: string) {
    const confirmed = window.confirm(t("share.revokeConfirm", "Revoke this share link?"));
    if (!confirmed) return;
    
    setBusy(true);
    setShareError(null);
    try {
      await revokeShare(shareID);
      setShares((prev) =>
        prev.map((item) => (item.id === shareID ? { ...item, revoked_at: new Date().toISOString() } : item))
      );
      window.dispatchEvent(new Event("shares-updated"));
      showToast("info", t("share.revoked", "Share link revoked."));
    } catch (error) {
      showToast("error", error instanceof Error ? error.message : t("share.revokeFailed", "Share revoke failed."));
    } finally {
      setBusy(false);
    }
  }

  if (!isOpen || !node) return null;
  const visibleShares = showRevoked ? shares : shares.filter(isShareActive);

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50" onClick={onClose}>
      <div className="surface-card w-full max-w-lg rounded-2xl p-6 shadow-2xl" data-testid="share-modal" onClick={e => e.stopPropagation()}>
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg font-semibold text-[var(--text-main)]">
            {t("share.modalTitle", "Share")}: {node.name}
          </h2>
          <button type="button" onClick={onClose} className="p-2 hover:bg-[var(--bg-soft)] rounded-lg">
            <i className="fa-solid fa-xmark" />
          </button>
        </div>

        <div className="space-y-4">
          <div className="space-y-3 rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] p-4">
            <div>
              <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">
                {t("share.duration", "Duration")}
              </label>
              <select
                value={duration}
                onChange={e => setDuration(e.target.value as ShareDuration)}
                className="focus-ring w-full rounded-lg border border-[var(--line)] bg-[var(--bg-card)] px-3 py-2 text-sm outline-none"
              >
                <option value="permanent">{t("share.permanent", "Permanent")}</option>
                <option value="1h">{t("share.1hour", "1 hour")}</option>
                <option value="6h">{t("share.6hours", "6 hours")}</option>
                <option value="1d">{t("share.1day", "1 day")}</option>
                <option value="7d">{t("share.7days", "7 days")}</option>
                <option value="custom">{t("share.custom", "Custom")}</option>
              </select>
              {duration === "custom" && (
                <input
                  type="datetime-local"
                  value={customExpiry}
                  onChange={e => setCustomExpiry(e.target.value)}
                  className="focus-ring mt-2 w-full rounded-lg border border-[var(--line)] bg-[var(--bg-card)] px-3 py-2 text-sm outline-none"
                />
              )}
            </div>
            
            <div>
              <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">
                {t("share.password", "Password")} ({t("share.optional", "optional")})
              </label>
              <input
                type="password"
                value={password}
                onChange={e => setPassword(e.target.value)}
                placeholder={t("share.passwordPlaceholder", "Enter password if needed")}
                className="focus-ring w-full rounded-lg border border-[var(--line)] bg-[var(--bg-card)] px-3 py-2 text-sm outline-none"
              />
            </div>
            
            <label className="inline-flex items-center gap-2 text-sm text-[var(--text-main)]">
              <input
                type="checkbox"
                checked={allowDownload}
                onChange={e => setAllowDownload(e.target.checked)}
                className="h-4 w-4 rounded border-[var(--line)]"
              />
              {t("share.allowDownload", "Allow download")}
            </label>
            
            <div className="flex gap-2 pt-2">
              <button
                type="button"
                onClick={handleCreateShare}
                disabled={busy}
                data-testid="share-create-submit"
                className="focus-ring flex-1 rounded-lg bg-[var(--brand)] px-4 py-2 text-sm font-semibold text-white hover:opacity-90 disabled:opacity-60"
              >
                {t("share.createLink", "Create share link")}
              </button>
              <button
                type="button"
                onClick={onClose}
                className="focus-ring rounded-lg border border-[var(--line)] px-4 py-2 text-sm hover:bg-[var(--bg-soft)]"
              >
                {t("common.cancel", "Cancel")}
              </button>
            </div>
            
            {shareError && <p className="text-xs text-red-500">{shareError}</p>}
          </div>

          <div className="space-y-2">
            <div className="flex items-center justify-between gap-2">
              <p className="text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)]">
                {t("share.activeShares", "Active Shares")}
              </p>
              <label className="inline-flex items-center gap-2 text-xs text-[var(--text-muted)]">
                <input
                  type="checkbox"
                  checked={showRevoked}
                  onChange={(event) => setShowRevoked(event.target.checked)}
                  className="h-3.5 w-3.5 rounded border-[var(--line)]"
                />
                {t("share.showRevoked", "Show revoked")}
              </label>
            </div>
            
            {loading ? (
              <p className="text-sm text-[var(--text-muted)]">{t("share.loading", "Loading shares...")}</p>
            ) : visibleShares.length === 0 ? (
              <p className="text-sm text-[var(--text-muted)]">{t("share.noneForFile", "There is no active share for this file.")}</p>
            ) : (
              visibleShares.map(share => (
                <div key={share.id} className="rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] p-3" data-testid="share-item">
                  <div className="flex items-center justify-between">
                    <div>
                      <p className="text-sm font-medium text-[var(--text-main)]">
                        {share.revoked_at ? t("share.statusRevoked", "Revoked") : t("share.statusActive", "Active")}
                      </p>
                      <p className="text-xs text-[var(--text-muted)]">
                        {share.expires_at 
                          ? `${t("share.expires", "Expires")}: ${formatDateTime(share.expires_at)}`
                          : t("share.permanent", "Permanent")}
                      </p>
                      <p className="text-xs text-[var(--text-muted)]">
                        {t("share.downloads", "Downloads")}: {share.download_count}
                      </p>
                    </div>
                    <div className="flex gap-2">
                      <button
                        type="button"
                        onClick={() => handleRevoke(share.id)}
                        disabled={busy || !!share.revoked_at}
                        data-testid="share-revoke-button"
                        className="focus-ring rounded border border-red-500/40 px-3 py-1.5 text-xs text-red-500 hover:bg-red-500/10 disabled:opacity-50"
                      >
                        {t("share.revoke", "Revoke")}
                      </button>
                      {share.token && !share.revoked_at && (
                        <button
                          type="button"
                          onClick={() => copyShareLink(share.id, share.token!)}
                          data-testid="share-copy-button"
                          className="focus-ring rounded border border-[var(--line)] px-3 py-1.5 text-xs hover:bg-[var(--bg-card)]"
                        >
                          {copiedID === share.id ? t("share.copied", "Copied") : t("share.copyLink", "Copy link")}
                        </button>
                      )}
                    </div>
                  </div>
                  {share.token && !share.revoked_at ? (
                    <input
                      ref={(element) => {
                        shareLinkInputRefs.current[share.id] = element;
                      }}
                      readOnly
                      value={buildPublicURL(share.token)}
                      data-testid="share-link-input"
                      className="focus-ring mt-2 w-full rounded border border-[var(--line)] bg-[var(--bg-card)] px-2 py-1.5 text-xs text-[var(--text-muted)]"
                    />
                  ) : null}
                </div>
              ))
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
