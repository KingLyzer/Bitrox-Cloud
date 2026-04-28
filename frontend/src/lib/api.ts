const rawApiBaseUrl = (process.env.NEXT_PUBLIC_API_BASE_URL ?? "").trim();
const API_BASE_URL = rawApiBaseUrl.replace(/\/+$/, "");

type ApiErrorBody = {
  error?: {
    code?: string;
    message?: string;
  };
};

export class ApiError extends Error {
  readonly status: number;
  readonly code: string;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.status = status;
    this.code = code;
  }
}

export type AuthUser = {
  id: string;
  email: string;
  display_name: string;
  preferred_language?: string | null;
  role: string;
  is_active?: boolean;
};

export function resolveDisplayName(input: {
  display_name?: string | null;
  name?: string | null;
  email?: string | null;
}): string {
  const displayName = (input.display_name ?? "").trim();
  if (displayName) {
    return displayName;
  }
  const name = (input.name ?? "").trim();
  if (name) {
    return name;
  }
  const email = (input.email ?? "").trim();
  if (email) {
    return email;
  }
  return "User";
}

export type NodeRecord = {
  id: string;
  owner_user_id: string;
  parent_id: string | null;
  type: "file" | "folder";
  name: string;
  size_bytes: number;
  mime_type?: string;
  content_hash?: string;
  current_version_id?: string;
  current_version_no?: number;
  deleted_at?: string | null;
  created_at: string;
  updated_at: string;
};

export type UploadSession = {
  id: string;
  owner_user_id: string;
  target_type: "new_file" | "new_version";
  target_node_id?: string;
  parent_id: string | null;
  file_name: string;
  expected_size_bytes: number;
  chunk_size_bytes: number;
  expected_chunks: number;
  status: string;
  uploaded_bytes: number;
  expires_at: string;
};

export type QuotaUsage = {
  used_bytes: number;
  limit_bytes: number;
  remaining_bytes: number;
  is_limit_exceeded: boolean;
};

export type AdminUser = {
  id: string;
  email: string;
  display_name: string;
  preferred_language?: string | null;
  role: "owner" | "admin" | "user";
  quota_bytes: number | null;
  used_bytes?: number;
  limit_bytes?: number;
  is_active: boolean;
  status?: "active" | "inactive" | "deleted" | "disabled" | "all";
  deleted_at?: string | null;
  created_at: string;
  updated_at: string;
};

export type PublicSettings = {
  site_name: string;
  site_subtitle: string;
  browser_title: string;
  logo_url: string;
  favicon_url: string;
  accent_color: string;
  public_base_url: string;
  default_language: "en" | "tr" | string;
  timezone: string;
  maintenance_mode: boolean;
};

export type AdminAuditEvent = {
  event_type: string;
  severity: string;
  user_id: string | null;
  user_email?: string | null;
  session_id: string | null;
  ip: string;
  user_agent: string;
  user_agent_short?: string;
  device_type?: string;
  browser?: string;
  metadata: Record<string, unknown>;
  created_at: string;
  created_at_human?: string;
};

export type AdminAuditPagination = {
  page: number;
  limit: number;
  total: number;
  total_pages: number;
};

export type AdminAuditResponse = {
  events: AdminAuditEvent[];
  pagination: AdminAuditPagination;
};

export type GeneralSettings = {
  site_name: string;
  site_subtitle: string;
  browser_title: string;
  site_logo_url: string;
  favicon_url: string;
  brand_color: string;
  default_storage_quota_bytes: number;
  default_language: string;
  default_timezone: string;
  public_base_url: string;
  maintenance_mode: boolean;
};

export type SharingSettings = {
  public_sharing_enabled: boolean;
  allow_password_protected_links: boolean;
  allow_expiration: boolean;
  default_expiration_days: number;
  maximum_expiration_days: number;
  allow_public_downloads: boolean;
  allow_folder_sharing: boolean;
  require_password_for_public_links: boolean;
};

