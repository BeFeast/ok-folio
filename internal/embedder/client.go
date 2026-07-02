package embedder

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"ok-folio/internal/database"
)

const (
	ModelID        = "clip-vit-b32"
	DefaultTimeout = 30 * time.Second
)

type Client struct {
	baseURL string
	client  *http.Client
}

type HealthResponse struct {
	OK    bool   `json:"ok"`
	Model string `json:"model"`
	Dim   int    `json:"dim"`
}

type embedResponse struct {
	Embedding []float32 `json:"embedding"`
	Model     string    `json:"model"`
	Dim       int       `json:"dim"`
}

func New(baseURL string) *Client {
	return NewWithHTTPClient(baseURL, &http.Client{Timeout: DefaultTimeout})
}

func NewWithHTTPClient(baseURL string, client *http.Client) *Client {
	if client == nil {
		client = &http.Client{Timeout: DefaultTimeout}
	}
	return &Client{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		client:  client,
	}
}

func (c *Client) Health(ctx context.Context) (HealthResponse, error) {
	if c == nil || c.baseURL == "" {
		return HealthResponse{}, fmt.Errorf("embedder sidecar URL is empty")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/health", nil)
	if err != nil {
		return HealthResponse{}, fmt.Errorf("create health request: %w", err)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return HealthResponse{}, fmt.Errorf("send health request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return HealthResponse{}, responseError("health", resp)
	}
	var out HealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return HealthResponse{}, fmt.Errorf("decode health response: %w", err)
	}
	if !out.OK || out.Model != ModelID || out.Dim != database.EmbeddingDim {
		return HealthResponse{}, fmt.Errorf("unexpected embedder health response: ok=%t model=%q dim=%d", out.OK, out.Model, out.Dim)
	}
	return out, nil
}

func (c *Client) Embed(ctx context.Context, jpegBytes []byte) ([]float32, error) {
	if c == nil || c.baseURL == "" {
		return nil, fmt.Errorf("embedder sidecar URL is empty")
	}
	if len(jpegBytes) == 0 {
		return nil, fmt.Errorf("image bytes are empty")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/embed", bytes.NewReader(jpegBytes))
	if err != nil {
		return nil, fmt.Errorf("create embed request: %w", err)
	}
	req.Header.Set("Content-Type", "image/jpeg")
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send embed request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, responseError("embed", resp)
	}
	var out embedResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode embed response: %w", err)
	}
	if out.Model != ModelID || out.Dim != database.EmbeddingDim || len(out.Embedding) != database.EmbeddingDim {
		return nil, fmt.Errorf("unexpected embed response: model=%q dim=%d len=%d", out.Model, out.Dim, len(out.Embedding))
	}
	return out.Embedding, nil
}

func responseError(operation string, resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	if len(body) == 0 {
		return fmt.Errorf("%s request failed with status %d", operation, resp.StatusCode)
	}
	return fmt.Errorf("%s request failed with status %d: %s", operation, resp.StatusCode, strings.TrimSpace(string(body)))
}
