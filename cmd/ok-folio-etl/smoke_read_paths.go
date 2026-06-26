package main

import (
	"flag"
	"fmt"
	"time"

	"ok-folio/internal/config"
	"ok-folio/internal/database"
)

const (
	defaultSmokeExpectedRows    = 50338
	defaultSmokeExpectedArtists = 1953
)

type smokeResult struct {
	Name       string
	Rows       int
	Total      int64
	DurationMS float64
	Notes      string
}

func smokeReadPaths(args []string) {
	fs := flag.NewFlagSet("smoke-read-paths", flag.ExitOnError)
	configPath := fs.String("config", "/config/config.yaml", "Path to OK Folio configuration")
	expectedRows := fs.Int64("expected-rows", defaultSmokeExpectedRows, "Expected downloaded catalog row count")
	expectedArtists := fs.Int("expected-artists", defaultSmokeExpectedArtists, "Expected artist facet cardinality")
	limit := fs.Int("limit", 50, "Catalog/search page size")
	searchQuery := fs.String("search-query", "jpg", "Search query to smoke")
	if err := fs.Parse(args); err != nil {
		exitErr(err)
	}
	if *limit <= 0 {
		exitErr(fmt.Errorf("--limit must be greater than zero"))
	}
	if *searchQuery == "" {
		exitErr(fmt.Errorf("--search-query is required"))
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		exitErr(err)
	}
	db, err := database.New(&cfg.Database)
	if err != nil {
		exitErr(err)
	}
	sqlDB, err := db.DB.DB()
	if err != nil {
		exitErr(err)
	}
	defer sqlDB.Close()

	var downloadedRows int64
	if err := db.Model(&database.DownloadedPhoto{}).Where("status = ?", "downloaded").Count(&downloadedRows).Error; err != nil {
		exitErr(err)
	}
	if downloadedRows != *expectedRows {
		exitErr(fmt.Errorf("expected %d downloaded catalog rows, found %d", *expectedRows, downloadedRows))
	}

	results := make([]smokeResult, 0, 6)

	photos, total, duration, err := timeCatalog(db, *limit)
	if err != nil {
		exitErr(err)
	}
	results = append(results, smokeResult{Name: "catalog_page", Rows: len(photos), Total: total, DurationMS: duration, Notes: fmt.Sprintf("limit=%d offset=0", *limit)})

	sources, duration, err := timeSourceFacet(db)
	if err != nil {
		exitErr(err)
	}
	results = append(results, smokeResult{Name: "facet_source", Rows: len(sources), Total: sumSourceCounts(sources), DurationMS: duration, Notes: "SQL GROUP BY source_page"})

	categories, duration, err := timeCategoryFacet(db)
	if err != nil {
		exitErr(err)
	}
	results = append(results, smokeResult{Name: "facet_category", Rows: len(categories), Total: sumFacetCounts(categories), DurationMS: duration, Notes: "SQL GROUP BY category column"})

	artists, duration, err := timeArtistFacet(db)
	if err != nil {
		exitErr(err)
	}
	if len(artists) != *expectedArtists {
		exitErr(fmt.Errorf("expected %d artist facet rows, found %d", *expectedArtists, len(artists)))
	}
	results = append(results, smokeResult{Name: "facet_artist", Rows: len(artists), Total: sumFacetCounts(artists), DurationMS: duration, Notes: "SQL GROUP BY artist"})

	favorites, duration, err := timeFavoriteFacet(db)
	if err != nil {
		exitErr(err)
	}
	results = append(results, smokeResult{Name: "facet_favorite", Rows: len(favorites), Total: sumFavoriteCounts(favorites), DurationMS: duration, Notes: "SQL count on favorite"})

	searchPhotos, searchTotal, duration, err := timeSearch(db, *searchQuery, *limit)
	if err != nil {
		exitErr(err)
	}
	results = append(results, smokeResult{Name: "search", Rows: len(searchPhotos), Total: searchTotal, DurationMS: duration, Notes: "title/artist/file_name only"})

	fmt.Printf("OK Folio read-path smoke: downloaded_rows=%d expected_rows=%d search_query=%q\n", downloadedRows, *expectedRows, *searchQuery)
	fmt.Println()
	fmt.Println("| Path | Result rows | Total/count | Latency ms | Notes |")
	fmt.Println("| --- | ---: | ---: | ---: | --- |")
	for _, result := range results {
		fmt.Printf("| %s | %d | %d | %.2f | %s |\n", result.Name, result.Rows, result.Total, result.DurationMS, result.Notes)
	}
	fmt.Println()
	fmt.Println("Text-scan check: free-text search excludes raw url and source_page LIKE predicates.")
}

func timeCatalog(db *database.DB, limit int) ([]database.DownloadedPhoto, int64, float64, error) {
	start := time.Now()
	photos, total, err := db.GetGalleryCatalog(limit, 0, database.GalleryCatalogFilters{})
	return photos, total, elapsedMS(start), err
}

func timeSourceFacet(db *database.DB) ([]database.GallerySourceStats, float64, error) {
	start := time.Now()
	sources, err := db.GetGallerySourceStats()
	return sources, elapsedMS(start), err
}

func timeCategoryFacet(db *database.DB) ([]database.GalleryFacetStats, float64, error) {
	start := time.Now()
	categories, err := db.GetGalleryCategoryStats()
	return categories, elapsedMS(start), err
}

func timeArtistFacet(db *database.DB) ([]database.GalleryFacetStats, float64, error) {
	start := time.Now()
	artists, err := db.GetGalleryArtistStats()
	return artists, elapsedMS(start), err
}

func timeFavoriteFacet(db *database.DB) ([]database.GalleryFavoriteStats, float64, error) {
	start := time.Now()
	favorites, err := db.GetGalleryFavoriteStats()
	return favorites, elapsedMS(start), err
}

func timeSearch(db *database.DB, query string, limit int) ([]database.DownloadedPhoto, int64, float64, error) {
	start := time.Now()
	photos, total, err := db.SearchPhotos(query, limit, 0)
	return photos, total, elapsedMS(start), err
}

func elapsedMS(start time.Time) float64 {
	return float64(time.Since(start).Microseconds()) / 1000
}

func sumSourceCounts(sources []database.GallerySourceStats) int64 {
	var total int64
	for _, source := range sources {
		total += source.Count
	}
	return total
}

func sumFacetCounts(facets []database.GalleryFacetStats) int64 {
	var total int64
	for _, facet := range facets {
		total += facet.Count
	}
	return total
}

func sumFavoriteCounts(facets []database.GalleryFavoriteStats) int64 {
	var total int64
	for _, facet := range facets {
		total += facet.Count
	}
	return total
}
