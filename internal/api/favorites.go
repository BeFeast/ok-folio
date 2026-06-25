package api

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
)

// handleGetFavoriteStatus returns the favorite status of a photo
func (s *Server) handleGetFavoriteStatus(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid photo ID")
		return
	}

	// Check if PhotoPrism is enabled
	if !s.scraper.IsPhotoprismEnabled() {
		s.writeError(w, http.StatusServiceUnavailable, "PhotoPrism integration not enabled")
		return
	}

	// Get photo from database
	photo, err := s.db.GetPhotoByID(uint(id))
	if err != nil {
		s.writeError(w, http.StatusNotFound, "Photo not found")
		return
	}

	// Search PhotoPrism for the photo UID
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	ppClient := s.scraper.GetPhotoprismClient()
	_, isFavorite, err := ppClient.SearchByFilename(ctx, photo.FileName)
	if err != nil {
		s.logger.Warn().Err(err).Str("filename", photo.FileName).Msg("Failed to get favorite status from PhotoPrism")
		s.writeJSON(w, http.StatusOK, map[string]interface{}{
			"id":        id,
			"favorite":  false,
			"available": false,
			"error":     err.Error(),
		})
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":        id,
		"favorite":  isFavorite,
		"available": true,
	})
}

// handleAddFavorite adds a photo to favorites in PhotoPrism
func (s *Server) handleAddFavorite(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid photo ID")
		return
	}

	// Check if PhotoPrism is enabled
	if !s.scraper.IsPhotoprismEnabled() {
		s.writeError(w, http.StatusServiceUnavailable, "PhotoPrism integration not enabled")
		return
	}

	// Get photo from database
	photo, err := s.db.GetPhotoByID(uint(id))
	if err != nil {
		s.writeError(w, http.StatusNotFound, "Photo not found")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	ppClient := s.scraper.GetPhotoprismClient()

	// Search PhotoPrism for the photo UID
	uid, _, err := ppClient.SearchByFilename(ctx, photo.FileName)
	if err != nil {
		s.logger.Error().Err(err).Str("filename", photo.FileName).Msg("Failed to find photo in PhotoPrism")
		s.writeError(w, http.StatusNotFound, "Photo not found in PhotoPrism. It may not be indexed yet.")
		return
	}

	// Add to favorites
	if err := ppClient.LikePhoto(ctx, uid); err != nil {
		s.logger.Error().Err(err).Str("uid", uid).Msg("Failed to add photo to favorites")
		s.writeError(w, http.StatusInternalServerError, "Failed to add to favorites")
		return
	}

	s.logger.Info().Uint64("id", id).Str("uid", uid).Msg("Photo added to favorites")
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":       id,
		"uid":      uid,
		"favorite": true,
		"message":  "Photo added to favorites",
	})
}

// handleRemoveFavorite removes a photo from favorites in PhotoPrism
func (s *Server) handleRemoveFavorite(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid photo ID")
		return
	}

	// Check if PhotoPrism is enabled
	if !s.scraper.IsPhotoprismEnabled() {
		s.writeError(w, http.StatusServiceUnavailable, "PhotoPrism integration not enabled")
		return
	}

	// Get photo from database
	photo, err := s.db.GetPhotoByID(uint(id))
	if err != nil {
		s.writeError(w, http.StatusNotFound, "Photo not found")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	ppClient := s.scraper.GetPhotoprismClient()

	// Search PhotoPrism for the photo UID
	uid, _, err := ppClient.SearchByFilename(ctx, photo.FileName)
	if err != nil {
		s.logger.Error().Err(err).Str("filename", photo.FileName).Msg("Failed to find photo in PhotoPrism")
		s.writeError(w, http.StatusNotFound, "Photo not found in PhotoPrism. It may not be indexed yet.")
		return
	}

	// Remove from favorites
	if err := ppClient.DislikePhoto(ctx, uid); err != nil {
		s.logger.Error().Err(err).Str("uid", uid).Msg("Failed to remove photo from favorites")
		s.writeError(w, http.StatusInternalServerError, "Failed to remove from favorites")
		return
	}

	s.logger.Info().Uint64("id", id).Str("uid", uid).Msg("Photo removed from favorites")
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":       id,
		"uid":      uid,
		"favorite": false,
		"message":  "Photo removed from favorites",
	})
}
