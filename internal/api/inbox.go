package api

import (
	"errors"
	"net/http"
	"strconv"

	"ok-folio/internal/database"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
)

const (
	DefaultInboxLimit = 50
	MaxInboxLimit     = 200
)

func (s *Server) handleInbox(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	if status != "" && status != "duplicate" && status != "ambiguous" {
		s.writeError(w, http.StatusBadRequest, "Invalid inbox status")
		return
	}

	limit := DefaultInboxLimit
	if value := r.URL.Query().Get("limit"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 && parsed <= MaxInboxLimit {
			limit = parsed
		}
	}

	offset := 0
	if value := r.URL.Query().Get("offset"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	items, total, err := s.db.GetInboxExceptionsFiltered(status, limit, offset)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to fetch inbox exceptions")
		s.writeError(w, http.StatusInternalServerError, "Failed to fetch inbox")
		return
	}
	if err := s.decorateInboxCovers(items); err != nil {
		s.logger.Error().Err(err).Msg("Failed to resolve inbox covers")
		s.writeError(w, http.StatusInternalServerError, "Failed to fetch inbox")
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"items":  items,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

func (s *Server) handleInboxItem(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id == 0 {
		s.writeError(w, http.StatusBadRequest, "Invalid inbox item ID")
		return
	}
	item, err := s.db.GetInboxItem(id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		s.writeError(w, http.StatusNotFound, "Inbox item not found")
		return
	}
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to fetch inbox item")
		s.writeError(w, http.StatusInternalServerError, "Failed to fetch inbox item")
		return
	}
	if err := s.decorateInboxCover(item); err != nil {
		s.logger.Error().Err(err).Msg("Failed to resolve inbox cover")
		s.writeError(w, http.StatusInternalServerError, "Failed to fetch inbox item")
		return
	}
	s.writeJSON(w, http.StatusOK, item)
}

func (s *Server) decorateInboxCover(item *database.InboxItem) error {
	if item.Status != "duplicate" || len(item.ContentHash) == 0 {
		return nil
	}
	photo, err := s.db.GetPhotoByContentHash(item.ContentHash)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	item.CoverPhotoID = &photo.ID
	return nil
}

func (s *Server) decorateInboxCovers(items []database.InboxItem) error {
	hashes := make([][]byte, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		if item.Status != "duplicate" || len(item.ContentHash) == 0 {
			continue
		}
		key := string(item.ContentHash)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		hashes = append(hashes, item.ContentHash)
	}
	if len(hashes) == 0 {
		return nil
	}

	photos, err := s.db.GetPhotosByContentHashes(hashes)
	if err != nil {
		return err
	}
	covers := make(map[string]uint64, len(photos))
	for _, photo := range photos {
		if len(photo.ContentHash) == 0 {
			continue
		}
		covers[string(photo.ContentHash)] = photo.ID
	}
	for idx := range items {
		id, ok := covers[string(items[idx].ContentHash)]
		if !ok {
			continue
		}
		items[idx].CoverPhotoID = &id
	}
	return nil
}

func (s *Server) handleDismissInboxItem(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id == 0 {
		s.writeError(w, http.StatusBadRequest, "Invalid inbox item ID")
		return
	}
	err = s.db.DeleteInboxItem(id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		s.writeError(w, http.StatusNotFound, "Inbox item not found")
		return
	}
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to dismiss inbox item")
		s.writeError(w, http.StatusInternalServerError, "Failed to dismiss inbox item")
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]bool{"dismissed": true})
}

func (s *Server) handleInboxCounts(w http.ResponseWriter, r *http.Request) {
	counts, err := s.db.CountInboxByStatus()
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to count inbox exceptions")
		s.writeError(w, http.StatusInternalServerError, "Failed to count inbox")
		return
	}
	var total int64
	for _, count := range counts {
		total += count
	}
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"counts": counts,
		"total":  total,
	})
}
