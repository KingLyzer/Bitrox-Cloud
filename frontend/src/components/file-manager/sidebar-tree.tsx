"use client";

import { Fragment } from "react";
import type { NodeRecord } from "@/lib/api";
import { cacheKeyForParent } from "@/components/file-manager/helpers";
import { resolveNodeTypeMeta } from "@/components/file-manager/file-type";
import { useAppContext } from "@/components/app-context-provider";

type Props = {
  rootNodes: NodeRecord[];
  childrenByParent: Record<string, NodeRecord[]>;
  expandedFolderIDs: Record<string, boolean>;
  loadingFolderIDs: Record<string, boolean>;
  activeFolderID: string | null;
  activeNodeID: string | null;
  onToggleFolder: (node: NodeRecord) => void;
  onOpenFolder: (node: NodeRecord) => void;
  onOpenFile: (node: NodeRecord) => void;
};

type TreeNodeProps = {
  node: NodeRecord;
  depth: number;
  childrenByParent: Record<string, NodeRecord[]>;
  expandedFolderIDs: Record<string, boolean>;
  loadingFolderIDs: Record<string, boolean>;
  activeFolderID: string | null;
  activeNodeID: string | null;
  onToggleFolder: (node: NodeRecord) => void;
  onOpenFolder: (node: NodeRecord) => void;
  onOpenFile: (node: NodeRecord) => void;
};

function TreeNode({
  node,
  depth,
  childrenByParent,
  expandedFolderIDs,
  loadingFolderIDs,
  activeFolderID,
  activeNodeID,
  onToggleFolder,
  onOpenFolder,
  onOpenFile,
}: TreeNodeProps) {
  const { t } = useAppContext();
  const isFolder = node.type === "folder";
  const isExpanded = isFolder ? !!expandedFolderIDs[node.id] : false;
  const isLoading = isFolder ? !!loadingFolderIDs[node.id] : false;
  const childrenKey = cacheKeyForParent(node.id);
  const hasLoadedChildren = Object.prototype.hasOwnProperty.call(childrenByParent, childrenKey);
  const children = hasLoadedChildren ? childrenByParent[childrenKey] ?? [] : [];
  const isActive = node.id === activeFolderID || node.id === activeNodeID;
  const typeMeta = resolveNodeTypeMeta(node);
  const rowPadding = 8 + depth * 14;

  return (
    <Fragment>
      <div
        className={`group flex items-center gap-1 rounded-lg pr-2 text-sm ${
          isActive ? "bg-[var(--brand-soft)] text-[var(--brand)]" : "hover:bg-[var(--bg-soft)]"
        }`}
        style={{ paddingLeft: `${rowPadding}px` }}
        data-testid="sidebar-tree-node"
        data-node-id={node.id}
        data-node-type={node.type}
        data-node-depth={depth}
      >
        {isFolder ? (
          <button
            type="button"
            onClick={() => onToggleFolder(node)}
            data-testid="sidebar-tree-toggle"
            aria-label={isExpanded ? t("sidebar.collapse", "Collapse") : t("sidebar.expand", "Expand")}
            className="focus-ring grid h-6 w-6 place-items-center rounded text-[10px] text-[var(--text-muted)] hover:bg-[var(--bg-card)]"
          >
            {isLoading ? <i className="fa-solid fa-spinner fa-spin" /> : <i className={`fa-solid ${isExpanded ? "fa-chevron-down" : "fa-chevron-right"}`} />}
          </button>
        ) : (
          <span className="block h-6 w-6" aria-hidden="true" />
        )}
        <button
          type="button"
          onClick={() => (isFolder ? onOpenFolder(node) : onOpenFile(node))}
          onDoubleClick={(event) => {
            event.preventDefault();
            event.stopPropagation();
            if (isFolder) {
              onOpenFolder(node);
            } else {
              onOpenFile(node);
            }
          }}
          className="focus-ring flex min-w-0 flex-1 items-center gap-2 rounded px-1 py-1.5 text-left"
        >
          <span className={`inline-flex h-6 w-6 shrink-0 items-center justify-center rounded ${typeMeta.wrapClass}`}>
            <i className={typeMeta.iconClass} />
          </span>
          <span className="truncate">{node.name}</span>
        </button>
      </div>
      {isFolder && isExpanded ? (
        isLoading ? (
          <p className="px-3 py-1 text-xs text-[var(--text-muted)]" style={{ paddingLeft: `${rowPadding + 28}px` }}>
            {t("sidebar.loading", "Loading...")}
          </p>
        ) : children.length === 0 ? (
          <p className="px-3 py-1 text-xs text-[var(--text-muted)]" style={{ paddingLeft: `${rowPadding + 28}px` }}>
            {t("sidebar.emptyChildren", "No subfolders or files")}
          </p>
        ) : (
          children.map((child) => (
            <TreeNode
              key={child.id}
              node={child}
              depth={depth + 1}
              childrenByParent={childrenByParent}
              expandedFolderIDs={expandedFolderIDs}
              loadingFolderIDs={loadingFolderIDs}
              activeFolderID={activeFolderID}
              activeNodeID={activeNodeID}
              onToggleFolder={onToggleFolder}
              onOpenFolder={onOpenFolder}
              onOpenFile={onOpenFile}
            />
          ))
        )
      ) : null}
    </Fragment>
  );
}

export function SidebarTree({
  rootNodes,
  childrenByParent,
  expandedFolderIDs,
  loadingFolderIDs,
  activeFolderID,
  activeNodeID,
  onToggleFolder,
  onOpenFolder,
  onOpenFile,
}: Props) {
  const { t } = useAppContext();
  if (rootNodes.length === 0) {
    return <p className="text-xs text-[var(--text-muted)]">{t("sidebar.noFolders", "No folders found.")}</p>;
  }

  return (
    <div className="space-y-1" data-testid="sidebar-tree">
      {rootNodes.map((node) => (
        <TreeNode
          key={node.id}
          node={node}
          depth={0}
          childrenByParent={childrenByParent}
          expandedFolderIDs={expandedFolderIDs}
          loadingFolderIDs={loadingFolderIDs}
          activeFolderID={activeFolderID}
          activeNodeID={activeNodeID}
          onToggleFolder={onToggleFolder}
          onOpenFolder={onOpenFolder}
          onOpenFile={onOpenFile}
        />
      ))}
    </div>
  );
}
