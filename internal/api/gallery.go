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

func (s *Server) handleGalleryCatalog(w http.ResponseWriter, r *http.Request) {
	limit, offset := s.parsePagination(r)
	filters := database.GalleryCatalogFilters{
		Provider: strings.TrimSpace(r.URL.Query().Get("provider")),
		Source:   strings.TrimSpace(r.URL.Query().Get("source")),
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

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"photos":    photos,
		"total":     total,
		"limit":     limit,
		"offset":    offset,
		"provider":  filters.Provider,
		"source":    filters.Source,
		"providers": galleryProviderFacets(sourceStats),
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
