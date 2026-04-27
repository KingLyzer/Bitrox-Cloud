"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { createPortal } from "react-dom";
import type { NodeRecord } from "@/lib/api";
import { isTextEditableNode, resolveNodeTypeMeta } from "@/components/file-manager/file-type";
import type { SortDirection, SortKey } from "@/components/file-manager/helpers";
import { formatBytes, formatDateTime } from "@/components/file-manager/helpers";
import { useAppContext } from "@/components/app-context-provider";

type Props = {
  nodes: NodeRecord[];
  loading: boolean;
  error: string | null;
  selectedNodeIDs: string[];
  selectedNodeIDForDetails: string | null;
  sharedNodeIDs: Record<string, boolean>;
  viewMode: "list" | "grid";
  sortKey: SortKey;
  sortDirection: SortDirection;
  menuNodeID: string | null;
  onRetry: () => void;
  onChangeSort: (key: SortKey) => void;
  onToggleSelectAll: () => void;
  onToggleSelectNode: (nodeID: string) => void;
  onSelectForDetails: (nodeID: string) => void;
  onOpenFolder: (node: NodeRecord) => void;
  onRenameNode: (node: NodeRecord) => void;
  onDeleteNode: (node: NodeRecord) => void;
  onSetMenuNodeID: (nodeID: string | null) => void;
  onDownloadNode: (node: NodeRecord) => void;
  onOpenFile: (node: NodeRecord) => void;
  emptyState?: {
    title: string;
    description: string;
    actionLabel?: string;
    onAction?: () => void;
  };
};

