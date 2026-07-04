package api

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"ok-folio/internal/derivatives"
)

const (
	// DefaultThumbnailSize is the bounding box (longest side, px) when no size
	// is requested. Sized up from the original 128px so grid tiles stay crisp
	// on HiDPI displays. Callers may request a size via ?w= (clamped below).
	DefaultThumbnailSize = derivatives.DefaultThumbnailSize
	MinThumbnailSize     = derivatives.MinThumbnailSize
	MaxThumbnailSize     = derivatives.MaxThumbnailSize

	immutableImageCacheControl = "public, max-age=31536000, immutable"
)

// handleImageThumbnail serves a thumbnail version of an image
func (s *Server) handleImageThumbnail(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid photo ID")
		return
	}

	// Get photo from database
	photo, err := s.db.GetPhotoByID(id)
	if err != nil {
		s.writeError(w, http.StatusNotFound, "Photo not found")
		return
	}

	// Build full file path (prepend base directory if path is relative)
	filePath := photo.FilePath
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(s.cfg.Storage.BaseDirectory, filePath)
	}

	size := thumbnailSize(r)

	baseETag, err := derivatives.Validator(photo, filePath)
	if err != nil {
		s.logger.Error().Err(err).Str("file_path", photo.FilePath).Msg("Failed to build image validator")
		s.writeError(w, http.StatusNotFound, "Image file not found")
		return
	}
	etag := thumbnailETag(baseETag, size)
	w.Header().Set("Cache-Control", immutableImageCacheControl)
	w.Header().Set("ETag", etag)
	w.Header().Set("Content-Type", "image/jpeg")
	if requestETagMatches(r, etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	entry := s.thumbCache.Entry(photo, size, baseETag)
	if tier, ok := s.serveThumbnailFromCache(w, r, entry); ok {
		s.thumbnailTiers.record(tier)
		return
	}

	// Cache miss: generate from OK Folio's own original. This is the normal path
	// and needs no legacy PhotoPrism storage mount.
	data, err := derivatives.GenerateThumbnail(r.Context(), filePath, size)
	if err == nil {
		w.Header().Set("X-OK-Folio-Thumbnail-Cache", "miss")
		s.storeThumbnail(entry, data)
		s.thumbnailTiers.record(thumbnailTierGenerated)
		_, _ = w.Write(data)
		return
	}

	// The own original is unavailable or undecodable. Fall back to the optional,
	// read-only legacy PhotoPrism storage thumbnail when one is configured and
	// present. This tier is measured so operators can see whether the legacy mount
	// is still needed before dropping it.
	if legacyData, ok := derivatives.LegacyThumbnail(s.cfg.Storage.LegacyThumbDirectory, photo, baseETag); ok {
		w.Header().Set("X-OK-Folio-Thumbnail-Cache", "legacy-storage")
		s.thumbnailTiers.record(thumbnailTierLegacyStorage)
		_, _ = w.Write(legacyData)
		return
	}

	s.logger.Error().Err(err).Str("file_path", photo.FilePath).Msg("Failed to generate thumbnail")
	s.writeError(w, http.StatusNotFound, "Image file not found")
}

// handleImageFull serves the full-size image
func (s *Server) handleImageFull(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid photo ID")
		return
	}

	// Get photo from database
	photo, err := s.db.GetPhotoByID(id)
	if err != nil {
		s.writeError(w, http.StatusNotFound, "Photo not found")
		return
	}

	// Build full file path (prepend base directory if path is relative)
	filePath := photo.FilePath
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(s.cfg.Storage.BaseDirectory, filePath)
	}

	etag, err := derivatives.Validator(photo, filePath)
	if err != nil {
		s.logger.Error().Err(err).Str("file_path", photo.FilePath).Msg("Failed to build image validator")
		s.writeError(w, http.StatusNotFound, "Image file not found")
		return
	}
	w.Header().Set("Cache-Control", immutableImageCacheControl)
	w.Header().Set("ETag", etag)
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", photo.FileName))
	if requestETagMatches(r, etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		s.writeError(w, http.StatusNotFound, "Image file not found")
		return
	}

	// Determine content type from file extension
	ext := strings.ToLower(filepath.Ext(filePath))
	var contentType string
	switch ext {
	case ".jpg", ".jpeg":
		contentType = "image/jpeg"
	case ".png":
		contentType = "image/png"
	case ".tif", ".tiff":
		contentType = "image/tiff"
	case ".gif":
		contentType = "image/gif"
	case ".webp":
		contentType = "image/webp"
	case ".heic", ".heif":
		contentType = "image/heic"
	default:
		contentType = "application/octet-stream"
	}

	// Set headers
	w.Header().Set("Content-Type", contentType)

	// Serve the file
	http.ServeFile(w, r, filePath)
}

