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
	"unicode"

	"ok-folio/internal/catalogquality"
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
	// captionArtistPattern matches an artist line ending in a parenthetical with a
	// year: a lifespan range "(США, 1928 - 2017)", a single year "(Omar Ortiz, 1977)",
	// or a birth-only "(США, р. 1975)" / "(b. 1956)".
	captionArtistPattern = regexp.MustCompile(`(?i)^.+?\s*\((?:[^)]*,\s*)?(?:р\.\s*|b\.\s*)?\d{4}(?:\s*[-–—]\s*\d{4})?\)\s*$`)
	captionYearPattern   = regexp.MustCompile(`\s+(\d{4})\s*г\.?\s*$`)
	// captionEnglishArtistPattern matches the English one-line form
	// "<Medium>. <Nationality> artist <Name> (b. <year>)", e.g.
	// "Pastel. Spanish artist Vicente Romero Redondo (b. 1956)" — medium + artist
	// on one line, no separate title.
	captionEnglishArtistPattern = regexp.MustCompile(`(?i)^(\p{L}+)\.\s+(\p{L}+)\s+artist\s+(.+?)\s*\(\s*b\.?\s*(\d{4})\s*\)\s*$`)
	// captionArtistPrefix strips a leading role label such as "Художник:".
	captionArtistPrefix = regexp.MustCompile(`(?i)^\s*(?:художник|painter|artist|автор)\s*[:\-—]\s*`)
	// captionDashOnly trims dash-only placeholder titles.
	captionDashOnly = regexp.MustCompile(`^[-–—\s]*$`)
)

type Config struct {
	BotToken         string
	BaseURL          string
	FileBaseURL      string
	ChatID           string
	Sources          []SourceConfig
	DisplayName      string
	Limit            int
	Schedule         string
	RateLimitBackoff time.Duration
	Retry            retry.Config
	SourceStore      SourceStore
}

type SourceConfig struct {
	ChatID string
	Label  string
}

