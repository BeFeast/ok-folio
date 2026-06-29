package webgallery

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"ok-folio/internal/catalogquality"
	"ok-folio/internal/provider"
	"ok-folio/pkg/retry"

	"github.com/PuerkitoBio/goquery"
	"github.com/rs/zerolog"
)

const (
	ProviderID            = "webgallery"
	defaultRateLimitDelay = 60 * time.Second
	statusTooManyRequests = 429
)

type Config struct {
	BaseURL          string
	SourceID         string
	DisplayName      string
	Schedule         string
	UserAgent        string
	RateLimitBackoff time.Duration
	Retry            retry.Config
	Gallery          WebGalleryConfig
}

type WebGalleryConfig struct {
	ListURL        string             `json:"list_url"`
	Pagination     PaginationConfig   `json:"pagination"`
	Selectors      SelectorConfig     `json:"selectors"`
	ItemLinkFilter []string           `json:"item_link_filter"`
	UserAgent      string             `json:"user_agent,omitempty"`
	Retry          SourceRetryConfig  `json:"retry,omitempty"`
	RateLimit      SourceRateLimitCfg `json:"rate_limit,omitempty"`
	Schedule       string             `json:"schedule,omitempty"`
}

type PaginationConfig struct {
	Strategy         string `json:"strategy"`
	ParamName        string `json:"param_name,omitempty"`
	StartIndex       *int   `json:"start_index,omitempty"`
	NextLinkSelector string `json:"next_link_selector,omitempty"`
}

type SelectorConfig struct {
	ItemLink string        `json:"item_link"`
	Image    FieldSelector `json:"image"`
	Artist   FieldSelector `json:"artist,omitempty"`
	Title    FieldSelector `json:"title,omitempty"`
	Date     FieldSelector `json:"date,omitempty"`
}

type FieldSelector struct {
	Selector string `json:"selector"`
	Attr     string `json:"attr,omitempty"`
}

type SourceRetryConfig struct {
	MaxAttempts  int     `json:"max_attempts,omitempty"`
	InitialDelay string  `json:"initial_delay,omitempty"`
	MaxDelay     string  `json:"max_delay,omitempty"`
	Multiplier   float64 `json:"multiplier,omitempty"`
}

type SourceRateLimitCfg struct {
	Backoff string `json:"backoff,omitempty"`
}

type Connector struct {
	cfg    Config
	client *http.Client
	logger zerolog.Logger
}

func New(cfg Config, client *http.Client, logger zerolog.Logger) *Connector {
	if client == nil {
		client = http.DefaultClient
	}
	cfg.Gallery = withDefaults(cfg.Gallery, cfg.BaseURL)
	if cfg.BaseURL == "" {
		cfg.BaseURL = cfg.Gallery.ListURL
	}
	if cfg.UserAgent == "" {
		cfg.UserAgent = cfg.Gallery.UserAgent
	}
	if cfg.Schedule == "" {
		cfg.Schedule = cfg.Gallery.Schedule
	}
	if cfg.RateLimitBackoff == 0 && cfg.Gallery.RateLimit.Backoff != "" {
		if parsed, err := time.ParseDuration(cfg.Gallery.RateLimit.Backoff); err == nil {
			cfg.RateLimitBackoff = parsed
		}
	}
	applyRetryConfig(&cfg.Retry, cfg.Gallery.Retry)
	if cfg.Retry.MaxAttempts == 0 {
		cfg.Retry.MaxAttempts = 1
	}
	return &Connector{
		cfg:    cfg,
		client: client,
		logger: logger,
	}
}

func (c *Connector) Provider() provider.Source {
	displayName := strings.TrimSpace(c.cfg.DisplayName)
	if displayName == "" {
		displayName = "Web Gallery"
	}
	return provider.Source{
		ID:          c.providerID(),
		DisplayName: displayName,
		BaseURL:     c.cfg.BaseURL,
		Scope:       "category",
		Schedule:    c.cfg.Schedule,
	}
}

func (c *Connector) providerID() string {
	if c.cfg.SourceID != "" {
		return ProviderID + ":" + c.cfg.SourceID
	}
	return ProviderID
}

