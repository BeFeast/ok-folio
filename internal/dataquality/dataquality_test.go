package dataquality

import (
	"reflect"
	"testing"
)

func TestNormalizeTitleDropsJunk(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		sources []string
		want    string
	}{
		{name: "empty", raw: " \t ", want: ""},
		{name: "asterisks", raw: "***", want: ""},
		{name: "dash", raw: "—", want: ""},
		{name: "middle dot", raw: "·", want: ""},
		{name: "uuid", raw: "550e8400-e29b-41d4-a716-446655440000", want: ""},
		{name: "compact uuid", raw: "550e8400e29b41d4a716446655440000", want: ""},
		{name: "source filename", raw: "piece.jpg", sources: []string{"piece.jpg"}, want: ""},
		{name: "source filename without extension", raw: "piece", sources: []string{"/downloads/piece.jpg"}, want: ""},
		{name: "windows source filename without extension", raw: "piece", sources: []string{`C:\downloads\piece.jpg`}, want: ""},
		{name: "keeps real title", raw: "  Portrait in Blue  ", sources: []string{"portrait.jpg"}, want: "Portrait in Blue"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeTitle(tt.raw, tt.sources...); got != tt.want {
				t.Fatalf("NormalizeTitle() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSanitizeKeywordsDropsArtistAndSystemTokens(t *testing.T) {
	got := SanitizeKeywords("Vlad Gansovsky", []string{"vlad", "gansovsky", "nonps", "portrait", "favorites"})
	want := []string{"portrait"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SanitizeKeywords() = %#v, want %#v", got, want)
	}
}

func TestSanitizeKeywordsTrimsDedupesAndSplitsArtist(t *testing.T) {
	got := SanitizeKeywords("Gene_Zachar.Example-Name", []string{" Gene ", "gold", "GOLD", "", "hidden", "zachar", "study"})
	want := []string{"gold", "study"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SanitizeKeywords() = %#v, want %#v", got, want)
	}
}
