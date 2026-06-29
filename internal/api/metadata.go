package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	okfcache "ok-folio/internal/cache"
	"ok-folio/internal/database"

	"gorm.io/gorm"
)

const maxBulkEditIDs = 500

type pieceMetadataPatch struct {
	Title    *string          `json:"title"`
	Artist   *string          `json:"artist"`
	Date     *json.RawMessage `json:"date"`
	Keywords *[]string        `json:"keywords"`
}

type bulkEditRequest struct {
	IDs            []uint64 `json:"ids"`
	SetArtist      *string  `json:"set_artist"`
	SetDate        *string  `json:"set_date"`
	SetTitle       *string  `json:"set_title"`
	AddKeywords    []string `json:"add_keywords"`
	RemoveKeywords []string `json:"remove_keywords"`
}

type bulkEditResponse struct {
	Updated int                        `json:"updated"`
	Skipped int                        `json:"skipped"`
	Photos  []database.DownloadedPhoto `json:"photos"`
}

func (s *Server) handleUpdatePieceMetadata(w http.ResponseWriter, r *http.Request) {
	photo, ok := s.photoFromRoute(w, r)
	if !ok {
		return
	}
	var req pieceMetadataPatch
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid metadata JSON")
		return
	}
	update := database.PhotoMetadataUpdate{}
	if req.Title != nil {
		title := strings.TrimSpace(*req.Title)
		update.Title = &title
		update.LockFields = append(update.LockFields, "title")
	}
	if req.Artist != nil {
		artist := strings.TrimSpace(*req.Artist)
		update.Artist = &artist
		update.LockFields = append(update.LockFields, "artist")
	}
	if req.Date != nil {
		uploadDate, err := parseEditorDateRaw(*req.Date)
		if err != nil {
			s.writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		update.UploadDate = &uploadDate
		update.LockFields = append(update.LockFields, "date")
	}
	if req.Keywords != nil {
		keywords := database.NormalizeManualKeywords(*req.Keywords)
		update.Keywords = &keywords
		update.LockFields = append(update.LockFields, "keywords")
	}
	updated, _, err := s.db.UpdatePhotoMetadata(photo.ID, update)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			s.writeError(w, http.StatusNotFound, "Photo not found")
			return
		}
		s.logger.Error().Err(err).Uint64("id", photo.ID).Msg("Failed to update piece metadata")
		s.writeError(w, http.StatusInternalServerError, "Failed to update piece metadata")
		return
	}
	s.invalidatePhotoMetadataCaches(r.Context(), []uint64{photo.ID})
	s.writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleBulkEditCatalog(w http.ResponseWriter, r *http.Request) {
	var req bulkEditRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid bulk edit JSON")
		return
	}
	if len(req.IDs) == 0 {
		s.writeError(w, http.StatusBadRequest, "ids are required")
		return
	}
	if len(req.IDs) > maxBulkEditIDs {
		s.writeError(w, http.StatusBadRequest, "Too many ids for one bulk edit")
		return
	}
	edit := database.BulkMetadataEdit{IDs: req.IDs}
	if req.SetTitle != nil {
		title := strings.TrimSpace(*req.SetTitle)
		edit.SetTitle = &title
	}
	if req.SetArtist != nil {
		artist := strings.TrimSpace(*req.SetArtist)
		edit.SetArtist = &artist
	}
	if req.SetDate != nil {
		uploadDate, err := parseEditorDateString(*req.SetDate)
		if err != nil {
			s.writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		edit.SetUploadDate = &uploadDate
	}
	if req.AddKeywords != nil {
		edit.AddKeywords = database.NormalizeManualKeywords(req.AddKeywords)
	}
	if req.RemoveKeywords != nil {
		edit.RemoveKeywords = database.NormalizeManualKeywords(req.RemoveKeywords)
	}
	if edit.SetTitle == nil && edit.SetArtist == nil && edit.SetUploadDate == nil && edit.AddKeywords == nil && edit.RemoveKeywords == nil {
		s.writeError(w, http.StatusBadRequest, "No bulk edit operations provided")
		return
	}
	result, err := s.db.BulkEditPhotoMetadata(edit)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to bulk edit catalog metadata")
		s.writeError(w, http.StatusInternalServerError, "Failed to bulk edit catalog metadata")
		return
	}
	s.invalidatePhotoMetadataCaches(r.Context(), resultIDs(result.Photos))
	s.writeJSON(w, http.StatusOK, bulkEditResponse{Updated: result.Updated, Skipped: result.Skipped, Photos: result.Photos})
}

func parseEditorDateRaw(raw json.RawMessage) (*time.Time, error) {
	if string(raw) == "null" {
		return nil, nil
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, errors.New("date must be a string or null")
	}
	return parseEditorDateString(value)
}

func parseEditorDateString(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02", "2006-01", "2006"} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			utc := parsed.UTC()
			return &utc, nil
		}
	}
	return nil, errors.New("Invalid date")
}

func (s *Server) invalidatePhotoMetadataCaches(ctx context.Context, ids []uint64) {
	for _, id := range ids {
		_ = s.cache.Delete(ctx, okfcache.PhotoKey(id))
	}
	s.statsCache.Invalidate()
	_ = s.cache.BumpEpoch(ctx)
}

func resultIDs(photos []database.DownloadedPhoto) []uint64 {
	ids := make([]uint64, 0, len(photos))
	for _, photo := range photos {
		ids = append(ids, photo.ID)
	}
	return ids
}
