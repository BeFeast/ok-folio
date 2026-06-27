package scraper

import (
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"ok-folio/internal/config"
	"ok-folio/internal/database"
	"ok-folio/internal/derivatives"
	"ok-folio/internal/provider"
	"ok-folio/internal/testguard"

	"github.com/rs/zerolog"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestScrapePageAutoKeepsNewMediaByStableDedupeKey(t *testing.T) {
	imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("fixture image bytes"))
	}))
	defer imageServer.Close()

	db := setupScraperTestDB(t)
	cfg := setupScraperTestConfig(t)
	connector := &fakeConnector{
		items: []provider.DiscoveredMedia{
			{
				ProviderID: "fixture",
				DedupeKey:  provider.DedupeKey{ProviderID: "fixture", Value: "source-1:media-1"},
				Source: provider.SourceMetadata{
					URL:        "https://fixture.test/source/1",
					ExternalID: "source-1",
				},
				Media: provider.MediaMetadata{
					ExternalID: "media-1",
					FileName:   "media-1.jpg",
				},
				Title:       "New Fixture",
				Artist:      "Fixture Artist",
				PublishedAt: time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC),
			},
		},
		mediaURL: imageServer.URL + "/media-1.jpg",
	}
	s := NewWithProvider(cfg, db, zerolog.New(os.Stderr).Level(zerolog.Disabled), connector)

	downloaded, skipped, failed, err := s.ScrapePage(context.Background(), 1)
	if err != nil {
		t.Fatalf("ScrapePage returned error: %v", err)
	}
	if downloaded != 1 || skipped != 0 || failed != 0 {
		t.Fatalf("Expected one auto-kept item, got downloaded=%d skipped=%d failed=%d", downloaded, skipped, failed)
	}

	var photo database.DownloadedPhoto
	if err := db.Where("url = ?", "fixture:source-1:media-1").First(&photo).Error; err != nil {
		t.Fatalf("Expected downloaded photo keyed by connector dedupe key: %v", err)
	}
	if photo.SourcePage != "https://fixture.test/source/1" {
		t.Fatalf("Expected source provenance to remain separate from dedupe key, got %q", photo.SourcePage)
	}

	inbox, total, err := db.GetInboxExceptions(10, 0)
	if err != nil {
		t.Fatalf("Failed to query inbox: %v", err)
	}
	if total != 0 || len(inbox) != 0 {
		t.Fatalf("Expected no inbox exceptions for new media, got total=%d inbox=%#v", total, inbox)
	}
}

func TestPublishedAtPtrLeavesZeroDateEmpty(t *testing.T) {
	if got := publishedAtPtr(time.Time{}); got != nil {
		t.Fatalf("expected zero published date to map to nil, got %s", got)
	}

	publishedAt := time.Date(1933, 1, 1, 0, 0, 0, 0, time.UTC)
	got := publishedAtPtr(publishedAt)
	if got == nil || !got.Equal(publishedAt) {
		t.Fatalf("expected non-zero published date to be preserved, got %v", got)
	}
}

func TestScrapePageRecordsDuplicateAndAmbiguousInboxExceptions(t *testing.T) {
	db := setupScraperTestDB(t)
	cfg := setupScraperTestConfig(t)
	if err := db.RecordDownload(&database.DownloadedPhoto{
		URL:      "fixture:source-1:media-1",
		FilePath: filepath.Join(cfg.Storage.BaseDirectory, "existing.jpg"),
		FileName: "existing.jpg",
		Status:   "downloaded",
	}); err != nil {
		t.Fatalf("Failed to seed downloaded media: %v", err)
	}

	connector := &fakeConnector{
		items: []provider.DiscoveredMedia{
			{
				ProviderID: "fixture",
				DedupeKey:  provider.DedupeKey{ProviderID: "fixture", Value: "source-1:media-1"},
				Source:     provider.SourceMetadata{URL: "https://fixture.test/source/1", ExternalID: "source-1"},
				Media:      provider.MediaMetadata{ExternalID: "media-1"},
				Title:      "Duplicate Fixture",
			},
			{
				ProviderID: "fixture",
				Source:     provider.SourceMetadata{URL: "https://fixture.test/source/ambiguous", ExternalID: "source-ambiguous"},
				Media:      provider.MediaMetadata{ExternalID: "media-ambiguous"},
				Title:      "Ambiguous Fixture",
			},
		},
	}
	s := NewWithProvider(cfg, db, zerolog.New(os.Stderr).Level(zerolog.Disabled), connector)

	downloaded, skipped, failed, err := s.ScrapePage(context.Background(), 1)
	if err != nil {
		t.Fatalf("ScrapePage returned error: %v", err)
	}
	if downloaded != 0 || skipped != 2 || failed != 0 {
		t.Fatalf("Expected duplicate and ambiguous items to be skipped into inbox, got downloaded=%d skipped=%d failed=%d", downloaded, skipped, failed)
	}

	inbox, total, err := db.GetInboxExceptions(10, 0)
	if err != nil {
		t.Fatalf("Failed to query inbox: %v", err)
	}
	if total != 2 || len(inbox) != 2 {
		t.Fatalf("Expected two inbox exceptions, got total=%d inbox=%#v", total, inbox)
	}

	statuses := map[string]bool{}
	for _, item := range inbox {
		statuses[item.Status] = true
		if item.Status == "duplicate" && item.DedupeKey != "fixture:source-1:media-1" {
			t.Fatalf("Duplicate inbox item used wrong dedupe key: %#v", item)
		}
	}
	if !statuses["duplicate"] || !statuses["ambiguous"] {
		t.Fatalf("Expected duplicate and ambiguous statuses, got %#v", statuses)
	}
}

