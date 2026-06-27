package database

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"ok-folio/internal/config"
	"ok-folio/internal/testguard"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

// DefaultProvider is the OK Folio default media provider. Provider is set on
// INSERT only and is never written by the ETL backfill.
const DefaultProvider = "sight.photo"

// EmbeddingDim is the provisional embedding vector dimension. It is managed via
// a raw post-migrate Exec (not a Go-bound field) so the dimension can change
// with the model choice without a non-idempotent struct migration.
const EmbeddingDim = 512

// DownloadedPhoto represents a photo that has been downloaded.
//
// Types map onto OK Folio's own Postgres: text instead of varchar(191),
// timestamptz instead of datetime(3), bytea hashes, and a bigint identity PK so
// the loader can insert legacy ids verbatim.
type DownloadedPhoto struct {
	ID uint64 `gorm:"primarykey"`
	// URL carries NO btree index: long URLs can exceed Postgres' ~2704-byte
	// btree tuple limit and would fail AutoMigrate at boot. Uniqueness is
	// enforced on URLHash instead.
	URL string `gorm:"type:text;not null"`
	// URLHash is sha256(canonicalize(url)) as raw 32 bytes (NOT hex text). It is
	// NOT NULL and populated by the single BeforeSave hook so every insert path
	// fills it; the unique index here is the real duplicate guard.
	URLHash []byte `gorm:"type:bytea;not null;uniqueIndex"`
	// SourcePage stores the full source page URL (the scraper writes
	// resolved.Source.URL here), so it carries NO plain btree: like the raw url
	// column, a long URL can exceed Postgres' ~2704-byte btree tuple limit and
	// fail inserts during index maintenance. A bounded hash index (immune to that
	// limit) backs the equality lookups; it is created in postMigratePostgres.
	SourcePage string `gorm:"type:text"`
	Title      string `gorm:"type:text;index"`
	// Artist carries its own single-column index plus position 2 of the
	// (downloaded_at, artist) composite. Values are preserved byte-for-byte.
	Artist     string    `gorm:"type:text;index;index:idx_downloaded_photos_downloaded_at_artist,priority:2"`
	UploadDate time.Time `gorm:"index"`
	FilePath   string    `gorm:"type:text;default:''"`
	FileName   string    `gorm:"type:text;index"`
	// DownloadedAt is position 1 of the composite index; its leading column also
	// serves ORDER BY downloaded_at, so the redundant standalone downloaded_at
	// index is intentionally dropped.
	DownloadedAt time.Time `gorm:"autoCreateTime;index:idx_downloaded_photos_downloaded_at_artist,priority:1"`
	FileSize     int64
	// Favorite is OK Folio-owned and never written by the ETL.
	Favorite bool `gorm:"not null;default:false;index"`
	// Provider is set on INSERT only; it is text, NOT a Postgres ENUM.
	Provider string `gorm:"type:text;not null;default:'sight.photo';index"`
	// Category is derived once at write time (replacing the N+1 derive-from-URL
	// filter) and set on INSERT only.
	Category string `gorm:"type:text;index"`
	// ContentHash is raw 32 bytes for exact cross-source dedupe; OK Folio-owned.
	ContentHash []byte `gorm:"type:bytea;index"`
	// PerceptualHash is a 64-bit pHash stored as bigint to enable Hamming via
	// bit ops and to avoid a later non-idempotent ALTER from bytea.
	PerceptualHash int64  `gorm:"index"`
	Status         string `gorm:"type:text;index;default:'downloaded'"` // downloaded, failed, deleted, pending (transient)
	ErrorMessage   string `gorm:"type:text"`                            // Error message if status is 'failed'
}

// InboxItem is an ingestion exception that needs an operator decision.
type InboxItem struct {
	ID         uint64 `gorm:"primarykey"`
	ProviderID string `gorm:"type:text;index;not null"`
	DedupeKey  string `gorm:"type:text;index"`
	SourceID   string `gorm:"type:text;index"`
	MediaID    string `gorm:"type:text;index"`
	SourceURL  string `gorm:"type:text"`
	Title      string `gorm:"type:text"`
	Artist     string `gorm:"type:text"`
	Status     string `gorm:"type:text;index;not null"` // duplicate, ambiguous
	Reason     string `gorm:"type:text"`
	// Fingerprint is a stable identity used as the ON CONFLICT target so inbox
	// exceptions upsert atomically. It is populated by the BeforeSave hook.
	Fingerprint string    `gorm:"type:text;uniqueIndex"`
	CreatedAt   time.Time `gorm:"autoCreateTime"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime"`
}