func (c *Connector) DiscoverPage(ctx context.Context, req provider.PageRequest) (*provider.PageResult, error) {
	pageURL, err := c.pageURL(req)
	if err != nil {
		return nil, err
	}

	type discoveryResult struct {
		items      []provider.DiscoveredMedia
		nextCursor string
	}
	discovered, err := retry.DoWithValue(ctx, c.cfg.Retry, func() (discoveryResult, error) {
		resp, err := c.get(ctx, pageURL)
		if err != nil {
			return discoveryResult{}, err
		}
		defer resp.Body.Close()

		if err := c.checkStatus(resp); err != nil {
			return discoveryResult{}, err
		}

		doc, err := goquery.NewDocumentFromReader(resp.Body)
		if err != nil {
			return discoveryResult{}, &provider.ProviderError{ProviderID: ProviderID, Kind: provider.ErrorKindParse, Err: err}
		}

		providerID := c.providerID()
		seen := make(map[string]bool)
		var items []provider.DiscoveredMedia
		doc.Find(c.cfg.Gallery.Selectors.ItemLink).Each(func(_ int, s *goquery.Selection) {
			href, exists := s.Attr("href")
			if !exists || c.skipItemLink(href) {
				return
			}

			sourceURL, err := resolveURL(pageURL, href)
			if err != nil || seen[sourceURL] {
				return
			}

			seen[sourceURL] = true
			sourceID := externalID(sourceURL)
			items = append(items, provider.DiscoveredMedia{
				ProviderID: providerID,
				DedupeKey:  provider.DedupeKey{ProviderID: providerID, Value: sourceID},
				Source: provider.SourceMetadata{
					URL:        sourceURL,
					ExternalID: sourceID,
				},
			})
		})

		nextCursor := ""
		if c.cfg.Gallery.Pagination.Strategy == "next_link" {
			doc.Find(c.cfg.Gallery.Pagination.NextLinkSelector).EachWithBreak(func(_ int, s *goquery.Selection) bool {
				if href, ok := s.Attr("href"); ok {
					if resolved, err := resolveURL(pageURL, href); err == nil {
						nextCursor = resolved
						return false
					}
				}
				return true
			})
		}

		return discoveryResult{items: items, nextCursor: nextCursor}, nil
	})
	if err != nil {
		return nil, err
	}

	return &provider.PageResult{
		Items:      discovered.items,
		Pagination: c.pagination(req.Page, discovered.items, discovered.nextCursor),
	}, nil
}

func (c *Connector) ResolveMedia(ctx context.Context, item provider.DiscoveredMedia) (*provider.DiscoveredMedia, error) {
	if item.Source.URL == "" {
		return nil, &provider.ProviderError{
			ProviderID: ProviderID,
			Kind:       provider.ErrorKindParse,
			Err:        fmt.Errorf("source URL is required"),
		}
	}

	resolved, err := retry.DoWithValue(ctx, c.cfg.Retry, func() (*provider.DiscoveredMedia, error) {
		resp, err := c.get(ctx, item.Source.URL)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if err := c.checkStatus(resp); err != nil {
			return nil, err
		}

		doc, err := goquery.NewDocumentFromReader(resp.Body)
		if err != nil {
			return nil, &provider.ProviderError{ProviderID: ProviderID, Kind: provider.ErrorKindParse, Err: err}
		}

		out := item
		providerID := c.providerID()
		out.ProviderID = providerID
		if out.DedupeKey.Value == "" {
			out.DedupeKey = provider.DedupeKey{ProviderID: providerID, Value: externalID(item.Source.URL)}
		}

		if title := selectorValue(doc, c.cfg.Gallery.Selectors.Title); title != "" {
			out.Title = catalogquality.NormalizeTitle(title)
		}

		if value := selectorValue(doc, c.cfg.Gallery.Selectors.Date); value != "" {
			if t, err := parsePublishedAt(value); err == nil {
				out.PublishedAt = t
			}
		}

		if artist := selectorValue(doc, c.cfg.Gallery.Selectors.Artist); artist != "" {
			out.Artist = strings.TrimSpace(artist)
		}

		doc.Find(c.cfg.Gallery.Selectors.Image.Selector).EachWithBreak(func(_ int, s *goquery.Selection) bool {
			if src := selectionValue(s, c.cfg.Gallery.Selectors.Image); src != "" {
				out.Media.URL, _ = resolveURL(item.Source.URL, src)
				out.Media.FileName = path.Base(out.Media.URL)
				return false
			}
			return true
		})

		if out.Media.URL == "" {
			return nil, &provider.ProviderError{
				ProviderID: ProviderID,
				Kind:       provider.ErrorKindParse,
				Err:        fmt.Errorf("image URL not found"),
			}
		}

		return &out, nil
	})
	if err != nil {
		return nil, err
	}

	return resolved, nil
}

