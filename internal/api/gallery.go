package api

import (
	"net/http"
	"net/url"
	"sort"
	"strings"

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

func (s *Server) handleGalleryCatalog(w http.ResponseWriter, r *http.Request) {
	limit, offset := s.parsePagination(r)
	filters := database.GalleryCatalogFilters{
		Provider: strings.TrimSpace(r.URL.Query().Get("provider")),
		Source:   strings.TrimSpace(r.URL.Query().Get("source")),
		Category: strings.TrimSpace(r.URL.Query().Get("category")),
		Artist:   strings.TrimSpace(r.URL.Query().Get("artist")),
		Favorite: parseOptionalBool(r.URL.Query().Get("favorite")),
	}

	photos, total, err := s.db.GetGalleryCatalog(limit, offset, filters)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to fetch gallery catalog")
		s.writeError(w, http.StatusInternalServerError, "Failed to fetch gallery catalog")
		return
	}

	sourceStats, err := s.db.GetGallerySourceStats()
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to fetch gallery source facets")
		s.writeError(w, http.StatusInternalServerError, "Failed to fetch gallery sources")
		return
	}
	categoryStats, err := s.db.GetGalleryCategoryStats()
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to fetch gallery category facets")
		s.writeError(w, http.StatusInternalServerError, "Failed to fetch gallery categories")
		return
	}
	artistStats, err := s.db.GetGalleryArtistStats()
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to fetch gallery artist facets")
		s.writeError(w, http.StatusInternalServerError, "Failed to fetch gallery artists")
		return
	}
	favoriteStats, err := s.db.GetGalleryFavoriteStats()
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to fetch gallery favorite facets")
		s.writeError(w, http.StatusInternalServerError, "Failed to fetch gallery favorites")
		return
	}

	providerFacets := galleryProviderFacets(sourceStats)

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"photos":    photos,
		"total":     total,
		"limit":     limit,
		"offset":    offset,
		"provider":  filters.Provider,
		"source":    filters.Source,
		"category":  filters.Category,
		"artist":    filters.Artist,
		"favorite":  filters.Favorite,
		"providers": providerFacets,
		"facets": galleryCatalogFacets{
			Sources:    flattenGallerySourceFacets(providerFacets),
			Categories: galleryCategoryFacets(categoryStats),
			Artists:    galleryFacets(artistStats),
			Favorites:  galleryFavoriteFacets(favoriteStats),
		},
	})
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
