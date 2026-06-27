package legacyetl

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ok-folio/internal/database"

	postgresdriver "gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestDownloadedPhotoUpsertContract(t *testing.T) {
	sql := normalizeSQL(downloadedPhotoUpsertSQL)
	if !strings.Contains(sql, "ON CONFLICT (url_hash) DO UPDATE SET") {
		t.Fatalf("downloaded_photos must upsert on url_hash: %s", sql)
	}
	for _, mutable := range []string{
		"source_page", "title", "artist", "file_name", "upload_date",
		"file_path", "file_size", "status", "error_message", "downloaded_at",
	} {
		if !strings.Contains(sql, mutable+" = EXCLUDED."+mutable) {
			t.Fatalf("expected mutable legacy field %s in SET list: %s", mutable, sql)
		}
	}
	setList := sql[strings.Index(sql, " DO UPDATE SET "):strings.Index(sql, " RETURNING ")]
	for _, owned := range []string{"favorite", "content_hash", "perceptual_hash", "embedding", "provider", "category"} {
		if strings.Contains(setList, owned) {
			t.Fatalf("%s must never be in downloaded_photos SET list: %s", owned, setList)
		}
	}
	if !strings.Contains(sql, "provider") {
		t.Fatalf("provider must be stamped on insert: %s", sql)
	}
	if !strings.Contains(sql, "provider, category") {
		t.Fatalf("category must be derived and stamped on insert: %s", sql)
	}
}

func TestExtractionRunUpsertContract(t *testing.T) {
	sql := normalizeSQL(extractionRunUpsertSQL)
	if !strings.Contains(sql, "ON CONFLICT (id) DO UPDATE SET") {
		t.Fatalf("extraction_runs must upsert on id: %s", sql)
	}
	for _, mutable := range []string{
		"end_time", "status", "pages_processed", "photos_found",
		"photos_downloaded", "photos_skipped", "photos_failed", "error_message",
	} {
		if !strings.Contains(sql, mutable+" = EXCLUDED."+mutable) {
			t.Fatalf("expected run field %s in SET list: %s", mutable, sql)
		}
	}
	setList := sql[strings.Index(sql, " DO UPDATE SET "):strings.Index(sql, " RETURNING ")]
	if strings.Contains(setList, "start_time") {
		t.Fatalf("start_time is insert-only for extraction_runs: %s", setList)
	}
}

func TestRestartIdentitySequenceSQLUsesColumnIdentity(t *testing.T) {
	sql, err := restartIdentitySequenceSQL(DownloadedPhotosTable)
	if err != nil {
		t.Fatalf("restartIdentitySequenceSQL failed: %v", err)
	}
	normalized := normalizeSQL(sql)
	if !strings.Contains(normalized, "SELECT COALESCE(MAX(id), 1) + 1 INTO next_id") {
		t.Fatalf("empty ETL loads must restart at 2 to reserve legacy id 1, got: %s", normalized)
	}
	if !strings.Contains(normalized, "ALTER TABLE %I ALTER COLUMN id RESTART WITH %s") {
		t.Fatalf("expected ALTER TABLE identity restart, got: %s", normalized)
	}
	if strings.Contains(normalized, "pg_get_serial_sequence") || strings.Contains(normalized, "setval") {
		t.Fatalf("identity restart must not guess a sequence name, got: %s", normalized)
	}
	if _, err := restartIdentitySequenceSQL("photoprism_albums"); err == nil {
		t.Fatal("expected restartIdentitySequenceSQL to reject non-owned tables")
	}
}

func TestLoadDumpRequiresVerifiedLegacyTimezone(t *testing.T) {
	db := setupSQLiteETLDB(t)
	_, err := LoadDump(db, DumpRows{}, LoadOptions{})
	if err == nil {
		t.Fatal("expected load to require a verified legacy timezone")
	}
}

