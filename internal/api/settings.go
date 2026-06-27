package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"ok-folio/internal/database"
	"ok-folio/internal/provider/telegram"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
)

type connectorSourcesResponse struct {
	Sources []database.ConnectorSource `json:"sources"`
}

type connectorSourceRequest struct {
	Type    string `json:"type"`
	ChatID  string `json:"chat_id"`
	Label   string `json:"label"`
	Enabled *bool  `json:"enabled"`
}

func (s *Server) handleListConnectorSources(w http.ResponseWriter, r *http.Request) {
	sources, err := s.db.ListConnectorSources(r.URL.Query().Get("type"))
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "Failed to fetch connector sources")
		return
	}
	s.writeJSON(w, http.StatusOK, connectorSourcesResponse{Sources: sources})
}

func (s *Server) handleCreateConnectorSource(w http.ResponseWriter, r *http.Request) {
	input, ok := s.readConnectorSourceRequest(w, r)
	if !ok {
		return
	}
	if err := s.validateConnectorSource(input); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	source, err := s.db.CreateConnectorSource(database.ConnectorSource{
		Type:    input.Type,
		ChatID:  input.ChatID,
		Label:   input.Label,
		Enabled: enabled,
	})
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "Failed to create connector source")
		return
	}
	s.writeJSON(w, http.StatusCreated, source)
}

func (s *Server) handleUpdateConnectorSource(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id == 0 {
		s.writeError(w, http.StatusBadRequest, "Invalid connector source ID")
		return
	}
	input, ok := s.readConnectorSourceRequest(w, r)
	if !ok {
		return
	}
	if err := s.validateConnectorSource(input); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}

	source, err := s.db.UpdateConnectorSource(id, database.ConnectorSource{
		ChatID:  input.ChatID,
		Label:   input.Label,
		Enabled: enabled,
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		s.writeError(w, http.StatusNotFound, "Connector source not found")
		return
	}
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "Failed to update connector source")
		return
	}
	s.writeJSON(w, http.StatusOK, source)
}

func (s *Server) handleDeleteConnectorSource(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id == 0 {
		s.writeError(w, http.StatusBadRequest, "Invalid connector source ID")
		return
	}
	err = s.db.DeleteConnectorSource(id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		s.writeError(w, http.StatusNotFound, "Connector source not found")
		return
	}
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "Failed to delete connector source")
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (s *Server) readConnectorSourceRequest(w http.ResponseWriter, r *http.Request) (connectorSourceRequest, bool) {
	var input connectorSourceRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid connector source JSON")
		return connectorSourceRequest{}, false
	}
	input.Type = strings.TrimSpace(input.Type)
	input.ChatID = strings.TrimSpace(input.ChatID)
	input.Label = strings.TrimSpace(input.Label)
	return input, true
}

func (s *Server) validateConnectorSource(input connectorSourceRequest) error {
	if input.Type != telegram.ProviderID {
		return fmt.Errorf("only Telegram connector sources are supported")
	}
	if input.ChatID == "" {
		return fmt.Errorf("Telegram chat ID is required")
	}
	if _, err := strconv.ParseInt(input.ChatID, 10, 64); err != nil {
		return fmt.Errorf("Telegram chat ID must be a numeric ID")
	}
	return nil
}
