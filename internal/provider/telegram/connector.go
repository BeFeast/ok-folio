package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"ok-folio/internal/provider"
	"ok-folio/pkg/retry"

	"github.com/rs/zerolog"
)

const (
	ProviderID            = "telegram"
	defaultAPIBaseURL     = "https://api.telegram.org"
	defaultFileBaseURL    = "https://api.telegram.org/file"
	defaultLimit          = 100
	defaultRateLimitDelay = 60 * time.Second
)

type Config struct {
	BotToken         string
	BaseURL          string
	FileBaseURL      string
	ChatID           string
	DisplayName      string
	Limit            int
	RateLimitBackoff time.Duration
	Retry            retry.Config
}

type Connector struct {
	cfg    Config
	client apiClient
}

type apiClient interface {
	GetUpdates(ctx context.Context, offset int, limit int) ([]Update, error)
	GetFile(ctx context.Context, fileID string) (*File, error)
	FileDownloadURL(filePath string) (string, error)
}

func New(cfg Config, client *http.Client, _ zerolog.Logger) *Connector {
	if client == nil {
		client = http.DefaultClient
	}
	if cfg.Retry.MaxAttempts == 0 {
		cfg.Retry.MaxAttempts = 1
	}
	return &Connector{
		cfg:    cfg,
		client: newHTTPClient(cfg, client),
	}
}

func NewWithClient(cfg Config, client apiClient, _ zerolog.Logger) *Connector {
	if cfg.Retry.MaxAttempts == 0 {
		cfg.Retry.MaxAttempts = 1
	}
	return &Connector{cfg: cfg, client: client}
}

func (c *Connector) Provider() provider.Source {
	displayName := c.cfg.DisplayName
	if displayName == "" {
		displayName = "Telegram"
	}
	return provider.Source{
		ID:          ProviderID,
		DisplayName: displayName,
		BaseURL:     strings.TrimRight(c.cfg.BaseURL, "/"),
		Scope:       c.cfg.ChatID,
	}
}

func (c *Connector) DiscoverPage(ctx context.Context, req provider.PageRequest) (*provider.PageResult, error) {
	offset, err := cursorOffset(req.Cursor)
	if err != nil {
		return nil, err
	}

	limit := c.cfg.Limit
	if limit <= 0 {
		limit = defaultLimit
	}

	updates, err := retry.DoWithValue(ctx, c.cfg.Retry, func() ([]Update, error) {
		return c.client.GetUpdates(ctx, offset, limit)
	})
	if err != nil {
		return nil, err
	}

	var (
		items       []provider.DiscoveredMedia
		maxUpdateID = offset - 1
	)
	for _, update := range updates {
		if update.UpdateID > maxUpdateID {
			maxUpdateID = update.UpdateID
		}

		message := update.ProviderMessage()
		if message == nil {
			continue
		}
		if c.cfg.ChatID != "" && message.Chat.IDString() != c.cfg.ChatID {
			continue
		}

		item, ok := discoveredMedia(*message)
		if ok {
			items = append(items, item)
		}
	}

	nextCursor := ""
	if maxUpdateID >= offset {
		nextCursor = strconv.Itoa(maxUpdateID + 1)
	}

	return &provider.PageResult{
		Items: items,
		Pagination: provider.Pagination{
			Page:       req.Page,
			NextCursor: nextCursor,
			HasNext:    nextCursor != "",
		},
	}, nil
}

