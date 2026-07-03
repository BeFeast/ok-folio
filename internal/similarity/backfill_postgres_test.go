package similarity

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rs/zerolog"
	postgresdriver "gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"ok-folio/internal/config"
	"ok-folio/internal/database"
)

func openBackfillTestDB(t *testing.T) *database.DB {
	t.Helper()
	dsn := os.Getenv("OKFOLIO_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set OKFOLIO_TEST_POSTGRES_DSN to run the Postgres-backed similarity backfill test")
	}
	gormDB, err := gorm.Open(postgresdriver.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("connect postgres: %v", err)
	}
	gormDB.Exec("DROP TABLE IF EXISTS downloaded_photos, extraction_runs, inbox_items, connector_states, connector_sources, etl_watermark, folios, folio_pieces CASCADE")
	gormDB.Exec("CREATE EXTENSION IF NOT EXISTS vector")
	if err := database.Migrate(gormDB); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	db := &database.DB{DB: gormDB}
	db.RefreshEmbeddingColumnCapability()
	if !db.HasEmbeddingColumn() {
		t.Skip("embedding column unavailable; pgvector extension is required")
	}
	return db
}

func backfillTestStorage(t *testing.T, root string) config.StorageConfig {
	t.Helper()
	return config.StorageConfig{
		BaseDirectory:        root,
		DerivativesDirectory: filepath.Join(t.TempDir(), "derivatives"),
		DerivativesMaxBytes:  50 * 1024 * 1024,
	}
}

func TestPostgresBackfillWritesEmbeddingsAndCreatesHNSW(t *testing.T) {
	db := openBackfillTestDB(t)

	root := t.TempDir()
	storage := backfillTestStorage(t, root)
	writeFixtureJPEG(t, filepath.Join(root, "one.jpg"), color.RGBA{R: 220, G: 20, B: 20, A: 255})
	writeFixtureJPEG(t, filepath.Join(root, "two.jpg"), color.RGBA{R: 20, G: 220, B: 20, A: 255})
	insertBackfillPhoto(t, db, 1, "one.jpg", []byte("one"))
	insertBackfillPhoto(t, db, 2, "two.jpg", []byte("two"))
	insertBackfillPhoto(t, db, 3, "missing.jpg", []byte("missing"))
	if err := db.StoreEmbedding(2, fixtureEmbedding(2)); err != nil {
		t.Fatalf("pre-store embedding: %v", err)
	}

	var calls int
	sidecar := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		_ = json.NewEncoder(w).Encode(map[string]any{
			"embedding": fixtureEmbedding(1),
			"model":     "clip-vit-b32",
			"dim":       database.EmbeddingDim,
		})
	}))
	defer sidecar.Close()

	result, err := Backfill(context.Background(), db, storage, NewClient(sidecar.URL), Options{Concurrency: 1, BatchSize: 2, Progress: 1}, zerolog.Nop())
	if err != nil {
		t.Fatalf("Backfill failed: %v", err)
	}
	if result.Scanned != 2 || result.Embedded != 1 || result.Missing != 1 || result.Permanent != 0 || result.Failed != 0 {
		t.Fatalf("unexpected result: %#v", result)
	}
	if calls != 1 {
		t.Fatalf("expected one sidecar call for the only unembedded existing file, got %d", calls)
	}
	assertEmbeddingPresent(t, db, 1)
	assertEmbeddingPresent(t, db, 2)
	assertEmbeddingMissing(t, db, 3)
	assertIndexExists(t, db, database.EmbeddingHNSWIndex)

	calls = 0
	result, err = Backfill(context.Background(), db, storage, NewClient(sidecar.URL), Options{Concurrency: 1}, zerolog.Nop())
	if err != nil {
		t.Fatalf("second Backfill failed: %v", err)
	}
	if result.Embedded != 0 || calls != 0 {
		t.Fatalf("expected rerun to skip populated rows, result=%#v calls=%d", result, calls)
	}
}

func TestPostgresBackfillPermanentDecodeFailuresStillCreateHNSW(t *testing.T) {
	db := openBackfillTestDB(t)

	root := t.TempDir()
	storage := backfillTestStorage(t, root)
	if err := os.WriteFile(filepath.Join(root, "corrupt.jpg"), []byte("not an image"), 0o644); err != nil {
		t.Fatalf("write corrupt fixture: %v", err)
	}
	insertBackfillPhoto(t, db, 1, "corrupt.jpg", []byte("corrupt"))

	var calls int
	sidecar := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		_ = json.NewEncoder(w).Encode(map[string]any{
			"embedding": fixtureEmbedding(1),
			"model":     "clip-vit-b32",
			"dim":       database.EmbeddingDim,
		})
	}))
	defer sidecar.Close()

	result, err := Backfill(context.Background(), db, storage, NewClient(sidecar.URL), Options{Concurrency: 1}, zerolog.Nop())
	if err != nil {
		t.Fatalf("Backfill failed: %v", err)
	}
	if result.Scanned != 1 || result.Permanent != 1 || result.Failed != 0 {
		t.Fatalf("unexpected result: %#v", result)
	}
	if calls != 0 {
		t.Fatalf("expected no sidecar calls for an undecodable original, got %d", calls)
	}
	assertEmbeddingMissing(t, db, 1)
	assertIndexExists(t, db, database.EmbeddingHNSWIndex)
}

