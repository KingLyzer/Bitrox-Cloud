"use client";

import type { DragEvent, FormEvent } from "react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import {
  ApiError,
  createFolder,
  deleteNode,
  downloadUrlForNode,
  fetchMe,
  listNotifications,
  listShares,
  fetchQuota,
  listNodes,
  login,
  logout,
  updateMe,
  markNotificationRead,
  moveNode,
  type AuthUser,
  type NotificationRecord,
  type NodeRecord,
  type ShareRecord,
  refreshSession,
  renameNode,
  resolveDisplayName,
} from "@/lib/api";
import { readCSRFTokenFromCookie } from "@/lib/browser";
import { AuthView } from "@/components/file-manager/auth-view";
import { DetailsPanel } from "@/components/file-manager/details-panel";
import { FileActionModals } from "@/components/file-manager/file-action-modals";
import { FileBrowserToolbar } from "@/components/file-manager/file-browser-toolbar";
import { FileTable } from "@/components/file-manager/file-table";
import { SidebarTree } from "@/components/file-manager/sidebar-tree";
import type { ProfilePrefs } from "@/components/file-manager/profile-settings-panel";
import { isTextEditableNode, matchesNodeTypeFilter, nodeTypeFilterOptions, type NodeTypeFilter, resolveNodeTypeLabel } from "@/components/file-manager/file-type";
import { ProfileSettingsPanel } from "@/components/file-manager/profile-settings-panel";
import { useUploadManager } from "@/components/file-manager/use-upload-manager";
import { WorkspaceHeader } from "@/components/file-manager/workspace-header";
import { CalendarPanel } from "@/components/calendar/calendar-panel";
import { SharesPage } from "@/components/file-manager/shares-page";
import {
  cacheKeyForParent,
  formatBytes,
  normalizeErrorMessage,
  type SortDirection,
  type SortKey,
} from "@/components/file-manager/helpers";
import { useAppContext } from "@/components/app-context-provider";
import { ThemeToggle } from "@/components/theme-toggle";
import { useToast } from "@/components/toast-provider";

const defaultChunkSizeBytes = 8 * 1024 * 1024;
const profilePrefsStorageKey = "bitroxcloud_profile_prefs_v1";
const pendingSidebarFolderStorageKey = "bitroxcloud_pending_sidebar_folder_v1";
const avatarColorOptions = ["#4f7cff", "#14b8a6", "#f59e0b", "#ef4444", "#8b5cf6", "#22c55e"];

const defaultProfilePrefs: ProfilePrefs = {
  avatarColor: "#4f7cff"
};

type FolderCrumb = {
  id: string | null;
  name: string;
};

function parseSortValue(node: NodeRecord, key: SortKey): string | number {
  if (key === "name") {
    return node.name.toLowerCase();
  }
  if (key === "type") {
    return resolveNodeTypeLabel(node).toLowerCase();
  }
  if (key === "size") {
    return node.type === "folder" ? -1 : node.size_bytes;
  }
  return new Date(node.updated_at).getTime();
}

function sortNodes(nodes: NodeRecord[], sortKey: SortKey, sortDirection: SortDirection): NodeRecord[] {
  const sorted = [...nodes].sort((a, b) => {
    if (a.type !== b.type) {
      return a.type === "folder" ? -1 : 1;
    }
    const av = parseSortValue(a, sortKey);
    const bv = parseSortValue(b, sortKey);
    if (av < bv) {
      return sortDirection === "asc" ? -1 : 1;
    }
    if (av > bv) {
      return sortDirection === "asc" ? 1 : -1;
    }
    return 0;
  });
  return sorted;
}

function sortNodesForTree(nodes: NodeRecord[]): NodeRecord[] {
  return [...nodes].sort((a, b) => {
    if (a.type !== b.type) {
      return a.type === "folder" ? -1 : 1;
    }
    return a.name.localeCompare(b.name);
  });
}

function isShareActive(share: ShareRecord): boolean {
  if (share.revoked_at) {
    return false;
  }
  if (!share.expires_at) {
    return true;
  }
  return new Date(share.expires_at).getTime() > Date.now();
}

function userInitials(displayName: string | null | undefined, email: string | null | undefined): string {
  const trimmed = (displayName ?? "").trim();
  if (trimmed.length > 0) {
    return trimmed
      .split(/\s+/)
      .slice(0, 2)
      .map((part) => part[0]?.toUpperCase() ?? "")
      .join("");
  }
  if (!email) {
    return "??";
  }
  return email.slice(0, 2).toUpperCase();
}

function dragHasFiles(event: DragEvent<HTMLElement>): boolean {
  return Array.from(event.dataTransfer.types ?? []).includes("Files");
}

function extractFileNameFromDisposition(disposition: string | null): string | null {
  if (!disposition) {
    return null;
  }
  const utf8Match = disposition.match(/filename\*=UTF-8''([^;]+)/i);
  if (utf8Match?.[1]) {
    try {
      return decodeURIComponent(utf8Match[1].trim());
    } catch {
      return utf8Match[1].trim();
    }
  }
  const basicMatch = disposition.match(/filename="?([^";]+)"?/i);
  if (basicMatch?.[1]) {
    return basicMatch[1].trim();
  }
  return null;
}

type DashboardPage = "files" | "profile" | "calendar" | "shares";

type DashboardShellProps = {
  page?: DashboardPage;
};

