package scheduler

import (
	"context"
	"fmt"

	"ok-folio/internal/config"
	"ok-folio/internal/database"
	"ok-folio/internal/scraper"

	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog"
)

type Scheduler struct {
	cfg     *config.Config
	db      *database.DB
	scraper *scraper.Scraper
	logger  zerolog.Logger
	cron    *cron.Cron
}

func New(cfg *config.Config, db *database.DB, scraper *scraper.Scraper, logger zerolog.Logger) *Scheduler {
	return &Scheduler{
		cfg:     cfg,
		db:      db,
		scraper: scraper,
		logger:  logger,
		cron:    cron.New(cron.WithSeconds()),
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
		s.logger.Info().Int("page", page).Msg("Scraping page")

		downloaded, skipped, failed, err := s.scraper.ScrapePage(ctx, page)
		if err != nil {
			s.logger.Error().Err(err).Int("page", page).Msg("Failed to scrape page")
			// Continue with other pages even if one fails
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
		Msg("Scheduled extraction completed")

	// Trigger PhotoPrism indexing if photos were downloaded
	if totalDownloaded > 0 {
		if err := s.scraper.TriggerPhotoprismIndex(ctx); err != nil {
			s.logger.Warn().Err(err).Msg("Failed to trigger PhotoPrism indexing")
		}
	}
}
