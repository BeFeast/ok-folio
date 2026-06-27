// FolioContext centralizes the shared state for every OK Folio surface —
// theme, search, gallery mode, the piece viewer, favorites, and the
// Add-Piece modal — mirroring the single stateful component in the canonical
// design but wired to the real catalog/stats API.

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import {
  addToFavorites,
  fetchGalleryCatalog,
  fetchStats,
  getPhotoImageUrl,
  getPhotoThumbnailUrl,
  removeFromFavorites,
} from "../api";
import type { Photo } from "../types";
import {
  applyTheme,
  readStoredTheme,
  storeTheme,
  type ThemeName,
} from "./theme";

export type GalleryMode = "magazine" | "library" | "wall";

export interface PieceVM {
  id: number;
  t: string; // title
  a: string; // artist
  y: string; // date label (empty when unknown)
  src: string; // source label
  med: string; // medium (empty when unknown)
  kind: string; // eyebrow kind label
  note: string; // personal note (empty when none)
  folio: string; // suggested/assigned folio (empty when none)
  img: string;
  thumb: string;
  fav: boolean;
  file: string;
  size: string;
  dim: string; // dimensions (empty when unknown)
  added: string;
}

const PAGE_SIZE = 120;

/* ---- mapping helpers ---- */

function prettifyFileName(fn: string): string {
  if (!fn) return "Untitled piece";
  const base = fn.replace(/\.[a-z0-9]{2,5}$/i, "");
  const cleaned = base
    .replace(/[_]+/g, " ")
    .replace(/\s*-\s*/g, " ")
    .replace(/\s{2,}/g, " ")
    .trim();
  return cleaned || "Untitled piece";
}

function yearFrom(value: string): string {
  if (!value) return "";
  const parsed = new Date(value);
  if (!Number.isNaN(parsed.getTime())) {
    const y = parsed.getFullYear();
    if (y > 1000 && y < 3000) return String(y);
  }
  const m = value.match(/\b(1[0-9]{3}|20[0-9]{2})\b/);
  return m ? m[1] : "";
}

function hostFrom(value: string): string {
  if (!value) return "";
  try {
    return new URL(value).hostname.replace(/^www\./, "");
  } catch {
    return value;
  }
}

export function formatBytes(n: number): string {
  if (!n || n <= 0) return "—";
  const units = ["B", "KB", "MB", "GB", "TB"];
  let v = n;
  let i = 0;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i++;
  }
  return `${v >= 10 || i === 0 ? Math.round(v) : v.toFixed(1)} ${units[i]}`;
}

function relativeAdded(value: string): string {
  if (!value) return "—";
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return "—";
  const now = new Date();
  const days = Math.floor((now.getTime() - d.getTime()) / 86_400_000);
  if (days <= 0) return "Today";
  if (days === 1) return "Yesterday";
  if (days < 7) return "This week";
  if (days < 14) return "Last week";
  if (days < 60) return `${Math.floor(days / 7)} weeks ago`;
  return d.toLocaleDateString(undefined, { month: "short", year: "numeric" });
}

function mapPhoto(p: Photo): PieceVM {
  const title = (p.Title || "").trim() || prettifyFileName(p.FileName);
  const artist = (p.Artist || "").trim();
  return {
    id: p.ID,
    t: title,
    a: artist || "Unknown",
    y: yearFrom(p.UploadDate) || yearFrom(p.DownloadedAt),
    src: hostFrom(p.SourcePage) || p.SourcePage || "—",
    med: "",
    kind: "",
    note: "",
    folio: "",
    img: getPhotoImageUrl(p.ID),
    thumb: getPhotoThumbnailUrl(p.ID, 400),
    fav: !!p.Favorite,
    file: p.FileName || "—",
    size: formatBytes(p.FileSize),
    dim: "",
    added: relativeAdded(p.DownloadedAt),
  };
}

/* ---- context ---- */

interface FolioContextValue {
  theme: ThemeName;
  setTheme: (t: ThemeName) => void;
  toggleTheme: () => void;

  query: string;
  setQuery: (q: string) => void;

  mode: GalleryMode;
  setMode: (m: GalleryMode) => void;

  pieces: PieceVM[];
  total: number;
  isLoading: boolean;
  isError: boolean;
  loadMore: () => void;
  hasMore: boolean;
  loadingMore: boolean;

  totalPhotos: number;
  totalSizeBytes: number;

  selected: PieceVM | null;
  selIndex: number;
  selCount: number;
  openPiece: (id: number) => void;
  closePiece: () => void;
  stepPiece: (dir: number) => void;

  addOpen: boolean;
  openAdd: () => void;
  closeAdd: () => void;

  isFav: (id: number) => boolean;
  toggleFav: (id: number) => void;
}

const FolioContext = createContext<FolioContextValue | null>(null);

export function useFolio(): FolioContextValue {
  const ctx = useContext(FolioContext);
  if (!ctx) throw new Error("useFolio must be used within FolioProvider");
  return ctx;
}

