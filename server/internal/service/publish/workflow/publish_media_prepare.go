package workflow

import (
	"fmt"
	neturl "net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	processingmedia "github.com/pt-nexus/server/internal/service/processing/media"
	processingrepair "github.com/pt-nexus/server/internal/service/processing/repair"
)

var rePublishEpisodeOne = regexp.MustCompile(`(?i)(?:^|[ ._\-\[\(])(?:s\d{1,2})?e0?1(?:[ ._\-\]\)]|$)`)

type publishMediaPrepareResult struct {
	MediaInfo string
	Logs      []string
}

func preparePublishMediaForTarget(uploadData map[string]any, payload map[string]any, torrentPath, savePath, downloaderID, currentMediaInfo string) (publishMediaPrepareResult, error) {
	result := publishMediaPrepareResult{MediaInfo: strings.TrimSpace(currentMediaInfo)}
	if uploadData == nil {
		uploadData = map[string]any{}
	}

	screenshots, screenshotErr := preparePublishScreenshots(uploadData, payload, torrentPath, savePath, downloaderID)
	if screenshotErr != nil {
		return result, screenshotErr
	}
	setPublishUploadSection(uploadData, "screenshots", screenshots)
	result.Logs = append(result.Logs, "截图已统一为 PNG 并使用白名单图床链接")

	if !isPublishSeries(uploadData, payload) {
		return result, nil
	}
	if strings.TrimSpace(result.MediaInfo) != "" {
		return result, nil
	}
	mediaInfo, mediaErr := preparePublishEpisodeOneMediaInfo(uploadData, payload, savePath)
	if mediaErr != nil {
		return result, mediaErr
	}
	if strings.TrimSpace(mediaInfo) != "" {
		uploadData["mediainfo"] = strings.TrimSpace(mediaInfo)
		result.MediaInfo = strings.TrimSpace(mediaInfo)
		result.Logs = append(result.Logs, "剧集 MediaInfo 已重新提取为 E01")
	}
	return result, nil
}

func preparePublishScreenshots(uploadData map[string]any, payload map[string]any, torrentPath, savePath, downloaderID string) (string, error) {
	raw := pickPublishUploadSection(uploadData, "screenshots")
	if urls := normalizePublishWhitelistedPNGURLs(extractPublishImageURLs(raw)); len(urls) > 0 {
		return processingrepair.ToBBCodeImages(urls), nil
	}

	nextPayload := map[string]any{}
	for key, value := range payload {
		nextPayload[key] = value
	}
	if strings.TrimSpace(savePath) != "" {
		nextPayload["savePath"] = strings.TrimSpace(savePath)
		nextPayload["save_path"] = strings.TrimSpace(savePath)
	}
	if strings.TrimSpace(downloaderID) != "" {
		nextPayload["downloaderId"] = strings.TrimSpace(downloaderID)
		nextPayload["downloader_id"] = strings.TrimSpace(downloaderID)
	}
	if strings.TrimSpace(firstPublishPayloadString(nextPayload, "torrentName", "torrent_name", "name")) == "" {
		if name := strings.TrimSpace(firstNonEmpty(
			toStringAny(uploadData["name"], ""),
			toStringAny(uploadData["title"], ""),
			strings.TrimSuffix(filepath.Base(strings.TrimSpace(torrentPath)), filepath.Ext(strings.TrimSpace(torrentPath))),
		)); name != "" {
			nextPayload["torrentName"] = name
			nextPayload["name"] = name
		}
	}

	sourceInfo := map[string]any{}
	for key, value := range uploadData {
		sourceInfo[key] = value
	}
	generated, err := processingrepair.GenerateAndUploadScreenshots(processingrepair.ScreenshotGenerateInput{
		Payload:     nextPayload,
		SourceInfo:  sourceInfo,
		ContentName: strings.TrimSpace(firstPublishPayloadString(uploadData, "content_name", "contentName")),
		RootConfig:  nil,
	})
	if err != nil {
		return "", fmt.Errorf("生成并上传 PNG 截图失败: %w", err)
	}
	urls := normalizePublishWhitelistedPNGURLs(generated)
	if len(urls) == 0 {
		return "", fmt.Errorf("生成截图后未得到白名单图床的 PNG 链接")
	}
	return processingrepair.ToBBCodeImages(urls), nil
}

