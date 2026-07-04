import type {
  Stats,
  RunsResponse,
  HealthResponse,
  WorkerStatus,
  ConnectorStatusResponse,
  TimelineResponse,
  TopArtistsResponse,
  FailedPhotosResponse,
  SearchResponse,
  ArtistsResponse,
  ArtistDetailResponse,
  TodayPhotosResponse,
  WeekPhotosResponse,
  GalleryCatalogResponse,
  GallerySimilarResponse,
  InboxCountsResponse,
  InboxItem,
  InboxResponse,
  PhotoDetailResponse,
  RunPhotosResponse,
  Folio,
  FoliosResponse,
  FolioPiecesResponse,
  Photo,
  ConnectorSourceInput,
  ConnectorSourceBackfillResult,
  ConnectorSourcePreviewInput,
  ConnectorSourcePreviewResponse,
  ConnectorSourceSetting,
  ConnectorSourcesResponse,
} from "./types";

const API_BASE = "/api/v1";
const API_GET_CACHE_NAME = "ok-folio-api-get";
const PIECE_IMAGE_CACHE_NAME = "ok-folio-piece-images";
const gallerySimilarNotFoundIds = new Set<number>();

async function clearOfflineCaches(cacheNames: string[] = [API_GET_CACHE_NAME]): Promise<void> {
  if (typeof caches === "undefined") {
    return;
  }

  await Promise.all(
    cacheNames.map(async (cacheName) => {
      try {
        await caches.delete(cacheName);
      } catch {
        // Cache cleanup should not turn a successful server mutation into a UI error.
      }
    }),
  );
}

export async function fetchStats(): Promise<Stats> {
  const response = await fetch(`${API_BASE}/stats`);
  if (!response.ok) {
    throw new Error("Failed to fetch stats");
  }
  return response.json();
}

export async function fetchRuns(limit: number = 20): Promise<RunsResponse> {
  const response = await fetch(`${API_BASE}/runs?limit=${limit}`);
  if (!response.ok) {
    throw new Error("Failed to fetch runs");
  }
  return response.json();
}

export async function fetchHealth(): Promise<HealthResponse> {
  const response = await fetch("/health");
  if (!response.ok) {
    throw new Error("Failed to fetch health");
  }
  return response.json();
}

export async function triggerExtraction(): Promise<void> {
  const response = await fetch(`${API_BASE}/extract`, {
    method: "POST",
  });
  if (!response.ok) {
    throw new Error("Failed to trigger extraction");
  }
  await clearOfflineCaches([API_GET_CACHE_NAME, PIECE_IMAGE_CACHE_NAME]);
}

export async function triggerPageExtraction(page: number): Promise<void> {
  const response = await fetch(`${API_BASE}/extract/page/${page}`, {
    method: "POST",
  });
  if (!response.ok) {
    throw new Error("Failed to trigger page extraction");
  }
  await clearOfflineCaches([API_GET_CACHE_NAME, PIECE_IMAGE_CACHE_NAME]);
}

export async function triggerPagesExtraction(count: number): Promise<void> {
  const response = await fetch(`${API_BASE}/extract/pages?count=${count}`, {
    method: "POST",
  });
  if (!response.ok) {
    throw new Error("Failed to trigger pages extraction");
  }
  await clearOfflineCaches([API_GET_CACHE_NAME, PIECE_IMAGE_CACHE_NAME]);
}

export async function fetchWorkerStatus(): Promise<WorkerStatus> {
  const response = await fetch(`${API_BASE}/workers/status`);
  if (!response.ok) {
    throw new Error("Failed to fetch worker status");
  }
  return response.json();
}

export async function fetchConnectorStatus(): Promise<ConnectorStatusResponse> {
  const response = await fetch(`${API_BASE}/streams/connectors/status`);
  if (!response.ok) {
    throw new Error("Failed to fetch connector status");
  }
  return response.json();
}

export async function fetchConnectorSources(type: string = "telegram"): Promise<ConnectorSourcesResponse> {
  const params = new URLSearchParams();
  if (type) {
    params.set("type", type);
  }
  const response = await fetch(`${API_BASE}/settings/connector-sources?${params}`);
  if (!response.ok) {
    throw new Error("Failed to fetch connector sources");
  }
  return response.json();
}

