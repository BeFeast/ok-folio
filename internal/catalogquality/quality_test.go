package catalogquality

import (
	"reflect"
	"testing"
)

func TestNormalizeTitleDropsJunk(t *testing.T) {
	tests := []struct {
		name        string
		raw         string
		sourceNames []string
		want        string
	}{
		{name: "empty", raw: " \t ", want: ""},
		{name: "asterisks", raw: "***", want: ""},
		{name: "em dash", raw: "—", want: ""},
		{name: "middle dot", raw: " · ", want: ""},
		{name: "canonical uuid", raw: "550e8400-e29b-41d4-a716-446655440000", want: ""},
		{name: "compact uuid", raw: "550e8400e29b41d4a716446655440000", want: ""},
		{name: "filename with extension", raw: "550e8400-e29b-41d4-a716-446655440000.jpg", sourceNames: []string{"550e8400-e29b-41d4-a716-446655440000.jpg"}, want: ""},
		{name: "filename stem", raw: "piece-001", sourceNames: []string{"/imports/piece-001.jpg"}, want: ""},
		{name: "real title", raw: "  Portrait Study  ", sourceNames: []string{"portrait.jpg"}, want: "Portrait Study"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeTitle(tt.raw, tt.sourceNames...); got != tt.want {
				t.Fatalf("NormalizeTitle() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSanitizeKeywords(t *testing.T) {
	raw := []string{"vlad", "gansovsky", "nonps", "portrait", "favorites"}
	want := []string{"portrait"}

	if got := SanitizeKeywords("Vlad Gansovsky", raw); !reflect.DeepEqual(got, want) {
		t.Fatalf("SanitizeKeywords() = %#v, want %#v", got, want)
	}
}

func TestSanitizeKeywordsTrimsAndDedupesCaseInsensitively(t *testing.T) {
	raw := []string{" Gene ", "portrait", "Portrait", "", "hidden", "study"}
	want := []string{"portrait", "study"}

	if got := SanitizeKeywords("Gene Davis", raw); !reflect.DeepEqual(got, want) {
		t.Fatalf("SanitizeKeywords() = %#v, want %#v", got, want)
	}
}
