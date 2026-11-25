package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"ok-folio/internal/database"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
)

type foliosResponse struct {
	Folios []database.Folio `json:"folios"`
}

type folioRequest struct {
	Name         *string `json:"name"`
	CoverPhotoID *uint64 `json:"cover_photo_id"`
}

type folioPieceRequest struct {
	PhotoID uint64 `json:"photo_id"`
}

const maxFolioRequestBytes = 1 << 20

const (
	defaultFolioPiecesLimit = 100
	maxFolioPiecesLimit     = 200
)

func (s *Server) handleListFolios(w http.ResponseWriter, r *http.Request) {
	folios, err := s.db.ListFolios()
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "Failed to fetch folios")
		return
	}
	s.writeJSON(w, http.StatusOK, foliosResponse{Folios: folios})
}

func (s *Server) handleCreateFolio(w http.ResponseWriter, r *http.Request) {
	input, ok := s.readFolioRequest(w, r)
	if !ok {
		return
	}
	if err := validateFolioCoverPhotoID(input.CoverPhotoID); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	folio := database.Folio{
		CoverPhotoID: input.CoverPhotoID,
	}
	if input.Name != nil {
		folio.Name = *input.Name
	}
	created, err := s.db.CreateFolio(folio)
	if isFolioNameRequired(err) {
		s.writeError(w, http.StatusBadRequest, "Folio name is required")
		return
	}
	if database.IsUniqueViolation(err) {
		s.writeError(w, http.StatusConflict, "Folio name already exists")
		return
	}
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "Failed to create folio")
		return
	}
	s.writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleGetFolio(w http.ResponseWriter, r *http.Request) {
	id, err := parseFolioID(r)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid folio ID")
		return
	}
	folio, err := s.db.GetFolio(id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		s.writeError(w, http.StatusNotFound, "Folio not found")
		return
	}
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "Failed to fetch folio")
		return
	}
	s.writeJSON(w, http.StatusOK, folio)
}

func (s *Server) handleUpdateFolio(w http.ResponseWriter, r *http.Request) {
	id, err := parseFolioID(r)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid folio ID")
		return
	}
	input, coverProvided, ok := s.readFolioPatchRequest(w, r)
	if !ok {
		return
	}
	if input.Name == nil && !coverProvided {
		s.writeError(w, http.StatusBadRequest, "No folio update fields provided")
		return
	}
	if err := validateFolioCoverPhotoID(input.CoverPhotoID); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	updated, err := s.db.UpdateFolio(id, database.FolioUpdates{
		Name:          input.Name,
		CoverProvided: coverProvided,
		CoverPhotoID:  input.CoverPhotoID,
	})
	if isFolioNameRequired(err) {
		s.writeError(w, http.StatusBadRequest, "Folio name is required")
		return
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		s.writeError(w, http.StatusNotFound, "Folio not found")
		return
	}
	if database.IsUniqueViolation(err) {
		s.writeError(w, http.StatusConflict, "Folio name already exists")
		return
	}
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "Failed to update folio")
		return
	}
	s.writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleDeleteFolio(w http.ResponseWriter, r *http.Request) {
	id, err := parseFolioID(r)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid folio ID")
		return
	}
	err = s.db.DeleteFolio(id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		s.writeError(w, http.StatusNotFound, "Folio not found")
		return
	}
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "Failed to delete folio")
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (s *Server) handleAddFolioPiece(w http.ResponseWriter, r *http.Request) {
	folioID, err := parseFolioID(r)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid folio ID")
		return
	}
	input, ok := s.readFolioPieceRequest(w, r)
	if !ok {
		return
	}
	if input.PhotoID == 0 {
		s.writeError(w, http.StatusBadRequest, "Folio piece photo_id must be greater than zero")
		return
	}
	if _, err := s.db.GetFolio(folioID); errors.Is(err, gorm.ErrRecordNotFound) {
		s.writeError(w, http.StatusNotFound, "Folio not found")
		return
	} else if err != nil {
		s.writeError(w, http.StatusInternalServerError, "Failed to fetch folio")
		return
	}
	photo, err := s.db.GetPhotoByID(input.PhotoID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		s.writeError(w, http.StatusNotFound, "Photo not found")
		return
	} else if err != nil {
		s.writeError(w, http.StatusInternalServerError, "Failed to fetch photo")
		return
	}
	if photo.Status != "downloaded" {
		s.writeError(w, http.StatusNotFound, "Photo not found")
		return
	}
	added, err := s.db.AddPieceToFolioIfMissing(folioID, input.PhotoID)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "Failed to add piece to folio")
		return
	}
	if !added {
		s.writeJSON(w, http.StatusOK, map[string]bool{"added": false, "duplicate": true})
		return
	}
	s.writeJSON(w, http.StatusCreated, map[string]bool{"added": true})
}

