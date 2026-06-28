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
import { useInfiniteQuery, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import {
  addPieceToFolio,
  addToFavorites,
  createFolio,
  createPiece,
  deleteFolio,
  dismissInboxItem,
  fetchGalleryCatalog,
  fetchInboxCounts,
  fetchStats,
  getPhotoImageUrl,
  getPhotoThumbnailUrl,
  removeFromFavorites,
  removePieceFromFolio,
  updateFolio,
  type CreatePieceInput,
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

export function mapPhoto(p: Photo): PieceVM {
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
    note: (p.Notes || "").trim(),
    folio: "",
    img: getPhotoImageUrl(p.ID),
    thumb: getPhotoThumbnailUrl(p.ID, 400),
    fav: !!p.Favorite,
    file: p.FileName || "—",
    size: formatBytes(p.FileSize),
    dim: p.ImageWidth && p.ImageHeight ? `${p.ImageWidth} x ${p.ImageHeight}` : "",
    added: relativeAdded(p.DownloadedAt),
  };
}

/* ---- context ---- */

// A transient background-task notification (e.g. an Add-Piece import that runs
// after the modal has already closed). Surfaced by the Toaster.
export type ToastStatus = "loading" | "success" | "error";
export interface Toast {
  id: number;
  status: ToastStatus;
  title: string;
  detail?: string;
}
let toastSeq = 0;

interface FolioContextValue {
  theme: ThemeName;
  setTheme: (t: ThemeName) => void;
  toggleTheme: () => void;

  query: string;
  setQuery: (q: string) => void;
  favoriteOnly: boolean;
  setFavoriteOnly: (enabled: boolean) => void;
  artist: string;
  setArtist: (artist: string) => void;
  filterByArtist: (artist: string) => void;

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
  inboxCount: number;

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

  importPiece: (input: CreatePieceInput) => void;
  dismissInboxAction: (id: number) => void;
  createFolioAction: (name: string) => void;
  renameFolioAction: (id: number, name: string) => void;
  deleteFolioAction: (id: number) => void;
  addPieceToFolioAction: (folioId: number, photoId: number) => void;
  removePieceFromFolioAction: (folioId: number, photoId: number) => void;
  setViewerPieces: (pieces: PieceVM[]) => void;
  toasts: Toast[];
  dismissToast: (id: number) => void;
}

const FolioContext = createContext<FolioContextValue | null>(null);

export function useFolio(): FolioContextValue {
  const ctx = useContext(FolioContext);
  if (!ctx) throw new Error("useFolio must be used within FolioProvider");
  return ctx;
}

export function FolioProvider({ children }: { children: ReactNode }) {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [theme, setThemeState] = useState<ThemeName>(() => readStoredTheme());
  const [query, setQuery] = useState("");
  const [debouncedQuery, setDebouncedQuery] = useState("");
  const [favoriteOnly, setFavoriteOnly] = useState(false);
  const [artist, setArtist] = useState("");
  const [mode, setMode] = useState<GalleryMode>("library");
  const [selectedId, setSelectedId] = useState<number | null>(null);
  const [addOpen, setAddOpen] = useState(false);
  const [favOverride, setFavOverride] = useState<Record<number, boolean>>({});
  const [toasts, setToasts] = useState<Toast[]>([]);
  const [viewerPieces, setViewerPiecesState] = useState<PieceVM[]>([]);

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
    queryKey: ["folio-catalog", debouncedQuery, favoriteOnly, artist],
    queryFn: ({ pageParam }) => {
      const filters: Parameters<typeof fetchGalleryCatalog>[2] = {};
      if (debouncedQuery) filters.query = debouncedQuery;
      if (favoriteOnly) filters.favorite = true;
      if (artist) filters.artist = artist;
      return fetchGalleryCatalog(PAGE_SIZE, pageParam as number, filters);
    },
    initialPageParam: 0,
    getNextPageParam: (lastPage, allPages) => {
      const loaded = allPages.reduce((n, pg) => n + pg.photos.length, 0);
      return loaded < lastPage.total ? loaded : undefined;
    },
  });

  const stats = useQuery({ queryKey: ["folio-stats"], queryFn: fetchStats });
  const inboxCounts = useQuery({ queryKey: ["inbox-counts"], queryFn: fetchInboxCounts });

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
          void queryClient.invalidateQueries({ queryKey: ["folio-catalog"] });
        });
    },
    [isFav, queryClient],
  );

  const dismissToast = useCallback((id: number) => {
    setToasts((prev) => prev.filter((t) => t.id !== id));
  }, []);

  // Fire-and-forget upload: the Add-Piece modal closes immediately and the
  // import runs here (outside the modal's lifecycle), surfacing progress as a
  // toast — a loading spinner that resolves to success/duplicate (auto-dismiss)
  // or an error (stays until clicked).
  const importPiece = useCallback(
    (input: CreatePieceInput) => {
      const id = ++toastSeq;
      const label = input.title.trim() || input.file.name;
      setToasts((prev) => [...prev, { id, status: "loading", title: "Adding piece", detail: label }]);
      createPiece(input)
        .then((res) => {
          setToasts((prev) =>
            prev.map((t) =>
              t.id === id
                ? { ...t, status: "success", title: res.duplicate ? "Already in your folio" : "Piece added", detail: label }
                : t,
            ),
          );
          window.setTimeout(() => {
            setToasts((prev) => prev.filter((t) => t.id !== id));
          }, 2800);
          // Refresh the gallery + stats in the background; failures here must
          // not flip the success toast, so they are deliberately swallowed.
          void queryClient.invalidateQueries({ queryKey: ["folio-catalog"] });
          void queryClient.invalidateQueries({ queryKey: ["folio-stats"] });
        })
        .catch((err: unknown) => {
          setToasts((prev) =>
            prev.map((t) =>
              t.id === id
                ? { ...t, status: "error", title: "Couldn’t add piece", detail: err instanceof Error ? err.message : label }
                : t,
            ),
          );
        });
    },
    [queryClient],
  );

  const dismissInboxAction = useCallback(
    (inboxItemId: number) => {
      const id = ++toastSeq;
      setToasts((prev) => [...prev, { id, status: "loading", title: "Dismissing inbox item" }]);
      dismissInboxItem(inboxItemId)
        .then(() => {
          setToasts((prev) =>
            prev.map((t) =>
              t.id === id ? { ...t, status: "success", title: "Inbox item dismissed" } : t,
            ),
          );
          window.setTimeout(() => {
            setToasts((prev) => prev.filter((t) => t.id !== id));
          }, 2800);
          void queryClient.invalidateQueries({ queryKey: ["inbox"] });
          void queryClient.invalidateQueries({ queryKey: ["inbox-counts"] });
        })
        .catch((err: unknown) => {
          setToasts((prev) =>
            prev.map((t) =>
              t.id === id
                ? { ...t, status: "error", title: "Couldn’t dismiss inbox item", detail: err instanceof Error ? err.message : undefined }
                : t,
            ),
          );
        });
    },
    [queryClient],
  );

  const createFolioAction = useCallback(
    (name: string) => {
      const id = ++toastSeq;
      const label = name.trim();
      setToasts((prev) => [...prev, { id, status: "loading", title: "Creating folio", detail: label }]);
      createFolio({ name: label })
        .then(() => {
          setToasts((prev) =>
            prev.map((t) => (t.id === id ? { ...t, status: "success", title: "Folio created", detail: label } : t)),
          );
          window.setTimeout(() => {
            setToasts((prev) => prev.filter((t) => t.id !== id));
          }, 2800);
          void queryClient.invalidateQueries({ queryKey: ["folios"] });
        })
        .catch((err: unknown) => {
          setToasts((prev) =>
            prev.map((t) =>
              t.id === id
                ? { ...t, status: "error", title: "Couldn’t create folio", detail: err instanceof Error ? err.message : label }
                : t,
            ),
          );
        });
    },
    [queryClient],
  );

  const renameFolioAction = useCallback(
    (folioId: number, name: string) => {
      const id = ++toastSeq;
      const label = name.trim();
      setToasts((prev) => [...prev, { id, status: "loading", title: "Renaming folio", detail: label }]);
      updateFolio(folioId, { name: label })
        .then(() => {
          setToasts((prev) =>
            prev.map((t) => (t.id === id ? { ...t, status: "success", title: "Folio renamed", detail: label } : t)),
          );
          window.setTimeout(() => {
            setToasts((prev) => prev.filter((t) => t.id !== id));
          }, 2800);
          void queryClient.invalidateQueries({ queryKey: ["folios"] });
        })
        .catch((err: unknown) => {
          setToasts((prev) =>
            prev.map((t) =>
              t.id === id
                ? { ...t, status: "error", title: "Couldn’t rename folio", detail: err instanceof Error ? err.message : label }
                : t,
            ),
          );
        });
    },
    [queryClient],
  );

  const deleteFolioAction = useCallback(
    (folioId: number) => {
      const id = ++toastSeq;
      setToasts((prev) => [...prev, { id, status: "loading", title: "Deleting folio" }]);
      deleteFolio(folioId)
        .then(() => {
          setToasts((prev) =>
            prev.map((t) => (t.id === id ? { ...t, status: "success", title: "Folio deleted" } : t)),
          );
          window.setTimeout(() => {
            setToasts((prev) => prev.filter((t) => t.id !== id));
          }, 2800);
          void queryClient.invalidateQueries({ queryKey: ["folios"] });
        })
        .catch((err: unknown) => {
          setToasts((prev) =>
            prev.map((t) =>
              t.id === id
                ? { ...t, status: "error", title: "Couldn’t delete folio", detail: err instanceof Error ? err.message : undefined }
                : t,
            ),
          );
        });
    },
    [queryClient],
  );

  const addPieceToFolioAction = useCallback(
    (folioId: number, photoId: number) => {
      const id = ++toastSeq;
      setToasts((prev) => [...prev, { id, status: "loading", title: "Adding piece to folio" }]);
      addPieceToFolio(folioId, photoId)
        .then(() => {
          setToasts((prev) =>
            prev.map((t) => (t.id === id ? { ...t, status: "success", title: "Piece added to folio" } : t)),
          );
          window.setTimeout(() => {
            setToasts((prev) => prev.filter((t) => t.id !== id));
          }, 2800);
          void queryClient.invalidateQueries({ queryKey: ["folio-pieces", folioId] });
          void queryClient.invalidateQueries({ queryKey: ["folios"] });
        })
        .catch((err: unknown) => {
          setToasts((prev) =>
            prev.map((t) =>
              t.id === id
                ? { ...t, status: "error", title: "Couldn’t add piece to folio", detail: err instanceof Error ? err.message : undefined }
                : t,
            ),
          );
        });
    },
    [queryClient],
  );

  const removePieceFromFolioAction = useCallback(
    (folioId: number, photoId: number) => {
      const id = ++toastSeq;
      setToasts((prev) => [...prev, { id, status: "loading", title: "Removing piece from folio" }]);
      removePieceFromFolio(folioId, photoId)
        .then(() => {
          setToasts((prev) =>
            prev.map((t) => (t.id === id ? { ...t, status: "success", title: "Piece removed from folio" } : t)),
          );
          window.setTimeout(() => {
            setToasts((prev) => prev.filter((t) => t.id !== id));
          }, 2800);
          void queryClient.invalidateQueries({ queryKey: ["folio-pieces", folioId] });
          void queryClient.invalidateQueries({ queryKey: ["folios"] });
        })
        .catch((err: unknown) => {
          setToasts((prev) =>
            prev.map((t) =>
              t.id === id
                ? { ...t, status: "error", title: "Couldn’t remove piece from folio", detail: err instanceof Error ? err.message : undefined }
                : t,
            ),
          );
        });
    },
    [queryClient],
  );

  const openPiece = useCallback((id: number) => setSelectedId(id), []);
  const closePiece = useCallback(() => setSelectedId(null), []);
  const filterByArtist = useCallback(
    (name: string) => {
      const next = name.trim();
      if (!next || next === "Unknown") return;
      setArtist(next);
      setSelectedId(null);
      void navigate("/");
    },
    [navigate],
  );
  const stepPiece = useCallback(
    (dir: number) => {
      setSelectedId((cur) => {
        const activePieces = viewerPieces.length > 0 ? viewerPieces : pieces;
        if (cur == null || activePieces.length === 0) return cur;
        const i = activePieces.findIndex((p) => p.id === cur);
        if (i < 0) return cur;
        const n = (i + dir + activePieces.length) % activePieces.length;
        return activePieces[n].id;
      });
    },
    [pieces, viewerPieces],
  );

  const setViewerPieces = useCallback((nextPieces: PieceVM[]) => {
    setViewerPiecesState(nextPieces);
  }, []);

  const activeViewerPieces = viewerPieces.length > 0 ? viewerPieces : pieces;

  const selected = useMemo(
    () => (selectedId == null ? null : activeViewerPieces.find((p) => p.id === selectedId) ?? null),
    [selectedId, activeViewerPieces],
  );
  const selIndex = useMemo(
    () => (selectedId == null ? -1 : activeViewerPieces.findIndex((p) => p.id === selectedId)),
    [selectedId, activeViewerPieces],
  );

  const value: FolioContextValue = {
    theme,
    setTheme,
    toggleTheme,
    query,
    setQuery,
    favoriteOnly,
    setFavoriteOnly,
    artist,
    setArtist,
    filterByArtist,
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
    inboxCount: inboxCounts.data?.total ?? 0,
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
    importPiece,
    dismissInboxAction,
    createFolioAction,
    renameFolioAction,
    deleteFolioAction,
    addPieceToFolioAction,
    removePieceFromFolioAction,
    setViewerPieces,
    toasts,
    dismissToast,
  };

  return <FolioContext.Provider value={value}>{children}</FolioContext.Provider>;
}
