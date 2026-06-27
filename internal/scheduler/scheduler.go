package scheduler

import (
	"context"
	"fmt"

	okfcache "ok-folio/internal/cache"
	"ok-folio/internal/config"
	"ok-folio/internal/ingest"
	"ok-folio/internal/provider"

	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog"
)

type Scheduler struct {
	cfg        *config.Config
	ingestor   *ingest.Ingestor
	connectors []provider.Connector
	cache      *okfcache.Client
	indexer    interface {
		TriggerPhotoprismIndex(context.Context) error
	}
	logger zerolog.Logger
	cron   *cron.Cron
}

func New(cfg *config.Config, ingestor *ingest.Ingestor, connectors []provider.Connector, cache *okfcache.Client, indexer interface {
	TriggerPhotoprismIndex(context.Context) error
}, logger zerolog.Logger) *Scheduler {
	return &Scheduler{
		cfg:        cfg,
		ingestor:   ingestor,
		connectors: connectors,
		cache:      cache,
		indexer:    indexer,
		logger:     logger,
		cron:       cron.New(cron.WithSeconds()),
	}
}

func (s *Scheduler) Start() error {
	if !s.cfg.Scheduler.Enabled {
		s.logger.Info().Msg("Scheduler is disabled")
		return nil
	}

	s.logger.Info().Str("schedule", s.cfg.Scheduler.Schedule).Msg("Starting scheduler")

	_, err := s.cron.AddFunc(s.cfg.Scheduler.Schedule, func() {
		s.runExtraction()
	})

	if err != nil {
		return fmt.Errorf("failed to add cron job: %w", err)
	}

	s.cron.Start()
	s.logger.Info().Msg("Scheduler started")

	return nil
}

func (s *Scheduler) Stop() {
	if s.cron != nil {
		s.logger.Info().Msg("Stopping scheduler")
		s.cron.Stop()
	}
}

func (s *Scheduler) runExtraction() {
	s.logger.Info().Msg("Starting scheduled extraction")

	ctx := context.Background()
	var (
		totalDownloaded int
		totalSkipped    int
		totalFailed     int
	)

	for _, connector := range s.connectors {
		providerID := connector.Provider().ID
		s.logger.Info().Str("provider", providerID).Msg("Running connector ingestion")
		result, err := s.ingestor.RunConnector(ctx, connector)
		if err != nil {
			s.logger.Error().Err(err).Str("provider", providerID).Msg("Connector ingestion failed")
		}

		totalDownloaded += result.PhotosDownloaded
		totalSkipped += result.PhotosSkipped
		totalFailed += result.PhotosFailed
	}

	s.logger.Info().
		Int("downloaded", totalDownloaded).
		Int("skipped", totalSkipped).
		Int("failed", totalFailed).
		Msg("Scheduled extraction completed")

	// Trigger PhotoPrism indexing if photos were downloaded
	if totalDownloaded > 0 {
		if s.cache != nil {
			s.logger.Debug().Msg("Catalog cache epoch bumped by ingestor batches")
		}
		if s.indexer != nil {
			if err := s.indexer.TriggerPhotoprismIndex(ctx); err != nil {
				s.logger.Warn().Err(err).Msg("Failed to trigger PhotoPrism indexing")
			}
		}
	}
}