export function FileTable({
  nodes,
  loading,
  error,
  selectedNodeIDs,
  selectedNodeIDForDetails,
  sharedNodeIDs,
  viewMode,
  sortKey,
  sortDirection,
  menuNodeID,
  onRetry,
  onChangeSort,
  onToggleSelectAll,
  onToggleSelectNode,
  onSelectForDetails,
  onOpenFolder,
  onRenameNode,
  onDeleteNode,
  onSetMenuNodeID,
  onDownloadNode,
  onOpenFile,
  emptyState
}: Props) {
  const { t } = useAppContext();
  const allSelected = nodes.length > 0 && selectedNodeIDs.length === nodes.length;
  const hasPartialSelection = selectedNodeIDs.length > 0 && !allSelected;
  const menuWidth = 160;
  const menuHeightEstimate = 140;
  const viewportMargin = 8;
  const menuButtonRefs = useRef<Record<string, HTMLButtonElement | null>>({});
  const headerCheckboxRef = useRef<HTMLInputElement | null>(null);
  const floatingMenuRef = useRef<HTMLDivElement | null>(null);
  const [floatingMenuPosition, setFloatingMenuPosition] = useState<{ top: number; left: number } | null>(null);
  const sortLabel = (key: SortKey) => {
    switch (key) {
      case "name":
        return t("file.columns.name", "Name");
      case "type":
        return t("file.columns.type", "Type");
      case "size":
        return t("file.columns.size", "Size");
      case "updated":
        return t("file.columns.updated", "Updated");
      default:
        return t("file.columns.name", "Name");
    }
  };

  const activeMenuNode = useMemo(() => {
    if (!menuNodeID) {
      return null;
    }
    return nodes.find((node) => node.id === menuNodeID) ?? null;
  }, [menuNodeID, nodes]);

  const closeMenu = useCallback(() => {
    onSetMenuNodeID(null);
  }, [onSetMenuNodeID]);

  const isRowInteractionTarget = useCallback((target: EventTarget | null): boolean => {
    if (!(target instanceof HTMLElement)) {
      return false;
    }
    return !!target.closest('input,button,a,[data-no-row-open="1"]');
  }, []);

  const updateFloatingMenuPosition = useCallback(() => {
    if (!menuNodeID || viewMode !== "list") {
      setFloatingMenuPosition(null);
      return;
    }
    const anchor = menuButtonRefs.current[menuNodeID];
    if (!anchor) {
      setFloatingMenuPosition(null);
      return;
    }
    const rect = anchor.getBoundingClientRect();
    const topBelow = rect.bottom + 6;
    const topAbove = rect.top - menuHeightEstimate - 6;
    const openUp = topBelow + menuHeightEstimate > window.innerHeight - viewportMargin && topAbove >= viewportMargin;
    const top = openUp ? topAbove : Math.min(topBelow, window.innerHeight - menuHeightEstimate - viewportMargin);
    const left = Math.min(Math.max(rect.right - menuWidth, viewportMargin), window.innerWidth - menuWidth - viewportMargin);
    setFloatingMenuPosition({ top, left });
  }, [menuNodeID, viewMode]);

  useEffect(() => {
    updateFloatingMenuPosition();
  }, [updateFloatingMenuPosition]);

  useEffect(() => {
    if (!headerCheckboxRef.current) {
      return;
    }
    headerCheckboxRef.current.indeterminate = hasPartialSelection;
  }, [hasPartialSelection]);

  useEffect(() => {
    if (!menuNodeID || viewMode !== "list") {
      return;
    }
    const handleResize = () => updateFloatingMenuPosition();
    const handleScroll = () => updateFloatingMenuPosition();
    window.addEventListener("resize", handleResize);
    window.addEventListener("scroll", handleScroll, true);
    return () => {
      window.removeEventListener("resize", handleResize);
      window.removeEventListener("scroll", handleScroll, true);
    };
  }, [menuNodeID, updateFloatingMenuPosition, viewMode]);

  useEffect(() => {
    if (!menuNodeID || viewMode !== "list") {
      return;
    }
    const handlePointerDown = (event: MouseEvent) => {
      const target = event.target as Node | null;
      if (!target) {
        return;
      }
      const anchor = menuButtonRefs.current[menuNodeID];
      if (anchor?.contains(target)) {
        return;
      }
      if (floatingMenuRef.current?.contains(target)) {
        return;
      }
      closeMenu();
    };
    window.addEventListener("pointerdown", handlePointerDown);
    return () => window.removeEventListener("pointerdown", handlePointerDown);
  }, [closeMenu, menuNodeID, viewMode]);

  if (error) {
    return (
      <div className="surface-card rounded-2xl p-6">
        <p className="text-sm text-red-500">{t("file.loadError", "Failed to load list")}: {error}</p>
        <button
          type="button"
          onClick={onRetry}
          className="focus-ring mt-3 rounded-lg bg-[var(--brand)] px-3 py-2 text-sm font-semibold text-white"
        >
          {t("common.retry", "Retry")}
        </button>
      </div>
    );
  }

  if (loading) {
    return (
      <div className="surface-card rounded-2xl p-6" data-testid="file-table-loading">
        <p className="text-sm text-[var(--text-muted)]">{t("file.loading", "Loading files...")}</p>
      </div>
    );
  }

  if (nodes.length === 0) {
    return (
      <div className="surface-card rounded-2xl p-10 text-center" data-testid="file-table-empty">
        <i className="fa-regular fa-folder-open text-3xl text-[var(--text-muted)]" />
        <p className="mt-3 text-sm text-[var(--text-main)]">{emptyState?.title ?? t("file.empty.title", "This folder is empty.")}</p>
        <p className="mt-1 text-xs text-[var(--text-muted)]">{emptyState?.description ?? t("file.empty.desc", "You can create a folder or upload a file.")}</p>
        {emptyState?.actionLabel && emptyState.onAction ? (
          <button
            type="button"
            onClick={emptyState.onAction}
            className="focus-ring mt-4 rounded-lg border border-[var(--line)] px-3 py-2 text-sm hover:bg-[var(--bg-soft)]"
          >
            {emptyState.actionLabel}
          </button>
        ) : null}
      </div>
    );
  }

  if (viewMode === "grid") {
    return (
      <div className="space-y-3">
        <div className="flex flex-wrap items-center gap-2">
          <label className="inline-flex items-center gap-2 rounded-lg border border-[var(--line)] px-3 py-1.5 text-sm text-[var(--text-main)]">
            <input
              type="checkbox"
              checked={allSelected}
              onChange={onToggleSelectAll}
              className="h-4 w-4 rounded border-[var(--line)]"
            />
            {t("file.selectAll", "Select all")}
          </label>
          <button
            type="button"
            onClick={() => onChangeSort(sortKey)}
            className="focus-ring rounded-lg border border-[var(--line)] px-3 py-1.5 text-sm text-[var(--text-main)] hover:bg-[var(--bg-soft)]"
          >
            {t("file.sortBy", "Sort")}: {sortLabel(sortKey)} ({sortDirection})
          </button>
        </div>
        <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
          {nodes.map((node) => {
            const typeMeta = resolveNodeTypeMeta(node);
            const selected = selectedNodeIDs.includes(node.id);
            const isShared = !!sharedNodeIDs[node.id];
            return (
              <article
                key={node.id}
                className={`surface-card table-row-hover rounded-xl p-4 ${selectedNodeIDForDetails === node.id ? "ring-2 ring-[var(--brand)] bg-[var(--brand-soft)]/50" : ""}`}
              >
                <div className="flex items-start justify-between gap-3">
                  <button
                    type="button"
                    onClick={() => onToggleSelectNode(node.id)}
                    onClickCapture={(event) => event.stopPropagation()}
                    className="mt-1 h-4 w-4 rounded border border-[var(--line)] text-[var(--brand)]"
                    aria-label={t("common.select", "Select")}
                  >
                    {selected ? <i className="fa-solid fa-check text-[10px]" /> : null}
                  </button>
                  <button
                    type="button"
                    onClick={() => {
                      onSelectForDetails(node.id);
                      if (node.type === "file" && isTextEditableNode(node)) {
                        onOpenFile(node);
                      }
                    }}
                    className="focus-ring flex min-w-0 flex-1 items-start gap-2 text-left"
                  >
                    <span className={`inline-flex h-9 w-9 shrink-0 items-center justify-center rounded-lg ${typeMeta.wrapClass}`}>
                      <i className={typeMeta.iconClass} />
                    </span>
                    <span className="min-w-0">
                      <span className="flex items-center gap-1">
                        <span className="block truncate text-sm font-semibold text-[var(--text-main)]">{node.name}</span>
                        {isShared ? (
                          <i className="fa-solid fa-share-nodes shrink-0 text-[11px] text-[var(--brand)]" title={t("share.shared", "Shared")} aria-label={t("share.shared", "Shared")} />
                        ) : null}
                      </span>
                      <span className="block text-xs text-[var(--text-muted)]">{node.type === "folder" ? t("file.folder", "Folder") : typeMeta.label}</span>
                    </span>
                  </button>
                  <div className="relative">
                    <button
                      type="button"
                      onClick={() => onSetMenuNodeID(menuNodeID === node.id ? null : node.id)}
                      data-testid="row-action-menu-toggle"
                      className="focus-ring rounded-lg px-2 py-1 text-[var(--text-muted)] hover:bg-[var(--bg-soft)]"
                    >
                      <i className="fa-solid fa-ellipsis-vertical" />
                    </button>
                    {menuNodeID === node.id ? (
                      <div className="absolute right-0 top-full z-30 mt-1 w-40 rounded-lg border border-[var(--line)] bg-[var(--bg-card)] p-1 text-left shadow-xl">
                        {node.type === "folder" ? (
                          <>
                            <button
                              type="button"
                              onClick={() => onOpenFolder(node)}
                              data-testid="row-action-open"
                              className="block w-full rounded px-2 py-1.5 text-left text-sm text-[var(--text-main)] hover:bg-[var(--bg-soft)]"
                            >
                              {t("common.open", "Open")}
                            </button>
                            <button
                              type="button"
                              onClick={() => onDownloadNode(node)}
                              data-testid="row-action-download"
                              className="block w-full rounded px-2 py-1.5 text-left text-sm text-[var(--text-main)] hover:bg-[var(--bg-soft)]"
                            >
                              {t("common.download", "Download")}
                            </button>
                          </>
                        ) : (
                          <>
                            <button
                              type="button"
                              onClick={() => (isTextEditableNode(node) ? onOpenFile(node) : onDownloadNode(node))}
                              data-testid="row-action-open"
                              className="block w-full rounded px-2 py-1.5 text-left text-sm text-[var(--text-main)] hover:bg-[var(--bg-soft)]"
                            >
                              {t("common.open", "Open")}
                            </button>
                            <button
                              type="button"
                              onClick={() => onDownloadNode(node)}
                              data-testid="row-action-download"
                              className="block w-full rounded px-2 py-1.5 text-left text-sm text-[var(--text-main)] hover:bg-[var(--bg-soft)]"
                            >
                              {t("common.download", "Download")}
                            </button>
                          </>
                        )}
                        <button
                          type="button"
                          onClick={() => onRenameNode(node)}
                          data-testid="row-action-rename"
                          className="block w-full rounded px-2 py-1.5 text-left text-sm text-[var(--text-main)] hover:bg-[var(--bg-soft)]"
                        >
                          {t("common.rename", "Rename")}
                        </button>
                        <button
                          type="button"
                          onClick={() => onDeleteNode(node)}
                          data-testid="row-action-delete"
                          className="block w-full rounded px-2 py-1.5 text-left text-sm text-red-500 hover:bg-red-500/10"
                        >
                          {t("common.delete", "Delete")}
                        </button>
                      </div>
                    ) : null}
                  </div>
                </div>
                <p className="mt-3 text-xs text-[var(--text-muted)]">{formatDateTime(node.updated_at)}</p>
              </article>
            );
          })}
        </div>
      </div>
    );
  }

  return (
    <>
      <div className="surface-card custom-scrollbar overflow-x-auto overflow-y-hidden rounded-2xl">
        <table className="w-full table-fixed" data-testid="file-table">
        <thead>
          <tr className="border-b border-[var(--line)] bg-[var(--bg-soft)]">
            <th className="w-14 px-4 py-3 text-left">
              <input
                ref={headerCheckboxRef}
                type="checkbox"
                checked={allSelected}
                onChange={onToggleSelectAll}
                className="h-4 w-4 rounded border-[var(--line)]"
                aria-label={t("file.selectAll", "Select all")}
              />
            </th>
            <th className="px-4 py-3 text-left text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)]">
              <button type="button" onClick={() => onChangeSort("name")} className="hover:text-[var(--text-main)]">
                {t("file.columns.name", "Name")}
              </button>
            </th>
            <th className="w-40 px-4 py-3 text-left text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)]">
              <button type="button" onClick={() => onChangeSort("type")} className="hover:text-[var(--text-main)]">
                {t("file.columns.type", "Type")}
              </button>
            </th>
            <th className="w-32 px-4 py-3 text-left text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)]">
              <button type="button" onClick={() => onChangeSort("size")} className="hover:text-[var(--text-main)]">
                {t("file.columns.size", "Size")}
              </button>
            </th>
            <th className="w-44 px-4 py-3 text-left text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)]">
              <button type="button" onClick={() => onChangeSort("updated")} className="hover:text-[var(--text-main)]">
                {t("file.columns.updated", "Updated")}
              </button>
            </th>
            <th className="w-24 px-4 py-3 text-right text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)]">{t("file.columns.action", "Action")}</th>
          </tr>
        </thead>
        <tbody>
          {nodes.map((node) => {
            const typeMeta = resolveNodeTypeMeta(node);
            const selected = selectedNodeIDs.includes(node.id);
            const isShared = !!sharedNodeIDs[node.id];
            return (
              <tr
                key={node.id}
                data-testid="file-row"
                data-node-id={node.id}
                data-node-name={node.name}
                data-node-type={node.type}
                className={`table-row-hover border-b border-[var(--line)] ${
                  selectedNodeIDForDetails === node.id ? "bg-[var(--bg-soft)] shadow-[inset_3px_0_0_var(--brand)]" : ""
                }`}
                onClick={(event) => {
                  if (isRowInteractionTarget(event.target)) {
                    return;
                  }
                  onSelectForDetails(node.id);
                }}
                onDoubleClick={(event) => {
                  if (isRowInteractionTarget(event.target)) {
                    return;
                  }
                  onSelectForDetails(node.id);
                  if (node.type === "folder") {
                    onOpenFolder(node);
                    return;
                  }
                  if (isTextEditableNode(node)) {
                    onOpenFile(node);
                    return;
                  }
                  onDownloadNode(node);
                }}
              >
                <td className="px-4 py-3">
                  <input
                    type="checkbox"
                    checked={selected}
                    onClick={(event) => event.stopPropagation()}
                    onChange={() => onToggleSelectNode(node.id)}
                    data-no-row-open="1"
                    className="h-4 w-4 rounded border-[var(--line)]"
                  />
                </td>
                <td className="px-4 py-3">
                  <button
                    type="button"
                    onClick={() => {
                      onSelectForDetails(node.id);
                    }}
                    onDoubleClick={(event) => {
                      event.stopPropagation();
                      onSelectForDetails(node.id);
                      if (node.type === "folder") {
                        onOpenFolder(node);
                        return;
                      }
                      if (isTextEditableNode(node)) {
                        onOpenFile(node);
                        return;
                      }
                      onDownloadNode(node);
                    }}
                    className="focus-ring flex w-full min-w-0 items-center gap-3 text-left"
                  >
                    <span className={`inline-flex h-9 w-9 shrink-0 items-center justify-center rounded-lg ${typeMeta.wrapClass}`}>
                      <i className={typeMeta.iconClass} />
                    </span>
                    <span className="flex min-w-0 items-center gap-1">
                      <span className="truncate text-sm font-semibold text-[var(--text-main)]">{node.name}</span>
                      {isShared ? (
                        <i className="fa-solid fa-share-nodes shrink-0 text-[11px] text-[var(--brand)]" title={t("share.shared", "Shared")} aria-label={t("share.shared", "Shared")} />
                      ) : null}
                    </span>
                  </button>
                </td>
                <td className="px-4 py-3 text-sm text-[var(--text-muted)]">
                  {node.type === "folder" ? (
                    <span className="status-badge status-badge-folder">{t("file.folder", "Folder")}</span>
                  ) : (
                    typeMeta.label
                  )}
                </td>
                <td className="px-4 py-3 text-sm text-[var(--text-muted)]">{node.type === "folder" ? "--" : formatBytes(node.size_bytes)}</td>
                <td className="px-4 py-3 text-sm text-[var(--text-muted)]">{formatDateTime(node.updated_at)}</td>
                <td className="px-4 py-3 text-right align-top">
                  <div className="relative inline-flex justify-end">
                    <button
                      ref={(element) => {
                        menuButtonRefs.current[node.id] = element;
                      }}
                      type="button"
                      aria-label={`${t("file.actionMenu", "Action menu")} ${node.name}`}
                      data-testid="row-action-menu-toggle"
                      data-no-row-open="1"
                      onClick={() => onSetMenuNodeID(menuNodeID === node.id ? null : node.id)}
                      className="focus-ring rounded-lg p-2 text-[var(--text-muted)] hover:bg-[var(--bg-soft)]"
                    >
                      <i className="fa-solid fa-ellipsis-vertical" />
                    </button>
                  </div>
                </td>
              </tr>
            );
          })}
        </tbody>
        </table>
      </div>
      {viewMode === "list" && activeMenuNode && floatingMenuPosition && typeof document !== "undefined"
        ? createPortal(
            <div
              ref={floatingMenuRef}
              className="z-40 w-40 rounded-lg border border-[var(--line)] bg-[var(--bg-card)] p-1 text-left shadow-2xl"
              style={{
                position: "fixed",
                top: `${floatingMenuPosition.top}px`,
                left: `${floatingMenuPosition.left}px`
              }}
            >
              {activeMenuNode.type === "folder" ? (
                <>
                  <button
                    type="button"
                    onClick={() => {
                      closeMenu();
                      onOpenFolder(activeMenuNode);
                    }}
                    data-testid="row-action-open"
                    className="block w-full rounded px-2 py-1.5 text-left text-sm text-[var(--text-main)] hover:bg-[var(--bg-soft)]"
                  >
                    {t("common.open", "Open")}
                  </button>
                  <button
                    type="button"
                    onClick={() => {
                      closeMenu();
                      onDownloadNode(activeMenuNode);
                    }}
                    data-testid="row-action-download"
                    className="block w-full rounded px-2 py-1.5 text-left text-sm text-[var(--text-main)] hover:bg-[var(--bg-soft)]"
                  >
                    {t("common.download", "Download")}
                  </button>
                </>
              ) : (
                <>
                  <button
                    type="button"
                    onClick={() => {
                      closeMenu();
                      if (isTextEditableNode(activeMenuNode)) {
                        onOpenFile(activeMenuNode);
                      } else {
                        onDownloadNode(activeMenuNode);
                      }
                    }}
                    data-testid="row-action-open"
                    className="block w-full rounded px-2 py-1.5 text-left text-sm text-[var(--text-main)] hover:bg-[var(--bg-soft)]"
                  >
                    {t("common.open", "Open")}
                  </button>
                  <button
                    type="button"
                    onClick={() => {
                      closeMenu();
                      onDownloadNode(activeMenuNode);
                    }}
                    data-testid="row-action-download"
                    className="block w-full rounded px-2 py-1.5 text-left text-sm text-[var(--text-main)] hover:bg-[var(--bg-soft)]"
                  >
                    {t("common.download", "Download")}
                  </button>
                </>
              )}
              <button
                type="button"
                onClick={() => {
                  closeMenu();
                  onRenameNode(activeMenuNode);
                }}
                data-testid="row-action-rename"
                className="block w-full rounded px-2 py-1.5 text-left text-sm text-[var(--text-main)] hover:bg-[var(--bg-soft)]"
              >
                {t("common.rename", "Rename")}
              </button>
              <button
                type="button"
                onClick={() => {
                  closeMenu();
                  onDeleteNode(activeMenuNode);
                }}
                data-testid="row-action-delete"
                className="block w-full rounded px-2 py-1.5 text-left text-sm text-red-500 hover:bg-red-500/10"
              >
                {t("common.delete", "Delete")}
              </button>
            </div>,
            document.body
          )
        : null}
    </>
  );
}
