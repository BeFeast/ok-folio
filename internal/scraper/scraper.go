package scraper

import (
	"context"
	"crypto/sha256"
	"fmt"
	"hash"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	okfcache "ok-folio/internal/cache"
	"ok-folio/internal/config"
	"ok-folio/internal/database"
	"ok-folio/internal/dataquality"
	"ok-folio/internal/derivatives"
	"ok-folio/internal/exif"
	"ok-folio/internal/photoprism"
	"ok-folio/internal/provider"
	"ok-folio/internal/provider/webgallery"
	"ok-folio/pkg/retry"

	"github.com/rs/zerolog"
)

const (
	// Directory permissions for created folders
	DefaultDirPermissions = 0755
	// HTTP status code for rate limiting
	StatusTooManyRequests = 429
)

// RateLimitError indicates the server returned a 429 response
type RateLimitError struct {
	RetryAfter time.Duration
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("rate limited, retry after %v", e.RetryAfter)
}

type Scraper struct {
	cfg              *config.Config
	db               *database.DB
	logger           zerolog.Logger
	client           *http.Client
	provider         provider.Connector
	photoprismClient *photoprism.Client
	thumbHotCache    *okfcache.Client
	thumbWarmSem     chan struct{}
}

func New(cfg *config.Config, db *database.DB, logger zerolog.Logger) *Scraper {
	var ppClient *photoprism.Client
	if cfg.PhotoPrism.Enabled {
		ppClient = photoprism.New(
			cfg.PhotoPrism.ServiceURL,
			cfg.PhotoPrism.Username,
			cfg.PhotoPrism.Password,
			logger.With().Str("component", "photoprism").Logger(),
		)
	}

	client := &http.Client{
		Timeout: cfg.Download.Timeout,
	}
	var thumbHotCache *okfcache.Client
	var thumbWarmSem chan struct{}
	if cfg.Storage.WarmOnIngest {
		thumbHotCache = okfcache.New(context.Background(), cfg.Cache, logger.With().Str("component", "thumbnail-warmer-cache").Logger())
		thumbWarmSem = make(chan struct{}, 2)
	}

	return &Scraper{
		cfg:              cfg,
		db:               db,
		logger:           logger,
		photoprismClient: ppClient,
		client:           client,
		provider: webgallery.New(webgallery.Config{
			BaseURL:          cfg.Source.BaseURL,
			UserAgent:        cfg.Download.UserAgent,
			RateLimitBackoff: cfg.Download.RateLimitBackoff,
			Retry: retry.Config{
				MaxAttempts:  cfg.Retry.MaxAttempts,
				InitialDelay: cfg.Retry.InitialDelay,
				MaxDelay:     cfg.Retry.MaxDelay,
				Multiplier:   cfg.Retry.Multiplier,
			},
		}, client, logger.With().Str("provider", webgallery.ProviderID).Logger()),
		thumbHotCache: thumbHotCache,
		thumbWarmSem:  thumbWarmSem,
	}
}

func NewWithProvider(cfg *config.Config, db *database.DB, logger zerolog.Logger, connector provider.Connector) *Scraper {
	s := New(cfg, db, logger)
	s.provider = connector
	return s
}

