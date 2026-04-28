"use client";

import { useEffect, useState } from "react";
import {
  downloadUrlForNode,
  type NodeRecord
} from "@/lib/api";
import { isImagePreviewableNode, isTextEditableNode, resolveNodeTypeLabel, resolveNodeTypeMeta } from "@/components/file-manager/file-type";
import { formatBytes, formatDateTime } from "@/components/file-manager/helpers";
import { useAppContext } from "@/components/app-context-provider";
import { ShareModal } from "./share-modal";

type Props = {
  node: NodeRecord | null;
  onOpenTextFile: (node: NodeRecord) => void;
  onDownloadFile: (node: NodeRecord) => void;
};

function field(label: string, value: string) {
  return (
    <div>
      <p className="text-[11px] uppercase tracking-wide text-[var(--text-muted)]">{label}</p>
      <p className="mt-0.5 break-all text-sm text-[var(--text-main)]">{value}</p>
    </div>
  );
}

export function DetailsPanel({ node, onOpenTextFile, onDownloadFile }: Props) {
  const { t } = useAppContext();
  const [shareModalOpen, setShareModalOpen] = useState(false);
  const [textPreview, setTextPreview] = useState("");
  const [textPreviewLoading, setTextPreviewLoading] = useState(false);
  const [textPreviewError, setTextPreviewError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;

    if (!node || node.type !== "file" || !isTextEditableNode(node)) {
      setTextPreview("");
      setTextPreviewLoading(false);
      setTextPreviewError(null);
      return () => {
        cancelled = true;
      };
    }

    setTextPreview("");
    setTextPreviewError(null);
    setTextPreviewLoading(true);

    void fetch(downloadUrlForNode(node.id), {
      method: "GET",
      credentials: "include"
    })
      .then(async (response) => {
        if (!response.ok) {
          throw new Error(t("preview.loadFailedHttp", "Preview could not be loaded (HTTP {status}).", { status: response.status }));
        }
        const content = await response.text();
        if (!cancelled) {
          const clipped = content.length > 4000 ? `${content.slice(0, 4000)}\n\n...` : content;
          setTextPreview(clipped);
        }
      })
      .catch((error) => {
        if (!cancelled) {
          setTextPreviewError(error instanceof Error ? error.message : t("preview.loadFailed", "Preview could not be loaded."));
        }
      })
      .finally(() => {
        if (!cancelled) {
          setTextPreviewLoading(false);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [node, t]);

  if (!node) {
    return (
      <aside className="surface-card hidden w-[320px] shrink-0 self-start rounded-2xl p-4 lg:block lg:sticky lg:top-2 lg:max-h-[calc(100vh-8rem)] lg:overflow-y-auto" data-testid="details-panel">
        <h2 className="text-sm font-semibold text-[var(--text-main)]">{t("details.title", "Details")}</h2>
        <p className="mt-3 text-sm text-[var(--text-muted)]">{t("details.empty", "Select a file or folder to view details.")}</p>
      </aside>
    );
  }

  const typeMeta = resolveNodeTypeMeta(node);

  return (
    <>
      <aside className="surface-card hidden w-[320px] shrink-0 self-start rounded-2xl p-4 lg:block lg:sticky lg:top-2 lg:max-h-[calc(100vh-8rem)] lg:overflow-y-auto" data-testid="details-panel">
        <div className="sticky top-0 z-10 -mx-1 mb-2 flex items-center justify-between gap-2 bg-[var(--bg-card)] px-1 py-1">
          <h2 className="text-sm font-semibold text-[var(--text-main)]">{t("details.title", "Details")}</h2>
        </div>
        <div className="mt-4 space-y-3">
          <div className={`inline-flex h-10 w-10 items-center justify-center rounded-lg ${typeMeta.wrapClass}`}>
            <i className={typeMeta.iconClass} />
          </div>
          {field(t("details.name", "Name"), node.name)}
          {field(t("details.type", "Type"), resolveNodeTypeLabel(node))}
          {field(t("details.size", "Size"), node.type === "folder" ? "--" : formatBytes(node.size_bytes))}
          {field(t("details.updated", "Updated"), formatDateTime(node.updated_at))}
          {field("ID", node.id)}
        </div>
        {node.type === "file" ? (
          <div className="mt-4">
            <button
              type="button"
              onClick={() => setShareModalOpen(true)}
              data-testid="share-create-button"
              className="focus-ring w-full rounded-lg bg-[var(--brand)] px-2.5 py-2 text-xs font-semibold text-white hover:opacity-90"
            >
              <i className="fa-solid fa-share-nodes mr-1" />
              {t("share.openModal", "Create share link")}
            </button>
          </div>
        ) : null}

        <div className="mt-5 border-t border-[var(--line)] pt-4">
          <p className="text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)]">{t("preview.title", "Preview")}</p>
          {node.type === "folder" ? (
            <div className="mt-2 space-y-2">
              <p className="text-sm text-[var(--text-muted)]">{t("preview.folderUnavailable", "No inline preview for folders.")}</p>
              <button
                type="button"
                onClick={() => onDownloadFile(node)}
                data-testid="preview-folder-download"
                className="focus-ring w-full rounded-lg border border-[var(--line)] px-2.5 py-2 text-sm hover:bg-[var(--bg-soft)]"
              >
                {t("preview.downloadFolderZip", "Download folder as ZIP")}
              </button>
            </div>
          ) : isTextEditableNode(node) ? (
            <div className="mt-2 space-y-2">
              <div className="grid grid-cols-2 gap-2">
                <button
                  type="button"
                  onClick={() => onOpenTextFile(node)}
                  className="focus-ring rounded-lg border border-[var(--line)] px-2.5 py-2 text-sm hover:bg-[var(--bg-soft)]"
                >
                  {t("preview.openInEditor", "Open in editor")}
                </button>
                <button
                  type="button"
                  onClick={() => onDownloadFile(node)}
                  data-testid="preview-text-download"
                  className="focus-ring rounded-lg border border-[var(--line)] px-2.5 py-2 text-sm hover:bg-[var(--bg-soft)]"
                >
                  {t("common.download", "Download")}
                </button>
              </div>
              <div className="max-h-48 overflow-auto rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] p-2" data-testid="preview-text-container">
                {textPreviewLoading ? <p className="text-xs text-[var(--text-muted)]">{t("preview.loading", "Loading preview...")}</p> : null}
                {!textPreviewLoading && textPreviewError ? <p className="text-xs text-red-500">{textPreviewError}</p> : null}
                {!textPreviewLoading && !textPreviewError ? (
                  <pre className="whitespace-pre-wrap break-words font-mono text-xs text-[var(--text-main)]" data-testid="preview-text-content">
                    {textPreview || t("preview.emptyFile", "(Empty file)")}
                  </pre>
                ) : null}
              </div>
            </div>
          ) : isImagePreviewableNode(node) ? (
            <div className="mt-2">
              {/* eslint-disable-next-line @next/next/no-img-element */}
              <img
                src={downloadUrlForNode(node.id)}
                alt={node.name}
                data-testid="preview-image"
                className="max-h-56 w-full rounded-lg border border-[var(--line)] object-contain bg-[var(--bg-soft)]"
                loading="lazy"
              />
            </div>
          ) : (
            <div className="mt-2 space-y-2">
              <p className="text-sm text-[var(--text-muted)]">{t("preview.unsupported", "Built-in preview is not available for this file type.")}</p>
              <button
                type="button"
                onClick={() => onDownloadFile(node)}
                data-testid="preview-fallback-download"
                className="focus-ring w-full rounded-lg border border-[var(--line)] px-2.5 py-2 text-sm hover:bg-[var(--bg-soft)]"
              >
                {t("common.download", "Download")}
              </button>
            </div>
          )}
        </div>

        {node.type === "file" ? <div className="mt-2 border-t border-[var(--line)] pt-2" /> : null}
      </aside>
      <ShareModal
        node={node}
        isOpen={shareModalOpen}
        onClose={() => setShareModalOpen(false)}
      />
    </>
  );
}
