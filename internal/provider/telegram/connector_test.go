package telegram

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"ok-folio/internal/provider"
	"ok-folio/pkg/retry"

	"github.com/rs/zerolog"
)

func TestDiscoverPageMapsTelegramFixture(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/botfixture-token/getUpdates" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("offset") != "1000" {
			t.Fatalf("expected offset=1000, got %q", r.URL.Query().Get("offset"))
		}
		if r.URL.Query().Get("limit") != "25" {
			t.Fatalf("expected limit=25, got %q", r.URL.Query().Get("limit"))
		}
		http.ServeFile(w, r, "testdata/updates.json")
	}))
	defer server.Close()

	connector := newTestConnector(server.URL)

	result, err := connector.DiscoverPage(context.Background(), provider.PageRequest{Cursor: "1000"})
	if err != nil {
		t.Fatalf("DiscoverPage returned error: %v", err)
	}

	if len(result.Items) != 2 {
		t.Fatalf("expected 2 media items, got %d", len(result.Items))
	}
	if result.Pagination.NextCursor != "1003" || !result.Pagination.HasNext {
		t.Fatalf("unexpected pagination: %+v", result.Pagination)
	}

	item := result.Items[0]
	if item.ProviderID != ProviderID {
		t.Fatalf("expected provider ID %q, got %q", ProviderID, item.ProviderID)
	}
	if item.Source.CollectionID != "-1001234567890" {
		t.Fatalf("unexpected collection ID: %q", item.Source.CollectionID)
	}
	if item.Source.CollectionName != "Fixture Channel" {
		t.Fatalf("unexpected collection name: %q", item.Source.CollectionName)
	}
	if item.Source.ItemID != "42" || item.Source.ExternalID != "-1001234567890:42" {
		t.Fatalf("unexpected source IDs: %+v", item.Source)
	}
	if item.Source.URL != "https://t.me/fixture_channel/42" {
		t.Fatalf("unexpected source URL: %s", item.Source.URL)
	}
	if item.Media.ExternalID != "photo-large-file-id" {
		t.Fatalf("expected largest photo file ID, got %q", item.Media.ExternalID)
	}
	if item.Media.MIMEType != "image/jpeg" {
		t.Fatalf("unexpected MIME type: %q", item.Media.MIMEType)
	}
	if item.DedupeKey.Value != "-1001234567890:42:photo-large-unique-id" {
		t.Fatalf("unexpected dedupe key: %s", item.DedupeKey.Value)
	}
	if item.PublishedAt != time.Unix(1700000000, 0).UTC() {
		t.Fatalf("unexpected published date: %s", item.PublishedAt)
	}
}

func TestResolveMediaUsesGetFileFixture(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/botfixture-token/getFile" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("file_id") != "photo-large-file-id" {
			t.Fatalf("unexpected file_id: %q", r.URL.Query().Get("file_id"))
		}
		http.ServeFile(w, r, "testdata/get_file.json")
	}))
	defer server.Close()

	connector := newTestConnector(server.URL)
	item := provider.DiscoveredMedia{
		ProviderID: ProviderID,
		DedupeKey:  provider.DedupeKey{ProviderID: ProviderID, Value: "-1001234567890:42:photo-large-unique-id"},
		Source:     provider.SourceMetadata{ExternalID: "-1001234567890:42"},
		Media:      provider.MediaMetadata{ExternalID: "photo-large-file-id", MIMEType: "image/jpeg"},
	}

	resolved, err := connector.ResolveMedia(context.Background(), item)
	if err != nil {
		t.Fatalf("ResolveMedia returned error: %v", err)
	}

	expectedURL := server.URL + "/file/botfixture-token/photos/file_42.jpg"
	if resolved.Media.URL != expectedURL {
		t.Fatalf("unexpected media URL: %s", resolved.Media.URL)
	}
	if resolved.Media.FileName != "file_42.jpg" {
		t.Fatalf("unexpected file name: %s", resolved.Media.FileName)
	}
}

