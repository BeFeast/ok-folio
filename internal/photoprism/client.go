package photoprism

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// Client represents a PhotoPrism API client
type Client struct {
	baseURL  string
	username string
	password string
	client   *http.Client
	logger   zerolog.Logger

	// Session management
	sessionMu    sync.RWMutex
	sessionToken string
	sessionExp   time.Time
}

// SessionRequest is the request body for creating a session
type SessionRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// SessionResponse is the response from creating a session
type SessionResponse struct {
	ID          string `json:"id"`
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

// IndexOptions represents options for triggering indexing
type IndexOptions struct {
	Path string `json:"path,omitempty"`
}

// New creates a new PhotoPrism API client
func New(baseURL, username, password string, logger zerolog.Logger) *Client {
	return &Client{
		baseURL:  baseURL,
		username: username,
		password: password,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

// authenticate obtains a session token from PhotoPrism
func (c *Client) authenticate(ctx context.Context) error {
	c.sessionMu.Lock()
	defer c.sessionMu.Unlock()

	// Check if we have a valid token
	if c.sessionToken != "" && time.Now().Before(c.sessionExp) {
		return nil
	}

	c.logger.Debug().Msg("Authenticating with PhotoPrism")

	reqBody := SessionRequest{
		Username: c.username,
		Password: c.password,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal session request: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/session", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("authentication failed with status: %d", resp.StatusCode)
	}

	var sessionResp SessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&sessionResp); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	c.sessionToken = sessionResp.AccessToken
	// Set expiration to 80% of actual expiration time for safety
	expiresIn := time.Duration(sessionResp.ExpiresIn) * time.Second
	c.sessionExp = time.Now().Add(expiresIn * 80 / 100)

	c.logger.Info().Msg("Successfully authenticated with PhotoPrism")
	return nil
}

// getToken returns a valid session token, authenticating if necessary
func (c *Client) getToken(ctx context.Context) (string, error) {
	c.sessionMu.RLock()
	if c.sessionToken != "" && time.Now().Before(c.sessionExp) {
		token := c.sessionToken
		c.sessionMu.RUnlock()
		return token, nil
	}
	c.sessionMu.RUnlock()

	if err := c.authenticate(ctx); err != nil {
		return "", err
	}

	c.sessionMu.RLock()
	defer c.sessionMu.RUnlock()
	return c.sessionToken, nil
}

// TriggerIndex triggers PhotoPrism to start indexing
func (c *Client) TriggerIndex(ctx context.Context) error {
	token, err := c.getToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to get auth token: %w", err)
	}

	c.logger.Info().Msg("Triggering PhotoPrism indexing")

	// IndexOptions structure expected by PhotoPrism API
	type IndexOptions struct {
		Path    string `json:"path,omitempty"`
		Rescan  bool   `json:"rescan,omitempty"`
		Cleanup bool   `json:"cleanup,omitempty"`
	}

	// Send empty object - let PhotoPrism use defaults
	options := IndexOptions{}

	body, err := json.Marshal(options)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/index", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Auth-Token", token)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("index trigger failed with status: %d", resp.StatusCode)
	}

	c.logger.Info().Msg("Successfully triggered PhotoPrism indexing")
	return nil
}

// Ping checks if PhotoPrism is accessible
func (c *Client) Ping(ctx context.Context) error {
	url := fmt.Sprintf("%s/api/v1/config", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ping failed with status: %d", resp.StatusCode)
	}

	return nil
}

// PhotoSearchResult represents a photo from PhotoPrism search
type PhotoSearchResult struct {
	UID      string `json:"UID"`
	Type     string `json:"Type"`
	Title    string `json:"Title"`
	FileName string `json:"FileName"`
	Favorite bool   `json:"Favorite"`
}

// SearchByFilename searches PhotoPrism for a photo by filename and returns its UID
func (c *Client) SearchByFilename(ctx context.Context, filename string) (string, bool, error) {
	token, err := c.getToken(ctx)
	if err != nil {
		return "", false, fmt.Errorf("failed to get auth token: %w", err)
	}

	// Search using the name filter
	url := fmt.Sprintf("%s/api/v1/photos?count=1&q=name:%q", c.baseURL, filename)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", false, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Auth-Token", token)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", false, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", false, fmt.Errorf("search failed with status: %d", resp.StatusCode)
	}

	var results []PhotoSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return "", false, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(results) == 0 {
		return "", false, fmt.Errorf("photo not found in PhotoPrism: %s", filename)
	}

	return results[0].UID, results[0].Favorite, nil
}

// LikePhoto adds a photo to favorites in PhotoPrism
func (c *Client) LikePhoto(ctx context.Context, uid string) error {
	token, err := c.getToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to get auth token: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/photos/%s/like", c.baseURL, uid)
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Auth-Token", token)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("like failed with status: %d", resp.StatusCode)
	}

	c.logger.Debug().Str("uid", uid).Msg("Photo added to favorites")
	return nil
}

// DislikePhoto removes a photo from favorites in PhotoPrism
func (c *Client) DislikePhoto(ctx context.Context, uid string) error {
	token, err := c.getToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to get auth token: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/photos/%s/like", c.baseURL, uid)
	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Auth-Token", token)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("dislike failed with status: %d", resp.StatusCode)
	}

	c.logger.Debug().Str("uid", uid).Msg("Photo removed from favorites")
	return nil
}
