package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	okfcache "ok-folio/internal/cache"
	"ok-folio/internal/config"
	"ok-folio/internal/database"
	"ok-folio/internal/derivatives"
	"ok-folio/internal/scraper"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"
	"golang.org/x/time/rate"
)

const (
	// MaxRunsLimit is the maximum number of extraction runs to return
	MaxRunsLimit = 100
	// DefaultRunsLimit is the default number of runs to return if not specified
	DefaultRunsLimit = 10
	// MaxConcurrentExtractions is the maximum number of extraction jobs that can run simultaneously
	MaxConcurrentExtractions = 3
	// ExtractionJobQueueSize is the size of the extraction job queue
	ExtractionJobQueueSize = 10
	// RateLimitPerSecond is the per-client request rate for expensive,
	// state-changing endpoints. Cheap cache-served reads (GET/HEAD) are exempt.
	RateLimitPerSecond = 10
	// RateLimitBurst is the per-client burst size for rate limiting.
	RateLimitBurst = 20
)

// ipRateLimiter hands out a separate token bucket per client IP so one busy
// client cannot starve the others. It is applied only to expensive,
// state-changing requests — cheap cache-served GET/HEAD reads bypass it
// entirely (a single gallery screen fetches dozens of thumbnails at once and
// must never trip a 429).
type ipRateLimiter struct {
	mu       sync.Mutex
	limiters map[string]*rate.Limiter
	rps      rate.Limit
	burst    int
}

func newIPRateLimiter(rps rate.Limit, burst int) *ipRateLimiter {
	return &ipRateLimiter{
		limiters: make(map[string]*rate.Limiter),
		rps:      rps,
		burst:    burst,
	}
}

func (l *ipRateLimiter) limiterFor(key string) *rate.Limiter {
	l.mu.Lock()
	defer l.mu.Unlock()
	limiter, ok := l.limiters[key]
	if !ok {
		limiter = rate.NewLimiter(l.rps, l.burst)
		l.limiters[key] = limiter
	}
	return limiter
}

type Server struct {
	cfg            *config.Config
	db             *database.DB
	scraper        *scraper.Scraper
	logger         zerolog.Logger
	router         *chi.Mux
	ctx            context.Context
	cancel         context.CancelFunc
	jobQueue       chan func()
	limiter        *ipRateLimiter
	statsCache     *StatsCache
	cache          *okfcache.Client
	thumbCache     *derivatives.Cache
	thumbnailTiers *thumbnailTierMetrics
}

func New(cfg *config.Config, db *database.DB, scraper *scraper.Scraper, logger zerolog.Logger) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	cacheClient := okfcache.New(ctx, cfg.Cache, logger)

	s := &Server{
		cfg:            cfg,
		db:             db,
		scraper:        scraper,
		logger:         logger,
		router:         chi.NewRouter(),
		ctx:            ctx,
		cancel:         cancel,
		jobQueue:       make(chan func(), ExtractionJobQueueSize),
		limiter:        newIPRateLimiter(RateLimitPerSecond, RateLimitBurst),
		statsCache:     NewStatsCache(5 * time.Minute), // Cache stats for 5 minutes
		cache:          cacheClient,
		thumbCache:     derivatives.NewCache(cfg.Storage),
		thumbnailTiers: &thumbnailTierMetrics{},
	}

	// Start worker pool for extraction jobs
	for i := 0; i < MaxConcurrentExtractions; i++ {
		go s.extractionWorker(i)
	}

	s.setupRoutes()
	return s
}

func (s *Server) Cache() *okfcache.Client {
	return s.cache
}

// extractionWorker processes extraction jobs from the queue
func (s *Server) extractionWorker(id int) {
	s.logger.Info().Int("worker_id", id).Msg("Extraction worker started")
	for {
		select {
		case <-s.ctx.Done():
			s.logger.Info().Int("worker_id", id).Msg("Extraction worker stopping")
			return
		case job, ok := <-s.jobQueue:
			if !ok {
				s.logger.Info().Int("worker_id", id).Msg("Extraction worker stopping (queue closed)")
				return
			}
			job()
		}
	}
}

// Shutdown gracefully stops the API server and workers
func (s *Server) Shutdown() {
	s.logger.Info().Msg("Shutting down API server")
	s.cancel()
	close(s.jobQueue)
}

