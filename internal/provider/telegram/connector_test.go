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
		if r.URL.Path != "/botfixture-credential/getUpdates" {
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
	if item.Title != "Fixture channel photo" {
		t.Fatalf("unexpected title: %q", item.Title)
	}
	if item.Artist != "" {
		t.Fatalf("expected fixture caption without artist signal to leave artist empty, got %q", item.Artist)
	}
	if !item.PublishedAt.IsZero() {
		t.Fatalf("expected fixture caption without artwork year to leave published date empty, got %s", item.PublishedAt)
	}

	legacy := result.Items[1]
	if legacy.Source.CollectionID != "-1009876543210" {
		t.Fatalf("unexpected legacy collection ID: %q", legacy.Source.CollectionID)
	}
	if legacy.Source.CollectionName != "Legacy Fixture Channel" {
		t.Fatalf("unexpected legacy collection name: %q", legacy.Source.CollectionName)
	}
	if legacy.Source.ItemID != "88" || legacy.Source.ExternalID != "-1009876543210:88" {
		t.Fatalf("unexpected legacy source IDs: %+v", legacy.Source)
	}
	if legacy.Source.URL != "https://t.me/legacy_fixture/88" {
		t.Fatalf("unexpected legacy source URL: %s", legacy.Source.URL)
	}
	if legacy.DedupeKey.Value != "-1009876543210:88:document-unique-id" {
		t.Fatalf("unexpected legacy dedupe key: %s", legacy.DedupeKey.Value)
	}
	if legacy.Source.URL != "https://t.me/legacy_fixture/88" {
		t.Fatalf("unexpected legacy source URL: %s", legacy.Source.URL)
	}
	if legacy.Artist != "" {
		t.Fatalf("expected legacy fixture caption without artist signal to leave artist empty, got %q", legacy.Artist)
	}
}

func TestResolveMediaUsesGetFileFixture(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/botfixture-credential/getFile" {
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
		DedupeKey:  provider.DedupeKey{ProviderID: ProviderID, Value: "-1001234567890:42"},
		Source:     provider.SourceMetadata{ExternalID: "-1001234567890:42"},
		Media:      provider.MediaMetadata{ExternalID: "photo-large-file-id", MIMEType: "image/jpeg"},
	}

	resolved, err := connector.ResolveMedia(context.Background(), item)
	if err != nil {
		t.Fatalf("ResolveMedia returned error: %v", err)
	}

	expectedURL := server.URL + "/file/botfixture-credential/photos/file_42.jpg"
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
		BotToken:    "fixture-credential",
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
	if result.Items[0].Source.CollectionName != "Fixture Channel" {
		t.Fatalf("unexpected filtered item: %+v", result.Items[0].Source)
	}
}

func TestDiscoverPageFallsBackToBotMessageDedupe(t *testing.T) {
	item, ok := discoveredMedia(Message{
		MessageID: 12,
		Date:      1700000900,
		Chat: Chat{
			ID:        12345,
			Type:      "private",
			FirstName: "Fixture",
			LastName:  "User",
		},
		ForwardOrigin: &ForwardOrigin{
			Type:           "hidden_user",
			Date:           1700000800,
			SenderUserName: "Hidden Sender",
		},
		Document: &Document{
			FileID:       "hidden-document-file-id",
			FileUniqueID: "hidden-document-unique-id",
			FileName:     "hidden.png",
			MIMEType:     "image/png",
		},
	})
	if !ok {
		t.Fatal("expected media item")
	}
	if item.DedupeKey.Value != "12345:12:hidden-document-unique-id" {
		t.Fatalf("expected bot chat fallback dedupe key, got %q", item.DedupeKey.Value)
	}
	if item.Source.ExternalID != "12345:12" || item.Source.ItemID != "12" {
		t.Fatalf("unexpected fallback source IDs: %+v", item.Source)
	}
	if item.Source.CollectionName != "Hidden Sender" {
		t.Fatalf("expected hidden sender provenance, got %q", item.Source.CollectionName)
	}
	if item.Source.URL != "Hidden Sender" {
		t.Fatalf("expected source URL to be the channel/sender name, got %q", item.Source.URL)
	}
	if item.Artist != "" {
		t.Fatalf("expected missing caption to leave artist empty, got %q", item.Artist)
	}
}

func TestDiscoverPageUsesForwardOriginSenderChat(t *testing.T) {
	item, ok := discoveredMedia(Message{
		MessageID: 15,
		Date:      1700000900,
		Chat: Chat{
			ID:        12345,
			Type:      "private",
			FirstName: "Fixture",
			LastName:  "User",
		},
		ForwardOrigin: &ForwardOrigin{
			Type: "chat",
			Date: 1700000800,
			SenderChat: &Chat{
				ID:    -1002223334445,
				Type:  "supergroup",
				Title: "Origin Group",
			},
			AuthorSignature: "Origin Author",
		},
		Photo: []PhotoSize{{
			FileID:       "sender-chat-photo-file-id",
			FileUniqueID: "sender-chat-photo-unique-id",
			Width:        800,
			Height:       600,
		}},
	})
	if !ok {
		t.Fatal("expected media item")
	}
	if item.Source.CollectionID != "-1002223334445" {
		t.Fatalf("expected sender_chat collection ID, got %q", item.Source.CollectionID)
	}
	if item.Source.CollectionName != "Origin Group" {
		t.Fatalf("expected sender_chat collection name, got %q", item.Source.CollectionName)
	}
	if item.Source.ExternalID != "12345:15" {
		t.Fatalf("expected bot message fallback source ID without origin message ID, got %q", item.Source.ExternalID)
	}
	if item.DedupeKey.Value != "12345:15:sender-chat-photo-unique-id" {
		t.Fatalf("unexpected dedupe key: %q", item.DedupeKey.Value)
	}
	if item.Artist != "" {
		t.Fatalf("expected caption without artist signal to leave artist empty, got %q", item.Artist)
	}
}

