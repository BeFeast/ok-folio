package catalogquality

import (
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

var (
	canonicalUUID = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	compactUUID   = regexp.MustCompile(`(?i)^[0-9a-f]{32}$`)
	artistSplit   = regexp.MustCompile(`[\s_.-]+`)
)

var systemKeywordBlocklist = map[string]struct{}{
	"favorite":  {},
	"favorites": {},
	"hidden":    {},
	"nonps":     {},
	"panorama":  {},
}

// NormalizeTitle trims source titles and returns an empty title for known junk.
func NormalizeTitle(raw string, sourceNames ...string) string {
	title := strings.TrimSpace(raw)
	if title == "" || isPunctuationOnly(title) || isUUID(title) {
		return ""
	}
	titleKey := strings.ToLower(title)
	for _, sourceName := range sourceNames {
		for _, candidate := range filenameTitleCandidates(sourceName) {
			if titleKey == strings.ToLower(candidate) {
				return ""
			}
		}
	}
	return title
}

// SanitizeKeywords trims, blocklists, removes artist-name tokens, and dedupes keywords.
func SanitizeKeywords(artist string, raw []string) []string {
	artistTokens := artistKeywordTokens(artist)
	seen := make(map[string]struct{}, len(raw))
	out := make([]string, 0, len(raw))
	for _, keyword := range raw {
		trimmed := strings.TrimSpace(keyword)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, blocked := systemKeywordBlocklist[key]; blocked {
			continue
		}
		if _, artistToken := artistTokens[key]; artistToken {
			continue
		}
		if _, duplicate := seen[key]; duplicate {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func isPunctuationOnly(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func isUUID(s string) bool {
	return canonicalUUID.MatchString(s) || compactUUID.MatchString(s)
}

func filenameTitleCandidates(sourceName string) []string {
	base := strings.TrimSpace(filepath.Base(sourceName))
	if base == "" || base == "." || base == string(filepath.Separator) {
		return nil
	}
	candidates := []string{base}
	if ext := filepath.Ext(base); ext != "" {
		stem := strings.TrimSuffix(base, ext)
		if stem != "" {
			candidates = append(candidates, stem)
		}
	}
	return candidates
}

func artistKeywordTokens(artist string) map[string]struct{} {
	tokens := make(map[string]struct{})
	for _, token := range artistSplit.Split(strings.TrimSpace(artist), -1) {
		token = strings.ToLower(strings.TrimSpace(token))
		if token != "" {
			tokens[token] = struct{}{}
		}
	}
	return tokens
}