// ExtractionRun tracks extraction job runs
type ExtractionRun struct {
	ID               uint64    `gorm:"primarykey"`
	StartTime        time.Time `gorm:"autoCreateTime"`
	EndTime          *time.Time
	Provider         string `gorm:"type:text;index"`
	Status           string `gorm:"type:text;index;default:'running'"` // running, completed, failed
	PagesProcessed   int
	PhotosFound      int
	PhotosDownloaded int
	PhotosSkipped    int
	PhotosFailed     int
	ErrorMessage     string `gorm:"type:text"`
}

// ConnectorState stores the latest durable sync state for one provider
// connector. It is keyed by provider_id so Streams can render connector status
// even when that connector has no catalog rows yet.
type ConnectorState struct {
	ProviderID   string     `gorm:"column:provider_id;primaryKey;type:text"`
	LastRunAt    *time.Time `gorm:"column:last_run_at"`
	LastStatus   string     `gorm:"column:last_status;type:text;index"`
	Cursor       string     `gorm:"column:cursor;type:text"`
	ErrorMessage string     `gorm:"column:error_message;type:text"`
	UpdatedAt    time.Time  `gorm:"autoUpdateTime"`
}

func (ConnectorState) TableName() string {
	return "connector_state"
}

// ETLWatermark stores legacy import progress in OK Folio Postgres. The legacy
// source remains read-only; incremental runs advance this row only after the
// loader transaction commits.
type ETLWatermark struct {
	Name          string    `gorm:"column:table_name;primaryKey;type:text"`
	LastID        uint64    `gorm:"not null;default:0"`
	LastTimestamp time.Time `gorm:"not null"`
	UpdatedAt     time.Time `gorm:"autoUpdateTime"`
}

func (ETLWatermark) TableName() string {
	return "etl_watermark"
}

// canonicalizeURL applies OK Folio's conservative V1 URL canonicalization
// before hashing. V1 only trims surrounding whitespace so that distinct URLs
// (and non-URL dedupe keys stored in the url column) never collide; richer
// canonicalization is deferred to avoid silently merging rows.
func canonicalizeURL(rawURL string) string {
	return strings.TrimSpace(rawURL)
}

// HashURL returns sha256(canonicalize(url)) as raw 32 bytes.
func HashURL(rawURL string) []byte {
	sum := sha256.Sum256([]byte(canonicalizeURL(rawURL)))
	return sum[:]
}

// BeforeSave populates the NOT NULL url_hash and the derived category from the
// single hook so every insert/update path is covered. A NULL url_hash would let
// duplicates slip the unique guard, so this must be the only place it is set.
func (p *DownloadedPhoto) BeforeSave(tx *gorm.DB) error {
	p.URLHash = HashURL(p.URL)
	p.Category = resolveCategory(p)
	if p.Provider == "" {
		p.Provider = DefaultProvider
	}
	return nil
}

// resolveCategory derives the stored category for a photo, mirroring the
// BeforeSave hook: an explicit Category wins, otherwise it is derived from the
// SourcePage. The conflict-update path uses it so a retry update writes the same
// derived category a fresh insert's hook would compute, instead of capturing the
// caller's empty Category before BeforeSave runs.
func resolveCategory(p *DownloadedPhoto) string {
	if p.Category != "" {
		return p.Category
	}
	return CategoryIDFromSourcePage(p.SourcePage)
}

// BeforeSave keeps the inbox fingerprint in sync so RecordInboxException can
// upsert atomically via ON CONFLICT.
func (i *InboxItem) BeforeSave(tx *gorm.DB) error {
	i.Fingerprint = inboxFingerprint(i)
	return nil
}