export function FolioProvider({ children }: { children: ReactNode }) {
  const [theme, setThemeState] = useState<ThemeName>(() => readStoredTheme());
  const [query, setQuery] = useState("");
  const [debouncedQuery, setDebouncedQuery] = useState("");
  const [mode, setMode] = useState<GalleryMode>("library");
  const [selectedId, setSelectedId] = useState<number | null>(null);
  const [addOpen, setAddOpen] = useState(false);
  const [favOverride, setFavOverride] = useState<Record<number, boolean>>({});

  // Theme: keep the document tokens in sync with state.
  useEffect(() => {
    applyTheme(theme);
  }, [theme]);

  const setTheme = useCallback((t: ThemeName) => {
    storeTheme(t);
    setThemeState(t);
  }, []);
  const toggleTheme = useCallback(() => {
    setThemeState((prev) => {
      const next: ThemeName = prev === "dark" ? "light" : "dark";
      storeTheme(next);
      return next;
    });
  }, []);

  // Debounce the search query before it hits the catalog endpoint.
  useEffect(() => {
    const id = setTimeout(() => setDebouncedQuery(query.trim()), 250);
    return () => clearTimeout(id);
  }, [query]);

  const catalog = useInfiniteQuery({
    queryKey: ["folio-catalog", debouncedQuery],
    queryFn: ({ pageParam }) =>
      fetchGalleryCatalog(
        PAGE_SIZE,
        pageParam as number,
        debouncedQuery ? { query: debouncedQuery } : {},
      ),
    initialPageParam: 0,
    getNextPageParam: (lastPage, allPages) => {
      const loaded = allPages.reduce((n, pg) => n + pg.photos.length, 0);
      return loaded < lastPage.total ? loaded : undefined;
    },
  });

  const stats = useQuery({ queryKey: ["folio-stats"], queryFn: fetchStats });

  const catalogTotal = catalog.data?.pages[0]?.total;

  const pieces = useMemo<PieceVM[]>(() => {
    const photos = catalog.data?.pages.flatMap((pg) => pg.photos) ?? [];
    return photos.map((p) => {
      const vm = mapPhoto(p);
      const override = favOverride[vm.id];
      return override === undefined ? vm : { ...vm, fav: override };
    });
  }, [catalog.data, favOverride]);

  const isFav = useCallback(
    (id: number) => {
      const override = favOverride[id];
      if (override !== undefined) return override;
      return pieces.find((p) => p.id === id)?.fav ?? false;
    },
    [favOverride, pieces],
  );

  const inFlight = useRef<Set<number>>(new Set());
  const toggleFav = useCallback(
    (id: number) => {
      const next = !isFav(id);
      setFavOverride((prev) => ({ ...prev, [id]: next }));
      if (inFlight.current.has(id)) return;
      inFlight.current.add(id);
      const call = next ? addToFavorites(id) : removeFromFavorites(id);
      call
        .catch(() => {
          // Revert optimistic state on failure (e.g. gallery backend offline).
          setFavOverride((prev) => ({ ...prev, [id]: !next }));
        })
        .finally(() => {
          inFlight.current.delete(id);
        });
    },
    [isFav],
  );

  const openPiece = useCallback((id: number) => setSelectedId(id), []);
  const closePiece = useCallback(() => setSelectedId(null), []);
  const stepPiece = useCallback(
    (dir: number) => {
      setSelectedId((cur) => {
        if (cur == null || pieces.length === 0) return cur;
        const i = pieces.findIndex((p) => p.id === cur);
        if (i < 0) return cur;
        const n = (i + dir + pieces.length) % pieces.length;
        return pieces[n].id;
      });
    },
    [pieces],
  );

  const selected = useMemo(
    () => (selectedId == null ? null : pieces.find((p) => p.id === selectedId) ?? null),
    [selectedId, pieces],
  );
  const selIndex = useMemo(
    () => (selectedId == null ? -1 : pieces.findIndex((p) => p.id === selectedId)),
    [selectedId, pieces],
  );

  const value: FolioContextValue = {
    theme,
    setTheme,
    toggleTheme,
    query,
    setQuery,
    mode,
    setMode,
    pieces,
    total: catalogTotal ?? pieces.length,
    isLoading: catalog.isLoading,
    isError: catalog.isError,
    loadMore: () => {
      if (catalog.hasNextPage && !catalog.isFetchingNextPage) {
        void catalog.fetchNextPage();
      }
    },
    hasMore: !!catalog.hasNextPage,
    loadingMore: catalog.isFetchingNextPage,
    totalPhotos: stats.data?.total_photos ?? catalogTotal ?? 0,
    totalSizeBytes: stats.data?.total_size_bytes ?? 0,
    selected,
    selIndex,
    selCount: pieces.length,
    openPiece,
    closePiece,
    stepPiece,
    addOpen,
    openAdd: useCallback(() => setAddOpen(true), []),
    closeAdd: useCallback(() => setAddOpen(false), []),
    isFav,
    toggleFav,
  };

  return <FolioContext.Provider value={value}>{children}</FolioContext.Provider>;
}
