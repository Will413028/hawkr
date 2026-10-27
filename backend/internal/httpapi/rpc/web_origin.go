package rpc

import (
	"fmt"
	"net/url"
	"strings"
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
