package dataquality

import (
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

var uuidTitlePattern = regexp.MustCompile(`(?i)^(?:[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}|[0-9a-f]{32})$`)

var systemKeywordBlocklist = map[string]struct{}{
	"favorite":  {},
	"favorites": {},
	"hidden":    {},
	"nonps":     {},
	"panorama":  {},
}

// NormalizeTitle returns an empty title for placeholders and source-derived junk.
func NormalizeTitle(raw string, sourceNames ...string) string {
	title := strings.TrimSpace(raw)
	if title == "" || isPunctuationOnly(title) || isUUIDLike(title) {
		return ""
	}

	titleKey := comparableName(title)
	for _, sourceName := range sourceNames {
		if titleKey == "" {
			break
		}
		if sourceMatchesTitle(titleKey, sourceName) {
			return ""
		}
	}

	return title
}

// SanitizeKeywords removes source/system tokens while preserving useful order.
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

func isUUIDLike(s string) bool {
	return uuidTitlePattern.MatchString(strings.TrimSpace(s))
}

func sourceMatchesTitle(titleKey string, sourceName string) bool {
	sourceKey := comparableName(sourceName)
	if sourceKey == "" {
		return false
	}
	return titleKey == sourceKey || titleKey == strings.TrimSuffix(sourceKey, filepath.Ext(sourceKey))
}

func comparableName(s string) string {
	normalized := strings.ReplaceAll(strings.TrimSpace(s), `\`, `/`)
	base := filepath.Base(normalized)
	base = strings.TrimSpace(base)
	if base == "." || base == string(filepath.Separator) {
		return ""
	}
	return strings.ToLower(base)
}

func artistKeywordTokens(artist string) map[string]struct{} {
	tokens := strings.FieldsFunc(artist, func(r rune) bool {
		return unicode.IsSpace(r) || r == '_' || r == '.' || r == '-'
	})
	out := make(map[string]struct{}, len(tokens))
	for _, token := range tokens {
		token = strings.TrimFunc(token, func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsDigit(r)
		})
		token = strings.ToLower(token)
		if token == "" {
			continue
		}
		out[token] = struct{}{}
	}
	return out
}
