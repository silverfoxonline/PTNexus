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

func NormalizeSeedTagsForReview(value any) []string {
	result := []string{}
	for _, raw := range parseSeedTagValues(value) {
		tag := normalizeSeedTagForReview(raw)
		if tag == "" {
			continue
		}
		exists := false
		for _, current := range result {
			if current == tag {
				exists = true
				break
			}
		}
		if !exists {
			result = append(result, tag)
		}
	}
	return result
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

func parseSeedTagValues(value any) []string {
	if typed, ok := value.([]byte); ok {
		return parseSeedTagValues(string(typed))
	}
	return ParseStringArray(value)
}

func normalizeSeedTagForReview(raw string) string {
	tag := strings.TrimSpace(raw)
	if tag == "" {
		return ""
	}
	if shouldIgnoreSeedTagForReview(tag) {
		return ""
	}
	if len(tag) >= 4 && strings.EqualFold(tag[:4], "tag.") {
		rest := strings.TrimSpace(tag[4:])
		if rest == "" {
			return ""
		}
		if shouldIgnoreSeedTagForReview(rest) {
			return ""
		}
		return "tag." + rest
	}
	switch {
	case strings.Contains(tag, "中字"):
		return "tag.中字"
	case strings.Contains(tag, "国配") || strings.Contains(tag, "国语"):
		return "tag.国语"
	case strings.Contains(tag, "合集"):
		return "tag.合集"
	case strings.Contains(tag, "完结"):
		return "tag.完结"
	case strings.Contains(tag, "禁转"):
		return "tag.禁转"
	case strings.Contains(tag, "限转"):
		return "tag.限转"
	case strings.Contains(tag, "特效"):
		return "tag.特效"
	case strings.Contains(tag, "自译"):
		return "tag.自译"
	case strings.Contains(tag, "原生"):
		return "tag.原生"
	case strings.EqualFold(tag, "DIY"):
		return "tag.DIY"
	default:
		return "tag." + tag
	}
}

func shouldIgnoreSeedTagForReview(tag string) bool {
	trimmed := strings.TrimSpace(tag)
	if trimmed == "" {
		return true
	}
	for _, ignored := range []string{"官方", "官种", "官字", "官字组", "首发", "自购", "自抓", "应求", "高码"} {
		if strings.EqualFold(trimmed, ignored) {
			return true
		}
	}
	return false
}
