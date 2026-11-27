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

func buildURL(base, key string) string {
	if key == "" {
		return ""
	}
	return mediaurl.BuildURL(base, key)
}
