package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"ok-folio/internal/database"
	"ok-folio/internal/provider"
	"ok-folio/internal/provider/telegram"
	"ok-folio/internal/provider/webgallery"
	"ok-folio/pkg/retry"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
)

const (
	connectorSourcePreviewTimeout      = 10 * time.Second
	connectorSourcePreviewDefaultLimit = 6
	connectorSourcePreviewMaxLimit     = 6
)

var connectorSourcePreviewAllowPrivateHosts bool

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

type connectorSourcePreviewRequest struct {
	webgallery.WebGalleryConfig
	Config *json.RawMessage `json:"config,omitempty"`
	Page   int              `json:"page,omitempty"`
	Limit  int              `json:"limit,omitempty"`
	Cursor string           `json:"cursor,omitempty"`
}

type connectorSourcePreviewResponse struct {
	Provider   string                         `json:"provider"`
	Page       int                            `json:"page"`
	ItemsFound int                            `json:"items_found"`
	HasNext    bool                           `json:"has_next"`
	NextPage   int                            `json:"next_page,omitempty"`
	NextCursor string                         `json:"next_cursor,omitempty"`
	Sample     []connectorSourcePreviewSample `json:"sample"`
}

type connectorSourcePreviewSample struct {
	SourceURL string `json:"source_url"`
	ImageURL  string `json:"image_url"`
	Title     string `json:"title,omitempty"`
	Artist    string `json:"artist,omitempty"`
	Date      string `json:"date,omitempty"`
}

func (s *Server) handleListConnectorSources(w http.ResponseWriter, r *http.Request) {
	sources, err := s.db.ListConnectorSources(r.URL.Query().Get("type"))
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "Failed to fetch connector sources")
		return
	}
	s.writeJSON(w, http.StatusOK, connectorSourcesResponse{Sources: sources})
}

func (s *Server) handlePreviewConnectorSource(w http.ResponseWriter, r *http.Request) {
	var input connectorSourcePreviewRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid connector source preview JSON")
		return
	}

	cfg, err := previewWebGalleryConfig(input)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := webgallery.ValidateConfig(cfg); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := validatePreviewURL(cfg.ListURL); err != nil {
		s.writeProviderError(w, err)
		return
	}
	cursor := strings.TrimSpace(input.Cursor)
	if cursor != "" {
		if err := validatePreviewURL(cursor); err != nil {
			s.writeProviderError(w, err)
			return
		}
	}

	limit := input.Limit
	if limit <= 0 {
		limit = connectorSourcePreviewDefaultLimit
	}
	if limit > connectorSourcePreviewMaxLimit {
		limit = connectorSourcePreviewMaxLimit
	}
	if input.Page < 0 {
		s.writeError(w, http.StatusBadRequest, "preview page must be zero or greater")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), connectorSourcePreviewTimeout)
	defer cancel()

	client := newPreviewHTTPClient(connectorSourcePreviewTimeout)
	cfg.Retry = webgallery.SourceRetryConfig{}
	connector := webgallery.New(webgallery.Config{
		UserAgent:        previewUserAgent(s, cfg),
		RateLimitBackoff: previewRateLimitBackoff(s, cfg),
		Gallery:          cfg,
		Retry:            retry.Config{MaxAttempts: 1},
	}, client, s.logger)

	result, err := connector.DiscoverPage(ctx, provider.PageRequest{Page: input.Page, Cursor: cursor})
	if err != nil {
		s.writeProviderError(w, err)
		return
	}
	if len(result.Items) == 0 {
		s.writeProviderError(w, &provider.ProviderError{
			ProviderID: webgallery.ProviderID,
			Kind:       provider.ErrorKindParse,
			Err:        fmt.Errorf("item_link selector %q matched 0 items on list page", cfg.Selectors.ItemLink),
		})
		return
	}

	sampleCount := limit
	if len(result.Items) < sampleCount {
		sampleCount = len(result.Items)
	}
	samples := make([]connectorSourcePreviewSample, 0, sampleCount)
	for _, item := range result.Items[:sampleCount] {
		resolved, err := connector.ResolveMedia(ctx, item)
		if err != nil {
			s.writeProviderError(w, err)
			return
		}
		samples = append(samples, previewSample(*resolved))
	}

	s.writeJSON(w, http.StatusOK, connectorSourcePreviewResponse{
		Provider:   webgallery.ProviderID,
		Page:       result.Pagination.Page,
		ItemsFound: len(result.Items),
		HasNext:    result.Pagination.HasNext,
		NextPage:   result.Pagination.NextPage,
		NextCursor: result.Pagination.NextCursor,
		Sample:     samples,
	})
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

