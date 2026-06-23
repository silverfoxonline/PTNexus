package sites

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
	publishmapping "github.com/pt-nexus/server/internal/service/publish/mapping"
	"github.com/pt-nexus/server/internal/service/publish/publisher"
)

const ssdPublishLogModule = "publish-ssd"

var reSSDEpisodeOne = regexp.MustCompile(`(?i)(?:^|[ ._\-\[\(])(?:s\d{1,2})?e0?1(?:[ ._\-\]\)]|$)`)
var reSSDMediaInfoExtraBlankLine = regexp.MustCompile(`(?m)^((?:General|Video|Audio|Text(?:\s*#\d+)?|Menu|Chapters))\r?\n\s*\r?\n`)

type ssdPublisher struct {
	publicSiteDefaults
}

func PublishSSD(input publisher.PublishInput) (publisher.PublishResult, error) {
	return publishWithPublicSite(input, ssdPublisher{})
}

func (ssdPublisher) LogModule() string {
	return ssdPublishLogModule
}

func (ssdPublisher) AttemptPrefix(input publisher.PublishInput) string {
	return "Detected CMCT target: using direct PNG screenshot URLs, preserved extra description, and refreshed MediaInfo when available."
}

func (ssdPublisher) BuildDescription(input publisher.PublishInput) string {
	return buildSSDDescription(input.UploadData)
}

func (ssdPublisher) BuildExtraFormFields(input publisher.PublishInput) (map[string]string, error) {
	siteCfg, _ := publishmapping.LoadSitePublishConfig(strings.TrimSpace(input.SiteCode))
	resolveFieldName := func(mappingKey string, fallback string) string {
		if siteCfg == nil {
			return fallback
		}
		for _, key := range []string{mappingKey, fallback} {
			if resolved := strings.TrimSpace(siteCfg.FormFields[key]); resolved != "" {
				return resolved
			}
		}
		return fallback
	}

	screenshotURLs, err := resolveSSDScreenshotURLs(input)
	if err != nil {
		return nil, err
	}
	if len(screenshotURLs) == 0 {
		return nil, fmt.Errorf("CMCT requires PNG screenshots on a whitelisted image host")
	}
	infoURL := strings.TrimSpace(input.DoubanLink)
	if infoURL == "" {
		infoURL = strings.TrimSpace(input.IMDbLink)
	}
	if infoURL == "" {
		return nil, fmt.Errorf("CMCT requires a Douban or IMDb link")
	}

	extra := map[string]string{
		resolveFieldName("imdb_url", "url"):              infoURL,
		resolveFieldName("screenshots", "url_vimages"): strings.Join(screenshotURLs, "\n"),
	}
	if mediaInfo, mediaErr := resolveSSDMediaInfo(input); mediaErr == nil && strings.TrimSpace(mediaInfo) != "" {
		extra[resolveFieldName("technical_info", "technical_info")] = normalizeSSDMediaInfo(mediaInfo)
	}
	return extra, nil
}

func (ssdPublisher) AdjustFormFields(input publisher.PublishInput, formFields map[string]string) {
	if formFields == nil {
		return
	}
	delete(formFields, "dburl")
	delete(formFields, "team_sel")
	removeFormFieldsByPrefix(formFields, "tags[")
	applySSDTagCheckboxes(input.UploadData, formFields)
	if mediaInfo := strings.TrimSpace(formFields["Media_BDInfo"]); mediaInfo != "" {
		formFields["Media_BDInfo"] = normalizeSSDMediaInfo(mediaInfo)
	}
	for _, key := range []string{"descr", "description"} {
		if _, exists := formFields[key]; exists {
			formFields[key] = strings.TrimSpace(formFields[key])
		}
	}
}

func normalizeSSDMediaInfo(text string) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return ""
	}
	return strings.TrimSpace(reSSDMediaInfoExtraBlankLine.ReplaceAllString(trimmed, "$1\n"))
}

