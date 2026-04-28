"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import {
  emptyTrash,
  listTrashNodes,
  permanentlyDeleteTrashNode,
  restoreTrashNode,
  type NodeRecord,
} from "@/lib/api";
import { formatBytes, formatDateTime, normalizeErrorMessage, type Flash } from "@/components/file-manager/helpers";
import { resolveNodeTypeLabel, resolveNodeTypeMeta } from "@/components/file-manager/file-type";
import { useAppContext } from "@/components/app-context-provider";

type Props = {
  runWithRefresh: <T>(operation: () => Promise<T>) => Promise<T>;
  showFlash: (tone: Flash["tone"], message: string) => void;
};

export function TrashPage({ runWithRefresh, showFlash }: Props) {
  const { t } = useAppContext();
  const [loading, setLoading] = useState(true);
  const [busyID, setBusyID] = useState<string | null>(null);
  const [emptying, setEmptying] = useState(false);
  const [nodes, setNodes] = useState<NodeRecord[]>([]);
  const [selectedNodeID, setSelectedNodeID] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const loadTrash = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const listed = await runWithRefresh(() => listTrashNodes());
      setNodes(listed);
      setSelectedNodeID((prev) => (prev && listed.some((item) => item.id === prev) ? prev : listed[0]?.id ?? null));
    } catch (err) {
      setError(normalizeErrorMessage(err));
    } finally {
      setLoading(false);
    }
  }, [runWithRefresh]);

  useEffect(() => {
    void loadTrash();
  }, [loadTrash]);

  const selectedNode = useMemo(
    () => (selectedNodeID ? nodes.find((node) => node.id === selectedNodeID) ?? null : null),
    [nodes, selectedNodeID]
  );

  const handleRestore = useCallback(
    async (node: NodeRecord) => {
      setBusyID(node.id);
      try {
        await runWithRefresh(() => restoreTrashNode(node.id));
        setNodes((prev) => prev.filter((item) => item.id !== node.id));
        setSelectedNodeID((prev) => (prev === node.id ? null : prev));
        showFlash("success", t("trash.restored", "{name} restored.", { name: node.name }));
      } catch (err) {
        showFlash("error", normalizeErrorMessage(err));
      } finally {
        setBusyID(null);
      }
    },
    [runWithRefresh, showFlash, t]
  );

  const handlePermanentDelete = useCallback(
    async (node: NodeRecord) => {
      const approved = window.confirm(
        t("trash.permanentDeleteConfirm", "Permanently delete {name}? This cannot be undone.", { name: node.name })
      );
      if (!approved) {
        return;
      }
      setBusyID(node.id);
      try {
        await runWithRefresh(() => permanentlyDeleteTrashNode(node.id));
        setNodes((prev) => prev.filter((item) => item.id !== node.id));
        setSelectedNodeID((prev) => (prev === node.id ? null : prev));
        showFlash("info", t("trash.permanentlyDeleted", "{name} permanently deleted.", { name: node.name }));
      } catch (err) {
        showFlash("error", normalizeErrorMessage(err));
      } finally {
        setBusyID(null);
      }
    },
    [runWithRefresh, showFlash, t]
  );

  const handleEmptyTrash = useCallback(async () => {
    if (nodes.length === 0) {
      return;
    }
    const approved = window.confirm(
      t("trash.emptyConfirm", "Empty trash permanently? This action cannot be undone.")
    );
    if (!approved) {
      return;
    }

    setEmptying(true);
    try {
      const result = await runWithRefresh(() => emptyTrash());
      setNodes([]);
      setSelectedNodeID(null);
      showFlash("info", t("trash.emptied", "Trash emptied. Deleted: {count}", { count: result.deleted_count }));
    } catch (err) {
      showFlash("error", normalizeErrorMessage(err));
    } finally {
      setEmptying(false);
    }
  }, [nodes.length, runWithRefresh, showFlash, t]);

  return (
    <section className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_320px]">
      <div className="surface-card overflow-hidden rounded-2xl border border-[var(--line)]">
        <div className="flex items-center justify-between border-b border-[var(--line)] bg-[var(--bg-soft)] px-4 py-3">
          <div>
            <h2 className="text-base font-semibold">{t("trash.title", "Trash")}</h2>
            <p className="text-xs text-[var(--text-muted)]">
              {t("trash.autoDeleteInfo", "Items are permanently deleted after 30 days.")}
            </p>
          </div>
          <div className="flex items-center gap-2">
            <button
              type="button"
              onClick={handleEmptyTrash}
              disabled={emptying || nodes.length === 0}
              className="focus-ring rounded-lg border border-red-500/40 px-3 py-1.5 text-xs font-medium text-red-500 hover:bg-red-500/10 disabled:opacity-60"
            >
              <i className="fa-solid fa-trash-can mr-1" />
              {t("trash.emptyAction", "Empty Trash")}
            </button>
            <button
              type="button"
              onClick={() => void loadTrash()}
              className="focus-ring rounded-lg border border-[var(--line)] px-3 py-1.5 text-xs hover:bg-[var(--bg-card)]"
            >
              {t("common.refresh", "Refresh")}
            </button>
          </div>
        </div>

        {loading ? (
          <div className="p-6 text-sm text-[var(--text-muted)]">{t("trash.loading", "Loading trash...")}</div>
        ) : error ? (
          <div className="p-6">
            <p className="text-sm text-red-500">{error}</p>
          </div>
        ) : nodes.length === 0 ? (
          <div className="p-10 text-center">
            <i className="fa-regular fa-trash-can text-3xl text-[var(--text-muted)]" />
            <p className="mt-3 text-sm text-[var(--text-main)]">{t("trash.empty", "Trash is empty.")}</p>
          </div>
        ) : (
          <div className="custom-scrollbar max-h-[calc(100vh-15.5rem)] overflow-auto">
            <table className="w-full table-fixed">
              <thead>
                <tr className="border-b border-[var(--line)] bg-[var(--bg-soft)]">
                  <th className="px-4 py-3 text-left text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)]">
                    {t("file.columns.name", "Name")}
                  </th>
                  <th className="w-32 px-4 py-3 text-left text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)]">
                    {t("file.columns.type", "Type")}
                  </th>
                  <th className="w-28 px-4 py-3 text-left text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)]">
                    {t("file.columns.size", "Size")}
                  </th>
                  <th className="w-44 px-4 py-3 text-left text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)]">
                    {t("trash.deletedAt", "Deleted")}
                  </th>
                  <th className="w-24 px-4 py-3 text-right text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)]">
                    {t("file.columns.action", "Action")}
                  </th>
                </tr>
              </thead>
              <tbody>
                {nodes.map((node) => {
                  const typeMeta = resolveNodeTypeMeta(node);
                  const selected = selectedNodeID === node.id;
                  const deletedAt = node.deleted_at ?? node.updated_at;
                  return (
                    <tr
                      key={node.id}
                      className={`border-b border-[var(--line)] ${selected ? "bg-[var(--bg-soft)]" : "hover:bg-[var(--bg-soft)]/70"}`}
                      onClick={() => setSelectedNodeID(node.id)}
                    >
                      <td className="px-4 py-3">
                        <div className="flex items-center gap-2">
                          <span className={`inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-lg ${typeMeta.wrapClass}`}>
                            <i className={typeMeta.iconClass} />
                          </span>
                          <span className="truncate text-sm font-medium text-[var(--text-main)]">{node.name}</span>
                        </div>
                      </td>
                      <td className="px-4 py-3 text-sm text-[var(--text-muted)]">{resolveNodeTypeLabel(node)}</td>
                      <td className="px-4 py-3 text-sm text-[var(--text-muted)]">
                        {node.type === "folder" ? "--" : formatBytes(node.size_bytes)}
                      </td>
                      <td className="px-4 py-3 text-sm text-[var(--text-muted)]">{formatDateTime(deletedAt)}</td>
                      <td className="px-4 py-3">
                        <div className="flex items-center justify-end gap-2">
                          <button
                            type="button"
                            onClick={(event) => {
                              event.stopPropagation();
                              void handleRestore(node);
                            }}
                            disabled={busyID === node.id}
                            aria-label={t("trash.restore", "Restore")}
                            title={t("trash.restore", "Restore")}
                            className="focus-ring rounded border border-[var(--line)] px-2 py-1.5 text-xs hover:bg-[var(--bg-card)] disabled:opacity-60"
                          >
                            <i className="fa-solid fa-rotate-left" />
                          </button>
                          <button
                            type="button"
                            onClick={(event) => {
                              event.stopPropagation();
                              void handlePermanentDelete(node);
                            }}
                            disabled={busyID === node.id}
                            aria-label={t("trash.deleteForever", "Delete forever")}
                            title={t("trash.deleteForever", "Delete forever")}
                            className="focus-ring rounded border border-red-500/40 px-2 py-1.5 text-xs text-red-500 hover:bg-red-500/10 disabled:opacity-60"
                          >
                            <i className="fa-solid fa-trash-can" />
                          </button>
                        </div>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </div>

      <aside className="surface-card hidden rounded-2xl p-4 lg:block">
        <h3 className="text-sm font-semibold text-[var(--text-main)]">{t("details.title", "Details")}</h3>
        {!selectedNode ? (
          <p className="mt-3 text-sm text-[var(--text-muted)]">{t("details.empty", "Select a file or folder to view details.")}</p>
        ) : (
          <div className="mt-3 space-y-3">
            <div className="text-sm">
              <p className="text-xs uppercase tracking-wide text-[var(--text-muted)]">{t("details.name", "Name")}</p>
              <p className="mt-1 break-all">{selectedNode.name}</p>
            </div>
            <div className="text-sm">
              <p className="text-xs uppercase tracking-wide text-[var(--text-muted)]">{t("details.type", "Type")}</p>
              <p className="mt-1">{resolveNodeTypeLabel(selectedNode)}</p>
            </div>
            <div className="text-sm">
              <p className="text-xs uppercase tracking-wide text-[var(--text-muted)]">{t("details.size", "Size")}</p>
              <p className="mt-1">{selectedNode.type === "folder" ? "--" : formatBytes(selectedNode.size_bytes)}</p>
            </div>
            <div className="text-sm">
              <p className="text-xs uppercase tracking-wide text-[var(--text-muted)]">ID</p>
              <p className="mt-1 break-all">{selectedNode.id}</p>
            </div>
          </div>
        )}
      </aside>
    </section>
  );
}
