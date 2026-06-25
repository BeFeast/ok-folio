package database

import (
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"time"

	"ok-folio/internal/config"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DownloadedPhoto represents a photo that has been downloaded
type DownloadedPhoto struct {
	ID           uint      `gorm:"primarykey"`
	URL          string    `gorm:"uniqueIndex;not null"`
	SourcePage   string    `gorm:"index"`
	Title        string    `gorm:"index"`
	Artist       string    `gorm:"index"`
	UploadDate   time.Time `gorm:"index"`
	FilePath     string    `gorm:"default:''"`
	FileName     string    `gorm:"index"`
	DownloadedAt time.Time `gorm:"autoCreateTime"`
	FileSize     int64
	Favorite     bool   `gorm:"index;default:false"`
	Status       string `gorm:"index;default:'downloaded'"` // downloaded, failed, deleted
	ErrorMessage string `gorm:"type:text"`                  // Error message if status is 'failed'
}

// InboxItem is an ingestion exception that needs an operator decision.
type InboxItem struct {
	ID         uint   `gorm:"primarykey"`
	ProviderID string `gorm:"index;not null"`
	DedupeKey  string `gorm:"index"`
	SourceID   string `gorm:"index"`
	MediaID    string `gorm:"index"`
	SourceURL  string
	Title      string
	Artist     string
	Status     string    `gorm:"index;not null"` // duplicate, ambiguous
	Reason     string    `gorm:"type:text"`
	CreatedAt  time.Time `gorm:"autoCreateTime"`
	UpdatedAt  time.Time `gorm:"autoUpdateTime"`
}

// ExtractionRun tracks extraction job runs
type ExtractionRun struct {
	ID               uint      `gorm:"primarykey"`
	StartTime        time.Time `gorm:"autoCreateTime"`
	EndTime          *time.Time
	Status           string `gorm:"index;default:'running'"` // running, completed, failed
	PagesProcessed   int
	PhotosFound      int
	PhotosDownloaded int
	PhotosSkipped    int
	PhotosFailed     int
	ErrorMessage     string `gorm:"type:text"`
}

type DB struct {
	*gorm.DB
}

// GalleryCatalogFilters narrows the OK Folio gallery catalog without
// coupling the gallery API to a specific provider storage implementation.
type GalleryCatalogFilters struct {
	Provider  string
	Source    string
	Category  string
	Artist    string
	ArtistSet bool
	Favorite  *bool
	Query     string
}

// GallerySourceStats summarizes downloaded media by provider source page.
type GallerySourceStats struct {
	SourcePage string `json:"source_page"`
	Count      int64  `json:"count"`
}

// GalleryFacetStats summarizes a catalog facet value.
type GalleryFacetStats struct {
	ID    string `json:"id"`
	Count int64  `json:"count"`
}

// GalleryFavoriteStats summarizes favorite and non-favorite catalog counts.
type GalleryFavoriteStats struct {
	Favorite bool  `json:"favorite"`
	Count    int64 `json:"count"`
}

// ConnectorSourceStats summarizes media state for a connector source.
type ConnectorSourceStats struct {
	SourcePage   string     `gorm:"column:source_page" json:"source_page"`
	URL          string     `gorm:"column:url" json:"url"`
	Status       string     `gorm:"column:status" json:"status"`
	Count        int64      `gorm:"column:count" json:"count"`
	LastActivity *time.Time `gorm:"column:last_activity" json:"last_activity"`
}

type connectorSourceStatsRow struct {
	SourcePage   string         `gorm:"column:source_page"`
	URL          string         `gorm:"column:url"`
	Status       string         `gorm:"column:status"`
	Count        int64          `gorm:"column:count"`
	LastActivity sql.NullString `gorm:"column:last_activity"`
}

// ConnectorError captures recent persisted connector failures.
type ConnectorError struct {
	ID           uint      `gorm:"column:id" json:"id"`
	SourcePage   string    `gorm:"column:source_page" json:"source_page"`
	URL          string    `gorm:"column:url" json:"url"`
	Title        string    `gorm:"column:title" json:"title"`
	ErrorMessage string    `gorm:"column:error_message" json:"error_message"`
	OccurredAt   time.Time `gorm:"column:occurred_at" json:"occurred_at"`
}

// New creates a new database connection
func New(cfg *config.DatabaseConfig) (*DB, error) {
	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	}

	db, err := gorm.Open(mysql.Open(cfg.DSN()), gormConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database instance: %w", err)
	}

	// Set connection pool settings
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	// Auto-migrate schemas
	if err := db.AutoMigrate(&DownloadedPhoto{}, &ExtractionRun{}, &InboxItem{}); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return &DB{db}, nil
}

