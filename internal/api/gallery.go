package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	okfcache "ok-folio/internal/cache"
	"ok-folio/internal/database"
	"ok-folio/internal/gallery"
)

type galleryProviderFacet struct {
	ID          string               `json:"id"`
	DisplayName string               `json:"display_name"`
	Count       int64                `json:"count"`
	Sources     []gallerySourceFacet `json:"sources"`
}

type gallerySourceFacet struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Count       int64  `json:"count"`
}

type galleryFacet struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Count       int64  `json:"count"`
}

type galleryFavoriteFacet struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Favorite    bool   `json:"favorite"`
	Count       int64  `json:"count"`
}

type galleryCatalogFacets struct {
	Sources    []gallerySourceFacet   `json:"sources"`
	Categories []galleryFacet         `json:"categories"`
	Artists    []galleryFacet         `json:"artists"`
	Favorites  []galleryFavoriteFacet `json:"favorites"`
}

type galleryCatalogResponse struct {
	Photos    []database.DownloadedPhoto `json:"photos"`
	Total     int64                      `json:"total"`
	Limit     int                        `json:"limit"`
	Offset    int                        `json:"offset"`
	Provider  string                     `json:"provider"`
	Source    string                     `json:"source"`
	Category  string                     `json:"category"`
	Artist    string                     `json:"artist"`
	Favorite  *bool                      `json:"favorite"`
	Query     string                     `json:"query"`
	Providers []galleryProviderFacet     `json:"providers"`
	Facets    galleryCatalogFacets       `json:"facets"`
}

type cacheQueryValue struct {
	Set   bool   `json:"set"`
	Value string `json:"value"`
}

type cacheGalleryCatalogFilters struct {
	Provider cacheQueryValue `json:"provider"`
	Source   cacheQueryValue `json:"source"`
	Category cacheQueryValue `json:"category"`
	Artist   cacheQueryValue `json:"artist"`
	Favorite cacheQueryValue `json:"favorite"`
	Query    cacheQueryValue `json:"query"`
}

type cacheGalleryCatalogETagShape struct {
	Version int                        `json:"version"`
	Filters cacheGalleryCatalogFilters `json:"filters"`
	Limit   int                        `json:"limit"`
	Offset  int                        `json:"offset"`
}

const catalogCacheControl = "private, no-cache, stale-while-revalidate=120"
const catalogCacheVersion = 2

func (s *Server) handleGalleryCatalog(w http.ResponseWriter, r *http.Request) {
	limit, offset := s.parsePagination(r)
	values := r.URL.Query()
	cacheFilters := cacheFiltersFromQuery(values)
	filters := database.GalleryCatalogFilters{
		Provider:  strings.TrimSpace(values.Get("provider")),
		Source:    strings.TrimSpace(values.Get("source")),
		Category:  strings.TrimSpace(values.Get("category")),
		Artist:    strings.TrimSpace(values.Get("artist")),
		ArtistSet: queryHasKey(values, "artist"),
		Favorite:  parseOptionalBool(values.Get("favorite")),
		Query:     strings.TrimSpace(values.Get("q")),
	}

	epoch := s.cache.Epoch(r.Context())
	canUseConditionalResponse := !s.cache.Passthrough()
	key, err := okfcache.CatalogKey(epoch, galleryCatalogCacheShape(cacheFilters), limit, offset)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to build gallery cache key")
		s.writeError(w, http.StatusInternalServerError, "Failed to fetch gallery catalog")
		return
	}
	etag, err := galleryCatalogETag(epoch, cacheFilters, limit, offset)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to build gallery ETag")
		s.writeError(w, http.StatusInternalServerError, "Failed to fetch gallery catalog")
		return
	}

	w.Header().Set("Cache-Control", catalogCacheControl)
	w.Header().Set("ETag", etag)
	if canUseConditionalResponse && requestETagMatches(r, etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	response, err := okfcache.GetOrCompute(r.Context(), s.cache, key, 2*time.Minute, func(ctx context.Context) (galleryCatalogResponse, error) {
		return s.galleryCatalogResponse(ctx, limit, offset, filters)
	})
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to fetch gallery catalog")
		s.writeError(w, http.StatusInternalServerError, "Failed to fetch gallery catalog")
		return
	}

	s.writeJSON(w, http.StatusOK, response)
}

func galleryCatalogETag(epoch int64, filters cacheGalleryCatalogFilters, limit int, offset int) (string, error) {
	hash, err := okfcache.FilterHash(cacheGalleryCatalogETagShape{
		Version: catalogCacheVersion,
		Filters: filters,
		Limit:   limit,
		Offset:  offset,
	})
	if err != nil {
		return "", err
	}
	return quoteETag(fmt.Sprintf("catalog-e%d-%s", epoch, hash)), nil
}