export async function createConnectorSource(input: ConnectorSourceInput): Promise<ConnectorSourceSetting> {
  const response = await fetch(`${API_BASE}/settings/connector-sources`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  });
  if (!response.ok) {
    const data = await response.json().catch(() => ({}));
    throw new Error(data.error || "Failed to create connector source");
  }
  const source = await response.json();
  await clearOfflineCaches();
  return source;
}

export async function previewConnectorSource(input: ConnectorSourcePreviewInput): Promise<ConnectorSourcePreviewResponse> {
  const response = await fetch(`${API_BASE}/settings/connector-sources/preview`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  });
  if (!response.ok) {
    const data = await response.json().catch(() => ({}));
    throw new Error(data.error || "Failed to preview connector source");
  }
  return response.json();
}

export async function updateConnectorSource(id: number, input: ConnectorSourceInput): Promise<ConnectorSourceSetting> {
  const response = await fetch(`${API_BASE}/settings/connector-sources/${id}`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  });
  if (!response.ok) {
    const data = await response.json().catch(() => ({}));
    throw new Error(data.error || "Failed to update connector source");
  }
  const source = await response.json();
  await clearOfflineCaches();
  return source;
}

export async function deleteConnectorSource(id: number): Promise<void> {
  const response = await fetch(`${API_BASE}/settings/connector-sources/${id}`, {
    method: "DELETE",
  });
  if (!response.ok) {
    const data = await response.json().catch(() => ({}));
    throw new Error(data.error || "Failed to delete connector source");
  }
  await clearOfflineCaches();
}

export async function backfillConnectorSource(id: number): Promise<ConnectorSourceBackfillResult> {
  const response = await fetch(`${API_BASE}/settings/connector-sources/${id}/backfill`, {
    method: "POST",
  });
  if (!response.ok) {
    const data = await response.json().catch(() => ({}));
    throw new Error(data.error || "Failed to backfill connector source");
  }
  const result = await response.json();
  await clearOfflineCaches();
  return result;
}

export async function fetchFolios(): Promise<FoliosResponse> {
  const response = await fetch(`${API_BASE}/folios`);
  if (!response.ok) {
    throw new Error("Failed to fetch folios");
  }
  return response.json();
}

export async function fetchFolioPieces(
  id: number,
  limit: number = 100,
  offset: number = 0,
): Promise<FolioPiecesResponse> {
  const params = new URLSearchParams({
    limit: limit.toString(),
    offset: offset.toString(),
  });
  const response = await fetch(`${API_BASE}/folios/${id}/pieces?${params}`);
  if (!response.ok) {
    throw new Error("Failed to fetch folio pieces");
  }
  return response.json();
}

export async function createFolio(input: { name: string; cover_photo_id?: number | null }): Promise<Folio> {
  const response = await fetch(`${API_BASE}/folios`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  });
  if (!response.ok) {
    const data = await response.json().catch(() => ({}));
    throw new Error(data.error || "Failed to create folio");
  }
  const folio = await response.json();
  await clearOfflineCaches();
  return folio;
}

export async function updateFolio(id: number, input: { name?: string; cover_photo_id?: number | null }): Promise<Folio> {
  const response = await fetch(`${API_BASE}/folios/${id}`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  });
  if (!response.ok) {
    const data = await response.json().catch(() => ({}));
    throw new Error(data.error || "Failed to update folio");
  }
  const folio = await response.json();
  await clearOfflineCaches();
  return folio;
}

export async function deleteFolio(id: number): Promise<void> {
  const response = await fetch(`${API_BASE}/folios/${id}`, {
    method: "DELETE",
  });
  if (!response.ok) {
    const data = await response.json().catch(() => ({}));
    throw new Error(data.error || "Failed to delete folio");
  }
  await clearOfflineCaches();
}

export type AddPieceToFolioResult = {
  added: boolean;
  duplicate?: boolean;
};