// IsPhotoDownloaded checks if a photo URL has already been downloaded
func (db *DB) IsPhotoDownloaded(url string) (bool, error) {
	var count int64
	err := db.Model(&DownloadedPhoto{}).Where("url = ? AND status = ?", url, "downloaded").Count(&count).Error
	return count > 0, err
}

// RecordDownload records a successful photo download (create or update)
func (db *DB) RecordDownload(photo *DownloadedPhoto) error {
	var existing DownloadedPhoto
	result := db.DB.Where("url = ?", photo.URL).First(&existing)

	if result.Error == gorm.ErrRecordNotFound {
		// Create new record
		return db.Create(photo).Error
	} else if result.Error != nil {
		return result.Error
	}

	if existing.Status == "downloaded" {
		return fmt.Errorf("photo already downloaded: %s", photo.URL)
	}

	// Update existing record (handles retry of previously failed photos)
	return db.Model(&existing).Updates(map[string]interface{}{
		"source_page":   photo.SourcePage,
		"title":         photo.Title,
		"artist":        photo.Artist,
		"upload_date":   photo.UploadDate,
		"file_path":     photo.FilePath,
		"file_name":     photo.FileName,
		"file_size":     photo.FileSize,
		"status":        photo.Status,
		"error_message": "",
	}).Error
}

// MarkPhotoFailed marks a photo download as failed
func (db *DB) MarkPhotoFailed(url, errorMsg string) error {
	return db.Model(&DownloadedPhoto{}).
		Where("url = ?", url).
		Updates(map[string]interface{}{
			"status":        "failed",
			"error_message": errorMsg,
		}).Error
}

// RecordFailedDownload records a failed download attempt (create or update)
func (db *DB) RecordFailedDownload(url, errorMsg string) error {
	var existing DownloadedPhoto
	result := db.DB.Where("url = ?", url).First(&existing)

	if result.Error == gorm.ErrRecordNotFound {
		// Create new record for failed download
		photo := &DownloadedPhoto{
			URL:          url,
			Status:       "failed",
			ErrorMessage: errorMsg,
		}
		return db.Create(photo).Error
	} else if result.Error != nil {
		return result.Error
	}

	// Update existing record to failed status
	return db.Model(&existing).Updates(map[string]interface{}{
		"status":        "failed",
		"error_message": errorMsg,
	}).Error
}

// RecordInboxException records a duplicate or ambiguous ingest item for Inbox review.
func (db *DB) RecordInboxException(item *InboxItem) error {
	if item.Status != "duplicate" && item.Status != "ambiguous" {
		return fmt.Errorf("invalid inbox exception status: %s", item.Status)
	}
	if item.ProviderID == "" {
		return fmt.Errorf("provider ID is required")
	}

	var existing InboxItem
	query := db.DB.Where("provider_id = ? AND status = ?", item.ProviderID, item.Status)
	if item.DedupeKey != "" {
		query = query.Where("dedupe_key = ?", item.DedupeKey)
	} else {
		query = query.Where("dedupe_key = ? AND source_id = ? AND media_id = ? AND source_url = ?", "", item.SourceID, item.MediaID, item.SourceURL)
	}
	result := query.First(&existing)
	if result.Error == gorm.ErrRecordNotFound {
		return db.Create(item).Error
	}
	if result.Error != nil {
		return result.Error
	}

	return db.Model(&existing).Updates(map[string]interface{}{
		"source_id":  item.SourceID,
		"media_id":   item.MediaID,
		"source_url": item.SourceURL,
		"title":      item.Title,
		"artist":     item.Artist,
		"reason":     item.Reason,
	}).Error
}

