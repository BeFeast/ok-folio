// OK Folio theme tokens — ported verbatim from the canonical Claude Design
// ("OK Folio - Product.dc.html"). The light/dark token tables and the
// document-level applier mirror the design's __okfolioApplyTheme so the
// running app and the design system stay pixel-identical.

export type ThemeName = "light" | "dark" | "auto";
export type ResolvedThemeName = "light" | "dark";
export type InfoPanelMode = "pinned" | "remember" | "hidden";

export const SANS =
  "'Instrument Sans', system-ui, -apple-system, 'Segoe UI', sans-serif";
export const SERIF = "'Newsreader', Georgia, 'Times New Roman', serif";

export const TOKENS: Record<ResolvedThemeName, Record<string, string>> = {
  light: {
    "--bg": "#F3EFE7",
    "--surface": "#FBF9F3",
    "--surface-2": "#FFFFFF",
    "--wall": "#E7E1D5",
    "--ink": "#1C1A16",
    "--graphite": "#57534B",
    "--muted": "#8B847A",
    "--faint": "#B6AE9F",
    "--line": "#E6DFD2",
    "--line-2": "#D8CFBE",
    "--accent": "#7C2420",
    "--accent-soft": "rgba(124,36,32,0.08)",
    "--accent-line": "rgba(124,36,32,0.34)",
    "--on-accent": "#FBF6EE",
    "--danger": "#C0392B",
    "--on-danger": "#FFFFFF",
    "--shadow": "rgba(48,40,28,0.10)",
    "--shadow-2": "rgba(48,40,28,0.20)",
    "--serif": SERIF,
    "--sans": SANS,
  },
  dark: {
    "--bg": "#141210",
    "--surface": "#1C1915",
    "--surface-2": "#232019",
    "--wall": "#0C0B09",
    "--ink": "#F0EBE0",
    "--graphite": "#ABA495",
    "--muted": "#7E7768",
    "--faint": "#5C5648",
    "--line": "#2B2720",
    "--line-2": "#3A352B",
    "--accent": "#C75D49",
    "--accent-soft": "rgba(199,93,73,0.15)",
    "--accent-line": "rgba(199,93,73,0.5)",
    "--on-accent": "#15110D",
    "--danger": "#E07060",
    "--on-danger": "#FFFFFF",
    "--shadow": "rgba(0,0,0,0.5)",
    "--shadow-2": "rgba(0,0,0,0.66)",
    "--serif": SERIF,
    "--sans": SANS,
  },
};

const STORAGE_KEY = "okfolio-theme";
const INFO_PANEL_MODE_STORAGE_KEY = "okfolio-info-panel-mode";
const INFO_PANEL_OPEN_STORAGE_KEY = "okfolio-info-panel-open";

export function resolveTheme(name: ThemeName): ResolvedThemeName {
  if (name !== "auto") return name;
  if (typeof window !== "undefined" && typeof window.matchMedia === "function") {
    return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
  }
  return "light";
}

export function applyTheme(name: ThemeName): void {
  const resolved = resolveTheme(name);
  const t = TOKENS[resolved] || TOKENS.light;
  const root = document.documentElement;
  for (const k in t) root.style.setProperty(k, t[k]);
  root.dataset.theme = name;
  root.dataset.resolvedTheme = resolved;
  root.style.colorScheme = resolved === "dark" ? "dark" : "light";
  if (document.body) document.body.style.background = t["--bg"];
}

export function readStoredTheme(): ThemeName {
  try {
    const s = localStorage.getItem(STORAGE_KEY);
    if (s === "dark" || s === "light" || s === "auto") return s;
  } catch {
    /* ignore */
  }
  return "light";
}

export function storeTheme(name: ThemeName): void {
  try {
    localStorage.setItem(STORAGE_KEY, name);
  } catch {
    /* ignore */
  }
}

export function readStoredInfoPanelMode(): InfoPanelMode {
  try {
    const s = localStorage.getItem(INFO_PANEL_MODE_STORAGE_KEY);
    if (s === "pinned" || s === "remember" || s === "hidden") return s;
  } catch {
    /* ignore */
  }
  return "remember";
}

export function storeInfoPanelMode(mode: InfoPanelMode): void {
  try {
    localStorage.setItem(INFO_PANEL_MODE_STORAGE_KEY, mode);
  } catch {
    /* ignore */
  }
}

export function readStoredInfoPanelOpen(): boolean {
  try {
    return localStorage.getItem(INFO_PANEL_OPEN_STORAGE_KEY) === "true";
  } catch {
    /* ignore */
  }
  return false;
}

export function storeInfoPanelOpen(open: boolean): void {
  try {
    localStorage.setItem(INFO_PANEL_OPEN_STORAGE_KEY, open ? "true" : "false");
  } catch {
    /* ignore */
  }
}