export async function addPieceToFolio(folioId: number, photoId: number): Promise<AddPieceToFolioResult> {
  const response = await fetch(`${API_BASE}/folios/${folioId}/pieces`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ photo_id: photoId }),
  });
  if (!response.ok) {
    const data = await response.json().catch(() => ({}));
    throw new Error(data.error || "Failed to add piece to folio");
  }
  const result = await response.json();
  await clearOfflineCaches();
  return result;
}

export async function removePieceFromFolio(folioId: number, photoId: number): Promise<void> {
  const response = await fetch(`${API_BASE}/folios/${folioId}/pieces/${photoId}`, {
    method: "DELETE",
  });
  if (!response.ok) {
    const data = await response.json().catch(() => ({}));
    throw new Error(data.error || "Failed to remove piece from folio");
  }
  await clearOfflineCaches();
}

export async function fetchTimeline(
  days: number = 7,
): Promise<TimelineResponse> {
  const response = await fetch(`${API_BASE}/stats/timeline?days=${days}`);
  if (!response.ok) {
    throw new Error("Failed to fetch timeline");
  }
  return response.json();
}

export async function fetchTopArtists(
  limit: number = 10,
): Promise<TopArtistsResponse> {
  const response = await fetch(`${API_BASE}/stats/artists/top?limit=${limit}`);
  if (!response.ok) {
    throw new Error("Failed to fetch top artists");
  }
  return response.json();
}

export async function fetchFailedPhotos(
  limit: number = 20,
): Promise<FailedPhotosResponse> {
  const response = await fetch(`${API_BASE}/photos/failed?limit=${limit}`);
  if (!response.ok) {
    throw new Error("Failed to fetch failed photos");
  }
  return response.json();
}

export async function retryPhoto(id: number): Promise<void> {
  const response = await fetch(`${API_BASE}/photos/retry?id=${id}`, {
    method: "POST",
  });
  if (!response.ok) {
    throw new Error("Failed to retry photo");
  }
  await clearOfflineCaches([API_GET_CACHE_NAME, PIECE_IMAGE_CACHE_NAME]);
}

export async function searchPhotos(
  query: string,
  limit: number = 50,
  offset: number = 0,
): Promise<SearchResponse> {
  const params = new URLSearchParams({
    q: query,
    limit: limit.toString(),
    offset: offset.toString(),
  });
  const response = await fetch(`${API_BASE}/search?${params}`);
  if (!response.ok) {
    throw new Error("Failed to search photos");
  }
  return response.json();
}

export async function fetchArtists(
  limit: number = 50,
  offset: number = 0,
  sort: string = "count",
): Promise<ArtistsResponse> {
  const params = new URLSearchParams({
    limit: limit.toString(),
    offset: offset.toString(),
    sort,
  });
  const response = await fetch(`${API_BASE}/artists?${params}`);
  if (!response.ok) {
    throw new Error("Failed to fetch artists");
  }
  return response.json();
}

export async function fetchArtistDetail(
  artist: string,
  limit: number = 50,
  offset: number = 0,
): Promise<ArtistDetailResponse> {
  const params = new URLSearchParams({
    artist,
    limit: limit.toString(),
    offset: offset.toString(),
  });
  const response = await fetch(`${API_BASE}/artists/detail?${params}`);
  if (!response.ok) {
    throw new Error("Failed to fetch artist detail");
  }
  return response.json();
}

export async function fetchTodayPhotos(
  limit: number = 50,
  offset: number = 0,
): Promise<TodayPhotosResponse> {
  const params = new URLSearchParams({
    limit: limit.toString(),
    offset: offset.toString(),
  });
  const response = await fetch(`${API_BASE}/photos/today?${params}`);
  if (!response.ok) {
    throw new Error("Failed to fetch today's photos");
  }
  return response.json();
}

export async function fetchWeekPhotos(
  limit: number = 50,
  offset: number = 0,
): Promise<WeekPhotosResponse> {
  const params = new URLSearchParams({
    limit: limit.toString(),
    offset: offset.toString(),
  });
  const response = await fetch(`${API_BASE}/photos/week?${params}`);
  if (!response.ok) {
    throw new Error("Failed to fetch week's photos");
  }
  return response.json();
}