func (s *Server) setupRoutes() {
	s.router.Use(middleware.RequestID)
	s.router.Use(middleware.RealIP)
	s.router.Use(middleware.Recoverer)
	s.router.Use(s.rateLimitMiddleware)
	s.router.Use(middleware.Timeout(60 * time.Second))

	// Health check
	s.router.Get("/health", s.handleHealth)

	// API routes
	s.router.Route("/api/v1", func(r chi.Router) {
		r.Get("/stats", s.handleStats)
		r.Get("/runs", s.handleGetRuns)
		r.Get("/runs/{id}/photos", s.handleRunPhotos)
		r.Post("/extract", s.handleExtract)
		r.Post("/extract/page/{page}", s.handleExtractPage)
		r.Post("/extract/pages", s.handleExtractPages)

		// Analytics endpoints
		r.Get("/stats/timeline", s.handleStatsTimeline)
		r.Get("/stats/artists/top", s.handleTopArtists)
		r.Get("/stats/thumbnail-tiers", s.handleThumbnailTiers)
		r.Get("/workers/status", s.handleWorkerStatus)

		// Failed photos management
		r.Get("/photos/failed", s.handleFailedPhotos)
		r.Post("/photos/retry", s.handleRetryPhoto)

		// Search and artist endpoints
		r.Get("/search", s.handleSearch)
		r.Get("/artists", s.handleArtists)
		r.Get("/artists/detail", s.handleArtistDetail)

		// Photo endpoints
		r.Post("/pieces", s.handleCreatePiece)
		r.Get("/photos/{id}", s.handlePhotoDetail)
		r.Patch("/photos/{id}", s.handleUpdatePieceMetadata)
		r.Get("/photos/{id}/thumbnail", s.handleImageThumbnail)
		r.Get("/photos/{id}/image", s.handleImageFull)
		r.Get("/photos/{id}/favorite", s.handleGetFavoriteStatus)
		r.Post("/photos/{id}/favorite", s.handleAddFavorite)
		r.Delete("/photos/{id}/favorite", s.handleRemoveFavorite)
		r.Get("/photos/{id}/folios", s.handleListPhotoFolios)
		r.Get("/photos/today", s.handleTodayPhotos)
		r.Get("/photos/week", s.handleWeekPhotos)

		// Gallery architecture prototype
		r.Post("/catalog/bulk-edit", s.handleBulkEditCatalog)
		r.Get("/gallery/catalog", s.handleGalleryCatalog)
		r.Get("/gallery/{id}/similar", s.handleGallerySimilar)
		r.Get("/gallery/decision", s.handleGalleryDecision)
		r.Get("/inbox", s.handleInbox)
		r.Get("/inbox/counts", s.handleInboxCounts)
		r.Get("/inbox/{id}", s.handleInboxItem)
		r.Post("/inbox/{id}/keep", s.handleKeepInboxItem)
		r.Post("/inbox/{id}/skip", s.handleSkipInboxItem)
		r.Post("/inbox/{id}/move", s.handleMoveInboxItem)
		r.Delete("/inbox/{id}", s.handleDismissInboxItem)
		r.Get("/streams/connectors/status", s.handleConnectorStatus)
		r.Get("/settings/connector-sources", s.handleListConnectorSources)
		r.Post("/settings/connector-sources/preview", s.handlePreviewConnectorSource)
		r.Post("/settings/connector-sources", s.handleCreateConnectorSource)
		r.Post("/settings/connector-sources/{id}/backfill", s.handleBackfillConnectorSource)
		r.Patch("/settings/connector-sources/{id}", s.handleUpdateConnectorSource)
		r.Delete("/settings/connector-sources/{id}", s.handleDeleteConnectorSource)

		// Folios
		r.Get("/folios", s.handleListFolios)
		r.Post("/folios", s.handleCreateFolio)
		r.Get("/folios/{id}", s.handleGetFolio)
		r.Patch("/folios/{id}", s.handleUpdateFolio)
		r.Delete("/folios/{id}", s.handleDeleteFolio)
		r.Get("/folios/{id}/pieces", s.handleListFolioPieces)
		r.Post("/folios/{id}/pieces", s.handleAddFolioPiece)
		r.Delete("/folios/{id}/pieces/{photoId}", s.handleRemoveFolioPiece)

		// Legacy PhotoPrism admin escape hatch (disabled by default). Kept out of
		// the normal product path during Wave 6 retirement; returns a disabled
		// response unless photoprism.enabled is explicitly set.
		r.Post("/photoprism/index", s.handleTriggerIndex)

		// Real-time streaming
		r.Get("/stream/stats", s.handleStatsStream)
	})

	// Dashboard routes (serves embedded React app)
	s.setupDashboardRoutes()
}