func inboxFingerprint(i *InboxItem) string {
	if i.DedupeKey != "" {
		return strings.Join([]string{"k", i.ProviderID, i.Status, i.DedupeKey}, "\x1f")
	}
	return strings.Join([]string{"t", i.ProviderID, i.Status, i.SourceID, i.MediaID, i.SourceURL}, "\x1f")
}

// IsUniqueViolation reports whether err is a unique-constraint violation. It
// detects Postgres errcode 23505 via pgconn.PgError and falls back to a string
// match for the sqlite test driver.
func IsUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique constraint") || strings.Contains(msg, "duplicate key")
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

// ConnectorError captures recent persisted connector failures.
type ConnectorError struct {
	ID           uint64    `gorm:"column:id" json:"id"`
	SourcePage   string    `gorm:"column:source_page" json:"source_page"`
	URL          string    `gorm:"column:url" json:"url"`
	Title        string    `gorm:"column:title" json:"title"`
	ErrorMessage string    `gorm:"column:error_message" json:"error_message"`
	OccurredAt   time.Time `gorm:"column:occurred_at" json:"occurred_at"`
}

// New opens OK Folio's own Postgres, tunes the pool, and migrates the schema.
//
// It refuses to start when DB_HOST points at the legacy MariaDB/MySQL host and
// asserts the resolved DSN and the live GORM dialect are Postgres, so a legacy
// DSN can never reach gorm.Open.
func New(cfg *config.DatabaseConfig) (*DB, error) {
	if err := testguard.GuardAppDatabaseConfig(*cfg); err != nil {
		return nil, err
	}

	dsn := cfg.DSN()
	if err := testguard.AssertNonLegacyDSN(dsn); err != nil {
		return nil, err
	}

	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	}

	db, err := gorm.Open(postgres.Open(dsn), gormConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	if name := db.Dialector.Name(); name != "postgres" {
		return nil, fmt.Errorf("refusing to start: expected postgres GORM dialect, got %q", name)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database instance: %w", err)
	}

	// Set connection pool settings. Default to a modest Postgres pool when the
	// config leaves these unset.
	maxOpen := cfg.MaxOpenConns
	if maxOpen <= 0 {
		maxOpen = 15
	}
	maxIdle := cfg.MaxIdleConns
	if maxIdle <= 0 {
		maxIdle = 5
	}
	sqlDB.SetMaxOpenConns(maxOpen)
	sqlDB.SetMaxIdleConns(maxIdle)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	if err := Migrate(db); err != nil {
		return nil, err
	}

	return &DB{db}, nil
}

// Migrate runs AutoMigrate on the owned models and, on Postgres, the
// non-destructive post-migrate steps (identity PK, embedding column). It is the
// single migration entry point so tests exercise the same path as boot.
func Migrate(db *gorm.DB) error {
	if err := db.AutoMigrate(&DownloadedPhoto{}, &ExtractionRun{}, &InboxItem{}, &ConnectorState{}, &ETLWatermark{}); err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}
	if db.Dialector.Name() == "postgres" {
		if err := postMigratePostgres(db); err != nil {
			return err
		}
	}
	return nil
}

