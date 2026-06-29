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

func TestConfigDrivenConnectorSupportsDifferentSiteStructure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/archive":
			if r.URL.Query().Get("p") != "5" {
				t.Fatalf("expected p=5, got %q", r.URL.Query().Get("p"))
			}
			_, _ = w.Write([]byte(`
				<main>
					<a class="card" href="/work/one">One</a>
					<a class="card" href="/profile/artist">Skip profile</a>
				</main>
			`))
		case "/work/one":
			_, _ = w.Write([]byte(`
				<article>
					<h2 class="piece-title">  Alternate Fixture  </h2>
					<a class="artist" data-name="Alt Artist"></a>
					<time datetime="2025-03-04"></time>
					<img class="full" data-src="/media/alt.jpg">
				</article>
			`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	connector := New(Config{
		SourceID: "alt",
		Gallery: WebGalleryConfig{
			ListURL: server.URL + "/archive",
			Pagination: PaginationConfig{
				Strategy:   "page_param",
				ParamName:  "p",
				StartIndex: intPtr(1),
			},
			Selectors: SelectorConfig{
				ItemLink: "a.card",
				Image:    FieldSelector{Selector: "img.full", Attr: "data-src"},
				Title:    FieldSelector{Selector: ".piece-title"},
				Artist:   FieldSelector{Selector: ".artist", Attr: "data-name"},
				Date:     FieldSelector{Selector: "time", Attr: "datetime"},
			},
			ItemLinkFilter: []string{"/profile/"},
		},
		Retry: retry.Config{MaxAttempts: 1},
	}, server.Client(), zerolog.New(os.Stdout))

	result, err := connector.DiscoverPage(context.Background(), provider.PageRequest{Page: 5})
	if err != nil {
		t.Fatalf("DiscoverPage returned error: %v", err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected 1 filtered media item, got %d", len(result.Items))
	}
	if result.Items[0].ProviderID != "webgallery:alt" || result.Items[0].DedupeKey.ProviderID != "webgallery:alt" {
		t.Fatalf("expected per-source provider IDs, got item=%q dedupe=%q", result.Items[0].ProviderID, result.Items[0].DedupeKey.ProviderID)
	}

	resolved, err := connector.ResolveMedia(context.Background(), result.Items[0])
	if err != nil {
		t.Fatalf("ResolveMedia returned error: %v", err)
	}
	if resolved.Title != "Alternate Fixture" || resolved.Artist != "Alt Artist" {
		t.Fatalf("unexpected metadata: title=%q artist=%q", resolved.Title, resolved.Artist)
	}
	if resolved.Media.URL != server.URL+"/media/alt.jpg" {
		t.Fatalf("unexpected media URL: %s", resolved.Media.URL)
	}
	if resolved.PublishedAt.Format("2006-01-02") != "2025-03-04" {
		t.Fatalf("unexpected published date: %s", resolved.PublishedAt)
	}
}

func TestPageParamAllowsZeroStartIndex(t *testing.T) {
	connector := New(Config{
		Gallery: WebGalleryConfig{
			ListURL: "https://gallery.example.test/archive",
			Pagination: PaginationConfig{
				Strategy:   "page_param",
				ParamName:  "offset",
				StartIndex: intPtr(0),
			},
			Selectors: SelectorConfig{
				ItemLink: "a.item",
				Image:    FieldSelector{Selector: "img.full"},
			},
		},
		Retry: retry.Config{MaxAttempts: 1},
	}, nil, zerolog.Nop())

	pageURL, err := connector.pageURL(provider.PageRequest{})
	if err != nil {
		t.Fatalf("pageURL returned error: %v", err)
	}
	if pageURL != "https://gallery.example.test/archive?offset=0" {
		t.Fatalf("expected zero-based first page, got %q", pageURL)
	}
}

func TestDiscoverPageFollowsConfiguredNextLink(t *testing.T) {
	requests := make([]string, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.URL.String())
		switch r.URL.Path {
		case "/first":
			_, _ = w.Write([]byte(`<a class="item" href="/work/one">One</a><a rel="next" href="/second">Next</a>`))
		case "/second":
			_, _ = w.Write([]byte(`<a class="item" href="/work/two">Two</a>`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	connector := New(Config{
		Gallery: WebGalleryConfig{
			ListURL: server.URL + "/first",
			Pagination: PaginationConfig{
				Strategy:         "next_link",
				NextLinkSelector: "a[rel='next']",
			},
			Selectors: SelectorConfig{
				ItemLink: "a.item",
				Image:    FieldSelector{Selector: "img"},
			},
		},
		Retry: retry.Config{MaxAttempts: 1},
	}, server.Client(), zerolog.New(os.Stdout))

	first, err := connector.DiscoverPage(context.Background(), provider.PageRequest{Page: 1})
	if err != nil {
		t.Fatalf("first DiscoverPage returned error: %v", err)
	}
	if !first.Pagination.HasNext || first.Pagination.NextCursor != server.URL+"/second" {
		t.Fatalf("unexpected first pagination: %+v", first.Pagination)
	}
	second, err := connector.DiscoverPage(context.Background(), provider.PageRequest{Page: 2, Cursor: first.Pagination.NextCursor})
	if err != nil {
		t.Fatalf("second DiscoverPage returned error: %v", err)
	}
	if second.Pagination.HasNext {
		t.Fatalf("expected second page to end pagination: %+v", second.Pagination)
	}
	if len(requests) != 2 || requests[0] != "/first" || requests[1] != "/second" {
		t.Fatalf("unexpected requests: %#v", requests)
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

func intPtr(value int) *int {
	return &value
}
