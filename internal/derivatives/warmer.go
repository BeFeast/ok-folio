package derivatives

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"ok-folio/internal/config"
	"ok-folio/internal/database"
)

var ErrNoWidths = errors.New("at least one thumbnail width is required")

const (
	MaxWarmConcurrency = 8

	warmPruneEveryWrites = 1000
	tempSweepAge         = time.Hour
	tempSweepInterval    = time.Hour
)

type WarmOptions struct {
	Widths      []int
	Concurrency int
	BatchSize   int
	Limit       int
	Progress    int
}

type WarmResult struct {
	Scanned   int
	Generated int
	Skipped   int
	Missing   int
	Failed    int
}

type warmJob struct {
	photo    database.DownloadedPhoto
	filePath string
	width    int
	entry    Entry
}

type warmCacheWriter struct {
	cache      *Cache
	pruneEvery int
	mu         sync.Mutex
	writes     int
}

func (w *warmCacheWriter) Write(entry Entry, data []byte) error {
	if err := w.cache.write(entry, data); err != nil {
		return err
	}
	if w.pruneEvery <= 0 {
		return nil
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	w.writes++
	if w.writes < w.pruneEvery {
		return nil
	}
	w.writes = 0
	return w.cache.Prune()
}

func (w *warmCacheWriter) Prune() error {
	return w.cache.Prune()
}

func WarmThumbnails(ctx context.Context, db *database.DB, cfg config.StorageConfig, opts WarmOptions, logger zerolog.Logger) (WarmResult, error) {
	widths, err := normalizeWidths(opts.Widths)
	if err != nil {
		return WarmResult{}, err
	}
	concurrency := opts.Concurrency
	concurrency = normalizeWarmConcurrency(concurrency, logger)
	batchSize := opts.BatchSize
	if batchSize <= 0 {
		batchSize = 500
	}
	progressEvery := opts.Progress
	if progressEvery <= 0 {
		progressEvery = 100
	}

	cache := NewCache(cfg)
	if err := cache.SweepTempFiles(tempSweepAge); err != nil {
		return WarmResult{}, err
	}
	sweepCtx, stopSweep := context.WithCancel(ctx)
	defer stopSweep()
	go sweepTempFilesPeriodically(sweepCtx, cache, tempSweepAge, tempSweepInterval, logger)

	cacheWriter := &warmCacheWriter{cache: cache, pruneEvery: warmPruneEveryWrites}
	jobs := make(chan warmJob)
	results := make(chan WarmResult, concurrency)

	var workers sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			var result WarmResult
			for job := range jobs {
				data, err := GenerateThumbnail(ctx, job.filePath, job.width)
				if err != nil {
					result.Failed++
					logger.Warn().Err(err).Uint64("photo_id", job.photo.ID).Int("width", job.width).Str("file_path", job.photo.FilePath).Msg("Thumbnail warm failed")
					continue
				}
				if err := cacheWriter.Write(job.entry, data); err != nil {
					result.Failed++
					logger.Warn().Err(err).Uint64("photo_id", job.photo.ID).Int("width", job.width).Str("path", job.entry.Path).Msg("Thumbnail cache write failed")
					continue
				}
				result.Generated++
			}
			results <- result
		}()
	}

	result, err := enqueueWarmJobs(ctx, db, cfg, cache, widths, batchSize, opts.Limit, progressEvery, jobs, logger)
	close(jobs)
	workers.Wait()
	close(results)
	for workerResult := range results {
		result.Generated += workerResult.Generated
		result.Failed += workerResult.Failed
	}
	pruneErr := cacheWriter.Prune()
	if err != nil {
		return result, err
	}
	if pruneErr != nil {
		return result, pruneErr
	}

	logger.Info().
		Int("scanned", result.Scanned).
		Int("generated", result.Generated).
		Int("skipped", result.Skipped).
		Int("missing", result.Missing).
		Int("failed", result.Failed).
		Msg("Thumbnail warm completed")
	return result, nil
}

