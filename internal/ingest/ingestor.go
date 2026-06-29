package ingest

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	okfcache "ok-folio/internal/cache"
	"ok-folio/internal/database"
	"ok-folio/internal/provider"
	"ok-folio/internal/scraper"

	"github.com/rs/zerolog"
	"gorm.io/gorm"
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
	Cursor           string
}

type RunOptions struct {
	StartPage    int
	AllowedPages []int
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
	return i.RunConnectorWithOptions(ctx, connector, RunOptions{})
}

func (i *Ingestor) RunConnectorWithOptions(ctx context.Context, connector provider.Connector, opts RunOptions) (Result, error) {
	providerID := connector.Provider().ID
	state, err := i.db.LoadConnectorState(providerID)
	if err != nil {
		return Result{}, err
	}

	run, err := i.db.StartExtractionRun(providerID)
	if err != nil {
		return Result{}, err
	}

	startCursor := ""
	if state != nil {
		startCursor = state.Cursor
	}
	result, runErr := i.ingestPages(ctx, connector, run, normalizeRunOptions(opts), startCursor)
	status := "completed"
	stateStatus := "completed"
	message := result.ErrorMessage
	if result.Halted {
		status = "failed"
		stateStatus = "permission_halt"
	}
	if runErr != nil {
		status = "failed"
		stateStatus = "failed"
		if message == "" {
			message = safeErrorMessage(runErr)
		}
	}
	if finishErr := i.db.FinishExtractionRun(run.ID, status, message); finishErr != nil && runErr == nil {
		runErr = finishErr
	}
	if stateErr := i.saveConnectorState(providerID, result.Cursor, stateStatus, message); stateErr != nil && runErr == nil {
		runErr = stateErr
	}
	return result, runErr
}

func (i *Ingestor) ingestPages(ctx context.Context, connector provider.Connector, run *database.ExtractionRun, opts RunOptions, startCursor string) (Result, error) {
	result := Result{Cursor: startCursor}
	providerID := connector.Provider().ID
	req := provider.PageRequest{Page: opts.StartPage, Cursor: startCursor}

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
				result.ErrorMessage = safeErrorMessage(handledErr)
				return result, handledErr
			}
			result.ErrorMessage = safeErrorMessage(err)
			return result, err
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
				result.ErrorMessage = safeErrorMessage(err)
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
				if isRetryableProviderError(err) {
					result.ErrorMessage = safeErrorMessage(err)
					return result, err
				}
				continue
			}

			if len(resolved.Media.ContentHash) > 0 {
				if _, hit, err := i.cache.DedupeHashOwner(ctx, resolved.Media.ContentHash); err != nil {
					return result, err
				} else if hit {
					if err := i.recordInboxDuplicate(providerID, dedupeKey, *resolved, "exact content hash already kept"); err != nil {
						return result, err
					}
					if err := i.cache.MarkSeen(ctx, providerID, dedupeKey); err != nil {
						return result, err
					}
					result.PhotosSkipped++
					continue
				}
			}

			source, sourceErr := i.connectorSourceForResolved(providerID, *resolved)
			if sourceErr != nil {
				return result, sourceErr
			}
			routing := scraper.IngestRouting{}
			if source != nil {
				routing.ConnectorSourceID = &source.ID
				if source.TargetFolioID != nil {
					routing.HiddenFromGallery = !source.ShowInLibrary
				}
			}
			photo, kept, err := i.scraper.DownloadResolvedMediaOrDuplicate(ctx, *resolved, providerID, routing)
			if err != nil {
				if dbErr := i.db.RecordFailedDownload(dedupeKey, safeErrorMessage(err)); dbErr != nil {
					i.logger.Warn().Err(dbErr).Str("dedupe_key", dedupeKey).Msg("Failed to record failed download")
				}
				result.PhotosFailed++
				continue
			}

			if err := i.cache.MarkSeen(ctx, providerID, dedupeKey); err != nil {
				return result, err
			}
			if !kept {
				result.PhotosSkipped++
				continue
			}
			if source != nil {
				previousTarget := source.TargetFolioID
				if err := i.db.ApplyConnectorSourceRouting(*source, photo.ID); err != nil {
					return result, err
				}
				if previousTarget != nil && source.TargetFolioID == nil {
					i.logger.Warn().
						Uint64("source_id", source.ID).
						Uint64("missing_folio_id", *previousTarget).
						Msg("Connector source target folio missing; routed piece to library")
				}
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

		if page.Pagination.NextCursor != "" {
			result.Cursor = page.Pagination.NextCursor
		}
		if err := i.saveConnectorState(providerID, result.Cursor, "running", result.ErrorMessage); err != nil {
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
		if !opts.pageAllowed(req.Page) {
			nextPage, ok := opts.nextAllowedAfter(req.Page)
			if !ok {
				return result, nil
			}
			req.Page = nextPage
			req.Cursor = ""
		}
	}
}

