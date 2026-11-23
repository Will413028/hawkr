package handler

import "github.com/Will413028/hawkr/go-pkg/mediaurl"

func buildURLs(base string, keys []string) []string {
	if len(keys) == 0 {
		return nil
	}
	urls := make([]string, 0, len(keys))
	for _, k := range keys {
		urls = append(urls, mediaurl.BuildURL(base, k))
	}
	return urls
}

// buildURL 將單一 objectKey 組為顯示 URL（Gateway 唯一組裝點）。
// key 為空回傳空字串（omitempty 生效）；http 開頭值由 mediaurl.BuildURL passthrough。
func buildURL(base, key string) string {
	if key == "" {
		return ""
	}
	return mediaurl.BuildURL(base, key)
}