export type SecuritySettings = {
  minimum_password_length: number;
  require_uppercase: boolean;
  require_lowercase: boolean;
  require_number: boolean;
  require_symbol: boolean;
  session_lifetime_minutes: number;
  refresh_token_lifetime_minutes: number;
  two_factor_required: boolean;
  login_rate_limit_per_window: number;
  login_rate_window_seconds: number;
  trusted_proxy_note: string;
};

export type OtherSettings = {
  storage_cleanup_interval_seconds: number;
  upload_max_file_size_bytes: number;
  upload_max_chunk_size_bytes: number;
  preview_generation_enabled: boolean;
  logging_level: string;
  requires_restart_note: string;
  system_storage_total_bytes?: number;
  system_storage_used_bytes?: number;
  system_storage_free_bytes?: number;
  system_storage_checked_at?: string;
};

export type AdminSettings = {
  general: GeneralSettings;
  sharing: SharingSettings;
  security: SecuritySettings;
  other: OtherSettings;
};

export type CalendarEvent = {
  id: string;
  user_id: string;
  title: string;
  description: string;
  location: string | null;
  start_datetime: string;
  end_datetime: string;
  reminders: string[];
  created_at: string;
  updated_at: string;
};

export type CalendarEventInput = {
  title: string;
  description: string;
  location?: string | null;
  start_datetime: string;
  end_datetime: string;
  reminders: string[];
};

export type NotificationRecord = {
  id: string;
  user_id: string;
  type: string;
  title: string;
  message: string;
  payload: Record<string, unknown>;
  is_read: boolean;
  read_at: string | null;
  created_at: string;
};

export type ShareRecord = {
  id: string;
  node_id: string;
  node_name?: string;
  node_type?: "file" | "folder" | string;
  size?: number;
  size_bytes?: number;
  mime_type?: string | null;
  node_updated_at?: string;
  node_deleted?: boolean;
  owner_user_id: string;
  expires_at: string | null;
  max_downloads: number | null;
  download_count: number;
  allow_download: boolean;
  allow_preview: boolean;
  created_at: string;
  updated_at: string;
  revoked_at: string | null;
  needs_password: boolean;
  public_url?: string;
  token?: string;
};

export type PublicShareView = {
  node_id: string;
  node_name: string;
  node_type: "file" | "folder";
  size_bytes: number;
  mime_type: string | null;
  updated_at: string;
  allow_download: boolean;
  expires_at: string | null;
  download_count: number;
  max_downloads: number | null;
  needs_password: boolean;
};

type RequestOptions = {
  method?: "GET" | "POST" | "PUT" | "PATCH" | "DELETE";
  headers?: Record<string, string>;
  body?: unknown;
  rawBody?: BodyInit | null;
};
const REQUEST_TIMEOUT_MS = 15000;
const GET_CACHE_TTL_MS = 1200;

type GetCacheEntry = {
  expiresAt: number;
  value: unknown;
};

const inFlightGetRequests = new Map<string, Promise<unknown>>();
const recentGetCache = new Map<string, GetCacheEntry>();

function buildUrl(path: string): string {
  if (API_BASE_URL === "") {
    return path.startsWith("/") ? path : `/${path}`;
  }
  if (!path.startsWith("/")) {
    return `${API_BASE_URL}/${path}`;
  }
  return `${API_BASE_URL}${path}`;
}