func galleryCatalogCacheShape(filters cacheGalleryCatalogFilters) any {
	return struct {
		Version int                        `json:"version"`
		Filters cacheGalleryCatalogFilters `json:"filters"`
	}{
		Version: catalogCacheVersion,
		Filters: filters,
	}
}

func (s *Server) galleryCatalogResponse(_ context.Context, limit int, offset int, filters database.GalleryCatalogFilters) (galleryCatalogResponse, error) {
	photos, total, err := s.db.GetGalleryCatalog(limit, offset, filters)
	if err != nil {
		return galleryCatalogResponse{}, err
	}

	sourceStats, err := s.db.GetGallerySourceStatsForFilters(filters)
	if err != nil {
		return galleryCatalogResponse{}, err
	}
	categoryStats, err := s.db.GetGalleryCategoryStatsForFilters(filters)
	if err != nil {
		return galleryCatalogResponse{}, err
	}
	artistStats, err := s.db.GetGalleryArtistStatsForFilters(filters)
	if err != nil {
		return galleryCatalogResponse{}, err
	}
	favoriteStats, err := s.db.GetGalleryFavoriteStatsForFilters(filters)
	if err != nil {
		return galleryCatalogResponse{}, err
	}

	providerFacets := galleryProviderFacets(sourceStats)

	return galleryCatalogResponse{
		Photos:    photos,
		Total:     total,
		Limit:     limit,
		Offset:    offset,
		Provider:  filters.Provider,
		Source:    filters.Source,
		Category:  filters.Category,
		Artist:    filters.Artist,
		Favorite:  filters.Favorite,
		Query:     filters.Query,
		Providers: providerFacets,
		Facets: galleryCatalogFacets{
			Sources:    flattenGallerySourceFacets(providerFacets),
			Categories: galleryCategoryFacets(categoryStats),
			Artists:    galleryFacets(artistStats),
			Favorites:  galleryFavoriteFacets(favoriteStats),
		},
	}, nil
}

func cacheFiltersFromQuery(values url.Values) cacheGalleryCatalogFilters {
	return cacheGalleryCatalogFilters{
		Provider: cacheQueryValue{Set: queryHasKey(values, "provider"), Value: strings.TrimSpace(values.Get("provider"))},
		Source:   cacheQueryValue{Set: queryHasKey(values, "source"), Value: strings.TrimSpace(values.Get("source"))},
		Category: cacheQueryValue{Set: queryHasKey(values, "category"), Value: strings.TrimSpace(values.Get("category"))},
		Artist:   cacheQueryValue{Set: queryHasKey(values, "artist"), Value: strings.TrimSpace(values.Get("artist"))},
		Favorite: cacheQueryValue{Set: queryHasKey(values, "favorite"), Value: strings.TrimSpace(values.Get("favorite"))},
		Query:    cacheQueryValue{Set: queryHasKey(values, "q"), Value: strings.TrimSpace(values.Get("q"))},
	}
}

func (s *Server) handleGalleryDecision(w http.ResponseWriter, r *http.Request) {
	decision := gallery.MVPDecision()
	if err := gallery.ValidateDecision(decision); err != nil {
		s.logger.Error().Err(err).Msg("Gallery decision is invalid")
		s.writeError(w, http.StatusInternalServerError, "Gallery decision is invalid")
		return
	}

	s.writeJSON(w, http.StatusOK, decision)
}

func galleryProviderFacets(sourceStats []database.GallerySourceStats) []galleryProviderFacet {
	byProvider := make(map[string]*galleryProviderFacet)

	for _, source := range sourceStats {
		providerID := providerIDFromSourcePage(source.SourcePage)
		provider, ok := byProvider[providerID]
		if !ok {
			provider = &galleryProviderFacet{
				ID:          providerID,
				DisplayName: providerDisplayName(providerID),
			}
			byProvider[providerID] = provider
		}

		provider.Count += source.Count
		provider.Sources = append(provider.Sources, gallerySourceFacet{
			ID:          source.SourcePage,
			DisplayName: sourceDisplayName(source.SourcePage),
			Count:       source.Count,
		})
	}

	providers := make([]galleryProviderFacet, 0, len(byProvider))
	for _, provider := range byProvider {
		sort.Slice(provider.Sources, func(i, j int) bool {
			if provider.Sources[i].Count == provider.Sources[j].Count {
				return provider.Sources[i].DisplayName < provider.Sources[j].DisplayName
			}
			return provider.Sources[i].Count > provider.Sources[j].Count
		})
		providers = append(providers, *provider)
	}

	sort.Slice(providers, func(i, j int) bool {
		if providers[i].Count == providers[j].Count {
			return providers[i].DisplayName < providers[j].DisplayName
		}
		return providers[i].Count > providers[j].Count
	})

	return providers
}