func TestScrapePageSerializesSamePageDedupeKeyDuplicates(t *testing.T) {
	var imageRequests int32
	imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&imageRequests, 1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("fixture image bytes"))
	}))
	defer imageServer.Close()

	db := setupScraperTestDB(t)
	cfg := setupScraperTestConfig(t)
	cfg.Download.ConcurrentLimit = 2
	connector := &fakeConnector{
		items: []provider.DiscoveredMedia{
			{
				ProviderID: "fixture",
				DedupeKey:  provider.DedupeKey{ProviderID: "fixture", Value: "source-1:media-1"},
				Source:     provider.SourceMetadata{URL: "https://fixture.test/source/1", ExternalID: "source-1"},
				Media:      provider.MediaMetadata{ExternalID: "media-1", FileName: "media-1.jpg"},
				Title:      "First Fixture",
			},
			{
				ProviderID: "fixture",
				DedupeKey:  provider.DedupeKey{ProviderID: "fixture", Value: "source-1:media-1"},
				Source:     provider.SourceMetadata{URL: "https://fixture.test/source/1-copy", ExternalID: "source-1-copy"},
				Media:      provider.MediaMetadata{ExternalID: "media-1", FileName: "media-1-copy.jpg"},
				Title:      "Duplicate Fixture",
			},
		},
		mediaURL: imageServer.URL + "/media-1.jpg",
	}
	s := NewWithProvider(cfg, db, zerolog.New(os.Stderr).Level(zerolog.Disabled), connector)

	downloaded, skipped, failed, err := s.ScrapePage(context.Background(), 1)
	if err != nil {
		t.Fatalf("ScrapePage returned error: %v", err)
	}
	if downloaded != 1 || skipped != 1 || failed != 0 {
		t.Fatalf("Expected one kept item and one duplicate exception, got downloaded=%d skipped=%d failed=%d", downloaded, skipped, failed)
	}
	if got := atomic.LoadInt32(&imageRequests); got != 1 {
		t.Fatalf("Expected duplicate media to download only once, got %d requests", got)
	}

	inbox, total, err := db.GetInboxExceptions(10, 0)
	if err != nil {
		t.Fatalf("Failed to query inbox: %v", err)
	}
	if total != 1 || len(inbox) != 1 || inbox[0].Status != "duplicate" {
		t.Fatalf("Expected one duplicate inbox exception, total=%d inbox=%#v", total, inbox)
	}
}