func (c *Connector) ResolveMedia(ctx context.Context, item provider.DiscoveredMedia) (*provider.DiscoveredMedia, error) {
	if item.Media.ExternalID == "" {
		return nil, &provider.ProviderError{
			ProviderID: ProviderID,
			Kind:       provider.ErrorKindMissingMedia,
			Err:        errors.New("telegram media file id is required"),
		}
	}

	file, err := retry.DoWithValue(ctx, c.cfg.Retry, func() (*File, error) {
		return c.client.GetFile(ctx, item.Media.ExternalID)
	})
	if err != nil {
		return nil, err
	}
	if file == nil || file.FilePath == "" {
		return nil, &provider.ProviderError{
			ProviderID: ProviderID,
			Kind:       provider.ErrorKindMissingMedia,
			Err:        fmt.Errorf("telegram file path not found for %s", item.Media.ExternalID),
		}
	}

	downloadURL, err := c.client.FileDownloadURL(file.FilePath)
	if err != nil {
		return nil, err
	}

	out := item
	out.ProviderID = ProviderID
	out.Media.URL = downloadURL
	if out.Media.FileName == "" {
		out.Media.FileName = path.Base(file.FilePath)
	}
	if out.DedupeKey.ProviderID == "" {
		out.DedupeKey.ProviderID = ProviderID
	}
	if out.DedupeKey.Value == "" {
		out.DedupeKey.Value = out.Source.ExternalID + ":" + item.Media.ExternalID
	}
	return &out, nil
}

func cursorOffset(cursor string) (int, error) {
	if cursor == "" {
		return 0, nil
	}
	offset, err := strconv.Atoi(cursor)
	if err != nil || offset < 0 {
		return 0, &provider.ProviderError{
			ProviderID: ProviderID,
			Kind:       provider.ErrorKindParse,
			Err:        fmt.Errorf("invalid telegram cursor %q", cursor),
		}
	}
	return offset, nil
}

func discoveredMedia(message Message) (provider.DiscoveredMedia, bool) {
	ref, ok := message.MediaRef()
	if !ok {
		return provider.DiscoveredMedia{}, false
	}

	source := message.SourceRef()
	externalID := fallbackExternalID(message)
	if source.ChatID != "" && source.MessageID > 0 {
		externalID = fmt.Sprintf("%s:%d", source.ChatID, source.MessageID)
	}
	collectionID := source.ChatID
	if collectionID == "" {
		collectionID = message.Chat.IDString()
	}
	collectionName := source.ChatName
	if collectionName == "" {
		collectionName = message.Chat.DisplayName()
	}
	itemID := strconv.Itoa(message.MessageID)
	if source.MessageID > 0 {
		itemID = strconv.Itoa(source.MessageID)
	}
	title := message.Caption
	if title == "" {
		title = ref.FileName
	}
	artist := collectionName
	if source.Author != "" {
		artist = source.Author
	}

	return provider.DiscoveredMedia{
		ProviderID: ProviderID,
		DedupeKey: provider.DedupeKey{
			ProviderID: ProviderID,
			Value:      externalID,
		},
		Source: provider.SourceMetadata{
			URL:            source.URL,
			ExternalID:     externalID,
			CollectionID:   collectionID,
			CollectionName: collectionName,
			ItemID:         itemID,
		},
		Media: provider.MediaMetadata{
			MIMEType:   ref.MIMEType,
			FileName:   ref.FileName,
			ExternalID: ref.FileID,
		},
		Title:       title,
		Artist:      artist,
		PublishedAt: time.Unix(message.Date, 0).UTC(),
	}, true
}

func fallbackExternalID(message Message) string {
	return fmt.Sprintf("%s:%d", message.Chat.IDString(), message.MessageID)
}

type httpAPIClient struct {
	cfg    Config
	client *http.Client
}

func newHTTPClient(cfg Config, client *http.Client) *httpAPIClient {
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultAPIBaseURL
	}
	if cfg.FileBaseURL == "" {
		cfg.FileBaseURL = defaultFileBaseURL
	}
	return &httpAPIClient{cfg: cfg, client: client}
}

func (c *httpAPIClient) GetUpdates(ctx context.Context, offset int, limit int) ([]Update, error) {
	values := url.Values{}
	values.Set("offset", strconv.Itoa(offset))
	values.Set("limit", strconv.Itoa(limit))

	var resp telegramResponse[[]Update]
	if err := c.get(ctx, "getUpdates", values, &resp); err != nil {
		return nil, err
	}
	return resp.Result, nil
}