func (s *Server) handleRemoveFolioPiece(w http.ResponseWriter, r *http.Request) {
	folioID, err := parseFolioID(r)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid folio ID")
		return
	}
	photoID, err := parseFolioPiecePhotoID(r)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid photo ID")
		return
	}
	if _, err := s.db.GetFolio(folioID); errors.Is(err, gorm.ErrRecordNotFound) {
		s.writeError(w, http.StatusNotFound, "Folio not found")
		return
	} else if err != nil {
		s.writeError(w, http.StatusInternalServerError, "Failed to fetch folio")
		return
	}
	if err := s.db.RemovePieceFromFolio(folioID, photoID); errors.Is(err, gorm.ErrRecordNotFound) {
		s.writeError(w, http.StatusNotFound, "Piece not in folio")
		return
	} else if err != nil {
		s.writeError(w, http.StatusInternalServerError, "Failed to remove piece from folio")
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]bool{"removed": true})
}

func (s *Server) handleListFolioPieces(w http.ResponseWriter, r *http.Request) {
	folioID, err := parseFolioID(r)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid folio ID")
		return
	}
	if _, err := s.db.GetFolio(folioID); errors.Is(err, gorm.ErrRecordNotFound) {
		s.writeError(w, http.StatusNotFound, "Folio not found")
		return
	} else if err != nil {
		s.writeError(w, http.StatusInternalServerError, "Failed to fetch folio")
		return
	}

	limit := defaultFolioPiecesLimit
	if value := r.URL.Query().Get("limit"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 && parsed <= maxFolioPiecesLimit {
			limit = parsed
		}
	}

	offset := 0
	if value := r.URL.Query().Get("offset"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	photos, total, err := s.db.ListFolioPieces(folioID, limit, offset)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "Failed to fetch folio pieces")
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"photos": photos,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

func (s *Server) handleListPhotoFolios(w http.ResponseWriter, r *http.Request) {
	photo, ok := s.photoFromRoute(w, r)
	if !ok {
		return
	}
	folios, err := s.db.ListFoliosForPhoto(photo.ID)
	if err != nil {
		s.logger.Error().Err(err).Uint64("id", photo.ID).Msg("Failed to fetch folios for photo")
		s.writeError(w, http.StatusInternalServerError, "Failed to fetch folios for photo")
		return
	}
	s.writeJSON(w, http.StatusOK, foliosResponse{Folios: folios})
}

func (s *Server) readFolioRequest(w http.ResponseWriter, r *http.Request) (folioRequest, bool) {
	var input folioRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxFolioRequestBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid folio JSON")
		return folioRequest{}, false
	}
	return input, true
}

func (s *Server) readFolioPieceRequest(w http.ResponseWriter, r *http.Request) (folioPieceRequest, bool) {
	var input folioPieceRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxFolioRequestBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid folio piece JSON")
		return folioPieceRequest{}, false
	}
	return input, true
}

func (s *Server) readFolioPatchRequest(w http.ResponseWriter, r *http.Request) (folioRequest, bool, bool) {
	var raw map[string]json.RawMessage
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxFolioRequestBytes))
	if err := decoder.Decode(&raw); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid folio JSON")
		return folioRequest{}, false, false
	}

	var input folioRequest
	for field, value := range raw {
		switch field {
		case "name":
			if err := json.Unmarshal(value, &input.Name); err != nil {
				s.writeError(w, http.StatusBadRequest, "Invalid folio JSON")
				return folioRequest{}, false, false
			}
		case "cover_photo_id":
			if err := json.Unmarshal(value, &input.CoverPhotoID); err != nil {
				s.writeError(w, http.StatusBadRequest, "Invalid folio JSON")
				return folioRequest{}, false, false
			}
		default:
			s.writeError(w, http.StatusBadRequest, "Invalid folio JSON")
			return folioRequest{}, false, false
		}
	}

	_, coverProvided := raw["cover_photo_id"]
	return input, coverProvided, true
}

func parseFolioID(r *http.Request) (uint64, error) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id == 0 {
		return 0, strconv.ErrSyntax
	}
	return id, nil
}

func parseFolioPiecePhotoID(r *http.Request) (uint64, error) {
	id, err := strconv.ParseUint(chi.URLParam(r, "photoId"), 10, 64)
	if err != nil || id == 0 {
		return 0, strconv.ErrSyntax
	}
	return id, nil
}

func isFolioNameRequired(err error) bool {
	return err != nil && err.Error() == "folio name is required"
}

func validateFolioCoverPhotoID(id *uint64) error {
	if id != nil && *id == 0 {
		return errors.New("Folio cover_photo_id must be greater than zero")
	}
	return nil
}
