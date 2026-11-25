package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"ok-folio/internal/api"
	okfcache "ok-folio/internal/cache"
	"ok-folio/internal/config"
	"ok-folio/internal/database"
	"ok-folio/internal/ingest"
	"ok-folio/internal/provider"
	"ok-folio/internal/provider/telegram"
	"ok-folio/internal/provider/webgallery"
	"ok-folio/internal/scheduler"
	"ok-folio/internal/scraper"
	"ok-folio/internal/similarity"
	"ok-folio/pkg/retry"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	// DefaultShutdownWait is the time to wait for graceful shutdown
	DefaultShutdownWait = 2 * time.Second
	// MaxDBRetries is the maximum number of database connection attempts
	MaxDBRetries = 10
	// InitialDBRetryDelay is the initial delay before retrying database connection
	InitialDBRetryDelay = 1 * time.Second
)

var (
	configPath = flag.String("config", "/config/config.yaml", "Path to configuration file")
	version    = "1.0.0"
)

func main() {
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Setup logger
	logger := setupLogger(cfg)
	logger.Info().Str("version", version).Msg("Starting PhotoPrism Extractor")

	// Connect to database with retry logic
	logger.Info().Msg("Connecting to database")
	var db *database.DB

	for i := 0; i < MaxDBRetries; i++ {
		db, err = database.New(&cfg.Database)
		if err == nil {
			break
		}

		if i < MaxDBRetries-1 {
			delay := InitialDBRetryDelay * time.Duration(i+1)
			logger.Warn().
				Err(err).
				Int("attempt", i+1).
				Int("max_attempts", MaxDBRetries).
				Dur("retry_in", delay).
				Msg("Database connection failed, retrying...")
			time.Sleep(delay)
		}
	}

	if err != nil {
		logger.Fatal().
			Err(err).
			Int("attempts", MaxDBRetries).
			Msg("Failed to connect to database after retries")
	}
	logger.Info().Msg("Database connected")
	checkSimilaritySidecar(cfg, logger)

	// Create scraper
	scraperInstance := scraper.New(cfg, db, logger)
	if cfg.Similarity.Enabled && cfg.Similarity.Backfill {
		go runSimilarityBackfill(ctx, cfg, db, logger)
	}
	connectors := buildConnectors(cfg, db, logger)

	// Create and start API server
	var apiServer *api.Server
	if cfg.API.Enabled {
		apiServer = api.New(cfg, db, scraperInstance, logger)
		go func() {
			if err := apiServer.Start(); err != nil {
				logger.Fatal().Err(err).Msg("API server failed")
			}
		}()
	}

	// Create and start scheduler
	var schedulerInstance *scheduler.Scheduler
	if cfg.Scheduler.Enabled {
		var cacheClient *okfcache.Client
		if apiServer != nil {
			cacheClient = apiServer.Cache()
		}
		if cacheClient == nil {
			cacheClient = okfcache.New(context.Background(), cfg.Cache, logger)
		}
		ingestor := ingest.New(db, cacheClient, scraperInstance, logger.With().Str("component", "ingestor").Logger())
		schedulerInstance = scheduler.New(cfg, ingestor, connectors, cacheClient, scraperInstance, logger)
		if err := schedulerInstance.Start(); err != nil {
			logger.Fatal().Err(err).Msg("Failed to start scheduler")
		}
	}

	logger.Info().Msg("PhotoPrism Extractor is running")

	// Wait for interrupt signal
	<-ctx.Done()
	stop()

	logger.Info().Msg("Shutting down gracefully...")

	// Cleanup
	if apiServer != nil {
		apiServer.Shutdown()
	}

	if schedulerInstance != nil {
		schedulerInstance.Stop()
	}

	// Give goroutines time to finish
	time.Sleep(DefaultShutdownWait)

	// Close database connection
	sqlDB, _ := db.DB.DB()
	if sqlDB != nil {
		sqlDB.Close()
	}

	logger.Info().Msg("Shutdown complete")
}

func checkSimilaritySidecar(cfg *config.Config, logger zerolog.Logger) {
	if !cfg.Similarity.Enabled || strings.TrimSpace(cfg.Similarity.SidecarURL) == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	health, err := similarity.NewClient(cfg.Similarity.SidecarURL).Health(ctx)
	if err != nil {
		logger.Warn().Err(err).Str("sidecar_url", cfg.Similarity.SidecarURL).Msg("Similarity sidecar health check failed")
		return
	}
	logger.Info().Str("model", health.Model).Int("dim", health.Dim).Msg("Similarity sidecar health check passed")
}

func runSimilarityBackfill(ctx context.Context, cfg *config.Config, db *database.DB, logger zerolog.Logger) {
	if strings.TrimSpace(cfg.Similarity.SidecarURL) == "" {
		logger.Warn().Msg("Similarity backfill skipped because sidecar_url is empty")
		return
	}
	_, err := similarity.Backfill(ctx, db, cfg.Storage, similarity.NewClient(cfg.Similarity.SidecarURL), similarity.Options{
		Concurrency: similarity.MaxConcurrency,
		Progress:    500,
	}, logger.With().Str("component", "similarity-backfill").Logger())
	if err != nil {
		logger.Warn().Err(err).Msg("Similarity backfill stopped")
	}
}