func flattenGallerySourceFacets(providers []galleryProviderFacet) []gallerySourceFacet {
	sources := make([]gallerySourceFacet, 0)
	for _, provider := range providers {
		sources = append(sources, provider.Sources...)
	}
	sort.Slice(sources, func(i, j int) bool {
		if sources[i].Count == sources[j].Count {
			return sources[i].DisplayName < sources[j].DisplayName
		}
		return sources[i].Count > sources[j].Count
	})
	return sources
}

func galleryCategoryFacets(stats []database.GalleryFacetStats) []galleryFacet {
	facets := make([]galleryFacet, 0, len(stats))
	for _, stat := range stats {
		facets = append(facets, galleryFacet{
			ID:          stat.ID,
			DisplayName: categoryDisplayName(stat.ID),
			Count:       stat.Count,
		})
	}
	sortGalleryFacets(facets)
	return facets
}

func galleryFacets(stats []database.GalleryFacetStats) []galleryFacet {
	facets := make([]galleryFacet, 0, len(stats))
	for _, stat := range stats {
		displayName := stat.ID
		if displayName == "" {
			displayName = "Unknown artist"
		}
		facets = append(facets, galleryFacet{
			ID:          stat.ID,
			DisplayName: displayName,
			Count:       stat.Count,
		})
	}
	sortGalleryFacets(facets)
	return facets
}

func galleryFavoriteFacets(stats []database.GalleryFavoriteStats) []galleryFavoriteFacet {
	facets := make([]galleryFavoriteFacet, 0, len(stats))
	for _, stat := range stats {
		id := "false"
		displayName := "Not favorites"
		if stat.Favorite {
			id = "true"
			displayName = "Favorites"
		}
		facets = append(facets, galleryFavoriteFacet{
			ID:          id,
			DisplayName: displayName,
			Favorite:    stat.Favorite,
			Count:       stat.Count,
		})
	}
	sort.Slice(facets, func(i, j int) bool {
		return facets[i].ID > facets[j].ID
	})
	return facets
}

func sortGalleryFacets(facets []galleryFacet) {
	sort.Slice(facets, func(i, j int) bool {
		if facets[i].Count == facets[j].Count {
			return facets[i].DisplayName < facets[j].DisplayName
		}
		return facets[i].Count > facets[j].Count
	})
}

func providerIDFromSourcePage(sourcePage string) string {
	if sourcePage == "" {
		return "unknown"
	}
	parsed, err := url.Parse(sourcePage)
	if err != nil || parsed.Hostname() == "" {
		return sourcePage
	}
	return strings.TrimPrefix(parsed.Hostname(), "www.")
}

func providerDisplayName(providerID string) string {
	if providerID == "unknown" {
		return "Unknown source"
	}
	return providerID
}

func sourceDisplayName(sourcePage string) string {
	if sourcePage == "" {
		return "Unknown source"
	}
	parsed, err := url.Parse(sourcePage)
	if err != nil || parsed.Hostname() == "" {
		return sourcePage
	}
	path := strings.Trim(parsed.EscapedPath(), "/")
	if path == "" {
		return strings.TrimPrefix(parsed.Hostname(), "www.")
	}
	return strings.TrimPrefix(parsed.Hostname(), "www.") + "/" + path
}

func categoryDisplayName(category string) string {
	if category == "" || category == "unknown" {
		return "Unknown category"
	}
	return "Category " + category
}

func galleryCategoryIDFromSourcePage(sourcePage string) string {
	if sourcePage == "" {
		return "unknown"
	}
	parsed, err := url.Parse(sourcePage)
	if err != nil {
		return "unknown"
	}

	parts := strings.Split(strings.Trim(parsed.EscapedPath(), "/"), "/")
	for i := 0; i < len(parts)-1; i++ {
		if strings.EqualFold(parts[i], "category") && parts[i+1] != "" {
			return parts[i+1]
		}
	}

	query := parsed.Query()
	for _, key := range []string{"category", "category_id", "cat"} {
		if value := strings.TrimSpace(query.Get(key)); value != "" {
			return value
		}
	}

	return "unknown"
}

func parseOptionalBool(value string) *bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "1", "yes":
		parsed := true
		return &parsed
	case "false", "0", "no":
		parsed := false
		return &parsed
	default:
		return nil
	}
}

func queryHasKey(values url.Values, key string) bool {
	_, ok := values[key]
	return ok
}