// postMigratePostgres applies Postgres-only schema steps that AutoMigrate does
// not express. Every statement is idempotent and runs outside an explicit
// transaction. It never runs CREATE EXTENSION (that needs superuser and belongs
// to the stack's initdb) and never Fatals when the vector extension is absent.
func postMigratePostgres(db *gorm.DB) error {
	// Promote the bigserial PK to GENERATED BY DEFAULT AS IDENTITY so the ETL
	// loader can insert legacy ids verbatim. BY DEFAULT (not ALWAYS) keeps
	// explicit-id inserts allowed. Idempotent: only converts non-identity ids.
	identityStmts := []string{
		`DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM pg_attribute a
				JOIN pg_class c ON c.oid = a.attrelid
				WHERE c.relname = 'downloaded_photos' AND a.attname = 'id' AND a.attidentity <> ''
			) THEN
				ALTER TABLE downloaded_photos ALTER COLUMN id DROP DEFAULT;
				ALTER TABLE downloaded_photos ALTER COLUMN id ADD GENERATED BY DEFAULT AS IDENTITY;
			END IF;
		END $$;`,
		`DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM pg_attribute a
				JOIN pg_class c ON c.oid = a.attrelid
				WHERE c.relname = 'extraction_runs' AND a.attname = 'id' AND a.attidentity <> ''
			) THEN
				ALTER TABLE extraction_runs ALTER COLUMN id DROP DEFAULT;
				ALTER TABLE extraction_runs ALTER COLUMN id ADD GENERATED BY DEFAULT AS IDENTITY;
			END IF;
		END $$;`,
	}
	for _, stmt := range identityStmts {
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("post-migrate identity step failed: %w", err)
		}
	}

	// Index source_page with a hash index instead of a plain btree. source_page
	// holds full source URLs whose length can exceed the btree tuple limit and
	// fail inserts; a hash index stores only the 32-bit hash, so it is immune to
	// that limit while still serving the source equality lookups. Drop any legacy
	// btree of the GORM-default name first (older builds tagged the column
	// `index`), then create the hash index idempotently.
	sourcePageIndexStmts := []string{
		`DROP INDEX IF EXISTS idx_downloaded_photos_source_page`,
		`CREATE INDEX IF NOT EXISTS idx_downloaded_photos_source_page_hash ON downloaded_photos USING hash (source_page)`,
	}
	for _, stmt := range sourcePageIndexStmts {
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("post-migrate source_page index step failed: %w", err)
		}
	}

	// Add the embedding column only when the vector type exists. The app must
	// not CREATE EXTENSION; if the extension is missing this logs nothing and
	// continues rather than failing the boot.
	embeddingStmt := fmt.Sprintf(`DO $$
	BEGIN
		IF EXISTS (SELECT 1 FROM pg_type WHERE typname = 'vector') THEN
			ALTER TABLE downloaded_photos ADD COLUMN IF NOT EXISTS embedding vector(%d);
		END IF;
	END $$;`, EmbeddingDim)
	if err := db.Exec(embeddingStmt).Error; err != nil {
		return fmt.Errorf("post-migrate embedding step failed: %w", err)
	}

	return nil
}

// IsPhotoDownloaded checks if a photo URL has already been downloaded. The
// lookup is keyed on url_hash (never the raw url) via the shared hash helper.
func (db *DB) IsPhotoDownloaded(url string) (bool, error) {
	var count int64
	err := db.Model(&DownloadedPhoto{}).Where("url_hash = ? AND status = ?", HashURL(url), "downloaded").Count(&count).Error
	return count > 0, err
}

// RecordDownload atomically records a successful photo download.
//
// It upserts on the url_hash unique index: a fresh url inserts, and a row that
// is not yet downloaded (a previously failed/pending retry) is updated to
// downloaded. An already-downloaded row is left untouched and reported as a
// duplicate. ON CONFLICT makes this safe under the concurrent Ingestor where
// the old First-then-Create race would trip Postgres 23505.
func (db *DB) RecordDownload(photo *DownloadedPhoto) error {
	// Derive the category up front: this assignment map is built before GORM runs
	// BeforeSave, so reading photo.Category here would capture the caller's empty
	// value (the scraper relies on the hook deriving it from SourcePage) and a
	// retry update would regroup recovered downloads as "unknown".
	result := db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "url_hash"}},
		Where: clause.Where{Exprs: []clause.Expression{
			gorm.Expr("downloaded_photos.status <> ?", "downloaded"),
		}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"source_page":   photo.SourcePage,
			"title":         photo.Title,
			"artist":        photo.Artist,
			"category":      resolveCategory(photo),
			"upload_date":   photo.UploadDate,
			"file_path":     photo.FilePath,
			"file_name":     photo.FileName,
			"file_size":     photo.FileSize,
			"provider":      photo.Provider,
			"content_hash":  photo.ContentHash,
			"status":        photo.Status,
			"error_message": "",
		}),
	}).Create(photo)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("photo already downloaded: %s", photo.URL)
	}
	return nil
}

