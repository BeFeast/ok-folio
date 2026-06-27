package ingest

import (
	"context"
	"errors"
	"fmt"
	"time"

	okfcache "ok-folio/internal/cache"
	"ok-folio/internal/database"
	"ok-folio/internal/provider"
	"ok-folio/internal/scraper"

	"github.com/rs/zerolog"
)

type Ingestor struct {
	db      *database.DB
	cache   *okfcache.Client
	scraper *scraper.Scraper
	logger  zerolog.Logger
	backoff func(context.Context, time.Duration) error
}

type Result struct {
	PagesProcessed   int
	PhotosFound      int
	PhotosDownloaded int
	PhotosSkipped    int
	PhotosFailed     int
	Halted           bool
	ErrorMessage     string
}

func New(db *database.DB, cache *okfcache.Client, scraper *scraper.Scraper, logger zerolog.Logger) *Ingestor {
	return &Ingestor{
		db:      db,
		cache:   cache,
		scraper: scraper,
		logger:  logger,
		backoff: sleepBackoff,
	}
}

func (i *Ingestor) RunConnector(ctx context.Context, connector provider.Connector) (Result, error) {
	providerID := connector.Provider().ID
	run, err := i.db.StartExtractionRun(providerID)
	if err != nil {
		return Result{}, err
	}

	result, runErr := i.ingestPages(ctx, connector, run)
	status := "completed"
	message := result.ErrorMessage
	if result.Halted {
		status = "failed"
	}
	if runErr != nil {
		status = "failed"
		if message == "" {
			message = runErr.Error()
		}
	}
	if finishErr := i.db.FinishExtractionRun(run.ID, status, message); finishErr != nil && runErr == nil {
		runErr = finishErr
	}
	return result, runErr
}

func (i *Ingestor) ingestPages(ctx context.Context, connector provider.Connector, run *database.ExtractionRun) (Result, error) {
	var result Result
	providerID := connector.Provider().ID
	req := provider.PageRequest{Page: 1}

	for {
		select {
		case <-ctx.Done():
			result.ErrorMessage = ctx.Err().Error()
			return result, ctx.Err()
		default:
		}

		page, err := connector.DiscoverPage(ctx, req)
		if err != nil {
			halt, handledErr := i.handleProviderError(ctx, providerID, "", err, run, &result)
			if halt {
				return result, nil
			}
			if handledErr != nil {
				result.ErrorMessage = handledErr.Error()
				return result, handledErr
			}
			if isRetryableProviderError(err) {
				continue
			}
			return result, nil
		}
		if page == nil {
			return result, nil
		}

		result.PagesProcessed++
		result.PhotosFound += len(page.Items)
		committed := 0

		for _, item := range page.Items {
			if err := ctx.Err(); err != nil {
				result.ErrorMessage = err.Error()
				return result, err
			}

			dedupeKey := scraper.StableDedupeKey(item)
			if dedupeKey == "" {
				if err := i.db.RecordFailedDownload("", "missing connector dedupe key"); err != nil {
					i.logger.Warn().Err(err).Msg("Failed to record missing dedupe key")
				}
				result.PhotosFailed++
				continue
			}

			seen, err := i.cache.Seen(ctx, providerID, dedupeKey)
			if err != nil {
				return result, err
			}
			if seen {
				result.PhotosSkipped++
				continue
			}

			kept, err := i.scraper.IsMediaAlreadyKept(item)
			if err != nil {
				result.ErrorMessage = err.Error()
				return result, err
			}
			if kept {
				if err := i.cache.MarkSeen(ctx, providerID, dedupeKey); err != nil {
					return result, err
				}
				result.PhotosSkipped++
				continue
			}

			resolved, err := connector.ResolveMedia(ctx, item)
			if err != nil {
				halt, handledErr := i.handleProviderError(ctx, providerID, dedupeKey, err, run, &result)
				if halt {
					return result, nil
				}
				if handledErr != nil {
					result.ErrorMessage = handledErr.Error()
					return result, handledErr
				}
				continue
			}

			photo, err := i.scraper.DownloadResolvedMedia(ctx, *resolved, providerID)
			if err != nil {
				if dbErr := i.db.RecordFailedDownload(dedupeKey, err.Error()); dbErr != nil {
					i.logger.Warn().Err(dbErr).Str("dedupe_key", dedupeKey).Msg("Failed to record failed download")
				}
				result.PhotosFailed++
				continue
			}

			if err := i.cache.MarkSeen(ctx, providerID, dedupeKey); err != nil {
				return result, err
			}
			if err := i.cache.MarkDedupeHash(ctx, photo.ContentHash, dedupeKey); err != nil {
				return result, err
			}
			result.PhotosDownloaded++
			committed++
		}

		if committed > 0 {
			if err := i.cache.BumpEpoch(ctx); err != nil {
				return result, err
			}
		}

		run.PagesProcessed = result.PagesProcessed
		run.PhotosFound = result.PhotosFound
		run.PhotosDownloaded = result.PhotosDownloaded
		run.PhotosSkipped = result.PhotosSkipped
		run.PhotosFailed = result.PhotosFailed
		if err := i.db.UpdateExtractionRun(run); err != nil {
			return result, err
		}

		if !page.Pagination.HasNext {
			return result, nil
		}
		req.Cursor = page.Pagination.NextCursor
		req.Page = page.Pagination.NextPage
		if req.Page == 0 {
			req.Page = result.PagesProcessed + 1
		}
	}
}

func (i *Ingestor) handleProviderError(ctx context.Context, providerID string, dedupeKey string, err error, run *database.ExtractionRun, result *Result) (bool, error) {
	var providerErr *provider.ProviderError
	if !errors.As(err, &providerErr) {
		return false, err
	}

	switch providerErr.Kind {
	case provider.ErrorKindTemporary, provider.ErrorKindRateLimit:
		delay := providerErr.RetryAfter
		if delay <= 0 {
			delay = time.Second
		}
		return false, i.backoff(ctx, delay)
	case provider.ErrorKindNotFound, provider.ErrorKindParse, provider.ErrorKindMissingMedia:
		if dedupeKey == "" {
			dedupeKey = fmt.Sprintf("%s:discovery-error", providerID)
		}
		if dbErr := i.db.RecordFailedDownload(dedupeKey, providerErr.Error()); dbErr != nil {
			return false, dbErr
		}
		result.PhotosFailed++
		return false, nil
	case provider.ErrorKindPermission:
		result.Halted = true
		result.ErrorMessage = providerErr.Error()
		run.ErrorMessage = providerErr.Error()
		if dbErr := i.db.UpdateExtractionRun(run); dbErr != nil {
			return true, dbErr
		}
		return true, nil
	default:
		return false, err
	}
}

func isRetryableProviderError(err error) bool {
	var providerErr *provider.ProviderError
	return errors.As(err, &providerErr) && providerErr.Retryable()
}

func sleepBackoff(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
