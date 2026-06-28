package api

import (
	"encoding/json"
	"errors"
	"io"
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

func (s *Server) readFolioRequest(w http.ResponseWriter, r *http.Request) (folioRequest, bool) {
	var input folioRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid folio JSON")
		return folioRequest{}, false
	}
	return input, true
}

func (s *Server) readFolioPatchRequest(w http.ResponseWriter, r *http.Request) (folioRequest, bool, bool) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid folio JSON")
		return folioRequest{}, false, false
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid folio JSON")
		return folioRequest{}, false, false
	}
	_, coverProvided := raw["cover_photo_id"]
	var input folioRequest
	if err := json.Unmarshal(body, &input); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid folio JSON")
		return folioRequest{}, false, false
	}
	return input, coverProvided, true
}

func parseFolioID(r *http.Request) (uint64, error) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id == 0 {
		return 0, strconv.ErrSyntax
	}
	return id, nil
}

func isFolioNameRequired(err error) bool {
	return err != nil && err.Error() == "folio name is required"
}
