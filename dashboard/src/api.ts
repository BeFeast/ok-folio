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
  PhotoDetailResponse,
  RunPhotosResponse,
} from "./types";

const API_BASE = "/api/v1";

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
}

export async function triggerPageExtraction(page: number): Promise<void> {
  const response = await fetch(`${API_BASE}/extract/page/${page}`, {
    method: "POST",
  });
  if (!response.ok) {
    throw new Error("Failed to trigger page extraction");
  }
}

export async function triggerPagesExtraction(count: number): Promise<void> {
  const response = await fetch(`${API_BASE}/extract/pages?count=${count}`, {
    method: "POST",
  });
  if (!response.ok) {
    throw new Error("Failed to trigger pages extraction");
  }
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

export async function triggerPhotoprismIndex(): Promise<void> {
  const response = await fetch(`${API_BASE}/photoprism/index`, {
    method: "POST",
  });
  if (!response.ok) {
    throw new Error("Failed to trigger PhotoPrism indexing");
  }
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
}

export async function removeFromFavorites(photoId: number): Promise<void> {
  const response = await fetch(`${API_BASE}/photos/${photoId}/favorite`, {
    method: "DELETE",
  });
  if (!response.ok) {
    const data = await response.json();
    throw new Error(data.error || "Failed to remove from favorites");
  }
}