func buildConnectors(cfg *config.Config, db *database.DB, logger zerolog.Logger) []provider.Connector {
	client := &http.Client{Timeout: cfg.Download.Timeout}
	retryConfig := retry.Config{
		MaxAttempts:  cfg.Retry.MaxAttempts,
		InitialDelay: cfg.Retry.InitialDelay,
		MaxDelay:     cfg.Retry.MaxDelay,
		Multiplier:   cfg.Retry.Multiplier,
	}
	connectors := []provider.Connector{}
	webGalleryConnectors, hasManagedWebGallerySources := buildWebGalleryConnectors(cfg, db, client, retryConfig, logger)
	connectors = append(connectors, webGalleryConnectors...)
	if len(webGalleryConnectors) == 0 && !hasManagedWebGallerySources {
		connectors = append(connectors, webgallery.New(webgallery.Config{
			BaseURL:          cfg.Source.BaseURL,
			Schedule:         cfg.Source.Schedule,
			UserAgent:        cfg.Download.UserAgent,
			RateLimitBackoff: cfg.Download.RateLimitBackoff,
			Retry:            retryConfig,
		}, client, logger.With().Str("provider", webgallery.ProviderID).Logger()))
	}
	if cfg.Telegram.BotToken != "" {
		sources := make([]telegram.SourceConfig, 0, len(cfg.Telegram.Sources))
		for _, source := range cfg.Telegram.Sources {
			sources = append(sources, telegram.SourceConfig{
				ChatID: source.ChatID,
				Label:  source.Label,
			})
		}
		connectors = append(connectors, telegram.New(telegram.Config{
			BotToken:         cfg.Telegram.BotToken,
			BaseURL:          cfg.Telegram.BaseURL,
			FileBaseURL:      cfg.Telegram.FileBaseURL,
			ChatID:           cfg.Telegram.ChatID,
			Sources:          sources,
			DisplayName:      cfg.Telegram.DisplayName,
			Limit:            cfg.Telegram.Limit,
			Schedule:         cfg.Telegram.Schedule,
			RateLimitBackoff: cfg.Download.RateLimitBackoff,
			Retry:            retryConfig,
			SourceStore:      db,
		}, client, logger.With().Str("provider", telegram.ProviderID).Logger()))
	}
	return connectors
}

func buildWebGalleryConnectors(cfg *config.Config, db *database.DB, client *http.Client, retryConfig retry.Config, logger zerolog.Logger) ([]provider.Connector, bool) {
	if db == nil {
		return legacyWebGalleryConnectors(cfg, client, retryConfig, logger), false
	}

	sources, err := db.ListConnectorSources(webgallery.ProviderID)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to list webgallery connector sources")
		return legacyWebGalleryConnectors(cfg, client, retryConfig, logger), false
	}
	hasManagedSources := len(sources) > 0
	if len(sources) == 0 && strings.TrimSpace(cfg.Source.BaseURL) != "" {
		seed, err := json.Marshal(webgallery.DefaultConfig(cfg.Source.BaseURL))
		if err != nil {
			logger.Error().Err(err).Msg("Failed to encode default webgallery connector source")
		} else if created, err := db.CreateConnectorSource(database.ConnectorSource{
			Type:    webgallery.ProviderID,
			ChatID:  "default",
			Label:   "Default Web Gallery",
			Config:  database.JSONConfig(seed),
			Enabled: true,
		}); err != nil {
			logger.Error().Err(err).Msg("Failed to seed default webgallery connector source")
		} else {
			sources = append(sources, *created)
			hasManagedSources = true
		}
	}

	connectors := make([]provider.Connector, 0, len(sources))
	for _, source := range sources {
		if !source.Enabled {
			continue
		}
		galleryConfig, err := webgallery.ParseConfig(source.Config)
		if err != nil {
			logger.Error().Err(err).Uint64("source_id", source.ID).Msg("Skipping invalid webgallery connector source")
			continue
		}
		label := source.Label
		if strings.TrimSpace(label) == "" {
			label = "Web Gallery"
		}
		schedule := strings.TrimSpace(galleryConfig.Schedule)
		if schedule == "" {
			schedule = cfg.Source.Schedule
		}
		userAgent := strings.TrimSpace(galleryConfig.UserAgent)
		if userAgent == "" {
			userAgent = cfg.Download.UserAgent
		}
		rateLimitBackoff := time.Duration(0)
		if strings.TrimSpace(galleryConfig.RateLimit.Backoff) == "" {
			rateLimitBackoff = cfg.Download.RateLimitBackoff
		}
		connectors = append(connectors, webgallery.New(webgallery.Config{
			SourceID:         fmt.Sprintf("%d", source.ID),
			DisplayName:      label,
			Schedule:         schedule,
			UserAgent:        userAgent,
			RateLimitBackoff: rateLimitBackoff,
			Retry:            retryConfig,
			Gallery:          galleryConfig,
		}, client, logger.With().Str("provider", webgallery.ProviderID).Uint64("source_id", source.ID).Logger()))
	}
	return connectors, hasManagedSources
}

func legacyWebGalleryConnectors(cfg *config.Config, client *http.Client, retryConfig retry.Config, logger zerolog.Logger) []provider.Connector {
	return []provider.Connector{
		webgallery.New(webgallery.Config{
			BaseURL:          cfg.Source.BaseURL,
			Schedule:         cfg.Source.Schedule,
			UserAgent:        cfg.Download.UserAgent,
			RateLimitBackoff: cfg.Download.RateLimitBackoff,
			Retry:            retryConfig,
		}, client, logger.With().Str("provider", webgallery.ProviderID).Logger()),
	}
}

func setupLogger(cfg *config.Config) zerolog.Logger {
	// Set log level
	level, err := zerolog.ParseLevel(strings.TrimSpace(cfg.Logging.Level))
	if err != nil || level == zerolog.NoLevel {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	// Set output format
	var logger zerolog.Logger
	if cfg.Logging.Format == "console" {
		logger = log.Output(zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.RFC3339,
		})
	} else {
		logger = zerolog.New(os.Stdout).With().Timestamp().Logger()
	}

	return logger
}