func buildSSDDescription(uploadData map[string]any) string {
	intro, _ := uploadData["intro"].(map[string]any)
	parts := make([]string, 0, 3)
	for _, key := range []string{"statement", "body"} {
		if section := strings.TrimSpace(resolveUploadSection(uploadData, key)); section != "" {
			parts = append(parts, section)
			continue
		}
		if intro != nil {
			if section := strings.TrimSpace(toStringAny(intro[key], "")); section != "" {
				parts = append(parts, section)
			}
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}

func applySSDTagCheckboxes(uploadData map[string]any, formFields map[string]string) {
	tags := resolveSiteCombinedTags(uploadData)
	setIfAnyTag := func(field string, candidates ...string) {
		if hasAnySiteTagLower(tags, candidates...) {
			formFields[field] = "1"
		}
	}
	setIfAnyTag("animation", "tag.动画", "动画")
	setIfAnyTag("exclusive", "tag.禁转", "禁转")
	setIfAnyTag("pack", "tag.合集", "tag.完结", "合集", "完结")
	setIfAnyTag("untouched", "tag.原生", "原生")
	setIfAnyTag("mandarin", "tag.国配", "tag.国语", "国配", "国语")
	setIfAnyTag("subtitlezh", "tag.中字", "中字")
	setIfAnyTag("subtitlesp", "tag.特效", "特效", "特效字幕")
	setIfAnyTag("selfcompile", "tag.自译", "自译")
	setIfAnyTag("dovi", "tag.杜比", "tag.Dolby Vision", "DoVi", "Dolby Vision", "杜比")
	setIfAnyTag("hdr10", "tag.HDR", "tag.HDR10", "HDR", "HDR10")
	setIfAnyTag("hdr10plus", "tag.HDR10+", "HDR10+")
	setIfAnyTag("hdrvivid", "tag.菁彩HDR", "菁彩HDR")
	setIfAnyTag("hlg", "tag.HLG", "HLG")
	setIfAnyTag("cc", "tag.cc", "CC")
	setIfAnyTag("3d", "tag.3d", "3D")
}

func resolveSSDScreenshotURLs(input publisher.PublishInput) ([]string, error) {
	raw := resolveUploadSection(input.UploadData, "screenshots")
	if urls := normalizeSSDWhitelistedPNGURLs(extractImageURLsFromText(raw)); len(urls) > 0 {
		return urls, nil
	}

	payload := map[string]any{}
	for key, value := range input.Payload {
		payload[key] = value
	}
	if strings.TrimSpace(input.SavePath) != "" {
		payload["savePath"] = strings.TrimSpace(input.SavePath)
		payload["save_path"] = strings.TrimSpace(input.SavePath)
	}
	if strings.TrimSpace(input.DownloaderID) != "" {
		payload["downloaderId"] = strings.TrimSpace(input.DownloaderID)
		payload["downloader_id"] = strings.TrimSpace(input.DownloaderID)
	}
	if strings.TrimSpace(payloadString(payload, "torrentName", "torrent_name", "name")) == "" {
		if name := strings.TrimSpace(firstNonEmpty(
			toStringAny(input.UploadData["name"], ""),
			toStringAny(input.UploadData["title"], ""),
			strings.TrimSuffix(filepath.Base(strings.TrimSpace(input.TorrentPath)), filepath.Ext(strings.TrimSpace(input.TorrentPath))),
		)); name != "" {
			payload["torrentName"] = name
			payload["name"] = name
		}
	}

	sourceInfo := map[string]any{}
	for key, value := range input.UploadData {
		sourceInfo[key] = value
	}
	generated, err := processingrepair.GenerateAndUploadScreenshots(processingrepair.ScreenshotGenerateInput{
		Payload:     payload,
		SourceInfo:  sourceInfo,
		ContentName: strings.TrimSpace(input.ContentName),
		RootConfig:  nil,
	})
	if err != nil {
		return nil, fmt.Errorf("CMCT screenshot refresh failed: %w", err)
	}
	urls := normalizeSSDWhitelistedPNGURLs(generated)
	if len(urls) == 0 {
		return nil, fmt.Errorf("CMCT screenshot refresh did not return PNG links on a whitelisted host")
	}
	return urls, nil
}

func normalizeSSDWhitelistedPNGURLs(urls []string) []string {
	out := make([]string, 0, len(urls))
	for _, raw := range urls {
		url := normalizeSSDImageURL(raw)
		if url == "" {
			continue
		}
		out = appendUniqueSSDString(out, url)
	}
	return out
}

func appendUniqueSSDString(items []string, value string) []string {
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

func normalizeSSDImageURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if direct := processingrepair.PixhostShowToDirectURL(trimmed); strings.TrimSpace(direct) != "" {
		trimmed = strings.TrimSpace(direct)
	} else if direct := processingrepair.NormalizePixhostDirectHost(trimmed); strings.TrimSpace(direct) != "" {
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

func resolveSSDMediaInfo(input publisher.PublishInput) (string, error) {
	fallback := strings.TrimSpace(input.MediaInfo)
	savePath := strings.TrimSpace(input.SavePath)
	if savePath == "" {
		return fallback, fmt.Errorf("missing save path")
	}
	torrentName := strings.TrimSpace(payloadString(input.Payload, "torrentName", "torrent_name", "name"))
	if torrentName == "" {
		torrentName = strings.TrimSpace(firstNonEmpty(
			toStringAny(input.UploadData["name"], ""),
			toStringAny(input.UploadData["title"], ""),
		))
	}
	if isSSDSeries(input) {
		episodePath, err := findSSDEpisodeOnePath(savePath, torrentName)
		if err != nil {
			return fallback, err
		}
		return extractSSDMediaInfoFromPath(episodePath, "CMCT E01 MediaInfo", fallback)
	}
	target, err := processingmedia.ResolveMediaTargetByCandidates(savePath, torrentName, strings.TrimSpace(input.ContentName), "CMCT MediaInfo")
	if err != nil {
		return fallback, err
	}
	defer target.Close()
	mediaInfo, err := processingmedia.ExtractMediaInfo(target.TargetFile)
	if err != nil || strings.TrimSpace(mediaInfo) == "" {
		return fallback, err
	}
	return mediaInfo, nil
}

func extractSSDMediaInfoFromPath(path string, scene string, fallback string) (string, error) {
	target, err := processingmedia.ResolveMediaTargetForPath(path, scene)
	if err != nil {
		return strings.TrimSpace(fallback), err
	}
	defer target.Close()
	mediaInfo, err := processingmedia.ExtractMediaInfo(target.TargetFile)
	if err != nil || strings.TrimSpace(mediaInfo) == "" {
		return strings.TrimSpace(fallback), err
	}
	return mediaInfo, nil
}
func isSSDSeries(input publisher.PublishInput) bool {
	values := []string{
		strings.TrimSpace(input.Title),
		strings.TrimSpace(input.Subtitle),
	}
	if standardized, ok := input.UploadData["standardized_params"].(map[string]any); ok && standardized != nil {
		values = append(values,
			toStringAny(standardized["type"], ""),
			toStringAny(standardized["category"], ""),
			toStringAny(standardized["cat"], ""),
		)
	}
	if sourceParams, ok := input.UploadData["source_params"].(map[string]any); ok && sourceParams != nil {
		for _, value := range sourceParams {
			values = append(values, toStringAny(value, ""))
		}
	}
	for _, value := range values {
		lower := strings.ToLower(strings.TrimSpace(value))
		if lower == "" {
			continue
		}
		if strings.Contains(lower, "tv_series") || strings.Contains(lower, "tv series") || strings.Contains(lower, "series") {
			return true
		}
		if strings.Contains(lower, "s01") || strings.Contains(lower, "s02") || strings.Contains(lower, "e01") {
			return true
		}
	}
	return false
}

func findSSDEpisodeOnePath(savePath string, torrentName string) (string, error) {
	roots := []string{savePath}
	if strings.TrimSpace(torrentName) != "" {
		roots = append([]string{filepath.Join(savePath, torrentName)}, roots...)
	}
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
			if isSSDMediaPath(root) && reSSDEpisodeOne.MatchString(filepath.Base(root)) {
				candidates = append(candidates, root)
			}
			continue
		}
		_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil || entry == nil || entry.IsDir() {
				return nil
			}
			if !isSSDMediaPath(path) || !reSSDEpisodeOne.MatchString(entry.Name()) {
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
		return "", fmt.Errorf("no E01 media file found")
	}
	sort.Slice(candidates, func(i, j int) bool {
		sizeI := fileSizeOrZero(candidates[i])
		sizeJ := fileSizeOrZero(candidates[j])
		if sizeI == sizeJ {
			return candidates[i] < candidates[j]
		}
		return sizeI > sizeJ
	})
	return candidates[0], nil
}

func isSSDMediaPath(path string) bool {
	switch strings.ToLower(filepath.Ext(strings.TrimSpace(path))) {
	case ".mkv", ".mp4", ".m2ts", ".ts", ".avi":
		return true
	default:
		return false
	}
}

func fileSizeOrZero(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

func payloadString(payload map[string]any, keys ...string) string {
	if payload == nil {
		return ""
	}
	for _, key := range keys {
		if value := strings.TrimSpace(toStringAny(payload[key], "")); value != "" {
			return value
		}
	}
	return ""
}