export function DashboardShell({ page = "files" }: DashboardShellProps) {
  const router = useRouter();
  const { settings, setLocale, t } = useAppContext();
  const { showToast } = useToast();
  const [authLoading, setAuthLoading] = useState(true);
  const [loginBusy, setLoginBusy] = useState(false);
  const [actionBusy, setActionBusy] = useState(false);

  const [email, setEmail] = useState("admin@example.com");
  const [password, setPassword] = useState("");

  const [user, setUser] = useState<AuthUser | null>(null);
  const [quota, setQuota] = useState<{
    usedBytes: number;
    limitBytes: number;
    remainingBytes: number;
  } | null>(null);

  const [nodes, setNodes] = useState<NodeRecord[]>([]);
  const [nodesLoading, setNodesLoading] = useState(false);
  const [nodesError, setNodesError] = useState<string | null>(null);
  const [nodeIndex, setNodeIndex] = useState<Record<string, NodeRecord>>({});
  const [folderChildrenByParent, setFolderChildrenByParent] = useState<Record<string, NodeRecord[]>>({});
  const [treeChildrenByParent, setTreeChildrenByParent] = useState<Record<string, NodeRecord[]>>({});
  const [expandedTreeFolderIDs, setExpandedTreeFolderIDs] = useState<Record<string, boolean>>({});
  const [treeLoadingFolderIDs, setTreeLoadingFolderIDs] = useState<Record<string, boolean>>({});
  const [activeSharedNodeIDs, setActiveSharedNodeIDs] = useState<Record<string, boolean>>({});

  const rootLabel = t("sidebar.allFiles", "All Files");
  const [currentParentID, setCurrentParentID] = useState<string | null>(null);
  const [breadcrumbs, setBreadcrumbs] = useState<FolderCrumb[]>([{ id: null, name: rootLabel }]);
  const [searchTerm, setSearchTerm] = useState("");
  const [typeFilter, setTypeFilter] = useState<NodeTypeFilter>("all");
  const [sortKey, setSortKey] = useState<SortKey>("name");
  const [sortDirection, setSortDirection] = useState<SortDirection>("asc");
  const [viewMode, setViewMode] = useState<"list" | "grid">("list");

  const [selectedNodeIDs, setSelectedNodeIDs] = useState<string[]>([]);
  const [selectedNodeIDForDetails, setSelectedNodeIDForDetails] = useState<string | null>(null);
  const [menuNodeID, setMenuNodeID] = useState<string | null>(null);

  const [dragActive, setDragActive] = useState(false);
  const [userMenuOpen, setUserMenuOpen] = useState(false);
  const [logoLoadFailed, setLogoLoadFailed] = useState(false);
  const [profilePrefs, setProfilePrefs] = useState<ProfilePrefs>(defaultProfilePrefs);
  const [profileDisplayName, setProfileDisplayName] = useState("");
  const [profileLanguage, setProfileLanguage] = useState<"en" | "tr">("en");
  const [profileSaving, setProfileSaving] = useState(false);
  const [notifications, setNotifications] = useState<NotificationRecord[]>([]);

  const [showCreateModal, setShowCreateModal] = useState(false);
  const [createFolderName, setCreateFolderName] = useState("");
  const [showCreateTextModal, setShowCreateTextModal] = useState(false);
  const [createTextFileName, setCreateTextFileName] = useState("");
  const [textEditorTarget, setTextEditorTarget] = useState<NodeRecord | null>(null);
  const [textEditorContent, setTextEditorContent] = useState("");
  const [textEditorLoading, setTextEditorLoading] = useState(false);
  const [textEditorSaving, setTextEditorSaving] = useState(false);
  const [renameTarget, setRenameTarget] = useState<NodeRecord | null>(null);
  const [renameName, setRenameName] = useState("");
  const [deleteTargets, setDeleteTargets] = useState<NodeRecord[]>([]);
  const [showMoveModal, setShowMoveModal] = useState(false);
  const [moveTargets, setMoveTargets] = useState<NodeRecord[]>([]);
  const [moveDestinationID, setMoveDestinationID] = useState<string | null>(null);
  const initializedWorkspaceUserRef = useRef<string | null>(null);
  const loadNodesRequestSeqRef = useRef(0);
  const refreshPromiseRef = useRef<Promise<AuthUser> | null>(null);
  const authBrokenRef = useRef(false);
  const dragDepthRef = useRef(0);

  const currentParentRef = useRef<string | null>(null);
  useEffect(() => {
    currentParentRef.current = currentParentID;
  }, [currentParentID]);

  useEffect(() => {
    setBreadcrumbs((prev) => {
      if (prev.length === 0) {
        return [{ id: null, name: rootLabel }];
      }
      const [first, ...rest] = prev;
      return [{ ...first, name: rootLabel }, ...rest];
    });
  }, [rootLabel]);

  useEffect(() => {
    setLogoLoadFailed(false);
  }, [settings.logo_url]);

  const showFlash = useCallback(
    (tone: "success" | "error" | "info", message: string) => {
      showToast(tone === "info" ? "info" : tone, message);
    },
    [showToast]
  );

  useEffect(() => {
    if (!userMenuOpen) {
      return;
    }
    const handlePointerDown = (event: MouseEvent) => {
      const target = event.target as HTMLElement | null;
      if (!target) {
        return;
      }
      if (target.closest("[data-user-menu-anchor]")) {
        return;
      }
      setUserMenuOpen(false);
    };
    window.addEventListener("pointerdown", handlePointerDown);
    return () => window.removeEventListener("pointerdown", handlePointerDown);
  }, [userMenuOpen]);

  useEffect(() => {
    try {
      const raw = window.localStorage.getItem(profilePrefsStorageKey);
      if (!raw) {
        return;
      }
      const parsed = JSON.parse(raw) as Partial<ProfilePrefs>;
      setProfilePrefs({
        avatarColor: parsed.avatarColor ?? defaultProfilePrefs.avatarColor
      });
    } catch {
      // ignore invalid local profile prefs
    }
  }, []);

  const saveProfilePrefs = useCallback((next: ProfilePrefs) => {
    setProfilePrefs(next);
    try {
      window.localStorage.setItem(profilePrefsStorageKey, JSON.stringify(next));
    } catch {
      // ignore storage write errors
    }
  }, []);

  const refreshUserSession = useCallback(async () => {
    if (authBrokenRef.current) {
      throw new ApiError(401, "unauthorized", t("auth.sessionInvalid", "Session is invalid. Please sign in again."));
    }

    if (refreshPromiseRef.current) {
      return refreshPromiseRef.current;
    }

    const csrf = readCSRFTokenFromCookie();
    if (!csrf) {
      authBrokenRef.current = true;
      setUser(null);
      throw new ApiError(401, "missing_csrf", t("auth.sessionExpired", "Session expired. Please sign in again."));
    }

    const refreshPromise = (async () => {
      await refreshSession(csrf);
      const me = await fetchMe();
      setUser(me);
      return me;
    })();

    refreshPromiseRef.current = refreshPromise;
    try {
      const me = await refreshPromise;
      return me;
    } catch (error) {
      authBrokenRef.current = true;
      setUser(null);
      throw error;
    } finally {
      refreshPromiseRef.current = null;
    }
  }, [t]);

  const runWithRefresh = useCallback(
    async <T,>(operation: () => Promise<T>): Promise<T> => {
      try {
        return await operation();
      } catch (error) {
        if (error instanceof ApiError && error.status === 401) {
          await refreshUserSession();
          try {
            return await operation();
          } catch (retryError) {
            if (retryError instanceof ApiError && retryError.status === 401) {
              authBrokenRef.current = true;
              setUser(null);
            }
            throw retryError;
          }
        }
        throw error;
      }
    },
    [refreshUserSession]
  );

  const syncNodeCache = useCallback((listed: NodeRecord[], parentID: string | null) => {
    setNodeIndex((prev) => {
      const next = { ...prev };
      for (const node of listed) {
        next[node.id] = node;
      }
      return next;
    });

    const folders = listed.filter((node) => node.type === "folder").sort((a, b) => a.name.localeCompare(b.name));
    const key = cacheKeyForParent(parentID);
    setFolderChildrenByParent((prev) => ({ ...prev, [key]: folders }));
    setTreeChildrenByParent((prev) => ({ ...prev, [key]: sortNodesForTree(listed) }));
  }, []);

  const loadQuota = useCallback(async () => {
    const usage = await runWithRefresh(() => fetchQuota());
    setQuota({
      usedBytes: usage.used_bytes,
      limitBytes: usage.limit_bytes,
      remainingBytes: usage.remaining_bytes
    });
  }, [runWithRefresh]);

  const loadActiveShares = useCallback(async () => {
    try {
      const shares = await runWithRefresh(() => listShares());
      const next: Record<string, boolean> = {};
      for (const share of shares) {
        if (isShareActive(share)) {
          next[share.node_id] = true;
        }
      }
      setActiveSharedNodeIDs(next);
    } catch {
      // keep file list usable even if shares query fails
    }
  }, [runWithRefresh]);

  useEffect(() => {
    const handleSharesUpdated = () => {
      void loadActiveShares();
    };
    window.addEventListener("shares-updated", handleSharesUpdated);
    return () => window.removeEventListener("shares-updated", handleSharesUpdated);
  }, [loadActiveShares]);

  const loadNodes = useCallback(
    async (parentID: string | null, updateCurrent = true) => {
      const requestSeq = loadNodesRequestSeqRef.current + 1;
      loadNodesRequestSeqRef.current = requestSeq;
      setNodesLoading(true);
      setNodesError(null);
      try {
        const listed = await runWithRefresh(() => listNodes(parentID));
        if (requestSeq !== loadNodesRequestSeqRef.current) {
          return;
        }
        void loadActiveShares();
        syncNodeCache(listed, parentID);
        if (updateCurrent) {
          setNodes(listed);
        }
      } catch (error) {
        if (requestSeq !== loadNodesRequestSeqRef.current) {
          return;
        }
        const message = normalizeErrorMessage(error);
        setNodesError(message);
      } finally {
        if (requestSeq === loadNodesRequestSeqRef.current) {
          setNodesLoading(false);
        }
      }
    },
    [loadActiveShares, runWithRefresh, syncNodeCache]
  );

  const loadTreeChildren = useCallback(
    async (parentID: string | null, forceRefresh = false) => {
      const key = cacheKeyForParent(parentID);
      if (!forceRefresh && treeChildrenByParent[key]) {
        return;
      }

      const loadingKey = parentID ?? "__root__";
      setTreeLoadingFolderIDs((prev) => ({ ...prev, [loadingKey]: true }));
      try {
        const listed = await runWithRefresh(() => listNodes(parentID));
        syncNodeCache(listed, parentID);
      } catch (error) {
        showFlash("error", normalizeErrorMessage(error));
      } finally {
        setTreeLoadingFolderIDs((prev) => {
          const next = { ...prev };
          delete next[loadingKey];
          return next;
        });
      }
    },
    [runWithRefresh, showFlash, syncNodeCache, treeChildrenByParent]
  );

  const resolveBreadcrumbs = useCallback(
    (targetID: string | null, preferredName?: string): FolderCrumb[] => {
      if (targetID === null) {
        return [{ id: null, name: rootLabel }];
      }
      const existingIndex = breadcrumbs.findIndex((crumb) => crumb.id === targetID);
      if (existingIndex >= 0) {
        return breadcrumbs.slice(0, existingIndex + 1);
      }

      const chain: FolderCrumb[] = [];
      let cursor: string | null = targetID;
      let guard = 0;
      while (cursor !== null && guard < 128) {
        guard += 1;
        const cursorID: string = cursor;
        const cursorNode: NodeRecord | undefined = nodeIndex[cursorID] as NodeRecord | undefined;
        if (!cursorNode) {
          chain.push({ id: cursorID, name: preferredName ?? t("file.folder", "Folder") });
          break;
        }
        chain.push({ id: cursorNode.id, name: cursorNode.name });
        cursor = cursorNode.parent_id;
      }
      chain.reverse();
      return [{ id: null, name: rootLabel }, ...chain];
    },
    [breadcrumbs, nodeIndex, rootLabel, t]
  );

  const goToFolder = useCallback(
    async (parentID: string | null, preferredName?: string) => {
      setCurrentParentID(parentID);
      setBreadcrumbs(resolveBreadcrumbs(parentID, preferredName));
      setSelectedNodeIDs([]);
      setSelectedNodeIDForDetails(null);
      setMenuNodeID(null);
      await loadNodes(parentID, true);
    },
    [loadNodes, resolveBreadcrumbs]
  );

  const refreshCurrentFolder = useCallback(async () => {
    await loadNodes(currentParentRef.current, true);

    const expandedIDs = Object.entries(expandedTreeFolderIDs)
      .filter(([, expanded]) => expanded)
      .map(([folderID]) => folderID);
    const treeRefreshTargets: Array<string | null> = [null, ...expandedIDs];

    for (const parentID of treeRefreshTargets) {
      try {
        const listed = await runWithRefresh(() => listNodes(parentID));
        syncNodeCache(listed, parentID);
      } catch {
        // ignore tree refresh errors here; current folder refresh already surfaced failures
      }
    }

    await loadQuota();
  }, [expandedTreeFolderIDs, loadNodes, loadQuota, runWithRefresh, syncNodeCache]);

  const loadNotifications = useCallback(async () => {
    try {
      const items = await runWithRefresh(() => listNotifications(50));
      setNotifications(items);
    } catch {
      // keep silent in background polling
    }
  }, [runWithRefresh]);

  useEffect(() => {
    let active = true;
    setAuthLoading(true);
    void (async () => {
      try {
        const me = await fetchMe();
        if (!active) {
          return;
        }
        authBrokenRef.current = false;
        setUser(me);
      } catch (error) {
        if (error instanceof ApiError && error.status === 401) {
          authBrokenRef.current = true;
          setUser(null);
        } else {
          showFlash("error", normalizeErrorMessage(error));
        }
      } finally {
        if (active) {
          setAuthLoading(false);
        }
      }
    })();
    return () => {
      active = false;
    };
  }, [rootLabel, showFlash]);

  useEffect(() => {
    if (!user) {
      initializedWorkspaceUserRef.current = null;
      return;
    }
    if (initializedWorkspaceUserRef.current === user.id) {
      return;
    }
    initializedWorkspaceUserRef.current = user.id;
    void (async () => {
      try {
        await goToFolder(null);
        await loadQuota();
        await loadNotifications();
      } catch (error) {
        showFlash("error", normalizeErrorMessage(error));
      }
    })();
  }, [goToFolder, loadNotifications, loadQuota, showFlash, user]);

  useEffect(() => {
    if (!user) {
      return;
    }
    const timer = window.setInterval(() => {
      void loadNotifications();
    }, 30000);
    return () => window.clearInterval(timer);
  }, [loadNotifications, user]);

  useEffect(() => {
    if (!user) {
      return;
    }
    setProfileDisplayName(user.display_name);
    let preferred: "en" | "tr" = "en";
    if (user.preferred_language === "tr" || user.preferred_language === "en") {
      preferred = user.preferred_language;
    } else if (typeof window !== "undefined") {
      const stored = window.localStorage.getItem("bitrox_locale");
      if (stored === "tr" || stored === "en") {
        preferred = stored;
      } else if (settings.default_language === "tr" || settings.default_language === "en") {
        preferred = settings.default_language;
      }
    } else if (settings.default_language === "tr" || settings.default_language === "en") {
      preferred = settings.default_language;
    }
    setProfileLanguage(preferred);
    setLocale(preferred, false);
  }, [setLocale, settings.default_language, user]);

  const canAccessAdmin = user?.role === "owner" || user?.role === "admin";

  const {
    uploadJobs,
    runningUploadCount,
    confirmLeaveIfUploading,
    handleUploadFiles,
    retryUploadJob,
    cancelUploadJob
  } = useUploadManager({
    defaultChunkSizeBytes,
    currentParentRef,
    refreshCurrentFolder,
    runWithRefresh,
    showFlash
  });

  const submitProfileChanges = useCallback(async () => {
    if (!user) {
      return;
    }
    const nextDisplayName = profileDisplayName.trim();
    if (nextDisplayName.length === 0) {
      showFlash("error", t("profile.displayNameRequired", "Display name cannot be empty."));
      return;
    }

    setProfileSaving(true);
    try {
      const updated = await runWithRefresh(() =>
        updateMe({
          display_name: nextDisplayName,
          preferred_language: profileLanguage
        })
      );
      setUser(updated);
      setLocale(profileLanguage, true);
      showFlash("success", t("profile.saved", "Profile saved."));
    } catch (error) {
      showFlash("error", normalizeErrorMessage(error));
    } finally {
      setProfileSaving(false);
    }
  }, [profileDisplayName, profileLanguage, runWithRefresh, setLocale, showFlash, t, user]);

  const storageUsagePercent = useMemo(() => {
    if (!quota || quota.limitBytes <= 0) {
      return 0;
    }
    return Math.min(100, Math.round((quota.usedBytes / quota.limitBytes) * 100));
  }, [quota]);

  const rootFolders = useMemo(() => folderChildrenByParent[cacheKeyForParent(null)] ?? [], [folderChildrenByParent]);
  const sidebarTreeRootNodes = useMemo(
    () => treeChildrenByParent[cacheKeyForParent(null)] ?? rootFolders,
    [rootFolders, treeChildrenByParent]
  );
  const localizedNodeTypeFilterOptions = useMemo(
    () =>
      nodeTypeFilterOptions.map((option) => ({
        ...option,
        label:
          option.value === "all"
            ? t("file.type.all", "All types")
            : option.value === "folder"
            ? t("file.type.folder", "Folders")
            : option.value === "document"
            ? t("file.type.document", "Documents")
            : option.value === "text"
            ? t("file.type.text", "Text")
            : option.value === "image"
            ? t("file.type.image", "Images")
            : option.value === "video"
            ? t("file.type.video", "Videos")
            : option.value === "audio"
            ? t("file.type.audio", "Audio")
            : option.value === "archive"
            ? t("file.type.archive", "Archives")
            : option.value === "code"
            ? t("file.type.code", "Code/Data")
            : t("file.type.other", "Other files")
      })),
    [t]
  );

  const filteredNodes = useMemo(() => {
    const query = searchTerm.trim().toLowerCase();
    return nodes.filter((node) => {
      if (!matchesNodeTypeFilter(node, typeFilter)) {
        return false;
      }
      if (query === "") {
        return true;
      }
      return node.name.toLowerCase().includes(query);
    });
  }, [nodes, searchTerm, typeFilter]);
  const hasActiveFilters = searchTerm.trim() !== "" || typeFilter !== "all";

  const visibleNodes = useMemo(() => sortNodes(filteredNodes, sortKey, sortDirection), [filteredNodes, sortDirection, sortKey]);

  const selectedNodes = useMemo(() => nodes.filter((node) => selectedNodeIDs.includes(node.id)), [nodes, selectedNodeIDs]);

  useEffect(() => {
    setSelectedNodeIDs((prev) => prev.filter((id) => filteredNodes.some((node) => node.id === id)));
  }, [filteredNodes]);

  const selectedNodeForDetails = useMemo(
    () => (selectedNodeIDForDetails ? nodeIndex[selectedNodeIDForDetails] ?? null : null),
    [nodeIndex, selectedNodeIDForDetails]
  );

  const allFoldersForMove = useMemo(() => {
    const folders = Object.values(nodeIndex).filter((node) => node.type === "folder");
    folders.sort((a, b) => a.name.localeCompare(b.name));
    return folders;
  }, [nodeIndex]);

  const onDropFiles = useCallback(
    (event: DragEvent<HTMLElement>) => {
      event.preventDefault();
      event.stopPropagation();
      dragDepthRef.current = 0;
      setDragActive(false);
      if (!dragHasFiles(event)) {
        return;
      }
      const files = Array.from(event.dataTransfer.files ?? []);
      void handleUploadFiles(files);
    },
    [handleUploadFiles]
  );

  const onWorkspaceDragEnter = useCallback((event: DragEvent<HTMLElement>) => {
    if (!dragHasFiles(event)) {
      return;
    }
    event.preventDefault();
    event.stopPropagation();
    dragDepthRef.current += 1;
    setDragActive(true);
  }, []);

  const onWorkspaceDragOver = useCallback((event: DragEvent<HTMLElement>) => {
    if (!dragHasFiles(event)) {
      return;
    }
    event.preventDefault();
    event.stopPropagation();
    event.dataTransfer.dropEffect = "copy";
    setDragActive(true);
  }, []);

  const onWorkspaceDragLeave = useCallback((event: DragEvent<HTMLElement>) => {
    event.preventDefault();
    event.stopPropagation();
    dragDepthRef.current = Math.max(0, dragDepthRef.current - 1);
    if (dragDepthRef.current === 0) {
      setDragActive(false);
    }
  }, []);

  const handleLogin = useCallback(
    async (event: FormEvent<HTMLFormElement>) => {
      event.preventDefault();
      setLoginBusy(true);
      try {
        await login(email, password);
        const me = await fetchMe();
        authBrokenRef.current = false;
        setUser(me);
        setPassword("");
        showFlash("success", t("auth.loginSuccess", "Login successful."));
      } catch (error) {
        showFlash("error", normalizeErrorMessage(error));
      } finally {
        setLoginBusy(false);
      }
    },
    [email, password, showFlash, t]
  );

  const handleLogout = useCallback(async () => {
    const csrf = readCSRFTokenFromCookie();
    if (!csrf) {
      showFlash("error", t("auth.csrfMissing", "CSRF token is missing. Refresh the page and try again."));
      return;
    }
    setActionBusy(true);
    try {
      await logout(csrf);
      authBrokenRef.current = true;
      setUser(null);
      setNodes([]);
        setNodeIndex({});
        setFolderChildrenByParent({});
        setTreeChildrenByParent({});
        setActiveSharedNodeIDs({});
        setExpandedTreeFolderIDs({});
      setTreeLoadingFolderIDs({});
      setSelectedNodeIDs([]);
      setSelectedNodeIDForDetails(null);
      setCurrentParentID(null);
      setBreadcrumbs([{ id: null, name: rootLabel }]);
      setNotifications([]);
      setUserMenuOpen(false);
      showFlash("info", t("auth.logoutInfo", "Signed out."));
    } catch (error) {
      showFlash("error", normalizeErrorMessage(error));
    } finally {
      setActionBusy(false);
    }
  }, [rootLabel, showFlash, t]);

  const handleMarkNotificationRead = useCallback(
    async (notificationID: string) => {
      try {
        const updated = await runWithRefresh(() => markNotificationRead(notificationID));
        setNotifications((prev) => prev.map((item) => (item.id === updated.id ? updated : item)));
      } catch (error) {
        showFlash("error", normalizeErrorMessage(error));
      }
    },
    [runWithRefresh, showFlash]
  );

  const changeSort = useCallback(
    (key: SortKey) => {
      if (key === sortKey) {
        setSortDirection((prev) => (prev === "asc" ? "desc" : "asc"));
      } else {
        setSortKey(key);
        setSortDirection("asc");
      }
    },
    [sortKey]
  );

  const toggleSelectNode = useCallback((nodeID: string) => {
    setSelectedNodeIDs((prev) => (prev.includes(nodeID) ? prev.filter((id) => id !== nodeID) : [...prev, nodeID]));
  }, []);

  const toggleSelectAll = useCallback(() => {
    setSelectedNodeIDs((prev) => {
      if (visibleNodes.length > 0 && prev.length === visibleNodes.length) {
        return [];
      }
      return visibleNodes.map((node) => node.id);
    });
  }, [visibleNodes]);

  const openCreateFolderModal = useCallback(() => {
    setCreateFolderName("");
    setShowCreateModal(true);
  }, []);

  const submitCreateFolder = useCallback(async () => {
    const name = createFolderName.trim();
    if (name.length === 0) {
      showFlash("error", t("file.folderNameRequired", "Folder name cannot be empty."));
      return;
    }

    setActionBusy(true);
    try {
      await runWithRefresh(() => createFolder(name, currentParentRef.current));
      setShowCreateModal(false);
      setCreateFolderName("");
      await refreshCurrentFolder();
      showFlash("success", t("file.folderCreated", "Folder created: {name}", { name }));
    } catch (error) {
      showFlash("error", normalizeErrorMessage(error));
    } finally {
      setActionBusy(false);
    }
  }, [createFolderName, refreshCurrentFolder, runWithRefresh, showFlash, t]);

  const openCreateTextModal = useCallback(() => {
    setCreateTextFileName(t("file.newTextDefaultName", "new-file.txt"));
    setShowCreateTextModal(true);
  }, [t]);

  const submitCreateTextFile = useCallback(async () => {
    const rawName = createTextFileName.trim();
    if (rawName.length === 0) {
      showFlash("error", t("file.fileNameRequired", "File name cannot be empty."));
      return;
    }
    const fileName = rawName.toLowerCase().endsWith(".txt") ? rawName : `${rawName}.txt`;
    const file = new File(["\n"], fileName, { type: "text/plain;charset=utf-8" });
    setShowCreateTextModal(false);
    setCreateTextFileName("");
    await handleUploadFiles([file], { successMessage: t("file.textFileCreated", "{name} created.", { name: fileName }) });
  }, [createTextFileName, handleUploadFiles, showFlash, t]);

  const downloadNodeFile = useCallback(
    async (node: NodeRecord) => {
      try {
        const response = await fetch(downloadUrlForNode(node.id), {
          method: "GET",
          credentials: "include"
        });
        if (!response.ok) {
          throw new Error(t("file.downloadFailedHttp", "Download failed (HTTP {status}).", { status: response.status }));
        }
        const blob = await response.blob();
        const headerFileName = extractFileNameFromDisposition(response.headers.get("content-disposition"));
        const fallbackName = node.type === "folder" ? `${node.name}.zip` : node.name;
        const fileName = (headerFileName && headerFileName.length > 0 ? headerFileName : fallbackName).replace(/[\\/:*?"<>|]+/g, "_");
        const objectURL = URL.createObjectURL(blob);
        const anchor = document.createElement("a");
        anchor.href = objectURL;
        anchor.download = fileName;
        anchor.rel = "noopener";
        anchor.style.display = "none";
        document.body.appendChild(anchor);
        anchor.click();
        document.body.removeChild(anchor);
        window.setTimeout(() => URL.revokeObjectURL(objectURL), 1500);
      } catch (error) {
        showFlash("error", normalizeErrorMessage(error));
      }
    },
    [showFlash, t]
  );

  const openNodeFile = useCallback(
    async (node: NodeRecord) => {
      if (node.type !== "file") {
        return;
      }
      if (!isTextEditableNode(node)) {
        await downloadNodeFile(node);
        return;
      }

      setTextEditorTarget(node);
      setTextEditorLoading(true);
      setTextEditorContent("");
      try {
        const response = await fetch(downloadUrlForNode(node.id), {
          method: "GET",
          credentials: "include"
        });
        if (!response.ok) {
          throw new Error(t("file.openFailedHttp", "File could not be opened (HTTP {status}).", { status: response.status }));
        }
        const content = await response.text();
        setTextEditorContent(content);
      } catch (error) {
        setTextEditorTarget(null);
        showFlash("error", normalizeErrorMessage(error));
      } finally {
        setTextEditorLoading(false);
      }
    },
    [downloadNodeFile, showFlash, t]
  );

  const submitSaveTextFile = useCallback(async () => {
    if (!textEditorTarget) {
      return;
    }
    const normalizedContent = textEditorContent.length > 0 ? textEditorContent : "\n";
    const mimeType = textEditorTarget.mime_type?.trim() || "text/plain;charset=utf-8";
    const updatedFile = new File([normalizedContent], textEditorTarget.name, { type: mimeType });

    setTextEditorSaving(true);
    try {
      const ok = await handleUploadFiles([updatedFile], {
        targetType: "new_version",
        targetNodeID: textEditorTarget.id,
        successMessage: t("file.saved", "{name} saved.", { name: textEditorTarget.name })
      });
      if (ok) {
        setTextEditorTarget(null);
        setTextEditorContent("");
      }
    } finally {
      setTextEditorSaving(false);
    }
  }, [handleUploadFiles, t, textEditorContent, textEditorTarget]);

  const openRenameModal = useCallback((node: NodeRecord) => {
    setRenameTarget(node);
    setRenameName(node.name);
    setMenuNodeID(null);
  }, []);

  const submitRename = useCallback(async () => {
    if (!renameTarget) {
      return;
    }
    const name = renameName.trim();
    if (name.length === 0) {
      showFlash("error", t("file.renameRequired", "New name cannot be empty."));
      return;
    }
    setActionBusy(true);
    try {
      await runWithRefresh(() => renameNode(renameTarget.id, name));
      setRenameTarget(null);
      setRenameName("");
      await refreshCurrentFolder();
      showFlash("success", t("file.renamed", "Item renamed."));
    } catch (error) {
      showFlash("error", normalizeErrorMessage(error));
    } finally {
      setActionBusy(false);
    }
  }, [refreshCurrentFolder, renameName, renameTarget, runWithRefresh, showFlash, t]);

  const openDeleteModal = useCallback((targets: NodeRecord[]) => {
    setDeleteTargets(targets);
    setMenuNodeID(null);
  }, []);

  const submitDelete = useCallback(async () => {
    if (deleteTargets.length === 0) {
      return;
    }
    setActionBusy(true);
    try {
      const settled = await Promise.allSettled(deleteTargets.map((node) => runWithRefresh(() => deleteNode(node.id))));
      const deletedIDs = deleteTargets.filter((_, index) => settled[index]?.status === "fulfilled").map((node) => node.id);
      const failureCount = settled.length - deletedIDs.length;

      setDeleteTargets([]);
      setSelectedNodeIDs((prev) => prev.filter((id) => !deletedIDs.includes(id)));
      if (selectedNodeIDForDetails && deletedIDs.includes(selectedNodeIDForDetails)) {
        setSelectedNodeIDForDetails(null);
      }

      await refreshCurrentFolder();
      if (failureCount === 0) {
        showFlash("success", t("file.deletedCount", "{count} item(s) deleted.", { count: deletedIDs.length }));
      } else {
        const firstFailure = settled.find((result): result is PromiseRejectedResult => result.status === "rejected");
        showFlash(
          "info",
          t("file.deleteMixedResult", "{ok} deleted, {failed} failed. {reason}", {
            ok: deletedIDs.length,
            failed: failureCount,
            reason: firstFailure ? normalizeErrorMessage(firstFailure.reason) : ""
          }).trim()
        );
      }
    } finally {
      setActionBusy(false);
    }
  }, [deleteTargets, refreshCurrentFolder, runWithRefresh, selectedNodeIDForDetails, showFlash, t]);

  const isDescendantFolder = useCallback(
    (candidateFolderID: string, ancestorFolderID: string): boolean => {
      let cursor: string | null = candidateFolderID;
      let guard = 0;
      while (cursor && guard < 256) {
        guard += 1;
        if (cursor === ancestorFolderID) {
          return true;
        }
        cursor = nodeIndex[cursor]?.parent_id ?? null;
      }
      return false;
    },
    [nodeIndex]
  );

  const openMoveModal = useCallback((targets: NodeRecord[]) => {
    if (targets.length === 0) {
      showFlash("error", t("file.moveSelectRequired", "Select at least one item to move."));
      return;
    }
    setMoveTargets(targets);
    setMoveDestinationID(currentParentRef.current);
    setShowMoveModal(true);
    setMenuNodeID(null);
  }, [showFlash, t]);

  const moveValidationMessage = useMemo(() => {
    if (!showMoveModal || moveTargets.length === 0) {
      return null;
    }
    const invalidTargets = moveTargets.filter((node) => {
      if (moveDestinationID === null) {
        return false;
      }
      if (node.id === moveDestinationID) {
        return true;
      }
      if (node.type === "folder" && isDescendantFolder(moveDestinationID, node.id)) {
        return true;
      }
      return false;
    });
    if (invalidTargets.length === 0) {
      return null;
    }
    if (invalidTargets.length === 1) {
      return t("file.moveSelfInvalid", "'{name}' folder cannot be moved into itself.", { name: invalidTargets[0]?.name ?? "" });
    }
    return t("file.moveInvalidCount", "{count} selected item(s) cannot be moved to this destination.", {
      count: invalidTargets.length
    });
  }, [isDescendantFolder, moveDestinationID, moveTargets, showMoveModal, t]);

  const submitMove = useCallback(async () => {
    if (moveValidationMessage) {
      showFlash("error", moveValidationMessage);
      return;
    }
    if (moveTargets.length === 0) {
      return;
    }
    setActionBusy(true);
    try {
      const settled = await Promise.allSettled(moveTargets.map((node) => runWithRefresh(() => moveNode(node.id, moveDestinationID))));
      const movedCount = settled.filter((result) => result.status === "fulfilled").length;
      const failureCount = settled.length - movedCount;
      setShowMoveModal(false);
      setMoveTargets([]);
      setSelectedNodeIDs([]);
      await refreshCurrentFolder();
      if (failureCount === 0) {
        showFlash("success", t("file.movedCount", "{count} item(s) moved.", { count: movedCount }));
      } else {
        showFlash("info", t("file.moveMixedResult", "{ok} moved, {failed} failed.", { ok: movedCount, failed: failureCount }));
      }
    } finally {
      setActionBusy(false);
    }
  }, [moveDestinationID, moveTargets, moveValidationMessage, refreshCurrentFolder, runWithRefresh, showFlash, t]);

  const effectiveDisplayName = resolveDisplayName({
    display_name: profileDisplayName.trim() || user?.display_name,
    email: user?.email
  });
  const effectiveAvatarInitials = userInitials(effectiveDisplayName, user?.email);
  const activePage: DashboardPage = page;

  const navigateWithUploadGuard = useCallback(
    (target: string) => {
      if (target === "/" || target === "/profile" || target === "/calendar" || target === "/admin" || target === "/shares") {
        if (target !== window.location.pathname && !confirmLeaveIfUploading()) {
          return;
        }
      }
      setUserMenuOpen(false);
      router.push(target);
    },
    [confirmLeaveIfUploading, router]
  );

  const toggleSidebarTreeFolder = useCallback(
    (folder: NodeRecord) => {
      let shouldLoadChildren = false;
      setExpandedTreeFolderIDs((prev) => {
        const wasExpanded = !!prev[folder.id];
        shouldLoadChildren = !wasExpanded;
        return { ...prev, [folder.id]: !wasExpanded };
      });
      if (shouldLoadChildren) {
        void loadTreeChildren(folder.id, true);
      }
    },
    [loadTreeChildren]
  );

  const openSidebarFolder = useCallback(
    async (folder: NodeRecord) => {
      setExpandedTreeFolderIDs((prev) => ({ ...prev, [folder.id]: true }));
      if (page !== "files") {
        if (!confirmLeaveIfUploading()) {
          return;
        }
        try {
          window.sessionStorage.setItem(
            pendingSidebarFolderStorageKey,
            JSON.stringify({ id: folder.id, name: folder.name })
          );
        } catch {
          // ignore storage write errors
        }
        router.push("/");
        return;
      }
      await goToFolder(folder.id, folder.name);
    },
    [confirmLeaveIfUploading, goToFolder, page, router]
  );

  const openSidebarFile = useCallback(
    (file: NodeRecord) => {
      if (page !== "files") {
        if (!confirmLeaveIfUploading()) {
          return;
        }
        router.push("/");
        return;
      }
      setSelectedNodeIDForDetails(file.id);
    },
    [confirmLeaveIfUploading, page, router]
  );

  useEffect(() => {
    if (page !== "files" || !user) {
      return;
    }
    try {
      const raw = window.sessionStorage.getItem(pendingSidebarFolderStorageKey);
      if (!raw) {
        return;
      }
      window.sessionStorage.removeItem(pendingSidebarFolderStorageKey);
      const parsed = JSON.parse(raw) as { id?: string; name?: string };
      if (parsed?.id) {
        setExpandedTreeFolderIDs((prev) => ({ ...prev, [parsed.id as string]: true }));
        void goToFolder(parsed.id, parsed.name ?? t("file.folder", "Folder"));
      }
    } catch {
      // ignore malformed state
    }
  }, [goToFolder, page, t, user]);

  if (authLoading) {
    return (
      <main className="grid min-h-screen place-items-center bg-[var(--bg-main)]">
        <p className="text-sm text-[var(--text-muted)]">{t("auth.sessionChecking", "Checking session...")}</p>
      </main>
    );
  }

  if (!user) {
    return (
      <>
        <AuthView
          email={email}
          password={password}
          loading={loginBusy}
          onEmailChange={setEmail}
          onPasswordChange={setPassword}
          onSubmit={handleLogin}
        />
      </>
    );
  }

  return (
    <div className="min-h-screen bg-[var(--bg-main)] text-[var(--text-main)]">
      <div className="flex min-h-screen flex-col lg:flex-row">
        <aside className="surface-card custom-scrollbar w-full border-r border-[var(--line)] lg:sticky lg:top-0 lg:h-screen lg:w-72 lg:overflow-hidden" data-testid="sidebar-panel">
          <div className="flex h-full flex-col p-4">
            <div className="mb-5 flex items-center gap-3">
              {settings.logo_url && !logoLoadFailed ? (
                /* eslint-disable-next-line @next/next/no-img-element */
                <img
                  src={settings.logo_url}
                  alt={settings.site_name || t("common.logo", "Logo")}
                  className="h-10 w-10 rounded-xl border border-[var(--line)] object-cover"
                  onError={(event) => {
                    event.currentTarget.style.display = "none";
                    setLogoLoadFailed(true);
                  }}
                />
              ) : (
                <div className="grid h-10 w-10 place-items-center rounded-xl bg-[var(--brand)] text-white">
                  <i className="fa-solid fa-cloud" />
                </div>
              )}
              <div>
                <p className="text-sm font-semibold tracking-wide">{settings.site_name || "BitroxCloud"}</p>
                <p className="text-xs text-[var(--text-muted)]">{settings.site_subtitle || t("app.privateStorage", "Private storage")}</p>
              </div>
            </div>

            <nav className="space-y-1">
              <button
                type="button"
                onClick={() => navigateWithUploadGuard("/")}
                data-testid="sidebar-nav-files"
                className={`focus-ring flex w-full items-center gap-2 rounded-lg px-3 py-2 text-left text-sm ${
                  activePage === "files" ? "bg-[var(--brand-soft)] text-[var(--brand)]" : "hover:bg-[var(--bg-soft)]"
                }`}
              >
                <i className="fa-solid fa-hard-drive" />
                {t("sidebar.files", "Files")}
              </button>
              <button
                type="button"
                onClick={() => navigateWithUploadGuard("/shares")}
                data-testid="sidebar-nav-shares"
                className={`focus-ring flex w-full items-center gap-2 rounded-lg px-3 py-2 text-left text-sm ${
                  activePage === "shares" ? "bg-[var(--brand-soft)] text-[var(--brand)]" : "hover:bg-[var(--bg-soft)]"
                }`}
              >
                <i className="fa-solid fa-share-nodes" />
                {t("sidebar.shares", "Shares")}
              </button>
              <button
                type="button"
                onClick={() => navigateWithUploadGuard("/calendar")}
                data-testid="sidebar-nav-calendar"
                className={`focus-ring flex w-full items-center gap-2 rounded-lg px-3 py-2 text-left text-sm ${
                  activePage === "calendar" ? "bg-[var(--brand-soft)] text-[var(--brand)]" : "hover:bg-[var(--bg-soft)]"
                }`}
              >
                <i className="fa-regular fa-calendar-days" />
                {t("sidebar.calendar", "Calendar")}
              </button>
            </nav>

            <div className="mt-4 min-h-0 flex-1 overflow-y-auto custom-scrollbar">
              <p className="mb-2 text-xs uppercase tracking-wide text-[var(--text-muted)] sticky top-0 bg-[var(--bg-card)] py-1">{t("sidebar.allFiles", "All Files")}</p>
              <SidebarTree
                rootNodes={sidebarTreeRootNodes}
                childrenByParent={treeChildrenByParent}
                expandedFolderIDs={expandedTreeFolderIDs}
                loadingFolderIDs={treeLoadingFolderIDs}
                activeFolderID={currentParentID}
                activeNodeID={selectedNodeIDForDetails}
                onToggleFolder={toggleSidebarTreeFolder}
                onOpenFolder={(folder) => {
                  void openSidebarFolder(folder);
                }}
                onOpenFile={openSidebarFile}
              />
            </div>

            <div className="mt-6 space-y-3">
              <div className="rounded-xl border border-[var(--line)] bg-[var(--bg-soft)] p-3">
                <p className="text-xs uppercase tracking-wide text-[var(--text-muted)]">{t("sidebar.storage", "Storage")}</p>
                <p className="mt-2 text-lg font-semibold">
                  {quota ? formatBytes(quota.usedBytes) : "0 B"}
                  <span className="text-sm font-normal text-[var(--text-muted)]"> / {quota ? formatBytes(quota.limitBytes) : "..."}</span>
                </p>
                <div className="mt-2 h-2 rounded-full bg-black/10 dark:bg-white/10">
                  <div className="h-2 rounded-full bg-[var(--brand)]" style={{ width: `${storageUsagePercent}%` }} />
                </div>
                <p className="mt-2 text-xs text-[var(--text-muted)]">
                  {quota ? formatBytes(quota.remainingBytes) : "0 B"} {t("sidebar.freeSpace", "free space")}
                </p>
              </div>

              <div className="relative rounded-xl border border-[var(--line)] bg-[var(--bg-soft)] p-3" data-user-menu-anchor>
                <button
                  type="button"
                  onClick={() => setUserMenuOpen((prev) => !prev)}
                  data-testid="sidebar-user-menu-toggle"
                  className="focus-ring flex w-full items-center gap-3 rounded-lg px-1 py-1.5 text-left hover:bg-[var(--bg-card)]"
                >
                  <div className="grid h-10 w-10 place-items-center rounded-full text-sm font-semibold text-white" style={{ backgroundColor: profilePrefs.avatarColor }}>
                    {effectiveAvatarInitials}
                  </div>
                  <div className="min-w-0">
                    <p className="truncate text-sm font-semibold text-[var(--text-main)]">{effectiveDisplayName}</p>
                    <p className="truncate text-xs text-[var(--text-muted)]">{user.email}</p>
                  </div>
                  <i className={`fa-solid ml-auto text-xs text-[var(--text-muted)] ${userMenuOpen ? "fa-chevron-up" : "fa-chevron-down"}`} />
                </button>

                {userMenuOpen ? (
                  <div className="absolute bottom-full left-0 z-30 mb-2 w-full rounded-xl border border-[var(--line)] bg-[var(--bg-card)] p-2 shadow-2xl">
                    <button
                      type="button"
                      onClick={() => navigateWithUploadGuard("/profile")}
                      data-testid="sidebar-user-menu-profile"
                      className="focus-ring flex w-full items-center gap-2 rounded-lg px-2 py-2 text-left text-sm hover:bg-[var(--bg-soft)]"
                    >
                      <i className="fa-regular fa-user" />
                      {t("user.profile", "Profile")}
                    </button>
                    {canAccessAdmin ? (
                      <button
                        type="button"
                        onClick={() => navigateWithUploadGuard("/admin")}
                        data-testid="sidebar-user-menu-admin"
                        className="focus-ring flex w-full items-center gap-2 rounded-lg px-2 py-2 text-left text-sm hover:bg-[var(--bg-soft)]"
                      >
                        <i className="fa-solid fa-user-shield" />
                        {t("user.adminPanel", "Admin Panel")}
                      </button>
                    ) : null}
                    <button
                      type="button"
                      onClick={() => void handleLogout()}
                      disabled={actionBusy}
                      data-testid="sidebar-user-menu-logout"
                      className="focus-ring mt-1 flex w-full items-center gap-2 rounded-lg px-2 py-2 text-left text-sm text-red-500 hover:bg-red-500/10 disabled:opacity-60"
                    >
                      <i className="fa-solid fa-right-from-bracket" />
                      {t("user.logout", "Sign Out")}
                    </button>
                  </div>
                ) : null}
              </div>
            </div>
          </div>
        </aside>

        <main
          className={`custom-scrollbar relative flex-1 overflow-y-auto p-4 lg:p-6 ${
            activePage === "files" ? "lg:overflow-hidden" : ""
          }`}
          onDragEnter={activePage === "files" ? onWorkspaceDragEnter : undefined}
          onDragOver={activePage === "files" ? onWorkspaceDragOver : undefined}
          onDragLeave={activePage === "files" ? onWorkspaceDragLeave : undefined}
          onDrop={activePage === "files" ? onDropFiles : undefined}
        >
          <WorkspaceHeader
            appName={settings.site_name || "BitroxCloud"}
            pageLabel={
              activePage === "files"
                ? t("workspace.files", "Files")
                : activePage === "calendar"
                ? t("workspace.calendar", "Calendar")
                : activePage === "shares"
                ? t("workspace.shares", "Shares")
                : t("workspace.profile", "Profile")
            }
            runningUploadCount={runningUploadCount}
            uploadJobs={uploadJobs}
            onRetryUpload={retryUploadJob}
            onCancelUpload={(job) => {
              void cancelUploadJob(job);
            }}
            notifications={notifications}
            onMarkNotificationRead={(notificationID) => {
              void handleMarkNotificationRead(notificationID);
            }}
            rightSlot={<ThemeToggle />}
          />

          {dragActive && activePage === "files" ? (
            <div className="pointer-events-none absolute inset-6 z-20 grid place-items-center rounded-2xl border-2 border-dashed border-[var(--brand)] bg-[var(--brand-soft)]/30">
              <p className="rounded-xl bg-[var(--bg-card)] px-4 py-2 text-sm font-medium text-[var(--text-main)] shadow-lg">{t("file.dragDropOverlay", "Drop files to upload instantly")}</p>
            </div>
          ) : null}

          {activePage === "files" ? (
            <section className="space-y-4 lg:flex lg:h-[calc(100vh-10.25rem)] lg:min-h-0 lg:flex-col">
              <FileBrowserToolbar
                breadcrumbs={breadcrumbs}
                searchTerm={searchTerm}
                typeFilter={typeFilter}
                typeFilterOptions={localizedNodeTypeFilterOptions}
                selectedCount={selectedNodes.length}
                viewMode={viewMode}
                actionBusy={actionBusy}
                nodesLoading={nodesLoading}
                onGoToFolder={(folderID, folderName) => void goToFolder(folderID, folderName)}
                onSearchTermChange={setSearchTerm}
                onTypeFilterChange={(value) => setTypeFilter(value as NodeTypeFilter)}
                onSetViewMode={setViewMode}
                onUploadFiles={(files) => {
                  void handleUploadFiles(files);
                }}
                onOpenCreateFolder={openCreateFolderModal}
                onOpenCreateTextFile={openCreateTextModal}
                onOpenMoveSelected={() => openMoveModal(selectedNodes)}
                onOpenDeleteSelected={() => openDeleteModal(selectedNodes)}
                onRefresh={() => {
                  void refreshCurrentFolder();
                }}
              />

              <div className="flex min-h-0 flex-1 gap-4 overflow-hidden">
                <div className="min-w-0 flex-1 overflow-y-auto custom-scrollbar" data-testid="file-table-scroll-container">
                  <FileTable
                    nodes={visibleNodes}
                    loading={nodesLoading}
                    error={nodesError}
                    selectedNodeIDs={selectedNodeIDs}
                    selectedNodeIDForDetails={selectedNodeIDForDetails}
                    sharedNodeIDs={activeSharedNodeIDs}
                    viewMode={viewMode}
                    sortKey={sortKey}
                    sortDirection={sortDirection}
                    menuNodeID={menuNodeID}
                    onRetry={() => void refreshCurrentFolder()}
                    onChangeSort={changeSort}
                    onToggleSelectAll={toggleSelectAll}
                    onToggleSelectNode={toggleSelectNode}
                    onSelectForDetails={setSelectedNodeIDForDetails}
                    onOpenFolder={(node) => void goToFolder(node.id, node.name)}
                    onRenameNode={openRenameModal}
                    onDeleteNode={(node) => openDeleteModal([node])}
                    onSetMenuNodeID={setMenuNodeID}
                    onDownloadNode={(node) => {
                      void downloadNodeFile(node);
                    }}
                    onOpenFile={(node) => void openNodeFile(node)}
                    emptyState={
                      hasActiveFilters
                        ? {
                            title: t("fileTable.noResultsTitle", "No results found"),
                            description: t("fileTable.noResultsDesc", "No item matches current search or filters."),
                            actionLabel: t("fileTable.clearSearchFilter", "Clear Search/Filter"),
                            onAction: () => {
                              setSearchTerm("");
                              setTypeFilter("all");
                            }
                          }
                        : undefined
                    }
                  />
                </div>
                <DetailsPanel
                  node={selectedNodeForDetails}
                  onOpenTextFile={(node) => void openNodeFile(node)}
                  onDownloadFile={(node) => {
                    void downloadNodeFile(node);
                  }}
                />
              </div>
            </section>
          ) : activePage === "calendar" ? (
            <CalendarPanel runWithRefresh={runWithRefresh} showFlash={showFlash} />
          ) : activePage === "shares" ? (
            <SharesPage />
          ) : (
            <ProfileSettingsPanel
              displayName={profileDisplayName}
              preferredLanguage={profileLanguage}
              onDisplayNameChange={setProfileDisplayName}
              onPreferredLanguageChange={setProfileLanguage}
              onSaveProfile={() => {
                void submitProfileChanges();
              }}
              saving={profileSaving}
              profilePrefs={profilePrefs}
              saveProfilePrefs={saveProfilePrefs}
              defaultProfilePrefs={defaultProfilePrefs}
              avatarColorOptions={avatarColorOptions}
              effectiveAvatarInitials={effectiveAvatarInitials}
              userEmail={user.email}
            />
          )}
        </main>
      </div>

      <FileActionModals
        rootLabel={rootLabel}
        actionBusy={actionBusy}
        showCreateModal={showCreateModal}
        createFolderName={createFolderName}
        onChangeCreateFolderName={setCreateFolderName}
        onCloseCreateModal={() => setShowCreateModal(false)}
        onSubmitCreateFolder={() => void submitCreateFolder()}
        showCreateTextModal={showCreateTextModal}
        createTextFileName={createTextFileName}
        onChangeCreateTextFileName={setCreateTextFileName}
        onCloseCreateTextModal={() => setShowCreateTextModal(false)}
        onSubmitCreateTextFile={() => void submitCreateTextFile()}
        textEditorTarget={textEditorTarget}
        textEditorContent={textEditorContent}
        textEditorLoading={textEditorLoading}
        textEditorSaving={textEditorSaving}
        onChangeTextEditorContent={setTextEditorContent}
        onCloseTextEditor={() => {
          if (!textEditorSaving) {
            setTextEditorTarget(null);
            setTextEditorContent("");
          }
        }}
        onSubmitSaveTextFile={() => void submitSaveTextFile()}
        renameTarget={renameTarget}
        renameName={renameName}
        onChangeRenameName={setRenameName}
        onCloseRenameModal={() => setRenameTarget(null)}
        onSubmitRename={() => void submitRename()}
        deleteTargets={deleteTargets}
        onCloseDeleteModal={() => setDeleteTargets([])}
        onSubmitDelete={() => void submitDelete()}
        showMoveModal={showMoveModal}
        moveTargets={moveTargets}
        moveDestinationID={moveDestinationID}
        allFoldersForMove={allFoldersForMove}
        moveValidationMessage={moveValidationMessage}
        onChangeMoveDestinationID={setMoveDestinationID}
        onCloseMoveModal={() => setShowMoveModal(false)}
        onSubmitMove={() => void submitMove()}
      />
    </div>
  );
}
