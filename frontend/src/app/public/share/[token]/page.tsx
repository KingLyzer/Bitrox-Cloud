"use client";

import { FormEvent, useEffect, useMemo, useState } from "react";
import { ApiError, fetchPublicShare, publicShareDownloadURL, unlockPublicShare } from "@/lib/api";
import { useAppContext } from "@/components/app-context-provider";

type Props = {
  params: {
    token: string;
  };
};

export default function PublicSharePage({ params }: Props) {
  const { settings, t } = useAppContext();
  const token = useMemo(() => decodeURIComponent(params.token), [params.token]);
  const [accessGrant, setAccessGrant] = useState<string | undefined>(undefined);
  const [password, setPassword] = useState("");
  const [share, setShare] = useState<Awaited<ReturnType<typeof fetchPublicShare>> | null>(null);
  const [needsPassword, setNeedsPassword] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function loadShare() {
    setLoading(true);
    setError(null);
    try {
      const item = await fetchPublicShare(token, accessGrant);
      setShare(item);
      setNeedsPassword(false);
    } catch (err) {
      if (err instanceof ApiError && err.code === "share_locked") {
        setNeedsPassword(true);
        setShare(null);
      } else {
        setError(err instanceof Error ? err.message : t("share.public.loadFailed", "Share could not be loaded."));
      }
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void loadShare();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [token, accessGrant]);

  async function handleUnlock(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setLoading(true);
    setError(null);
    try {
      const result = await unlockPublicShare(token, password);
      if (result.access_grant) {
        setAccessGrant(result.access_grant);
      }
      setPassword("");
    } catch (err) {
      setError(err instanceof Error ? err.message : t("share.public.unlockFailed", "Unlock failed."));
      setLoading(false);
    }
  }

  return (
    <main className="min-h-screen bg-[var(--bg-main)] p-6 text-[var(--text-main)]">
      <div className="mx-auto max-w-2xl rounded-2xl border border-[var(--line)] bg-[var(--bg-card)] p-6 shadow-sm">
        <div className="mb-4">
          <p className="text-xs uppercase tracking-wide text-[var(--text-muted)]">{settings.site_name || "BitroxCloud"}</p>
          <h1 className="text-xl font-semibold">{t("share.public.title", "Shared File")}</h1>
        </div>
        {loading ? <p className="mt-3 text-sm text-[var(--text-muted)]">{t("common.loading", "Loading...")}</p> : null}
        {error ? <p className="mt-3 text-sm text-red-500">{error}</p> : null}

        {needsPassword ? (
          <form className="mt-4 space-y-3" onSubmit={handleUnlock}>
            <p className="text-sm text-[var(--text-muted)]">{t("share.public.passwordProtected", "This share is password protected.")}</p>
            <input
              type="password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              placeholder={t("share.password", "Password")}
              className="focus-ring w-full rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-3 py-2 text-sm outline-none"
            />
            <button
              type="submit"
              disabled={loading || password.trim() === ""}
              className="focus-ring rounded-lg bg-[var(--brand)] px-4 py-2 text-sm font-semibold text-white hover:brightness-110 disabled:opacity-60"
            >
              {t("share.public.unlock", "Unlock")}
            </button>
          </form>
        ) : null}

        {share ? (
          <div className="mt-4 space-y-3">
            <div className="rounded-xl border border-[var(--line)] bg-[var(--bg-soft)] p-3">
              <p className="text-sm font-semibold">{share.node_name}</p>
              <p className="mt-1 text-xs text-[var(--text-muted)]">{t("details.type", "Type")}: {share.node_type}</p>
              <p className="mt-1 text-xs text-[var(--text-muted)]">{t("details.size", "Size")}: {share.size_bytes} {t("share.public.bytes", "bytes")}</p>
            </div>
            {share.allow_download ? (
              <a
                href={publicShareDownloadURL(token, accessGrant)}
              className="focus-ring inline-flex items-center rounded-lg bg-[var(--brand)] px-4 py-2 text-sm font-semibold text-white hover:brightness-110"
              >
                {t("common.download", "Download")}
              </a>
            ) : (
              <p className="text-sm text-[var(--text-muted)]">{t("share.public.downloadDisabled", "Download is disabled for this share.")}</p>
            )}
          </div>
        ) : null}
      </div>
    </main>
  );
}