func TestDiscoveredMediaParsesArtworkCaptionFields(t *testing.T) {
	const channelName = "Нимфы и Музы"

	tests := []struct {
		name        string
		caption     string
		wantTitle   string
		wantArtist  string
		wantYear    int
		wantMedium  string
		wantNoDate  bool
		wantNoTitle bool
	}{
		{
			name:       "artist first without artwork year",
			caption:    "Тереза Кондеминас Солер (Teresa Condeminas Soler, 1905 — 2003)\nУтро\n\nСекретный контент 🔞",
			wantTitle:  "Утро",
			wantArtist: "Тереза Кондеминас Солер (Teresa Condeminas Soler, 1905 — 2003)",
			wantNoDate: true,
		},
		{
			name:       "title first with artwork year",
			caption:    "Обнаженная на софе 1933 г.\n\nГовард Чандлер Кристи (США, 1873 - 1952)\n\nСекретный контент 🔞",
			wantTitle:  "Обнаженная на софе",
			wantArtist: "Говард Чандлер Кристи (США, 1873 - 1952)",
			wantYear:   1933,
		},
		{
			name:       "title medium artist",
			caption:    "\"Scinscape\" 1965 г.\n\nхолст, масло\n\nРальф Гоингс (США, 1928 - 2016)\n\nСекретный контент 🔞",
			wantTitle:  "Scinscape",
			wantArtist: "Ральф Гоингс (США, 1928 - 2016)",
			wantYear:   1965,
			wantMedium: "холст, масло",
		},
		{
			name:       "title containing medium keyword",
			caption:    "Дерево 1971 г.\n\nхолст, масло\n\nГовард Чандлер Кристи (США, 1873 - 1952)",
			wantTitle:  "Дерево",
			wantArtist: "Говард Чандлер Кристи (США, 1873 - 1952)",
			wantYear:   1971,
			wantMedium: "холст, масло",
		},
		{
			name:        "all junk",
			caption:     "Секретный контент 🔞\n\n",
			wantNoDate:  true,
			wantNoTitle: true,
		},
		{
			name:        "captionless",
			caption:     "",
			wantNoDate:  true,
			wantNoTitle: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			message := telegramArtworkMessage(channelName, tt.caption)
			item, ok := discoveredMedia(message)
			if !ok {
				t.Fatal("expected media item")
			}
			parsed := parseArtworkCaption(tt.caption)

			wantTitle := tt.wantTitle
			if tt.wantNoTitle {
				wantTitle = "fixture.jpg"
			}
			if item.Title != wantTitle {
				t.Fatalf("unexpected title: got %q want %q", item.Title, wantTitle)
			}
			if item.Artist != tt.wantArtist {
				t.Fatalf("unexpected artist: got %q want %q", item.Artist, tt.wantArtist)
			}
			if item.Source.CollectionName != channelName {
				t.Fatalf("expected source collection to be channel name, got %q", item.Source.CollectionName)
			}
			if item.Source.URL != channelName {
				t.Fatalf("expected source URL to be the channel name, got %q", item.Source.URL)
			}
			if strings.Contains(item.Title, "Секретный контент") || strings.Contains(item.Title, "🔞") ||
				strings.Contains(item.Artist, "Секретный контент") || strings.Contains(item.Artist, "🔞") ||
				strings.Contains(parsed.Medium, "Секретный контент") || strings.Contains(parsed.Medium, "🔞") {
				t.Fatalf("junk leaked into parsed fields: item=%+v medium=%q", item, parsed.Medium)
			}
			if parsed.Medium != tt.wantMedium {
				t.Fatalf("unexpected medium: got %q want %q", parsed.Medium, tt.wantMedium)
			}
			if tt.wantNoDate {
				if !item.PublishedAt.IsZero() {
					t.Fatalf("expected empty artwork date, got %s", item.PublishedAt)
				}
				return
			}
			wantDate := time.Date(tt.wantYear, 1, 1, 0, 0, 0, 0, time.UTC)
			if !item.PublishedAt.Equal(wantDate) {
				t.Fatalf("unexpected artwork date: got %s want %s", item.PublishedAt, wantDate)
			}
		})
	}
}

func telegramArtworkMessage(channelName string, caption string) Message {
	return Message{
		MessageID: 41,
		Date:      1700000900,
		Caption:   caption,
		Chat: Chat{
			ID:    -1001112223334,
			Type:  "channel",
			Title: channelName,
		},
		Document: &Document{
			FileID:       "fixture-file-id",
			FileUniqueID: "fixture-unique-id",
			FileName:     "fixture.jpg",
			MIMEType:     "image/jpeg",
		},
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
		BotToken:    "fixture-credential",
		BaseURL:     baseURL,
		FileBaseURL: strings.TrimRight(baseURL, "/") + "/file",
		Limit:       25,
		Retry:       retry.Config{MaxAttempts: 1},
	}, http.DefaultClient, zerolog.New(os.Stdout))
}
