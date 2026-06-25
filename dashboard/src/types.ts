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
  DownloadedAt: string;
  FileSize: number;
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
  providers: GalleryProviderFacet[];
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

export interface PhotoDetailResponse {
  id: number;
  url: string;
  source_page: string;
  title: string;
  artist: string;
  upload_date: string;
  file_path: string;
  file_name: string;
  downloaded_at: string;
  file_size: number;
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