func TestNullableDatetimeCoercesLegacyEmptyAndZeroValues(t *testing.T) {
	for _, input := range []string{"", "   ", "0000-00-00 00:00:00", "0000-00-00 00:00:00.000"} {
		if got := nullableDatetime(input); got != nil {
			t.Fatalf("nullableDatetime(%q) = %#v, want nil", input, got)
		}
	}
	valid := "2026-01-02 03:04:05"
	if got := nullableDatetime(valid); got != valid {
		t.Fatalf("nullableDatetime(%q) = %#v, want original string", valid, got)
	}
	if got := nullableDatetimePtr(nil); got != nil {
		t.Fatalf("nullableDatetimePtr(nil) = %#v, want nil", got)
	}
	if got := nullableDatetimePtr(&valid); got != valid {
		t.Fatalf("nullableDatetimePtr(valid) = %#v, want original string", got)
	}
}

func TestUpsertDownloadedPhotoCoercesLegacyEmptyDatetimesToNull(t *testing.T) {
	db := setupSQLiteETLDB(t)
	uploadDate := "0000-00-00 00:00:00"
	loadedAt, err := upsertDownloadedPhoto(db.DB, LegacyDownloadedPhoto{
		ID:           6044,
		URL:          "https://example.test/piece-empty.jpg",
		SourcePage:   "https://example.test/gallery",
		Title:        "Empty datetime",
		Artist:       "Legacy",
		UploadDate:   &uploadDate,
		FilePath:     "piece-empty.jpg",
		FileName:     "piece-empty.jpg",
		DownloadedAt: "",
		FileSize:     42,
		Status:       "downloaded",
	})
	if err != nil {
		t.Fatalf("upsertDownloadedPhoto failed: %v", err)
	}
	if !loadedAt.IsZero() {
		t.Fatalf("NULL RETURNING downloaded_at should produce zero time, got %v", loadedAt)
	}
	assertNullTimeColumn(t, db.DB, DownloadedPhotosTable, "upload_date", 6044)
	assertNullTimeColumn(t, db.DB, DownloadedPhotosTable, "downloaded_at", 6044)
	var stored database.DownloadedPhoto
	if err := db.First(&stored, 6044).Error; err != nil {
		t.Fatalf("load nullable downloaded_photo model: %v", err)
	}
	if stored.UploadDate != nil || stored.DownloadedAt != nil {
		t.Fatalf("expected nullable model times to scan as nil, got upload_date=%v downloaded_at=%v", stored.UploadDate, stored.DownloadedAt)
	}
}

func TestUpsertExtractionRunCoercesLegacyZeroDatetimesToNull(t *testing.T) {
	db := setupSQLiteETLDB(t)
	endTime := "0000-00-00 00:00:00"
	startedAt, err := upsertExtractionRun(db.DB, LegacyExtractionRun{
		ID:               7,
		StartTime:        "0000-00-00 00:00:00",
		EndTime:          &endTime,
		Status:           "completed",
		PagesProcessed:   1,
		PhotosFound:      2,
		PhotosDownloaded: 3,
	})
	if err != nil {
		t.Fatalf("upsertExtractionRun failed: %v", err)
	}
	if !startedAt.IsZero() {
		t.Fatalf("NULL RETURNING start_time should produce zero time, got %v", startedAt)
	}
	assertNullTimeColumn(t, db.DB, ExtractionRunsTable, "start_time", 7)
	assertNullTimeColumn(t, db.DB, ExtractionRunsTable, "end_time", 7)
	var stored database.ExtractionRun
	if err := db.First(&stored, 7).Error; err != nil {
		t.Fatalf("load nullable extraction_run model: %v", err)
	}
	if stored.StartTime != nil || stored.EndTime != nil {
		t.Fatalf("expected nullable model times to scan as nil, got start_time=%v end_time=%v", stored.StartTime, stored.EndTime)
	}
}

