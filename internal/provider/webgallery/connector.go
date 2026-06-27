package webgallery

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

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
	Schedule         string
	UserAgent        string
	RateLimitBackoff time.Duration
	Retry            retry.Config
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
	return provider.Source{
		ID:          ProviderID,
		DisplayName: "Web Gallery",
		BaseURL:     c.cfg.BaseURL,
		Scope:       "category",
		Schedule:    c.cfg.Schedule,
	}
}

func (c *Connector) DiscoverPage(ctx context.Context, req provider.PageRequest) (*provider.PageResult, error) {
	pageURL, err := c.pageURL(req.Page)
	if err != nil {
		return nil, err
	}

	items, err := retry.DoWithValue(ctx, c.cfg.Retry, func() ([]provider.DiscoveredMedia, error) {
		resp, err := c.get(ctx, pageURL)
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

		seen := make(map[string]bool)
		var items []provider.DiscoveredMedia
		doc.Find("div.photo-item a").Each(func(_ int, s *goquery.Selection) {
			href, exists := s.Attr("href")
			if !exists || strings.Contains(href, "javascript") || strings.Contains(href, "users") {
				return
			}

			sourceURL, err := resolveURL(pageURL, href)
			if err != nil || seen[sourceURL] {
				return
			}

			seen[sourceURL] = true
			sourceID := externalID(sourceURL)
			items = append(items, provider.DiscoveredMedia{
				ProviderID: ProviderID,
				DedupeKey:  provider.DedupeKey{ProviderID: ProviderID, Value: sourceID},
				Source: provider.SourceMetadata{
					URL:        sourceURL,
					ExternalID: sourceID,
				},
			})
		})

		return items, nil
	})
	if err != nil {
		return nil, err
	}

	return &provider.PageResult{
		Items: items,
		Pagination: provider.Pagination{
			Page:     req.Page,
			NextPage: req.Page + 1,
			HasNext:  len(items) > 0,
		},
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
		out.ProviderID = ProviderID
		if out.DedupeKey.Value == "" {
			out.DedupeKey = provider.DedupeKey{ProviderID: ProviderID, Value: externalID(item.Source.URL)}
		}

		doc.Find("h1[itemprop='name']").Each(func(_ int, s *goquery.Selection) {
			out.Title = strings.TrimSpace(s.Text())
		})

		doc.Find("meta[itemprop='datePublished']").Each(func(_ int, s *goquery.Selection) {
			if content, exists := s.Attr("content"); exists {
				if t, err := time.Parse(time.RFC3339, content); err == nil {
					out.PublishedAt = t
				}
			}
		})

		doc.Find("span[itemprop='name']").Each(func(_ int, s *goquery.Selection) {
			if out.Artist == "" {
				out.Artist = strings.TrimSpace(s.Text())
			}
		})

		doc.Find("img#big_photo").Each(func(_ int, s *goquery.Selection) {
			if src, exists := s.Attr("src"); exists {
				out.Media.URL, _ = resolveURL(item.Source.URL, src)
				out.Media.FileName = path.Base(out.Media.URL)
			}
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

func (c *Connector) pageURL(page int) (string, error) {
	base, err := url.Parse(c.cfg.BaseURL)
	if err != nil {
		return "", err
	}
	q := base.Query()
	q.Set("pager", fmt.Sprintf("%d", page))
	base.RawQuery = q.Encode()
	return base.String(), nil
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