// RecordDownloadOrDuplicate atomically inserts a downloaded photo. If another
// row already owns the same url_hash (Postgres 23505), the loser is converted
// into an Inbox duplicate exception instead of overwriting the winner. It
// reports whether this caller won the insert.
func (db *DB) RecordDownloadOrDuplicate(photo *DownloadedPhoto, duplicate *InboxItem) (bool, error) {
	err := db.Create(photo).Error
	if err == nil {
		return true, nil
	}
	if IsUniqueViolation(err) {
		if duplicate != nil {
			if inboxErr := db.RecordInboxException(duplicate); inboxErr != nil {
				return false, inboxErr
			}
		}
		return false, nil
	}
	return false, err
}

// MarkPhotoFailed marks a photo download as failed, keyed on url_hash.
func (db *DB) MarkPhotoFailed(url, errorMsg string) error {
	return db.Model(&DownloadedPhoto{}).
		Where("url_hash = ?", HashURL(url)).
		Updates(map[string]interface{}{
			"status":        "failed",
			"error_message": errorMsg,
		}).Error
}

// RecordFailedDownload atomically records a failed download attempt, upserting
// on the url_hash unique index.
func (db *DB) RecordFailedDownload(url, errorMsg string) error {
	photo := &DownloadedPhoto{
		URL:          url,
		Status:       "failed",
		ErrorMessage: errorMsg,
	}
	return db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "url_hash"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"status":        "failed",
			"error_message": errorMsg,
		}),
	}).Create(photo).Error
}

// RecordInboxException records a duplicate or ambiguous ingest item for Inbox
// review. It upserts atomically on the stable fingerprint via ON CONFLICT, so a
// loser routed here under concurrency never trips a duplicate-key error.
func (db *DB) RecordInboxException(item *InboxItem) error {
	if item.Status != "duplicate" && item.Status != "ambiguous" {
		return fmt.Errorf("invalid inbox exception status: %s", item.Status)
	}
	if item.ProviderID == "" {
		return fmt.Errorf("provider ID is required")
	}

	return db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "fingerprint"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"source_id":  item.SourceID,
			"media_id":   item.MediaID,
			"source_url": item.SourceURL,
			"title":      item.Title,
			"artist":     item.Artist,
			"reason":     item.Reason,
		}),
	}).Create(item).Error
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

// StartExtractionRun creates a new extraction run record.
func (db *DB) StartExtractionRun(providerID ...string) (*ExtractionRun, error) {
	run := &ExtractionRun{
		Status: "running",
	}
	if len(providerID) > 0 {
		run.Provider = providerID[0]
	}
	err := db.Create(run).Error
	return run, err
}

// UpdateExtractionRun updates an extraction run
func (db *DB) UpdateExtractionRun(run *ExtractionRun) error {
	return db.Save(run).Error
}

// FinishExtractionRun marks an extraction run as completed
func (db *DB) FinishExtractionRun(runID uint64, status string, errorMsg string) error {
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

// GetRecentConnectorRuns returns the most recent extraction runs per provider.
// Empty legacy providers are grouped under webgallery so historical runs remain
// visible on the legacy connector while provider-attributed runs stay separate.
func (db *DB) GetRecentConnectorRuns(limitPerProvider int) ([]ExtractionRun, error) {
	if limitPerProvider <= 0 {
		return []ExtractionRun{}, nil
	}

	var runs []ExtractionRun
	err := db.Raw(`
		SELECT id, start_time, end_time, provider, status, pages_processed, photos_found,
			photos_downloaded, photos_skipped, photos_failed, error_message
		FROM (
			SELECT extraction_runs.*,
				ROW_NUMBER() OVER (
					PARTITION BY COALESCE(NULLIF(provider, ''), 'webgallery')
					ORDER BY start_time DESC, id DESC
				) AS row_num
			FROM extraction_runs
		) recent_runs
		WHERE row_num <= ?
		ORDER BY start_time DESC, id DESC
	`, limitPerProvider).Scan(&runs).Error
	return runs, err
}

// GetConnectorStates returns the latest durable state row for each connector.
func (db *DB) GetConnectorStates() ([]ConnectorState, error) {
	var states []ConnectorState
	err := db.Order("provider_id ASC").Find(&states).Error
	return states, err
}

// LoadConnectorState returns the durable cursor and status for a provider.
func (db *DB) LoadConnectorState(providerID string) (*ConnectorState, error) {
	if strings.TrimSpace(providerID) == "" {
		return nil, fmt.Errorf("provider ID is required")
	}

	var state ConnectorState
	err := db.Where("provider_id = ?", providerID).First(&state).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &state, nil
}

// SaveConnectorState upserts the durable cursor and last run outcome for a
// provider connector.
func (db *DB) SaveConnectorState(state ConnectorState) error {
	if strings.TrimSpace(state.ProviderID) == "" {
		return fmt.Errorf("provider ID is required")
	}

	return db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "provider_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"cursor":        state.Cursor,
			"last_run_at":   state.LastRunAt,
			"last_status":   state.LastStatus,
			"error_message": state.ErrorMessage,
		}),
	}).Create(&state).Error
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

