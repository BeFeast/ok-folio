package similarity

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"

	"ok-folio/internal/config"
	"ok-folio/internal/database"
	"ok-folio/internal/derivatives"
	"ok-folio/internal/embedder"
)

const (
	EmbedThumbnailWidth = 400
	MaxConcurrency      = 4
)

type Client interface {
	Embed(context.Context, []byte) ([]float32, error)
}

type Options struct {
	BatchSize   int
	Concurrency int
	Limit       int
	Progress    int
}

type Result struct {
	Scanned   int64
	Embedded  int64
	Skipped   int64
	Missing   int64
	Permanent int64
	Failed    int64
}

type job struct {
	photo database.DownloadedPhoto
}

func NewClient(sidecarURL string) *embedder.Client {
	return embedder.New(sidecarURL)
}

func Backfill(ctx context.Context, db *database.DB, storage config.StorageConfig, client Client, opts Options, logger zerolog.Logger) (Result, error) {
	if db == nil || db.DB == nil {
		return Result{}, fmt.Errorf("database is required")
	}
	if client == nil {
		return Result{}, fmt.Errorf("embedder client is required")
	}
	if db.Dialector.Name() != "postgres" || !db.HasEmbeddingColumn() {
		logger.Warn().Msg("Similarity backfill skipped because pgvector embedding column is unavailable")
		return Result{}, nil
	}
	batchSize := opts.BatchSize
	if batchSize <= 0 {
		batchSize = 500
	}
	concurrency := opts.Concurrency
	if concurrency <= 0 {
		concurrency = 2
	}
	if concurrency > MaxConcurrency {
		logger.Warn().Int("requested", concurrency).Int("max", MaxConcurrency).Msg("Similarity backfill concurrency clamped")
		concurrency = MaxConcurrency
	}
	progressEvery := opts.Progress
	if progressEvery <= 0 {
		progressEvery = 500
	}

	jobs := make(chan job)
	var result atomicResult
	var workers sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for item := range jobs {
				processOne(ctx, db, storage, client, item.photo, &result, logger)
			}
		}()
	}

	started := time.Now()
	err := enqueue(ctx, db, batchSize, opts.Limit, progressEvery, jobs, &result, started, logger)
	close(jobs)
	workers.Wait()
	out := result.snapshot()
	if err != nil {
		return out, err
	}
	// Permanent decode failures repeat on every sweep, so only transient
	// failures (sidecar, DB, context) block index creation.
	if out.Failed == 0 {
		if err := db.CreateEmbeddingHNSWIndex(); err != nil {
			return out, err
		}
	}
	if ids := result.permanentIDsSnapshot(); len(ids) > 0 {
		logger.Warn().
			Uints64("photo_ids", ids).
			Int("count", len(ids)).
			Msg("Similarity backfill skipped photos with permanently undecodable originals")
	}
	logger.Info().
		Int64("scanned", out.Scanned).
		Int64("embedded", out.Embedded).
		Int64("skipped", out.Skipped).
		Int64("missing", out.Missing).
		Int64("permanent", out.Permanent).
		Int64("failed", out.Failed).
		Msg("Similarity embedding backfill completed")
	return out, nil
}

func EmbedPhoto(ctx context.Context, db *database.DB, storage config.StorageConfig, client Client, photo *database.DownloadedPhoto) error {
	if db == nil || db.DB == nil || photo == nil {
		return fmt.Errorf("database and photo are required")
	}
	if db.Dialector.Name() != "postgres" || !db.HasEmbeddingColumn() {
		return nil
	}
	var hasEmbedding bool
	if err := db.Raw("SELECT embedding IS NOT NULL FROM downloaded_photos WHERE id = ?", photo.ID).Scan(&hasEmbedding).Error; err != nil {
		return err
	}
	if hasEmbedding {
		return nil
	}
	jpegBytes, _, err := derivatives.JPEGForEmbedding(ctx, storage, photo, EmbedThumbnailWidth)
	if err != nil {
		return err
	}
	embedding, err := client.Embed(ctx, jpegBytes)
	if err != nil {
		return err
	}
	return db.StoreEmbedding(photo.ID, embedding)
}

