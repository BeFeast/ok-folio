package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"ok-folio/internal/database"
	"ok-folio/internal/provider/telegram"
	"ok-folio/internal/provider/webgallery"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
)

type connectorSourcesResponse struct {
	Sources []database.ConnectorSource `json:"sources"`
}

type connectorSourceRequest struct {
	Type    string           `json:"type"`
	ChatID  string           `json:"chat_id"`
	Label   *string          `json:"label"`
	Config  *json.RawMessage `json:"config"`
	Enabled *bool            `json:"enabled"`
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
	if err := s.validateConnectorSource(input, true); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	source, err := s.db.CreateConnectorSource(database.ConnectorSource{
		Type:    input.Type,
		ChatID:  connectorSourceKey(input),
		Label:   connectorSourceLabel(input),
		Config:  connectorSourceConfig(input),
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
	if err := s.validateConnectorSource(input, false); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	source, err := s.db.UpdateConnectorSource(id, database.ConnectorSourceUpdates{
		ChatID:  optionalNonEmptyString(input.ChatID),
		Label:   input.Label,
		Config:  optionalConnectorSourceConfig(input.Config),
		Enabled: input.Enabled,
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
	if input.Label != nil {
		label := strings.TrimSpace(*input.Label)
		input.Label = &label
	}
	return input, true
}

func (s *Server) validateConnectorSource(input connectorSourceRequest, requireSource bool) error {
	if input.Type != "" && input.Type != telegram.ProviderID && input.Type != webgallery.ProviderID {
		return fmt.Errorf("unsupported connector source type")
	}
	if input.Type == "" && input.ChatID == "" && input.Config == nil {
		if requireSource {
			return fmt.Errorf("connector source type is required")
		}
		return nil
	}
	if input.Type == "" {
		return fmt.Errorf("connector source type is required")
	}
	switch input.Type {
	case telegram.ProviderID:
		if input.ChatID == "" {
			return fmt.Errorf("Telegram chat ID is required")
		}
		if _, err := strconv.ParseInt(input.ChatID, 10, 64); err != nil {
			return fmt.Errorf("Telegram chat ID must be a numeric ID")
		}
	case webgallery.ProviderID:
		if input.Config == nil {
			if requireSource {
				return fmt.Errorf("webgallery config is required")
			}
			return nil
		}
		if _, err := webgallery.ParseConfig(*input.Config); err != nil {
			return err
		}
	}
	return nil
}

func connectorSourceLabel(input connectorSourceRequest) string {
	if input.Label == nil {
		return ""
	}
	return *input.Label
}

func connectorSourceConfig(input connectorSourceRequest) database.JSONConfig {
	if input.Config == nil {
		return nil
	}
	return database.JSONConfig(*input.Config)
}

func optionalConnectorSourceConfig(input *json.RawMessage) *database.JSONConfig {
	if input == nil {
		return nil
	}
	cfg := database.JSONConfig(*input)
	return &cfg
}

func connectorSourceKey(input connectorSourceRequest) string {
	if input.ChatID != "" || input.Type != webgallery.ProviderID || input.Config == nil {
		return input.ChatID
	}
	cfg, err := webgallery.ParseConfig(*input.Config)
	if err != nil {
		return input.ChatID
	}
	parsed, err := url.Parse(cfg.ListURL)
	if err != nil {
		return input.ChatID
	}
	keySource := parsed.Host + parsed.EscapedPath()
	if parsed.RawQuery != "" {
		keySource += "?" + parsed.RawQuery
	}
	key := strings.Trim(keySource, "/")
	key = strings.NewReplacer("/", "-", ":", "-", "?", "-", "&", "-", "=", "-").Replace(key)
	if key == "" {
		return webgallery.ProviderID
	}
	return key
}

func optionalNonEmptyString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
