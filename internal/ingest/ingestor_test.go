package ingest

import (
	"context"
	"crypto/sha256"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"ok-folio/internal/cache"
	"ok-folio/internal/config"
	"ok-folio/internal/database"
	"ok-folio/internal/provider"
	"ok-folio/internal/scraper"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestRunConnectorCursorProvenanceHashAndEpochBatches(t *testing.T) {
	ctx := context.Background()
	body := []byte("fixture image bytes")
	imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(body)
	}))
	defer imageServer.Close()

	connector := &fakeConnector{
		pages: map[string]*provider.PageResult{
			"": {
				Items: []provider.DiscoveredMedia{fakeItem("one", imageServer.URL+"/one.jpg")},
				Pagination: provider.Pagination{
					NextCursor: "cursor-2",
					HasNext:    true,
				},
			},
			"cursor-2": {
				Items: []provider.DiscoveredMedia{fakeItem("two", imageServer.URL+"/two.jpg")},
			},
		},
	}
	db, c, ing := setupIngestorTest(t, connector)

	result, err := ing.RunConnector(ctx, connector)
	if err != nil {
		t.Fatalf("RunConnector failed: %v", err)
	}
	if result.PagesProcessed != 2 || result.PhotosDownloaded != 2 || result.PhotosFound != 2 {
		t.Fatalf("unexpected result: %#v", result)
	}
	if !reflect.DeepEqual(connector.requests, []provider.PageRequest{{Page: 1}, {Page: 2, Cursor: "cursor-2"}}) {
		t.Fatalf("unexpected cursor requests: %#v", connector.requests)
	}

	var photos []database.DownloadedPhoto
	if err := db.Order("url").Find(&photos).Error; err != nil {
		t.Fatalf("query photos: %v", err)
	}
	if len(photos) != 2 {
		t.Fatalf("expected 2 photos, got %d", len(photos))
	}
	sum := sha256.Sum256(body)
	for _, photo := range photos {
		if photo.Provider != "fixture" {
			t.Fatalf("expected provider provenance fixture, got %#v", photo)
		}
		if !strings.HasPrefix(photo.SourcePage, "https://fixture.test/source/") {
			t.Fatalf("expected source page provenance, got %#v", photo)
		}
		if !reflect.DeepEqual(photo.ContentHash, sum[:]) {
			t.Fatalf("content hash mismatch: got %x want %x", photo.ContentHash, sum[:])
		}
		if exists, err := c.Seen(ctx, "fixture", photo.URL); err != nil || !exists {
			t.Fatalf("expected seen-set write for %q, exists=%v err=%v", photo.URL, exists, err)
		}
		if got, err := c.Raw().Get(ctx, cache.DedupeHashKey(photo.ContentHash)).Result(); err != nil || got == "" {
			t.Fatalf("expected content-hash write-through, got=%q err=%v", got, err)
		}
	}
	if epoch := c.Epoch(ctx); epoch != 2 {
		t.Fatalf("expected one epoch bump per committed page batch, got %d", epoch)
	}

	var run database.ExtractionRun
	if err := db.First(&run).Error; err != nil {
		t.Fatalf("query run: %v", err)
	}
	if run.Provider != "fixture" || run.Status != "completed" || run.PhotosDownloaded != 2 {
		t.Fatalf("unexpected run record: %#v", run)
	}
}

func TestRunConnectorWithOptionsHonorsAllowedPages(t *testing.T) {
	ctx := context.Background()
	body := []byte("fixture image bytes")
	imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(body)
	}))
	defer imageServer.Close()

	connector := &fakeConnector{
		pagesByPage: map[int]*provider.PageResult{
			3: {
				Items: []provider.DiscoveredMedia{fakeItem("three", imageServer.URL+"/three.jpg")},
				Pagination: provider.Pagination{
					Page:     3,
					NextPage: 4,
					HasNext:  true,
				},
			},
			4: {
				Items: []provider.DiscoveredMedia{fakeItem("four", imageServer.URL+"/four.jpg")},
			},
		},
	}
	_, _, ing := setupIngestorTest(t, connector)

	result, err := ing.RunConnectorWithOptions(ctx, connector, RunOptions{AllowedPages: []int{3}})
	if err != nil {
		t.Fatalf("RunConnectorWithOptions failed: %v", err)
	}
	if result.PagesProcessed != 1 || result.PhotosDownloaded != 1 {
		t.Fatalf("unexpected result: %#v", result)
	}
	if !reflect.DeepEqual(connector.requests, []provider.PageRequest{{Page: 3}}) {
		t.Fatalf("unexpected page requests: %#v", connector.requests)
	}
}