type SourceStore interface {
	ConnectorSourceScopes(providerID string) (scopes []string, managed bool, err error)
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
		Scope:       strings.Join(c.configuredChatIDs(), ","),
		Schedule:    c.cfg.Schedule,
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
	allowedChatIDs, err := c.allowedChatIDs()
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
		if len(allowedChatIDs) > 0 && !messageMatchesAnyChatID(*message, allowedChatIDs) {
			continue
		}
		if allowedChatIDs != nil && len(allowedChatIDs) == 0 {
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

func (c *Connector) allowedChatIDs() (map[string]struct{}, error) {
	if c.cfg.SourceStore != nil {
		scopes, managed, err := c.cfg.SourceStore.ConnectorSourceScopes(ProviderID)
		if err != nil {
			return nil, err
		}
		if managed {
			return chatIDSet(scopes), nil
		}
	}
	ids := c.configuredChatIDs()
	if len(ids) == 0 {
		return nil, nil
	}
	return chatIDSet(ids), nil
}

func (c *Connector) configuredChatIDs() []string {
	var ids []string
	seen := make(map[string]struct{})
	add := func(chatID string) {
		chatID = strings.TrimSpace(chatID)
		if chatID == "" {
			return
		}
		if _, ok := seen[chatID]; ok {
			return
		}
		seen[chatID] = struct{}{}
		ids = append(ids, chatID)
	}
	add(c.cfg.ChatID)
	for _, source := range c.cfg.Sources {
		add(source.ChatID)
	}
	return ids
}

func chatIDSet(ids []string) map[string]struct{} {
	out := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id != "" {
			out[id] = struct{}{}
		}
	}
	return out
}

func messageMatchesAnyChatID(message Message, ids map[string]struct{}) bool {
	if _, ok := ids[message.Chat.IDString()]; ok {
		return true
	}
	source := message.SourceRef()
	if _, ok := ids[source.ChatID]; ok {
		return true
	}
	return false
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
	title := catalogquality.NormalizeTitle(parsedCaption.Title, ref.FileName)
	// Source should name the originating channel (e.g. "Нимфы и Музы"), which is
	// the forwarded-from chat, not the operator's DM. Prefer a real forward URL,
	// then the channel name, then the provider id as a last resort.
	sourceURL := source.URL
	if sourceURL == "" {
		sourceURL = collectionName
	}
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

	// English one-line form "<Medium>. <Nationality> artist <Name> (b. <year>)"
	// packs medium + artist together and carries no separate title.
	for idx, line := range lines {
		m := captionEnglishArtistPattern.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		parsed.Artist = fmt.Sprintf("%s (%s, b. %s)", strings.TrimSpace(m[3]), strings.TrimSpace(m[2]), m[4])
		parsed.Medium = strings.TrimSpace(m[1])
		artistIndex = idx
		mediumIndex = idx
		break
	}

	for idx, line := range lines {
		if parsed.Artist == "" && captionArtistPattern.MatchString(line) {
			parsed.Artist = stripArtistPrefix(line)
			artistIndex = idx
			continue
		}
		if parsed.Medium == "" && isMediumLine(line) {
			parsed.Medium = line
			mediumIndex = idx
		}
	}

	// Fallback: when no "Name (… year …)" line is present, the artist may be a
	// bare person name (e.g. "Виктор Анатольевич Долгополов"). Pick the first
	// name-shaped line that isn't the medium line.
	if parsed.Artist == "" {
		for idx, line := range lines {
			if idx == mediumIndex {
				continue
			}
			if looksLikeArtistName(line) {
				parsed.Artist = line
				artistIndex = idx
				break
			}
		}
	}

	for idx, line := range lines {
		if idx == artistIndex || idx == mediumIndex {
			continue
		}
		title, date := parseTitleAndDate(line)
		if title != "" {
			parsed.Title = title
			parsed.Date = date
			return parsed
		}
	}

	return parsed
}

// looksLikeArtistName reports whether a line looks like a bare person name
// (2-4 capitalized words, no quotes, digits, or sentence punctuation) — used
// only when no parenthetical "Name (… year …)" artist line was found.
func looksLikeArtistName(line string) bool {
	if strings.ContainsAny(line, `"'“”«»!?.:0123456789()`) {
		return false
	}
	fields := strings.Fields(line)
	if len(fields) < 2 || len(fields) > 4 {
		return false
	}
	for _, f := range fields {
		r := []rune(f)
		if len(r) == 0 || !unicode.IsUpper(r[0]) {
			return false
		}
	}
	return true
}

func captionLines(caption string) []string {
	rawLines := strings.Split(caption, "\n")
	lines := make([]string, 0, len(rawLines))
	for _, line := range rawLines {
		line = strings.TrimSpace(line)
		line = stripCaptionDecorations(line)
		if line == "" || isJunkCaptionLine(line) {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}

// stripCaptionDecorations trims leading/trailing decorative emoji/symbols
// (e.g. ⚡️, 🔥) and their variation selectors so they don't leak into a title.
func stripCaptionDecorations(line string) string {
	return strings.TrimFunc(line, func(r rune) bool {
		// So/Sk = emoji & symbol modifiers, Cf = ZWJ, Mn/Me = variation
		// selectors (e.g. U+FE0F after ⚡) and other combining marks.
		return unicode.IsSpace(r) || unicode.In(r, unicode.So, unicode.Sk, unicode.Cf, unicode.Mn, unicode.Me)
	})
}

// stripArtistPrefix removes a leading role label such as "Художник:" / "Artist -".
func stripArtistPrefix(s string) string {
	return strings.TrimSpace(captionArtistPrefix.ReplaceAllString(s, ""))
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
		"дерево": {}, "медь": {}, "бронза": {}, "мрамор": {}, "лён": {}, "лен": {},
		"оргалит": {}, "фанера": {}, "акрил": {}, "акрилик": {},
		"canvas": {}, "oil": {}, "linen": {}, "acrylic": {}, "panel": {},
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
	title = strings.TrimSpace(title)
	// A lone dash (and similar placeholders) means "no title".
	if captionDashOnly.MatchString(title) {
		return "", date
	}
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