func (i *Ingestor) recordInboxDuplicate(providerID string, dedupeKey string, item provider.DiscoveredMedia, reason string) error {
	return i.db.RecordInboxException(&database.InboxItem{
		ProviderID:  providerID,
		DedupeKey:   dedupeKey,
		SourceID:    item.Source.ExternalID,
		MediaID:     item.Media.ExternalID,
		SourceURL:   item.Source.URL,
		Title:       item.Title,
		Artist:      item.Artist,
		Status:      "duplicate",
		Reason:      reason,
		ContentHash: item.Media.ContentHash,
	})
}

func (i *Ingestor) saveConnectorState(providerID string, cursor string, status string, message string) error {
	now := time.Now()
	return i.db.SaveConnectorState(database.ConnectorState{
		ProviderID:   providerID,
		Cursor:       cursor,
		LastRunAt:    &now,
		LastStatus:   status,
		ErrorMessage: message,
	})
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
		if dbErr := i.db.RecordFailedDownload(dedupeKey, safeErrorMessage(providerErr)); dbErr != nil {
			return false, dbErr
		}
		result.PhotosFailed++
		return false, nil
	case provider.ErrorKindPermission:
		result.Halted = true
		result.ErrorMessage = safeErrorMessage(providerErr)
		run.ErrorMessage = result.ErrorMessage
		if dbErr := i.db.UpdateExtractionRun(run); dbErr != nil {
			return true, dbErr
		}
		return true, nil
	default:
		return false, err
	}
}

func normalizeRunOptions(opts RunOptions) RunOptions {
	if opts.StartPage <= 0 {
		opts.StartPage = 1
	}
	if len(opts.AllowedPages) == 0 {
		return opts
	}
	pages := append([]int(nil), opts.AllowedPages...)
	sort.Ints(pages)
	opts.AllowedPages = pages
	if opts.StartPage == 1 {
		opts.StartPage = pages[0]
	}
	return opts
}

func (opts RunOptions) pageAllowed(page int) bool {
	if len(opts.AllowedPages) == 0 {
		return true
	}
	for _, allowed := range opts.AllowedPages {
		if allowed == page {
			return true
		}
	}
	return false
}

func (opts RunOptions) nextAllowedAfter(page int) (int, bool) {
	for _, allowed := range opts.AllowedPages {
		if allowed > page {
			return allowed, true
		}
	}
	return 0, false
}

func isRetryableProviderError(err error) bool {
	var providerErr *provider.ProviderError
	return errors.As(err, &providerErr) && providerErr.Retryable()
}

func (i *Ingestor) connectorSourceForResolved(providerID string, resolved provider.DiscoveredMedia) (*database.ConnectorSource, error) {
	if i.db == nil {
		return nil, nil
	}
	if base, suffix, ok := strings.Cut(providerID, ":"); ok && base != "" && suffix != "" {
		id, err := strconv.ParseUint(suffix, 10, 64)
		if err == nil && id > 0 {
			source, err := i.db.GetConnectorSource(id)
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, nil
			}
			if isMissingConnectorSourcesTable(err) {
				return nil, nil
			}
			return source, err
		}
	}
	sourceType := providerID
	if base, _, ok := strings.Cut(providerID, ":"); ok {
		sourceType = base
	}
	chatID := strings.TrimSpace(resolved.Source.CollectionID)
	if chatID == "" {
		chatID = sourceIDPrefix(resolved.Source.ExternalID)
	}
	if chatID == "" {
		return nil, nil
	}
	source, err := i.db.FindConnectorSourceByTypeAndChatID(sourceType, chatID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if isMissingConnectorSourcesTable(err) {
		return nil, nil
	}
	return source, err
}

func isMissingConnectorSourcesTable(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "connector_sources")
}

func sourceIDPrefix(externalID string) string {
	externalID = strings.TrimSpace(externalID)
	if externalID == "" {
		return ""
	}
	if before, _, ok := strings.Cut(externalID, ":"); ok {
		return before
	}
	return externalID
}

var telegramTokenInURLPattern = regexp.MustCompile(`(https?://[^\s"'<>]+/(?:file/)?bot)[^/\s"'<>]+`)

func safeErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	return telegramTokenInURLPattern.ReplaceAllString(err.Error(), "${1}<redacted>")
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
