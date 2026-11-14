package rpc

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// WebOrigin 把網站的站內路徑組成對外的絕對網址（PUBLIC_WEB_ORIGIN）。對外的網站網址只在這裡組，
// 模組與生成的 manifest 只放站內路徑；本來就是絕對網址的（第三方文件）原樣通過。
type WebOrigin struct {
	origin string
}

// NewWebOrigin 檢查網站網域：規則與 NewImages 相同（只有 scheme 與 host，https，本機的 localhost、127.0.0.1 可以用 http）。
func NewWebOrigin(origin string) (WebOrigin, error) {
	o, err := parseOrigin("PUBLIC_WEB_ORIGIN", origin, "https://www.example.com")
	if err != nil {
		return WebOrigin{}, err
	}
	return WebOrigin{origin: o}, nil
}

// IsZero 回報是不是沒有設定網域的零值。
func (w WebOrigin) IsZero() bool { return w.origin == "" }

// Origin 回傳網站的 origin（scheme 與 host），零值回空字串。
func (w WebOrigin) Origin() string { return w.origin }

// URL 回傳 ref 的絕對網址：以 / 開頭的站內路徑（不含 //）接在網域後面，有 host 的 http:// 或 https:// 網址原樣回傳；
// 其他形式（相對路徑、//host、其他 scheme）回錯誤，不讓不是絕對的網址流出去。
func (w WebOrigin) URL(ref string) (string, error) {
	if strings.HasPrefix(ref, "/") && !strings.HasPrefix(ref, "//") {
		return w.origin + ref, nil
	}
	u, err := url.Parse(ref)
	if err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" {
		return ref, nil
	}
	return "", fmt.Errorf("網址 %q 不是站內路徑或絕對的 http(s) 網址", ref)
}

// ReturnPage 是開通結束後金流商把賣家送回的網站頁面。
type ReturnPage int

const (
	// ReturnPageCertification 是賣家中心的認證頁（站內路徑含語系前綴）。
	ReturnPageCertification ReturnPage = iota + 1
	// ReturnPageSellerApp 是給賣家 App 的回跳頁（沒有語系前綴）。
	ReturnPageSellerApp
)

// ErrUnsupportedLocale 是網站不路由的語系。
var ErrUnsupportedLocale = errors.New("rpc：不支援的網站語系")

// ErrUnknownReturnPage 是不認得的回跳頁。
var ErrUnknownReturnPage = errors.New("rpc：不認得的回跳頁")

// sitePrefix 是語系的網站路徑前綴：預設語系在根路徑，其他語系在 /<locale>（next-intl 的 as-needed）。
func sitePrefix(locale uitext.Locale) (string, error) {
	switch locale {
	case uitext.ZhTW:
		return "", nil
	case uitext.EN:
		return "/en", nil
	}
	return "", fmt.Errorf("%w：%q", ErrUnsupportedLocale, locale)
}

// ValidWebLocale 回報 locale 是不是網站路由的語系。
func ValidWebLocale(locale string) (uitext.Locale, bool) {
	l := uitext.Locale(locale)
	_, err := sitePrefix(l)
	return l, err == nil
}

// onboardingReturnPaths 是各回跳頁的站內路徑與 query；回跳頁的網站路由必須存在（web_origin_test.go 守門）。
func onboardingReturnPaths(page ReturnPage, locale uitext.Locale) (done, expired string, err error) {
	prefix, err := sitePrefix(locale)
	if err != nil {
		return "", "", err
	}
	switch page {
	case ReturnPageCertification:
		base := prefix + CertificationPath
		return base + "?payment=return", base + "?payment=refresh", nil
	case ReturnPageSellerApp:
		return SellerAppReturnPath + "?state=done", SellerAppReturnPath + "?state=expired", nil
	}
	return "", "", fmt.Errorf("%w：%d", ErrUnknownReturnPage, page)
}

// 網站頁面的站內路徑（不含語系前綴）。
const (
	CertificationPath   = "/seller-center/me/certifications"
	SellerAppReturnPath = "/seller-app/payment-return"
)

// OnboardingReturnURLs 組出開通結束（done）與連結過期（expired）時的絕對回跳網址。
func (w WebOrigin) OnboardingReturnURLs(page ReturnPage, locale uitext.Locale) (done, expired string, err error) {
	d, e, err := onboardingReturnPaths(page, locale)
	if err != nil {
		return "", "", err
	}
	if done, err = w.URL(d); err != nil {
		return "", "", err
	}
	if expired, err = w.URL(e); err != nil {
		return "", "", err
	}
	return done, expired, nil
}

// CertificationURL 組出賣家中心認證頁的絕對網址（含語系前綴），給不能在自己畫面完成內嵌開通的 client 開網頁。
func (w WebOrigin) CertificationURL(locale uitext.Locale) (string, error) {
	prefix, err := sitePrefix(locale)
	if err != nil {
		return "", err
	}
	return w.URL(prefix + CertificationPath)
}
