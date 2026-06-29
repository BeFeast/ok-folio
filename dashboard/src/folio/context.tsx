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
  bulkEditCatalog,
  createFolio,
  createPiece,
  deleteFolio,
  dismissInboxItem,
  fetchGalleryCatalog,
  fetchInboxCounts,
  fetchStats,
  getPhotoImageUrl,
  getPhotoThumbnailUrl,
  keepInboxItem,
  moveInboxItemToFolio,
  removeFromFavorites,
  removePieceFromFolio,
  skipInboxItem,
  updatePieceMetadata,
  updateFolio,
  type BulkMetadataEdit,
  type CreatePieceInput,
  type PieceMetadataPatch,
} from "../api";
import type { FolioPiecesResponse, GalleryCatalogResponse, Photo } from "../types";
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
  editDate: string; // YYYY-MM-DD date value for metadata editing
  src: string; // source label
  med: string; // medium (empty when unknown)
  kind: string; // eyebrow kind label
  note: string; // personal note (empty when none)
  keywords: string[];
  folio: string; // suggested/assigned folio (empty when none)
  img: string;
  thumb: string;
  fav: boolean;
  file: string;
  size: string;
  dim: string; // dimensions (empty when unknown)
  captured: string;
  camera: string;
  lens: string;
  added: string;
  addedExact: string; // absolute date-time for the "Added" tooltip ("" when unknown)
  editedFields: string[];
}

const PAGE_SIZE = 120;

/* ---- mapping helpers ---- */

// A real title or "" — we never fabricate one. The catalog stores junk for most pieces
// (empty, "***"/punctuation-only, or a UUID filename leaked into the title field); treat
// all of those as untitled so the gallery shows the artist only (an empty title collapses
// the title line). Real artwork dates are unknown, so museum captions omit the year and
// dates live in the info sheet (Added / Captured) instead.
function cleanTitle(raw: string): string {
  const t = (raw || "").trim();
  if (!t) return "";
  if (/^[\s*_.\-—–·•]+$/.test(t)) return "";
  if (/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}/i.test(t)) return "";
  return t;
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

function absoluteAdded(value: string): string {
  if (!value) return "";
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return "";
  return d.toLocaleString(undefined, { dateStyle: "long", timeStyle: "short" });
}

function formatMetadataDate(value: string | null): string {
  if (!value) return "";
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return "";
  return d.toLocaleDateString(undefined, {
    year: "numeric",
    month: "short",
    day: "numeric",
    timeZone: "UTC",
  });
}

function editorDateValue(value: string | null): string {
  if (!value) return "";
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return "";
  return d.toISOString().slice(0, 10);
}

function cameraLabel(make: string, model: string): string {
  const cleanMake = (make || "").trim();
  const cleanModel = (model || "").trim();
  if (!cleanMake) return cleanModel;
  if (!cleanModel) return cleanMake;
  if (cleanModel.toLowerCase().startsWith(cleanMake.toLowerCase())) return cleanModel;
  return `${cleanMake} ${cleanModel}`;
}

export function mapPhoto(p: Photo): PieceVM {
  const title = cleanTitle(p.Title);
  const artist = (p.Artist || "").trim();
  return {
    id: p.ID,
    t: title,
    a: artist || "Unknown",
    y: "",
    editDate: editorDateValue(p.UploadDate || null),
    src: hostFrom(p.SourcePage) || p.SourcePage || "—",
    med: "",
    kind: "",
    note: (p.Notes || "").trim(),
    keywords: Array.isArray(p.keywords) ? p.keywords.map((keyword) => keyword.trim()).filter(Boolean) : [],
    folio: "",
    img: getPhotoImageUrl(p.ID),
    thumb: getPhotoThumbnailUrl(p.ID, 400),
    fav: !!p.Favorite,
    file: p.FileName || "—",
    size: formatBytes(p.FileSize),
    dim: p.ImageWidth && p.ImageHeight ? `${p.ImageWidth} x ${p.ImageHeight}` : "",
    captured: formatMetadataDate(p.CapturedAt),
    camera: cameraLabel(p.CameraMake, p.CameraModel),
    lens: (p.LensModel || "").trim(),
    added: relativeAdded(p.DownloadedAt),
    addedExact: absoluteAdded(p.DownloadedAt),
    editedFields: Array.isArray(p.manual_fields) ? p.manual_fields.map((field) => field.trim()).filter(Boolean) : [],
  };
}

