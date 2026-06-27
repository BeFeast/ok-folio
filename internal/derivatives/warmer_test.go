package derivatives

import (
	"context"
	"crypto/sha256"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"ok-folio/internal/config"
	"ok-folio/internal/database"
)

func TestWarmThumbnailsGeneratesAndSkipsExistingDerivatives(t *testing.T) {
	db := setupWarmDB(t)
	storage := warmStorage(t)
	photo := createWarmPhoto(t, db, storage, "piece.jpg")

	result, err := WarmThumbnails(context.Background(), db, storage, WarmOptions{
		Widths:      []int{400, 700},
		Concurrency: 1,
		BatchSize:   1,
	}, zerolog.Nop())
	if err != nil {
		t.Fatalf("WarmThumbnails failed: %v", err)
	}
	if result.Scanned != 1 || result.Generated != 2 || result.Skipped != 0 || result.Missing != 0 || result.Failed != 0 {
		t.Fatalf("unexpected first warm result: %#v", result)
	}

	cache := NewCache(storage)
	validator, err := Validator(&photo, resolveOriginalPath(storage, photo.FilePath))
	if err != nil {
		t.Fatalf("Validator failed: %v", err)
	}
	for _, width := range []int{400, 700} {
		entry := cache.Entry(&photo, width, validator)
		if _, err := os.Stat(entry.Path); err != nil {
			t.Fatalf("expected warmed derivative for width %d: %v", width, err)
		}
	}

	result, err = WarmThumbnails(context.Background(), db, storage, WarmOptions{
		Widths:      []int{400, 700},
		Concurrency: 1,
		BatchSize:   1,
	}, zerolog.Nop())
	if err != nil {
		t.Fatalf("second WarmThumbnails failed: %v", err)
	}
	if result.Scanned != 1 || result.Generated != 0 || result.Skipped != 2 || result.Missing != 0 || result.Failed != 0 {
		t.Fatalf("unexpected second warm result: %#v", result)
	}
}

func TestWarmThumbnailsSkipsMissingOriginals(t *testing.T) {
	db := setupWarmDB(t)
	storage := warmStorage(t)
	photo := database.DownloadedPhoto{
		URL:          "https://example.test/missing.jpg",
		FilePath:     "missing.jpg",
		FileName:     "missing.jpg",
		FileSize:     123,
		Status:       "downloaded",
		DownloadedAt: ptrWarmTime(time.Now()),
	}
	if err := db.Create(&photo).Error; err != nil {
		t.Fatalf("create missing photo: %v", err)
	}

	result, err := WarmThumbnails(context.Background(), db, storage, WarmOptions{
		Widths:      []int{400, 700},
		Concurrency: 2,
	}, zerolog.Nop())
	if err != nil {
		t.Fatalf("WarmThumbnails failed: %v", err)
	}
	if result.Scanned != 1 || result.Generated != 0 || result.Skipped != 0 || result.Missing != 2 || result.Failed != 0 {
		t.Fatalf("unexpected missing warm result: %#v", result)
	}
}

func TestWarmOnePhotoGeneratesConfiguredWidths(t *testing.T) {
	db := setupWarmDB(t)
	storage := warmStorage(t)
	photo := createWarmPhoto(t, db, storage, "ingested.jpg")
	hotCache := &recordingHotCache{values: map[string][]byte{}}

	result, err := WarmOnePhoto(context.Background(), storage, &photo, WarmPhotoOptions{
		Widths:   []int{700, 400},
		HotCache: hotCache,
		HotTTL:   time.Hour,
	}, zerolog.Nop())
	if err != nil {
		t.Fatalf("WarmOnePhoto failed: %v", err)
	}
	if result.Scanned != 1 || result.Generated != 2 || result.Skipped != 0 || result.Missing != 0 || result.Failed != 0 {
		t.Fatalf("unexpected warm-one result: %#v", result)
	}

	cache := NewCache(storage)
	validator, err := Validator(&photo, resolveOriginalPath(storage, photo.FilePath))
	if err != nil {
		t.Fatalf("Validator failed: %v", err)
	}
	for _, width := range []int{400, 700} {
		entry := cache.Entry(&photo, width, validator)
		if _, err := os.Stat(entry.Path); err != nil {
			t.Fatalf("expected warmed derivative for width %d: %v", width, err)
		}
		if len(hotCache.values[entry.Key]) == 0 {
			t.Fatalf("expected hot cache write for width %d", width)
		}
	}
}

func TestNormalizeWidthsDedupesSortsAndClamps(t *testing.T) {
	widths, err := normalizeWidths([]int{700, 400, 700, 2048})
	if err != nil {
		t.Fatalf("normalizeWidths failed: %v", err)
	}
	want := []int{400, 700, MaxThumbnailSize}
	if len(widths) != len(want) {
		t.Fatalf("expected widths %v, got %v", want, widths)
	}
	for i := range want {
		if widths[i] != want[i] {
			t.Fatalf("expected widths %v, got %v", want, widths)
		}
	}
}

func TestNormalizeWarmConcurrencyClampsUpperBound(t *testing.T) {
	if got := normalizeWarmConcurrency(0, zerolog.Nop()); got != 2 {
		t.Fatalf("expected default concurrency 2, got %d", got)
	}
	if got := normalizeWarmConcurrency(MaxWarmConcurrency+10, zerolog.Nop()); got != MaxWarmConcurrency {
		t.Fatalf("expected concurrency clamp to %d, got %d", MaxWarmConcurrency, got)
	}
	if got := normalizeWarmConcurrency(3, zerolog.Nop()); got != 3 {
		t.Fatalf("expected concurrency 3, got %d", got)
	}
}

func setupWarmDB(t *testing.T) *database.DB {
	t.Helper()
	gormDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := gormDB.AutoMigrate(&database.DownloadedPhoto{}); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}
	return &database.DB{DB: gormDB}
}

func warmStorage(t *testing.T) config.StorageConfig {
	t.Helper()
	root := t.TempDir()
	return config.StorageConfig{
		BaseDirectory:        filepath.Join(root, "originals"),
		DerivativesDirectory: filepath.Join(root, "derivatives"),
		DerivativesMaxBytes:  50 * 1024 * 1024,
	}
}

func createWarmPhoto(t *testing.T, db *database.DB, storage config.StorageConfig, name string) database.DownloadedPhoto {
	t.Helper()
	path := filepath.Join(storage.BaseDirectory, name)
	createWarmJPEG(t, path)
	contentHash := sha256.Sum256([]byte(name))
	photo := database.DownloadedPhoto{
		URL:          "https://example.test/" + name,
		FilePath:     name,
		FileName:     name,
		FileSize:     123,
		ContentHash:  contentHash[:],
		Status:       "downloaded",
		DownloadedAt: ptrWarmTime(time.Now()),
	}
	if err := db.Create(&photo).Error; err != nil {
		t.Fatalf("create photo: %v", err)
	}
	return photo
}

func createWarmJPEG(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir image dir: %v", err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 32, 24))
	for y := 0; y < 24; y++ {
		for x := 0; x < 32; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 4), G: uint8(y * 6), B: 120, A: 255})
		}
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create image: %v", err)
	}
	defer file.Close()
	if err := jpeg.Encode(file, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("encode image: %v", err)
	}
}

func ptrWarmTime(t time.Time) *time.Time {
	return &t
}

type recordingHotCache struct {
	values map[string][]byte
}

func (c *recordingHotCache) SetBytes(_ context.Context, key string, value []byte, _ time.Duration) {
	c.values[key] = append([]byte(nil), value...)
}
