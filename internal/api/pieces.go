package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"ok-folio/internal/catalogquality"
	"ok-folio/internal/config"
	"ok-folio/internal/database"
	"ok-folio/internal/derivatives"
	"ok-folio/internal/exif"

	"github.com/disintegration/imaging"
	"gorm.io/gorm"

	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

const maxPieceUploadBytes = 50 << 20

var safeUploadNamePart = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

type createPieceResponse struct {
	Photo     database.DownloadedPhoto `json:"photo"`
	Duplicate bool                     `json:"duplicate"`
}

func (s *Server) handleCreatePiece(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxPieceUploadBytes+1024*1024)
	if err := r.ParseMultipartForm(maxPieceUploadBytes + 1024*1024); err != nil {
		s.writeError(w, http.StatusBadRequest, "Upload must be multipart/form-data and no larger than 50 MiB")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "Image file is required")
		return
	}
	defer file.Close()

	upload, err := readAndValidatePieceUpload(file, header)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	var existing database.DownloadedPhoto
	if err := s.db.Where("content_hash = ?", upload.contentHash).First(&existing).Error; err == nil {
		s.writeJSON(w, http.StatusOK, createPieceResponse{Photo: existing, Duplicate: true})
		return
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		s.logger.Error().Err(err).Msg("Failed to check uploaded piece duplicate")
		s.writeError(w, http.StatusInternalServerError, "Failed to import piece")
		return
	}

	relPath, err := s.writeUploadedOriginal(upload, header.Filename)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to write uploaded original")
		s.writeError(w, http.StatusInternalServerError, "Failed to store piece")
		return
	}

	now := time.Now().UTC()
	photo := database.DownloadedPhoto{
		URL:            "upload://" + upload.hexHash,
		SourcePage:     strings.TrimSpace(r.FormValue("source")),
		Title:          catalogquality.NormalizeTitle(r.FormValue("title"), header.Filename, filepath.Base(relPath)),
		Artist:         strings.TrimSpace(r.FormValue("artist")),
		UploadDate:     parsePieceUploadDate(r.FormValue("date")),
		FilePath:       relPath,
		FileName:       filepath.Base(relPath),
		ImageWidth:     upload.width,
		ImageHeight:    upload.height,
		CapturedAt:     upload.capturedAt,
		CameraMake:     upload.cameraMake,
		CameraModel:    upload.cameraModel,
		LensModel:      upload.lensModel,
		Orientation:    upload.orientation,
		GPSLatitude:    upload.gpsLatitude,
		GPSLongitude:   upload.gpsLongitude,
		DownloadedAt:   &now,
		FileSize:       int64(len(upload.data)),
		Notes:          strings.TrimSpace(r.FormValue("notes")),
		Provider:       "upload",
		Category:       "upload",
		ContentHash:    upload.contentHash,
		PerceptualHash: upload.perceptualHash,
		Status:         "downloaded",
	}

	if err := s.db.Create(&photo).Error; err != nil {
		if database.IsUniqueViolation(err) {
			if lookupErr := s.db.Where("content_hash = ?", upload.contentHash).First(&existing).Error; lookupErr == nil {
				s.writeJSON(w, http.StatusOK, createPieceResponse{Photo: existing, Duplicate: true})
				return
			}
		}
		s.logger.Error().Err(err).Msg("Failed to record uploaded piece")
		s.writeError(w, http.StatusInternalServerError, "Failed to import piece")
		return
	}

	s.statsCache.Invalidate()
	_ = s.cache.BumpEpoch(context.Background())
	s.warmUploadedPiece(r.Context(), &photo)
	s.writeJSON(w, http.StatusCreated, createPieceResponse{Photo: photo, Duplicate: false})
}

type pieceUpload struct {
	data           []byte
	contentHash    []byte
	hexHash        string
	format         string
	width          int
	height         int
	capturedAt     *time.Time
	cameraMake     string
	cameraModel    string
	lensModel      string
	orientation    string
	gpsLatitude    *float64
	gpsLongitude   *float64
	perceptualHash int64
}