// rateLimitMiddleware throttles only expensive, state-changing requests, and
// does so per client IP. Cheap cache-served reads (GET/HEAD) — e.g. the gallery
// fetching dozens of thumbnails per screen on infinite scroll — are never
// rate-limited, so a normal page load cannot trip a 429. RealIP runs before
// this middleware, so r.RemoteAddr is the real client address.
func (s *Server) rateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead {
			next.ServeHTTP(w, r)
			return
		}
		if !s.limiter.limiterFor(r.RemoteAddr).Allow() {
			s.writeError(w, http.StatusTooManyRequests, "Rate limit exceeded. Please try again later.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.cfg.API.Host, s.cfg.API.Port)
	s.logger.Info().Str("address", addr).Msg("Starting API server")
	return http.ListenAndServe(addr, s.router)
}

// handleHealth returns service health status
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	health := map[string]interface{}{
		"status": "healthy",
		"time":   time.Now().UTC(),
	}

	// Check database connection
	sqlDB, err := s.db.DB.DB()
	if err == nil {
		if err := sqlDB.Ping(); err == nil {
			health["database"] = "connected"
		} else {
			health["database"] = "disconnected"
			health["status"] = "degraded"
		}
	}

	s.writeJSON(w, http.StatusOK, health)
}

// handleStats returns download statistics
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	// Try to get from cache first
	if cachedStats, ok := s.statsCache.Get(); ok {
		s.logger.Debug().Msg("Serving stats from cache")
		s.writeJSON(w, http.StatusOK, cachedStats)
		return
	}

	// Cache miss - fetch from database
	s.logger.Debug().Msg("Cache miss - fetching stats from database")
	stats, err := s.db.GetDownloadStats()
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to get stats")
		s.writeError(w, http.StatusInternalServerError, "Failed to get statistics")
		return
	}

	// Store in cache
	s.statsCache.Set(stats)

	s.writeJSON(w, http.StatusOK, stats)
}

// handleGetRuns returns recent extraction runs
func (s *Server) handleGetRuns(w http.ResponseWriter, r *http.Request) {
	limit := DefaultRunsLimit
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= MaxRunsLimit {
			limit = l
		}
	}

	runs, err := s.db.GetRecentRuns(limit)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to get runs")
		s.writeError(w, http.StatusInternalServerError, "Failed to get runs")
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"runs": runs,
	})
}

// handleExtract triggers extraction of configured pages
func (s *Server) handleExtract(w http.ResponseWriter, r *http.Request) {
	job := func() {
		ctx, cancel := context.WithTimeout(s.ctx, 30*time.Minute)
		defer cancel()

		run, err := s.db.StartExtractionRun()
		if err != nil {
			s.logger.Error().Err(err).Msg("Failed to start extraction run")
			return
		}

		var (
			totalDownloaded int
			totalSkipped    int
			totalFailed     int
		)

		for _, page := range s.cfg.Scheduler.Pages {
			downloaded, skipped, failed, err := s.scraper.ScrapePage(ctx, page)
			if err != nil {
				s.logger.Error().Err(err).Int("page", page).Msg("Failed to scrape page")
				s.db.FinishExtractionRun(run.ID, "failed", err.Error())
				return
			}

			totalDownloaded += downloaded
			totalSkipped += skipped
			totalFailed += failed

			run.PagesProcessed++
			run.PhotosDownloaded = totalDownloaded
			run.PhotosSkipped = totalSkipped
			run.PhotosFailed = totalFailed
			s.db.UpdateExtractionRun(run)
		}

		s.db.FinishExtractionRun(run.ID, "completed", "")
		s.logger.Info().
			Int("downloaded", totalDownloaded).
			Int("skipped", totalSkipped).
			Int("failed", totalFailed).
			Msg("Extraction completed")

		// Invalidate stats cache after successful extraction
		if totalDownloaded > 0 {
			s.statsCache.Invalidate()
			_ = s.cache.BumpEpoch(ctx)
			s.logger.Debug().Msg("Stats cache invalidated after new downloads")
		}

		// Trigger PhotoPrism indexing
		if totalDownloaded > 0 {
			if err := s.scraper.TriggerPhotoprismIndex(ctx); err != nil {
				s.logger.Warn().Err(err).Msg("Failed to trigger PhotoPrism indexing")
			}
		}
	}

	select {
	case s.jobQueue <- job:
		s.writeJSON(w, http.StatusAccepted, map[string]string{
			"message": "Extraction queued",
			"status":  "queued",
		})
	default:
		s.writeError(w, http.StatusTooManyRequests, "Server busy, extraction queue is full. Try again later.")
	}
}

