package api

import (
	"context"
	"net/http"
	"time"

	"ok-folio/internal/derivatives"
)

const thumbnailHotTierTTL = 24 * time.Hour

func (s *Server) serveThumbnailFromCache(w http.ResponseWriter, r *http.Request, entry derivatives.Entry) bool {
	if !s.thumbCache.Exists(entry) {
		return false
	}
	if data, ok := s.cache.GetBytes(r.Context(), entry.Key); ok {
		s.thumbCache.Touch(entry)
		w.Header().Set("X-OK-Folio-Thumbnail-Cache", "valkey-hit")
		_, _ = w.Write(data)
		return true
	}
	if data, ok := s.thumbCache.Read(entry); ok {
		s.cache.SetBytes(r.Context(), entry.Key, data, thumbnailHotTierTTL)
		w.Header().Set("X-OK-Folio-Thumbnail-Cache", "disk-hit")
		_, _ = w.Write(data)
		return true
	}
	return false
}

func (s *Server) storeThumbnail(entry derivatives.Entry, data []byte) {
	if err := s.thumbCache.Write(entry, data); err != nil {
		s.logger.Warn().Err(err).Str("path", entry.Path).Msg("Thumbnail disk cache write failed")
		return
	}
	s.cache.SetBytes(context.Background(), entry.Key, data, thumbnailHotTierTTL)
}
