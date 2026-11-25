package derivatives

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"image"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"ok-folio/internal/config"
	"ok-folio/internal/database"
)

const auditSniffBytes = 512

type AuditOptions struct {
	BatchSize int
	Limit     int
	Progress  int
	// Exclude marks undecodable rows status='failed' so gallery, thumbnail
	// warm, and embedding backfill sweeps all skip them. Excluded rows keep
	// their url_hash, so the next connector sweep re-downloads and restores
	// any piece the source still serves.
	Exclude bool
}

type AuditFinding struct {
	PhotoID     uint64
	FilePath    string
	FileSize    int64
	SniffedMIME string
	FirstBytes  string
	Class       string
	DecodeError string
	Excluded    bool
}

type AuditResult struct {
	Scanned     int
	Decodable   int
	Missing     int
	Undecodable int
	Excluded    int
	Findings    []AuditFinding
}

// AuditOriginals full-decodes every downloaded original with the same decoder
// set used by thumbnails and the embedding backfill, so a clean audit means
// those sweeps will not fail on decode. Undecodable originals are classified
// from their leading bytes; missing files are counted but never excluded,
// because an unmounted originals volume must not fail the whole catalog.
func AuditOriginals(ctx context.Context, db *database.DB, cfg config.StorageConfig, opts AuditOptions, logger zerolog.Logger) (AuditResult, error) {
	if db == nil || db.DB == nil {
		return AuditResult{}, fmt.Errorf("database is required")
	}
	batchSize := opts.BatchSize
	if batchSize <= 0 {
		batchSize = 500
	}
	progressEvery := opts.Progress
	if progressEvery <= 0 {
		progressEvery = 1000
	}

	var result AuditResult
	started := time.Now()
	var lastID uint64
	for {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		remaining := batchSize
		if opts.Limit > 0 {
			left := opts.Limit - result.Scanned
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
			auditPhoto(db, cfg, photo, opts.Exclude, &result, logger)
			if result.Scanned%progressEvery == 0 {
				logger.Info().
					Int("scanned", result.Scanned).
					Int("decodable", result.Decodable).
					Int("missing", result.Missing).
					Int("undecodable", result.Undecodable).
					Dur("elapsed", time.Since(started)).
					Msg("Originals audit progress")
			}
		}
	}
}

func auditPhoto(db *database.DB, cfg config.StorageConfig, photo database.DownloadedPhoto, exclude bool, result *AuditResult, logger zerolog.Logger) {
	filePath := resolveOriginalPath(cfg, photo.FilePath)
	file, err := os.Open(filePath)
	if err != nil {
		result.Missing++
		logger.Warn().Err(err).Uint64("photo_id", photo.ID).Str("file_path", photo.FilePath).Msg("Original file unreadable; skipping audit")
		return
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		result.Missing++
		logger.Warn().Err(err).Uint64("photo_id", photo.ID).Str("file_path", photo.FilePath).Msg("Original file unreadable; skipping audit")
		return
	}

	header := make([]byte, auditSniffBytes)
	n, err := io.ReadFull(file, header)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		result.Missing++
		logger.Warn().Err(err).Uint64("photo_id", photo.ID).Str("file_path", photo.FilePath).Msg("Original file unreadable; skipping audit")
		return
	}
	header = header[:n]

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		result.Missing++
		logger.Warn().Err(err).Uint64("photo_id", photo.ID).Str("file_path", photo.FilePath).Msg("Original file unreadable; skipping audit")
		return
	}
	_, _, decodeErr := image.Decode(file)
	if decodeErr == nil {
		result.Decodable++
		return
	}

	finding := AuditFinding{
		PhotoID:     photo.ID,
		FilePath:    photo.FilePath,
		FileSize:    info.Size(),
		SniffedMIME: http.DetectContentType(header),
		FirstBytes:  firstBytesHex(header),
		Class:       ClassifyUndecodable(header, info.Size()),
		DecodeError: decodeErr.Error(),
	}
	result.Undecodable++
	if exclude {
		if err := excludeUndecodable(db, photo.ID, finding); err != nil {
			logger.Warn().Err(err).Uint64("photo_id", photo.ID).Msg("Failed to exclude undecodable original")
		} else {
			finding.Excluded = true
			result.Excluded++
		}
	}
	result.Findings = append(result.Findings, finding)
	logger.Warn().
		Uint64("photo_id", photo.ID).
		Str("file_path", photo.FilePath).
		Int64("file_size", finding.FileSize).
		Str("sniffed_mime", finding.SniffedMIME).
		Str("first_bytes", finding.FirstBytes).
		Str("class", finding.Class).
		Str("decode_error", finding.DecodeError).
		Bool("excluded", finding.Excluded).
		Msg("Undecodable original")
}

func excludeUndecodable(db *database.DB, photoID uint64, finding AuditFinding) error {
	res := db.Model(&database.DownloadedPhoto{}).
		Where("id = ? AND status = ?", photoID, "downloaded").
		Updates(map[string]interface{}{
			"status":        "failed",
			"error_message": fmt.Sprintf("undecodable original (%s): %s", finding.Class, finding.DecodeError),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("photo %d is no longer status=downloaded", photoID)
	}
	return nil
}

// ClassifyUndecodable names why an original failed Go image decode, from its
// leading bytes: a legitimate format we do not register, a non-image payload
// (an HTML error page saved with an image extension is the classic webgallery
// case), or a truncated/corrupt file in a registered format.
func ClassifyUndecodable(header []byte, size int64) string {
	if size == 0 {
		return "empty-file"
	}
	if brand := isoBMFFBrand(header); brand != "" {
		return "unsupported-format-" + brand
	}
	if isJPEGXL(header) {
		return "unsupported-format-jxl"
	}
	if isTIFF(header) {
		return "truncated-or-corrupt-tiff"
	}
	mime := http.DetectContentType(header)
	switch {
	case strings.HasPrefix(mime, "text/html"):
		return "non-image-payload-html"
	case strings.HasPrefix(mime, "text/"),
		strings.HasPrefix(mime, "application/json"),
		strings.Contains(mime, "xml"):
		return "non-image-payload-text"
	case strings.HasPrefix(mime, "image/"):
		return "truncated-or-corrupt-" + strings.TrimPrefix(mime, "image/")
	}
	return "unknown-payload"
}

// isoBMFFBrand reports the coarse brand family (avif, heic) for ISO BMFF
// containers, the formats most likely to reach us without a registered Go
// decoder.
func isoBMFFBrand(header []byte) string {
	if len(header) < 12 || !bytes.Equal(header[4:8], []byte("ftyp")) {
		return ""
	}
	switch string(header[8:12]) {
	case "avif", "avis":
		return "avif"
	case "heic", "heix", "heim", "heis", "hevc", "hevx", "hevm", "hevs", "mif1", "msf1":
		return "heic"
	}
	return "iso-bmff"
}

func isJPEGXL(header []byte) bool {
	if len(header) >= 2 && header[0] == 0xFF && header[1] == 0x0A {
		return true
	}
	return len(header) >= 12 && bytes.Equal(header[:12], []byte("\x00\x00\x00\x0cJXL \x0d\x0a\x87\x0a"))
}

func isTIFF(header []byte) bool {
	return len(header) >= 4 &&
		(bytes.Equal(header[:4], []byte("II*\x00")) || bytes.Equal(header[:4], []byte("MM\x00*")))
}

func firstBytesHex(header []byte) string {
	if len(header) > 16 {
		header = header[:16]
	}
	return hex.EncodeToString(header)
}