func TestRunConnectorSkipsSeenBeforeResolve(t *testing.T) {
	ctx := context.Background()
	imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("new image"))
	}))
	defer imageServer.Close()

	seen := fakeItem("seen", imageServer.URL+"/seen.jpg")
	fresh := fakeItem("fresh", imageServer.URL+"/fresh.jpg")
	connector := &fakeConnector{
		pages: map[string]*provider.PageResult{
			"": {Items: []provider.DiscoveredMedia{seen, fresh}},
		},
	}
	_, c, ing := setupIngestorTest(t, connector)
	if err := c.MarkSeen(ctx, "fixture", "fixture:seen"); err != nil {
		t.Fatalf("seed seen set: %v", err)
	}

	result, err := ing.RunConnector(ctx, connector)
	if err != nil {
		t.Fatalf("RunConnector failed: %v", err)
	}
	if result.PhotosDownloaded != 1 || result.PhotosSkipped != 1 {
		t.Fatalf("expected one download and one seen skip, got %#v", result)
	}
	if !reflect.DeepEqual(connector.resolved, []string{"fixture:fresh"}) {
		t.Fatalf("seen item was resolved: %#v", connector.resolved)
	}
	if epoch := c.Epoch(ctx); epoch != 1 {
		t.Fatalf("expected one committed batch epoch bump, got %d", epoch)
	}
}

func TestRunConnectorProviderErrorRouting(t *testing.T) {
	tests := []struct {
		name        string
		kind        provider.ErrorKind
		wantFailed  int
		wantHalted  bool
		wantBackoff time.Duration
	}{
		{name: "temporary", kind: provider.ErrorKindTemporary, wantBackoff: 25 * time.Millisecond},
		{name: "rate_limit", kind: provider.ErrorKindRateLimit, wantBackoff: 25 * time.Millisecond},
		{name: "not_found", kind: provider.ErrorKindNotFound, wantFailed: 1},
		{name: "parse", kind: provider.ErrorKindParse, wantFailed: 1},
		{name: "missing_media", kind: provider.ErrorKindMissingMedia, wantFailed: 1},
		{name: "permission", kind: provider.ErrorKindPermission, wantHalted: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := fakeItem(tt.name, "https://fixture.test/media.jpg")
			connector := &fakeConnector{
				pages: map[string]*provider.PageResult{"": {Items: []provider.DiscoveredMedia{item}}},
				resolveErr: &provider.ProviderError{
					ProviderID: "fixture",
					Kind:       tt.kind,
					RetryAfter: tt.wantBackoff,
					Err:        errors.New(string(tt.kind)),
				},
			}
			db, _, ing := setupIngestorTest(t, connector)
			var backedOff []time.Duration
			ing.backoff = func(_ context.Context, delay time.Duration) error {
				backedOff = append(backedOff, delay)
				return nil
			}

			result, err := ing.RunConnector(context.Background(), connector)
			if err != nil {
				t.Fatalf("RunConnector failed: %v", err)
			}
			if result.PhotosFailed != tt.wantFailed || result.Halted != tt.wantHalted {
				t.Fatalf("unexpected result: %#v", result)
			}
			if tt.wantBackoff > 0 && !reflect.DeepEqual(backedOff, []time.Duration{tt.wantBackoff}) {
				t.Fatalf("expected retry backoff %v, got %#v", tt.wantBackoff, backedOff)
			}
			if tt.wantFailed > 0 {
				var failed database.DownloadedPhoto
				if err := db.Where("url = ? AND status = ?", item.DedupeKey.String(), "failed").First(&failed).Error; err != nil {
					t.Fatalf("expected failed download record: %v", err)
				}
			}
			if tt.wantHalted {
				var run database.ExtractionRun
				if err := db.First(&run).Error; err != nil {
					t.Fatalf("query run: %v", err)
				}
				if run.Status != "failed" || run.ErrorMessage == "" {
					t.Fatalf("expected halted connector surfaced in run record, got %#v", run)
				}
			}
		})
	}
}

