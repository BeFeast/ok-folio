package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
	"ok-folio/internal/database"
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
	decorated := []database.InboxItem{*item}
	if err := s.decorateInboxCovers(decorated); err != nil {
		s.logger.Error().Err(err).Msg("Failed to resolve inbox item cover")
		s.writeError(w, http.StatusInternalServerError, "Failed to fetch inbox item")
		return
	}
	s.writeJSON(w, http.StatusOK, decorated[0])
}

func (s *Server) decorateInboxCovers(items []database.InboxItem) error {
	hashes := make([][]byte, 0)
	seen := make(map[string]struct{})
	for idx := range items {
		if items[idx].Status != "duplicate" || len(items[idx].ContentHash) == 0 {
			continue
		}
		key := string(items[idx].ContentHash)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		hashes = append(hashes, items[idx].ContentHash)
	}
	if len(hashes) == 0 {
		return nil
	}

	photos, err := s.db.GetPhotosByContentHashes(hashes)
	if err != nil {
		return err
	}
	photosByHash := make(map[string]uint64, len(photos))
	for _, photo := range photos {
		if len(photo.ContentHash) > 0 {
			photosByHash[string(photo.ContentHash)] = photo.ID
		}
	}
	for idx := range items {
		photoID, ok := photosByHash[string(items[idx].ContentHash)]
		if !ok {
			continue
		}
		items[idx].CoverPhotoID = &photoID
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