// ScrapePage scrapes a single page and downloads photos
func (s *Scraper) ScrapePage(ctx context.Context, page int) (int, int, int, error) {
	pageURL := fmt.Sprintf("%s?pager=%d", s.cfg.Source.BaseURL, page)
	s.logger.Info().Int("page", page).Str("url", pageURL).Msg("Scraping page")

	result, err := s.provider.DiscoverPage(ctx, provider.PageRequest{Page: page})
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to discover media: %w", err)
	}

	s.logger.Info().Int("count", len(result.Items)).Msg("Found media items")

	var (
		downloaded int
		skipped    int
		failed     int
		mu         sync.Mutex
		wg         sync.WaitGroup
		semaphore  = make(chan struct{}, s.cfg.Download.ConcurrentLimit)
		keyLocks   = make(map[string]*sync.Mutex)
		keyLocksMu sync.Mutex
	)
	getKeyLock := func(key string) *sync.Mutex {
		keyLocksMu.Lock()
		defer keyLocksMu.Unlock()
		if keyLocks[key] == nil {
			keyLocks[key] = &sync.Mutex{}
		}
		return keyLocks[key]
	}

	for _, item := range result.Items {
		select {
		case <-ctx.Done():
			return downloaded, skipped, failed, ctx.Err()
		default:
		}

		wg.Add(1)
		go func(mediaItem provider.DiscoveredMedia) {
			defer wg.Done()

			semaphore <- struct{}{}        // Acquire
			defer func() { <-semaphore }() // Release

			dedupeKey := StableDedupeKey(mediaItem)
			if dedupeKey == "" {
				if err := s.recordInboxException(mediaItem, "ambiguous", "missing connector dedupe key"); err != nil {
					s.logger.Error().Err(err).Msg("Failed to record ambiguous inbox item")
					mu.Lock()
					failed++
					mu.Unlock()
					return
				}
				mu.Lock()
				skipped++
				mu.Unlock()
				return
			}

			keyLock := getKeyLock(dedupeKey)
			keyLock.Lock()
			defer keyLock.Unlock()

			// Check if already downloaded
			exists, err := s.IsMediaAlreadyKept(mediaItem)
			if err != nil {
				s.logger.Error().Err(err).Str("dedupe_key", dedupeKey).Msg("Failed to check if photo exists")
				mu.Lock()
				failed++
				mu.Unlock()
				return
			}

			if exists {
				if err := s.recordInboxException(mediaItem, "duplicate", "dedupe key already kept"); err != nil {
					s.logger.Error().Err(err).Str("dedupe_key", dedupeKey).Msg("Failed to record duplicate inbox item")
					mu.Lock()
					failed++
					mu.Unlock()
					return
				}
				s.logger.Debug().Str("dedupe_key", dedupeKey).Msg("Photo already kept, adding duplicate exception")
				mu.Lock()
				skipped++
				mu.Unlock()
				return
			}

			// Add delay between downloads to avoid rate limiting
			if s.cfg.Download.DelayBetween > 0 {
				time.Sleep(s.cfg.Download.DelayBetween)
			}

			// Download photo
			if err := s.downloadPhoto(ctx, mediaItem); err != nil {
				s.logger.Error().Err(err).Str("dedupe_key", dedupeKey).Msg("Failed to download photo")

				// Record the failed download to the database
				if dbErr := s.db.RecordFailedDownload(dedupeKey, err.Error()); dbErr != nil {
					s.logger.Error().Err(dbErr).Str("dedupe_key", dedupeKey).Msg("Failed to record failed download")
				}

				mu.Lock()
				failed++
				mu.Unlock()
				return
			}

			mu.Lock()
			downloaded++
			mu.Unlock()
		}(item)
	}

	wg.Wait()

	return downloaded, skipped, failed, nil
}

// downloadPhoto downloads a single photo and its metadata
func (s *Scraper) downloadPhoto(ctx context.Context, item provider.DiscoveredMedia) error {
	dedupeKey := StableDedupeKey(item)
	if dedupeKey == "" {
		return fmt.Errorf("missing connector dedupe key")
	}
	s.logger.Debug().Str("dedupe_key", dedupeKey).Msg("Downloading photo")

	resolved, err := s.provider.ResolveMedia(ctx, item)
	if err != nil {
		return fmt.Errorf("failed to resolve media: %w", err)
	}

	_, err = s.DownloadResolvedMedia(ctx, *resolved, s.provider.Provider().ID)
	return err
}

// DownloadResolvedMedia persists a provider-resolved media item while preserving
// the legacy scraper's storage, EXIF, and daily symlink behavior.
func (s *Scraper) DownloadResolvedMedia(ctx context.Context, resolved provider.DiscoveredMedia, providerID string) (*database.DownloadedPhoto, error) {
	photo, _, err := s.DownloadResolvedMediaOrDuplicate(ctx, resolved, providerID)
	return photo, err
}

