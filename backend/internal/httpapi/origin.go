package httpapi

import (
	"fmt"
	"net/http"
	"slices"

	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
)

// csrfBypass 是第三方網站以瀏覽器的表單 POST 回跳的路徑（金流、物流）：它們本來就是跨站的請求，也不帶我們的
// session（SameSite=Lax 的 cookie 不會跟著跨站的 POST 送出）。
var csrfBypass = []string{
	"POST /payment/return",
	"POST /store_map/return",
	"POST /atm-refund/return",
}

// newCSRF 擋下瀏覽器跨來源的非安全請求（Go 標準庫的 http.CrossOriginProtection：依 Sec-Fetch-Site，沒有時比對
// Origin 與 Host）。trusted 是選擇跨來源時 web 的 origin；經同源反代的請求是 same-origin，不看這份名單。沒有 Sec-Fetch-Site 也沒有 Origin 的請求（App、伺服器）
// 視為非瀏覽器，照常放行。
func newCSRF(trusted []string) (*http.CrossOriginProtection, error) {
	p := http.NewCrossOriginProtection()
	for _, o := range trusted {
		if err := p.AddTrustedOrigin(o); err != nil {
			return nil, fmt.Errorf("httpapi：CORS／CSRF 的 origin %q：%w", o, err)
		}
	}
	for _, pattern := range csrfBypass {
		p.AddInsecureBypassPattern(pattern)
	}
	return p, nil
}

// withCookieCSRF 是 session cookie 專用的 CSRF 檢查（ADR 2026-10-30-identity-self-hosted-federated-login D6），疊在 newCSRF 之上：
// 標準庫那層對沒有 Sec-Fetch-Site 也沒有 Origin 的請求放行，cookie 是瀏覽器自動帶的憑證，不能用這個縫隙。
// 只管以 cookie 認證的非 GET／HEAD 請求（來源以 rpc.Credential 判定，Bearer 優先，Bearer 不是 ambient credential 所以不檢查）：
// Sec-Fetch-Site 必須是 same-origin；沒有時 Origin 必須在 allowed 裡；兩者都沒有就拒絕。它在 withSession 之外，
// 所以拒絕發生在任何 handler（與 session 解析）之前。
func withCookieCSRF(allowed []string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead {
			next.ServeHTTP(w, r)
			return
		}
		if _, src := rpc.Credential(r); src != rpc.CredentialCookie {
			next.ServeHTTP(w, r)
			return
		}
		if site := r.Header.Get("Sec-Fetch-Site"); site != "" {
			if site != "same-origin" {
				http.Error(w, "cross-origin request detected from Sec-Fetch-Site header", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
			return
		}
		origin := r.Header.Get("Origin")
		if origin == "" {
			http.Error(w, "cross-origin request detected, and/or browser is out of date: Sec-Fetch-Site is missing, and Origin is missing", http.StatusForbidden)
			return
		}
		if !slices.Contains(allowed, origin) {
			http.Error(w, "cross-origin request detected, and/or browser is out of date: Sec-Fetch-Site is missing, and Origin does not match", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// cookieCSRFOrigins 是 withCookieCSRF 認得的 Origin：web 的 origin（PUBLIC_WEB_ORIGIN）與選擇跨來源時信任的 origin（Deps.Origins，與 newCSRF 同一份）。
func cookieCSRFOrigins(web rpc.WebOrigin, trusted []string) []string {
	out := slices.Clone(trusted)
	if o := web.Origin(); o != "" && !slices.Contains(out, o) {
		out = append(out, o)
	}
	return out
}