export async function fetchGalleryCatalog(
  limit: number = 100,
  offset: number = 0,
  filters: { provider?: string; source?: string; category?: string; artist?: string; favorite?: boolean; query?: string } = {},
): Promise<GalleryCatalogResponse> {
  const params = new URLSearchParams({
    limit: limit.toString(),
    offset: offset.toString(),
  });
  if (filters.provider) {
    params.set("provider", filters.provider);
  }
  if (filters.source) {
    params.set("source", filters.source);
  }
  if (filters.category) {
    params.set("category", filters.category);
  }
  if (filters.artist !== undefined) {
    params.set("artist", filters.artist);
  }
  if (filters.favorite !== undefined) {
    params.set("favorite", String(filters.favorite));
  }
  if (filters.query) {
    params.set("q", filters.query);
  }
  const response = await fetch(`${API_BASE}/gallery/catalog?${params}`);
  if (!response.ok) {
    throw new Error("Failed to fetch gallery catalog");
  }
  return response.json();
}

export interface PieceMetadataPatch {
  title?: string;
  artist?: string;
  date?: string | null;
  keywords?: string[];
}

export interface BulkMetadataEdit {
  ids: number[];
  set_artist?: string;
  set_date?: string;
  add_keywords?: string[];
  remove_keywords?: string[];
}

export interface BulkMetadataEditResponse {
  updated: number;
  skipped: number;
  photos: Photo[];
}

export async function updatePieceMetadata(id: number, input: PieceMetadataPatch): Promise<Photo> {
  const response = await fetch(`${API_BASE}/photos/${id}`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  });
  if (!response.ok) {
    const data = await response.json().catch(() => ({}));
    throw new Error(data.error || "Failed to update piece metadata");
  }
  const photo = await response.json();
  await clearOfflineCaches();
  return photo;
}

export async function bulkEditCatalog(input: BulkMetadataEdit): Promise<BulkMetadataEditResponse> {
  const response = await fetch(`${API_BASE}/catalog/bulk-edit`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  });
  if (!response.ok) {
    const data = await response.json().catch(() => ({}));
    throw new Error(data.error || "Failed to update selected pieces");
  }
  const result = await response.json();
  await clearOfflineCaches();
  return result;
}

export async function fetchInbox(
  status: InboxItem["status"] | "" = "",
  limit: number = 50,
  offset: number = 0,
): Promise<InboxResponse> {
  const params = new URLSearchParams({
    limit: limit.toString(),
    offset: offset.toString(),
  });
  if (status) {
    params.set("status", status);
  }
  const response = await fetch(`${API_BASE}/inbox?${params}`);
  if (!response.ok) {
    throw new Error("Failed to fetch inbox");
  }
  return response.json();
}

export async function fetchInboxCounts(): Promise<InboxCountsResponse> {
  const response = await fetch(`${API_BASE}/inbox/counts`);
  if (!response.ok) {
    throw new Error("Failed to fetch inbox counts");
  }
  return response.json();
}

export async function dismissInboxItem(id: number): Promise<void> {
  const response = await fetch(`${API_BASE}/inbox/${id}`, {
    method: "DELETE",
  });
  if (!response.ok) {
    const data = await response.json().catch(() => ({}));
    throw new Error(data.error || "Failed to dismiss inbox item");
  }
  await clearOfflineCaches();
}

export async function keepInboxItem(id: number): Promise<void> {
  const response = await fetch(`${API_BASE}/inbox/${id}/keep`, {
    method: "POST",
  });
  if (!response.ok) {
    const data = await response.json().catch(() => ({}));
    throw new Error(data.error || "Failed to keep inbox item");
  }
  await clearOfflineCaches();
}

export async function skipInboxItem(id: number): Promise<void> {
  const response = await fetch(`${API_BASE}/inbox/${id}/skip`, {
    method: "POST",
  });
  if (!response.ok) {
    const data = await response.json().catch(() => ({}));
    throw new Error(data.error || "Failed to skip inbox item");
  }
  await clearOfflineCaches();
}

