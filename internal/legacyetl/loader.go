package legacyetl

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ok-folio/internal/catalogquality"
	"ok-folio/internal/database"

	"gorm.io/gorm"
)

type LoadOptions struct {
	LegacyTimeZone   string
	SetSequences     bool
	AdvanceWatermark bool
}

type LoadResult struct {
	DownloadedPhotos int
	ExtractionRuns   int
	PhotoMaxID       uint64
	RunMaxID         uint64
	PhotoMaxTime     time.Time
	RunMaxTime       time.Time
}

const downloadedPhotoUpsertSQL = `
		INSERT INTO downloaded_photos (
			id, url, url_hash, source_page, title, artist, upload_date, file_path,
			file_name, downloaded_at, file_size, status, error_message, provider, category
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (url_hash) DO UPDATE SET
			source_page = EXCLUDED.source_page,
			title = EXCLUDED.title,
			artist = EXCLUDED.artist,
			file_name = EXCLUDED.file_name,
			upload_date = EXCLUDED.upload_date,
			file_path = EXCLUDED.file_path,
			file_size = EXCLUDED.file_size,
			status = EXCLUDED.status,
			error_message = EXCLUDED.error_message,
			downloaded_at = EXCLUDED.downloaded_at
		RETURNING downloaded_at`

const extractionRunUpsertSQL = `
		INSERT INTO extraction_runs (
			id, start_time, end_time, status, pages_processed, photos_found,
			photos_downloaded, photos_skipped, photos_failed, error_message
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET
			end_time = EXCLUDED.end_time,
			status = EXCLUDED.status,
			pages_processed = EXCLUDED.pages_processed,
			photos_found = EXCLUDED.photos_found,
			photos_downloaded = EXCLUDED.photos_downloaded,
			photos_skipped = EXCLUDED.photos_skipped,
			photos_failed = EXCLUDED.photos_failed,
			error_message = EXCLUDED.error_message
		RETURNING start_time`

func LoadDump(db *database.DB, rows DumpRows, opts LoadOptions) (LoadResult, error) {
	if opts.LegacyTimeZone == "" {
		return LoadResult{}, fmt.Errorf("legacy timezone is required; verify the source zone before loading naive datetime values")
	}
	var result LoadResult
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT set_config('TimeZone', ?, true)", opts.LegacyTimeZone).Error; err != nil {
			return fmt.Errorf("set loader timezone: %w", err)
		}
		for _, row := range rows.DownloadedPhotos {
			loadedAt, err := upsertDownloadedPhoto(tx, row)
			if err != nil {
				return err
			}
			result.DownloadedPhotos++
			if row.ID > result.PhotoMaxID {
				result.PhotoMaxID = row.ID
			}
			if loadedAt.After(result.PhotoMaxTime) {
				result.PhotoMaxTime = loadedAt
			}
		}
		for _, row := range rows.ExtractionRuns {
			startedAt, err := upsertExtractionRun(tx, row)
			if err != nil {
				return err
			}
			result.ExtractionRuns++
			if row.ID > result.RunMaxID {
				result.RunMaxID = row.ID
			}
			if startedAt.After(result.RunMaxTime) {
				result.RunMaxTime = startedAt
			}
		}
		if opts.SetSequences {
			if err := setSequences(tx); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return LoadResult{}, err
	}
	if opts.AdvanceWatermark {
		if result.DownloadedPhotos > 0 {
			if err := advanceWatermark(db.DB, DownloadedPhotosTable, result.PhotoMaxID, result.PhotoMaxTime); err != nil {
				return LoadResult{}, err
			}
		}
		if result.ExtractionRuns > 0 {
			if err := advanceWatermark(db.DB, ExtractionRunsTable, result.RunMaxID, result.RunMaxTime); err != nil {
				return LoadResult{}, err
			}
		}
	}
	return result, nil
}

func upsertDownloadedPhoto(tx *gorm.DB, row LegacyDownloadedPhoto) (time.Time, error) {
	var loadedAt sql.NullTime
	title := catalogquality.NormalizeTitle(row.Title, row.FileName, filepath.Base(row.FilePath))
	err := tx.Raw(downloadedPhotoUpsertSQL,
		row.ID,
		row.URL,
		database.HashURL(row.URL),
		row.SourcePage,
		title,
		row.Artist,
		nullableDatetimePtr(row.UploadDate),
		row.FilePath,
		row.FileName,
		nullableDatetime(row.DownloadedAt),
		row.FileSize,
		row.Status,
		row.ErrorMessage,
		database.DefaultProvider,
		database.CategoryIDFromSourcePage(row.SourcePage),
	).Row().Scan(&loadedAt)
	if err != nil {
		return time.Time{}, fmt.Errorf("upsert downloaded_photos legacy id %d: %w", row.ID, err)
	}
	return loadedAt.Time, nil
}

