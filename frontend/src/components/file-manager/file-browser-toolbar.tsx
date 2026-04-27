"use client";

import type { ChangeEvent } from "react";
import { useEffect, useRef, useState } from "react";
import { useAppContext } from "@/components/app-context-provider";

type FolderCrumb = {
  id: string | null;
  name: string;
};

type Props = {
  breadcrumbs: FolderCrumb[];
  searchTerm: string;
  typeFilter: string;
  typeFilterOptions: Array<{ value: string; label: string }>;
  selectedCount: number;
  viewMode: "list" | "grid";
  actionBusy: boolean;
  nodesLoading: boolean;
  onGoToFolder: (folderID: string | null, folderName: string) => void;
  onSearchTermChange: (value: string) => void;
  onTypeFilterChange: (value: string) => void;
  onSetViewMode: (mode: "list" | "grid") => void;
  onUploadFiles: (files: File[]) => void;
  onOpenCreateFolder: () => void;
  onOpenCreateTextFile: () => void;
  onOpenMoveSelected: () => void;
  onOpenDeleteSelected: () => void;
  onRefresh: () => void;
};

export function FileBrowserToolbar({
  breadcrumbs,
  searchTerm,
  typeFilter,
  typeFilterOptions,
  selectedCount,
  viewMode,
  actionBusy,
  nodesLoading,
  onGoToFolder,
  onSearchTermChange,
  onTypeFilterChange,
  onSetViewMode,
  onUploadFiles,
  onOpenCreateFolder,
  onOpenCreateTextFile,
  onOpenMoveSelected,
  onOpenDeleteSelected,
  onRefresh
}: Props) {
  const { t } = useAppContext();
  const [searchOpen, setSearchOpen] = useState(false);
  const [quickActionsOpen, setQuickActionsOpen] = useState(false);
  const searchPanelRef = useRef<HTMLDivElement | null>(null);
  const quickActionsRef = useRef<HTMLDivElement | null>(null);
  const fileInputRef = useRef<HTMLInputElement | null>(null);

  useEffect(() => {
    if (!searchOpen && !quickActionsOpen) {
      return;
    }
    const handlePointerDown = (event: MouseEvent) => {
      const target = event.target as Node | null;
      if (!target) {
        return;
      }
      if (searchPanelRef.current && !searchPanelRef.current.contains(target)) {
        setSearchOpen(false);
      }
      if (quickActionsRef.current && !quickActionsRef.current.contains(target)) {
        setQuickActionsOpen(false);
      }
    };
    window.addEventListener("pointerdown", handlePointerDown);
    return () => window.removeEventListener("pointerdown", handlePointerDown);
  }, [quickActionsOpen, searchOpen]);

  const onFilePickerChange = (event: ChangeEvent<HTMLInputElement>) => {
    const files = Array.from(event.target.files ?? []);
    event.target.value = "";
    setQuickActionsOpen(false);
    onUploadFiles(files);
  };

  return (
    <div className="surface-card rounded-2xl px-4 py-3" data-testid="file-browser-toolbar">
      <div className="mb-3 flex flex-wrap items-start justify-between gap-3">
        <nav className="flex flex-wrap items-center gap-2 text-sm">
          {breadcrumbs.map((crumb, index) => (
            <div key={crumb.id ?? "__root__"} className="flex items-center gap-2">
              <button
                type="button"
                onClick={() => onGoToFolder(crumb.id, crumb.name)}
                className={`focus-ring rounded px-1.5 py-0.5 ${
                  index === breadcrumbs.length - 1 ? "font-semibold text-[var(--text-main)]" : "text-[var(--text-muted)] hover:text-[var(--text-main)]"
                }`}
              >
                {crumb.name}
              </button>
              {index < breadcrumbs.length - 1 ? <i className="fa-solid fa-chevron-right text-[10px] text-[var(--text-muted)]" /> : null}
            </div>
          ))}
        </nav>

        <div className="flex flex-wrap items-center justify-end gap-2">
          <div ref={searchPanelRef} className="relative">
            <button
              type="button"
              onClick={() => setSearchOpen((prev) => !prev)}
              data-testid="search-toggle-button"
              className="focus-ring rounded-lg border border-[var(--line)] px-3 py-2 text-sm hover:bg-[var(--bg-soft)]"
            >
              <i className="fa-solid fa-magnifying-glass mr-1.5" />
              {t("file.search", "Search")}
            </button>
            {searchOpen ? (
              <div className="absolute right-0 top-full z-30 mt-2 w-72 rounded-xl border border-[var(--line)] bg-[var(--bg-card)] p-2 shadow-xl">
                <input
                  value={searchTerm}
                  onChange={(event) => onSearchTermChange(event.target.value)}
                  placeholder={t("file.searchPlaceholder", "Search by name")}
                  data-testid="search-input"
                  className="focus-ring w-full rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-3 py-2 text-sm outline-none"
                />
                <select
                  value={typeFilter}
                  onChange={(event) => onTypeFilterChange(event.target.value)}
                  data-testid="type-filter-select"
                  className="focus-ring mt-2 w-full rounded-lg border border-[var(--line)] bg-[var(--bg-soft)] px-3 py-2 text-sm outline-none"
                >
                  {typeFilterOptions.map((option) => (
                    <option key={option.value} value={option.value}>
                      {option.label}
                    </option>
                  ))}
                </select>
              </div>
            ) : null}
          </div>

          <div className="inline-flex overflow-hidden rounded-lg border border-[var(--line)]">
            <button
              type="button"
              onClick={() => onSetViewMode("list")}
              data-testid="view-list-button"
              className={`focus-ring px-3 py-2 text-sm ${viewMode === "list" ? "bg-[var(--brand-soft)] text-[var(--brand)]" : "hover:bg-[var(--bg-soft)]"}`}
              aria-label={t("file.view.list", "List view")}
            >
              <i className="fa-solid fa-table-list" />
            </button>
            <button
              type="button"
              onClick={() => onSetViewMode("grid")}
              data-testid="view-grid-button"
              className={`focus-ring border-l border-[var(--line)] px-3 py-2 text-sm ${viewMode === "grid" ? "bg-[var(--brand-soft)] text-[var(--brand)]" : "hover:bg-[var(--bg-soft)]"}`}
              aria-label={t("file.view.grid", "Grid view")}
            >
              <i className="fa-solid fa-grip" />
            </button>
          </div>

          <div ref={quickActionsRef} className="relative">
            <button
              type="button"
              onClick={() => setQuickActionsOpen((prev) => !prev)}
              data-testid="quick-actions-toggle"
              className="focus-ring grid h-10 w-10 place-items-center rounded-lg border border-[var(--line)] text-lg hover:bg-[var(--bg-soft)]"
              aria-label={t("file.quickActions", "Quick actions")}
            >
              <i className="fa-solid fa-plus" />
            </button>
            {quickActionsOpen ? (
              <div className="absolute right-0 top-full z-30 mt-2 w-44 rounded-xl border border-[var(--line)] bg-[var(--bg-card)] p-1.5 shadow-xl">
                <button
                  type="button"
                  onClick={() => fileInputRef.current?.click()}
                  data-testid="quick-action-upload"
                  className="focus-ring flex w-full items-center gap-2 rounded-lg px-2 py-2 text-left text-sm hover:bg-[var(--bg-soft)]"
                >
                  <i className="fa-solid fa-upload text-[var(--text-muted)]" />
                  {t("file.upload", "Upload")}
                </button>
                <button
                  type="button"
                  onClick={() => {
                    setQuickActionsOpen(false);
                    onOpenCreateFolder();
                  }}
                  data-testid="quick-action-create-folder"
                  className="focus-ring flex w-full items-center gap-2 rounded-lg px-2 py-2 text-left text-sm hover:bg-[var(--bg-soft)]"
                >
                  <i className="fa-regular fa-folder text-[var(--text-muted)]" />
                  {t("file.newFolder", "Folder")}
                </button>
                <button
                  type="button"
                  onClick={() => {
                    setQuickActionsOpen(false);
                    onOpenCreateTextFile();
                  }}
                  data-testid="quick-action-create-text"
                  className="focus-ring flex w-full items-center gap-2 rounded-lg px-2 py-2 text-left text-sm hover:bg-[var(--bg-soft)]"
                >
                  <i className="fa-regular fa-file-lines text-[var(--text-muted)]" />
                  {t("file.newTextFile", "Text file")}
                </button>
              </div>
            ) : null}
          </div>
        </div>
      </div>

      <div className="flex flex-wrap items-center gap-2">
        <button
          type="button"
          onClick={onOpenMoveSelected}
          data-testid="bulk-move-button"
          disabled={selectedCount === 0 || actionBusy}
          className="focus-ring rounded-lg border border-[var(--line)] px-3 py-2 text-sm hover:bg-[var(--bg-soft)] disabled:opacity-50"
        >
          {t("file.move", "Move")}
        </button>
        <button
          type="button"
          onClick={onOpenDeleteSelected}
          data-testid="bulk-delete-button"
          disabled={selectedCount === 0 || actionBusy}
          className="focus-ring rounded-lg border border-red-500/40 px-3 py-2 text-sm text-red-500 hover:bg-red-500/10 disabled:opacity-50"
        >
          {t("file.delete", "Delete")}
        </button>
        <button
          type="button"
          onClick={onRefresh}
          data-testid="refresh-button"
          disabled={nodesLoading}
          className="focus-ring rounded-lg border border-[var(--line)] px-3 py-2 text-sm hover:bg-[var(--bg-soft)] disabled:opacity-60"
        >
          <i className="fa-solid fa-rotate-right mr-1.5" />
          {t("common.refresh", "Refresh")}
        </button>
        {searchTerm.trim() || typeFilter !== "all" ? (
          <button
            type="button"
            onClick={() => {
              onSearchTermChange("");
              onTypeFilterChange("all");
            }}
            className="focus-ring rounded-lg border border-[var(--line)] px-3 py-2 text-xs text-[var(--text-muted)] hover:bg-[var(--bg-soft)]"
          >
            {t("fileTable.clearSearchFilter", "Clear Search/Filter")}
          </button>
        ) : null}
      </div>

      <input ref={fileInputRef} type="file" multiple hidden onChange={onFilePickerChange} />
    </div>
  );
}