// GetInboxExceptions returns only Inbox exception items.
func (db *DB) GetInboxExceptions(limit int, offset int) ([]InboxItem, int64, error) {
	var items []InboxItem
	var total int64

	query := db.DB.Model(&InboxItem{}).Where("status IN ?", []string{"duplicate", "ambiguous"})
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := query.Order("updated_at DESC, id DESC").Limit(limit).Offset(offset).Find(&items).Error; err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

// StartExtractionRun creates a new extraction run record
func (db *DB) StartExtractionRun() (*ExtractionRun, error) {
	run := &ExtractionRun{
		Status: "running",
	}
	err := db.Create(run).Error
	return run, err
}

// UpdateExtractionRun updates an extraction run
func (db *DB) UpdateExtractionRun(run *ExtractionRun) error {
	return db.Save(run).Error
}

// FinishExtractionRun marks an extraction run as completed
func (db *DB) FinishExtractionRun(runID uint, status string, errorMsg string) error {
	now := time.Now()
	updates := map[string]interface{}{
		"end_time": now,
		"status":   status,
	}
	if errorMsg != "" {
		updates["error_message"] = errorMsg
	}
	return db.Model(&ExtractionRun{}).Where("id = ?", runID).Updates(updates).Error
}

// GetRecentRuns returns the most recent extraction runs
func (db *DB) GetRecentRuns(limit int) ([]ExtractionRun, error) {
	var runs []ExtractionRun
	err := db.Order("start_time DESC").Limit(limit).Find(&runs).Error
	return runs, err
}

// GetDownloadStats returns download statistics using a single optimized query
func (db *DB) GetDownloadStats() (map[string]interface{}, error) {
	type StatsResult struct {
		TotalPhotos   int64 `gorm:"column:total_photos"`
		TotalSize     int64 `gorm:"column:total_size"`
		UniqueArtists int64 `gorm:"column:unique_artists"`
	}

	var result StatsResult
	err := db.Model(&DownloadedPhoto{}).
		Select(`
			COUNT(*) as total_photos,
			COALESCE(SUM(file_size), 0) as total_size,
			COUNT(DISTINCT artist) as unique_artists
		`).
		Where("status = ?", "downloaded").
		Scan(&result).Error

	if err != nil {
		return nil, err
	}

	stats := make(map[string]interface{})
	stats["total_photos"] = result.TotalPhotos
	stats["total_size_bytes"] = result.TotalSize
	stats["unique_artists"] = result.UniqueArtists

	if result.TotalPhotos > 0 {
		var latest DownloadedPhoto
		if err := db.Where("status = ?", "downloaded").
			Order("downloaded_at DESC").
			First(&latest).Error; err != nil {
			return nil, err
		}
		if !latest.DownloadedAt.IsZero() {
			stats["last_download"] = latest.DownloadedAt
		}
	}

	return stats, nil
}

// ArtistStats represents photo count per artist
type ArtistStats struct {
	Artist     string `json:"artist"`
	PhotoCount int    `json:"photo_count"`
	TotalSize  int64  `json:"total_size"`
}

// GetTopArtists returns artists with the most photos
func (db *DB) GetTopArtists(limit int) ([]ArtistStats, error) {
	var artists []ArtistStats

	err := db.DB.Model(&DownloadedPhoto{}).
		Select("artist, COUNT(*) as photo_count, SUM(file_size) as total_size").
		Where("status = ?", "downloaded").
		Group("artist").
		Order("photo_count DESC").
		Limit(limit).
		Scan(&artists).Error

	if err != nil {
		return nil, err
	}

	return artists, nil
}

// GetFailedPhotos returns photos that failed to download
func (db *DB) GetFailedPhotos(limit int) ([]DownloadedPhoto, error) {
	var photos []DownloadedPhoto

	err := db.DB.Where("status = ?", "failed").
		Order("downloaded_at DESC").
		Limit(limit).
		Find(&photos).Error

	if err != nil {
		return nil, err
	}

	return photos, nil
}

// ResetPhotoStatus resets a photo's status to allow retry
func (db *DB) ResetPhotoStatus(photoID uint) error {
	return db.DB.Model(&DownloadedPhoto{}).
		Where("id = ?", photoID).
		Update("status", "pending").Error
}

// SearchPhotos searches photos by title, artist, or filename
func (db *DB) SearchPhotos(query string, limit int, offset int) ([]DownloadedPhoto, int64, error) {
	var photos []DownloadedPhoto
	var total int64

	searchPattern := "%" + query + "%"

	// Get total count
	countQuery := db.DB.Model(&DownloadedPhoto{}).
		Where("status = ?", "downloaded").
		Where("title LIKE ? OR artist LIKE ? OR file_name LIKE ?", searchPattern, searchPattern, searchPattern)

	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Get paginated results
	err := countQuery.
		Limit(limit).
		Offset(offset).
		Order("downloaded_at DESC").
		Find(&photos).Error

	if err != nil {
		return nil, 0, err
	}

	return photos, total, nil
}

// GetPhotoByID returns a single photo by ID
func (db *DB) GetPhotoByID(id uint) (*DownloadedPhoto, error) {
	var photo DownloadedPhoto
	err := db.DB.Where("id = ?", id).First(&photo).Error
	if err != nil {
		return nil, err
	}
	return &photo, nil
}

// SetPhotoFavorite persists the local OK Folio favorite state for a photo.
func (db *DB) SetPhotoFavorite(id uint, favorite bool) error {
	return db.DB.Model(&DownloadedPhoto{}).
		Where("id = ?", id).
		Update("favorite", favorite).Error
}

// GetPhotosToday returns photos downloaded today
func (db *DB) GetPhotosToday(limit int, offset int) ([]DownloadedPhoto, int64, error) {
	var photos []DownloadedPhoto
	var total int64

	// Get start of today (midnight)
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	// Get total count
	countQuery := db.DB.Model(&DownloadedPhoto{}).
		Where("status = ? AND downloaded_at >= ?", "downloaded", startOfDay)

	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Get paginated results
	err := countQuery.
		Limit(limit).
		Offset(offset).
		Order("downloaded_at DESC").
		Find(&photos).Error

	if err != nil {
		return nil, 0, err
	}

	return photos, total, nil
}

// GetPhotosLastWeek returns photos downloaded in the last 7 days
func (db *DB) GetPhotosLastWeek(limit int, offset int) ([]DownloadedPhoto, int64, error) {
	var photos []DownloadedPhoto
	var total int64

	// Get 7 days ago
	weekAgo := time.Now().AddDate(0, 0, -7)

	// Get total count
	countQuery := db.DB.Model(&DownloadedPhoto{}).
		Where("status = ? AND downloaded_at >= ?", "downloaded", weekAgo)

	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Get paginated results
	err := countQuery.
		Limit(limit).
		Offset(offset).
		Order("downloaded_at DESC").
		Find(&photos).Error

	if err != nil {
		return nil, 0, err
	}

	return photos, total, nil
}

// GetGalleryCatalog returns downloaded photos for the OK Folio gallery.
func (db *DB) GetGalleryCatalog(limit int, offset int, filters GalleryCatalogFilters) ([]DownloadedPhoto, int64, error) {
	var photos []DownloadedPhoto
	var total int64

	query := db.DB.Model(&DownloadedPhoto{}).
		Where("status = ?", "downloaded")
	query, err := db.applyGalleryCatalogFilters(query, filters)
	if err != nil {
		return nil, 0, err
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err = query.
		Limit(limit).
		Offset(offset).
		Order("downloaded_at DESC, id DESC").
		Find(&photos).Error
	if err != nil {
		return nil, 0, err
	}

	return photos, total, nil
}

// GetGallerySourceStats returns provider source facets for downloaded media.
func (db *DB) GetGallerySourceStats() ([]GallerySourceStats, error) {
	return db.GetGallerySourceStatsForFilters(GalleryCatalogFilters{})
}

// GetGallerySourceStatsForFilters returns provider source facets for the active gallery filter set.
func (db *DB) GetGallerySourceStatsForFilters(filters GalleryCatalogFilters) ([]GallerySourceStats, error) {
	var sources []GallerySourceStats

	query := db.DB.Model(&DownloadedPhoto{}).
		Select("source_page, COUNT(*) as count").
		Where("status = ?", "downloaded")
	query, err := db.applyGalleryCatalogFilters(query, filters)
	if err != nil {
		return nil, err
	}

	err = query.
		Group("source_page").
		Order("count DESC, source_page ASC").
		Scan(&sources).Error
	if err != nil {
		return nil, err
	}

	return sources, nil
}

// GetConnectorSourceStats returns per-source media counts for Streams status.
func (db *DB) GetConnectorSourceStats() ([]ConnectorSourceStats, error) {
	var rows []connectorSourceStatsRow
	err := db.DB.Model(&DownloadedPhoto{}).
		Select("source_page, url, status, COUNT(*) as count, CAST(MAX(downloaded_at) AS CHAR) as last_activity").
		Group("source_page, url, status").
		Order("source_page ASC, url ASC, status ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	sources := make([]ConnectorSourceStats, 0, len(rows))
	for _, row := range rows {
		stat := ConnectorSourceStats{
			SourcePage: row.SourcePage,
			URL:        row.URL,
			Status:     row.Status,
			Count:      row.Count,
		}
		if row.LastActivity.Valid {
			lastActivity, err := parseDBTime(row.LastActivity.String)
			if err != nil {
				return nil, err
			}
			stat.LastActivity = &lastActivity
		}
		sources = append(sources, stat)
	}

	return sources, nil
}

func parseDBTime(value string) (time.Time, error) {
	layouts := []string{
		time.RFC3339Nano,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999Z07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05-07:00",
		"2006-01-02 15:04:05Z07:00",
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("failed to parse database time %q", value)
}

// GetRecentConnectorErrors returns failed media with operator-facing error details.
func (db *DB) GetRecentConnectorErrors(limit int) ([]ConnectorError, error) {
	var errors []ConnectorError

	err := db.DB.Model(&DownloadedPhoto{}).
		Select("id, source_page, url, title, error_message, downloaded_at as occurred_at").
		Where("status = ?", "failed").
		Order("downloaded_at DESC, id DESC").
		Limit(limit).
		Scan(&errors).Error
	if err != nil {
		return nil, err
	}

	return errors, nil
}

// GetGalleryCategoryStats returns category facets inferred from source URLs.
func (db *DB) GetGalleryCategoryStats() ([]GalleryFacetStats, error) {
	return db.GetGalleryCategoryStatsForFilters(GalleryCatalogFilters{})
}

// GetGalleryCategoryStatsForFilters returns category facets for the active gallery filter set.
func (db *DB) GetGalleryCategoryStatsForFilters(filters GalleryCatalogFilters) ([]GalleryFacetStats, error) {
	sources, err := db.GetGallerySourceStatsForFilters(filters)
	if err != nil {
		return nil, err
	}

	byCategory := make(map[string]int64)
	for _, source := range sources {
		byCategory[categoryIDFromSourcePage(source.SourcePage)] += source.Count
	}

	categories := make([]GalleryFacetStats, 0, len(byCategory))
	for id, count := range byCategory {
		categories = append(categories, GalleryFacetStats{ID: id, Count: count})
	}

	return categories, nil
}

// GetGalleryArtistStats returns artist facets for downloaded media.
func (db *DB) GetGalleryArtistStats() ([]GalleryFacetStats, error) {
	return db.GetGalleryArtistStatsForFilters(GalleryCatalogFilters{})
}

// GetGalleryArtistStatsForFilters returns artist facets for the active gallery filter set.
func (db *DB) GetGalleryArtistStatsForFilters(filters GalleryCatalogFilters) ([]GalleryFacetStats, error) {
	var artists []GalleryFacetStats

	query := db.DB.Model(&DownloadedPhoto{}).
		Select("artist as id, COUNT(*) as count").
		Where("status = ?", "downloaded")
	query, err := db.applyGalleryCatalogFilters(query, filters)
	if err != nil {
		return nil, err
	}

	err = query.
		Group("artist").
		Order("count DESC, artist ASC").
		Scan(&artists).Error
	if err != nil {
		return nil, err
	}

	return artists, nil
}

// GetGalleryFavoriteStats returns favorite facets for downloaded media.
func (db *DB) GetGalleryFavoriteStats() ([]GalleryFavoriteStats, error) {
	return db.GetGalleryFavoriteStatsForFilters(GalleryCatalogFilters{})
}

// GetGalleryFavoriteStatsForFilters returns favorite facets for the active gallery filter set.
func (db *DB) GetGalleryFavoriteStatsForFilters(filters GalleryCatalogFilters) ([]GalleryFavoriteStats, error) {
	var total int64
	totalQuery := db.DB.Model(&DownloadedPhoto{}).Where("status = ?", "downloaded")
	totalQuery, err := db.applyGalleryCatalogFilters(totalQuery, filters)
	if err != nil {
		return nil, err
	}
	if err := totalQuery.Count(&total).Error; err != nil {
		return nil, err
	}

	column, ok, err := db.galleryFavoriteColumn()
	if err != nil {
		return nil, err
	}
	if !ok {
		return []GalleryFavoriteStats{
			{Favorite: true, Count: 0},
			{Favorite: false, Count: total},
		}, nil
	}

	var favoriteCount int64
	favoriteQuery := db.DB.Model(&DownloadedPhoto{}).Where("status = ?", "downloaded")
	favoriteQuery, err = db.applyGalleryCatalogFilters(favoriteQuery, filters)
	if err != nil {
		return nil, err
	}
	if err := favoriteQuery.Where(column+" = ?", true).Count(&favoriteCount).Error; err != nil {
		return nil, err
	}

	return []GalleryFavoriteStats{
		{Favorite: true, Count: favoriteCount},
		{Favorite: false, Count: total - favoriteCount},
	}, nil
}

func (db *DB) applyGalleryCatalogFilters(query *gorm.DB, filters GalleryCatalogFilters) (*gorm.DB, error) {
	if filters.Provider != "" {
		provider := filters.Provider
		if provider == "unknown" {
			query = query.Where("source_page = ? OR source_page IS NULL", "")
		} else {
			escapedProvider := escapeSQLLike(provider)
			query = query.Where(
				"source_page = ? OR source_page LIKE ? ESCAPE '\\' OR source_page LIKE ? ESCAPE '\\' OR source_page LIKE ? ESCAPE '\\' OR source_page LIKE ? ESCAPE '\\'",
				provider,
				"https://"+escapedProvider+"/%",
				"http://"+escapedProvider+"/%",
				"https://www."+escapedProvider+"/%",
				"http://www."+escapedProvider+"/%",
			)
		}
	}
	if filters.Source != "" {
		query = query.Where("source_page = ?", filters.Source)
	}
	if filters.Category != "" {
		var err error
		query, err = db.applyGalleryCategoryFilter(query, filters.Category)
		if err != nil {
			return nil, err
		}
	}
	if filters.ArtistSet || filters.Artist != "" {
		query = query.Where("artist = ?", filters.Artist)
	}
	if filters.Favorite != nil {
		column, ok, err := db.galleryFavoriteColumn()
		if err != nil {
			return nil, err
		}
		if ok {
			query = query.Where(column+" = ?", *filters.Favorite)
		} else if *filters.Favorite {
			query = query.Where("1 = 0")
		}
	}
	if filters.Query != "" {
		searchPattern := "%" + escapeSQLLike(filters.Query) + "%"
		query = query.Where(
			"title LIKE ? ESCAPE '\\' OR artist LIKE ? ESCAPE '\\' OR file_name LIKE ? ESCAPE '\\' OR url LIKE ? ESCAPE '\\' OR source_page LIKE ? ESCAPE '\\'",
			searchPattern,
			searchPattern,
			searchPattern,
			searchPattern,
			searchPattern,
		)
	}
	return query, nil
}

func escapeSQLLike(value string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(value)
}

func (db *DB) applyGalleryCategoryFilter(query *gorm.DB, category string) (*gorm.DB, error) {
	var candidates []DownloadedPhoto
	err := db.DB.Model(&DownloadedPhoto{}).
		Select("id, source_page").
		Where("status = ?", "downloaded").
		Find(&candidates).Error
	if err != nil {
		return nil, err
	}

	ids := make([]uint, 0)
	for _, candidate := range candidates {
		if categoryIDFromSourcePage(candidate.SourcePage) == category {
			ids = append(ids, candidate.ID)
		}
	}
	if len(ids) == 0 {
		return query.Where("1 = 0"), nil
	}
	return query.Where("id IN ?", ids), nil
}

func categoryIDFromSourcePage(sourcePage string) string {
	if sourcePage == "" {
		return "unknown"
	}
	parsed, err := url.Parse(sourcePage)
	if err != nil {
		return "unknown"
	}

	parts := strings.Split(strings.Trim(parsed.EscapedPath(), "/"), "/")
	for i := 0; i < len(parts)-1; i++ {
		if strings.EqualFold(parts[i], "category") && parts[i+1] != "" {
			return parts[i+1]
		}
	}

	query := parsed.Query()
	for _, key := range []string{"category", "category_id", "cat"} {
		if value := strings.TrimSpace(query.Get(key)); value != "" {
			return value
		}
	}

	return "unknown"
}

func (db *DB) galleryFavoriteColumn() (string, bool, error) {
	columnTypes, err := db.DB.Migrator().ColumnTypes(&DownloadedPhoto{})
	if err != nil {
		return "", false, err
	}

	available := make(map[string]string, len(columnTypes))
	for _, columnType := range columnTypes {
		name := strings.ToLower(columnType.Name())
		available[name] = columnType.Name()
	}

	for _, candidate := range []string{"favorite", "favorites", "is_favorite", "is_favourite"} {
		if column, ok := available[candidate]; ok {
			return column, true, nil
		}
	}
	return "", false, nil
}

// GetPhotosByRunID returns photos downloaded during a specific extraction run
func (db *DB) GetPhotosByRunID(runID uint, limit int, offset int) ([]DownloadedPhoto, int64, error) {
	var photos []DownloadedPhoto
	var total int64

	// Get the extraction run to find the time window
	var run ExtractionRun
	if err := db.DB.First(&run, runID).Error; err != nil {
		return nil, 0, err
	}

	// If run hasn't ended yet, use current time
	endTime := time.Now()
	if run.EndTime != nil {
		endTime = *run.EndTime
	}

	// Get total count
	countQuery := db.DB.Model(&DownloadedPhoto{}).
		Where("status = ? AND downloaded_at >= ? AND downloaded_at <= ?", "downloaded", run.StartTime, endTime)

	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Get paginated results
	err := countQuery.
		Limit(limit).
		Offset(offset).
		Order("downloaded_at DESC").
		Find(&photos).Error

	if err != nil {
		return nil, 0, err
	}

	return photos, total, nil
}

// GetAllArtists returns all artists with pagination
func (db *DB) GetAllArtists(limit int, offset int, sortBy string) ([]ArtistStats, int64, error) {
	var artists []ArtistStats
	var total int64

	// Get total count of unique artists
	if err := db.DB.Model(&DownloadedPhoto{}).
		Where("status = ?", "downloaded").
		Distinct("artist").
		Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Determine sort order
	orderClause := "photo_count DESC"
	switch sortBy {
	case "name":
		orderClause = "artist ASC"
	case "size":
		orderClause = "total_size DESC"
	}

	// Get paginated artists
	err := db.DB.Model(&DownloadedPhoto{}).
		Select("artist, COUNT(*) as photo_count, SUM(file_size) as total_size").
		Where("status = ?", "downloaded").
		Group("artist").
		Order(orderClause).
		Limit(limit).
		Offset(offset).
		Scan(&artists).Error

	if err != nil {
		return nil, 0, err
	}

	return artists, total, nil
}

// GetPhotosByArtist returns all photos by a specific artist
func (db *DB) GetPhotosByArtist(artist string, limit int, offset int) ([]DownloadedPhoto, int64, error) {
	var photos []DownloadedPhoto
	var total int64

	// Get total count for this artist
	countQuery := db.DB.Model(&DownloadedPhoto{}).
		Where("artist = ? AND status = ?", artist, "downloaded")

	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Get paginated results
	err := countQuery.
		Limit(limit).
		Offset(offset).
		Order("downloaded_at DESC").
		Find(&photos).Error

	if err != nil {
		return nil, 0, err
	}

	return photos, total, nil
}
