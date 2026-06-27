package provider

import (
	"context"
	"fmt"
	"time"
)

// Connector discovers provider-owned media and resolves provider metadata.
type Connector interface {
	Provider() Source
	DiscoverPage(ctx context.Context, req PageRequest) (*PageResult, error)
	ResolveMedia(ctx context.Context, item DiscoveredMedia) (*DiscoveredMedia, error)
}

// Source identifies the provider and the user-visible source scope.
type Source struct {
	ID          string
	DisplayName string
	BaseURL     string
	Scope       string
}

// PageRequest describes a provider page or cursor to discover.
type PageRequest struct {
	Page   int
	Cursor string
}

// PageResult contains media found on a single provider page.
type PageResult struct {
	Items      []DiscoveredMedia
	Pagination Pagination
}

// Pagination carries page and cursor state without assuming one strategy.
type Pagination struct {
	Page       int
	NextPage   int
	NextCursor string
	HasNext    bool
}

// DiscoveredMedia is the provider-neutral media item model.
type DiscoveredMedia struct {
	ProviderID  string
	DedupeKey   DedupeKey
	Source      SourceMetadata
	Media       MediaMetadata
	Title       string
	Artist      string
	PublishedAt time.Time
}

// DedupeKey is the stable provider-specific key used to avoid reprocessing.
type DedupeKey struct {
	ProviderID string
	Value      string
}

func (k DedupeKey) String() string {
	if k.ProviderID == "" {
		return k.Value
	}
	return fmt.Sprintf("%s:%s", k.ProviderID, k.Value)
}

// SourceMetadata points back to the provider page or message that yielded media.
type SourceMetadata struct {
	URL            string
	ExternalID     string
	CollectionID   string
	CollectionName string
	ItemID         string
}

// MediaMetadata describes the downloadable representation selected by a connector.
type MediaMetadata struct {
	URL        string
	MIMEType   string
	FileName   string
	ExternalID string
	// ContentHash is a raw exact-content hash when the connector can provide or
	// compute it before the catalog insert path.
	ContentHash []byte
}

// ErrorKind classifies provider failures for retry and operator handling.
type ErrorKind string

const (
	ErrorKindTemporary    ErrorKind = "temporary"
	ErrorKindRateLimit    ErrorKind = "rate_limit"
	ErrorKindNotFound     ErrorKind = "not_found"
	ErrorKindParse        ErrorKind = "parse"
	ErrorKindPermission   ErrorKind = "permission"
	ErrorKindMissingMedia ErrorKind = "missing_media"
)

// ProviderError wraps a connector error with provider-level handling metadata.
type ProviderError struct {
	ProviderID string
	Kind       ErrorKind
	RetryAfter time.Duration
	Err        error
}

func (e *ProviderError) Error() string {
	if e.Err == nil {
		return string(e.Kind)
	}
	return e.Err.Error()
}

func (e *ProviderError) Unwrap() error {
	return e.Err
}

func (e *ProviderError) Retryable() bool {
	return e.Kind == ErrorKindTemporary || e.Kind == ErrorKindRateLimit
}
