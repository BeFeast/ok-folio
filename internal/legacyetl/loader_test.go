package legacyetl

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ok-folio/internal/database"

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

func TestLoadDumpRequiresVerifiedLegacyTimezone(t *testing.T) {
	db := setupSQLiteETLDB(t)
	_, err := LoadDump(db, DumpRows{}, LoadOptions{})
	if err == nil {
		t.Fatal("expected load to require a verified legacy timezone")
	}
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

func normalizeSQL(sql string) string {
	return strings.Join(strings.Fields(sql), " ")
}