func (c *Connector) pageURL(req provider.PageRequest) (string, error) {
	if c.cfg.Gallery.Pagination.Strategy == "next_link" && strings.TrimSpace(req.Cursor) != "" {
		return req.Cursor, nil
	}
	base, err := url.Parse(c.cfg.Gallery.ListURL)
	if err != nil {
		return "", err
	}
	if c.cfg.Gallery.Pagination.Strategy != "page_param" {
		return base.String(), nil
	}
	page := req.Page
	if page == 0 {
		page = startIndexValue(c.cfg.Gallery.Pagination)
	}
	q := base.Query()
	q.Set(c.cfg.Gallery.Pagination.ParamName, fmt.Sprintf("%d", page))
	base.RawQuery = q.Encode()
	return base.String(), nil
}

func (c *Connector) pagination(page int, items []provider.DiscoveredMedia, nextCursor string) provider.Pagination {
	if page == 0 {
		page = startIndexValue(c.cfg.Gallery.Pagination)
	}
	switch c.cfg.Gallery.Pagination.Strategy {
	case "none":
		return provider.Pagination{Page: page, HasNext: false}
	case "next_link":
		return provider.Pagination{Page: page, NextPage: page + 1, NextCursor: nextCursor, HasNext: nextCursor != ""}
	default:
		return provider.Pagination{Page: page, NextPage: page + 1, HasNext: len(items) > 0}
	}
}

func (c *Connector) skipItemLink(href string) bool {
	for _, filter := range c.cfg.Gallery.ItemLinkFilter {
		if filter != "" && strings.Contains(href, filter) {
			return true
		}
	}
	return false
}

func (c *Connector) get(ctx context.Context, target string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	if c.cfg.UserAgent != "" {
		req.Header.Set("User-Agent", c.cfg.UserAgent)
	}
	return c.client.Do(req)
}

func (c *Connector) checkStatus(resp *http.Response) error {
	if resp.StatusCode == statusTooManyRequests {
		retryAfter := c.cfg.RateLimitBackoff
		if retryAfter == 0 {
			retryAfter = defaultRateLimitDelay
		}
		c.logger.Warn().Dur("retry_after", retryAfter).Msg("Rate limited by webgallery")
		time.Sleep(retryAfter)
		return &provider.ProviderError{
			ProviderID: ProviderID,
			Kind:       provider.ErrorKindRateLimit,
			RetryAfter: retryAfter,
			Err:        fmt.Errorf("rate limited, retry after %v", retryAfter),
		}
	}
	if resp.StatusCode == http.StatusNotFound {
		return &provider.ProviderError{
			ProviderID: ProviderID,
			Kind:       provider.ErrorKindNotFound,
			Err:        fmt.Errorf("not found: %s", resp.Request.URL.String()),
		}
	}
	if resp.StatusCode != http.StatusOK {
		return &provider.ProviderError{
			ProviderID: ProviderID,
			Kind:       provider.ErrorKindTemporary,
			Err:        fmt.Errorf("unexpected status code: %d", resp.StatusCode),
		}
	}
	return nil
}

func resolveURL(baseURL, ref string) (string, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	parsed, err := url.Parse(ref)
	if err != nil {
		return "", err
	}
	return base.ResolveReference(parsed).String(), nil
}

func externalID(sourceURL string) string {
	parsed, err := url.Parse(sourceURL)
	if err != nil {
		return sourceURL
	}
	id := strings.Trim(parsed.Path, "/")
	if parsed.RawQuery != "" {
		id += "?" + parsed.RawQuery
	}
	return id
}

func DefaultConfig(listURL string) WebGalleryConfig {
	startIndex := 1
	return WebGalleryConfig{
		ListURL: strings.TrimSpace(listURL),
		Pagination: PaginationConfig{
			Strategy:   "page_param",
			ParamName:  "pager",
			StartIndex: &startIndex,
		},
		Selectors: SelectorConfig{
			ItemLink: "div.photo-item a",
			Image:    FieldSelector{Selector: "img#big_photo", Attr: "src"},
			Title:    FieldSelector{Selector: "h1[itemprop='name']"},
			Artist:   FieldSelector{Selector: "span[itemprop='name']"},
			Date:     FieldSelector{Selector: "meta[itemprop='datePublished']", Attr: "content"},
		},
		ItemLinkFilter: []string{"javascript", "users"},
	}
}

