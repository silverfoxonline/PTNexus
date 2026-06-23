package persist

import (
	"regexp"
	"strings"
)

var reSeedSubtitleRatingTail = regexp.MustCompile(`\s+(?:\d+(?:\.\d+)?\s*/\s*10\s*){1,2}(?:\d+\s*%\s*)?(?:\d+\s*/\s*100\s*)?$`)

func NormalizeSeedSubtitle(value string) string {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return ""
	}
	normalized = strings.ReplaceAll(normalized, "\u3010", "[")
	normalized = strings.ReplaceAll(normalized, "\u3011", "]")
	normalized = reSeedSubtitleRatingTail.ReplaceAllString(normalized, "")
	return strings.TrimSpace(normalized)
}

func NormalizeSeedTagsForReview(_ any) []string {
	return []string{}
}