func normalizeWarmConcurrency(concurrency int, logger zerolog.Logger) int {
	if concurrency <= 0 {
		return 2
	}
	if concurrency > MaxWarmConcurrency {
		logger.Warn().Int("requested", concurrency).Int("max", MaxWarmConcurrency).Msg("Thumbnail warm concurrency clamped")
		return MaxWarmConcurrency
	}
	return concurrency
}

func sweepTempFilesPeriodically(ctx context.Context, cache *Cache, maxAge time.Duration, interval time.Duration, logger zerolog.Logger) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := cache.SweepTempFiles(maxAge); err != nil {
				logger.Warn().Err(err).Msg("Thumbnail temp sweep failed")
			}
		}
	}
}

func enqueueWarmJobs(
	ctx context.Context,
	db *database.DB,
	cfg config.StorageConfig,
	cache *Cache,
	widths []int,
	batchSize int,
	limit int,
	progressEvery int,
	jobs chan<- warmJob,
	logger zerolog.Logger,
) (WarmResult, error) {
	var result WarmResult
	var lastID uint64
	started := time.Now()
	for {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		remaining := batchSize
		if limit > 0 {
			left := limit - result.Scanned
			if left <= 0 {
				return result, nil
			}
			if left < remaining {
				remaining = left
			}
		}

		var photos []database.DownloadedPhoto
		err := db.DB.
			Where("status = ? AND id > ?", "downloaded", lastID).
			Order("id ASC").
			Limit(remaining).
			Find(&photos).Error
		if err != nil {
			return result, err
		}
		if len(photos) == 0 {
			return result, nil
		}

		for _, photo := range photos {
			lastID = photo.ID
			result.Scanned++
			filePath := resolveOriginalPath(cfg, photo.FilePath)
			if _, err := os.Stat(filePath); err != nil {
				result.Missing += len(widths)
				logger.Warn().Err(err).Uint64("photo_id", photo.ID).Str("file_path", photo.FilePath).Msg("Original file missing; skipping thumbnails")
				continue
			}
			validator, err := Validator(&photo, filePath)
			if err != nil {
				result.Missing += len(widths)
				logger.Warn().Err(err).Uint64("photo_id", photo.ID).Str("file_path", photo.FilePath).Msg("Original file unavailable; skipping thumbnails")
				continue
			}
			for _, width := range widths {
				entry := cache.Entry(&photo, width, validator)
				if cache.Exists(entry) {
					result.Skipped++
					continue
				}
				select {
				case <-ctx.Done():
					return result, ctx.Err()
				case jobs <- warmJob{photo: photo, filePath: filePath, width: width, entry: entry}:
				}
			}
			if result.Scanned%progressEvery == 0 {
				logger.Info().
					Int("scanned", result.Scanned).
					Int("generated", result.Generated).
					Int("skipped", result.Skipped).
					Int("missing", result.Missing).
					Int("failed", result.Failed).
					Dur("elapsed", time.Since(started)).
					Msg("Thumbnail warm progress")
			}
		}
	}
}

func normalizeWidths(widths []int) ([]int, error) {
	if len(widths) == 0 {
		return nil, ErrNoWidths
	}
	seen := make(map[int]bool, len(widths))
	out := make([]int, 0, len(widths))
	for _, width := range widths {
		if width <= 0 {
			return nil, fmt.Errorf("invalid thumbnail width %d", width)
		}
		width = ClampWidth(width)
		if seen[width] {
			continue
		}
		seen[width] = true
		out = append(out, width)
	}
	sort.Ints(out)
	return out, nil
}

func resolveOriginalPath(cfg config.StorageConfig, filePath string) string {
	if filepath.IsAbs(filePath) {
		return filePath
	}
	return filepath.Join(cfg.BaseDirectory, filePath)
}