func preparePublishEpisodeOneMediaInfo(uploadData map[string]any, payload map[string]any, savePath string) (string, error) {
	trimmedSavePath := strings.TrimSpace(savePath)
	if trimmedSavePath == "" {
		return "", fmt.Errorf("剧集重新提取 E01 MediaInfo 失败: 缺少保存路径")
	}
	torrentName := strings.TrimSpace(firstNonEmpty(
		firstPublishPayloadString(payload, "torrentName", "torrent_name", "name"),
		toStringAny(uploadData["name"], ""),
		toStringAny(uploadData["title"], ""),
	))
	contentName := strings.TrimSpace(firstNonEmpty(
		firstPublishPayloadString(payload, "content_name", "contentName"),
		toStringAny(uploadData["content_name"], ""),
		toStringAny(uploadData["contentName"], ""),
	))
	episodePath, err := findPublishEpisodeOnePath(trimmedSavePath, torrentName, contentName)
	if err != nil {
		return "", fmt.Errorf("剧集重新提取 E01 MediaInfo 失败: %w", err)
	}
	target, err := processingmedia.ResolveMediaTargetForPath(episodePath, "发布 E01 MediaInfo")
	if err != nil {
		return "", fmt.Errorf("剧集重新提取 E01 MediaInfo 失败: %w", err)
	}
	defer target.Close()
	mediaInfo, err := processingmedia.ExtractMediaInfo(target.TargetFile)
	if err != nil {
		return "", fmt.Errorf("剧集重新提取 E01 MediaInfo 失败: %w", err)
	}
	return strings.TrimSpace(mediaInfo), nil
}

func isPublishSeries(uploadData map[string]any, payload map[string]any) bool {
	values := []string{
		toStringAny(uploadData["title"], ""),
		toStringAny(uploadData["subtitle"], ""),
		toStringAny(payload["torrentName"], ""),
		toStringAny(payload["torrent_name"], ""),
	}
	if standardized, ok := uploadData["standardized_params"].(map[string]any); ok && standardized != nil {
		for _, key := range []string{"type", "category", "cat"} {
			values = append(values, toStringAny(standardized[key], ""))
		}
	}
	if sourceParams, ok := uploadData["source_params"].(map[string]any); ok && sourceParams != nil {
		for _, value := range sourceParams {
			values = append(values, toStringAny(value, ""))
		}
	}
	for _, value := range values {
		lower := strings.ToLower(strings.TrimSpace(value))
		if lower == "" {
			continue
		}
		if strings.Contains(lower, "tv_series") || strings.Contains(lower, "tv series") || strings.Contains(lower, "series") || strings.Contains(lower, "剧集") {
			return true
		}
		if strings.Contains(lower, "s01") || strings.Contains(lower, "s02") || strings.Contains(lower, "e01") {
			return true
		}
	}
	return false
}

func findPublishEpisodeOnePath(savePath string, torrentName string, contentName string) (string, error) {
	roots := buildPublishScopedMediaRoots(savePath, torrentName, contentName)
	candidates := make([]string, 0)
	for _, root := range roots {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		info, err := os.Stat(root)
		if err != nil {
			continue
		}
		if !info.IsDir() {
			if isPublishMediaPath(root) && rePublishEpisodeOne.MatchString(filepath.Base(root)) {
				candidates = append(candidates, root)
			}
			continue
		}
		_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil || entry == nil || entry.IsDir() {
				return nil
			}
			if !isPublishMediaPath(path) || !rePublishEpisodeOne.MatchString(entry.Name()) {
				return nil
			}
			candidates = append(candidates, path)
			return nil
		})
		if len(candidates) > 0 {
			break
		}
	}
	if len(candidates) == 0 {
		return "", fmt.Errorf("未找到 E01 媒体文件")
	}
	sort.Slice(candidates, func(i, j int) bool {
		sizeI := publishFileSize(candidates[i])
		sizeJ := publishFileSize(candidates[j])
		if sizeI == sizeJ {
			return candidates[i] < candidates[j]
		}
		return sizeI > sizeJ
	})
	return candidates[0], nil
}