func TestPostgresLoadDumpRestartsIdentityBeforeAppInsert(t *testing.T) {
	db := setupPostgresETLDB(t)
	downloadedAt := "2026-06-27 12:00:00"
	runEnd := "2026-06-27 12:05:00"
	rows := DumpRows{
		DownloadedPhotos: []LegacyDownloadedPhoto{{
			ID:           5,
			URL:          "https://fixture.test/legacy-photo.jpg",
			SourcePage:   "https://fixture.test/gallery",
			Title:        "Legacy photo",
			Artist:       "Legacy Artist",
			FilePath:     "legacy-photo.jpg",
			FileName:     "legacy-photo.jpg",
			DownloadedAt: downloadedAt,
			FileSize:     123,
			Status:       "downloaded",
		}},
		ExtractionRuns: []LegacyExtractionRun{{
			ID:               3,
			StartTime:        "2026-06-27 12:00:00",
			EndTime:          &runEnd,
			Status:           "completed",
			PagesProcessed:   1,
			PhotosFound:      1,
			PhotosDownloaded: 1,
		}},
	}

	result, err := LoadDump(db, rows, LoadOptions{
		LegacyTimeZone: "UTC",
		SetSequences:   true,
	})
	if err != nil {
		t.Fatalf("LoadDump failed: %v", err)
	}
	if result.PhotoMaxID != 5 || result.RunMaxID != 3 {
		t.Fatalf("unexpected max IDs from LoadDump: %#v", result)
	}

	appRun := database.ExtractionRun{Status: "running"}
	if err := db.Create(&appRun).Error; err != nil {
		t.Fatalf("plain app extraction_runs insert after ETL failed: %v", err)
	}
	if appRun.ID <= result.RunMaxID {
		t.Fatalf("app extraction_runs insert used id %d, want > %d", appRun.ID, result.RunMaxID)
	}

	appPhoto := database.DownloadedPhoto{
		URL:          "https://fixture.test/app-photo.jpg",
		SourcePage:   "https://fixture.test/gallery",
		Title:        "App photo",
		Artist:       "App Artist",
		FilePath:     "app-photo.jpg",
		FileName:     "app-photo.jpg",
		DownloadedAt: ptrTime(time.Date(2026, 6, 27, 12, 10, 0, 0, time.UTC)),
		Status:       "downloaded",
	}
	if err := db.Create(&appPhoto).Error; err != nil {
		t.Fatalf("plain app downloaded_photos insert after ETL failed: %v", err)
	}
	if appPhoto.ID <= result.PhotoMaxID {
		t.Fatalf("app downloaded_photos insert used id %d, want > %d", appPhoto.ID, result.PhotoMaxID)
	}

	assertOnlyIdentityIDSequence(t, db.DB, DownloadedPhotosTable)
	assertOnlyIdentityIDSequence(t, db.DB, ExtractionRunsTable)
}

func TestFillMissingContentHashesReadsOriginalsAndIsIdempotent(t *testing.T) {
	db := setupSQLiteETLDB(t)
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "piece.jpg"), []byte("fixture-bytes"), 0o600); err != nil {
		t.Fatalf("write fixture original: %v", err)
	}
	photo := &database.DownloadedPhoto{URL: "https://example.test/piece.jpg", FilePath: "piece.jpg", Status: "downloaded"}
	if err := db.Create(photo).Error; err != nil {
		t.Fatalf("insert photo: %v", err)
	}
	result, err := FillMissingContentHashes(db, root, 10)
	if err != nil {
		t.Fatalf("FillMissingContentHashes failed: %v", err)
	}
	if result.Updated != 1 || result.Skipped != 0 {
		t.Fatalf("unexpected hash result: %#v", result)
	}
	var stored database.DownloadedPhoto
	if err := db.First(&stored, photo.ID).Error; err != nil {
		t.Fatalf("load stored photo: %v", err)
	}
	if len(stored.ContentHash) != 32 {
		t.Fatalf("expected raw 32-byte sha256 content_hash, got %d bytes", len(stored.ContentHash))
	}
	second, err := FillMissingContentHashes(db, root, 10)
	if err != nil {
		t.Fatalf("second FillMissingContentHashes failed: %v", err)
	}
	if second.Updated != 0 {
		t.Fatalf("expected second pass to be idempotent, got %#v", second)
	}
}