func TestRunConnectorDiscoveryProviderErrorRouting(t *testing.T) {
	tests := []struct {
		name        string
		kind        provider.ErrorKind
		wantFailed  int
		wantHalted  bool
		wantErr     bool
		wantBackoff time.Duration
	}{
		{name: "temporary", kind: provider.ErrorKindTemporary, wantErr: true, wantBackoff: 25 * time.Millisecond},
		{name: "rate_limit", kind: provider.ErrorKindRateLimit, wantErr: true, wantBackoff: 25 * time.Millisecond},
		{name: "parse", kind: provider.ErrorKindParse, wantFailed: 1},
		{name: "permission", kind: provider.ErrorKindPermission, wantHalted: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			connector := &fakeConnector{
				discoverErr: &provider.ProviderError{
					ProviderID: "fixture",
					Kind:       tt.kind,
					RetryAfter: tt.wantBackoff,
					Err:        errors.New(string(tt.kind)),
				},
			}
			db, _, ing := setupIngestorTest(t, connector)
			var backedOff []time.Duration
			ing.backoff = func(_ context.Context, delay time.Duration) error {
				backedOff = append(backedOff, delay)
				return nil
			}

			result, err := ing.RunConnector(context.Background(), connector)
			if err != nil && !tt.wantErr {
				t.Fatalf("RunConnector failed: %v", err)
			}
			if err == nil && tt.wantErr {
				t.Fatal("expected RunConnector to fail after bounded discovery retry")
			}
			if result.PhotosFailed != tt.wantFailed || result.Halted != tt.wantHalted {
				t.Fatalf("unexpected result: %#v", result)
			}
			if tt.wantBackoff > 0 && !reflect.DeepEqual(backedOff, []time.Duration{tt.wantBackoff}) {
				t.Fatalf("expected retry backoff %v, got %#v", tt.wantBackoff, backedOff)
			}
			if tt.wantFailed > 0 {
				var failed database.DownloadedPhoto
				if err := db.Where("url = ? AND status = ?", "fixture:discovery-error", "failed").First(&failed).Error; err != nil {
					t.Fatalf("expected failed discovery record: %v", err)
				}
			}
			if tt.wantHalted {
				var run database.ExtractionRun
				if err := db.First(&run).Error; err != nil {
					t.Fatalf("query run: %v", err)
				}
				if run.Status != "failed" || run.ErrorMessage == "" {
					t.Fatalf("expected halted discovery surfaced in run record, got %#v", run)
				}
			}
		})
	}
}

func TestRunConnectorRedactsTelegramTokenFromFailedDownload(t *testing.T) {
	ctx := context.Background()
	token := "123456:secret-token"
	imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	downloadURL := imageServer.URL + "/file/bot" + token + "/photos/file.jpg"
	imageServer.Close()
	connector := &fakeConnector{
		pages: map[string]*provider.PageResult{
			"": {Items: []provider.DiscoveredMedia{fakeItem("telegram", downloadURL)}},
		},
		providerID: "telegram",
	}
	db, _, ing := setupIngestorTest(t, connector)

	result, err := ing.RunConnector(ctx, connector)
	if err != nil {
		t.Fatalf("RunConnector failed: %v", err)
	}
	if result.PhotosFailed != 1 {
		t.Fatalf("expected failed download, got %#v", result)
	}

	var failed database.DownloadedPhoto
	if err := db.Where("url = ? AND status = ?", "fixture:telegram", "failed").First(&failed).Error; err != nil {
		t.Fatalf("expected failed download record: %v", err)
	}
	if strings.Contains(failed.ErrorMessage, token) {
		t.Fatalf("token leaked in failed download error: %q", failed.ErrorMessage)
	}
	if !strings.Contains(failed.ErrorMessage, "bot<redacted>/photos/file.jpg") {
		t.Fatalf("expected redacted telegram URL, got %q", failed.ErrorMessage)
	}
}