func ParseConfig(data []byte) (WebGalleryConfig, error) {
	var cfg WebGalleryConfig
	if len(strings.TrimSpace(string(data))) == 0 {
		return cfg, fmt.Errorf("webgallery config is required")
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	cfg = withDefaults(cfg, "")
	return cfg, ValidateConfig(cfg)
}

func ValidateConfig(cfg WebGalleryConfig) error {
	cfg = withDefaults(cfg, "")
	parsed, err := url.Parse(strings.TrimSpace(cfg.ListURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("webgallery list_url must be an absolute URL")
	}
	if strings.TrimSpace(cfg.Selectors.ItemLink) == "" {
		return fmt.Errorf("webgallery selectors.item_link is required")
	}
	if strings.TrimSpace(cfg.Selectors.Image.Selector) == "" {
		return fmt.Errorf("webgallery selectors.image.selector is required")
	}
	switch cfg.Pagination.Strategy {
	case "page_param":
		if strings.TrimSpace(cfg.Pagination.ParamName) == "" {
			return fmt.Errorf("webgallery pagination.param_name is required for page_param")
		}
	case "next_link":
		if strings.TrimSpace(cfg.Pagination.NextLinkSelector) == "" {
			return fmt.Errorf("webgallery pagination.next_link_selector is required for next_link")
		}
	case "none":
	default:
		return fmt.Errorf("unknown webgallery pagination strategy %q", cfg.Pagination.Strategy)
	}
	return nil
}

func withDefaults(cfg WebGalleryConfig, fallbackListURL string) WebGalleryConfig {
	if strings.TrimSpace(cfg.ListURL) == "" {
		cfg.ListURL = strings.TrimSpace(fallbackListURL)
	}
	if cfg.Pagination.Strategy == "" {
		cfg.Pagination.Strategy = "page_param"
	}
	if cfg.Pagination.ParamName == "" && cfg.Pagination.Strategy == "page_param" {
		cfg.Pagination.ParamName = "pager"
	}
	if cfg.Pagination.StartIndex == nil {
		startIndex := 1
		cfg.Pagination.StartIndex = &startIndex
	}
	if cfg.Selectors.ItemLink == "" && fallbackListURL != "" {
		defaults := DefaultConfig(fallbackListURL)
		cfg.Selectors = defaults.Selectors
		cfg.ItemLinkFilter = defaults.ItemLinkFilter
	}
	return cfg
}

func startIndexValue(cfg PaginationConfig) int {
	if cfg.StartIndex == nil {
		return 1
	}
	return *cfg.StartIndex
}

func applyRetryConfig(dst *retry.Config, src SourceRetryConfig) {
	if src.MaxAttempts > 0 {
		dst.MaxAttempts = src.MaxAttempts
	}
	if src.InitialDelay != "" {
		if parsed, err := time.ParseDuration(src.InitialDelay); err == nil {
			dst.InitialDelay = parsed
		}
	}
	if src.MaxDelay != "" {
		if parsed, err := time.ParseDuration(src.MaxDelay); err == nil {
			dst.MaxDelay = parsed
		}
	}
	if src.Multiplier > 0 {
		dst.Multiplier = src.Multiplier
	}
}

func selectorValue(doc *goquery.Document, field FieldSelector) string {
	if strings.TrimSpace(field.Selector) == "" {
		return ""
	}
	var value string
	doc.Find(field.Selector).EachWithBreak(func(_ int, s *goquery.Selection) bool {
		value = selectionValue(s, field)
		return value == ""
	})
	return strings.TrimSpace(value)
}

func selectionValue(s *goquery.Selection, field FieldSelector) string {
	if field.Attr != "" {
		if value, ok := s.Attr(field.Attr); ok {
			return strings.TrimSpace(value)
		}
		return ""
	}
	return strings.TrimSpace(s.Text())
}

func parsePublishedAt(value string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t, nil
	}
	return time.Parse("2006-01-02", value)
}