func TestFillMissingContentHashesSkipsDuplicateOriginal(t *testing.T) {
	db := setupSQLiteETLDB(t)
	if err := db.Exec("CREATE UNIQUE INDEX idx_downloaded_photos_content_hash_unique ON downloaded_photos (content_hash) WHERE content_hash IS NOT NULL").Error; err != nil {
		t.Fatalf("create content hash unique index: %v", err)
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "winner.jpg"), []byte("same-original-bytes"), 0o600); err != nil {
		t.Fatalf("write winner original: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "loser.jpg"), []byte("same-original-bytes"), 0o600); err != nil {
		t.Fatalf("write loser original: %v", err)
	}
	photos := []*database.DownloadedPhoto{
		{URL: "https://example.test/winner.jpg", FilePath: "winner.jpg", Status: "downloaded"},
		{URL: "https://example.test/loser.jpg", FilePath: "loser.jpg", Status: "downloaded"},
	}
	if err := db.Create(&photos).Error; err != nil {
		t.Fatalf("insert photos: %v", err)
	}

	result, err := FillMissingContentHashes(db, root, 10)
	if err != nil {
		t.Fatalf("FillMissingContentHashes should skip duplicate content hash, got error: %v", err)
	}
	if result.Scanned != 2 || result.Updated != 1 || result.Skipped != 1 {
		t.Fatalf("unexpected hash result: %#v", result)
	}
	var hashedRows int64
	if err := db.Model(&database.DownloadedPhoto{}).Where("content_hash IS NOT NULL").Count(&hashedRows).Error; err != nil {
		t.Fatalf("count hashed rows: %v", err)
	}
	if hashedRows != 1 {
		t.Fatalf("expected one winning row to keep the duplicate content hash, got %d", hashedRows)
	}
	var loser database.DownloadedPhoto
	if err := db.First(&loser, photos[1].ID).Error; err != nil {
		t.Fatalf("load duplicate loser: %v", err)
	}
	if loser.Status != "deleted" {
		t.Fatalf("expected duplicate loser to be quarantined as deleted, got %q", loser.Status)
	}
}

func TestFillMissingContentHashesDuplicateLoserDoesNotBlockLaterRows(t *testing.T) {
	db := setupSQLiteETLDB(t)
	if err := db.Exec("CREATE UNIQUE INDEX idx_downloaded_photos_content_hash_unique ON downloaded_photos (content_hash) WHERE content_hash IS NOT NULL").Error; err != nil {
		t.Fatalf("create content hash unique index: %v", err)
	}
	root := t.TempDir()
	fixtures := map[string]string{
		"winner.jpg": "same-original-bytes",
		"loser.jpg":  "same-original-bytes",
		"later.jpg":  "later-original-bytes",
	}
	for name, contents := range fixtures {
		if err := os.WriteFile(filepath.Join(root, name), []byte(contents), 0o600); err != nil {
			t.Fatalf("write %s original: %v", name, err)
		}
	}
	photos := []*database.DownloadedPhoto{
		{URL: "https://example.test/winner.jpg", FilePath: "winner.jpg", Status: "downloaded"},
		{URL: "https://example.test/loser.jpg", FilePath: "loser.jpg", Status: "downloaded"},
		{URL: "https://example.test/later.jpg", FilePath: "later.jpg", Status: "downloaded"},
	}
	if err := db.Create(&photos).Error; err != nil {
		t.Fatalf("insert photos: %v", err)
	}

	first, err := FillMissingContentHashes(db, root, 2)
	if err != nil {
		t.Fatalf("first FillMissingContentHashes failed: %v", err)
	}
	if first.Scanned != 2 || first.Updated != 1 || first.Skipped != 1 {
		t.Fatalf("unexpected first hash result: %#v", first)
	}
	second, err := FillMissingContentHashes(db, root, 2)
	if err != nil {
		t.Fatalf("second FillMissingContentHashes failed: %v", err)
	}
	if second.Scanned != 1 || second.Updated != 1 || second.Skipped != 0 {
		t.Fatalf("expected second pass to reach the later row, got %#v", second)
	}
	var later database.DownloadedPhoto
	if err := db.First(&later, photos[2].ID).Error; err != nil {
		t.Fatalf("load later photo: %v", err)
	}
	if len(later.ContentHash) != 32 {
		t.Fatalf("expected later row to be hashed after duplicate loser was quarantined, got %d bytes", len(later.ContentHash))
	}
}

