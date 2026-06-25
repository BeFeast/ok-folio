package webgallery

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"ok-folio/internal/provider"
	"ok-folio/pkg/retry"

	"github.com/rs/zerolog"
)

func TestDiscoverPageUsesWebGalleryCategoryFixture(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/gallery/category/1/" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("pager") != "2" {
			t.Fatalf("expected pager=2, got %q", r.URL.Query().Get("pager"))
		}
		http.ServeFile(w, r, "testdata/category.html")
	}))
	defer server.Close()

	connector := newTestConnector(server.URL + "/gallery/category/1/")

	result, err := connector.DiscoverPage(context.Background(), provider.PageRequest{Page: 2})
	if err != nil {
		t.Fatalf("DiscoverPage returned error: %v", err)
	}

	if len(result.Items) != 2 {
		t.Fatalf("expected 2 unique media items, got %d", len(result.Items))
	}
	if result.Items[0].ProviderID != ProviderID {
		t.Fatalf("expected provider ID %q, got %q", ProviderID, result.Items[0].ProviderID)
	}
	if result.Items[0].Source.URL != server.URL+"/photos/alpha" {
		t.Fatalf("unexpected first source URL: %s", result.Items[0].Source.URL)
	}
	if result.Items[0].DedupeKey.Value != "photos/alpha" {
		t.Fatalf("dedupe key should use stable source identity, got %q", result.Items[0].DedupeKey.Value)
	}
	if !result.Pagination.HasNext || result.Pagination.NextPage != 3 {
		t.Fatalf("unexpected pagination: %+v", result.Pagination)
	}
}

func TestDiscoverPagePreservesQueryIdentityInDedupeKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`
			<div class="photo-item"><a href="/photo?id=1">One</a></div>
			<div class="photo-item"><a href="/photo?id=2">Two</a></div>
		`))
	}))
	defer server.Close()

	connector := newTestConnector(server.URL + "/gallery/category/1/")

	result, err := connector.DiscoverPage(context.Background(), provider.PageRequest{Page: 1})
	if err != nil {
		t.Fatalf("DiscoverPage returned error: %v", err)
	}

	if len(result.Items) != 2 {
		t.Fatalf("expected 2 media items with distinct query IDs, got %d", len(result.Items))
	}
	if result.Items[0].DedupeKey.Value != "photo?id=1" || result.Items[1].DedupeKey.Value != "photo?id=2" {
		t.Fatalf("dedupe keys should preserve query identity, got %q and %q", result.Items[0].DedupeKey.Value, result.Items[1].DedupeKey.Value)
	}
}

func TestResolveMediaUsesWebGalleryPhotoFixture(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "testdata/photo.html")
	}))
	defer server.Close()

	connector := newTestConnector(server.URL + "/gallery/category/1/")
	item := provider.DiscoveredMedia{
		ProviderID: ProviderID,
		DedupeKey:  provider.DedupeKey{ProviderID: ProviderID, Value: "photos/alpha"},
		Source:     provider.SourceMetadata{URL: server.URL + "/photos/alpha"},
	}

	resolved, err := connector.ResolveMedia(context.Background(), item)
	if err != nil {
		t.Fatalf("ResolveMedia returned error: %v", err)
	}

	if resolved.Title != "Fixture Photo" {
		t.Fatalf("expected title from fixture, got %q", resolved.Title)
	}
	if resolved.Artist != "Fixture Artist" {
		t.Fatalf("expected artist from fixture, got %q", resolved.Artist)
	}
	if resolved.Media.URL != server.URL+"/images/fixture-photo.jpg" {
		t.Fatalf("unexpected media URL: %s", resolved.Media.URL)
	}
	if resolved.Media.FileName != "fixture-photo.jpg" {
		t.Fatalf("unexpected file name: %s", resolved.Media.FileName)
	}
	expectedPublishedAt := time.Date(2024, 2, 3, 4, 5, 6, 0, time.UTC)
	if !resolved.PublishedAt.Equal(expectedPublishedAt) {
		t.Fatalf("unexpected published date: %s", resolved.PublishedAt)
	}
}

func TestDiscoverPageReturnsTypedRateLimitError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	connector := New(Config{
		BaseURL:          server.URL + "/gallery/category/1/",
		RateLimitBackoff: time.Nanosecond,
		Retry:            retry.Config{MaxAttempts: 1},
	}, server.Client(), zerolog.New(os.Stdout))

	_, err := connector.DiscoverPage(context.Background(), provider.PageRequest{Page: 1})
	if err == nil {
		t.Fatal("expected rate-limit error, got nil")
	}

	var providerErr *provider.ProviderError
	if !errors.As(err, &providerErr) {
		t.Fatalf("expected ProviderError, got %T: %v", err, err)
	}
	if providerErr.Kind != provider.ErrorKindRateLimit || !providerErr.Retryable() {
		t.Fatalf("unexpected provider error: %+v", providerErr)
	}
}

func newTestConnector(baseURL string) *Connector {
	return New(Config{
		BaseURL: baseURL,
		Retry:   retry.Config{MaxAttempts: 1},
	}, http.DefaultClient, zerolog.New(os.Stdout))
}