func TestPostgresBackfillTransientSidecarFailureBlocksHNSW(t *testing.T) {
	db := openBackfillTestDB(t)

	root := t.TempDir()
	storage := backfillTestStorage(t, root)
	writeFixtureJPEG(t, filepath.Join(root, "one.jpg"), color.RGBA{R: 20, G: 20, B: 220, A: 255})
	insertBackfillPhoto(t, db, 1, "one.jpg", []byte("one"))

	sidecar := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "embedder unavailable", http.StatusInternalServerError)
	}))
	defer sidecar.Close()

	result, err := Backfill(context.Background(), db, storage, NewClient(sidecar.URL), Options{Concurrency: 1}, zerolog.Nop())
	if err != nil {
		t.Fatalf("Backfill failed: %v", err)
	}
	if result.Scanned != 1 || result.Failed != 1 || result.Permanent != 0 {
		t.Fatalf("unexpected result: %#v", result)
	}
	assertIndexMissing(t, db, database.EmbeddingHNSWIndex)
}

func insertBackfillPhoto(t *testing.T, db *database.DB, id uint64, filePath string, hashSeed []byte) {
	t.Helper()
	sum := sha256.Sum256(hashSeed)
	now := time.Now().UTC()
	photo := database.DownloadedPhoto{
		ID:           id,
		URL:          "https://fixture.test/" + filePath,
		URLHash:      database.HashURL("https://fixture.test/" + filePath),
		SourcePage:   "https://fixture.test/source",
		Title:        filePath,
		Artist:       "Fixture",
		FilePath:     filePath,
		FileName:     filepath.Base(filePath),
		Status:       "downloaded",
		DownloadedAt: &now,
		ContentHash:  sum[:],
		Provider:     database.DefaultProvider,
	}
	if err := db.Create(&photo).Error; err != nil {
		t.Fatalf("insert photo %d: %v", id, err)
	}
}

func writeFixtureJPEG(t *testing.T, path string, c color.RGBA) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 32, 24))
	for y := 0; y < 24; y++ {
		for x := 0; x < 32; x++ {
			img.Set(x, y, c)
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir fixture: %v", err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create fixture: %v", err)
	}
	defer f.Close()
	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("encode fixture: %v", err)
	}
}

func fixtureEmbedding(seed int) []float32 {
	out := make([]float32, database.EmbeddingDim)
	out[seed%database.EmbeddingDim] = 1
	return out
}

func assertEmbeddingPresent(t *testing.T, db *database.DB, id uint64) {
	t.Helper()
	var present bool
	if err := db.Raw("SELECT embedding IS NOT NULL FROM downloaded_photos WHERE id = ?", id).Scan(&present).Error; err != nil {
		t.Fatalf("query embedding for %d: %v", id, err)
	}
	if !present {
		t.Fatalf("expected embedding for photo %d", id)
	}
}

func assertEmbeddingMissing(t *testing.T, db *database.DB, id uint64) {
	t.Helper()
	var present bool
	if err := db.Raw("SELECT embedding IS NOT NULL FROM downloaded_photos WHERE id = ?", id).Scan(&present).Error; err != nil {
		t.Fatalf("query embedding for %d: %v", id, err)
	}
	if present {
		t.Fatalf("expected no embedding for photo %d", id)
	}
}

func assertIndexExists(t *testing.T, db *database.DB, name string) {
	t.Helper()
	if !queryIndexExists(t, db, name) {
		t.Fatalf("expected index %s", name)
	}
}

func assertIndexMissing(t *testing.T, db *database.DB, name string) {
	t.Helper()
	if queryIndexExists(t, db, name) {
		t.Fatalf("expected index %s to be absent", name)
	}
}

func queryIndexExists(t *testing.T, db *database.DB, name string) bool {
	t.Helper()
	var exists bool
	if err := db.Raw("SELECT EXISTS (SELECT 1 FROM pg_indexes WHERE tablename = 'downloaded_photos' AND indexname = ?)", name).Scan(&exists).Error; err != nil {
		t.Fatalf("query index %s: %v", name, err)
	}
	return exists
}