func (c *httpAPIClient) GetFile(ctx context.Context, fileID string) (*File, error) {
	values := url.Values{}
	values.Set("file_id", fileID)

	var resp telegramResponse[File]
	if err := c.get(ctx, "getFile", values, &resp); err != nil {
		return nil, err
	}
	return &resp.Result, nil
}

func (c *httpAPIClient) FileDownloadURL(filePath string) (string, error) {
	base := strings.TrimRight(c.cfg.FileBaseURL, "/")
	if c.cfg.BotToken == "" {
		return "", &provider.ProviderError{
			ProviderID: ProviderID,
			Kind:       provider.ErrorKindPermission,
			Err:        errors.New("telegram bot token is required"),
		}
	}
	return fmt.Sprintf("%s/bot%s/%s", base, c.cfg.BotToken, strings.TrimLeft(filePath, "/")), nil
}

func (c *httpAPIClient) get(ctx context.Context, method string, values url.Values, target any) error {
	if c.cfg.BotToken == "" {
		return &provider.ProviderError{
			ProviderID: ProviderID,
			Kind:       provider.ErrorKindPermission,
			Err:        errors.New("telegram bot token is required"),
		}
	}

	endpoint := fmt.Sprintf("%s/bot%s/%s", strings.TrimRight(c.cfg.BaseURL, "/"), c.cfg.BotToken, method)
	if len(values) > 0 {
		endpoint += "?" + values.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return &provider.ProviderError{ProviderID: ProviderID, Kind: provider.ErrorKindTemporary, Err: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var apiErr telegramErrorResponse
		_ = json.NewDecoder(resp.Body).Decode(&apiErr)
		return classifyHTTPError(resp.StatusCode, apiErr, c.cfg.RateLimitBackoff)
	}

	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return &provider.ProviderError{ProviderID: ProviderID, Kind: provider.ErrorKindParse, Err: err}
	}
	if err := validateTelegramResponse(target); err != nil {
		return err
	}
	return nil
}

func validateTelegramResponse(target any) error {
	switch resp := target.(type) {
	case *telegramResponse[[]Update]:
		if !resp.OK {
			return telegramAPIError(resp.Description, resp.Parameters, http.StatusOK)
		}
	case *telegramResponse[File]:
		if !resp.OK {
			return telegramAPIError(resp.Description, resp.Parameters, http.StatusOK)
		}
	}
	return nil
}

func classifyHTTPError(statusCode int, apiErr telegramErrorResponse, fallback time.Duration) error {
	if statusCode == http.StatusTooManyRequests {
		retryAfter := fallback
		if apiErr.Parameters.RetryAfter > 0 {
			retryAfter = time.Duration(apiErr.Parameters.RetryAfter) * time.Second
		}
		if retryAfter == 0 {
			retryAfter = defaultRateLimitDelay
		}
		return &provider.ProviderError{
			ProviderID: ProviderID,
			Kind:       provider.ErrorKindRateLimit,
			RetryAfter: retryAfter,
			Err:        fmt.Errorf("telegram rate limited, retry after %v", retryAfter),
		}
	}

	if statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden {
		return &provider.ProviderError{
			ProviderID: ProviderID,
			Kind:       provider.ErrorKindPermission,
			Err:        fmt.Errorf("telegram permission error: %s", apiErr.Description),
		}
	}

	if statusCode == http.StatusBadRequest && strings.Contains(strings.ToLower(apiErr.Description), "file") {
		return &provider.ProviderError{
			ProviderID: ProviderID,
			Kind:       provider.ErrorKindMissingMedia,
			Err:        fmt.Errorf("telegram media error: %s", apiErr.Description),
		}
	}

	return &provider.ProviderError{
		ProviderID: ProviderID,
		Kind:       provider.ErrorKindTemporary,
		Err:        fmt.Errorf("telegram API returned status %d: %s", statusCode, apiErr.Description),
	}
}

func telegramAPIError(description string, parameters ResponseParameters, statusCode int) error {
	return classifyHTTPError(statusCode, telegramErrorResponse{
		Description: description,
		Parameters:  parameters,
	}, 0)
}
