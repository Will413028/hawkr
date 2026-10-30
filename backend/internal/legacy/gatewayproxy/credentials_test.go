package gatewayproxy

import (
	"fmt"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"net/http"
	"slices"
	"testing"
)

func TestStripSessionCredentials(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		cookies []string
		want    []string
	}{
		{"沒有 Cookie", nil, nil},
		{"只有 session cookie", []string{SessionCookie + "=tok"}, nil},
		{"其他 cookie 保留", []string{"a=1; " + SessionCookie + "=tok; b=2"}, []string{"a=1; b=2"}},
		{"session cookie 在最前", []string{SessionCookie + "=tok; a=1"}, []string{"a=1"}},
		{"多行 Cookie", []string{"a=1", SessionCookie + "=tok; b=2"}, []string{"a=1", "b=2"}},
		{"同名出現兩次都移除", []string{SessionCookie + "=x; a=1; " + SessionCookie + "=y"}, []string{"a=1"}},
		{"名稱相近的不動", []string{"x" + SessionCookie + "=1; " + SessionCookie + "x=2; __session=3"}, []string{"x" + SessionCookie + "=1; " + SessionCookie + "x=2; __session=3"}},
		{"沒有等號的片段保留", []string{"flag; " + SessionCookie + "=tok"}, []string{"flag"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := http.Header{"Authorization": {"Bearer tok"}}
			for _, c := range tc.cookies {
				h.Add("Cookie", c)
			}
			StripSessionCredentials(h)
			if v := h.Values("Authorization"); len(v) != 0 {
				t.Errorf("Authorization = %q", v)
			}
			if got := h.Values("Cookie"); !slices.Equal(got, tc.want) {
				t.Errorf("Cookie = %q，預期 %q", got, tc.want)
			}
		})
	}
}

// 轉出的請求沒有 Bearer 與 session cookie，其他 cookie 還在，身分只來自單體解析的 header。
func TestForwardedRequestCarriesNoSessionCredential(t *testing.T) {
	t.Parallel()
	gw := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, "auth=%q\ncookie=%q\naccount=%s\n", r.Header.Values("Authorization"), r.Header.Values("Cookie"), r.Header.Get(HeaderAccountID))
	})
	front := start(t, gw, &Caller{Principal: identity.Principal{Kind: identity.PrincipalAccount, ID: 7}})
	body := get(t, front.URL+"/api/v1/app/buyers/me", map[string]string{
		"Authorization": "Bearer secret-token",
		"Cookie":        "theme=dark; " + SessionCookie + "=secret-cookie; docs=1",
		HeaderAccountID: "999",
	})
	want := "auth=[]\ncookie=[\"theme=dark; docs=1\"]\naccount=7\n"
	if body != want {
		t.Fatalf("gateway 收到：\n%s預期：\n%s", body, want)
	}
}
