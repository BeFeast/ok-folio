package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"ok-folio/internal/config"
	"ok-folio/internal/database"
	"ok-folio/internal/gallery"
	"ok-folio/internal/scraper"
	"ok-folio/internal/testguard"

	"github.com/rs/zerolog"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// setupTestServer creates a test server with in-memory database
func setupTestServer(t *testing.T) (*Server, *database.DB) {
	// Create test database
	gormDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	if err := gormDB.AutoMigrate(&database.DownloadedPhoto{}, &database.ExtractionRun{}, &database.InboxItem{}); err != nil {
		t.Fatalf("Failed to migrate test database: %v", err)
	}

	db := &database.DB{DB: gormDB}

	storageDir := t.TempDir()
	cfg := &config.Config{
		API: config.APIConfig{
			Host: "0.0.0.0",
			Port: 8080,
		},
		Scheduler: config.SchedulerConfig{
			Pages: []int{1, 2, 3},
		},
		Storage: config.StorageConfig{
			BaseDirectory:  filepath.Join(storageDir, "originals"),
			DailyDirectory: filepath.Join(storageDir, "daily"),
		},
	}
	if err := testguard.ValidateConfig(cfg); err != nil {
		t.Fatalf("unsafe API test config: %v", err)
	}

	// Create test scraper (nil for now, we'll override in tests that need it)
	scr := &scraper.Scraper{}

	// Create test logger (silent for tests)
	testLogger := zerolog.New(os.Stderr).Level(zerolog.Disabled)

	// Create server
	server := New(cfg, db, scr, testLogger)

	return server, db
}

// safeShutdown shuts down server and waits for workers to stop
func safeShutdown(server *Server) {
	server.Shutdown()
	time.Sleep(100 * time.Millisecond) // Give workers time to stop
}

func TestIsDashboardRouteIncludesPieceDetail(t *testing.T) {
	if !isDashboardRoute("/pieces/123") {
		t.Fatalf("Expected piece detail route to fall back to dashboard index")
	}
}

func TestHandleHealth_Healthy(t *testing.T) {
	server, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	server.handleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.NewDecoder(w.Body).Decode(&response)

	if response["status"] != "healthy" && response["status"] != "degraded" {
		t.Errorf("Expected status 'healthy' or 'degraded', got %v", response["status"])
	}

	safeShutdown(server)
}

func TestHandleStats_Empty(t *testing.T) {
	server, _ := setupTestServer(t)
	defer safeShutdown(server)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil)
	w := httptest.NewRecorder()

	server.handleStats(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var stats map[string]interface{}
	json.NewDecoder(w.Body).Decode(&stats)

	if stats["total_photos"].(float64) != 0 {
		t.Errorf("Expected 0 photos, got %v", stats["total_photos"])
	}
}

