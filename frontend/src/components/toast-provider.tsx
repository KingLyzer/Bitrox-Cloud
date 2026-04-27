"use client";

import { createContext, useCallback, useContext, useMemo, useState, type ReactNode } from "react";

export type ToastTone = "success" | "error" | "info" | "warning";

type ToastItem = {
  id: string;
  tone: ToastTone;
  message: string;
};

type ToastContextValue = {
  showToast: (tone: ToastTone, message: string, timeoutMs?: number) => void;
};

const ToastContext = createContext<ToastContextValue | null>(null);

type Props = {
  children: ReactNode;
};

function toneStyles(tone: ToastTone): string {
  if (tone === "success") {
    return "border-emerald-500/40 bg-emerald-500/15 text-emerald-600 dark:text-emerald-300";
  }
  if (tone === "error") {
    return "border-red-500/40 bg-red-500/15 text-red-600 dark:text-red-300";
  }
  if (tone === "warning") {
    return "border-amber-500/40 bg-amber-500/15 text-amber-700 dark:text-amber-300";
  }
  return "border-blue-500/40 bg-blue-500/15 text-blue-700 dark:text-blue-300";
}

export function ToastProvider({ children }: Props) {
  const [toasts, setToasts] = useState<ToastItem[]>([]);

  const dismissToast = useCallback((id: string) => {
    setToasts((current) => current.filter((item) => item.id !== id));
  }, []);

  const showToast = useCallback(
    (tone: ToastTone, message: string, timeoutMs = 4200) => {
      const id = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
      setToasts((current) => [{ id, tone, message }, ...current].slice(0, 6));
      window.setTimeout(() => dismissToast(id), timeoutMs);
    },
    [dismissToast]
  );

  const contextValue = useMemo<ToastContextValue>(
    () => ({
      showToast
    }),
    [showToast]
  );

  return (
    <ToastContext.Provider value={contextValue}>
      {children}
      <div className="pointer-events-none fixed bottom-4 right-4 z-[120] flex w-[min(360px,calc(100vw-1.5rem))] flex-col gap-2" aria-live="polite" aria-atomic="false">
        {toasts.map((toast) => (
          <div
            key={toast.id}
            className={`pointer-events-auto rounded-xl border px-3 py-2 text-sm shadow-2xl backdrop-blur ${toneStyles(toast.tone)}`}
            data-testid="global-toast"
            data-tone={toast.tone}
          >
            <div className="flex items-start gap-2">
              <p className="flex-1 leading-5">{toast.message}</p>
              <button
                type="button"
                onClick={() => dismissToast(toast.id)}
                className="focus-ring rounded px-1 text-xs opacity-80 hover:opacity-100"
                aria-label="Close notification"
              >
                <i className="fa-solid fa-xmark" />
              </button>
            </div>
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  );
}

export function useToast() {
  const ctx = useContext(ToastContext);
  if (!ctx) {
    throw new Error("useToast must be used within ToastProvider");
  }
  return ctx;
}
