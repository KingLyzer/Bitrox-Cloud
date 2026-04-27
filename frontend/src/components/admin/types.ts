export type AdminRole = "owner" | "admin" | "user";

export type AdminUserFormValues = {
  email: string;
  displayName: string;
  role: AdminRole;
  password: string;
  useDefaultQuota: boolean;
  quotaGb: string;
  isActive: boolean;
};

export function quotaGbToBytes(raw: string): number | null {
  const parsed = Number(raw.trim());
  if (!Number.isFinite(parsed) || parsed <= 0) {
    return null;
  }
  return Math.round(parsed * 1024 * 1024 * 1024);
}

export function bytesToQuotaGb(bytes: number | null | undefined): string {
  if (!bytes || bytes <= 0) {
    return "";
  }
  return (bytes / 1024 / 1024 / 1024).toFixed(2);
}

