package httpapi

import (
	"fmt"
	"net/http"
	"slices"
	"strings"

	httpidentity "github.com/Will413028/hawkr/backend/internal/httpapi/identity"
)

// 瀏覽器跨來源的呼叫只在入口處理一次；轉給 gateway 的 REST 也在這裡決定，gateway 不再回 CORS。web 在各環境都經
// 第一方同源反代打單體（〈系統架構與資料流向〉§2.4：session cookie 因此落在 web 的 host），CORS 只有選擇跨來源的
// 開發模式用得到。後台的瀏覽器只打自己的 BFF，BFF 以伺服器身分呼叫單體，跨來源的防護在後台自己那一層。

// corsAllowHeaders 是跨來源請求可以帶的 header：Connect 協定的、session（App 以外的瀏覽器用 cookie，Authorization
// 給同源的 BFF 以外的情況），以及還沒搬的 REST 用到的（X-Requested-With、Idempotency-Key、代行的 session、SSE 的
// Last-Event-ID）。X-Active-Role 也只為還沒搬的 REST：舊 REST 的 client 跨來源送它（工作區提示，gateway 的 X-Login-Role
// 由它推導）；Connect 不讀它。
var corsAllowHeaders = []string{
	"Content-Type", "Connect-Protocol-Version", "Connect-Timeout-Ms",
	"Authorization", "X-Active-Role", "If-Match", "If-None-Match",
	"X-Requested-With", "Idempotency-Key", "X-HK-Operation-Session", "Last-Event-ID", "X-App-Version",
}

// corsExposeHeaders 是瀏覽器的 JS 讀得到的回應 header：限流的退避提示（gateway 的 REST 回 429 時帶，不在 CORS 的
// safelist 裡）、trace id 與 ETag。
var corsExposeHeaders = []string{"Retry-After", "X-RateLimit-Limit", "X-RateLimit-Remaining", "X-RateLimit-Reset", "X-Trace-Id", "ETag"}

const corsMaxAge = 24 * 60 * 60

// withCORS 對 allowed 裡的 origin 回 CORS 的 header（允許帶 cookie）；preflight 一律在這裡回 204。
// 不在名單的 origin 不回 CORS 的 header，瀏覽器就擋下；非瀏覽器的 client（App、BFF）不受影響。
func withCORS(allowed []string, next http.Handler) http.Handler {
	allowHeaders := strings.Join(corsAllowHeaders, ", ")
	exposeHeaders := strings.Join(corsExposeHeaders, ", ")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// web 登入入口（/api/auth/*）只走同源反代，不放行跨來源：不回 CORS 的 header，preflight 也交給路由（405）。
		if strings.HasPrefix(r.URL.Path, httpidentity.AuthPathPrefix) {
			next.ServeHTTP(w, r)
			return
		}
		origin := r.Header.Get("Origin")
		ok := origin != "" && slices.Contains(allowed, origin)
		h := w.Header()
		if origin != "" {
			h.Add("Vary", "Origin")
		}
		if ok {
			h.Set("Access-Control-Allow-Origin", origin)
			h.Set("Access-Control-Allow-Credentials", "true")
			h.Set("Access-Control-Expose-Headers", exposeHeaders)
		}
		if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
			if ok {
				h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE")
				h.Set("Access-Control-Allow-Headers", allowHeaders)
				h.Set("Access-Control-Max-Age", fmt.Sprint(corsMaxAge))
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

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
