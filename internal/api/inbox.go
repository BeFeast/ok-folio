package api

import (
	"net/http"
	"strconv"
)

const (
	DefaultInboxLimit = 50
	MaxInboxLimit     = 200
)

func (s *Server) handleInbox(w http.ResponseWriter, r *http.Request) {
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

	items, total, err := s.db.GetInboxExceptions(limit, offset)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to fetch inbox exceptions")
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