function normalizeKeywords(keywords: string[]): string[] {
  const seen = new Set<string>();
  const out: string[] = [];
  for (const raw of keywords) {
    const value = raw.trim();
    const key = value.toLowerCase();
    if (!value || seen.has(key)) continue;
    seen.add(key);
    out.push(value);
  }
  return out;
}

function mergeEditedFields(existing: string[], fields: string[]): string[] {
  return normalizeKeywords([...existing, ...fields]);
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
  category: string;
  setCategory: (category: string) => void;
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

  viewOpen: boolean;
  openView: () => void;
  closeView: () => void;

  isFav: (id: number) => boolean;
  toggleFav: (id: number) => void;

  importPiece: (input: CreatePieceInput) => void;
  dismissInboxAction: (id: number) => void;
  keepInboxAction: (id: number) => void;
  skipInboxAction: (id: number) => void;
  moveInboxToFolioAction: (id: number, folioId: number, photoId?: number) => void;
  createFolioAction: (name: string) => void;
  renameFolioAction: (id: number, name: string) => void;
  changeFolioCoverAction: (id: number, photoId: number | null) => void;
  deleteFolioAction: (id: number) => Promise<boolean>;
  addPieceToFolioAction: (folioId: number, photoId: number) => void;
  removePieceFromFolioAction: (folioId: number, photoId: number) => void;
  editPieceMetadata: (id: number, input: PieceMetadataPatch) => Promise<boolean>;
  bulkEditPieces: (input: BulkMetadataEdit) => Promise<boolean>;
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
  const [category, setCategory] = useState("");
  const [mode, setMode] = useState<GalleryMode>("library");
  const [selectedId, setSelectedId] = useState<number | null>(null);
  const [addOpen, setAddOpen] = useState(false);
  const [viewOpen, setViewOpen] = useState(false);
  const [favOverride, setFavOverride] = useState<Record<number, boolean>>({});
  const [metadataOverrides, setMetadataOverrides] = useState<Record<number, Partial<PieceVM>>>({});
  const [toasts, setToasts] = useState<Toast[]>([]);
  const [viewerPieces, setViewerPiecesState] = useState<PieceVM[]>([]);

  // Theme: keep the document tokens in sync with state.
  useEffect(() => {
    applyTheme(theme);
    if (theme !== "auto" || typeof window === "undefined" || typeof window.matchMedia !== "function") {
      return;
    }
    const query = window.matchMedia("(prefers-color-scheme: dark)");
    const update = () => applyTheme("auto");
    if (typeof query.addEventListener === "function") {
      query.addEventListener("change", update);
      return () => query.removeEventListener("change", update);
    }
    query.addListener(update);
    return () => query.removeListener(update);
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
    queryKey: ["folio-catalog", debouncedQuery, favoriteOnly, artist, category],
    queryFn: ({ pageParam }) => {
      const filters: Parameters<typeof fetchGalleryCatalog>[2] = {};
      if (debouncedQuery) filters.query = debouncedQuery;
      if (favoriteOnly) filters.favorite = true;
      if (artist) filters.artist = artist;
      if (category) filters.category = category;
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

  const overlayPiece = useCallback(
    (piece: PieceVM): PieceVM => {
      const override = metadataOverrides[piece.id];
      const withMetadata = override ? { ...piece, ...override } : piece;
      const fav = favOverride[withMetadata.id];
      return fav === undefined ? withMetadata : { ...withMetadata, fav };
    },
    [favOverride, metadataOverrides],
  );

  const pieces = useMemo<PieceVM[]>(() => {
    const photos = catalog.data?.pages.flatMap((pg) => pg.photos) ?? [];
    return photos.map((p) => overlayPiece(mapPhoto(p)));
  }, [catalog.data, overlayPiece]);

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

  const resolveInboxAction = useCallback(
    (inboxItemId: number, action: "keep" | "skip", request: (id: number) => Promise<void>) => {
      const id = ++toastSeq;
      const verb = action === "keep" ? "Keeping" : "Skipping";
      const done = action === "keep" ? "Inbox item kept" : "Inbox item skipped";
      const failed = action === "keep" ? "Couldn’t keep inbox item" : "Couldn’t skip inbox item";
      setToasts((prev) => [...prev, { id, status: "loading", title: `${verb} inbox item` }]);
      request(inboxItemId)
        .then(() => {
          setToasts((prev) => prev.map((t) => (t.id === id ? { ...t, status: "success", title: done } : t)));
          window.setTimeout(() => {
            setToasts((prev) => prev.filter((t) => t.id !== id));
          }, 2800);
          void queryClient.invalidateQueries({ queryKey: ["inbox"] });
          void queryClient.invalidateQueries({ queryKey: ["inbox-counts"] });
        })
        .catch((err: unknown) => {
          setToasts((prev) =>
            prev.map((t) =>
              t.id === id ? { ...t, status: "error", title: failed, detail: err instanceof Error ? err.message : undefined } : t,
            ),
          );
        });
    },
    [queryClient],
  );

  const keepInboxAction = useCallback(
    (inboxItemId: number) => resolveInboxAction(inboxItemId, "keep", keepInboxItem),
    [resolveInboxAction],
  );

  const skipInboxAction = useCallback(
    (inboxItemId: number) => resolveInboxAction(inboxItemId, "skip", skipInboxItem),
    [resolveInboxAction],
  );

  const moveInboxToFolioAction = useCallback(
    (inboxItemId: number, folioId: number, photoId?: number) => {
      const id = ++toastSeq;
      setToasts((prev) => [...prev, { id, status: "loading", title: "Adding inbox item to folio" }]);
      moveInboxItemToFolio(inboxItemId, folioId, photoId)
        .then(() => {
          setToasts((prev) => prev.map((t) => (t.id === id ? { ...t, status: "success", title: "Inbox item added to folio" } : t)));
          window.setTimeout(() => {
            setToasts((prev) => prev.filter((t) => t.id !== id));
          }, 2800);
          void queryClient.invalidateQueries({ queryKey: ["inbox"] });
          void queryClient.invalidateQueries({ queryKey: ["inbox-counts"] });
          void queryClient.invalidateQueries({ queryKey: ["folios"] });
          void queryClient.invalidateQueries({ queryKey: ["folio-pieces", folioId] });
        })
        .catch((err: unknown) => {
          setToasts((prev) =>
            prev.map((t) =>
              t.id === id
                ? { ...t, status: "error", title: "Couldn’t add inbox item to folio", detail: err instanceof Error ? err.message : undefined }
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

  const changeFolioCoverAction = useCallback(
    (folioId: number, photoId: number | null) => {
      const id = ++toastSeq;
      setToasts((prev) => [...prev, { id, status: "loading", title: "Changing folio cover" }]);
      updateFolio(folioId, { cover_photo_id: photoId })
        .then(() => {
          setToasts((prev) =>
            prev.map((t) => (t.id === id ? { ...t, status: "success", title: "Folio cover changed" } : t)),
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
                ? { ...t, status: "error", title: "Couldn’t change folio cover", detail: err instanceof Error ? err.message : undefined }
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
      return deleteFolio(folioId)
        .then(() => {
          setToasts((prev) =>
            prev.map((t) => (t.id === id ? { ...t, status: "success", title: "Folio deleted" } : t)),
          );
          window.setTimeout(() => {
            setToasts((prev) => prev.filter((t) => t.id !== id));
          }, 2800);
          void queryClient.invalidateQueries({ queryKey: ["folios"] });
          void queryClient.invalidateQueries({ queryKey: ["folio-pieces", folioId] });
          return true;
        })
        .catch((err: unknown) => {
          setToasts((prev) =>
            prev.map((t) =>
              t.id === id
                ? { ...t, status: "error", title: "Couldn’t delete folio", detail: err instanceof Error ? err.message : undefined }
                : t,
            ),
          );
          return false;
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

  const updatePhotoInCaches = useCallback(
    (photo: Photo) => {
      queryClient.setQueriesData<{ pages: GalleryCatalogResponse[] }>({ queryKey: ["folio-catalog"] }, (data) =>
        data
          ? {
              ...data,
              pages: data.pages.map((page) => ({
                ...page,
                photos: page.photos.map((item) => (item.ID === photo.ID ? photo : item)),
              })),
            }
          : data,
      );
      queryClient.setQueriesData<{ pages: FolioPiecesResponse[] }>({ queryKey: ["folio-pieces"] }, (data) =>
        data
          ? {
              ...data,
              pages: data.pages.map((page) => ({
                ...page,
                photos: page.photos.map((item) => (item.ID === photo.ID ? photo : item)),
              })),
            }
          : data,
      );
      setMetadataOverrides((prev) => ({ ...prev, [photo.ID]: mapPhoto(photo) }));
    },
    [queryClient],
  );

  const updatePhotosInInfiniteCaches = useCallback(
    (photos: Photo[]) => {
      if (photos.length === 0) return;
      const byId = new Map(photos.map((photo) => [photo.ID, photo]));
      queryClient.setQueriesData<{ pages: GalleryCatalogResponse[] }>({ queryKey: ["folio-catalog"] }, (data) =>
        data
          ? {
              ...data,
              pages: data.pages.map((page) => ({
                ...page,
                photos: page.photos.map((photo) => byId.get(photo.ID) ?? photo),
              })),
            }
          : data,
      );
      queryClient.setQueriesData<{ pages: FolioPiecesResponse[] }>({ queryKey: ["folio-pieces"] }, (data) =>
        data
          ? {
              ...data,
              pages: data.pages.map((page) => ({
                ...page,
                photos: page.photos.map((photo) => byId.get(photo.ID) ?? photo),
              })),
            }
          : data,
      );
      setMetadataOverrides((prev) => {
        const next = { ...prev };
        for (const photo of photos) next[photo.ID] = mapPhoto(photo);
        return next;
      });
    },
    [queryClient],
  );

  const editPieceMetadata = useCallback(
    (id: number, input: PieceMetadataPatch) => {
      const toastId = ++toastSeq;
      const fields = [
        input.title !== undefined ? "title" : "",
        input.artist !== undefined ? "artist" : "",
        input.date !== undefined ? "date" : "",
        input.keywords !== undefined ? "keywords" : "",
      ].filter(Boolean);
      if (fields.length === 0) return Promise.resolve(true);
      const hadPreviousOverride = Object.prototype.hasOwnProperty.call(metadataOverrides, id);
      const previousOverride = metadataOverrides[id];
      setToasts((prev) => [...prev, { id: toastId, status: "loading", title: "Saving edits" }]);
      setMetadataOverrides((prev) => {
        const current = prev[id] ?? pieces.find((piece) => piece.id === id) ?? viewerPieces.find((piece) => piece.id === id);
        if (!current) return prev;
        const next: Partial<PieceVM> = { ...current };
        if (input.title !== undefined) next.t = input.title.trim();
        if (input.artist !== undefined) next.a = input.artist.trim() || "Unknown";
        if (input.date !== undefined) next.editDate = input.date ?? "";
        if (input.keywords !== undefined) next.keywords = normalizeKeywords(input.keywords);
        next.editedFields = mergeEditedFields(current.editedFields ?? [], fields);
        return { ...prev, [id]: next };
      });
      return updatePieceMetadata(id, input)
        .then((photo) => {
          updatePhotoInCaches(photo);
          setToasts((prev) => prev.map((toast) => (toast.id === toastId ? { ...toast, status: "success", title: "Piece edited" } : toast)));
          window.setTimeout(() => setToasts((prev) => prev.filter((toast) => toast.id !== toastId)), 2600);
          void queryClient.invalidateQueries({ queryKey: ["folio-catalog"] });
          void queryClient.invalidateQueries({ queryKey: ["gallery-catalog"] });
          void queryClient.invalidateQueries({ queryKey: ["folio-pieces"] });
          return true;
        })
        .catch((err: unknown) => {
          setToasts((prev) =>
            prev.map((toast) =>
              toast.id === toastId
                ? { ...toast, status: "error", title: "Couldn’t save edits", detail: err instanceof Error ? err.message : undefined }
                : toast,
            ),
          );
          setMetadataOverrides((prev) => {
            const next = { ...prev };
            if (hadPreviousOverride && previousOverride) next[id] = previousOverride;
            else delete next[id];
            return next;
          });
          void queryClient.invalidateQueries({ queryKey: ["folio-catalog"] });
          return false;
        });
    },
    [metadataOverrides, pieces, queryClient, updatePhotoInCaches, viewerPieces],
  );

  const bulkEditPieces = useCallback(
    (input: BulkMetadataEdit) => {
      const ids = input.ids.filter((id, index) => input.ids.indexOf(id) === index);
      if (ids.length === 0) return Promise.resolve(false);
      const toastId = ++toastSeq;
      const fields = [
        input.set_artist !== undefined ? "artist" : "",
        input.set_date !== undefined ? "date" : "",
        input.add_keywords !== undefined || input.remove_keywords !== undefined ? "keywords" : "",
      ].filter(Boolean);
      if (fields.length === 0) return Promise.resolve(true);
      const previousOverrides = new Map<number, Partial<PieceVM> | undefined>();
      for (const id of ids) {
        if (Object.prototype.hasOwnProperty.call(metadataOverrides, id)) {
          previousOverrides.set(id, metadataOverrides[id]);
        }
      }
      setToasts((prev) => [...prev, { id: toastId, status: "loading", title: `Updating ${ids.length} pieces` }]);
      setMetadataOverrides((prev) => {
        const known = new Map([...pieces, ...viewerPieces].map((piece) => [piece.id, piece]));
        const next = { ...prev };
        for (const id of ids) {
          const current = next[id] ?? known.get(id);
          if (!current) continue;
          const patch: Partial<PieceVM> = { ...current };
          if (input.set_artist !== undefined) patch.a = input.set_artist.trim() || "Unknown";
          if (input.set_date !== undefined) patch.editDate = input.set_date.trim();
          if (input.add_keywords !== undefined || input.remove_keywords !== undefined) {
            const remove = new Set((input.remove_keywords ?? []).map((keyword) => keyword.trim().toLowerCase()).filter(Boolean));
            patch.keywords = normalizeKeywords([...(current.keywords ?? []).filter((keyword) => !remove.has(keyword.toLowerCase())), ...(input.add_keywords ?? [])]);
          }
          patch.editedFields = mergeEditedFields(current.editedFields ?? [], fields);
          next[id] = patch;
        }
        return next;
      });
      return bulkEditCatalog({ ...input, ids })
        .then((result) => {
          updatePhotosInInfiniteCaches(result.photos);
          setToasts((prev) =>
            prev.map((toast) =>
              toast.id === toastId
                ? { ...toast, status: "success", title: `Updated ${result.updated} ${result.updated === 1 ? "piece" : "pieces"}` }
                : toast,
            ),
          );
          window.setTimeout(() => setToasts((prev) => prev.filter((toast) => toast.id !== toastId)), 3000);
          void queryClient.invalidateQueries({ queryKey: ["folio-catalog"] });
          void queryClient.invalidateQueries({ queryKey: ["gallery-catalog"] });
          void queryClient.invalidateQueries({ queryKey: ["folio-pieces"] });
          return true;
        })
        .catch((err: unknown) => {
          setToasts((prev) =>
            prev.map((toast) =>
              toast.id === toastId
                ? { ...toast, status: "error", title: "Couldn’t update selected pieces", detail: err instanceof Error ? err.message : undefined }
                : toast,
            ),
          );
          setMetadataOverrides((prev) => {
            const next = { ...prev };
            for (const id of ids) {
              const previousOverride = previousOverrides.get(id);
              if (previousOverride) next[id] = previousOverride;
              else delete next[id];
            }
            return next;
          });
          void queryClient.invalidateQueries({ queryKey: ["folio-catalog"] });
          void queryClient.invalidateQueries({ queryKey: ["folio-pieces"] });
          return false;
        });
    },
    [metadataOverrides, pieces, queryClient, updatePhotosInInfiniteCaches, viewerPieces],
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

  const activeViewerPieces = viewerPieces.length > 0 ? viewerPieces.map(overlayPiece) : pieces;

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
    category,
    setCategory,
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
    viewOpen,
    openView: useCallback(() => setViewOpen(true), []),
    closeView: useCallback(() => setViewOpen(false), []),
    isFav,
    toggleFav,
    importPiece,
    dismissInboxAction,
    keepInboxAction,
    skipInboxAction,
    moveInboxToFolioAction,
    createFolioAction,
    renameFolioAction,
    changeFolioCoverAction,
    deleteFolioAction,
    addPieceToFolioAction,
    removePieceFromFolioAction,
    editPieceMetadata,
    bulkEditPieces,
    setViewerPieces,
    toasts,
    dismissToast,
  };

  return <FolioContext.Provider value={value}>{children}</FolioContext.Provider>;
}