async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const method = options.method ?? "GET";
  const resolvedUrl = buildUrl(path);
  const isGet = method === "GET";

  if (isGet) {
    const now = Date.now();
    const cached = recentGetCache.get(resolvedUrl);
    if (cached && cached.expiresAt > now) {
      return cached.value as T;
    }

    const ongoing = inFlightGetRequests.get(resolvedUrl);
    if (ongoing) {
      return ongoing as Promise<T>;
    }
  }

  const headers: Record<string, string> = {
    ...(options.headers ?? {})
  };

  let body: BodyInit | null | undefined = options.rawBody;
  if (options.body !== undefined) {
    headers["Content-Type"] = "application/json";
    body = JSON.stringify(options.body);
  }

  const doFetch = async (): Promise<T> => {
    let response: Response;
    const controller = new AbortController();
    const timeoutID = globalThis.setTimeout(() => controller.abort(), REQUEST_TIMEOUT_MS);
    try {
      response = await fetch(resolvedUrl, {
        method,
        credentials: "include",
        headers,
        body,
        signal: controller.signal
      });
    } catch (error) {
      const detail = error instanceof Error ? error.message : String(error);
      const isAbort = error instanceof DOMException && error.name === "AbortError";
      throw new ApiError(
        0,
        "network_error",
        isAbort
          ? `API request timed out (${REQUEST_TIMEOUT_MS}ms). Base URL: ${API_BASE_URL}`
          : `API connection failed. Base URL: ${API_BASE_URL}. Check CORS/URL or network access. Details: ${detail}`
      );
    } finally {
      globalThis.clearTimeout(timeoutID);
    }

    const contentType = response.headers.get("Content-Type") ?? "";
    let payload: unknown = null;
    if (contentType.includes("application/json")) {
      payload = await response.json();
    } else if (response.status !== 204) {
      payload = await response.text().catch(() => "");
    }

    if (!response.ok) {
      const errorBody = (payload as ApiErrorBody | null) ?? null;
      const code = errorBody?.error?.code ?? `http_${response.status}`;
      const message = errorBody?.error?.message ?? `Request failed with status ${response.status}`;
      throw new ApiError(response.status, code, message);
    }

    return payload as T;
  };

  if (!isGet) {
    return doFetch();
  }

  const getPromise = doFetch();
  inFlightGetRequests.set(resolvedUrl, getPromise as Promise<unknown>);
  try {
    const result = await getPromise;
    recentGetCache.set(resolvedUrl, {
      value: result,
      expiresAt: Date.now() + GET_CACHE_TTL_MS
    });
    return result;
  } finally {
    inFlightGetRequests.delete(resolvedUrl);
  }
}

export function downloadUrlForNode(nodeID: string): string {
  return buildUrl(`/api/v1/files/nodes/${encodeURIComponent(nodeID)}/download`);
}

export async function login(email: string, password: string): Promise<AuthUser> {
  const result = await request<{ user: AuthUser }>("/api/v1/auth/login", {
    method: "POST",
    body: { email, password }
  });
  return result.user;
}

export async function refreshSession(csrfToken: string): Promise<AuthUser> {
  const result = await request<{ user: AuthUser }>("/api/v1/auth/refresh", {
    method: "POST",
    headers: {
      "X-CSRF-Token": csrfToken
    }
  });
  return result.user;
}

export async function logout(csrfToken: string): Promise<void> {
  await request<unknown>("/api/v1/auth/logout", {
    method: "POST",
    headers: {
      "X-CSRF-Token": csrfToken
    }
  });
}

export async function fetchMe(): Promise<AuthUser> {
  return request<AuthUser>("/api/v1/me");
}

export async function updateMe(input: {
  display_name?: string;
  preferred_language?: string | null;
}): Promise<AuthUser> {
  return request<AuthUser>("/api/v1/me", {
    method: "PATCH",
    body: input
  });
}

export async function updateMyPassword(input: {
  current_password: string;
  new_password: string;
}): Promise<void> {
  await request<unknown>("/api/v1/me/password", {
    method: "PATCH",
    body: input
  });
}

export async function fetchQuota(): Promise<QuotaUsage> {
  return request<QuotaUsage>("/api/v1/quota");
}

export async function listNodes(parentID: string | null): Promise<NodeRecord[]> {
  const query = parentID ? `?parent_id=${encodeURIComponent(parentID)}` : "";
  const result = await request<{ nodes: NodeRecord[] }>(`/api/v1/files/nodes${query}`);
  return result.nodes;
}

