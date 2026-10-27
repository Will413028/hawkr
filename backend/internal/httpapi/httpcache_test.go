package httpapi

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
)

// 公開 GET 的回應一定帶 Vary: Accept-Encoding，但 handler（connect-go）已經加過時不重複。
func TestCachePolicyVary(t *testing.T) {
	cases := []struct {
		name string
		set  []string // handler 自己寫的 Vary
		want []string
	}{
		{"handler 沒有寫", nil, []string{"Accept-Encoding"}},
		{"handler 已經寫了（connect-go 的 GET）", []string{"Accept-Encoding"}, []string{"Accept-Encoding"}},
		{"寫在逗號清單裡、大小寫不同", []string{"Origin, accept-encoding"}, []string{"Origin, accept-encoding"}},
		{"只有別的 header", []string{"Origin"}, []string{"Origin", "Accept-Encoding"}},
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