func buildPublishScopedMediaRoots(savePath string, torrentName string, contentName string) []string {
	roots := make([]string, 0, 3)
	seen := map[string]struct{}{}
	appendRoot := func(path string) {
		trimmed := strings.TrimSpace(path)
		if trimmed == "" {
			return
		}
		if _, exists := seen[trimmed]; exists {
			return
		}
		seen[trimmed] = struct{}{}
		roots = append(roots, trimmed)
	}

	trimmedSavePath := strings.TrimSpace(savePath)
	trimmedTorrentName := strings.TrimSpace(torrentName)
	trimmedContentName := strings.TrimSpace(contentName)
	if trimmedSavePath == "" {
		return roots
	}
	if trimmedTorrentName != "" {
		appendRoot(filepath.Join(trimmedSavePath, trimmedTorrentName))
	}
	if trimmedContentName != "" && !strings.EqualFold(trimmedContentName, trimmedTorrentName) {
		appendRoot(filepath.Join(trimmedSavePath, trimmedContentName))
	}
	baseName := strings.TrimSpace(filepath.Base(trimmedSavePath))
	if trimmedTorrentName == "" && trimmedContentName == "" {
		appendRoot(trimmedSavePath)
	} else if strings.EqualFold(baseName, trimmedTorrentName) || strings.EqualFold(baseName, trimmedContentName) {
		appendRoot(trimmedSavePath)
	} else if info, err := os.Stat(trimmedSavePath); err == nil && !info.IsDir() {
		appendRoot(trimmedSavePath)
	}
	return roots
}

func isPublishMediaPath(path string) bool {
	switch strings.ToLower(filepath.Ext(strings.TrimSpace(path))) {
	case ".mkv", ".mp4", ".m2ts", ".ts", ".avi":
		return true
	default:
		return false
	}
}

func publishFileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

func normalizePublishWhitelistedPNGURLs(urls []string) []string {
	out := make([]string, 0, len(urls))
	for _, raw := range urls {
		url := normalizePublishImageURL(raw)
		if url == "" {
			continue
		}
		out = appendUniquePublishString(out, url)
	}
	return out
}

func normalizePublishImageURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if direct := processingrepair.NormalizePixhostDirectHost(trimmed); strings.TrimSpace(direct) != "" {
		trimmed = strings.TrimSpace(direct)
	} else if direct := processingrepair.PixhostShowToDirectURL(trimmed); strings.TrimSpace(direct) != "" {
		trimmed = strings.TrimSpace(direct)
	}
	parsed, err := neturl.Parse(trimmed)
	if err != nil || parsed == nil {
		return ""
	}
	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	path := strings.ToLower(strings.TrimSpace(parsed.Path))
	if !strings.HasSuffix(path, ".png") {
		return ""
	}
	switch {
	case host == "pixhost.to" || strings.HasSuffix(host, ".pixhost.to"):
		return trimmed
	case host == "imgbox.com" || strings.HasSuffix(host, ".imgbox.com"):
		return trimmed
	case host == "gifyu.com" || strings.HasSuffix(host, ".gifyu.com"):
		return trimmed
	default:
		return ""
	}
}

func setPublishUploadSection(uploadData map[string]any, key string, value string) {
	if uploadData == nil {
		return
	}
	trimmed := strings.TrimSpace(value)
	uploadData[key] = trimmed
	intro, _ := uploadData["intro"].(map[string]any)
	if intro != nil {
		intro[key] = trimmed
	}
}

func pickPublishUploadSection(uploadData map[string]any, key string) string {
	if uploadData == nil {
		return ""
	}
	if fromTop := strings.TrimSpace(toStringAny(uploadData[key], "")); fromTop != "" {
		return fromTop
	}
	intro, _ := uploadData["intro"].(map[string]any)
	if intro == nil {
		return ""
	}
	return strings.TrimSpace(toStringAny(intro[key], ""))
}

func firstPublishPayloadString(values map[string]any, keys ...string) string {
	if values == nil {
		return ""
	}
	for _, key := range keys {
		if value := strings.TrimSpace(toStringAny(values[key], "")); value != "" {
			return value
		}
	}
	return ""
}

func appendUniquePublishString(items []string, value string) []string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return items
	}
	for _, existing := range items {
		if existing == trimmed {
			return items
		}
	}
	return append(items, trimmed)
}

func extractPublishImageURLs(text string) []string {
	return processingrepair.ExtractImageURLsFromText(text)
}
