package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"regexp"
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

var (
	captionArtistPattern = regexp.MustCompile(`(?i)^.+?\s*\((?:[^)]*,\s*)?\d{4}\s*[-–—]\s*\d{4}\)`)
	captionYearPattern   = regexp.MustCompile(`\s+(\d{4})\s*г\.?\s*$`)
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
	parsedCaption := parseArtworkCaption(message.Caption)
	title := parsedCaption.Title
	if title == "" {
		title = ref.FileName
	}
	sourceURL := source.URL
	if sourceURL == "" {
		sourceURL = ProviderID
	}

	return provider.DiscoveredMedia{
		ProviderID: ProviderID,
		DedupeKey: provider.DedupeKey{
			ProviderID: ProviderID,
			Value:      telegramDedupeValue(externalID, ref),
		},
		Source: provider.SourceMetadata{
			URL:            sourceURL,
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
		Artist:      parsedCaption.Artist,
		PublishedAt: parsedCaption.Date,
	}, true
}

type parsedArtworkCaption struct {
	Title  string
	Artist string
	Date   time.Time
	Medium string
}

func parseArtworkCaption(caption string) parsedArtworkCaption {
	lines := captionLines(caption)
	if len(lines) == 0 {
		return parsedArtworkCaption{}
	}

	artistIndex := -1
	mediumIndex := -1
	parsed := parsedArtworkCaption{}
	for idx, line := range lines {
		if parsed.Artist == "" && captionArtistPattern.MatchString(line) {
			parsed.Artist = line
			artistIndex = idx
			continue
		}
		if parsed.Medium == "" && isMediumLine(line) {
			parsed.Medium = line
			mediumIndex = idx
		}
	}

	for idx, line := range lines {
		if idx == artistIndex || idx == mediumIndex {
			continue
		}
		parsed.Title, parsed.Date = parseTitleAndDate(line)
		if parsed.Title != "" {
			return parsed
		}
	}

	parsed.Title = lines[0]
	return parsed
}

func captionLines(caption string) []string {
	rawLines := strings.Split(caption, "\n")
	lines := make([]string, 0, len(rawLines))
	for _, line := range rawLines {
		line = strings.TrimSpace(line)
		if line == "" || isJunkCaptionLine(line) {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}

func isJunkCaptionLine(line string) bool {
	normalized := strings.ToLower(line)
	return strings.Contains(line, "🔞") ||
		strings.Contains(normalized, "секретный контент") ||
		strings.Contains(normalized, "secret content")
}

func isMediumLine(line string) bool {
	normalized := strings.ToLower(line)
	if captionYearPattern.MatchString(normalized) {
		return false
	}

	keywords := map[string]struct{}{
		"холст": {}, "масло": {}, "бумага": {}, "акварель": {}, "картон": {}, "темпера": {},
		"гуашь": {}, "пастель": {}, "карандаш": {}, "тушь": {}, "уголь": {}, "сангина": {},
		"дерево": {}, "медь": {}, "бронза": {}, "мрамор": {}, "canvas": {}, "oil": {},
		"paper": {}, "watercolor": {}, "gouache": {}, "tempera": {}, "pastel": {},
		"pencil": {}, "ink": {}, "charcoal": {}, "board": {},
	}
	connectors := map[string]struct{}{
		"на": {}, "по": {}, "и": {}, "с": {}, "on": {}, "and": {},
	}

	materialCount := 0
	tokens := strings.FieldsFunc(normalized, func(r rune) bool {
		return r == ',' || r == ';' || r == '/' || r == '\\' || r == '+' || r == '&' || r == '.' || r == ':' || r == '(' || r == ')' || r == '\t' || r == ' '
	})
	for _, token := range tokens {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		if _, ok := keywords[token]; ok {
			materialCount++
			continue
		}
		if _, ok := connectors[token]; ok {
			continue
		}
		return false
	}
	if materialCount == 0 {
		return false
	}
	if strings.ContainsAny(line, ",;/\\+&") || materialCount > 1 {
		return true
	}
	return line == normalized
}

func parseTitleAndDate(line string) (string, time.Time) {
	title := line
	var date time.Time
	if match := captionYearPattern.FindStringSubmatch(line); len(match) == 2 {
		if year, err := strconv.Atoi(match[1]); err == nil {
			date = time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
		}
		title = captionYearPattern.ReplaceAllString(line, "")
	}
	title = strings.TrimSpace(title)
	title = strings.Trim(title, `"'“”«»`)
	return title, date
}

func fallbackExternalID(message Message) string {
	return fmt.Sprintf("%s:%d", message.Chat.IDString(), message.MessageID)
}

func telegramDedupeValue(sourceID string, ref MediaRef) string {
	if ref.StableID() == "" {
		return sourceID
	}
	return sourceID + ":" + ref.StableID()
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
