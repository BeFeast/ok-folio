package scraper

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"ok-folio/internal/config"
	"ok-folio/internal/database"
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
	}
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
	)

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

			dedupeURL := mediaItem.DedupeKey.Value
			if dedupeURL == "" {
				dedupeURL = mediaItem.Source.URL
			}

			// Check if already downloaded
			exists, err := s.db.IsPhotoDownloaded(dedupeURL)
			if err != nil {
				s.logger.Error().Err(err).Str("url", dedupeURL).Msg("Failed to check if photo exists")
				mu.Lock()
				failed++
				mu.Unlock()
				return
			}

			if exists {
				s.logger.Debug().Str("url", dedupeURL).Msg("Photo already downloaded, skipping")
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
				s.logger.Error().Err(err).Str("url", dedupeURL).Msg("Failed to download photo")

				// Record the failed download to the database
				if dbErr := s.db.RecordFailedDownload(dedupeURL, err.Error()); dbErr != nil {
					s.logger.Error().Err(dbErr).Str("url", dedupeURL).Msg("Failed to record failed download")
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
	photoURL := item.DedupeKey.Value
	if photoURL == "" {
		photoURL = item.Source.URL
	}
	s.logger.Debug().Str("url", photoURL).Msg("Downloading photo")

	resolved, err := s.provider.ResolveMedia(ctx, item)
	if err != nil {
		return fmt.Errorf("failed to resolve media: %w", err)
	}

	// Sanitize artist name for folder
	sanitizedArtist := sanitizeFolderName(resolved.Artist)
	artistDir := filepath.Join(s.cfg.Storage.BaseDirectory, sanitizedArtist)

	// Validate artist directory path
	if err := validatePath(s.cfg.Storage.BaseDirectory, artistDir); err != nil {
		return fmt.Errorf("invalid artist directory path: %w", err)
	}

	// Create artist directory
	if err := os.MkdirAll(artistDir, DefaultDirPermissions); err != nil {
		return fmt.Errorf("failed to create artist directory: %w", err)
	}

	// Download image
	fileName := resolved.Media.FileName
	if fileName == "" {
		fileName = filepath.Base(resolved.Media.URL)
	}
	filePath := filepath.Join(artistDir, fileName)

	// Validate file path
	if err := validatePath(s.cfg.Storage.BaseDirectory, filePath); err != nil {
		return fmt.Errorf("invalid file path: %w", err)
	}

	fileSize, err := s.downloadFile(ctx, resolved.Media.URL, filePath)
	if err != nil {
		return fmt.Errorf("failed to download image file: %w", err)
	}

	// Set EXIF metadata
	if s.cfg.EXIF.SetArtist || s.cfg.EXIF.SetDate || s.cfg.EXIF.SetTitle {
		metadata := exif.Metadata{
			Title:      resolved.Title,
			Artist:     resolved.Artist,
			UploadDate: resolved.PublishedAt,
		}

		if err := exif.SetMetadata(filePath, metadata, &s.cfg.EXIF); err != nil {
			s.logger.Warn().Err(err).Str("file", filePath).Msg("Failed to set EXIF metadata")
		}
	}

	// Record in database
	photo := &database.DownloadedPhoto{
		URL:        photoURL,
		SourcePage: resolved.Source.URL,
		Title:      resolved.Title,
		Artist:     resolved.Artist,
		UploadDate: resolved.PublishedAt,
		FilePath:   filePath,
		FileName:   fileName,
		FileSize:   fileSize,
		Status:     "downloaded",
	}

	if err := s.db.RecordDownload(photo); err != nil {
		s.logger.Error().Err(err).Msg("Failed to record download in database")
		return fmt.Errorf("failed to record download in database: %w", err)
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

	return nil
}

// downloadFile downloads a file from a URL
func (s *Scraper) downloadFile(ctx context.Context, fileURL, destPath string) (int64, error) {
	retryConfig := retry.Config{
		MaxAttempts:  s.cfg.Retry.MaxAttempts,
		InitialDelay: s.cfg.Retry.InitialDelay,
		MaxDelay:     s.cfg.Retry.MaxDelay,
		Multiplier:   s.cfg.Retry.Multiplier,
	}

	return retry.DoWithValue(ctx, retryConfig, func() (int64, error) {
		req, err := http.NewRequestWithContext(ctx, "GET", fileURL, nil)
		if err != nil {
			return 0, err
		}
		req.Header.Set("User-Agent", s.cfg.Download.UserAgent)

		resp, err := s.client.Do(req)
		if err != nil {
			return 0, err
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
			return 0, &RateLimitError{RetryAfter: retryAfter}
		}

		if resp.StatusCode != http.StatusOK {
			return 0, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
		}

		out, err := os.Create(destPath)
		if err != nil {
			return 0, err
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

		written, err := io.Copy(out, resp.Body)
		if err != nil {
			return 0, err
		}

		// Mark as successful before returning
		success = true
		return written, nil
	})
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
