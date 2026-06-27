package scheduler

import (
	"context"
	"fmt"
	"strings"
	"sync"

	okfcache "ok-folio/internal/cache"
	"ok-folio/internal/config"
	"ok-folio/internal/ingest"
	"ok-folio/internal/provider"
	"ok-folio/internal/provider/webgallery"

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

type connectorJob struct {
	connector provider.Connector
	mu        sync.Mutex
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

	s.logger.Info().Str("default_schedule", s.cfg.Scheduler.Schedule).Msg("Starting scheduler")

	seen := make(map[string]struct{}, len(s.connectors))
	for _, connector := range s.connectors {
		source := connector.Provider()
		providerID := source.ID
		if _, ok := seen[providerID]; ok {
			return fmt.Errorf("duplicate connector provider %q", providerID)
		}
		seen[providerID] = struct{}{}

		schedule := connectorSchedule(source, s.cfg.Scheduler.Schedule)
		job := &connectorJob{connector: connector}
		if _, err := s.cron.AddFunc(schedule, func() {
			s.runConnectorExtraction(job)
		}); err != nil {
			return fmt.Errorf("failed to add cron job for %s: %w", providerID, err)
		}
		s.logger.Info().
			Str("provider", providerID).
			Str("schedule", schedule).
			Bool("uses_default_schedule", strings.TrimSpace(source.Schedule) == "").
			Msg("Registered connector schedule")
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

func connectorSchedule(source provider.Source, fallback string) string {
	if schedule := strings.TrimSpace(source.Schedule); schedule != "" {
		return schedule
	}
	return strings.TrimSpace(fallback)
}

func (s *Scheduler) runConnectorExtraction(job *connectorJob) {
	source := job.connector.Provider()
	providerID := source.ID
	if !job.mu.TryLock() {
		s.logger.Warn().Str("provider", providerID).Msg("Skipping scheduled connector extraction because previous run is still active")
		return
	}
	defer job.mu.Unlock()

	s.logger.Info().Str("provider", providerID).Msg("Starting scheduled connector extraction")

	ctx := context.Background()
	opts := ingest.RunOptions{}
	if providerID == webgallery.ProviderID && len(s.cfg.Scheduler.Pages) > 0 {
		opts.AllowedPages = s.cfg.Scheduler.Pages
	}
	result, err := s.ingestor.RunConnectorWithOptions(ctx, job.connector, opts)
	if err != nil {
		s.logger.Error().Err(err).Str("provider", providerID).Msg("Connector ingestion failed")
	}

	s.logger.Info().
		Str("provider", providerID).
		Int("downloaded", result.PhotosDownloaded).
		Int("skipped", result.PhotosSkipped).
		Int("failed", result.PhotosFailed).
		Msg("Scheduled connector extraction completed")

	// Trigger PhotoPrism indexing if photos were downloaded
	if result.PhotosDownloaded > 0 {
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
