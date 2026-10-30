package rpc

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
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

func TestVendorPortalCookieNameAndAttributes(t *testing.T) {
	now := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	c := NewVendorPortalCookie(42, "tok", now.Add(90*time.Second), now)
	if c.Name != "__Host-hk_vp_42" || c.Value != "tok" || c.Path != "/" || c.Domain != "" || !c.Secure || !c.HttpOnly || c.SameSite != http.SameSiteLaxMode || c.MaxAge != 90 {
		t.Fatalf("cookie = %+v", c)
	}
	if got := NewVendorPortalCookie(1, "t", now.Add(-time.Hour), now).MaxAge; got != 1 {
		t.Fatalf("Max-Age 至少 1：%d", got)
	}
	x := ExpiredVendorPortalCookie("__Host-hk_vp_7")
	if x.MaxAge >= 0 || x.Path != "/" || !x.Secure || !x.HttpOnly || !strings.Contains(x.String(), "Max-Age=0") {
		t.Fatalf("expired = %+v（%s）", x, x.String())
	}
}

func TestVendorPortalCookieTokenSelectsByRecipient(t *testing.T) {
	h := http.Header{"Cookie": {"a=b; " + VendorPortalCookieName(1) + "=one", VendorPortalCookieName(2) + "=two"}}
	if got := VendorPortalCookieToken(h, 1); got != "one" {
		t.Errorf("1 = %q", got)
	}
	if got := VendorPortalCookieToken(h, 2); got != "two" {
		t.Errorf("2 = %q", got)
	}
	if got := VendorPortalCookieToken(h, 3); got != "" {
		t.Errorf("3 = %q", got)
	}
	dup := http.Header{"Cookie": {VendorPortalCookieName(1) + "=a; " + VendorPortalCookieName(1) + "=b"}}
	if got := VendorPortalCookieToken(dup, 1); got != "" {
		t.Errorf("同名兩個 = %q", got)
	}
}

func TestVendorPortalCookieTokenSkipsMalformedCookies(t *testing.T) {
	vendor := VendorPortalCookieName(7) + "=tok"
	for _, tc := range []struct {
		name  string
		lines []string
	}{
		{"pair without equals before", []string{"foo; " + vendor}},
		{"pair without equals after", []string{vendor + "; foo"}},
		{"pair without equals on both sides", []string{"foo; " + vendor + "; bar"}},
		{"non-ASCII value before", []string{"bad=caf\u00e9; " + vendor}},
		{"control byte in value after", []string{vendor + "; bad=a\x01b"}},
		{"quote inside value", []string{"bad=a\"b; " + vendor}},
		{"invalid name", []string{"a b=c; " + vendor}},
		{"malformed line before", []string{"foo", vendor}},
		{"malformed line after", []string{vendor, "bad=caf\u00e9"}},
		{"malformed lines on both sides", []string{"foo; bad=a\x01b", vendor, "bar"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := VendorPortalCookieToken(http.Header{"Cookie": tc.lines}, 7); got != "tok" {
				t.Fatalf("token = %q; want %q（%q）", got, "tok", tc.lines)
			}
		})
	}
}

func TestExcessVendorPortalCookies(t *testing.T) {
	names := func(ids ...string) http.Header {
		var parts []string
		for _, id := range ids {
			parts = append(parts, VendorPortalCookiePrefix+id+"=x")
		}
		return http.Header{"Cookie": {strings.Join(parts, "; "), "other=1"}}
	}
	var many []string
	for i := 1; i <= 25; i++ {
		many = append(many, strconv.Itoa(i))
	}
	got := ExcessVendorPortalCookies(names(many...), 25)
	want := []string{}
	for i := 1; i <= 5; i++ {
		want = append(want, VendorPortalCookieName(int64(i)))
	}
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("清掉 %v，預期最小的 5 個 %v", got, want)
	}
	if got := ExcessVendorPortalCookies(names(many[:19]...), 100); got != nil {
		t.Fatalf("19 個加 keep 剛好 20，不清：%v", got)
	}
	if got := ExcessVendorPortalCookies(names(many[:20]...), 100); len(got) != 1 || got[0] != VendorPortalCookieName(1) {
		t.Fatalf("20 個加 keep 是 21，清最小的：%v", got)
	}
	// 格式不對的名稱最先清；keep 不會被清，即使它的 ID 最小。
	mixed := names(append([]string{"abc", "007"}, many[:19]...)...)
	got = ExcessVendorPortalCookies(mixed, 1)
	if !slices.Contains(got, VendorPortalCookiePrefix+"abc") || slices.Contains(got, VendorPortalCookieName(1)) {
		t.Fatalf("got = %v", got)
	}
	// 同樣的輸入，結果相同（只看名稱）。
	if again := ExcessVendorPortalCookies(mixed, 1); !slices.Equal(got, again) {
		t.Fatal("不確定的結果")
	}
}
