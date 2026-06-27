package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
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

	// Create scraper
	scraperInstance := scraper.New(cfg, db, logger)
	connectors := buildConnectors(cfg, logger)

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
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

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

func buildConnectors(cfg *config.Config, logger zerolog.Logger) []provider.Connector {
	client := &http.Client{Timeout: cfg.Download.Timeout}
	retryConfig := retry.Config{
		MaxAttempts:  cfg.Retry.MaxAttempts,
		InitialDelay: cfg.Retry.InitialDelay,
		MaxDelay:     cfg.Retry.MaxDelay,
		Multiplier:   cfg.Retry.Multiplier,
	}
	connectors := []provider.Connector{
		webgallery.New(webgallery.Config{
			BaseURL:          cfg.Source.BaseURL,
			UserAgent:        cfg.Download.UserAgent,
			RateLimitBackoff: cfg.Download.RateLimitBackoff,
			Retry:            retryConfig,
		}, client, logger.With().Str("provider", webgallery.ProviderID).Logger()),
	}
	if cfg.Telegram.BotToken != "" {
		connectors = append(connectors, telegram.New(telegram.Config{
			BotToken:         cfg.Telegram.BotToken,
			BaseURL:          cfg.Telegram.BaseURL,
			FileBaseURL:      cfg.Telegram.FileBaseURL,
			ChatID:           cfg.Telegram.ChatID,
			DisplayName:      cfg.Telegram.DisplayName,
			Limit:            cfg.Telegram.Limit,
			RateLimitBackoff: cfg.Download.RateLimitBackoff,
			Retry:            retryConfig,
		}, client, logger.With().Str("provider", telegram.ProviderID).Logger()))
	}
	return connectors
}

func setupLogger(cfg *config.Config) zerolog.Logger {
	// Set log level
	level, err := zerolog.ParseLevel(cfg.Logging.Level)
	if err != nil {
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
