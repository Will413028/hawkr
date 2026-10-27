// Package urlsafety 提供對外顯示用連結的最小驗證——只允許 credential-free
// HTTPS、拒絕 loopback/private/link-local 字面 IP。這是字串層級驗證，
// 不做 DNS 解析（不防「網域名稱解析到內網 IP」這種 SSRF 變形，見 repo 內
// 既有 payuni_redirect.go／website_url_validation.go 同等級的既有先例）。
package urlsafety

import (
	"fmt"
	"net"
	"net/url"
)

// ValidatePublicEvidenceURL 驗證一個要顯示給買家的輔助佐證連結是否安全。
// 允許：credential-free https URL，host 非字面 loopback/private/link-local/
// unspecified IP。拒絕：非 https scheme（含 javascript:/data:）、帶
// userinfo、空 host、格式錯誤、字面 loopback/private/link-local/unspecified IP。
func ValidatePublicEvidenceURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("urlsafety: malformed URL: %w", err)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("urlsafety: scheme %q not allowed, only https", u.Scheme)
	}
	if u.User != nil {
		return fmt.Errorf("urlsafety: URL must not contain userinfo")
	}
	if u.Host == "" {
		return fmt.Errorf("urlsafety: URL must have a non-empty host")
	}
	host := u.Hostname()
	if ip := net.ParseIP(host); ip != nil {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
			return fmt.Errorf("urlsafety: literal IP %q is loopback/private/link-local/unspecified", host)
		}
	}
	return nil
}
