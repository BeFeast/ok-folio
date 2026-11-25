package catalogquality

import (
	"reflect"
	"testing"
)

func TestNormalizeTitleDropsJunk(t *testing.T) {
	tests := []struct {
		name      string
		raw       string
		fileNames []string
		want      string
	}{
		{name: "empty", raw: " \t ", want: ""},
		{name: "punctuation", raw: "***", want: ""},
		{name: "dash", raw: "—", want: ""},
		{name: "middle dot", raw: "·", want: ""},
		{name: "uuid", raw: "550e8400-e29b-41d4-a716-446655440000", want: ""},
		{name: "uuid filename", raw: "550e8400-e29b-41d4-a716-446655440000.jpg", want: ""},
		{name: "source filename", raw: "fixture-photo.jpg", fileNames: []string{"fixture-photo.jpg"}, want: ""},
		{name: "source filename stem kept", raw: "Fixture Photo", fileNames: []string{"fixture-photo.jpg"}, want: "Fixture Photo"},
		{name: "real title", raw: "  Portrait Study  ", want: "Portrait Study"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeTitle(tt.raw, tt.fileNames...); got != tt.want {
				t.Fatalf("NormalizeTitle(%q) = %q, want %q", tt.raw, got, tt.want)
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

func TestSanitizeKeywordsPreservesOrderAndDedupe(t *testing.T) {
	raw := []string{" portrait ", "Portrait", "hidden", "Gold", "gold"}
	want := []string{"portrait", "Gold"}
	if got := SanitizeKeywords("", raw); !reflect.DeepEqual(got, want) {
		t.Fatalf("SanitizeKeywords() = %#v, want %#v", got, want)
	}
}

func TestSanitizeKeywordsKeepsProtectedSubjectArtistTokens(t *testing.T) {
	raw := []string{"blue", "artist", "portrait"}
	want := []string{"blue", "portrait"}
	if got := SanitizeKeywords("Blue Artist", raw); !reflect.DeepEqual(got, want) {
		t.Fatalf("SanitizeKeywords() = %#v, want %#v", got, want)
	}
}
