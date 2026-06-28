package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	okfcache "ok-folio/internal/cache"
	"ok-folio/internal/config"
	"ok-folio/internal/database"
	"ok-folio/internal/derivatives"
	"ok-folio/internal/gallery"
	"ok-folio/internal/scraper"
	"ok-folio/internal/testguard"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
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

	if err := gormDB.AutoMigrate(&database.DownloadedPhoto{}, &database.ExtractionRun{}, &database.InboxItem{}, &database.ConnectorState{}, &database.ConnectorSource{}, &database.Folio{}); err != nil {
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
			BaseDirectory:        filepath.Join(storageDir, "originals"),
			DailyDirectory:       filepath.Join(storageDir, "daily"),
			DerivativesDirectory: filepath.Join(storageDir, "derivatives"),
			DerivativesMaxBytes:  50 * 1024 * 1024,
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

func ptrTime(t time.Time) *time.Time {
	return &t
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

func TestConnectorSourceSettingsCRUD(t *testing.T) {
	server, _ := setupTestServer(t)
	defer safeShutdown(server)

	createBody := bytes.NewBufferString(`{"type":"telegram","chat_id":"-1001234567890","label":"Fixture channel"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/settings/connector-sources", createBody)
	w := httptest.NewRecorder()
	server.router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%q", w.Code, w.Body.String())
	}
	var created database.ConnectorSource
	if err := json.NewDecoder(w.Body).Decode(&created); err != nil {
		t.Fatalf("decode created connector source: %v", err)
	}
	if created.ID == 0 || created.Type != "telegram" || created.ChatID != "-1001234567890" || !created.Enabled {
		t.Fatalf("unexpected created connector source: %#v", created)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/settings/connector-sources?type=telegram", nil)
	w = httptest.NewRecorder()
	server.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%q", w.Code, w.Body.String())
	}
	var listed connectorSourcesResponse
	if err := json.NewDecoder(w.Body).Decode(&listed); err != nil {
		t.Fatalf("decode connector source list: %v", err)
	}
	if len(listed.Sources) != 1 || listed.Sources[0].ChatID != "-1001234567890" {
		t.Fatalf("unexpected connector source list: %#v", listed)
	}

	patchBody := bytes.NewBufferString(`{"type":"telegram","chat_id":"-1001234567890","label":"Fixture channel","enabled":false}`)
	req = httptest.NewRequest(http.MethodPatch, "/api/v1/settings/connector-sources/"+strconv.FormatUint(created.ID, 10), patchBody)
	w = httptest.NewRecorder()
	server.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("patch status=%d body=%q", w.Code, w.Body.String())
	}
	var updated database.ConnectorSource
	if err := json.NewDecoder(w.Body).Decode(&updated); err != nil {
		t.Fatalf("decode updated connector source: %v", err)
	}
	if updated.Enabled {
		t.Fatalf("expected disabled connector source: %#v", updated)
	}

	patchBody = bytes.NewBufferString(`{"label":""}`)
	req = httptest.NewRequest(http.MethodPatch, "/api/v1/settings/connector-sources/"+strconv.FormatUint(created.ID, 10), patchBody)
	w = httptest.NewRecorder()
	server.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("patch label status=%d body=%q", w.Code, w.Body.String())
	}
	if err := json.NewDecoder(w.Body).Decode(&updated); err != nil {
		t.Fatalf("decode label-updated connector source: %v", err)
	}
	if updated.Enabled || updated.Label != "" {
		t.Fatalf("expected label-only patch to preserve disabled source and clear label: %#v", updated)
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/v1/settings/connector-sources/"+strconv.FormatUint(created.ID, 10), nil)
	w = httptest.NewRecorder()
	server.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("delete status=%d body=%q", w.Code, w.Body.String())
	}
}

func TestFolioAPICRUD(t *testing.T) {
	server, _ := setupTestServer(t)
	defer safeShutdown(server)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/folios", bytes.NewBufferString(`{"name":"Invalid cover","cover_photo_id":0}`))
	w := httptest.NewRecorder()
	server.router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("create zero cover status=%d body=%q", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/folios", bytes.NewBufferString(`{"name":"Sunsets"}`))
	w = httptest.NewRecorder()
	server.router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%q", w.Code, w.Body.String())
	}
	var created database.Folio
	if err := json.NewDecoder(w.Body).Decode(&created); err != nil {
		t.Fatalf("decode created folio: %v", err)
	}
	if created.ID == 0 || created.Name != "Sunsets" {
		t.Fatalf("unexpected created folio: %#v", created)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/folios", nil)
	w = httptest.NewRecorder()
	server.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%q", w.Code, w.Body.String())
	}
	var listed foliosResponse
	if err := json.NewDecoder(w.Body).Decode(&listed); err != nil {
		t.Fatalf("decode folio list: %v", err)
	}
	if len(listed.Folios) != 1 || listed.Folios[0].Name != "Sunsets" {
		t.Fatalf("unexpected folio list: %#v", listed)
	}

	idPath := strconv.FormatUint(created.ID, 10)
	req = httptest.NewRequest(http.MethodGet, "/api/v1/folios/"+idPath, nil)
	w = httptest.NewRecorder()
	server.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("get status=%d body=%q", w.Code, w.Body.String())
	}
	var got database.Folio
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode fetched folio: %v", err)
	}
	if got.ID != created.ID {
		t.Fatalf("unexpected fetched folio: %#v", got)
	}

	req = httptest.NewRequest(http.MethodPatch, "/api/v1/folios/"+idPath, bytes.NewBufferString(`{"name":"Golden hour","cover_photo_id":1}`))
	w = httptest.NewRecorder()
	server.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("patch status=%d body=%q", w.Code, w.Body.String())
	}
	var updated database.Folio
	if err := json.NewDecoder(w.Body).Decode(&updated); err != nil {
		t.Fatalf("decode updated folio: %v", err)
	}
	if updated.Name != "Golden hour" || updated.CoverPhotoID == nil || *updated.CoverPhotoID != 1 {
		t.Fatalf("unexpected updated folio: %#v", updated)
	}

	req = httptest.NewRequest(http.MethodPatch, "/api/v1/folios/"+idPath, bytes.NewBufferString(`{"cover_photo_id":null}`))
	w = httptest.NewRecorder()
	server.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("clear cover status=%d body=%q", w.Code, w.Body.String())
	}
	if err := json.NewDecoder(w.Body).Decode(&updated); err != nil {
		t.Fatalf("decode cover-cleared folio: %v", err)
	}
	if updated.CoverPhotoID != nil {
		t.Fatalf("expected cleared cover override: %#v", updated)
	}

	req = httptest.NewRequest(http.MethodPatch, "/api/v1/folios/"+idPath, bytes.NewBufferString(`{"nam":"Golden hour"}`))
	w = httptest.NewRecorder()
	server.router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("unknown patch field status=%d body=%q", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodPatch, "/api/v1/folios/"+idPath, bytes.NewBufferString(`{"cover_photo_id":0}`))
	w = httptest.NewRecorder()
	server.router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("patch zero cover status=%d body=%q", w.Code, w.Body.String())
	}

	oversizedPatch := `{"name":"` + strings.Repeat("x", maxFolioRequestBytes) + `"}`
	req = httptest.NewRequest(http.MethodPatch, "/api/v1/folios/"+idPath, bytes.NewBufferString(oversizedPatch))
	w = httptest.NewRecorder()
	server.router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("oversized patch status=%d body=%q", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/v1/folios/"+idPath, nil)
	w = httptest.NewRecorder()
	server.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("delete status=%d body=%q", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/folios/"+idPath, nil)
	w = httptest.NewRecorder()
	server.router.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("get deleted status=%d body=%q", w.Code, w.Body.String())
	}
}

func TestConnectorSourceSettingsCreateDisabled(t *testing.T) {
	server, _ := setupTestServer(t)
	defer safeShutdown(server)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/settings/connector-sources", bytes.NewBufferString(`{"type":"telegram","chat_id":"-1001234567890","label":"Paused channel","enabled":false}`))
	w := httptest.NewRecorder()
	server.router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%q", w.Code, w.Body.String())
	}
	var created database.ConnectorSource
	if err := json.NewDecoder(w.Body).Decode(&created); err != nil {
		t.Fatalf("decode created connector source: %v", err)
	}
	if created.Enabled {
		t.Fatalf("expected disabled connector source create to remain disabled: %#v", created)
	}
}

func TestConnectorSourceSettingsRejectInvalidTelegramChatID(t *testing.T) {
	server, _ := setupTestServer(t)
	defer safeShutdown(server)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/settings/connector-sources", bytes.NewBufferString(`{"type":"telegram","chat_id":"channel-name"}`))
	w := httptest.NewRecorder()
	server.router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid chat ID to return 400, got %d body=%q", w.Code, w.Body.String())
	}
}

func TestConnectorSourceSettingsAllowForwardedTelegramSourceWithoutChatAccess(t *testing.T) {
	telegramAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("settings validation should not call Telegram getChat for forwarded source IDs")
	}))
	defer telegramAPI.Close()

	server, _ := setupTestServer(t)
	defer safeShutdown(server)
	server.cfg.Telegram.BotToken = "test-token"
	server.cfg.Telegram.BaseURL = telegramAPI.URL

	req := httptest.NewRequest(http.MethodPost, "/api/v1/settings/connector-sources", bytes.NewBufferString(`{"type":"telegram","chat_id":"-1001234567890","label":"Forwarded channel"}`))
	w := httptest.NewRecorder()
	server.router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected forwarded source create to succeed without getChat access, got %d body=%q", w.Code, w.Body.String())
	}
}

func TestHandleCreatePieceValidUpload(t *testing.T) {
	server, db := setupTestServer(t)
	defer safeShutdown(server)

	statsReq := httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil)
	statsW := httptest.NewRecorder()
	server.router.ServeHTTP(statsW, statsReq)
	if statsW.Code != http.StatusOK {
		t.Fatalf("prime stats status=%d body=%q", statsW.Code, statsW.Body.String())
	}
	var beforeStats map[string]interface{}
	if err := json.NewDecoder(statsW.Body).Decode(&beforeStats); err != nil {
		t.Fatalf("decode primed stats: %v", err)
	}
	if beforeStats["total_photos"].(float64) != 0 {
		t.Fatalf("expected empty primed stats, got %#v", beforeStats)
	}

	body, contentType := createPieceMultipart(t, "piece.jpg", createTestJPEGBytes(t), map[string]string{
		"title":  "Manual Piece",
		"source": "https://example.com/source",
		"artist": "Upload Artist",
		"date":   "2026-06-27",
		"notes":  "Kept from the manual modal.",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/pieces", body)
	req.Header.Set("Content-Type", contentType)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("Expected status 201, got %d body=%q", w.Code, w.Body.String())
	}
	var response createPieceResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("decode upload response: %v", err)
	}
	if response.Duplicate {
		t.Fatal("expected first upload not to be marked duplicate")
	}
	photo := response.Photo
	if photo.Provider != "upload" || photo.URL == "" || photo.Status != "downloaded" {
		t.Fatalf("unexpected upload photo identity: %#v", photo)
	}
	if photo.Title != "Manual Piece" || photo.Artist != "Upload Artist" || photo.SourcePage != "https://example.com/source" || photo.Notes != "Kept from the manual modal." {
		t.Fatalf("uploaded metadata was not stored: %#v", photo)
	}
	if photo.ImageWidth != 12 || photo.ImageHeight != 8 {
		t.Fatalf("expected dimensions 12x8, got %dx%d", photo.ImageWidth, photo.ImageHeight)
	}
	if len(photo.ContentHash) != sha256.Size || photo.PerceptualHash == 0 {
		t.Fatalf("expected content and perceptual hashes, got content=%d phash=%d", len(photo.ContentHash), photo.PerceptualHash)
	}
	if photo.UploadDate == nil || photo.UploadDate.Format("2006-01-02") != "2026-06-27" {
		t.Fatalf("expected parsed upload date, got %v", photo.UploadDate)
	}
	if _, err := os.Stat(filepath.Join(server.cfg.Storage.BaseDirectory, photo.FilePath)); err != nil {
		t.Fatalf("expected uploaded original to be written: %v", err)
	}
	var count int64
	if err := db.Model(&database.DownloadedPhoto{}).Where("provider = ?", "upload").Count(&count).Error; err != nil {
		t.Fatalf("count uploaded rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one uploaded row, got %d", count)
	}

	statsReq = httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil)
	statsW = httptest.NewRecorder()
	server.router.ServeHTTP(statsW, statsReq)
	if statsW.Code != http.StatusOK {
		t.Fatalf("refetch stats status=%d body=%q", statsW.Code, statsW.Body.String())
	}
	var afterStats map[string]interface{}
	if err := json.NewDecoder(statsW.Body).Decode(&afterStats); err != nil {
		t.Fatalf("decode refreshed stats: %v", err)
	}
	if afterStats["total_photos"].(float64) != 1 {
		t.Fatalf("expected stats cache to refresh after upload, got %#v", afterStats)
	}

	thumbReq := httptest.NewRequest(http.MethodGet, "/api/v1/photos/"+strconv.FormatUint(photo.ID, 10)+"/thumbnail?w=400", nil)
	thumbW := httptest.NewRecorder()
	server.router.ServeHTTP(thumbW, thumbReq)
	if thumbW.Code != http.StatusOK {
		t.Fatalf("expected warmed thumbnail to serve, got %d body=%q", thumbW.Code, thumbW.Body.String())
	}
	if got := thumbW.Header().Get("X-OK-Folio-Thumbnail-Cache"); got != "disk-hit" && got != "hot" {
		t.Fatalf("expected thumbnail to be warmed before first read, got cache header %q", got)
	}
}

func TestHandleCreatePieceRejectsNonImage(t *testing.T) {
	server, _ := setupTestServer(t)
	defer safeShutdown(server)

	body, contentType := createPieceMultipart(t, "piece.txt", []byte("not an image"), map[string]string{
		"title": "Nope",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/pieces", body)
	req.Header.Set("Content-Type", contentType)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected status 400, got %d body=%q", w.Code, w.Body.String())
	}
}

func TestHandleCreatePieceReturnsExistingDuplicate(t *testing.T) {
	server, db := setupTestServer(t)
	defer safeShutdown(server)

	imageBytes := createTestJPEGBytes(t)
	firstBody, firstContentType := createPieceMultipart(t, "piece.jpg", imageBytes, map[string]string{
		"title": "First",
	})
	firstReq := httptest.NewRequest(http.MethodPost, "/api/v1/pieces", firstBody)
	firstReq.Header.Set("Content-Type", firstContentType)
	firstW := httptest.NewRecorder()
	server.router.ServeHTTP(firstW, firstReq)
	if firstW.Code != http.StatusCreated {
		t.Fatalf("first upload status=%d body=%q", firstW.Code, firstW.Body.String())
	}
	var firstResponse createPieceResponse
	if err := json.NewDecoder(firstW.Body).Decode(&firstResponse); err != nil {
		t.Fatalf("decode first response: %v", err)
	}

	secondBody, secondContentType := createPieceMultipart(t, "again.jpg", imageBytes, map[string]string{
		"title": "Second",
	})
	secondReq := httptest.NewRequest(http.MethodPost, "/api/v1/pieces", secondBody)
	secondReq.Header.Set("Content-Type", secondContentType)
	secondW := httptest.NewRecorder()
	server.router.ServeHTTP(secondW, secondReq)
	if secondW.Code != http.StatusOK {
		t.Fatalf("duplicate upload status=%d body=%q", secondW.Code, secondW.Body.String())
	}
	var secondResponse createPieceResponse
	if err := json.NewDecoder(secondW.Body).Decode(&secondResponse); err != nil {
		t.Fatalf("decode duplicate response: %v", err)
	}
	if !secondResponse.Duplicate {
		t.Fatal("expected duplicate upload to be reported")
	}
	if secondResponse.Photo.ID != firstResponse.Photo.ID || secondResponse.Photo.Title != "First" {
		t.Fatalf("expected existing piece to be returned, got %#v first=%#v", secondResponse.Photo, firstResponse.Photo)
	}
	var count int64
	if err := db.Model(&database.DownloadedPhoto{}).Where("content_hash = ?", firstResponse.Photo.ContentHash).Count(&count).Error; err != nil {
		t.Fatalf("count duplicate content hash rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected duplicate upload not to create a second row, got %d", count)
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
		DownloadedAt: ptrTime(time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)),
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
		DownloadedAt: ptrTime(time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)),
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
		Query     string                     `json:"query"`
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
	if len(response.Facets.Categories) != 1 || response.Facets.Categories[0].ID != "2" {
		t.Fatalf("Expected filtered category facets to reflect active filters, got %#v", response.Facets.Categories)
	}
	if len(response.Facets.Artists) != 1 || response.Facets.Artists[0].ID != "Artist B" {
		t.Fatalf("Expected filtered artist facets to reflect active filters, got %#v", response.Facets.Artists)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/gallery/catalog?q=old", nil)
	w = httptest.NewRecorder()

	server.handleGalleryCatalog(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected search-filtered status 200, got %d", w.Code)
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode search-filtered gallery catalog: %v", err)
	}
	if response.Total != 1 || len(response.Photos) != 1 || response.Photos[0].Title != "Oldest" {
		t.Fatalf("Expected search to match piece metadata, total=%d photos=%#v", response.Total, response.Photos)
	}
	if response.Query != "old" {
		t.Fatalf("Expected search query echo, got %q", response.Query)
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
			DownloadedAt: ptrTime(time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)),
			Status:       "downloaded",
		},
		{
			URL:          "https://example.com/known-artist.jpg",
			SourcePage:   "https://webgallery/gallery/category/1/",
			Title:        "Known Artist",
			Artist:       "Artist A",
			FilePath:     filepath.Join(server.cfg.Storage.BaseDirectory, "known-artist.jpg"),
			FileName:     "known-artist.jpg",
			DownloadedAt: ptrTime(time.Date(2026, 6, 25, 11, 0, 0, 0, time.UTC)),
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

func TestHandleConnectorStatus(t *testing.T) {
	server, db := setupTestServer(t)
	defer safeShutdown(server)

	sourcePage := "https://example.com/gallery/category/7/source"
	photos := []database.DownloadedPhoto{
		{
			URL:          "webgallery:gallery/category/7/source",
			SourcePage:   sourcePage,
			Title:        "Kept Piece",
			FilePath:     filepath.Join(server.cfg.Storage.BaseDirectory, "kept.jpg"),
			FileName:     "kept.jpg",
			DownloadedAt: ptrTime(time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)),
			Status:       "downloaded",
		},
		{
			URL:          "webgallery:gallery/category/7/failed",
			SourcePage:   sourcePage,
			Title:        "Failed Piece",
			FileName:     "failed.jpg",
			DownloadedAt: ptrTime(time.Date(2026, 6, 25, 12, 5, 0, 0, time.UTC)),
			Status:       "failed",
			ErrorMessage: "upstream returned 500",
		},
		{
			URL:          "https://example.com/telegram.jpg",
			SourcePage:   "https://t.me/sourcechannel/42",
			Title:        "Telegram Piece",
			FileName:     "telegram.jpg",
			DownloadedAt: ptrTime(time.Date(2026, 6, 25, 11, 0, 0, 0, time.UTC)),
			Status:       "downloaded",
		},
		{
			URL:          "telegram:-1001234567890:99:photo-unique-id",
			Title:        "Telegram Failed Piece",
			FileName:     "telegram-failed.jpg",
			DownloadedAt: ptrTime(time.Date(2026, 6, 25, 12, 7, 0, 0, time.UTC)),
			Status:       "failed",
			ErrorMessage: "telegram media expired",
		},
	}
	for _, photo := range photos {
		if err := db.Create(&photo).Error; err != nil {
			t.Fatalf("Failed to create photo: %v", err)
		}
	}

	webLastRun := time.Date(2026, 6, 25, 12, 10, 0, 0, time.UTC)
	telegramLastRun := time.Date(2026, 6, 25, 12, 8, 0, 0, time.UTC)
	states := []database.ConnectorState{
		{
			ProviderID: "webgallery",
			LastRunAt:  &webLastRun,
			LastStatus: "permission_halt",
		},
		{
			ProviderID: "telegram",
			LastRunAt:  &telegramLastRun,
			LastStatus: "completed_with_errors",
		},
	}
	for _, state := range states {
		if err := db.Create(&state).Error; err != nil {
			t.Fatalf("Failed to create connector state: %v", err)
		}
	}

	webRun := database.ExtractionRun{
		StartTime:        ptrTime(time.Date(2026, 6, 25, 12, 1, 0, 0, time.UTC)),
		EndTime:          &webLastRun,
		Provider:         "webgallery",
		Status:           "failed",
		PagesProcessed:   1,
		PhotosFound:      2,
		PhotosDownloaded: 1,
		PhotosFailed:     1,
		ErrorMessage:     "connector failed",
	}
	telegramRun := database.ExtractionRun{
		StartTime:        ptrTime(time.Date(2026, 6, 25, 12, 2, 0, 0, time.UTC)),
		EndTime:          &telegramLastRun,
		Provider:         "telegram",
		Status:           "completed",
		PagesProcessed:   1,
		PhotosFound:      2,
		PhotosDownloaded: 1,
		PhotosFailed:     1,
	}
	if err := db.Create(&webRun).Error; err != nil {
		t.Fatalf("Failed to create webgallery run: %v", err)
	}
	if err := db.Create(&telegramRun).Error; err != nil {
		t.Fatalf("Failed to create telegram run: %v", err)
	}

	if _, err := db.GetConnectorSourceStats(); err != nil {
		t.Fatalf("Failed to get connector source stats: %v", err)
	}
	if _, err := db.GetConnectorStates(); err != nil {
		t.Fatalf("Failed to get connector states: %v", err)
	}
	if _, err := db.GetRecentConnectorErrors(10); err != nil {
		t.Fatalf("Failed to get connector errors: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/streams/connectors/status", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	var response connectorStatusResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode connector status: %v", err)
	}
	if len(response.Connectors) == 0 {
		t.Fatalf("Expected at least webgallery connector, got none")
	}
	if len(response.Connectors) != 2 {
		t.Fatalf("Expected webgallery and telegram connectors, got %#v", response.Connectors)
	}
	connector := response.Connectors[0]
	if connector.ID != "webgallery" || connector.DisplayName != "Web Gallery" {
		t.Fatalf("Expected webgallery connector first, got %#v", connector)
	}
	if connector.Health != "error" || connector.State != "Needs review" {
		t.Fatalf("Expected permission halt state to mark connector unhealthy, got health=%q state=%q", connector.Health, connector.State)
	}
	if connector.Counts.Downloaded != 1 || connector.Counts.Failed != 1 || connector.Counts.Total != 2 {
		t.Fatalf("Expected per-source media counts, got %#v", connector.Counts)
	}
	if len(connector.Sources) != 1 || connector.Sources[0].ID != sourcePage {
		t.Fatalf("Expected source status for source page, got %#v", connector.Sources)
	}
	if connector.Sources[0].Counts.Downloaded != 1 || connector.Sources[0].Counts.Failed != 1 {
		t.Fatalf("Expected source counts, got %#v", connector.Sources[0].Counts)
	}
	if len(connector.RecentRuns) != 1 || connector.RecentRuns[0].Status != "failed" {
		t.Fatalf("Expected recent failed run, got %#v", connector.RecentRuns)
	}
	if connector.RecentRuns[0].ID == telegramRun.ID {
		t.Fatalf("Expected telegram run to stay under telegram connector, got %#v", connector.RecentRuns)
	}
	if len(connector.RecentErrors) != 1 || connector.RecentErrors[0].Message != "upstream returned 500" {
		t.Fatalf("Expected recent failed media error, got %#v", connector.RecentErrors)
	}
	if connector.LastSync == nil || !connector.LastSync.Equal(webLastRun) {
		t.Fatalf("Expected last sync from connector_state, got %v", connector.LastSync)
	}
	for _, item := range response.Connectors {
		if item.ID == "example.com" {
			t.Fatalf("Expected web gallery host source to stay under webgallery, got %#v", response.Connectors)
		}
	}

	var telegram *connectorStatus
	for i := range response.Connectors {
		if response.Connectors[i].ID == "telegram" {
			telegram = &response.Connectors[i]
			break
		}
	}
	if telegram == nil || telegram.DisplayName != "Telegram" {
		t.Fatalf("Expected Telegram connector from t.me source, got %#v", response.Connectors)
	}
	if telegram.Counts.Downloaded != 1 || telegram.Counts.Failed != 1 || telegram.Health != "degraded" {
		t.Fatalf("Expected degraded Telegram counts from source URL and dedupe-key failure, got %#v", telegram)
	}
	if telegram.State != "Degraded" || telegram.LastSync == nil || !telegram.LastSync.Equal(telegramLastRun) {
		t.Fatalf("Expected Telegram state from connector_state, got %#v", telegram)
	}
	if len(telegram.RecentRuns) != 1 || telegram.RecentRuns[0].ID != telegramRun.ID || telegram.RecentRuns[0].Status != "completed" {
		t.Fatalf("Expected Telegram run under Telegram connector, got %#v", telegram.RecentRuns)
	}
	if len(telegram.RecentErrors) != 1 || telegram.RecentErrors[0].Message != "telegram media expired" {
		t.Fatalf("Expected Telegram dedupe-key failure under Telegram connector, got %#v", telegram.RecentErrors)
	}
}

func TestHandleConnectorStatusRendersKnownConnectorsWithoutCatalogRows(t *testing.T) {
	server, _ := setupTestServer(t)
	defer safeShutdown(server)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/streams/connectors/status", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	var response connectorStatusResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode connector status: %v", err)
	}
	if len(response.Connectors) != 2 {
		t.Fatalf("Expected both known connectors without catalog rows, got %#v", response.Connectors)
	}
	for _, connector := range response.Connectors {
		if connector.LastSync != nil || connector.Health != "idle" || connector.State != "Not synced" {
			t.Fatalf("Expected unsynced idle connector without state row, got %#v", connector)
		}
	}
}

func TestBuildConnectorStatusesUsesRunLastSyncWhenStateMissing(t *testing.T) {
	runEnd := time.Date(2026, 6, 25, 12, 10, 0, 0, time.UTC)
	run := database.ExtractionRun{
		ID:        7,
		Provider:  "telegram",
		StartTime: ptrTime(time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)),
		EndTime:   &runEnd,
		Status:    "completed",
	}

	connectors := buildConnectorStatuses(nil, []database.ExtractionRun{run}, nil, nil)

	var telegram *connectorStatus
	for i := range connectors {
		if connectors[i].ID == "telegram" {
			telegram = &connectors[i]
			break
		}
	}
	if telegram == nil {
		t.Fatalf("Expected Telegram connector, got %#v", connectors)
	}
	if telegram.LastSync == nil || !telegram.LastSync.Equal(runEnd) {
		t.Fatalf("Expected last sync fallback from extraction run, got %v", telegram.LastSync)
	}
	if telegram.Health != "healthy" || telegram.State != "Healthy" {
		t.Fatalf("Expected completed run without state to mark connector healthy, got health=%q state=%q", telegram.Health, telegram.State)
	}
}

func TestConnectorHealth(t *testing.T) {
	syncingRun := connectorStatus{
		RecentRuns: []connectorRunStatus{{Status: "running"}},
	}
	failedRun := connectorStatus{
		RecentRuns: []connectorRunStatus{{Status: "failed"}},
	}
	completedState := connectorStatus{
		hasState:   true,
		lastStatus: "completed",
	}
	degradedState := connectorStatus{
		hasState:   true,
		lastStatus: "completed_with_errors",
	}
	permissionHaltState := connectorStatus{
		hasState:   true,
		lastStatus: "permission_halt",
	}
	runningRunWithStaleState := connectorStatus{
		hasState:   true,
		lastStatus: "completed",
		RecentRuns: []connectorRunStatus{{Status: "running"}},
	}
	noState := connectorStatus{}

	tests := []struct {
		name       string
		connector  connectorStatus
		wantHealth string
		wantState  string
	}{
		{name: "running run", connector: syncingRun, wantHealth: "syncing", wantState: "Syncing"},
		{name: "failed run", connector: failedRun, wantHealth: "error", wantState: "Needs review"},
		{name: "completed state", connector: completedState, wantHealth: "healthy", wantState: "Healthy"},
		{name: "completed state with failed rows", connector: connectorStatus{hasState: true, lastStatus: "completed", Counts: connectorCounts{Failed: 1}}, wantHealth: "degraded", wantState: "Degraded"},
		{name: "degraded state", connector: degradedState, wantHealth: "degraded", wantState: "Degraded"},
		{name: "permission halt state", connector: permissionHaltState, wantHealth: "error", wantState: "Needs review"},
		{name: "running run with stale state", connector: runningRunWithStaleState, wantHealth: "syncing", wantState: "Syncing"},
		{name: "no state", connector: noState, wantHealth: "idle", wantState: "Not synced"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			health, state := connectorHealth(tt.connector)
			if health != tt.wantHealth || state != tt.wantState {
				t.Fatalf("connectorHealth() = %q, %q; want %q, %q", health, state, tt.wantHealth, tt.wantState)
			}
		})
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
		UploadDate:   ptrTime(time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC)),
		DownloadedAt: ptrTime(time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)),
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
		ID        uint64 `json:"id"`
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

func TestImageHandlersUseImmutableETagAndConditional304(t *testing.T) {
	server, db := setupTestServer(t)
	defer safeShutdown(server)

	filePath := filepath.Join(server.cfg.Storage.BaseDirectory, "etag-photo.jpg")
	createTestJPEG(t, filePath)
	contentHash := sha256.Sum256([]byte("stable image bytes"))
	photo := database.DownloadedPhoto{
		URL:          "https://example.com/etag-photo.jpg",
		Title:        "ETag Photo",
		FilePath:     filePath,
		FileName:     "etag-photo.jpg",
		FileSize:     1234,
		ContentHash:  contentHash[:],
		Status:       "downloaded",
		DownloadedAt: ptrTime(time.Now()),
	}
	if err := db.Create(&photo).Error; err != nil {
		t.Fatalf("Failed to create photo: %v", err)
	}

	imageURL := "/api/v1/photos/" + strconv.Itoa(int(photo.ID)) + "/image"
	req := httptest.NewRequest(http.MethodGet, imageURL, nil)
	w := httptest.NewRecorder()
	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected image status 200, got %d", w.Code)
	}
	expectedETag := `"` + strconv.FormatUint(photo.ID, 10) + "-" + hex.EncodeToString(contentHash[:]) + `"`
	if w.Header().Get("Cache-Control") != immutableImageCacheControl {
		t.Fatalf("Expected immutable image cache header, got %q", w.Header().Get("Cache-Control"))
	}
	if w.Header().Get("ETag") != expectedETag {
		t.Fatalf("Expected content-hash ETag %q, got %q", expectedETag, w.Header().Get("ETag"))
	}

	if err := os.Remove(filePath); err != nil {
		t.Fatalf("Failed to remove test image: %v", err)
	}

	req = httptest.NewRequest(http.MethodGet, imageURL, nil)
	req.Header.Set("If-None-Match", expectedETag)
	w = httptest.NewRecorder()
	server.router.ServeHTTP(w, req)
	if w.Code != http.StatusNotModified {
		t.Fatalf("Expected image conditional status 304 without reading file, got %d body=%q", w.Code, w.Body.String())
	}
	if w.Body.Len() != 0 {
		t.Fatalf("Expected empty 304 image body, got %q", w.Body.String())
	}

	thumbURL := "/api/v1/photos/" + strconv.Itoa(int(photo.ID)) + "/thumbnail"
	req = httptest.NewRequest(http.MethodGet, thumbURL, nil)
	expectedThumbETag := `"` + "thumb-w400-" + strconv.FormatUint(photo.ID, 10) + "-" + hex.EncodeToString(contentHash[:]) + `"`
	req.Header.Set("If-None-Match", expectedETag)
	w = httptest.NewRecorder()
	server.router.ServeHTTP(w, req)
	if w.Code == http.StatusNotModified {
		t.Fatalf("Expected full image ETag not to validate thumbnail width")
	}
	if w.Header().Get("ETag") != expectedThumbETag {
		t.Fatalf("Expected width-qualified thumbnail ETag %q, got %q", expectedThumbETag, w.Header().Get("ETag"))
	}

	req = httptest.NewRequest(http.MethodGet, thumbURL, nil)
	req.Header.Set("If-None-Match", expectedThumbETag)
	w = httptest.NewRecorder()
	server.router.ServeHTTP(w, req)
	if w.Code != http.StatusNotModified {
		t.Fatalf("Expected thumbnail conditional status 304 without decoding file, got %d body=%q", w.Code, w.Body.String())
	}
	if w.Header().Get("Cache-Control") != immutableImageCacheControl {
		t.Fatalf("Expected immutable thumbnail cache header, got %q", w.Header().Get("Cache-Control"))
	}
}

func TestImageHandlersUseFallbackETagWithoutContentHash(t *testing.T) {
	server, db := setupTestServer(t)
	defer safeShutdown(server)

	filePath := filepath.Join(server.cfg.Storage.BaseDirectory, "fallback-photo.jpg")
	createTestJPEG(t, filePath)
	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("Failed to stat test image: %v", err)
	}
	photo := database.DownloadedPhoto{
		URL:          "https://example.com/fallback-photo.jpg",
		Title:        "Fallback Photo",
		FilePath:     filePath,
		FileName:     "fallback-photo.jpg",
		FileSize:     4321,
		Status:       "downloaded",
		DownloadedAt: ptrTime(time.Now()),
	}
	if err := db.Create(&photo).Error; err != nil {
		t.Fatalf("Failed to create photo: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/photos/"+strconv.Itoa(int(photo.ID))+"/image", nil)
	w := httptest.NewRecorder()
	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected fallback image status 200, got %d", w.Code)
	}
	expectedETag := `"` + strconv.FormatUint(photo.ID, 10) + "-4321-" + strconv.FormatInt(info.ModTime().UnixNano(), 10) + `"`
	if w.Header().Get("ETag") != expectedETag {
		t.Fatalf("Expected fallback ETag %q, got %q", expectedETag, w.Header().Get("ETag"))
	}
	if w.Header().Get("Cache-Control") != immutableImageCacheControl {
		t.Fatalf("Expected immutable fallback cache header, got %q", w.Header().Get("Cache-Control"))
	}
}

func TestImageThumbnailUsesDiskCacheAfterMiss(t *testing.T) {
	server, db := setupTestServer(t)
	defer safeShutdown(server)

	filePath := filepath.Join(server.cfg.Storage.BaseDirectory, "thumbnail-cache.jpg")
	createTestJPEG(t, filePath)
	contentHash := sha256.Sum256([]byte("thumbnail-cache-v1"))
	photo := database.DownloadedPhoto{
		URL:          "https://example.com/thumbnail-cache.jpg",
		Title:        "Thumbnail Cache",
		FilePath:     filePath,
		FileName:     "thumbnail-cache.jpg",
		FileSize:     1234,
		ContentHash:  contentHash[:],
		Status:       "downloaded",
		DownloadedAt: ptrTime(time.Now()),
	}
	if err := db.Create(&photo).Error; err != nil {
		t.Fatalf("Failed to create photo: %v", err)
	}

	thumbURL := "/api/v1/photos/" + strconv.Itoa(int(photo.ID)) + "/thumbnail?w=320"
	req := httptest.NewRequest(http.MethodGet, thumbURL, nil)
	w := httptest.NewRecorder()
	server.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected first thumbnail status 200, got %d body=%q", w.Code, w.Body.String())
	}
	if got := w.Header().Get("X-OK-Folio-Thumbnail-Cache"); got != "miss" {
		t.Fatalf("Expected first thumbnail cache miss, got %q", got)
	}
	firstBody := append([]byte(nil), w.Body.Bytes()...)

	if err := os.WriteFile(filePath, []byte("not a decodable image"), 0o644); err != nil {
		t.Fatalf("Failed to replace original with corrupt bytes: %v", err)
	}

	req = httptest.NewRequest(http.MethodGet, thumbURL, nil)
	w = httptest.NewRecorder()
	server.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected cached thumbnail status 200, got %d body=%q", w.Code, w.Body.String())
	}
	if got := w.Header().Get("X-OK-Folio-Thumbnail-Cache"); got != "disk-hit" {
		t.Fatalf("Expected disk cache hit, got %q", got)
	}
	if !bytes.Equal(w.Body.Bytes(), firstBody) {
		t.Fatalf("Expected cached thumbnail bytes to match first response")
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/photos/"+strconv.Itoa(int(photo.ID))+"/thumbnail?w=321", nil)
	w = httptest.NewRecorder()
	server.router.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("Expected corrupt uncached original to return 404, got %d body=%q", w.Code, w.Body.String())
	}
}

func TestImageThumbnailCacheInvalidatesByContentHash(t *testing.T) {
	server, db := setupTestServer(t)
	defer safeShutdown(server)

	filePath := filepath.Join(server.cfg.Storage.BaseDirectory, "thumbnail-invalidate.jpg")
	createSolidJPEG(t, filePath, color.RGBA{R: 220, G: 20, B: 20, A: 255})
	firstHash := sha256.Sum256([]byte("thumbnail-invalidate-v1"))
	photo := database.DownloadedPhoto{
		URL:          "https://example.com/thumbnail-invalidate.jpg",
		Title:        "Thumbnail Invalidate",
		FilePath:     filePath,
		FileName:     "thumbnail-invalidate.jpg",
		FileSize:     1234,
		ContentHash:  firstHash[:],
		Status:       "downloaded",
		DownloadedAt: ptrTime(time.Now()),
	}
	if err := db.Create(&photo).Error; err != nil {
		t.Fatalf("Failed to create photo: %v", err)
	}

	thumbURL := "/api/v1/photos/" + strconv.Itoa(int(photo.ID)) + "/thumbnail?w=256"
	req := httptest.NewRequest(http.MethodGet, thumbURL, nil)
	w := httptest.NewRecorder()
	server.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected first thumbnail status 200, got %d", w.Code)
	}
	if got := w.Header().Get("X-OK-Folio-Thumbnail-Cache"); got != "miss" {
		t.Fatalf("Expected first thumbnail cache miss, got %q", got)
	}
	firstBody := append([]byte(nil), w.Body.Bytes()...)

	secondHash := sha256.Sum256([]byte("thumbnail-invalidate-v2"))
	photo.ContentHash = secondHash[:]
	if err := db.Save(&photo).Error; err != nil {
		t.Fatalf("Failed to update photo content hash: %v", err)
	}
	createSolidJPEG(t, filePath, color.RGBA{R: 20, G: 180, B: 60, A: 255})

	req = httptest.NewRequest(http.MethodGet, thumbURL, nil)
	w = httptest.NewRecorder()
	server.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected updated thumbnail status 200, got %d body=%q", w.Code, w.Body.String())
	}
	if got := w.Header().Get("X-OK-Folio-Thumbnail-Cache"); got != "miss" {
		t.Fatalf("Expected content-hash change to miss cache, got %q", got)
	}
	if bytes.Equal(w.Body.Bytes(), firstBody) {
		t.Fatalf("Expected content-hash change to generate different thumbnail bytes")
	}
}

func TestThumbnailCachePrunesToConfiguredSize(t *testing.T) {
	dir := t.TempDir()
	cache := derivatives.NewCacheForDir(dir, 50)

	oldEntry := cache.Entry(&database.DownloadedPhoto{ID: 1, ContentHash: bytes.Repeat([]byte{0x11}, 32)}, 320, "")
	newEntry := cache.Entry(&database.DownloadedPhoto{ID: 2, ContentHash: bytes.Repeat([]byte{0x22}, 32)}, 320, "")
	oldPath := oldEntry.Path
	newPath := newEntry.Path
	nonCachePath := filepath.Join(dir, "existing-media", "original.jpg")
	if err := os.MkdirAll(filepath.Dir(oldPath), 0o755); err != nil {
		t.Fatalf("Failed to create old shard: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(newPath), 0o755); err != nil {
		t.Fatalf("Failed to create new shard: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(nonCachePath), 0o755); err != nil {
		t.Fatalf("Failed to create non-cache dir: %v", err)
	}
	if err := os.WriteFile(oldPath, bytes.Repeat([]byte("a"), 40), 0o644); err != nil {
		t.Fatalf("Failed to write old cache file: %v", err)
	}
	if err := os.WriteFile(newPath, bytes.Repeat([]byte("b"), 40), 0o644); err != nil {
		t.Fatalf("Failed to write new cache file: %v", err)
	}
	if err := os.WriteFile(nonCachePath, bytes.Repeat([]byte("c"), 40), 0o644); err != nil {
		t.Fatalf("Failed to write non-cache JPEG: %v", err)
	}
	oldTime := time.Now().Add(-time.Hour)
	if err := os.Chtimes(oldPath, oldTime, oldTime); err != nil {
		t.Fatalf("Failed to set old cache time: %v", err)
	}
	if err := os.Chtimes(nonCachePath, oldTime.Add(-time.Hour), oldTime.Add(-time.Hour)); err != nil {
		t.Fatalf("Failed to set non-cache time: %v", err)
	}

	if err := cache.Prune(); err != nil {
		t.Fatalf("Prune failed: %v", err)
	}
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatalf("Expected oldest cache file to be pruned, stat err=%v", err)
	}
	if _, err := os.Stat(newPath); err != nil {
		t.Fatalf("Expected newest cache file to remain: %v", err)
	}
	if _, err := os.Stat(nonCachePath); err != nil {
		t.Fatalf("Expected non-cache JPEG to remain: %v", err)
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
		ID        uint64 `json:"id"`
		Favorite  bool   `json:"favorite"`
		Available bool   `json:"available"`
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

func TestGalleryCatalogCacheUsesEpochInvalidation(t *testing.T) {
	server, db := setupTestServer(t)
	defer safeShutdown(server)

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr(), Protocol: 2})
	server.cache = okfcache.NewForRedis(rdb, zerolog.Nop())

	photo := database.DownloadedPhoto{
		URL:          "https://example.com/cached-favorite.jpg",
		Title:        "Cached Favorite",
		FilePath:     filepath.Join(server.cfg.Storage.BaseDirectory, "cached-favorite.jpg"),
		FileName:     "cached-favorite.jpg",
		Status:       "downloaded",
		DownloadedAt: ptrTime(time.Now()),
	}
	if err := db.Create(&photo).Error; err != nil {
		t.Fatalf("Failed to create photo: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/gallery/catalog", nil)
	w := httptest.NewRecorder()
	server.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected first catalog status 200, got %d", w.Code)
	}

	var first struct {
		Facets struct {
			Favorites []struct {
				Favorite bool  `json:"favorite"`
				Count    int64 `json:"count"`
			} `json:"favorites"`
		} `json:"facets"`
	}
	if err := json.NewDecoder(w.Body).Decode(&first); err != nil {
		t.Fatalf("Failed to decode first catalog: %v", err)
	}
	if favoriteCount(first.Facets.Favorites, true) != 0 {
		t.Fatalf("Expected initial favorites count 0, got %#v", first.Facets.Favorites)
	}
	oldKey, err := okfcache.CatalogKey(0, cacheGalleryCatalogFilters{}, 50, 0)
	if err != nil {
		t.Fatalf("Failed to build legacy catalog cache key: %v", err)
	}
	newKey, err := okfcache.CatalogKey(0, galleryCatalogCacheShape(cacheGalleryCatalogFilters{}), 50, 0)
	if err != nil {
		t.Fatalf("Failed to build versioned catalog cache key: %v", err)
	}
	if mr.Exists(oldKey) {
		t.Fatalf("Expected catalog response not to use legacy unversioned cache key %q", oldKey)
	}
	if !mr.Exists(newKey) {
		t.Fatalf("Expected catalog response to use versioned cache key %q", newKey)
	}

	favoriteURL := "/api/v1/photos/" + strconv.Itoa(int(photo.ID)) + "/favorite"
	req = httptest.NewRequest(http.MethodPost, favoriteURL, nil)
	w = httptest.NewRecorder()
	server.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected favorite status 200, got %d", w.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/gallery/catalog", nil)
	w = httptest.NewRecorder()
	server.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected second catalog status 200, got %d", w.Code)
	}

	var second struct {
		Facets struct {
			Favorites []struct {
				Favorite bool  `json:"favorite"`
				Count    int64 `json:"count"`
			} `json:"favorites"`
		} `json:"facets"`
	}
	if err := json.NewDecoder(w.Body).Decode(&second); err != nil {
		t.Fatalf("Failed to decode second catalog: %v", err)
	}
	if favoriteCount(second.Facets.Favorites, true) != 1 {
		t.Fatalf("Expected epoch-bumped favorites count 1, got %#v", second.Facets.Favorites)
	}
}

func TestGalleryCatalogUsesPrivateETagAndConditional304(t *testing.T) {
	server, db := setupTestServer(t)
	defer safeShutdown(server)

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr(), Protocol: 2})
	server.cache = okfcache.NewForRedis(rdb, zerolog.Nop())

	photo := database.DownloadedPhoto{
		URL:          "https://example.com/catalog-etag.jpg",
		Title:        "Catalog ETag",
		FilePath:     filepath.Join(server.cfg.Storage.BaseDirectory, "catalog-etag.jpg"),
		FileName:     "catalog-etag.jpg",
		Status:       "downloaded",
		DownloadedAt: ptrTime(time.Now()),
	}
	if err := db.Create(&photo).Error; err != nil {
		t.Fatalf("Failed to create photo: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/gallery/catalog?q=Catalog", nil)
	w := httptest.NewRecorder()
	server.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected catalog status 200, got %d", w.Code)
	}
	if got := w.Header().Get("Cache-Control"); got != catalogCacheControl {
		t.Fatalf("Expected private catalog cache header, got %q", got)
	}
	etag := w.Header().Get("ETag")
	if etag == "" {
		t.Fatalf("Expected catalog ETag")
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/gallery/catalog?q=Catalog", nil)
	req.Header.Set("If-None-Match", etag)
	w = httptest.NewRecorder()
	server.router.ServeHTTP(w, req)
	if w.Code != http.StatusNotModified {
		t.Fatalf("Expected catalog conditional status 304, got %d body=%q", w.Code, w.Body.String())
	}
	if w.Body.Len() != 0 {
		t.Fatalf("Expected empty catalog 304 body, got %q", w.Body.String())
	}
}

func TestGalleryCatalogDoesNot304WhenCachePassthrough(t *testing.T) {
	server, db := setupTestServer(t)
	defer safeShutdown(server)

	mr := miniredis.RunT(t)
	addr := mr.Addr()
	mr.Close()
	rdb := redis.NewClient(&redis.Options{Addr: addr, Protocol: 2})
	server.cache = okfcache.NewForRedis(rdb, zerolog.Nop())

	photo := database.DownloadedPhoto{
		URL:          "https://example.com/passthrough-catalog-etag.jpg",
		Title:        "Passthrough Catalog ETag",
		FilePath:     filepath.Join(server.cfg.Storage.BaseDirectory, "passthrough-catalog-etag.jpg"),
		FileName:     "passthrough-catalog-etag.jpg",
		Status:       "downloaded",
		DownloadedAt: ptrTime(time.Now()),
	}
	if err := db.Create(&photo).Error; err != nil {
		t.Fatalf("Failed to create photo: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/gallery/catalog?q=Passthrough", nil)
	w := httptest.NewRecorder()
	server.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected first catalog status 200, got %d", w.Code)
	}
	etag := w.Header().Get("ETag")
	if etag == "" {
		t.Fatalf("Expected catalog ETag")
	}

	photo.Title = "Passthrough Catalog Updated"
	if err := db.Save(&photo).Error; err != nil {
		t.Fatalf("Failed to update photo: %v", err)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/gallery/catalog?q=Passthrough", nil)
	req.Header.Set("If-None-Match", etag)
	w = httptest.NewRecorder()
	server.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected passthrough catalog status 200, got %d body=%q", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Cache-Control"); got != catalogCacheControl {
		t.Fatalf("Expected private catalog cache header, got %q", got)
	}

	var response struct {
		Photos []struct {
			Title string `json:"title"`
		} `json:"photos"`
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode passthrough catalog: %v", err)
	}
	if len(response.Photos) != 1 || response.Photos[0].Title != "Passthrough Catalog Updated" {
		t.Fatalf("Expected passthrough request to fetch updated DB row, got %#v", response.Photos)
	}
}

func TestStatsStreamUsesNoStore(t *testing.T) {
	server, _ := setupTestServer(t)
	defer safeShutdown(server)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stream/stats", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	server.handleStatsStream(w, req)

	if got := w.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Expected SSE no-store cache header, got %q", got)
	}
}

func favoriteCount(facets []struct {
	Favorite bool  `json:"favorite"`
	Count    int64 `json:"count"`
}, favorite bool) int64 {
	for _, facet := range facets {
		if facet.Favorite == favorite {
			return facet.Count
		}
	}
	return 0
}

func createTestJPEG(t *testing.T, filePath string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		t.Fatalf("Failed to create test image directory: %v", err)
	}
	file, err := os.Create(filePath)
	if err != nil {
		t.Fatalf("Failed to create test image: %v", err)
	}
	defer file.Close()

	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			img.Set(x, y, color.RGBA{R: uint8(20 * x), G: uint8(20 * y), B: 80, A: 255})
		}
	}
	if err := jpeg.Encode(file, img, &jpeg.Options{Quality: 80}); err != nil {
		t.Fatalf("Failed to encode test image: %v", err)
	}
}

func createTestJPEGBytes(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 12, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 12; x++ {
			img.Set(x, y, color.RGBA{R: uint8(15 * x), G: uint8(25 * y), B: 90, A: 255})
		}
	}
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
		t.Fatalf("Failed to encode test image bytes: %v", err)
	}
	return buf.Bytes()
}

func createPieceMultipart(t *testing.T, fileName string, fileBytes []byte, fields map[string]string) (*bytes.Buffer, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		t.Fatalf("create multipart file: %v", err)
	}
	if _, err := part.Write(fileBytes); err != nil {
		t.Fatalf("write multipart file: %v", err)
	}
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatalf("write multipart field %s: %v", key, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	return &body, writer.FormDataContentType()
}

func createSolidJPEG(t *testing.T, filePath string, c color.Color) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		t.Fatalf("Failed to create test image directory: %v", err)
	}
	file, err := os.Create(filePath)
	if err != nil {
		t.Fatalf("Failed to create test image: %v", err)
	}
	defer file.Close()

	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			img.Set(x, y, c)
		}
	}
	if err := jpeg.Encode(file, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("Failed to encode test image: %v", err)
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

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	rateLimitedHandler := server.rateLimitMiddleware(handler)

	// Cheap cache-served reads (the gallery fetches dozens of thumbnails per
	// screen) must NEVER be rate-limited, even in a large burst from one client.
	for i := 0; i < 100; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/photos/1/thumbnail", nil)
		req.RemoteAddr = "10.0.0.1:1000"
		w := httptest.NewRecorder()
		rateLimitedHandler.ServeHTTP(w, req)
		if w.Code == http.StatusTooManyRequests {
			t.Fatalf("GET request %d was rate-limited; cache-served reads must never 429", i)
		}
	}

	// Expensive, state-changing requests are still throttled (per client IP).
	rateLimitedCount := 0
	for i := 0; i < 3*RateLimitBurst; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/extract", nil)
		req.RemoteAddr = "10.0.0.2:1000"
		w := httptest.NewRecorder()
		rateLimitedHandler.ServeHTTP(w, req)
		if w.Code == http.StatusTooManyRequests {
			rateLimitedCount++
		}
	}
	if rateLimitedCount == 0 {
		t.Error("Expected some POST requests to be rate-limited")
	}

	// A different client IP gets its own fresh bucket and is not starved by the
	// client that just exhausted its limit above.
	freshOK := 0
	for i := 0; i < RateLimitBurst; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/extract", nil)
		req.RemoteAddr = "10.0.0.3:1000"
		w := httptest.NewRecorder()
		rateLimitedHandler.ServeHTTP(w, req)
		if w.Code == http.StatusOK {
			freshOK++
		}
	}
	if freshOK == 0 {
		t.Error("Expected a fresh client IP to have its own rate-limit bucket")
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
