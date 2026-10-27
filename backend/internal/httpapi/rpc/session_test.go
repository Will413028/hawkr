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

func TestCredentialCookieOnlyForGETBrowserResources(t *testing.T) {
	for _, tc := range []struct{ name, method, path, mode, want string }{
		{"download", "GET", "/api/v1/app/invoices/1/pdf", "navigate", "cookie-token"},
		{"image no-cors", "GET", "/api/v1/app/media/secure_download/1", "no-cors", "cookie-token"},
		{"post", "POST", "/api/v1/app/invoices/1/pdf", "navigate", ""},
		{"post no-cors", "POST", "/api/v1/app/media/secure_download/1", "no-cors", ""},
		{"fetch cors", "GET", "/api/v1/app/invoices/1/pdf", "cors", ""},
		{"fetch same-origin", "GET", "/api/v1/app/invoices/1/pdf", "same-origin", ""},
		{"connect", "GET", "/hawkr.identity.v1alpha1.IdentityAppService/GetSession", "navigate", ""},
		{"connect prefix", "GET", "/api/connect/hawkr.identity.v1alpha1.IdentityAppService/GetSession", "navigate", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequestWithContext(t.Context(), tc.method, tc.path, nil)
			r.Header.Set("Sec-Fetch-Mode", tc.mode)
			r.AddCookie(&http.Cookie{Name: SessionCookie, Value: "cookie-token"})
			if got := Credential(r); got != tc.want {
				t.Fatalf("credential = %q; want %q", got, tc.want)
			}
			r.Header.Set("Authorization", "Bearer bearer-token")
			if got := Credential(r); got != "bearer-token" {
				t.Fatalf("bearer priority: %q", got)
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
