"use client";

import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from "react";
import { fetchPublicSettings, type PublicSettings } from "@/lib/api";
import en from "@/locales/en.json";
import tr from "@/locales/tr.json";

type Locale = "en" | "tr";

type Dictionary = Record<string, string>;

type AppContextValue = {
  settings: PublicSettings;
  settingsLoading: boolean;
  locale: Locale;
  setLocale: (locale: Locale, persist?: boolean) => void;
  t: (key: string, fallback?: string, params?: Record<string, string | number>) => string;
  refreshSettings: () => Promise<void>;
};

const defaultSettings: PublicSettings = {
  site_name: "BitroxCloud",
  site_subtitle: "Private storage",
  browser_title: "BitroxCloud",
  logo_url: "https://pb.dashboardicons.com/api/files/community_gallery/myyy4r7vdmreido/bitrocloud_ameyfihhth.png",
  favicon_url: "",
  accent_color: "#2563eb",
  public_base_url: "",
  default_language: "en",
  timezone: "Europe/Istanbul",
  maintenance_mode: false
};

const dictionaries: Record<Locale, Dictionary> = {
  en: en as Dictionary,
  tr: tr as Dictionary
};

const userLocaleStorageKey = "bitrox_locale";

const AppContext = createContext<AppContextValue | null>(null);

type Props = {
  children: ReactNode;
};

function parseHexColor(input: string): [number, number, number] | null {
  const value = input.trim();
  if (!/^#[0-9a-fA-F]{6}$/.test(value)) {
    return null;
  }
  const r = Number.parseInt(value.slice(1, 3), 16);
  const g = Number.parseInt(value.slice(3, 5), 16);
  const b = Number.parseInt(value.slice(5, 7), 16);
  return [r, g, b];
}

function mixWithWhite(hex: string, ratio = 0.82): string {
  const rgb = parseHexColor(hex);
  if (!rgb) {
    return "#dbeafe";
  }
  const [r, g, b] = rgb;
  const nr = Math.round(r + (255 - r) * ratio);
  const ng = Math.round(g + (255 - g) * ratio);
  const nb = Math.round(b + (255 - b) * ratio);
  return `rgb(${nr}, ${ng}, ${nb})`;
}

function resolveLocale(raw: string | null | undefined): Locale {
  if (!raw) {
    return "en";
  }
  const normalized = raw.trim().toLowerCase();
  if (normalized === "tr") {
    return "tr";
  }
  return "en";
}

function interpolateTemplate(template: string, params?: Record<string, string | number>): string {
  if (!params) {
    return template;
  }
  return template.replace(/\{([a-zA-Z0-9_]+)\}/g, (full, key: string) => {
    if (!(key in params)) {
      return full;
    }
    const value = params[key];
    return typeof value === "number" ? String(value) : value;
  });
}

function upsertFavicon(url: string) {
  if (typeof document === "undefined") {
    return;
  }
  const head = document.head;
  if (!head) {
    return;
  }
  let link =
    head.querySelector<HTMLLinkElement>('link[rel="icon"][data-runtime-favicon="1"]') ??
    head.querySelector<HTMLLinkElement>('link[rel="icon"]') ??
    head.querySelector<HTMLLinkElement>('link[rel="shortcut icon"]');
  if (!link) {
    link = document.createElement("link");
    link.rel = "icon";
    head.appendChild(link);
  }
  link.rel = "icon";
  link.setAttribute("data-runtime-favicon", "1");
  link.href = url;
}

function resolveDocumentTitle(settings: PublicSettings): string {
  const browserTitle = settings.browser_title?.trim() ?? "";
  if (browserTitle) {
    return browserTitle;
  }
  const siteName = settings.site_name?.trim() ?? "";
  if (siteName) {
    return `${siteName} | Private Cloud`;
  }
  return defaultSettings.browser_title;
}

export function AppContextProvider({ children }: Props) {
  const [hasStoredLocale, setHasStoredLocale] = useState<boolean>(() => {
    if (typeof window === "undefined") {
      return false;
    }
    return !!window.localStorage.getItem(userLocaleStorageKey);
  });
  const [settings, setSettings] = useState<PublicSettings>(defaultSettings);
  const [settingsLoading, setSettingsLoading] = useState(true);
  const [locale, setLocaleState] = useState<Locale>(() => {
    if (typeof window === "undefined") {
      return "en";
    }
    return resolveLocale(window.localStorage.getItem(userLocaleStorageKey));
  });

  const setLocale = useCallback((nextLocale: Locale, persist = true) => {
    setLocaleState(nextLocale);
    if (persist && typeof window !== "undefined") {
      window.localStorage.setItem(userLocaleStorageKey, nextLocale);
      setHasStoredLocale(true);
    }
  }, []);

  const refreshSettings = useCallback(async () => {
    setSettingsLoading(true);
    try {
      const loaded = await fetchPublicSettings();
      setSettings(loaded);
      if (!hasStoredLocale) {
        setLocale(resolveLocale(loaded.default_language), false);
      }
    } catch {
      setSettings(defaultSettings);
    } finally {
      setSettingsLoading(false);
    }
  }, [hasStoredLocale, setLocale]);

  useEffect(() => {
    void refreshSettings();
  }, [refreshSettings]);

  useEffect(() => {
    if (typeof document === "undefined") {
      return;
    }
    const root = document.documentElement;
    root.style.setProperty("--brand", settings.accent_color || defaultSettings.accent_color);
    root.style.setProperty("--brand-soft", mixWithWhite(settings.accent_color || defaultSettings.accent_color));
    root.lang = locale;
    document.title = resolveDocumentTitle(settings);
    if (settings.favicon_url?.trim()) {
      upsertFavicon(settings.favicon_url.trim());
    } else {
      upsertFavicon("/favicon.ico");
    }
  }, [locale, settings]);

  const t = useCallback(
    (key: string, fallback?: string, params?: Record<string, string | number>): string => {
      const localized = dictionaries[locale]?.[key];
      if (localized) {
        return interpolateTemplate(localized, params);
      }
      const english = dictionaries.en[key];
      if (english) {
        return interpolateTemplate(english, params);
      }
      if (fallback) {
        return interpolateTemplate(fallback, params);
      }
      if (process.env.NODE_ENV !== "production") {
        return `[${key}]`;
      }
      return key
        .split(".")
        .at(-1)
        ?.replace(/[_-]+/g, " ")
        ?.replace(/\b\w/g, (char) => char.toUpperCase()) ?? key;
    },
    [locale]
  );

  const contextValue = useMemo<AppContextValue>(
    () => ({
      settings,
      settingsLoading,
      locale,
      setLocale,
      t,
      refreshSettings
    }),
    [locale, refreshSettings, setLocale, settings, settingsLoading, t]
  );

  return <AppContext.Provider value={contextValue}>{children}</AppContext.Provider>;
}

export function useAppContext() {
  const ctx = useContext(AppContext);
  if (!ctx) {
    throw new Error("useAppContext must be used within AppContextProvider");
  }
  return ctx;
}
