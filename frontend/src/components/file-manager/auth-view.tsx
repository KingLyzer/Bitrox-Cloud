"use client";

import type { FormEvent } from "react";
import { useAppContext } from "@/components/app-context-provider";

type Props = {
  email: string;
  password: string;
  loading: boolean;
  onEmailChange: (value: string) => void;
  onPasswordChange: (value: string) => void;
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
};

export function AuthView({
  email,
  password,
  loading,
  onEmailChange,
  onPasswordChange,
  onSubmit
}: Props) {
  const { settings, t } = useAppContext();

  return (
    <main className="min-h-screen bg-[var(--bg-main)] px-4 py-10" data-testid="auth-view">
      <div className="mx-auto w-full max-w-md rounded-2xl border border-[var(--line)] bg-[var(--bg-card)] p-7 shadow-xl">
        <div className="mb-7">
          <p className="text-xs font-semibold uppercase tracking-[0.2em] text-[var(--text-muted)]">{settings.site_name || "BitroxCloud"}</p>
          <h1 className="mt-2 text-2xl font-bold text-[var(--text-main)]">{t("auth.title", "Sign in")}</h1>
          <p className="mt-1 text-sm text-[var(--text-muted)]">{t("auth.subtitle", "Sign in to access your cloud workspace.")}</p>
        </div>

        <form onSubmit={onSubmit} className="space-y-4" data-testid="auth-form">
          <label className="block">
            <span className="mb-1.5 block text-sm font-medium text-[var(--text-main)]">{t("auth.email", "Email")}</span>
            <input
              required
              type="email"
              value={email}
              onChange={(event) => onEmailChange(event.target.value)}
              data-testid="login-email-input"
              className="focus-ring w-full rounded-xl border border-[var(--line)] bg-[var(--bg-soft)] px-3 py-2.5 text-sm text-[var(--text-main)] outline-none transition"
              placeholder="admin@example.com"
            />
          </label>

          <label className="block">
            <span className="mb-1.5 block text-sm font-medium text-[var(--text-main)]">{t("auth.password", "Password")}</span>
            <input
              required
              type="password"
              value={password}
              onChange={(event) => onPasswordChange(event.target.value)}
              data-testid="login-password-input"
              className="focus-ring w-full rounded-xl border border-[var(--line)] bg-[var(--bg-soft)] px-3 py-2.5 text-sm text-[var(--text-main)] outline-none transition"
              placeholder="********"
            />
          </label>

          <button
            type="submit"
            disabled={loading}
            data-testid="login-submit-button"
            className="focus-ring inline-flex w-full items-center justify-center rounded-xl bg-[var(--brand)] px-4 py-2.5 text-sm font-semibold text-white transition hover:brightness-110 disabled:cursor-not-allowed disabled:opacity-60"
          >
            {loading ? t("auth.signingIn", "Signing in...") : t("auth.signIn", "Sign in")}
          </button>
        </form>
      </div>
    </main>
  );
}
