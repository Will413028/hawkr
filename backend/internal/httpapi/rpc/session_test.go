package rpc

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCredentialAcceptsOnlyBearerAndTheSessionCookie(t *testing.T) {
	t.Parallel()
	req := func(t *testing.T, method, target string, mut func(*http.Request)) *http.Request {
		t.Helper()
		r := httptest.NewRequestWithContext(t.Context(), method, target, nil)
		if mut != nil {
			mut(r)
		}
		return r
	}
	cookie := func(name, value string) func(*http.Request) {
		return func(r *http.Request) { r.AddCookie(&http.Cookie{Name: name, Value: value}) }
	}
	for _, tc := range []struct {
		name   string
		r      *http.Request
		want   string
		source CredentialSource
	}{
		{"bearer", req(t, "POST", "/x", func(r *http.Request) { r.Header.Set("Authorization", "Bearer tok") }), "tok", CredentialBearer},
		{"make dev-token 的 dev session（32 bytes 的 base64url）以 Bearer 帶", req(t, "POST", "/hawkr.buyer.v1alpha1.BuyerAppService/GetProfile", func(r *http.Request) {
			r.Header.Set("Authorization", "Bearer "+strings.Repeat("Ab-_09", 7)+"Z")
		}), strings.Repeat("Ab-_09", 7) + "Z", CredentialBearer},
		{"bearer 的 scheme 不分大小寫", req(t, "GET", "/x", func(r *http.Request) { r.Header.Set("Authorization", "bearer tok") }), "tok", CredentialBearer},
		{"cookie 不限方法與路徑", req(t, "POST", "/hawkr.buyer.v1alpha1.BuyerAppService/CreateAddress", cookie(SessionCookie, "tok")), "tok", CredentialCookie},
		{"cookie 的 GET Connect", req(t, "GET", "/api/connect/hawkr.identity.v1alpha1.IdentityAppService/GetSession", cookie(SessionCookie, "tok")), "tok", CredentialCookie},
		{"query 不收", req(t, "GET", "/x?token=tok&access_token=tok&"+SessionCookie+"=tok", nil), "", CredentialNone},
		{"token header 不收", req(t, "GET", "/x", func(r *http.Request) { r.Header.Set("Token", "tok"); r.Header.Set("X-Session-Token", "tok") }), "", CredentialNone},
		{"Basic 不收", req(t, "GET", "/x", func(r *http.Request) { r.Header.Set("Authorization", "Basic tok") }), "", CredentialNone},
		{"舊的 __session cookie 不收", req(t, "GET", "/api/v1/app/invoices/1/pdf", cookie("__session", "tok")), "", CredentialNone},
		{"沒有 __Host- 前綴的同名 cookie 不收", req(t, "GET", "/x", cookie("hk_session", "tok")), "", CredentialNone},
		{"空的 Bearer 不退回 cookie", req(t, "GET", "/x", func(r *http.Request) {
			r.Header.Set("Authorization", "Bearer ")
			r.AddCookie(&http.Cookie{Name: SessionCookie, Value: "tok"})
		}), "", CredentialNone},
		{"同名 cookie 出現兩次", req(t, "GET", "/x", func(r *http.Request) {
			r.AddCookie(&http.Cookie{Name: SessionCookie, Value: "a"})
			r.AddCookie(&http.Cookie{Name: SessionCookie, Value: "b"})
		}), "", CredentialNone},
		{"兩者都有時以 Bearer 為準", req(t, "POST", "/x", func(r *http.Request) {
			r.Header.Set("Authorization", "Bearer bearer-token")
			r.AddCookie(&http.Cookie{Name: SessionCookie, Value: "cookie-token"})
		}), "bearer-token", CredentialBearer},
		{"沒有憑證", req(t, "GET", "/x", nil), "", CredentialNone},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, source := Credential(tc.r)
			if got != tc.want || source != tc.source {
				t.Fatalf("Credential = %q, %d；預期 %q, %d", got, source, tc.want, tc.source)
			}
		})
	}
}

func TestCredentialFromHeaderFollowsTheSameRulesAsCredential(t *testing.T) {
	t.Parallel()
	cases := map[string]http.Header{
		"bearer":         {"Authorization": {"Bearer tok"}},
		"cookie":         {"Cookie": {SessionCookie + "=tok"}},
		"兩者都有，Bearer 優先": {"Authorization": {"Bearer a"}, "Cookie": {SessionCookie + "=b"}},
		"空 Bearer":       {"Authorization": {"Bearer "}, "Cookie": {SessionCookie + "=b"}},
		"同名 cookie 兩個":   {"Cookie": {SessionCookie + "=a; " + SessionCookie + "=b"}},
		"其他 scheme":      {"Authorization": {"Basic abc"}},
		"舊的 cookie 名稱":   {"Cookie": {"__session=tok"}},
		"沒有":             {},
	}
	for name, h := range cases {
		r := &http.Request{Header: h}
		wantToken, wantSource := Credential(r)
		gotToken, gotSource := CredentialFromHeader(h)
		if gotToken != wantToken || gotSource != wantSource {
			t.Errorf("%s：CredentialFromHeader = %q, %v；Credential = %q, %v", name, gotToken, gotSource, wantToken, wantSource)
		}
	}
}