func setupIngestorTest(t *testing.T, connector provider.Connector) (*database.DB, *testCache, *Ingestor) {
	t.Helper()

	gormDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := gormDB.AutoMigrate(&database.DownloadedPhoto{}, &database.ExtractionRun{}, &database.InboxItem{}, &database.ConnectorState{}); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}
	db := &database.DB{DB: gormDB}

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	c := &testCache{Client: cache.NewForRedis(rdb, zerolog.Nop()), rdb: rdb}

	root := t.TempDir()
	cfg := &config.Config{
		Source:  config.SourceConfig{BaseURL: "https://fixture.test/gallery"},
		Storage: config.StorageConfig{BaseDirectory: filepath.Join(root, "originals"), DailyDirectory: filepath.Join(root, "daily")},
		Download: config.DownloadConfig{
			ConcurrentLimit: 1,
			Timeout:         5 * time.Second,
		},
		Retry: config.RetryConfig{MaxAttempts: 1},
	}
	s := scraper.NewWithProvider(cfg, db, zerolog.New(os.Stderr).Level(zerolog.Disabled), connector)
	ing := New(db, c.Client, s, zerolog.Nop())
	return db, c, ing
}

type testCache struct {
	*cache.Client
	rdb *redis.Client
}

func (c *testCache) Raw() *redis.Client {
	return c.rdb
}

func fakeItem(id string, mediaURL string) provider.DiscoveredMedia {
	return provider.DiscoveredMedia{
		ProviderID: "fixture",
		DedupeKey:  provider.DedupeKey{ProviderID: "fixture", Value: id},
		Source: provider.SourceMetadata{
			URL:        "https://fixture.test/source/" + id,
			ExternalID: "source-" + id,
		},
		Media: provider.MediaMetadata{
			URL:        mediaURL,
			FileName:   id + ".jpg",
			ExternalID: "media-" + id,
		},
		Title:       "Fixture " + id,
		Artist:      "Fixture Artist",
		PublishedAt: time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC),
	}
}

type fakeConnector struct {
	providerID  string
	pages       map[string]*provider.PageResult
	pagesByPage map[int]*provider.PageResult
	discoverErr error
	resolveErr  error
	requests    []provider.PageRequest
	resolved    []string
}

func (c *fakeConnector) Provider() provider.Source {
	id := c.providerID
	if id == "" {
		id = "fixture"
	}
	return provider.Source{ID: id, DisplayName: "Fixture"}
}

func (c *fakeConnector) DiscoverPage(_ context.Context, req provider.PageRequest) (*provider.PageResult, error) {
	c.requests = append(c.requests, req)
	if c.discoverErr != nil {
		return nil, c.discoverErr
	}
	if c.pagesByPage != nil {
		page := c.pagesByPage[req.Page]
		if page == nil {
			return &provider.PageResult{}, nil
		}
		return page, nil
	}
	page := c.pages[req.Cursor]
	if page == nil {
		return &provider.PageResult{}, nil
	}
	return page, nil
}

func (c *fakeConnector) ResolveMedia(_ context.Context, item provider.DiscoveredMedia) (*provider.DiscoveredMedia, error) {
	c.resolved = append(c.resolved, item.DedupeKey.String())
	if c.resolveErr != nil {
		return nil, c.resolveErr
	}
	out := item
	out.ProviderID = c.Provider().ID
	if out.Media.URL == "" {
		out.Media.URL = "https://fixture.test/media/" + out.Media.FileName
	}
	return &out, nil
}