func quoteETag(value string) string {
	return derivatives.QuoteETag(value)
}

func thumbnailETag(baseETag string, width int) string {
	return quoteETag(fmt.Sprintf("thumb-w%d-%s", width, strings.Trim(baseETag, `"`)))
}

// thumbnailSize returns the requested thumbnail bounding size (longest side),
// clamped to a safe range. Defaults to DefaultThumbnailSize when ?w= is absent.
func thumbnailSize(r *http.Request) int {
	size := DefaultThumbnailSize
	if v := r.URL.Query().Get("w"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			size = n
		}
	}
	if size < MinThumbnailSize {
		size = MinThumbnailSize
	}
	if size > MaxThumbnailSize {
		size = MaxThumbnailSize
	}
	return size
}

func requestETagMatches(r *http.Request, etag string) bool {
	for _, part := range strings.Split(r.Header.Get("If-None-Match"), ",") {
		candidate := strings.TrimSpace(part)
		if candidate == "*" || candidate == etag {
			return true
		}
	}
	return false
}

// handlePhotoDetail returns detailed information about a photo
func (s *Server) handlePhotoDetail(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid photo ID")
		return
	}

	// Get photo from database
	photo, err := s.db.GetPhotoByID(id)
	if err != nil {
		s.writeError(w, http.StatusNotFound, "Photo not found")
		return
	}

	// Get file info for additional metadata
	fileInfo, err := os.Stat(photo.FilePath)
	var fileModTime time.Time
	if err == nil {
		fileModTime = fileInfo.ModTime()
	}

	// Return photo details
	response := map[string]interface{}{
		"id":            photo.ID,
		"url":           photo.URL,
		"source_page":   photo.SourcePage,
		"source":        sourceDisplayName(photo.SourcePage),
		"provider":      providerDisplayName(providerIDFromSourcePage(photo.SourcePage)),
		"category":      categoryDisplayName(galleryCategoryIDFromSourcePage(photo.SourcePage)),
		"title":         photo.Title,
		"artist":        photo.Artist,
		"keywords":      photo.Keywords,
		"upload_date":   photo.UploadDate,
		"file_path":     photo.FilePath,
		"file_name":     photo.FileName,
		"downloaded_at": photo.DownloadedAt,
		"file_size":     photo.FileSize,
		"favorite":      photo.Favorite,
		"status":        photo.Status,
		"file_mod_time": fileModTime,
	}

	s.writeJSON(w, http.StatusOK, response)
}

// handleTodayPhotos returns photos downloaded today
func (s *Server) handleTodayPhotos(w http.ResponseWriter, r *http.Request) {
	limit, offset := s.parsePagination(r)

	photos, total, err := s.db.GetPhotosToday(limit, offset)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to fetch today's photos")
		s.writeError(w, http.StatusInternalServerError, "Failed to fetch photos")
		return
	}

	response := map[string]interface{}{
		"photos": photos,
		"total":  total,
		"limit":  limit,
		"offset": offset,
		"date":   time.Now().Format("2006-01-02"),
	}

	s.writeJSON(w, http.StatusOK, response)
}

// handleWeekPhotos returns photos downloaded in the last 7 days
func (s *Server) handleWeekPhotos(w http.ResponseWriter, r *http.Request) {
	limit, offset := s.parsePagination(r)

	photos, total, err := s.db.GetPhotosLastWeek(limit, offset)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to fetch week's photos")
		s.writeError(w, http.StatusInternalServerError, "Failed to fetch photos")
		return
	}

	response := map[string]interface{}{
		"photos": photos,
		"total":  total,
		"limit":  limit,
		"offset": offset,
		"days":   7,
	}

	s.writeJSON(w, http.StatusOK, response)
}

// parsePagination extracts limit and offset from query parameters
func (s *Server) parsePagination(r *http.Request) (limit int, offset int) {
	limit = 50 // default
	offset = 0

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 200 {
			limit = l
		}
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	return limit, offset
}