func TestHandleStats_WithData(t *testing.T) {
	server, db := setupTestServer(t)
	defer safeShutdown(server)

	// Insert test data
	photo := &database.DownloadedPhoto{
		URL:      "https://example.com/photo1.jpg",
		Artist:   "Test Artist",
		FilePath: filepath.Join(server.cfg.Storage.BaseDirectory, "photo1.jpg"),
		FileName: "photo1.jpg",
		FileSize: 1000,
		Status:   "downloaded",
	}
	db.Create(photo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil)
	w := httptest.NewRecorder()

	server.handleStats(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestHandleGalleryDecision(t *testing.T) {
	server, _ := setupTestServer(t)
	defer safeShutdown(server)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/gallery/decision", nil)
	w := httptest.NewRecorder()

	server.handleGalleryDecision(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	var response gallery.Decision
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode gallery decision: %v", err)
	}

	if response.Product != "OK Folio" {
		t.Fatalf("Expected OK Folio product identity, got %q", response.Product)
	}
	if response.ChosenDirection != gallery.CustomMVPDirection {
		t.Fatalf("Expected custom gallery MVP direction, got %q", response.ChosenDirection)
	}
	if response.Prototype.LiveRoute != "/api/v1/gallery/decision" {
		t.Fatalf("Expected live route to point at decision endpoint, got %q", response.Prototype.LiveRoute)
	}
	if len(response.Options) < 3 {
		t.Fatalf("Expected PhotoPrism and custom gallery options, got %d", len(response.Options))
	}
	if err := gallery.ValidateDecision(response); err != nil {
		t.Fatalf("Expected response to validate: %v", err)
	}
}

func TestHandleGalleryCatalog(t *testing.T) {
	server, db := setupTestServer(t)
	defer safeShutdown(server)

	newPhoto := database.DownloadedPhoto{
		URL:          "https://example.com/new.jpg",
		SourcePage:   "https://webgallery/gallery/category/2/new",
		Title:        "Newest",
		Artist:       "Artist B",
		FilePath:     filepath.Join(server.cfg.Storage.BaseDirectory, "new.jpg"),
		FileName:     "new.jpg",
		DownloadedAt: time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC),
		FileSize:     2048,
		Status:       "downloaded",
	}
	oldPhoto := database.DownloadedPhoto{
		URL:          "https://example.com/old.jpg",
		SourcePage:   "https://webgallery/gallery/category/1/old",
		Title:        "Oldest",
		Artist:       "Artist A",
		FilePath:     filepath.Join(server.cfg.Storage.BaseDirectory, "old.jpg"),
		FileName:     "old.jpg",
		DownloadedAt: time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC),
		FileSize:     1024,
		Status:       "downloaded",
	}
	failedPhoto := database.DownloadedPhoto{
		URL:        "https://example.com/failed.jpg",
		SourcePage: "https://webgallery/gallery/failed",
		Title:      "Failed",
		FileName:   "failed.jpg",
		Status:     "failed",
	}

	for _, photo := range []database.DownloadedPhoto{oldPhoto, newPhoto, failedPhoto} {
		if err := db.Create(&photo).Error; err != nil {
			t.Fatalf("Failed to create photo: %v", err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/gallery/catalog?limit=1&offset=0", nil)
	w := httptest.NewRecorder()

	server.handleGalleryCatalog(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	var response struct {
		Photos    []database.DownloadedPhoto `json:"photos"`
		Total     int64                      `json:"total"`
		Limit     int                        `json:"limit"`
		Offset    int                        `json:"offset"`
		Provider  string                     `json:"provider"`
		Source    string                     `json:"source"`
		Category  string                     `json:"category"`
		Artist    string                     `json:"artist"`
		Providers []struct {
			ID          string `json:"id"`
			DisplayName string `json:"display_name"`
			Count       int64  `json:"count"`
			Sources     []struct {
				ID          string `json:"id"`
				DisplayName string `json:"display_name"`
				Count       int64  `json:"count"`
			} `json:"sources"`
		} `json:"providers"`
		Facets struct {
			Sources []struct {
				ID          string `json:"id"`
				DisplayName string `json:"display_name"`
				Count       int64  `json:"count"`
			} `json:"sources"`
			Categories []struct {
				ID          string `json:"id"`
				DisplayName string `json:"display_name"`
				Count       int64  `json:"count"`
			} `json:"categories"`
			Artists []struct {
				ID          string `json:"id"`
				DisplayName string `json:"display_name"`
				Count       int64  `json:"count"`
			} `json:"artists"`
			Favorites []struct {
				ID          string `json:"id"`
				DisplayName string `json:"display_name"`
				Favorite    bool   `json:"favorite"`
				Count       int64  `json:"count"`
			} `json:"favorites"`
		} `json:"facets"`
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode gallery catalog: %v", err)
	}

	if response.Total != 2 {
		t.Fatalf("Expected 2 downloaded photos, got %d", response.Total)
	}
	if response.Limit != 1 || response.Offset != 0 {
		t.Fatalf("Expected pagination limit=1 offset=0, got limit=%d offset=%d", response.Limit, response.Offset)
	}
	if len(response.Photos) != 1 {
		t.Fatalf("Expected 1 photo in page, got %d", len(response.Photos))
	}
	if response.Photos[0].Title != "Newest" {
		t.Fatalf("Expected newest downloaded photo first, got %q", response.Photos[0].Title)
	}
	if len(response.Providers) != 1 || response.Providers[0].ID != "webgallery" {
		t.Fatalf("Expected webgallery provider facet, got %#v", response.Providers)
	}
	if response.Providers[0].Count != 2 || len(response.Providers[0].Sources) != 2 {
		t.Fatalf("Expected provider facet counts from downloaded media only, got %#v", response.Providers[0])
	}
	if len(response.Facets.Categories) != 2 || len(response.Facets.Artists) != 2 || len(response.Facets.Favorites) != 2 {
		t.Fatalf("Expected category, artist, and favorite facets, got %#v", response.Facets)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/gallery/catalog?provider=webgallery&source=https%3A%2F%2Fwebgallery%2Fgallery%2Fcategory%2F1%2Fold", nil)
	w = httptest.NewRecorder()

	server.handleGalleryCatalog(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected filtered status 200, got %d", w.Code)
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode filtered gallery catalog: %v", err)
	}
	if response.Total != 1 || len(response.Photos) != 1 || response.Photos[0].Title != "Oldest" {
		t.Fatalf("Expected source-filtered oldest photo, total=%d photos=%#v", response.Total, response.Photos)
	}
	if response.Provider != "webgallery" || response.Source != "https://webgallery/gallery/category/1/old" {
		t.Fatalf("Expected filter echo, got provider=%q source=%q", response.Provider, response.Source)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/gallery/catalog?category=2&artist=Artist+B", nil)
	w = httptest.NewRecorder()

	server.handleGalleryCatalog(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected category/artist filtered status 200, got %d", w.Code)
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode category/artist filtered gallery catalog: %v", err)
	}
	if response.Total != 1 || len(response.Photos) != 1 || response.Photos[0].Title != "Newest" {
		t.Fatalf("Expected category and artist filters to return newest photo, total=%d photos=%#v", response.Total, response.Photos)
	}
	if response.Category != "2" || response.Artist != "Artist B" {
		t.Fatalf("Expected category/artist filter echo, got category=%q artist=%q", response.Category, response.Artist)
	}
}

func TestHandleGalleryCatalogFiltersEmptyArtist(t *testing.T) {
	server, db := setupTestServer(t)
	defer safeShutdown(server)

	photos := []database.DownloadedPhoto{
		{
			URL:          "https://example.com/unknown-artist.jpg",
			SourcePage:   "https://webgallery/gallery/category/1/",
			Title:        "Unknown Artist",
			FilePath:     filepath.Join(server.cfg.Storage.BaseDirectory, "unknown-artist.jpg"),
			FileName:     "unknown-artist.jpg",
			DownloadedAt: time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC),
			Status:       "downloaded",
		},
		{
			URL:          "https://example.com/known-artist.jpg",
			SourcePage:   "https://webgallery/gallery/category/1/",
			Title:        "Known Artist",
			Artist:       "Artist A",
			FilePath:     filepath.Join(server.cfg.Storage.BaseDirectory, "known-artist.jpg"),
			FileName:     "known-artist.jpg",
			DownloadedAt: time.Date(2026, 6, 25, 11, 0, 0, 0, time.UTC),
			Status:       "downloaded",
		},
	}

	for _, photo := range photos {
		if err := db.Create(&photo).Error; err != nil {
			t.Fatalf("Failed to create photo: %v", err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/gallery/catalog?artist=", nil)
	w := httptest.NewRecorder()

	server.handleGalleryCatalog(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	var response struct {
		Photos []database.DownloadedPhoto `json:"photos"`
		Total  int64                      `json:"total"`
		Artist string                     `json:"artist"`
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode gallery catalog: %v", err)
	}
	if response.Total != 1 || len(response.Photos) != 1 || response.Photos[0].Title != "Unknown Artist" {
		t.Fatalf("Expected empty artist filter to return unknown artist photo, total=%d photos=%#v", response.Total, response.Photos)
	}
	if response.Artist != "" {
		t.Fatalf("Expected empty artist filter echo, got %q", response.Artist)
	}
}

func TestHandlePhotoDetailIncludesProvenanceAndFavorite(t *testing.T) {
	server, db := setupTestServer(t)
	defer safeShutdown(server)

	photo := database.DownloadedPhoto{
		URL:          "https://example.com/piece.jpg",
		SourcePage:   "https://webgallery/gallery/category/7/piece",
		Title:        "Detail Piece",
		Artist:       "Detail Artist",
		FilePath:     filepath.Join(server.cfg.Storage.BaseDirectory, "piece.jpg"),
		FileName:     "piece.jpg",
		UploadDate:   time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC),
		DownloadedAt: time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC),
		FileSize:     4096,
		Favorite:     true,
		Status:       "downloaded",
	}
	if err := db.Create(&photo).Error; err != nil {
		t.Fatalf("Failed to create photo: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/photos/"+strconv.Itoa(int(photo.ID)), nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	var response struct {
		ID        uint   `json:"id"`
		Source    string `json:"source"`
		Provider  string `json:"provider"`
		Category  string `json:"category"`
		Artist    string `json:"artist"`
		Favorite  bool   `json:"favorite"`
		SourceURL string `json:"source_page"`
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode photo detail: %v", err)
	}

	if response.ID != photo.ID || response.Artist != "Detail Artist" || !response.Favorite {
		t.Fatalf("Expected detail identity and favorite state, got %#v", response)
	}
	if response.Provider != "webgallery" || response.Source != "webgallery/gallery/category/7/piece" || response.Category != "Category 7" {
		t.Fatalf("Expected provenance fields from source page, got %#v", response)
	}
	if response.SourceURL != photo.SourcePage {
		t.Fatalf("Expected raw source page to be preserved, got %q", response.SourceURL)
	}
}

func TestHandleFavoritePersistsLocally(t *testing.T) {
	server, db := setupTestServer(t)
	defer safeShutdown(server)

	photo := database.DownloadedPhoto{
		URL:      "https://example.com/favorite-toggle.jpg",
		Title:    "Favorite Toggle",
		FilePath: filepath.Join(server.cfg.Storage.BaseDirectory, "favorite-toggle.jpg"),
		FileName: "favorite-toggle.jpg",
		Status:   "downloaded",
	}
	if err := db.Create(&photo).Error; err != nil {
		t.Fatalf("Failed to create photo: %v", err)
	}

	favoriteURL := "/api/v1/photos/" + strconv.Itoa(int(photo.ID)) + "/favorite"
	req := httptest.NewRequest(http.MethodPost, favoriteURL, nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected add favorite status 200, got %d", w.Code)
	}

	stored, err := db.GetPhotoByID(photo.ID)
	if err != nil {
		t.Fatalf("Failed to fetch stored photo: %v", err)
	}
	if !stored.Favorite {
		t.Fatalf("Expected favorite to persist true")
	}

	req = httptest.NewRequest(http.MethodGet, favoriteURL, nil)
	w = httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected get favorite status 200, got %d", w.Code)
	}
	var response struct {
		ID        uint `json:"id"`
		Favorite  bool `json:"favorite"`
		Available bool `json:"available"`
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode favorite status: %v", err)
	}
	if response.ID != photo.ID || !response.Favorite || !response.Available {
		t.Fatalf("Expected favorite status from local DB, got %#v", response)
	}

	req = httptest.NewRequest(http.MethodDelete, favoriteURL, nil)
	w = httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected remove favorite status 200, got %d", w.Code)
	}
	stored, err = db.GetPhotoByID(photo.ID)
	if err != nil {
		t.Fatalf("Failed to fetch stored photo: %v", err)
	}
	if stored.Favorite {
		t.Fatalf("Expected favorite to persist false")
	}
}

func TestHandleInboxReturnsOnlyExceptions(t *testing.T) {
	server, db := setupTestServer(t)
	defer safeShutdown(server)

	items := []database.InboxItem{
		{
			ProviderID: "telegram",
			DedupeKey:  "telegram:source-1:media-1",
			SourceID:   "source-1",
			MediaID:    "media-1",
			Status:     "duplicate",
			Reason:     "dedupe key already kept",
		},
		{
			ProviderID: "webgallery",
			SourceID:   "source-2",
			Status:     "ambiguous",
			Reason:     "missing connector dedupe key",
		},
		{
			ProviderID: "webgallery",
			DedupeKey:  "webgallery:source-3:media-3",
			Status:     "dismissed",
		},
	}
	for _, item := range items {
		if err := db.Create(&item).Error; err != nil {
			t.Fatalf("Failed to create inbox item: %v", err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/inbox?limit=10", nil)
	w := httptest.NewRecorder()

	server.handleInbox(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	var response struct {
		Items  []database.InboxItem `json:"items"`
		Total  int64                `json:"total"`
		Limit  int                  `json:"limit"`
		Offset int                  `json:"offset"`
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode inbox response: %v", err)
	}
	if response.Total != 2 || len(response.Items) != 2 {
		t.Fatalf("Expected only duplicate and ambiguous inbox items, got total=%d items=%#v", response.Total, response.Items)
	}
	for _, item := range response.Items {
		if item.Status != "duplicate" && item.Status != "ambiguous" {
			t.Fatalf("Inbox returned non-exception status: %#v", item)
		}
	}
}

func TestHandleGetRuns_Empty(t *testing.T) {
	server, _ := setupTestServer(t)
	defer safeShutdown(server)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/runs", nil)
	w := httptest.NewRecorder()

	server.handleGetRuns(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.NewDecoder(w.Body).Decode(&response)

	runs := response["runs"].([]interface{})
	if len(runs) != 0 {
		t.Errorf("Expected 0 runs, got %d", len(runs))
	}
}

func TestHandleGetRuns_WithLimit(t *testing.T) {
	server, db := setupTestServer(t)
	defer safeShutdown(server)

	// Create test runs
	for i := 0; i < 5; i++ {
		run, _ := db.StartExtractionRun()
		db.FinishExtractionRun(run.ID, "completed", "")
		time.Sleep(10 * time.Millisecond)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/runs?limit=3", nil)
	w := httptest.NewRecorder()

	server.handleGetRuns(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.NewDecoder(w.Body).Decode(&response)

	runs := response["runs"].([]interface{})
	if len(runs) != 3 {
		t.Errorf("Expected 3 runs, got %d", len(runs))
	}
}

func TestHandleGetRuns_InvalidLimit(t *testing.T) {
	server, _ := setupTestServer(t)
	defer safeShutdown(server)

	// Test with negative limit (should use default)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/runs?limit=-1", nil)
	w := httptest.NewRecorder()

	server.handleGetRuns(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Test with limit exceeding max (should use default)
	req = httptest.NewRequest(http.MethodGet, "/api/v1/runs?limit=999", nil)
	w = httptest.NewRecorder()

	server.handleGetRuns(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestHandleExtractPage_InvalidPage(t *testing.T) {
	server, _ := setupTestServer(t)
	defer safeShutdown(server)

	// Test with non-numeric page
	req := httptest.NewRequest(http.MethodPost, "/api/v1/extract/page/abc", nil)
	w := httptest.NewRecorder()

	// Manually set URL param
	req.SetPathValue("page", "abc")

	server.handleExtractPage(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	// Test with page < 1
	req = httptest.NewRequest(http.MethodPost, "/api/v1/extract/page/0", nil)
	w = httptest.NewRecorder()
	req.SetPathValue("page", "0")

	server.handleExtractPage(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	server, _ := setupTestServer(t)
	defer safeShutdown(server)

	// Create a test handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Wrap with rate limit middleware
	rateLimitedHandler := server.rateLimitMiddleware(handler)

	// Make many requests rapidly
	successCount := 0
	rateLimitedCount := 0

	for i := 0; i < 30; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		rateLimitedHandler.ServeHTTP(w, req)

		if w.Code == http.StatusOK {
			successCount++
		} else if w.Code == http.StatusTooManyRequests {
			rateLimitedCount++
		}
	}

	// Should have some rate-limited requests
	if rateLimitedCount == 0 {
		t.Error("Expected some requests to be rate-limited")
	}
}

func TestWriteJSON(t *testing.T) {
	server, _ := setupTestServer(t)
	defer safeShutdown(server)

	w := httptest.NewRecorder()
	data := map[string]string{"message": "test"}

	server.writeJSON(w, http.StatusOK, data)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if w.Header().Get("Content-Type") != "application/json" {
		t.Error("Expected Content-Type to be application/json")
	}

	var response map[string]string
	json.NewDecoder(w.Body).Decode(&response)

	if response["message"] != "test" {
		t.Errorf("Expected message 'test', got '%s'", response["message"])
	}
}

func TestWriteError(t *testing.T) {
	server, _ := setupTestServer(t)
	defer safeShutdown(server)

	w := httptest.NewRecorder()

	server.writeError(w, http.StatusBadRequest, "test error")

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	var response map[string]string
	json.NewDecoder(w.Body).Decode(&response)

	if response["error"] != "test error" {
		t.Errorf("Expected error 'test error', got '%s'", response["error"])
	}
}

func TestRouterIntegration(t *testing.T) {
	server, _ := setupTestServer(t)
	defer safeShutdown(server)

	tests := []struct {
		name           string
		method         string
		path           string
		expectedStatus int
	}{
		{"Health check", http.MethodGet, "/health", http.StatusOK},
		{"Stats endpoint", http.MethodGet, "/api/v1/stats", http.StatusOK},
		{"Runs endpoint", http.MethodGet, "/api/v1/runs", http.StatusOK},
		{"Runs with limit", http.MethodGet, "/api/v1/runs?limit=5", http.StatusOK},
		{"Gallery catalog endpoint", http.MethodGet, "/api/v1/gallery/catalog", http.StatusOK},
		{"Gallery decision endpoint", http.MethodGet, "/api/v1/gallery/decision", http.StatusOK},
		{"Invalid endpoint", http.MethodGet, "/invalid", http.StatusNotFound},
		{"Extract - wrong method", http.MethodGet, "/api/v1/extract", http.StatusMethodNotAllowed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			server.router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

func TestExtractionWorker(t *testing.T) {
	server, _ := setupTestServer(t)

	// Send a job
	jobExecuted := false
	job := func() {
		jobExecuted = true
	}

	server.jobQueue <- job

	// Wait a bit for worker to process
	time.Sleep(100 * time.Millisecond)

	if !jobExecuted {
		t.Error("Expected job to be executed by worker")
	}

	safeShutdown(server)
}

func TestShutdown(t *testing.T) {
	server, _ := setupTestServer(t)

	// Shutdown should not panic
	safeShutdown(server)

	// Verify context is cancelled
	select {
	case <-server.ctx.Done():
		// Success
	default:
		t.Error("Expected context to be cancelled after shutdown")
	}
}

func TestNew(t *testing.T) {
	gormDB, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	db := &database.DB{DB: gormDB}
	cfg := &config.Config{
		API: config.APIConfig{Port: 8080},
		Scheduler: config.SchedulerConfig{
			Pages: []int{1, 2, 3},
		},
	}
	scr := &scraper.Scraper{}
	testLogger := zerolog.New(os.Stderr)

	server := New(cfg, db, scr, testLogger)

	if server == nil {
		t.Error("Expected server to be created")
	}
	if server.cfg != cfg {
		t.Error("Expected cfg to be set")
	}
	if server.db != db {
		t.Error("Expected db to be set")
	}
	if server.scraper != scr {
		t.Error("Expected scraper to be set")
	}
	if server.router == nil {
		t.Error("Expected router to be initialized")
	}
	if server.jobQueue == nil {
		t.Error("Expected jobQueue to be initialized")
	}
	if server.limiter == nil {
		t.Error("Expected limiter to be initialized")
	}

	safeShutdown(server)
}

func fillExtractionCapacity(t *testing.T, server *Server) func() {
	t.Helper()

	release := make(chan struct{})
	started := make(chan struct{}, MaxConcurrentExtractions)
	activeJob := func() {
		started <- struct{}{}
		<-release
	}

	for i := 0; i < MaxConcurrentExtractions; i++ {
		server.jobQueue <- activeJob
	}
	for i := 0; i < MaxConcurrentExtractions; i++ {
		select {
		case <-started:
		case <-time.After(time.Second):
			close(release)
			t.Fatal("timed out waiting for extraction workers to become busy")
		}
	}

	queuedJob := func() {
		<-release
	}
	for i := 0; i < ExtractionJobQueueSize; i++ {
		server.jobQueue <- queuedJob
	}

	return func() {
		close(release)
	}
}

func TestHandleExtract_QueueFull(t *testing.T) {
	server, _ := setupTestServer(t)
	defer safeShutdown(server)

	releaseJobs := fillExtractionCapacity(t, server)
	defer releaseJobs()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/extract", nil)
	w := httptest.NewRecorder()

	server.handleExtract(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Expected status 429, got %d", w.Code)
	}
}

func TestHandleExtractPage_QueueFull(t *testing.T) {
	server, _ := setupTestServer(t)
	defer safeShutdown(server)

	releaseJobs := fillExtractionCapacity(t, server)
	defer releaseJobs()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/extract/page/1", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Expected status 429, got %d", w.Code)
	}
}