export async function searchNodes(input: {
  query: string;
  type?: "file" | "folder" | "all";
  limit?: number;
}): Promise<NodeRecord[]> {
  const params = new URLSearchParams();
  params.set("q", input.query.trim());
  const type = input.type ?? "all";
  if (type === "file" || type === "folder") {
    params.set("type", type);
  }
  params.set("limit", String(input.limit ?? 50));
  const result = await request<{ nodes: NodeRecord[] }>(`/api/v1/files/search?${params.toString()}`);
  return result.nodes;
}

export async function createFolder(name: string, parentID: string | null): Promise<NodeRecord> {
  const result = await request<{ node: NodeRecord }>("/api/v1/files/folders", {
    method: "POST",
    body: {
      name,
      parent_id: parentID
    }
  });
  return result.node;
}

export async function renameNode(nodeID: string, name: string): Promise<NodeRecord> {
  const result = await request<{ node: NodeRecord }>(`/api/v1/files/nodes/${nodeID}/rename`, {
    method: "PATCH",
    body: {
      name
    }
  });
  return result.node;
}

export async function moveNode(nodeID: string, parentID: string | null): Promise<NodeRecord> {
  const result = await request<{ node: NodeRecord }>(`/api/v1/files/nodes/${nodeID}/move`, {
    method: "PATCH",
    body: {
      parent_id: parentID
    }
  });
  return result.node;
}

export async function deleteNode(nodeID: string): Promise<void> {
  await request<unknown>(`/api/v1/files/nodes/${nodeID}`, {
    method: "DELETE"
  });
}

export async function listTrashNodes(): Promise<NodeRecord[]> {
  const result = await request<{ nodes: NodeRecord[] }>("/api/v1/files/trash");
  return result.nodes;
}

export async function restoreTrashNode(nodeID: string): Promise<NodeRecord> {
  const result = await request<{ node: NodeRecord }>(`/api/v1/files/trash/${encodeURIComponent(nodeID)}/restore`, {
    method: "POST"
  });
  return result.node;
}

export async function permanentlyDeleteTrashNode(nodeID: string): Promise<void> {
  await request<unknown>(`/api/v1/files/trash/${encodeURIComponent(nodeID)}`, {
    method: "DELETE"
  });
}

export async function emptyTrash(): Promise<{ deleted_count: number }> {
  return request<{ deleted_count: number }>("/api/v1/files/trash", {
    method: "DELETE"
  });
}

export async function createUploadSession(input: {
  fileName: string;
  expectedSizeBytes: number;
  chunkSizeBytes: number;
  parentID: string | null;
  idempotencyKey: string;
  targetType?: "new_file" | "new_version";
  targetNodeID?: string | null;
}): Promise<UploadSession> {
  const targetType = input.targetType ?? "new_file";
  const result = await request<{ upload_session: UploadSession }>("/api/v1/files/uploads", {
    method: "POST",
    headers: {
      "Idempotency-Key": input.idempotencyKey
    },
    body: {
      target_type: targetType,
      target_node_id: targetType === "new_version" ? input.targetNodeID ?? null : null,
      parent_id: targetType === "new_version" ? null : input.parentID,
      file_name: input.fileName,
      expected_size_bytes: input.expectedSizeBytes,
      chunk_size_bytes: input.chunkSizeBytes
    }
  });
  return result.upload_session;
}

export async function uploadChunk(input: {
  uploadSessionID: string;
  chunkIndex: number;
  chunkBody: Blob;
  chunkHash: string;
}): Promise<UploadSession> {
  const result = await request<{ upload_session: UploadSession }>(
    `/api/v1/files/uploads/${input.uploadSessionID}/chunks/${input.chunkIndex}`,
    {
      method: "PUT",
      headers: {
        "X-Chunk-SHA256": input.chunkHash
      },
      rawBody: input.chunkBody
    }
  );
  return result.upload_session;
}

export async function finalizeUploadSession(uploadSessionID: string): Promise<void> {
  await request<unknown>(`/api/v1/files/uploads/${uploadSessionID}/finalize`, {
    method: "POST"
  });
}

