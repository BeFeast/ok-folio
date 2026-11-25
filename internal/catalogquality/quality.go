package catalogquality

import (
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

var uuidTitlePattern = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

var blockedKeywords = map[string]struct{}{
	"favorite":  {},
	"favorites": {},
	"hidden":    {},
	"nonps":     {},
	"panorama":  {},
}

var protectedSubjectKeywords = map[string]struct{}{
	"black":  {},
	"blue":   {},
	"brown":  {},
	"cyan":   {},
	"gold":   {},
	"gray":   {},
	"green":  {},
	"grey":   {},
	"orange": {},
	"pink":   {},
	"purple": {},
	"red":    {},
	"silver": {},
	"violet": {},
	"white":  {},
	"yellow": {},
}

// NormalizeTitle returns an empty title for source junk while preserving real
// titles. Optional source file names let write paths drop filename-as-title
// values without coupling callers to filename parsing.
func NormalizeTitle(raw string, sourceFileNames ...string) string {
	title := strings.TrimSpace(raw)
	if title == "" || !hasLetterOrNumber(title) || isUUIDTitle(title) {
		return ""
	}

	titleKey := comparableFileTitle(title)
	for _, sourceFileName := range sourceFileNames {
		if titleKey != "" && titleKey == comparableFileName(sourceFileName) {
			return ""
		}
	}
	return title
}

// SanitizeKeywords trims, dedupes, and removes system labels plus obvious
// artist-name leakage while preserving user-facing subject keywords.
func SanitizeKeywords(artist string, raw []string) []string {
	artistTokens := artistKeywordTokens(artist)
	seen := make(map[string]struct{}, len(raw))
	cleaned := make([]string, 0, len(raw))
	for _, keyword := range raw {
		trimmed := strings.TrimSpace(keyword)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, blocked := blockedKeywords[key]; blocked {
			continue
		}
		if _, artistToken := artistTokens[key]; artistToken {
			continue
		}
		if _, duplicate := seen[key]; duplicate {
			continue
		}
		seen[key] = struct{}{}
		cleaned = append(cleaned, trimmed)
	}
	return cleaned
}

func hasLetterOrNumber(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			return true
		}
	}
	return false
}

func isUUIDTitle(s string) bool {
	return uuidTitlePattern.MatchString(strings.TrimSuffix(filepath.Base(s), filepath.Ext(s)))
}

func comparableFileTitle(s string) string {
	return strings.ToLower(strings.TrimSpace(filepath.Base(s)))
}

func comparableFileName(s string) string {
	return strings.ToLower(strings.TrimSpace(filepath.Base(s)))
}

func artistKeywordTokens(artist string) map[string]struct{} {
	tokens := make(map[string]struct{})
	for _, token := range strings.FieldsFunc(artist, func(r rune) bool {
		return unicode.IsSpace(r) || r == '_' || r == '.' || r == '-'
	}) {
		key := strings.ToLower(strings.TrimSpace(token))
		if key == "" {
			continue
		}
		if _, protected := protectedSubjectKeywords[key]; protected {
			continue
		}
		tokens[key] = struct{}{}
	}
	return tokens
}
