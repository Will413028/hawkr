package model

import (
	"net/url"
	"strings"
)

const (
	matchKeyEmail  = "EMAIL:"
	matchKeyPhone  = "PHONE:"
	matchKeyDomain = "DOMAIN:"
	matchKeySocial = "SOCIAL:"
)

var prospectPlaceholders = map[string]struct{}{
	"":    {},
	"待確認": {},
	"n/a": {},
	"na":  {},
	"-":   {},
	"無":   {},
	"未知":  {},
}

var prospectSocialHosts = map[string]string{
	"threads.com":    "threads",
	"threads.net":    "threads",
	"facebook.com":   "facebook",
	"m.facebook.com": "facebook",
	"fb.com":         "facebook",
	"instagram.com":  "instagram",
	"line.me":        "line",
	"shopee.tw":      "shopee",
	"lihi.cc":        "",
}

var prospectSharedHosts = []string{
	"linktr.ee",
	"pinkoi.com",
}

func isSharedHost(host string) bool {
	for _, shared := range prospectSharedHosts {
		if host == shared || strings.HasSuffix(host, "."+shared) {
			return true
		}
	}
	return false
}

var prospectNonProfilePaths = map[string]struct{}{
	"p":           {},
	"reel":        {},
	"t":           {},
	"pages":       {},
	"groups":      {},
	"watch":       {},
	"explore":     {},
	"share":       {},
	"stories":     {},
	"profile.php": {},
}

func normalizeToken(s string) string {
	v := strings.ToLower(strings.TrimSpace(s))
	if _, isPlaceholder := prospectPlaceholders[v]; isPlaceholder {
		return ""
	}
	return v
}

func normalizePhone(s string) string {
	v := normalizeToken(s)
	if v == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range v {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	digits := b.String()
	if strings.HasPrefix(digits, "886") {
		digits = "0" + strings.TrimPrefix(digits, "886")
	}
	return digits
}

func parseProspectURL(raw string) (domain, social string) {
	v := normalizeToken(raw)
	if v == "" {
		return "", ""
	}
	if !strings.Contains(v, "://") {
		v = "https://" + v
	}
	u, err := url.Parse(v)
	if err != nil || u.Host == "" {
		return "", ""
	}
	host := strings.TrimPrefix(u.Host, "www.")
	platform, isSocial := prospectSocialHosts[host]
	if !isSocial {
		if isSharedHost(host) {
			return "", ""
		}
		return host, ""
	}
	if platform == "" {
		return "", ""
	}
	handle := strings.Trim(u.Path, "/")
	if handle == "" {
		return "", ""
	}
	if strings.Contains(handle, "/") {
		return "", ""
	}
	handle = strings.TrimPrefix(handle, "@")
	if handle == "" {
		return "", ""
	}
	if platform == "facebook" && strings.EqualFold(handle, "profile.php") {
		idParam := u.Query().Get("id")
		if idParam == "" {
			return "", ""
		}
		return "", "facebook:" + idParam
	}
	if _, isRoute := prospectNonProfilePaths[strings.ToLower(handle)]; isRoute {
		return "", ""
	}
	return "", platform + ":" + handle
}

// ContactMatchKey 是一個聯絡方式的比對鍵（帶種類前綴：EMAIL:、PHONE:、DOMAIN:、SOCIAL:），遷移與 admin 建檔共用這個解析器。
// 空字串代表這個聯絡方式不具識別力（佔位值、共用平台、短網址、貼文路徑、沒有數字的電話），呼叫端必須存成沒有比對鍵、
// 絕不可以拿空字串進 WHERE。網站欄位的值是社群網址時鍵是 SOCIAL:；社群欄位的值不是社群網址時沒有鍵。
func ContactMatchKey(kind ContactKind, value string) string {
	switch kind {
	case ContactEmail:
		return withPrefix(matchKeyEmail, normalizeToken(value))
	case ContactPhone:
		return withPrefix(matchKeyPhone, normalizePhone(value))
	case ContactWebsite:
		domain, social := parseProspectURL(value)
		if domain != "" {
			return matchKeyDomain + domain
		}
		return withPrefix(matchKeySocial, social)
	case ContactSocial:
		_, social := parseProspectURL(value)
		return withPrefix(matchKeySocial, social)
	}
	return ""
}

func withPrefix(prefix, key string) string {
	if key == "" {
		return ""
	}
	return prefix + key
}
