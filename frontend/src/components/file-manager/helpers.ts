import { ApiError } from "@/lib/api";

export type FlashTone = "success" | "error" | "info";

export type Flash = {
  tone: FlashTone;
  message: string;
};

export type UploadJob = {
  id: string;
  fileName: string;
  progress: number;
  status: "running" | "done" | "error" | "cancelled";
  detail: string;
  sourceFile?: File;
};

export type SortKey = "name" | "type" | "size" | "updated";
export type SortDirection = "asc" | "desc";

export function cacheKeyForParent(parentID: string | null): string {
  return parentID ?? "__root__";
}

export function formatBytes(value: number): string {
  if (!Number.isFinite(value) || value < 0) {
    return "0 B";
  }
  if (value < 1024) {
    return `${value} B`;
  }
  const units = ["KB", "MB", "GB", "TB"];
  let current = value;
  let idx = -1;
  while (current >= 1024 && idx < units.length - 1) {
    current /= 1024;
    idx += 1;
  }
  return `${current.toFixed(current >= 10 ? 1 : 2)} ${units[idx]}`;
}

export function formatDateTime(iso: string): string {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) {
    return "-";
  }
  const runtimeLocale =
    typeof document !== "undefined" && document.documentElement.lang
      ? document.documentElement.lang
      : typeof navigator !== "undefined"
      ? navigator.language
      : "en";
  return new Intl.DateTimeFormat(runtimeLocale, {
    dateStyle: "medium",
    timeStyle: "short"
  }).format(date);
}

export function normalizeErrorMessage(error: unknown): string {
  if (error instanceof ApiError) {
    if (error.status === 401) {
      return "Oturum gecersiz veya suresi dolmus. Lutfen tekrar giris yapin.";
    }
    return error.message;
  }
  if (error instanceof Error) {
    return error.message;
  }
  return "Beklenmeyen hata";
}