// DownloadResolvedMediaOrDuplicate persists a provider-resolved media item and
// reports whether it won the catalog insert. Exact duplicate losers are routed
// to Inbox and are not returned as errors.
func (s *Scraper) DownloadResolvedMediaOrDuplicate(ctx context.Context, resolved provider.DiscoveredMedia, providerID string) (*database.DownloadedPhoto, bool, error) {
	dedupeKey := StableDedupeKey(resolved)
	if dedupeKey == "" {
		return nil, false, fmt.Errorf("missing connector dedupe key")
	}
	if providerID == "" {
		providerID = resolved.ProviderID
	}
	if providerID == "" {
		providerID = database.DefaultProvider
	}

	// Sanitize artist name for folder
	sanitizedArtist := sanitizeFolderName(resolved.Artist)
	artistDir := filepath.Join(s.cfg.Storage.BaseDirectory, sanitizedArtist)

	// Validate artist directory path
	if err := validatePath(s.cfg.Storage.BaseDirectory, artistDir); err != nil {
		return nil, false, fmt.Errorf("invalid artist directory path: %w", err)
	}

	// Create artist directory
	if err := os.MkdirAll(artistDir, DefaultDirPermissions); err != nil {
		return nil, false, fmt.Errorf("failed to create artist directory: %w", err)
	}

	// Download image
	fileName := resolved.Media.FileName
	if fileName == "" {
		fileName = filepath.Base(resolved.Media.URL)
	}
	filePath := filepath.Join(artistDir, fileName)
	title := dataquality.NormalizeTitle(resolved.Title, fileName, filePath)

	// Validate file path
	if err := validatePath(s.cfg.Storage.BaseDirectory, filePath); err != nil {
		return nil, false, fmt.Errorf("invalid file path: %w", err)
	}

	fileSize, contentHash, err := s.downloadFile(ctx, resolved.Media.URL, filePath)
	if err != nil {
		return nil, false, fmt.Errorf("failed to download image file: %w", err)
	}
	embedded, err := exif.ReadEmbeddedMetadata(filePath)
	if err != nil {
		s.logger.Warn().Err(err).Str("file", filePath).Msg("Failed to read embedded image metadata")
	}

	// Set EXIF metadata
	if s.cfg.EXIF.SetArtist || s.cfg.EXIF.SetDate || s.cfg.EXIF.SetTitle {
		metadata := exif.Metadata{
			Title:      title,
			Artist:     resolved.Artist,
			UploadDate: resolved.PublishedAt,
		}

		if err := exif.SetMetadata(filePath, metadata, &s.cfg.EXIF); err != nil {
			s.logger.Warn().Err(err).Str("file", filePath).Msg("Failed to set EXIF metadata")
		}
	}

	// Record in database
	uploadDate := publishedAtPtr(resolved.PublishedAt)
	photo := &database.DownloadedPhoto{
		URL:          dedupeKey,
		SourcePage:   resolved.Source.URL,
		Title:        title,
		Artist:       resolved.Artist,
		UploadDate:   uploadDate,
		FilePath:     filePath,
		FileName:     fileName,
		ImageWidth:   embedded.Width,
		ImageHeight:  embedded.Height,
		CapturedAt:   embedded.CapturedAt,
		CameraMake:   embedded.CameraMake,
		CameraModel:  embedded.CameraModel,
		LensModel:    embedded.LensModel,
		Orientation:  embedded.Orientation,
		GPSLatitude:  embedded.GPSLatitude,
		GPSLongitude: embedded.GPSLongitude,
		FileSize:     fileSize,
		Status:       "downloaded",
		Provider:     providerID,
		ContentHash:  contentHash,
	}

	duplicate := &database.InboxItem{
		ProviderID: providerID,
		DedupeKey:  dedupeKey,
		SourceID:   resolved.Source.ExternalID,
		MediaID:    resolved.Media.ExternalID,
		SourceURL:  resolved.Source.URL,
		Title:      title,
		Artist:     resolved.Artist,
		Status:     "duplicate",
		Reason:     "exact content hash already kept",
	}
	kept, err := s.db.RecordDownloadOrDuplicate(photo, duplicate)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to record download in database")
		return nil, false, fmt.Errorf("failed to record download in database: %w", err)
	}
	if !kept {
		s.logger.Info().
			Str("file", fileName).
			Str("artist", resolved.Artist).
			Msg("Routed duplicate photo to Inbox")
		return photo, false, nil
	}

	// Create daily symlink
	if err := s.createDailySymlink(filePath, fileName); err != nil {
		s.logger.Warn().
			Err(err).
			Str("file", fileName).
			Str("path", filePath).
			Msg("Failed to create daily symlink")
		// Don't return error here - symlink failure is not critical
	}

	s.logger.Info().
		Str("file", fileName).
		Str("artist", resolved.Artist).
		Msg("Successfully downloaded photo")
	s.scheduleWarmThumbnailsOnIngest(*photo)

	return photo, true, nil
}