export async function abortUploadSession(uploadSessionID: string): Promise<void> {
  await request<unknown>(`/api/v1/files/uploads/${uploadSessionID}/abort`, {
    method: "POST"
  });
}

export async function listAdminUsers(status?: "all" | "active" | "inactive" | "deleted"): Promise<AdminUser[]> {
  const query = status && status !== "all" ? `?status=${encodeURIComponent(status)}` : "";
  const result = await request<{ users: AdminUser[] }>(`/api/v1/admin/users${query}`);
  return result.users;
}

export async function createAdminUser(input: {
  email: string;
  displayName: string;
  role: "owner" | "admin" | "user";
  password: string;
  quotaBytes: number | null;
  isActive: boolean;
  reactivateExisting?: boolean;
}): Promise<AdminUser> {
  const result = await request<{ user: AdminUser }>("/api/v1/admin/users", {
    method: "POST",
    body: {
      email: input.email,
      display_name: input.displayName,
      role: input.role,
      password: input.password,
      quota_bytes: input.quotaBytes,
      is_active: input.isActive,
      reactivate_existing: input.reactivateExisting ?? false
    }
  });
  return result.user;
}

export async function fetchPublicSettings(): Promise<PublicSettings> {
  return request<PublicSettings>("/api/v1/public/settings");
}

export async function updateAdminUser(input: {
  userID: string;
  displayName?: string;
  role?: "owner" | "admin" | "user";
  quotaBytes?: number;
  useDefaultQuota?: boolean;
  isActive?: boolean;
}): Promise<AdminUser> {
  const body: Record<string, unknown> = {};
  if (input.displayName !== undefined) {
    body.display_name = input.displayName;
  }
  if (input.role !== undefined) {
    body.role = input.role;
  }
  if (input.quotaBytes !== undefined) {
    body.quota_bytes = input.quotaBytes;
  }
  if (input.useDefaultQuota !== undefined) {
    body.use_default_quota = input.useDefaultQuota;
  }
  if (input.isActive !== undefined) {
    body.is_active = input.isActive;
  }

  const result = await request<{ user: AdminUser }>(`/api/v1/admin/users/${input.userID}`, {
    method: "PATCH",
    body
  });
  return result.user;
}

export async function updateAdminUserPassword(userID: string, newPassword: string): Promise<AdminUser> {
  const result = await request<{ user: AdminUser }>(`/api/v1/admin/users/${userID}/password`, {
    method: "POST",
    body: {
      new_password: newPassword
    }
  });
  return result.user;
}

export async function deleteAdminUser(userID: string): Promise<void> {
  await request<unknown>(`/api/v1/admin/users/${userID}`, {
    method: "DELETE"
  });
}

export async function permanentlyDeleteAdminUser(userID: string): Promise<void> {
  await request<unknown>(`/api/v1/admin/users/${userID}?permanent=true`, {
    method: "DELETE"
  });
}

export async function listAdminAudit(input: {
  limit?: number;
  page?: number;
  eventType?: string;
  search?: string;
} = {}): Promise<AdminAuditResponse> {
  const params = new URLSearchParams();
  params.set("limit", String(input.limit ?? 20));
  params.set("page", String(input.page ?? 1));
  if (input.eventType) {
    params.set("event_type", input.eventType);
  }
  if (input.search && input.search.trim() !== "") {
    params.set("search", input.search.trim());
  }

  return request<AdminAuditResponse>(`/api/v1/admin/audit?${params.toString()}`);
}

export async function fetchAdminSettings(): Promise<AdminSettings> {
  const result = await request<{ settings: AdminSettings }>("/api/v1/admin/settings");
  return result.settings;
}

export async function updateAdminSettingsSection<K extends keyof AdminSettings>(
  section: K,
  value: AdminSettings[K]
): Promise<AdminSettings[K]> {
  const result = await request<{ section: K; value: AdminSettings[K] }>(`/api/v1/admin/settings/${section}`, {
    method: "PATCH",
    body: value
  });
  return result.value;
}