func previewWebGalleryConfig(input connectorSourcePreviewRequest) (webgallery.WebGalleryConfig, error) {
	if input.Config != nil {
		return webgallery.ParseConfig(*input.Config)
	}
	data, err := json.Marshal(input.WebGalleryConfig)
	if err != nil {
		return webgallery.WebGalleryConfig{}, err
	}
	return webgallery.ParseConfig(data)
}

func previewUserAgent(s *Server, cfg webgallery.WebGalleryConfig) string {
	if strings.TrimSpace(cfg.UserAgent) != "" || s == nil || s.cfg == nil {
		return ""
	}
	return s.cfg.Download.UserAgent
}

func previewRateLimitBackoff(s *Server, cfg webgallery.WebGalleryConfig) time.Duration {
	if strings.TrimSpace(cfg.RateLimit.Backoff) != "" || s == nil || s.cfg == nil {
		return 0
	}
	return s.cfg.Download.RateLimitBackoff
}

func newPreviewHTTPClient(timeout time.Duration) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	baseDialContext := transport.DialContext
	if baseDialContext == nil {
		baseDialContext = (&net.Dialer{}).DialContext
	}
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			host = address
		}
		if err := validatePreviewHost(ctx, host); err != nil {
			return nil, err
		}
		return baseDialContext(ctx, network, address)
	}
	return &http.Client{Timeout: timeout, Transport: transport}
}

func validatePreviewURL(raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return &provider.ProviderError{
			ProviderID: webgallery.ProviderID,
			Kind:       provider.ErrorKindParse,
			Err:        fmt.Errorf("preview URL must be an absolute URL"),
		}
	}
	switch parsed.Scheme {
	case "http", "https":
	default:
		return &provider.ProviderError{
			ProviderID: webgallery.ProviderID,
			Kind:       provider.ErrorKindPermission,
			Err:        fmt.Errorf("preview URL scheme %q is not allowed", parsed.Scheme),
		}
	}
	if err := validatePreviewHost(context.Background(), parsed.Hostname()); err != nil {
		return err
	}
	return nil
}

func validatePreviewHost(ctx context.Context, host string) error {
	if connectorSourcePreviewAllowPrivateHosts {
		return nil
	}
	if strings.TrimSpace(host) == "" {
		return &provider.ProviderError{
			ProviderID: webgallery.ProviderID,
			Kind:       provider.ErrorKindParse,
			Err:        fmt.Errorf("preview URL host is required"),
		}
	}
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return &provider.ProviderError{
			ProviderID: webgallery.ProviderID,
			Kind:       provider.ErrorKindTemporary,
			Err:        fmt.Errorf("preview URL host could not be resolved"),
		}
	}
	if len(ips) == 0 {
		return &provider.ProviderError{
			ProviderID: webgallery.ProviderID,
			Kind:       provider.ErrorKindNotFound,
			Err:        fmt.Errorf("preview URL host has no addresses"),
		}
	}
	for _, resolved := range ips {
		if isBlockedPreviewIP(resolved.IP) {
			return &provider.ProviderError{
				ProviderID: webgallery.ProviderID,
				Kind:       provider.ErrorKindPermission,
				Err:        fmt.Errorf("preview URL host is not allowed"),
			}
		}
	}
	return nil
}

func isBlockedPreviewIP(ip net.IP) bool {
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified() ||
		ip.IsMulticast()
}

func previewSample(item provider.DiscoveredMedia) connectorSourcePreviewSample {
	sample := connectorSourcePreviewSample{
		SourceURL: item.Source.URL,
		ImageURL:  item.Media.URL,
		Title:     item.Title,
		Artist:    item.Artist,
	}
	if !item.PublishedAt.IsZero() {
		sample.Date = item.PublishedAt.Format(time.RFC3339)
	}
	return sample
}

func (s *Server) writeProviderError(w http.ResponseWriter, err error) {
	var providerErr *provider.ProviderError
	if !errors.As(err, &providerErr) {
		providerErr = &provider.ProviderError{
			ProviderID: webgallery.ProviderID,
			Kind:       provider.ErrorKindTemporary,
			Err:        err,
		}
	}
	status := http.StatusUnprocessableEntity
	switch providerErr.Kind {
	case provider.ErrorKindPermission:
		status = http.StatusForbidden
	case provider.ErrorKindNotFound:
		status = http.StatusNotFound
	case provider.ErrorKindTemporary:
		status = http.StatusServiceUnavailable
	case provider.ErrorKindRateLimit:
		status = http.StatusTooManyRequests
	case provider.ErrorKindParse, provider.ErrorKindMissingMedia:
		status = http.StatusUnprocessableEntity
	}
	body := map[string]interface{}{
		"error":    providerErr.Error(),
		"kind":     providerErr.Kind,
		"provider": providerErr.ProviderID,
	}
	if providerErr.RetryAfter > 0 {
		body["retry_after_seconds"] = providerErr.RetryAfter.Seconds()
	}
	s.writeJSON(w, status, body)
}