func upsertExtractionRun(tx *gorm.DB, row LegacyExtractionRun) (time.Time, error) {
	var startedAt sql.NullTime
	err := tx.Raw(extractionRunUpsertSQL,
		row.ID,
		nullableDatetime(row.StartTime),
		nullableDatetimePtr(row.EndTime),
		row.Status,
		row.PagesProcessed,
		row.PhotosFound,
		row.PhotosDownloaded,
		row.PhotosSkipped,
		row.PhotosFailed,
		row.ErrorMessage,
	).Row().Scan(&startedAt)
	if err != nil {
		return time.Time{}, fmt.Errorf("upsert extraction_runs legacy id %d: %w", row.ID, err)
	}
	return startedAt.Time, nil
}

// nullableDatetime coerces empty or MySQL zero-value datetime strings to a SQL
// NULL so legacy rows that carry absent timestamps are not rejected by Postgres
// timestamptz columns.
func nullableDatetime(s string) any {
	t := strings.TrimSpace(s)
	if t == "" || strings.HasPrefix(t, "0000-00-00") {
		return nil
	}
	return s
}

func nullableDatetimePtr(s *string) any {
	if s == nil {
		return nil
	}
	return nullableDatetime(*s)
}

func setSequences(tx *gorm.DB) error {
	for _, table := range []string{DownloadedPhotosTable, ExtractionRunsTable} {
		stmt, err := restartIdentitySequenceSQL(table)
		if err != nil {
			return err
		}
		if err := tx.Exec(stmt).Error; err != nil {
			return fmt.Errorf("restart %s id identity sequence: %w", table, err)
		}
	}
	return nil
}

func restartIdentitySequenceSQL(table string) (string, error) {
	if !isOwnedTable(table) {
		return "", fmt.Errorf("refusing to restart id identity for non-owned table %q", table)
	}
	return fmt.Sprintf(`
		DO $$
		DECLARE
			next_id bigint;
		BEGIN
			SELECT COALESCE(MAX(id), 1) + 1 INTO next_id FROM %s;
			EXECUTE format('ALTER TABLE %%I ALTER COLUMN id RESTART WITH %%s', %s, next_id);
		END $$;`,
		table,
		sqlLiteral(table),
	), nil
}

func sqlLiteral(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func advanceWatermark(db *gorm.DB, table string, lastID uint64, lastTimestamp time.Time) error {
	return db.Exec(`
		INSERT INTO etl_watermark (table_name, last_id, last_timestamp, updated_at)
		VALUES (?, ?, ?, now())
		ON CONFLICT (table_name) DO UPDATE SET
			last_id = EXCLUDED.last_id,
			last_timestamp = EXCLUDED.last_timestamp,
			updated_at = EXCLUDED.updated_at`,
		table,
		lastID,
		lastTimestamp,
	).Error
}

type HashResult struct {
	Scanned int
	Updated int
	Skipped int
}

// FillMissingContentHashes is a decoupled, idempotent pass over OK Folio rows.
// It reads bytes from the read-only originals mount and writes only the raw
// 32-byte sha256 content_hash in Postgres.
func FillMissingContentHashes(db *database.DB, originalsRoot string, limit int) (HashResult, error) {
	if originalsRoot == "" {
		return HashResult{}, fmt.Errorf("originals root is required")
	}
	if limit <= 0 {
		limit = 500
	}
	type pendingPhoto struct {
		ID       uint64
		FilePath string
	}
	var rows []pendingPhoto
	if err := db.Raw(`
		SELECT id, file_path
		FROM downloaded_photos
		WHERE content_hash IS NULL AND file_path <> '' AND COALESCE(status, '') <> 'deleted'
		ORDER BY id
		LIMIT ?`, limit).Scan(&rows).Error; err != nil {
		return HashResult{}, err
	}
	result := HashResult{Scanned: len(rows)}
	for _, row := range rows {
		path := resolveOriginalPath(originalsRoot, row.FilePath)
		data, err := os.ReadFile(path)
		if err != nil {
			result.Skipped++
			continue
		}
		sum := sha256.Sum256(data)
		if err := db.Exec("UPDATE downloaded_photos SET content_hash = ? WHERE id = ? AND content_hash IS NULL", sum[:], row.ID).Error; err != nil {
			if database.IsUniqueViolation(err) {
				if markErr := markContentHashDuplicateLoser(db, row.ID); markErr != nil {
					return result, markErr
				}
				result.Skipped++
				continue
			}
			return result, err
		}
		result.Updated++
	}
	return result, nil
}

func markContentHashDuplicateLoser(db *database.DB, id uint64) error {
	return db.Exec(`
		UPDATE downloaded_photos
		SET
			status = 'deleted',
			error_message = 'quarantined as exact content hash duplicate during backfill'
		WHERE id = ? AND content_hash IS NULL`, id).Error
}

func resolveOriginalPath(root, storedPath string) string {
	if filepath.IsAbs(storedPath) {
		return storedPath
	}
	return filepath.Join(root, storedPath)
}