func publishedAtPtr(publishedAt time.Time) *time.Time {
	if publishedAt.IsZero() {
		return nil
	}
	return &publishedAt
}

func (s *Scraper) scheduleWarmThumbnailsOnIngest(photo database.DownloadedPhoto) {
	if !s.cfg.Storage.WarmOnIngest || len(s.cfg.Storage.WarmOnIngestWidths) == 0 || s.thumbWarmSem == nil {
		return
	}
	widths := append([]int(nil), s.cfg.Storage.WarmOnIngestWidths...)
	select {
	case s.thumbWarmSem <- struct{}{}:
	default:
		s.logger.Warn().Uint64("photo_id", photo.ID).Msg("Thumbnail warm-on-ingest skipped because workers are full")
		return
	}
	go func() {
		defer func() { <-s.thumbWarmSem }()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		result, err := derivatives.WarmOnePhoto(ctx, s.cfg.Storage, &photo, derivatives.WarmPhotoOptions{
			Widths:   widths,
			HotCache: s.thumbHotCache,
			HotTTL:   24 * time.Hour,
		}, s.logger.With().Str("component", "thumbnail-warmer").Logger())
		if err != nil {
			s.logger.Warn().Err(err).Uint64("photo_id", photo.ID).Msg("Thumbnail warm-on-ingest failed")
			return
		}
		if result.Failed > 0 || result.Missing > 0 {
			s.logger.Warn().
				Uint64("photo_id", photo.ID).
				Int("generated", result.Generated).
				Int("skipped", result.Skipped).
				Int("missing", result.Missing).
				Int("failed", result.Failed).
				Msg("Thumbnail warm-on-ingest completed with warnings")
		}
	}()
}

func (s *Scraper) recordInboxException(item provider.DiscoveredMedia, status string, reason string) error {
	return s.db.RecordInboxException(&database.InboxItem{
		ProviderID: item.ProviderID,
		DedupeKey:  StableDedupeKey(item),
		SourceID:   item.Source.ExternalID,
		MediaID:    item.Media.ExternalID,
		SourceURL:  item.Source.URL,
		Title:      dataquality.NormalizeTitle(item.Title, item.Media.FileName),
		Artist:     item.Artist,
		Status:     status,
		Reason:     reason,
	})
}

func StableDedupeKey(item provider.DiscoveredMedia) string {
	if item.DedupeKey.ProviderID == "" || item.DedupeKey.Value == "" {
		return ""
	}
	return item.DedupeKey.String()
}

func (s *Scraper) IsMediaAlreadyKept(item provider.DiscoveredMedia) (bool, error) {
	keys := []string{StableDedupeKey(item)}
	if item.ProviderID == webgallery.ProviderID && item.Source.URL != "" {
		keys = append(keys, item.Source.URL)
	}

	for _, key := range keys {
		if key == "" {
			continue
		}
		exists, err := s.db.IsPhotoDownloaded(key)
		if err != nil {
			return false, err
		}
		if exists {
			return true, nil
		}
	}
	return false, nil
}