func readAndValidatePieceUpload(file multipart.File, header *multipart.FileHeader) (pieceUpload, error) {
	if header == nil {
		return pieceUpload{}, fmt.Errorf("image file is required")
	}
	if header.Size > maxPieceUploadBytes {
		return pieceUpload{}, fmt.Errorf("image must be no larger than 50 MiB")
	}

	var buf bytes.Buffer
	limited := io.LimitReader(file, maxPieceUploadBytes+1)
	if _, err := io.Copy(&buf, limited); err != nil {
		return pieceUpload{}, fmt.Errorf("failed to read image")
	}
	if buf.Len() == 0 {
		return pieceUpload{}, fmt.Errorf("image file is empty")
	}
	if buf.Len() > maxPieceUploadBytes {
		return pieceUpload{}, fmt.Errorf("image must be no larger than 50 MiB")
	}

	cfg, format, err := image.DecodeConfig(bytes.NewReader(buf.Bytes()))
	if err != nil {
		return pieceUpload{}, fmt.Errorf("file must be a supported image")
	}
	if !acceptedPieceImageFormat(format) {
		return pieceUpload{}, fmt.Errorf("file must be JPEG, PNG, TIFF, or WebP")
	}
	embedded, _ := exif.DecodeEmbeddedMetadata(bytes.NewReader(buf.Bytes()))

	img, _, err := image.Decode(bytes.NewReader(buf.Bytes()))
	if err != nil {
		return pieceUpload{}, fmt.Errorf("file must be a supported image")
	}

	sum := sha256.Sum256(buf.Bytes())
	return pieceUpload{
		data:           buf.Bytes(),
		contentHash:    append([]byte(nil), sum[:]...),
		hexHash:        hex.EncodeToString(sum[:]),
		format:         format,
		width:          cfg.Width,
		height:         cfg.Height,
		capturedAt:     embedded.CapturedAt,
		cameraMake:     embedded.CameraMake,
		cameraModel:    embedded.CameraModel,
		lensModel:      embedded.LensModel,
		orientation:    embedded.Orientation,
		gpsLatitude:    embedded.GPSLatitude,
		gpsLongitude:   embedded.GPSLongitude,
		perceptualHash: averageImageHash(img),
	}, nil
}

func acceptedPieceImageFormat(format string) bool {
	switch strings.ToLower(format) {
	case "jpeg", "png", "tiff", "webp":
		return true
	default:
		return false
	}
}

func (s *Server) writeUploadedOriginal(upload pieceUpload, originalName string) (string, error) {
	ext := strings.ToLower(filepath.Ext(originalName))
	if ext == "" {
		ext = "." + upload.format
		if ext == ".jpeg" {
			ext = ".jpg"
		}
	}
	base := strings.TrimSuffix(filepath.Base(originalName), filepath.Ext(originalName))
	base = strings.Trim(safeUploadNamePart.ReplaceAllString(base, "-"), "-._")
	if base == "" {
		base = "piece"
	}
	name := fmt.Sprintf("%s-%s%s", upload.hexHash[:16], base, ext)
	relPath := filepath.Join("Uploads", upload.hexHash[:2], name)
	absPath := filepath.Join(s.cfg.Storage.BaseDirectory, relPath)
	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(absPath, upload.data, 0o644); err != nil {
		return "", err
	}
	return relPath, nil
}

func parsePieceUploadDate(value string) *time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02", "2006-01", "2006"} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			utc := parsed.UTC()
			return &utc
		}
	}
	return nil
}

func averageImageHash(img image.Image) int64 {
	const size = 8
	thumb := imaging.Resize(img, size, size, imaging.Lanczos)
	values := make([]uint32, 0, size*size)
	var total uint32
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			r, g, b, _ := thumb.At(x, y).RGBA()
			luma := uint32((299*r + 587*g + 114*b) / 1000)
			values = append(values, luma)
			total += luma
		}
	}
	avg := total / uint32(len(values))
	var hash uint64
	for i, value := range values {
		if value >= avg {
			hash |= 1 << uint(i)
		}
	}
	return int64(hash)
}

func (s *Server) warmUploadedPiece(ctx context.Context, photo *database.DownloadedPhoto) {
	widths := s.cfg.Storage.WarmOnIngestWidths
	if len(widths) == 0 {
		widths = []int{config.DefaultWarmOnIngestWidthSmall, config.DefaultWarmOnIngestWidthLarge}
	}
	result, err := derivatives.WarmOnePhoto(ctx, s.cfg.Storage, photo, derivatives.WarmPhotoOptions{
		Widths:   widths,
		HotCache: s.cache,
		HotTTL:   thumbnailHotTierTTL,
	}, s.logger.With().Str("component", "manual-upload-thumbnail-warmer").Logger())
	if err != nil {
		s.logger.Warn().Err(err).Uint64("photo_id", photo.ID).Msg("Uploaded piece thumbnail warm failed")
		return
	}
	if result.Failed > 0 || result.Missing > 0 {
		s.logger.Warn().
			Uint64("photo_id", photo.ID).
			Int("generated", result.Generated).
			Int("failed", result.Failed).
			Int("missing", result.Missing).
			Msg("Uploaded piece thumbnail warm incomplete")
	}
}
