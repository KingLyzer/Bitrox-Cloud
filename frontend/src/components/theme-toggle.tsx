"use client";

import { useTheme } from "next-themes";
import { useEffect, useState } from "react";

export function ThemeToggle() {
  const { theme, setTheme, resolvedTheme } = useTheme();
  const [mounted, setMounted] = useState(false);

  useEffect(() => {
    setMounted(true);
  }, []);

  if (!mounted) {
    return (
      <div className="h-10 w-[76px] rounded-full border border-black/10 dark:border-white/15" />
    );
  }

  const effectiveTheme = theme === "system" ? resolvedTheme : theme;
  const isDark = effectiveTheme === "dark";
  const nextTheme = isDark ? "light" : "dark";

  return (
    <button
      onClick={() => setTheme(nextTheme)}
      className="focus-ring relative h-10 w-[76px] rounded-full border border-black/10 bg-white/60 transition hover:bg-black/5 dark:border-white/15 dark:bg-[#0f172a] dark:hover:bg-white/10"
      aria-label="Toggle theme"
      type="button"
    >
      <span className="absolute inset-0 flex items-center justify-between px-3 text-sm text-[var(--text-muted)]">
        <i className="fa-solid fa-gear" />
        <i className="fa-solid fa-moon" />
      </span>
      <span
        className={`absolute top-1 grid h-8 w-8 place-items-center rounded-full bg-[var(--brand)] text-white shadow transition-all ${
          isDark ? "left-[40px]" : "left-1"
        }`}
      >
        <i className={`fa-solid ${isDark ? "fa-moon" : "fa-gear"} text-xs`} />
      </span>
    </button>
  );
}
