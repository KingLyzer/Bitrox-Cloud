import { sha256 } from "@noble/hashes/sha256";
import { bytesToHex } from "@noble/hashes/utils";

const csrfCookieNames = ["__Host-cloud_csrf_token", "cloud_csrf_token"] as const;

export function readCSRFTokenFromCookie(): string | null {
  if (typeof document === "undefined") {
    return null;
  }

  const allCookies = document.cookie.split(";").map((entry) => entry.trim());
  for (const cookieName of csrfCookieNames) {
    const prefix = `${cookieName}=`;
    const found = allCookies.find((entry) => entry.startsWith(prefix));
    if (!found) {
      continue;
    }

    const raw = found.slice(prefix.length);
    if (raw === "") {
      continue;
    }
    return decodeURIComponent(raw);
  }

  return null;
}

export async function sha256Hex(blob: Blob): Promise<string> {
  const arrayBuffer = await blob.arrayBuffer();
  const bytes = new Uint8Array(arrayBuffer);

  // Prefer native WebCrypto when available, but support HTTP/non-secure contexts too.
  const subtle = typeof globalThis.crypto !== "undefined" ? globalThis.crypto.subtle : undefined;
  if (subtle && typeof subtle.digest === "function") {
    const hashBuffer = await subtle.digest("SHA-256", arrayBuffer);
    const hashBytes = new Uint8Array(hashBuffer);
    return Array.from(hashBytes)
      .map((byte) => byte.toString(16).padStart(2, "0"))
      .join("");
  }

  return bytesToHex(sha256(bytes));
}

function randomBytes(size: number): Uint8Array {
  const bytes = new Uint8Array(size);
  if (typeof globalThis.crypto !== "undefined" && typeof globalThis.crypto.getRandomValues === "function") {
    globalThis.crypto.getRandomValues(bytes);
    return bytes;
  }

  for (let i = 0; i < bytes.length; i += 1) {
    bytes[i] = Math.floor(Math.random() * 256);
  }
  return bytes;
}

export function generateClientID(): string {
  if (typeof globalThis.crypto !== "undefined" && typeof globalThis.crypto.randomUUID === "function") {
    return globalThis.crypto.randomUUID();
  }

  // RFC4122-ish v4 fallback for older/non-secure contexts.
  const bytes = randomBytes(16);
  bytes[6] = (bytes[6] & 0x0f) | 0x40;
  bytes[8] = (bytes[8] & 0x3f) | 0x80;
  const hex = Array.from(bytes).map((b) => b.toString(16).padStart(2, "0")).join("");
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20, 32)}`;
}
