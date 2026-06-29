export interface Stats {
  total_photos: number;
  unique_artists: number;
  total_size_bytes: number;
  last_download: string;
}

export interface ExtractionRun {
  ID: number;
  StartTime: string;
  EndTime: string;
  Status: string;
  PagesProcessed: number;
  PhotosFound: number;
  PhotosDownloaded: number;
  PhotosSkipped: number;
  PhotosFailed: number;
  ErrorMessage: string;
}

export interface RunsResponse {
  runs: ExtractionRun[];
}

export interface HealthResponse {
  status: string;
  database: string;
  time: string;
}

export interface WorkerStatus {
  total_workers: number;
  queue_size: number;
  queue_capacity: number;
  workers_busy: number;
  workers_idle: number;
  queue_utilization: number;
}

export interface ConnectorStatusResponse {
  connectors: ConnectorStatus[];
}

export interface ConnectorStatus {
  id: string;
  display_name: string;
  health: "healthy" | "degraded" | "error" | "syncing" | "idle";
  state: string;
  last_sync: string | null;
  counts: ConnectorCounts;
  sources: ConnectorSourceStatus[];
  recent_runs: ConnectorRunStatus[];
  recent_errors: ConnectorErrorStatus[];
}

export interface ConnectorCounts {
  downloaded: number;
  failed: number;
  pending: number;
  total: number;
}

export interface ConnectorSourceStatus {
  id: string;
  display_name: string;
  provider_id: string;
  last_sync: string | null;
  counts: ConnectorCounts;
}

export interface ConnectorSourceSetting {
  id: number;
  type: string;
  chat_id: string;
  label: string;
  config?: WebGalleryConfig | null;
  enabled: boolean;
  target_folio_id?: number | null;
  show_in_library: boolean;
  last_error?: string;
  last_seen_at?: string | null;
  created_at: string;
  updated_at: string;
}

export interface ConnectorSourcesResponse {
  sources: ConnectorSourceSetting[];
}

export interface ConnectorSourceInput {
  type: string;
  chat_id?: string;
  label?: string;
  config?: WebGalleryConfig;
  enabled?: boolean;
  target_folio_id?: number | null;
  show_in_library?: boolean;
}

export interface ConnectorSourceBackfillResult {
  matched: number;
  updated: number;
  added_to_folio: number;
  target_folio_id?: number | null;
  show_in_library: boolean;
}

export interface WebGalleryConfig {
  list_url: string;
  pagination: WebGalleryPaginationConfig;
  selectors: WebGallerySelectorConfig;
  item_link_filter: string[];
  schedule?: string;
}

export interface WebGalleryPaginationConfig {
  strategy: "page_param" | "next_link" | "none";
  param_name?: string;
  start_index?: number;
  next_link_selector?: string;
}

export interface WebGallerySelectorConfig {
  item_link: string;
  image: WebGalleryFieldSelector;
  artist?: WebGalleryFieldSelector;
  title?: WebGalleryFieldSelector;
  date?: WebGalleryFieldSelector;
}

export interface WebGalleryFieldSelector {
  selector: string;
  attr?: string;
}

export interface ConnectorSourcePreviewInput {
  config: WebGalleryConfig;
  page?: number;
  limit?: number;
  cursor?: string;
}

export interface ConnectorSourcePreviewResponse {
  provider: string;
  page: number;
  items_found: number;
  has_next: boolean;
  next_page?: number;
  next_cursor?: string;
  sample: ConnectorSourcePreviewSample[];
}

export interface ConnectorSourcePreviewSample {
  source_url: string;
  image_url: string;
  title?: string;
  artist?: string;
  date?: string;
}

export interface ConnectorRunStatus {
  id: number;
  start_time: string;
  end_time: string | null;
  status: string;
  pages_processed: number;
  photos_found: number;
  photos_downloaded: number;
  photos_skipped: number;
  photos_failed: number;
  error_message?: string;
}

export interface ConnectorErrorStatus {
  id: string;
  source_id: string;
  source: string;
  title: string;
  message: string;
  occurred_at: string;
}

export interface TimelineEntry {
  date: string;
  downloaded: number;
  skipped: number;
  failed: number;
  runs: number;
}