func TestDiscoverPageFiltersConfiguredChat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "testdata/updates.json")
	}))
	defer server.Close()

	connector := New(Config{
		BotToken:    "fixture-token",
		BaseURL:     server.URL,
		FileBaseURL: server.URL + "/file",
		ChatID:      "12345",
		Retry:       retry.Config{MaxAttempts: 1},
	}, server.Client(), zerolog.New(os.Stdout))

	result, err := connector.DiscoverPage(context.Background(), provider.PageRequest{})
	if err != nil {
		t.Fatalf("DiscoverPage returned error: %v", err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected 1 filtered media item, got %d", len(result.Items))
	}
	if result.Items[0].Source.CollectionName != "Fixture User" {
		t.Fatalf("unexpected filtered item: %+v", result.Items[0].Source)
	}
}

func TestDiscoverPageRejectsInvalidCursor(t *testing.T) {
	_, err := newTestConnector("https://example.test").DiscoverPage(context.Background(), provider.PageRequest{Cursor: "later"})
	assertProviderError(t, err, provider.ErrorKindParse)
}

func TestResolveMediaRequiresMediaID(t *testing.T) {
	_, err := newTestConnector("https://example.test").ResolveMedia(context.Background(), provider.DiscoveredMedia{})
	assertProviderError(t, err, provider.ErrorKindMissingMedia)
}

func TestTelegramHTTPErrorClassification(t *testing.T) {
	tests := []struct {
		name string
		code int
		body string
		kind provider.ErrorKind
	}{
		{
			name: "rate limit",
			code: http.StatusTooManyRequests,
			body: `{"ok":false,"description":"Too Many Requests","parameters":{"retry_after":7}}`,
			kind: provider.ErrorKindRateLimit,
		},
		{
			name: "permission",
			code: http.StatusForbidden,
			body: `{"ok":false,"description":"Forbidden: bot was blocked by the user"}`,
			kind: provider.ErrorKindPermission,
		},
		{
			name: "missing media",
			code: http.StatusBadRequest,
			body: `{"ok":false,"description":"Bad Request: file not found"}`,
			kind: provider.ErrorKindMissingMedia,
		},
		{
			name: "temporary",
			code: http.StatusInternalServerError,
			body: `{"ok":false,"description":"Internal Server Error"}`,
			kind: provider.ErrorKindTemporary,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.code)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			_, err := newTestConnector(server.URL).DiscoverPage(context.Background(), provider.PageRequest{})
			assertProviderError(t, err, tt.kind)

			if tt.kind == provider.ErrorKindRateLimit {
				var providerErr *provider.ProviderError
				if !errors.As(err, &providerErr) {
					t.Fatalf("expected ProviderError, got %T", err)
				}
				if providerErr.RetryAfter != 7*time.Second || !providerErr.Retryable() {
					t.Fatalf("unexpected rate-limit metadata: %+v", providerErr)
				}
			}
		})
	}
}

func TestTelegramParseFailureClassification(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"ok": true,`))
	}))
	defer server.Close()

	_, err := newTestConnector(server.URL).DiscoverPage(context.Background(), provider.PageRequest{})
	assertProviderError(t, err, provider.ErrorKindParse)
}

func TestProviderRequiresToken(t *testing.T) {
	connector := New(Config{BaseURL: "https://example.test", Retry: retry.Config{MaxAttempts: 1}}, http.DefaultClient, zerolog.New(os.Stdout))
	_, err := connector.DiscoverPage(context.Background(), provider.PageRequest{})
	assertProviderError(t, err, provider.ErrorKindPermission)
}

func assertProviderError(t *testing.T, err error, kind provider.ErrorKind) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected %s ProviderError, got nil", kind)
	}
	var providerErr *provider.ProviderError
	if !errors.As(err, &providerErr) {
		t.Fatalf("expected ProviderError, got %T: %v", err, err)
	}
	if providerErr.ProviderID != ProviderID {
		t.Fatalf("unexpected provider ID: %q", providerErr.ProviderID)
	}
	if providerErr.Kind != kind {
		t.Fatalf("expected error kind %q, got %q: %v", kind, providerErr.Kind, providerErr)
	}
}

func newTestConnector(baseURL string) *Connector {
	return New(Config{
		BotToken:    "fixture-token",
		BaseURL:     baseURL,
		FileBaseURL: strings.TrimRight(baseURL, "/") + "/file",
		Limit:       25,
		Retry:       retry.Config{MaxAttempts: 1},
	}, http.DefaultClient, zerolog.New(os.Stdout))
}
