package handler

import (
	"fmt"

	"github.com/Will413028/hawkr/go-pkg/mediaurl"
)

func resolveInputObjectKey(mediaBase, keyVal, urlVal string, allowedPrefixes ...string) (string, error) {
	bases := []string{}
	if mediaBase != "" {
		bases = append(bases, mediaBase)
	}

	normalizedKey, err := mediaurl.NormalizeToObjectKey(keyVal, bases, allowedPrefixes)
	if err != nil {
		return "", err
	}
	normalizedURL, err := mediaurl.NormalizeToObjectKey(urlVal, bases, allowedPrefixes)
	if err != nil {
		return "", err
	}

	if normalizedKey != "" && normalizedURL != "" && normalizedKey != normalizedURL {
		return "", fmt.Errorf("圖片 key 與 url 值不一致: %q / %q", normalizedKey, normalizedURL)
	}
	if normalizedKey != "" {
		return normalizedKey, nil
	}
	return normalizedURL, nil
}