func TestScrapePageRecognizesLegacyWebGallerySourceURLAsDuplicate(t *testing.T) {
	db := setupScraperTestDB(t)
	cfg := setupScraperTestConfig(t)
	legacySourceURL := "https://webgallery.test/photos/alpha?id=1"
	if err := db.RecordDownload(&database.DownloadedPhoto{
		URL:      legacySourceURL,
		FilePath: filepath.Join(cfg.Storage.BaseDirectory, "existing.jpg"),
		FileName: "existing.jpg",
		Status:   "downloaded",
	}); err != nil {
		t.Fatalf("Failed to seed legacy downloaded media: %v", err)
	}

	connector := &fakeConnector{
		providerID: "webgallery",
		items: []provider.DiscoveredMedia{
			{
				ProviderID: "webgallery",
				DedupeKey:  provider.DedupeKey{ProviderID: "webgallery", Value: "photos/alpha?id=1"},
				Source: provider.SourceMetadata{
					URL:        legacySourceURL,
					ExternalID: "photos/alpha?id=1",
				},
				Media: provider.MediaMetadata{ExternalID: "media-1"},
				Title: "Legacy Duplicate",
			},
		},
	}
	s := NewWithProvider(cfg, db, zerolog.New(os.Stderr).Level(zerolog.Disabled), connector)

	downloaded, skipped, failed, err := s.ScrapePage(context.Background(), 1)
	if err != nil {
		t.Fatalf("ScrapePage returned error: %v", err)
	}
	if downloaded != 0 || skipped != 1 || failed != 0 {
		t.Fatalf("Expected legacy URL keyed item to be treated as duplicate, got downloaded=%d skipped=%d failed=%d", downloaded, skipped, failed)
	}

	inbox, total, err := db.GetInboxExceptions(10, 0)
	if err != nil {
		t.Fatalf("Failed to query inbox: %v", err)
	}
	if total != 1 || len(inbox) != 1 {
		t.Fatalf("Expected one duplicate inbox exception, got total=%d inbox=%#v", total, inbox)
	}
	if inbox[0].Status != "duplicate" || inbox[0].DedupeKey != "webgallery:photos/alpha?id=1" {
		t.Fatalf("Unexpected inbox item for legacy duplicate: %#v", inbox[0])
	}
}

func TestDownloadResolvedMediaWarmsThumbnailsOnIngest(t *testing.T) {
	imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		writeScraperTestJPEG(t, w)
	}))
	defer imageServer.Close()

	db := setupScraperTestDB(t)
	cfg := setupScraperTestConfig(t)
	cfg.Storage.DerivativesDirectory = filepath.Join(t.TempDir(), "derivatives")
	cfg.Storage.DerivativesMaxBytes = 50 * 1024 * 1024
	cfg.Storage.WarmOnIngest = true
	cfg.Storage.WarmOnIngestWidths = []int{400, 700}
	s := NewWithProvider(cfg, db, zerolog.New(os.Stderr).Level(zerolog.Disabled), &fakeConnector{})
	s.thumbHotCache = nil

	publishedAt := time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)
	photo, kept, err := s.DownloadResolvedMediaOrDuplicate(context.Background(), provider.DiscoveredMedia{
		ProviderID:  "fixture",
		DedupeKey:   provider.DedupeKey{ProviderID: "fixture", Value: "source-1:media-1"},
		Source:      provider.SourceMetadata{URL: "https://fixture.test/source/1", ExternalID: "source-1"},
		Media:       provider.MediaMetadata{URL: imageServer.URL + "/media-1.jpg", ExternalID: "media-1", FileName: "media-1.jpg"},
		Title:       "Warm Fixture",
		Artist:      "Fixture Artist",
		PublishedAt: publishedAt,
	}, "fixture")
	if err != nil {
		t.Fatalf("DownloadResolvedMediaOrDuplicate failed: %v", err)
	}
	if !kept {
		t.Fatal("expected new media to be kept")
	}

	cache := derivatives.NewCache(cfg.Storage)
	validator, err := derivatives.Validator(photo, photo.FilePath)
	if err != nil {
		t.Fatalf("Validator failed: %v", err)
	}
	for _, width := range []int{400, 700} {
		entry := cache.Entry(photo, width, validator)
		waitForFile(t, entry.Path)
	}
}

