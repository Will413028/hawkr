package httpapi

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
)

// 公開 GET 的回應一定帶 Vary: Accept-Encoding，但 handler（connect-go）已經加過時不重複。
func TestCachePolicyVary(t *testing.T) {
	cases := []struct {
		name string
		set  []string // handler 自己寫的 Vary
		want []string
	}{
		{"handler 沒有寫", nil, []string{"Accept-Encoding", "Authorization", "Cookie"}},
		{"handler 已經寫了（connect-go 的 GET）", []string{"Accept-Encoding"}, []string{"Accept-Encoding", "Authorization", "Cookie"}},
		{"寫在逗號清單裡、大小寫不同", []string{"Origin, accept-encoding"}, []string{"Origin, accept-encoding", "Authorization", "Cookie"}},
		{"只有別的 header", []string{"Origin"}, []string{"Origin", "Accept-Encoding", "Authorization", "Cookie"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				for _, v := range c.set {
					w.Header().Add("Vary", v)
				}
				_, _ = w.Write([]byte("{}"))
			})
			rec := httptest.NewRecorder()
			cachePolicy(rpc.AudiencePublic, next).ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/x", nil))
			if got := rec.Header().Values("Vary"); !slices.Equal(got, c.want) {
				t.Errorf("Vary = %q，預期 %q", got, c.want)
			}
		})
	}
}

// Public 的 GET 依請求是否帶憑證決定快取（A7-D23 O1）：判斷依據是請求，不是 ctx 有沒有呼叫者；
// 停權帳號與無效 token 以訪客執行，ctx 沒有呼叫者，回應一樣不得進共享快取。
func TestCachePolicyCredential(t *testing.T) {
	cases := []struct {
		name         string
		header       http.Header
		callerInCtx  bool
		wantCache    string
		wantETagSent bool
	}{
		{"不帶憑證", nil, false, "public, no-cache", true},
		{"正常登入", http.Header{"Authorization": {"Bearer valid"}}, true, "private, no-store", false},
		{"停權帳號（以訪客執行）", http.Header{"Authorization": {"Bearer suspended"}}, false, "private, no-store", false},
		{"無效 token（以訪客執行）", http.Header{"Authorization": {"Bearer invalid"}}, false, "private, no-store", false},
		{"cookie 的 session", http.Header{"Cookie": {rpc.SessionCookie + "=tok"}}, false, "private, no-store", false},
		{"不認得的 cookie 不算憑證", http.Header{"Cookie": {"locale=zh-TW"}}, false, "public, no-cache", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("{}")) })
			ctx := t.Context()
			if c.callerInCtx {
				ctx = rpc.WithPrincipal(ctx, identity.Principal{})
			}
			req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/x", nil)
			for k, v := range c.header {
				req.Header[k] = v
			}
			rec := httptest.NewRecorder()
			cachePolicy(rpc.AudiencePublic, next).ServeHTTP(rec, req)
			if got := rec.Header().Get("Cache-Control"); got != c.wantCache {
				t.Errorf("Cache-Control = %q，預期 %q", got, c.wantCache)
			}
			if got := rec.Header().Get("ETag") != ""; got != c.wantETagSent {
				t.Errorf("有 ETag = %v，預期 %v", got, c.wantETagSent)
			}
			for _, name := range []string{"Authorization", "Cookie", "Accept-Encoding"} {
				if !varyIncludes(rec.Header(), name) {
					t.Errorf("Vary = %q，缺 %s", rec.Header().Values("Vary"), name)
				}
			}
		})
	}
}