export interface TimelineResponse {
  timeline: TimelineEntry[];
  period: string;
  days: number;
}

export interface ArtistStats {
  artist: string;
  photo_count: number;
  total_size: number;
}

export interface TopArtistsResponse {
  artists: ArtistStats[];
  limit: number;
}

export interface FailedPhoto {
  ID: number;
  URL: string;
  Artist: string;
  Title: string;
  UploadDate: string;
  FilePath: string;
  FileSize: number;
  Status: string;
  CreatedAt: string;
  DownloadedAt: string;
}

export interface FailedPhotosResponse {
  photos: FailedPhoto[];
  count: number;
}

export interface Photo {
  ID: number;
  URL: string;
  SourcePage: string;
  Title: string;
  Artist: string;
  UploadDate: string;
  FilePath: string;
  FileName: string;
  ImageWidth: number;
  ImageHeight: number;
  CapturedAt: string | null;
  CameraMake: string;
  CameraModel: string;
  LensModel: string;
  Orientation: string;
  GPSLatitude: number | null;
  GPSLongitude: number | null;
  DownloadedAt: string;
  FileSize: number;
  Notes: string;
  keywords: string[];
  manual_fields: string[];
  Favorite: boolean;
  Status: string;
}

export interface SearchResponse {
  photos: Photo[];
  total: number;
  limit: number;
  offset: number;
  query: string;
}

export interface ArtistsResponse {
  artists: ArtistStats[];
  total: number;
  limit: number;
  offset: number;
  sort: string;
}

export interface ArtistDetailResponse {
  artist: string;
  photos: Photo[];
  total: number;
  limit: number;
  offset: number;
}

export interface TodayPhotosResponse {
  photos: Photo[];
  total: number;
  limit: number;
  offset: number;
  date: string;
}

export interface WeekPhotosResponse {
  photos: Photo[];
  total: number;
  limit: number;
  offset: number;
  days: number;
}

export interface GalleryCatalogResponse {
  photos: Photo[];
  total: number;
  limit: number;
  offset: number;
  provider: string;
  source: string;
  category: string;
  artist: string;
  favorite: boolean | null;
  query: string;
  providers: GalleryProviderFacet[];
  facets: GalleryCatalogFacets;
}

export interface GalleryProviderFacet {
  id: string;
  display_name: string;
  count: number;
  sources: GallerySourceFacet[];
}

export interface GallerySourceFacet {
  id: string;
  display_name: string;
  count: number;
}

export interface GalleryFacet {
  id: string;
  display_name: string;
  count: number;
}

export interface GalleryFavoriteFacet {
  id: string;
  display_name: string;
  favorite: boolean;
  count: number;
}

export interface GalleryCatalogFacets {
  sources: GallerySourceFacet[];
  categories: GalleryFacet[];
  artists: GalleryFacet[];
  favorites: GalleryFavoriteFacet[];
}

export interface PhotoDetailResponse {
  id: number;
  url: string;
  source_page: string;
  source: string;
  provider: string;
  category: string;
  title: string;
  artist: string;
  keywords: string[];
  upload_date: string;
  file_path: string;
  file_name: string;
  downloaded_at: string;
  file_size: number;
  favorite: boolean;
  status: string;
  file_mod_time: string;
}

export interface RunPhotosResponse {
  photos: Photo[];
  total: number;
  limit: number;
  offset: number;
  run_id: number;
}

export interface Folio {
  id: number;
  name: string;
  cover_photo_id: number | null;
  piece_count: number;
  created_at: string;
  updated_at: string;
}

export interface FoliosResponse {
  folios: Folio[];
}

export interface FolioPiecesResponse {
  photos: Photo[];
  total: number;
  limit: number;
  offset: number;
}

export interface InboxItem {
  id: number;
  provider_id: string;
  dedupe_key: string;
  source_id: string;
  media_id: string;
  source_url: string;
  title: string;
  artist: string;
  status: "duplicate" | "ambiguous";
  reason: string;
  cover_photo_id: number | null;
  created_at: string;
  updated_at: string;
}

export interface InboxResponse {
  items: InboxItem[];
  total: number;
  limit: number;
  offset: number;
}

export interface InboxCountsResponse {
  counts: {
    duplicate: number;
    ambiguous: number;
  };
  total: number;
}
