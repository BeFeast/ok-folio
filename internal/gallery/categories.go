package gallery

import "strings"

// CategoryFacet is a stable category ID and display label exposed by gallery facets.
type CategoryFacet struct {
	ID          string
	DisplayName string
}

// MediumCategories are the stable category IDs that power the mobile medium filters.
var MediumCategories = []CategoryFacet{
	{ID: "painting", DisplayName: "Painting"},
	{ID: "photography", DisplayName: "Photography"},
	{ID: "drawing", DisplayName: "Drawing"},
	{ID: "print", DisplayName: "Print"},
	{ID: "sculpture", DisplayName: "Sculpture"},
}

// NormalizeMediumCategory maps a category value or display label to a stable medium ID.
func NormalizeMediumCategory(category string) (string, string, bool) {
	normalized := strings.ToLower(strings.TrimSpace(category))
	for _, medium := range MediumCategories {
		if normalized == medium.ID || normalized == strings.ToLower(medium.DisplayName) {
			return medium.ID, medium.DisplayName, true
		}
	}
	return "", "", false
}
