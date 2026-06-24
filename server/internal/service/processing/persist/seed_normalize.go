package persist

import (
	"regexp"
	"strings"
)

var reSeedSubtitleRatingTail = regexp.MustCompile(`\s+(?:\d+(?:\.\d+)?\s*/\s*10\s*){1,2}(?:\d+\s*%\s*)?(?:\d+\s*/\s*100\s*)?$`)
var reSeedMediaInfoExtraBlankLine = regexp.MustCompile(`(?m)^((?:General|Video|Audio|Text(?:\s*#\d+)?|Menu|Chapters))\r?\n\s*\r?\n`)

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

func NormalizeSeedMediaInfo(value string) string {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return ""
	}
	normalized = strings.ReplaceAll(normalized, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	normalized = reSeedMediaInfoExtraBlankLine.ReplaceAllString(normalized, "$1\n")
	return strings.TrimSpace(normalized)
}