// handleExtractPage triggers extraction of a specific page
func (s *Server) handleExtractPage(w http.ResponseWriter, r *http.Request) {
	pageStr := chi.URLParam(r, "page")
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		s.writeError(w, http.StatusBadRequest, "Invalid page number")
		return
	}

	job := func() {
		ctx, cancel := context.WithTimeout(s.ctx, 30*time.Minute)
		defer cancel()

		run, err := s.db.StartExtractionRun()
		if err != nil {
			s.logger.Error().Err(err).Msg("Failed to start extraction run")
			return
		}

		downloaded, skipped, failed, err := s.scraper.ScrapePage(ctx, page)
		if err != nil {
			s.logger.Error().Err(err).Int("page", page).Msg("Failed to scrape page")
			s.db.FinishExtractionRun(run.ID, "failed", err.Error())
			return
		}

		run.PagesProcessed = 1
		run.PhotosDownloaded = downloaded
		run.PhotosSkipped = skipped
		run.PhotosFailed = failed
		s.db.UpdateExtractionRun(run)
		s.db.FinishExtractionRun(run.ID, "completed", "")

		s.logger.Info().
			Int("page", page).
			Int("downloaded", downloaded).
			Int("skipped", skipped).
			Int("failed", failed).
			Msg("Page extraction completed")

		// Invalidate stats cache after successful extraction
		if downloaded > 0 {
			s.statsCache.Invalidate()
			_ = s.cache.BumpEpoch(ctx)
			s.logger.Debug().Msg("Stats cache invalidated after new downloads")
		}

		// Trigger PhotoPrism indexing
		if downloaded > 0 {
			if err := s.scraper.TriggerPhotoprismIndex(ctx); err != nil {
				s.logger.Warn().Err(err).Msg("Failed to trigger PhotoPrism indexing")
			}
		}
	}

	select {
	case s.jobQueue <- job:
		s.writeJSON(w, http.StatusAccepted, map[string]interface{}{
			"message": "Page extraction queued",
			"page":    page,
			"status":  "queued",
		})
	default:
		s.writeError(w, http.StatusTooManyRequests, "Server busy, extraction queue is full. Try again later.")
	}
}

// handleExtractPages triggers extraction of pages 1 through N
func (s *Server) handleExtractPages(w http.ResponseWriter, r *http.Request) {
	countStr := r.URL.Query().Get("count")
	if countStr == "" {
		s.writeError(w, http.StatusBadRequest, "Missing 'count' parameter")
		return
	}

	count, err := strconv.Atoi(countStr)
	if err != nil || count < 1 || count > 20 {
		s.writeError(w, http.StatusBadRequest, "Page count must be between 1 and 20")
		return
	}

	// Build pages array
	pages := make([]int, count)
	for i := 0; i < count; i++ {
		pages[i] = i + 1
	}

	job := func() {
		ctx, cancel := context.WithTimeout(s.ctx, 30*time.Minute)
		defer cancel()

		run, err := s.db.StartExtractionRun()
		if err != nil {
			s.logger.Error().Err(err).Msg("Failed to start extraction run")
			return
		}

		var (
			totalDownloaded int
			totalSkipped    int
			totalFailed     int
		)

		for _, page := range pages {
			downloaded, skipped, failed, err := s.scraper.ScrapePage(ctx, page)
			if err != nil {
				s.logger.Error().Err(err).Int("page", page).Msg("Failed to scrape page")
				s.db.FinishExtractionRun(run.ID, "failed", err.Error())
				return
			}

			totalDownloaded += downloaded
			totalSkipped += skipped
			totalFailed += failed

			run.PagesProcessed++
			run.PhotosDownloaded = totalDownloaded
			run.PhotosSkipped = totalSkipped
			run.PhotosFailed = totalFailed
			s.db.UpdateExtractionRun(run)
		}

		s.db.FinishExtractionRun(run.ID, "completed", "")
		s.logger.Info().
			Int("pages", count).
			Int("downloaded", totalDownloaded).
			Int("skipped", totalSkipped).
			Int("failed", totalFailed).
			Msg("Custom pages extraction completed")

		// Invalidate stats cache after successful extraction
		if totalDownloaded > 0 {
			s.statsCache.Invalidate()
			_ = s.cache.BumpEpoch(ctx)
			s.logger.Debug().Msg("Stats cache invalidated after new downloads")
		}

		// Trigger PhotoPrism indexing
		if totalDownloaded > 0 {
			if err := s.scraper.TriggerPhotoprismIndex(ctx); err != nil {
				s.logger.Warn().Err(err).Msg("Failed to trigger PhotoPrism indexing")
			}
		}
	}

	select {
	case s.jobQueue <- job:
		s.writeJSON(w, http.StatusAccepted, map[string]interface{}{
			"message": fmt.Sprintf("Extraction of %d pages queued", count),
			"pages":   pages,
			"status":  "queued",
		})
	default:
		s.writeError(w, http.StatusTooManyRequests, "Server busy, extraction queue is full. Try again later.")
	}
}

func (s *Server) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (s *Server) writeError(w http.ResponseWriter, status int, message string) {
	s.writeJSON(w, status, map[string]string{
		"error": message,
	})
}
