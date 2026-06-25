package api

import (
	"net/http"
	"strconv"
)

// handleSearch searches photos by query
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		s.writeError(w, http.StatusBadRequest, "Query parameter 'q' is required")
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	offsetStr := r.URL.Query().Get("offset")
	offset := 0
	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	photos, total, err := s.db.SearchPhotos(query, limit, offset)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to search photos")
		s.writeError(w, http.StatusInternalServerError, "Failed to search photos")
		return
	}

	response := map[string]interface{}{
		"photos": photos,
		"total":  total,
		"limit":  limit,
		"offset": offset,
		"query":  query,
	}

	s.writeJSON(w, http.StatusOK, response)
}

// handleArtists returns all artists with pagination
func (s *Server) handleArtists(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	offsetStr := r.URL.Query().Get("offset")
	offset := 0
	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	sortBy := r.URL.Query().Get("sort")
	if sortBy == "" {
		sortBy = "count" // Default sort by photo count
	}

	artists, total, err := s.db.GetAllArtists(limit, offset, sortBy)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to get artists")
		s.writeError(w, http.StatusInternalServerError, "Failed to get artists")
		return
	}

	response := map[string]interface{}{
		"artists": artists,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
		"sort":    sortBy,
	}

	s.writeJSON(w, http.StatusOK, response)
}

// handleArtistDetail returns photos for a specific artist
func (s *Server) handleArtistDetail(w http.ResponseWriter, r *http.Request) {
	artist := r.URL.Query().Get("artist")
	if artist == "" {
		s.writeError(w, http.StatusBadRequest, "Query parameter 'artist' is required")
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	offsetStr := r.URL.Query().Get("offset")
	offset := 0
	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	photos, total, err := s.db.GetPhotosByArtist(artist, limit, offset)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to get photos by artist")
		s.writeError(w, http.StatusInternalServerError, "Failed to get photos by artist")
		return
	}

	response := map[string]interface{}{
		"artist": artist,
		"photos": photos,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	}

	s.writeJSON(w, http.StatusOK, response)
}

// handleTriggerIndex triggers PhotoPrism indexing manually
func (s *Server) handleTriggerIndex(w http.ResponseWriter, r *http.Request) {
	s.logger.Info().Msg("Manual index trigger requested")

	err := s.scraper.TriggerPhotoprismIndex(r.Context())
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to trigger PhotoPrism indexing")
		s.writeError(w, http.StatusInternalServerError, "Failed to trigger indexing: "+err.Error())
		return
	}

	response := map[string]interface{}{
		"message": "PhotoPrism indexing triggered successfully",
		"status":  "triggered",
	}

	s.writeJSON(w, http.StatusOK, response)
}