// ResetPhotoStatus resets a photo's status to allow retry.
//
// 'pending' is an intentional transient status: the gallery filter, facets, and
// Streams enumerate downloaded/failed/deleted, so a reset row is deliberately
// hidden from those surfaces until a connector re-processes it back to
// downloaded or failed. The ETL reconcile set (owned by the ETL issue) must
// include 'pending' so reset rows are not lost during cutover.
func (db *DB) ResetPhotoStatus(photoID uint64) error {
	return db.DB.Model(&DownloadedPhoto{}).
		Where("id = ?", photoID).
		Update("status", "pending").Error
}

// SearchPhotos searches photos by title, artist, or filename
func (db *DB) SearchPhotos(query string, limit int, offset int) ([]DownloadedPhoto, int64, error) {
	var photos []DownloadedPhoto
	var total int64

	searchPattern := "%" + query + "%"

	// Get total count. Search is case-insensitive (ILIKE on Postgres).
	op := db.caseInsensitiveLike()
	countQuery := db.DB.Model(&DownloadedPhoto{}).
		Where("status = ?", "downloaded").
		Where("title "+op+" ? OR artist "+op+" ? OR file_name "+op+" ?", searchPattern, searchPattern, searchPattern)

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
func (db *DB) GetPhotoByID(id uint64) (*DownloadedPhoto, error) {
	var photo DownloadedPhoto
	err := db.DB.Where("id = ?", id).First(&photo).Error
	if err != nil {
		return nil, err
	}
	return &photo, nil
}

// SetPhotoFavorite persists the local OK Folio favorite state for a photo.
func (db *DB) SetPhotoFavorite(id uint64, favorite bool) error {
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
// On Postgres MAX(downloaded_at) is a timestamptz that scans natively into
// *time.Time, replacing the old CAST(... AS CHAR) + multi-layout parsing.
func (db *DB) GetConnectorSourceStats() ([]ConnectorSourceStats, error) {
	query := db.DB.Model(&DownloadedPhoto{}).
		Select("source_page, url, status, COUNT(*) as count, MAX(downloaded_at) as last_activity").
		Group("source_page, url, status").
		Order("source_page ASC, url ASC, status ASC")

	if db.Dialector.Name() != "postgres" {
		// The sqlite unit-test driver returns the aggregated time as a string
		// (an expression has no declared column type). Read it as text and parse
		// the single fixed layout gorm/sqlite writes; the native path above is
		// what production (Postgres) exercises.
		return scanConnectorSourceStatsSQLite(query)
	}

	var sources []ConnectorSourceStats
	if err := query.Scan(&sources).Error; err != nil {
		return nil, err
	}
	return sources, nil
}

func scanConnectorSourceStatsSQLite(query *gorm.DB) ([]ConnectorSourceStats, error) {
	var rows []struct {
		SourcePage   string  `gorm:"column:source_page"`
		URL          string  `gorm:"column:url"`
		Status       string  `gorm:"column:status"`
		Count        int64   `gorm:"column:count"`
		LastActivity *string `gorm:"column:last_activity"`
	}
	if err := query.Scan(&rows).Error; err != nil {
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
		if row.LastActivity != nil {
			parsed, err := time.Parse("2006-01-02 15:04:05.999999999-07:00", *row.LastActivity)
			if err != nil {
				return nil, fmt.Errorf("failed to parse sqlite test time %q: %w", *row.LastActivity, err)
			}
			stat.LastActivity = &parsed
		}
		sources = append(sources, stat)
	}
	return sources, nil
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

// galleryCategoryExpr resolves the category facet value from the stored
// category column, mapping NULL/empty to "unknown" so the legacy derive-from-URL
// fallback is no longer needed.
const galleryCategoryExpr = "COALESCE(NULLIF(category, ''), 'unknown')"

// GetGalleryCategoryStats returns category facets from the stored category column.
func (db *DB) GetGalleryCategoryStats() ([]GalleryFacetStats, error) {
	return db.GetGalleryCategoryStatsForFilters(GalleryCatalogFilters{})
}

// GetGalleryCategoryStatsForFilters returns category facets for the active
// gallery filter set, grouping on the indexed category column instead of the
// previous N+1 derive-from-URL scan.
func (db *DB) GetGalleryCategoryStatsForFilters(filters GalleryCatalogFilters) ([]GalleryFacetStats, error) {
	var categories []GalleryFacetStats

	query := db.DB.Model(&DownloadedPhoto{}).
		Select(galleryCategoryExpr+" as id, COUNT(*) as count").
		Where("status = ?", "downloaded")
	query, err := db.applyGalleryCatalogFilters(query, filters)
	if err != nil {
		return nil, err
	}

	err = query.
		Group(galleryCategoryExpr).
		Order("count DESC, id ASC").
		Scan(&categories).Error
	if err != nil {
		return nil, err
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

// GetGalleryFavoriteStatsForFilters returns favorite facets for the active
// gallery filter set. The favorite column is guaranteed by AutoMigrate, so it
// is referenced directly without the old runtime column probe.
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

	var favoriteCount int64
	favoriteQuery := db.DB.Model(&DownloadedPhoto{}).Where("status = ?", "downloaded")
	favoriteQuery, err = db.applyGalleryCatalogFilters(favoriteQuery, filters)
	if err != nil {
		return nil, err
	}
	if err := favoriteQuery.Where("favorite = ?", true).Count(&favoriteCount).Error; err != nil {
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
		// Match on the indexed category column (NULL/empty maps to "unknown"),
		// replacing the previous N+1 derive-from-URL scan.
		query = query.Where(galleryCategoryExpr+" = ?", filters.Category)
	}
	if filters.ArtistSet || filters.Artist != "" {
		query = query.Where("artist = ?", filters.Artist)
	}
	if filters.Favorite != nil {
		query = query.Where("favorite = ?", *filters.Favorite)
	}
	if filters.Query != "" {
		// Free-text search is case-insensitive (ILIKE on Postgres). The raw url
		// and source_page columns are intentionally excluded: they carry no
		// text-search index, so LIKE over either URL-shaped field would force a
		// full text scan on the real catalog.
		op := db.caseInsensitiveLike()
		searchPattern := "%" + escapeSQLLike(filters.Query) + "%"
		query = query.Where(
			"title "+op+" ? ESCAPE '\\' OR artist "+op+" ? ESCAPE '\\' OR file_name "+op+" ? ESCAPE '\\'",
			searchPattern,
			searchPattern,
			searchPattern,
		)
	}
	return query, nil
}

// caseInsensitiveLike returns the case-insensitive LIKE operator for the active
// dialect: ILIKE on Postgres, plain LIKE on the sqlite test driver (whose LIKE
// is already case-insensitive for ASCII).
func (db *DB) caseInsensitiveLike() string {
	if db.Dialector.Name() == "postgres" {
		return "ILIKE"
	}
	return "LIKE"
}

func escapeSQLLike(value string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(value)
}

// CategoryIDFromSourcePage derives the stored category from a provider source URL.
func CategoryIDFromSourcePage(sourcePage string) string {
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

// GetPhotosByRunID returns photos downloaded during a specific extraction run
func (db *DB) GetPhotosByRunID(runID uint64, limit int, offset int) ([]DownloadedPhoto, int64, error) {
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
