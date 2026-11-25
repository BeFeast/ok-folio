package api

import (
	"net/http"
	"strconv"

	okfcache "ok-folio/internal/cache"
	"ok-folio/internal/database"

	"github.com/go-chi/chi/v5"
)

func (s *Server) handleGetFavoriteStatus(w http.ResponseWriter, r *http.Request) {
	photo, ok := s.photoFromRoute(w, r)
	if !ok {
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":        photo.ID,
		"favorite":  photo.Favorite,
		"available": true,
	})
}

func (s *Server) handleAddFavorite(w http.ResponseWriter, r *http.Request) {
	s.handleSetFavorite(w, r, true)
}

func (s *Server) handleRemoveFavorite(w http.ResponseWriter, r *http.Request) {
	s.handleSetFavorite(w, r, false)
}

func (s *Server) handleSetFavorite(w http.ResponseWriter, r *http.Request, favorite bool) {
	photo, ok := s.photoFromRoute(w, r)
	if !ok {
		return
	}

	if err := s.db.SetPhotoFavorite(photo.ID, favorite); err != nil {
		s.logger.Error().Err(err).Uint64("id", photo.ID).Bool("favorite", favorite).Msg("Failed to update favorite")
		s.writeError(w, http.StatusInternalServerError, "Failed to update favorite")
		return
	}
	_ = s.cache.Delete(r.Context(), okfcache.PhotoKey(photo.ID))
	_ = s.cache.BumpEpoch(r.Context())

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":        photo.ID,
		"favorite":  favorite,
		"available": true,
	})
}

func (s *Server) photoFromRoute(w http.ResponseWriter, r *http.Request) (*database.DownloadedPhoto, bool) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid photo ID")
		return nil, false
	}

	photo, err := s.db.GetPhotoByID(id)
	if err != nil {
		s.writeError(w, http.StatusNotFound, "Photo not found")
		return nil, false
	}

	return photo, true
}