func processOne(ctx context.Context, db *database.DB, storage config.StorageConfig, client Client, photo database.DownloadedPhoto, result *atomicResult, logger zerolog.Logger) {
	if err := ctx.Err(); err != nil {
		result.addFailed()
		logger.Warn().Err(err).Uint64("photo_id", photo.ID).Msg("Similarity embed skipped because context ended")
		return
	}
	jpegBytes, _, err := derivatives.JPEGForEmbedding(ctx, storage, &photo, EmbedThumbnailWidth)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			result.addMissing()
			logger.Warn().Err(err).Uint64("photo_id", photo.ID).Str("file_path", photo.FilePath).Msg("Original file missing; skipping embedding")
			return
		}
		if errors.Is(err, derivatives.ErrUndecodable) {
			result.addPermanent(photo.ID)
			logger.Warn().Err(err).Uint64("photo_id", photo.ID).Str("file_path", photo.FilePath).Msg("Original image is undecodable; skipping embedding permanently")
			return
		}
		result.addFailed()
		logger.Warn().Err(err).Uint64("photo_id", photo.ID).Msg("Thumbnail unavailable; skipping embedding")
		return
	}
	embedding, err := client.Embed(ctx, jpegBytes)
	if err != nil {
		result.addFailed()
		logger.Warn().Err(err).Uint64("photo_id", photo.ID).Msg("Embedder request failed")
		return
	}
	if err := db.StoreEmbedding(photo.ID, embedding); err != nil {
		result.addFailed()
		logger.Warn().Err(err).Uint64("photo_id", photo.ID).Msg("Embedding update failed")
		return
	}
	result.addEmbedded()
}

func enqueue(ctx context.Context, db *database.DB, batchSize int, limit int, progressEvery int, jobs chan<- job, result *atomicResult, started time.Time, logger zerolog.Logger) error {
	var lastID uint64
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		remaining := batchSize
		scanned := int(result.scanned.Load())
		if limit > 0 {
			left := limit - scanned
			if left <= 0 {
				return nil
			}
			if left < remaining {
				remaining = left
			}
		}
		var photos []database.DownloadedPhoto
		err := db.Where("status = ? AND embedding IS NULL AND id > ?", "downloaded", lastID).
			Order("id ASC").
			Limit(remaining).
			Find(&photos).Error
		if err != nil {
			return err
		}
		if len(photos) == 0 {
			return nil
		}
		for _, photo := range photos {
			lastID = photo.ID
			current := result.addScanned()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case jobs <- job{photo: photo}:
			}
			if current%int64(progressEvery) == 0 {
				out := result.snapshot()
				logger.Info().
					Int64("scanned", out.Scanned).
					Int64("embedded", out.Embedded).
					Int64("skipped", out.Skipped).
					Int64("missing", out.Missing).
					Int64("permanent", out.Permanent).
					Int64("failed", out.Failed).
					Dur("elapsed", time.Since(started)).
					Msg("Similarity embedding backfill progress")
			}
		}
	}
}

type atomicResult struct {
	scanned   atomic.Int64
	embedded  atomic.Int64
	skipped   atomic.Int64
	missing   atomic.Int64
	permanent atomic.Int64
	failed    atomic.Int64

	permanentMu  sync.Mutex
	permanentIDs []uint64
}

func (r *atomicResult) addScanned() int64 { return r.scanned.Add(1) }
func (r *atomicResult) addEmbedded()      { r.embedded.Add(1) }
func (r *atomicResult) addMissing()       { r.missing.Add(1) }
func (r *atomicResult) addFailed()        { r.failed.Add(1) }

func (r *atomicResult) addPermanent(photoID uint64) {
	r.permanent.Add(1)
	r.permanentMu.Lock()
	r.permanentIDs = append(r.permanentIDs, photoID)
	r.permanentMu.Unlock()
}

func (r *atomicResult) permanentIDsSnapshot() []uint64 {
	r.permanentMu.Lock()
	ids := append([]uint64(nil), r.permanentIDs...)
	r.permanentMu.Unlock()
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func (r *atomicResult) snapshot() Result {
	return Result{
		Scanned:   r.scanned.Load(),
		Embedded:  r.embedded.Load(),
		Skipped:   r.skipped.Load(),
		Missing:   r.missing.Load(),
		Permanent: r.permanent.Load(),
		Failed:    r.failed.Load(),
	}
}