export async function moveInboxItemToFolio(id: number, folioId: number, photoId?: number): Promise<void> {
  const body: { folio_id: number; photo_id?: number } = { folio_id: folioId };
  if (photoId !== undefined) {
    body.photo_id = photoId;
  }
  const response = await fetch(`${API_BASE}/inbox/${id}/move`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  if (!response.ok) {
    const data = await response.json().catch(() => ({}));
    throw new Error(data.error || "Failed to move inbox item");
  }
  await clearOfflineCaches();
}

export async function fetchPhotoDetail(
  id: number,
): Promise<PhotoDetailResponse> {
  const response = await fetch(`${API_BASE}/photos/${id}`);
  if (!response.ok) {
    throw new Error("Failed to fetch photo detail");
  }
  return response.json();
}

export function getPhotoThumbnailUrl(id: number, width?: number): string {
  const base = `${API_BASE}/photos/${id}/thumbnail`;
  return width ? `${base}?w=${width}` : base;
}

export function getPhotoImageUrl(id: number): string {
  return `${API_BASE}/photos/${id}/image`;
}

export async function fetchGallerySimilar(id: number, limit: number = 12): Promise<GallerySimilarResponse> {
  if (gallerySimilarNotFoundIds.has(id)) {
    return { pieces: [] };
  }
  const params = new URLSearchParams({ limit: limit.toString() });
  const response = await fetch(`${API_BASE}/gallery/${id}/similar?${params}`);
  if (response.status === 404) {
    gallerySimilarNotFoundIds.add(id);
    return { pieces: [] };
  }
  if (!response.ok) {
    throw new Error("Failed to fetch similar pieces");
  }
  return response.json();
}

export async function fetchRunPhotos(
  runId: number,
  limit: number = 100,
  offset: number = 0,
): Promise<RunPhotosResponse> {
  const params = new URLSearchParams({
    limit: limit.toString(),
    offset: offset.toString(),
  });
  const response = await fetch(`${API_BASE}/runs/${runId}/photos?${params}`);
  if (!response.ok) {
    throw new Error("Failed to fetch run photos");
  }
  return response.json();
}

export interface FavoriteStatusResponse {
  id: number;
  favorite: boolean;
  available: boolean;
  error?: string;
}

export async function getFavoriteStatus(photoId: number): Promise<FavoriteStatusResponse> {
  const response = await fetch(`${API_BASE}/photos/${photoId}/favorite`);
  if (!response.ok) {
    throw new Error("Failed to get favorite status");
  }
  return response.json();
}

export async function addToFavorites(photoId: number): Promise<void> {
  const response = await fetch(`${API_BASE}/photos/${photoId}/favorite`, {
    method: "POST",
  });
  if (!response.ok) {
    const data = await response.json();
    throw new Error(data.error || "Failed to add to favorites");
  }
  await clearOfflineCaches();
}

export async function removeFromFavorites(photoId: number): Promise<void> {
  const response = await fetch(`${API_BASE}/photos/${photoId}/favorite`, {
    method: "DELETE",
  });
  if (!response.ok) {
    const data = await response.json();
    throw new Error(data.error || "Failed to remove from favorites");
  }
  await clearOfflineCaches();
}

export interface CreatePieceInput {
  file: File;
  title: string;
  source: string;
  artist: string;
  date: string;
  notes: string;
}

export interface CreatePieceResponse {
  photo: Photo;
  duplicate: boolean;
}

export async function createPiece(input: CreatePieceInput): Promise<CreatePieceResponse> {
  const form = new FormData();
  form.set("file", input.file);
  form.set("title", input.title);
  form.set("source", input.source);
  form.set("artist", input.artist);
  form.set("date", input.date);
  form.set("notes", input.notes);

  const response = await fetch(`${API_BASE}/pieces`, {
    method: "POST",
    body: form,
  });
  if (!response.ok) {
    let message = "Failed to add piece";
    try {
      const data = await response.json();
      if (typeof data.error === "string") {
        message = data.error;
      }
    } catch {
      // Keep the generic message when the backend returns a non-JSON error.
    }
    throw new Error(message);
  }
  const result = await response.json();
  await clearOfflineCaches([API_GET_CACHE_NAME, PIECE_IMAGE_CACHE_NAME]);
  return result;
}
