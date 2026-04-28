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
    <main className="relative min-h-screen overflow-hidden bg-[var(--bg-main)] px-4 py-10" data-testid="auth-view">
      <div className="pointer-events-none absolute inset-0 bg-[radial-gradient(circle_at_10%_20%,rgba(255,255,255,0.35),transparent_38%),radial-gradient(circle_at_85%_12%,rgba(59,130,246,0.28),transparent_38%),linear-gradient(135deg,#f8fbff_0%,#dce9ff_45%,#c7dcff_100%)] dark:bg-[radial-gradient(circle_at_10%_20%,rgba(255,255,255,0.06),transparent_38%),radial-gradient(circle_at_85%_12%,rgba(37,99,235,0.24),transparent_38%),linear-gradient(135deg,#0b1428_0%,#111f3f_45%,#152954_100%)]" />
      <div className="pointer-events-none absolute inset-0 opacity-40 [background-image:linear-gradient(rgba(255,255,255,.18)_1px,transparent_1px),linear-gradient(90deg,rgba(255,255,255,.18)_1px,transparent_1px)] [background-size:32px_32px]" />

      <div className="relative mx-auto flex min-h-[70vh] w-full max-w-md items-center">
        <section className="w-full rounded-3xl border border-white/35 bg-white/65 p-7 shadow-2xl backdrop-blur-xl dark:border-white/15 dark:bg-slate-950/55 md:p-8">
          <div className="mb-6 text-center">
            <p className="text-xs font-semibold uppercase tracking-[0.24em] text-[var(--text-muted)]">{settings.site_name || "BitroxCloud"}</p>
            <h2 className="mt-2 text-3xl font-bold text-[var(--text-main)]" style={{ fontFamily: '"Sora","Segoe UI",sans-serif' }}>
              {t("auth.title", "Sign in")}
            </h2>
          </div>

          <form onSubmit={onSubmit} className="space-y-4" data-testid="auth-form">
            <label className="block">
              <span className="mb-1.5 block text-sm font-medium text-[var(--text-main)]">{t("auth.email", "Email")}</span>
              <div className="relative">
                <i className="fa-regular fa-envelope pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-[var(--text-muted)]" />
                <input
                  required
                  type="email"
                  value={email}
                  onChange={(event) => onEmailChange(event.target.value)}
                  data-testid="login-email-input"
                  className="focus-ring w-full rounded-xl border border-[var(--line)] bg-white/80 py-2.5 pl-10 pr-3 text-sm text-[var(--text-main)] outline-none transition dark:bg-[var(--bg-soft)]"
                  placeholder="admin@example.com"
                />
              </div>
            </label>

            <label className="block">
              <span className="mb-1.5 block text-sm font-medium text-[var(--text-main)]">{t("auth.password", "Password")}</span>
              <div className="relative">
                <i className="fa-solid fa-lock pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-[var(--text-muted)]" />
                <input
                  required
                  type="password"
                  value={password}
                  onChange={(event) => onPasswordChange(event.target.value)}
                  data-testid="login-password-input"
                  className="focus-ring w-full rounded-xl border border-[var(--line)] bg-white/80 py-2.5 pl-10 pr-3 text-sm text-[var(--text-main)] outline-none transition dark:bg-[var(--bg-soft)]"
                  placeholder="********"
                />
              </div>
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
        </section>
      </div>
    </main>
  );
}