func TestDownloadResolvedMediaWarmsRecoveredFailedRowWithPersistedID(t *testing.T) {
	imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		writeScraperTestJPEG(t, w)
	}))
	defer imageServer.Close()

	db := setupScraperTestDB(t)
	cfg := setupScraperTestConfig(t)
	cfg.Storage.DerivativesDirectory = filepath.Join(t.TempDir(), "derivatives")
	cfg.Storage.DerivativesMaxBytes = 50 * 1024 * 1024
	cfg.Storage.WarmOnIngest = true
	cfg.Storage.WarmOnIngestWidths = []int{400}
	s := NewWithProvider(cfg, db, zerolog.New(os.Stderr).Level(zerolog.Disabled), &fakeConnector{})
	s.thumbHotCache = nil

	dedupeKey := provider.DedupeKey{ProviderID: "fixture", Value: "source-retry:media-retry"}.String()
	if err := db.RecordFailedDownload(dedupeKey, "temporary failure"); err != nil {
		t.Fatalf("RecordFailedDownload failed: %v", err)
	}

	publishedAt := time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)
	photo, kept, err := s.DownloadResolvedMediaOrDuplicate(context.Background(), provider.DiscoveredMedia{
		ProviderID:  "fixture",
		DedupeKey:   provider.DedupeKey{ProviderID: "fixture", Value: "source-retry:media-retry"},
		Source:      provider.SourceMetadata{URL: "https://fixture.test/source/retry", ExternalID: "source-retry"},
		Media:       provider.MediaMetadata{URL: imageServer.URL + "/media-retry.jpg", ExternalID: "media-retry", FileName: "media-retry.jpg"},
		Title:       "Recovered Warm Fixture",
		Artist:      "Fixture Artist",
		PublishedAt: publishedAt,
	}, "fixture")
	if err != nil {
		t.Fatalf("DownloadResolvedMediaOrDuplicate failed: %v", err)
	}
	if !kept {
		t.Fatal("expected recovered media to be kept")
	}
	if photo.ID == 0 {
		t.Fatalf("expected recovered photo to include persisted ID, got %#v", photo)
	}

	var stored database.DownloadedPhoto
	if err := db.Where("url_hash = ?", database.HashURL(dedupeKey)).First(&stored).Error; err != nil {
		t.Fatalf("failed to load recovered photo: %v", err)
	}
	if stored.ID != photo.ID {
		t.Fatalf("expected returned photo ID %d to match stored ID %d", photo.ID, stored.ID)
	}

	cache := derivatives.NewCache(cfg.Storage)
	validator, err := derivatives.Validator(&stored, stored.FilePath)
	if err != nil {
		t.Fatalf("Validator failed: %v", err)
	}
	entry := cache.Entry(&stored, 400, validator)
	waitForFile(t, entry.Path)
}

func TestScheduleWarmThumbnailsOnIngestReturnsWhenWorkersFull(t *testing.T) {
	cfg := setupScraperTestConfig(t)
	cfg.Storage.WarmOnIngest = true
	cfg.Storage.WarmOnIngestWidths = []int{400}
	s := &Scraper{
		cfg:          cfg,
		logger:       zerolog.Nop(),
		thumbWarmSem: make(chan struct{}, 1),
	}
	s.thumbWarmSem <- struct{}{}

	done := make(chan struct{})
	go func() {
		s.scheduleWarmThumbnailsOnIngest(database.DownloadedPhoto{ID: 42})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("scheduleWarmThumbnailsOnIngest blocked when workers were full")
	}
}

func setupScraperTestDB(t *testing.T) *database.DB {
	t.Helper()

	gormDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	if err := gormDB.AutoMigrate(&database.DownloadedPhoto{}, &database.ExtractionRun{}, &database.InboxItem{}, &database.ConnectorState{}); err != nil {
		t.Fatalf("Failed to migrate test database: %v", err)
	}
	return &database.DB{DB: gormDB}
}

func setupScraperTestConfig(t *testing.T) *config.Config {
	t.Helper()

	root := t.TempDir()
	cfg := &config.Config{
		Source: config.SourceConfig{
			BaseURL: "https://fixture.test/gallery",
		},
		Storage: config.StorageConfig{
			BaseDirectory:  filepath.Join(root, "originals"),
			DailyDirectory: filepath.Join(root, "daily"),
		},
		Download: config.DownloadConfig{
			ConcurrentLimit: 1,
			Timeout:         5 * time.Second,
		},
		Retry: config.RetryConfig{
			MaxAttempts: 1,
		},
	}
	if err := testguard.ValidateConfig(cfg); err != nil {
		t.Fatalf("unsafe scraper test config: %v", err)
	}
	return cfg
}

func writeScraperTestJPEG(t *testing.T, w http.ResponseWriter) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 32, 24))
	for y := 0; y < 24; y++ {
		for x := 0; x < 32; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 5), G: uint8(y * 7), B: 160, A: 255})
		}
	}
	if err := jpeg.Encode(w, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("encode jpeg response: %v", err)
	}
}

func waitForFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected warmed derivative %s: %v", path, err)
	}
}

type fakeConnector struct {
	providerID string
	items      []provider.DiscoveredMedia
	mediaURL   string
}

func (c *fakeConnector) Provider() provider.Source {
	id := c.providerID
	if id == "" {
		id = "fixture"
	}
	return provider.Source{ID: id, DisplayName: "Fixture"}
}

func (c *fakeConnector) DiscoverPage(context.Context, provider.PageRequest) (*provider.PageResult, error) {
	return &provider.PageResult{Items: c.items}, nil
}

func (c *fakeConnector) ResolveMedia(_ context.Context, item provider.DiscoveredMedia) (*provider.DiscoveredMedia, error) {
	out := item
	out.Media.URL = c.mediaURL
	if out.Media.FileName == "" {
		out.Media.FileName = "fixture.jpg"
	}
	return &out, nil
}