func setupSQLiteETLDB(t *testing.T) *database.DB {
	t.Helper()
	gormDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := gormDB.AutoMigrate(&database.DownloadedPhoto{}, &database.ExtractionRun{}, &database.InboxItem{}, &database.ConnectorState{}, &database.ETLWatermark{}); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}
	return &database.DB{DB: gormDB}
}

func setupPostgresETLDB(t *testing.T) *database.DB {
	t.Helper()
	dsn := os.Getenv("OKFOLIO_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set OKFOLIO_TEST_POSTGRES_DSN to run the Postgres-backed ETL integration test")
	}
	gormDB, err := gorm.Open(postgresdriver.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	assertDisposablePostgresTestDB(t, gormDB)
	if err := gormDB.Exec("DROP TABLE IF EXISTS downloaded_photos, extraction_runs, inbox_items, connector_states, etl_watermark CASCADE").Error; err != nil {
		t.Fatalf("reset postgres test tables: %v", err)
	}
	if err := gormDB.Exec("CREATE EXTENSION IF NOT EXISTS vector").Error; err != nil {
		t.Fatalf("create vector extension: %v", err)
	}
	if err := database.Migrate(gormDB); err != nil {
		t.Fatalf("migrate postgres: %v", err)
	}
	return &database.DB{DB: gormDB}
}

func assertDisposablePostgresTestDB(t *testing.T, db *gorm.DB) {
	t.Helper()
	var databaseName string
	if err := db.Raw("SELECT current_database()").Scan(&databaseName).Error; err != nil {
		t.Fatalf("verify postgres test database name: %v", err)
	}
	lowerName := strings.ToLower(databaseName)
	if !strings.Contains(lowerName, "test") {
		t.Fatalf("refusing to reset Postgres database %q; OKFOLIO_TEST_POSTGRES_DSN must point at a disposable database with \"test\" in its name", databaseName)
	}
	for _, blocked := range []string{"prod", "production", "staging", "stage"} {
		if strings.Contains(lowerName, blocked) {
			t.Fatalf("refusing to reset Postgres database %q; name looks non-disposable", databaseName)
		}
	}
}

func normalizeSQL(sql string) string {
	return strings.Join(strings.Fields(sql), " ")
}

func assertNullTimeColumn(t *testing.T, db *gorm.DB, table, column string, id uint64) {
	t.Helper()
	var got sql.NullTime
	if err := db.Raw("SELECT "+column+" FROM "+table+" WHERE id = ?", id).Row().Scan(&got); err != nil {
		t.Fatalf("read %s.%s for id %d: %v", table, column, id, err)
	}
	if got.Valid {
		t.Fatalf("expected %s.%s for id %d to be NULL, got %v", table, column, id, got.Time)
	}
}

func assertOnlyIdentityIDSequence(t *testing.T, db *gorm.DB, table string) {
	t.Helper()
	type sequenceRow struct {
		RelName string
		DepType sql.NullString
	}
	var rows []sequenceRow
	if err := db.Raw(`
		SELECT seq.relname AS rel_name, dep.deptype AS dep_type
		FROM pg_class tbl
		JOIN pg_namespace ns ON ns.oid = tbl.relnamespace
		JOIN pg_attribute attr ON attr.attrelid = tbl.oid AND attr.attname = 'id'
		JOIN pg_class seq ON seq.relnamespace = ns.oid AND seq.relkind = 'S'
		LEFT JOIN pg_depend dep ON dep.objid = seq.oid
			AND dep.refobjid = tbl.oid
			AND dep.refobjsubid = attr.attnum
		WHERE tbl.relname = ?
			AND seq.relname ~ ?
		ORDER BY seq.relname`, table, "^"+table+"_id_seq[0-9]*$").Scan(&rows).Error; err != nil {
		t.Fatalf("probe %s id sequences: %v", table, err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected one %s id sequence, got %#v", table, rows)
	}
	if rows[0].RelName != table+"_id_seq" {
		t.Fatalf("expected canonical %s id sequence, got %#v", table, rows)
	}
	if !rows[0].DepType.Valid || rows[0].DepType.String != "i" {
		t.Fatalf("expected %s id sequence to be identity-owned, got %#v", table, rows)
	}
}

func ptrTime(t time.Time) *time.Time {
	return &t
}
