package api

import (
	"fmt"
	"image"
	"image/jpeg"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/disintegration/imaging"
	"github.com/go-chi/chi/v5"
)

const (
	// Thumbnail dimensions
	ThumbnailWidth  = 128
	ThumbnailHeight = 96
	// JPEG quality for thumbnails
	ThumbnailQuality = 80
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

	// Open the image file
	imgFile, err := os.Open(filePath)
	if err != nil {
		s.logger.Error().Err(err).Str("file_path", photo.FilePath).Msg("Failed to open image file")
		s.writeError(w, http.StatusNotFound, "Image file not found")
		return
	}
	defer imgFile.Close()

	// Decode image
	img, _, err := image.Decode(imgFile)
	if err != nil {
		s.logger.Error().Err(err).Str("file_path", photo.FilePath).Msg("Failed to decode image")
		s.writeError(w, http.StatusInternalServerError, "Failed to decode image")
		return
	}

	// Create thumbnail (fit into 128x96 rectangle)
	thumbnail := imaging.Fit(img, ThumbnailWidth, ThumbnailHeight, imaging.Lanczos)

	// Set cache headers (thumbnails don't change)
	w.Header().Set("Cache-Control", "public, max-age=86400") // 24 hours
	w.Header().Set("Content-Type", "image/jpeg")

	// Encode and send thumbnail
	if err := jpeg.Encode(w, thumbnail, &jpeg.Options{Quality: ThumbnailQuality}); err != nil {
		s.logger.Error().Err(err).Msg("Failed to encode thumbnail")
		return
	}
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
	case ".gif":
		contentType = "image/gif"
	case ".webp":
		contentType = "image/webp"
	default:
		contentType = "application/octet-stream"
	}

	// Set headers
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=86400") // 24 hours
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", photo.FileName))

	// Serve the file
	http.ServeFile(w, r, filePath)
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