export async function listCalendarEvents(input: {
  from?: string;
  to?: string;
} = {}): Promise<CalendarEvent[]> {
  const params = new URLSearchParams();
  if (input.from) {
    params.set("from", input.from);
  }
  if (input.to) {
    params.set("to", input.to);
  }
  const query = params.toString();
  const path = query ? `/api/v1/calendar/events?${query}` : "/api/v1/calendar/events";
  const result = await request<{ events: CalendarEvent[] }>(path);
  return result.events;
}

export async function createCalendarEvent(input: CalendarEventInput): Promise<CalendarEvent> {
  const result = await request<{ event: CalendarEvent }>("/api/v1/calendar/events", {
    method: "POST",
    body: input
  });
  return result.event;
}

export async function updateCalendarEvent(eventID: string, input: CalendarEventInput): Promise<CalendarEvent> {
  const result = await request<{ event: CalendarEvent }>(`/api/v1/calendar/events/${eventID}`, {
    method: "PATCH",
    body: input
  });
  return result.event;
}

export async function deleteCalendarEvent(eventID: string): Promise<void> {
  await request<unknown>(`/api/v1/calendar/events/${eventID}`, {
    method: "DELETE"
  });
}

export async function listNotifications(limit = 50): Promise<NotificationRecord[]> {
  const result = await request<{ notifications: NotificationRecord[] }>(`/api/v1/notifications?limit=${limit}`);
  return result.notifications;
}

export async function markNotificationRead(notificationID: string): Promise<NotificationRecord> {
  const result = await request<{ notification: NotificationRecord }>(`/api/v1/notifications/${notificationID}/read`, {
    method: "PATCH"
  });
  return result.notification;
}

export async function createShare(input: {
  node_id: string;
  password?: string;
  expires_at?: string | null;
  allow_download?: boolean;
  max_downloads?: number | null;
}): Promise<ShareRecord> {
  const result = await request<{ share: ShareRecord }>("/api/v1/shares", {
    method: "POST",
    body: input
  });
  return result.share;
}

export async function listShares(nodeID?: string): Promise<ShareRecord[]> {
  const query = nodeID ? `?node_id=${encodeURIComponent(nodeID)}` : "";
  const result = await request<{ shares: ShareRecord[] }>(`/api/v1/shares${query}`);
  return result.shares;
}

export async function updateShare(
  shareID: string,
  input: {
    password?: string;
    clear_password?: boolean;
    expires_at?: string | null;
    allow_download?: boolean;
    max_downloads?: number | null;
  }
): Promise<ShareRecord> {
  const result = await request<{ share: ShareRecord }>(`/api/v1/shares/${shareID}`, {
    method: "PATCH",
    body: input
  });
  return result.share;
}

export async function revokeShare(shareID: string): Promise<void> {
  await request<unknown>(`/api/v1/shares/${shareID}`, {
    method: "DELETE"
  });
}

export async function fetchPublicShare(token: string, accessGrant?: string): Promise<PublicShareView> {
  const suffix = accessGrant ? `?grant=${encodeURIComponent(accessGrant)}` : "";
  const result = await request<{ share: PublicShareView }>(`/api/public/shares/${token}${suffix}`);
  return result.share;
}

export async function unlockPublicShare(token: string, password: string): Promise<{ unlocked: boolean; access_grant?: string; expires_at?: string }> {
  return request<{ unlocked: boolean; access_grant?: string; expires_at?: string }>(`/api/public/shares/${token}/unlock`, {
    method: "POST",
    body: { password }
  });
}

export function publicShareDownloadURL(token: string, accessGrant?: string): string {
  const suffix = accessGrant ? `?grant=${encodeURIComponent(accessGrant)}` : "";
  return buildUrl(`/api/public/shares/${token}/download${suffix}`);
}