// downloadFile downloads a file from a URL
func (s *Scraper) downloadFile(ctx context.Context, fileURL, destPath string) (int64, []byte, error) {
	retryConfig := retry.Config{
		MaxAttempts:  s.cfg.Retry.MaxAttempts,
		InitialDelay: s.cfg.Retry.InitialDelay,
		MaxDelay:     s.cfg.Retry.MaxDelay,
		Multiplier:   s.cfg.Retry.Multiplier,
	}

	type downloadResult struct {
		size int64
		hash []byte
	}
	result, err := retry.DoWithValue(ctx, retryConfig, func() (downloadResult, error) {
		req, err := http.NewRequestWithContext(ctx, "GET", fileURL, nil)
		if err != nil {
			return downloadResult{}, err
		}
		req.Header.Set("User-Agent", s.cfg.Download.UserAgent)

		resp, err := s.client.Do(req)
		if err != nil {
			return downloadResult{}, err
		}
		defer resp.Body.Close()

		if resp.StatusCode == StatusTooManyRequests {
			// Rate limited - wait longer before retry
			retryAfter := s.cfg.Download.RateLimitBackoff
			if retryAfter == 0 {
				retryAfter = 60 * time.Second // Default 60s
			}
			s.logger.Warn().Dur("retry_after", retryAfter).Msg("Rate limited on file download, waiting before retry")
			time.Sleep(retryAfter)
			return downloadResult{}, &RateLimitError{RetryAfter: retryAfter}
		}

		if resp.StatusCode != http.StatusOK {
			return downloadResult{}, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
		}

		out, err := os.Create(destPath)
		if err != nil {
			return downloadResult{}, err
		}

		// Track success for cleanup
		var success bool
		defer func() {
			out.Close()
			if !success {
				// Clean up partial file on any failure
				os.Remove(destPath)
			}
		}()

		hasher := sha256.New()
		written, err := io.Copy(out, io.TeeReader(resp.Body, hasher))
		if err != nil {
			return downloadResult{}, err
		}

		// Mark as successful before returning
		success = true
		return downloadResult{size: written, hash: hashSum(hasher)}, nil
	})
	if err != nil {
		return 0, nil, err
	}
	return result.size, result.hash, nil
}

func hashSum(h hash.Hash) []byte {
	sum := h.Sum(nil)
	out := make([]byte, len(sum))
	copy(out, sum)
	return out
}

// createDailySymlink creates a symlink in the daily directory
func (s *Scraper) createDailySymlink(filePath, fileName string) error {
	now := time.Now()
	dailyDir := filepath.Join(
		s.cfg.Storage.DailyDirectory,
		fmt.Sprintf("%d/%02d/%02d", now.Year(), now.Month(), now.Day()),
	)

	if err := os.MkdirAll(dailyDir, DefaultDirPermissions); err != nil {
		return err
	}

	linkPath := filepath.Join(dailyDir, fileName)

	// Remove existing symlink if any
	os.Remove(linkPath)

	// Create symlink
	return os.Symlink(filePath, linkPath)
}

// sanitizeFolderName sanitizes a folder name to prevent path traversal
func sanitizeFolderName(name string) string {
	// Trim whitespace first
	name = strings.TrimSpace(name)

	// Remove path separators and traversal attempts
	name = strings.ReplaceAll(name, "..", "")
	name = strings.ReplaceAll(name, "/", "-")
	name = strings.ReplaceAll(name, "\\", "-")
	name = strings.ReplaceAll(name, "\n", "")
	name = strings.ReplaceAll(name, "\r", "")
	name = strings.ReplaceAll(name, "\t", "")

	// Replace problematic characters
	name = strings.ReplaceAll(name, "*", ".")

	// Remove dangerous characters
	reg := regexp.MustCompile(`[<>:"|?]`)
	name = reg.ReplaceAllString(name, "")

	// Trim again after replacements
	name = strings.TrimSpace(name)

	// Prevent empty or dot-only names
	if name == "" || name == "." {
		name = "unknown"
	}

	return name
}

// validatePath ensures the resolved path is within the base directory
func validatePath(basePath, fullPath string) error {
	// Clean both paths to resolve any . or .. elements
	cleanBase := filepath.Clean(basePath)
	cleanFull := filepath.Clean(fullPath)

	// Check if the full path starts with the base path
	if !strings.HasPrefix(cleanFull, cleanBase) {
		return fmt.Errorf("path traversal detected: path escapes base directory")
	}

	return nil
}

// TriggerPhotoprismIndex triggers PhotoPrism indexing
func (s *Scraper) TriggerPhotoprismIndex(ctx context.Context) error {
	if !s.cfg.PhotoPrism.Enabled || !s.cfg.PhotoPrism.AutoIndex {
		return nil
	}

	if s.photoprismClient == nil {
		return fmt.Errorf("PhotoPrism client not initialized")
	}

	return s.photoprismClient.TriggerIndex(ctx)
}

// GetPhotoprismClient returns the PhotoPrism client for direct API access
func (s *Scraper) GetPhotoprismClient() *photoprism.Client {
	return s.photoprismClient
}

// IsPhotoprismEnabled returns true if PhotoPrism integration is enabled
func (s *Scraper) IsPhotoprismEnabled() bool {
	return s.cfg.PhotoPrism.Enabled && s.photoprismClient != nil
}
