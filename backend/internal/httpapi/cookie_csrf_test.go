package httpapi

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
)

func cookieCSRFHandler() (http.Handler, *int) {
	reached := 0
	h := withCookieCSRF([]string{testOrigin, "https://dev.example"}, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		reached++
		w.WriteHeader(http.StatusNoContent)
	}))
	return h, &reached
}

func TestCookieCSRF(t *testing.T) {
	t.Parallel()
	cookie := func(r *http.Request) { r.AddCookie(&http.Cookie{Name: rpc.SessionCookie, Value: "tok"}) }
	bearer := func(r *http.Request) { r.Header.Set("Authorization", "Bearer tok") }
	with := func(fs ...func(*http.Request)) func(*http.Request) {
		return func(r *http.Request) {
			for _, f := range fs {
				f(r)
			}
		}
	}
	fetch := func(v string) func(*http.Request) { return func(r *http.Request) { r.Header.Set("Sec-Fetch-Site", v) } }
	origin := func(v string) func(*http.Request) { return func(r *http.Request) { r.Header.Set("Origin", v) } }
	cases := []struct {
		name   string
		method string
		mut    func(*http.Request)
		want   int
	}{
		{"cookie 跨站寫入", http.MethodPost, with(cookie, fetch("cross-site")), http.StatusForbidden},
		{"cookie same-site 但不是 same-origin", http.MethodPost, with(cookie, fetch("same-site")), http.StatusForbidden},
		{"cookie 使用者直接輸入網址（none）的寫入", http.MethodPost, with(cookie, fetch("none")), http.StatusForbidden},
		{"cookie same-origin", http.MethodPost, with(cookie, fetch("same-origin")), http.StatusNoContent},
		{"cookie same-origin 但 Origin 是外站（以 Sec-Fetch-Site 為準）", http.MethodPost, with(cookie, fetch("same-origin"), origin("https://evil.example")), http.StatusNoContent},
		{"cookie 無 Sec-Fetch-Site，Origin 相符", http.MethodPost, with(cookie, origin(testOrigin)), http.StatusNoContent},
		{"cookie 無 Sec-Fetch-Site，Origin 是信任的開發 origin", http.MethodDelete, with(cookie, origin("https://dev.example")), http.StatusNoContent},
		{"cookie 無 Sec-Fetch-Site，Origin 不符", http.MethodPost, with(cookie, origin("https://evil.example")), http.StatusForbidden},
		{"cookie 無 Sec-Fetch-Site，Origin 是 null", http.MethodPost, with(cookie, origin("null")), http.StatusForbidden},
		{"cookie 兩者都沒有", http.MethodPost, cookie, http.StatusForbidden},
		{"cookie PUT 兩者都沒有", http.MethodPut, cookie, http.StatusForbidden},
		{"Bearer 跨站寫入", http.MethodPost, with(bearer, fetch("cross-site")), http.StatusNoContent},
		{"Bearer 兩者都沒有", http.MethodPost, bearer, http.StatusNoContent},
		{"Bearer 與 cookie 同時，以 Bearer 為準：跨站放行", http.MethodPost, with(bearer, cookie, fetch("cross-site")), http.StatusNoContent},
		{"沒有憑證", http.MethodPost, fetch("cross-site"), http.StatusNoContent},
		{"cookie GET 不檢查", http.MethodGet, with(cookie, fetch("cross-site")), http.StatusNoContent},
		{"cookie HEAD 不檢查", http.MethodHead, with(cookie, fetch("cross-site")), http.StatusNoContent},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			h, reached := cookieCSRFHandler()
			r := httptest.NewRequestWithContext(t.Context(), c.method, "/x", nil)
			c.mut(r)
			rec := do(h, r)
			if rec.Code != c.want {
				t.Fatalf("status = %d，要 %d", rec.Code, c.want)
			}
			if (c.want == http.StatusNoContent) != (*reached == 1) {
				t.Fatalf("handler 執行 %d 次，status %d", *reached, rec.Code)
			}
		})
	}
}

func TestCookieCSRFRunsBeforeSessionAndHandlers(t *testing.T) {
	t.Parallel()
	reached := false
	h, err := assemble(Deps{
		Now: func() time.Time { return entryNow }, Origins: []string{testOrigin}, WebOrigin: webOriginOf(t),
		OriginAuth: OriginAuth{Main: []byte("main-secret")}, Logger: slog.New(slog.DiscardHandler),
	}, []service{{"/hawkr.test.v1alpha1.CsrfPublicService/", http.HandlerFunc(func(http.ResponseWriter, *http.Request) { reached = true })}})
	if err != nil {
		t.Fatal(err)
	}
	for _, mut := range []func(*http.Request){
		func(*http.Request) {},
		func(r *http.Request) { r.Header.Set("Origin", "https://evil.example") },
	} {
		r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/hawkr.test.v1alpha1.CsrfPublicService/Do", nil)
		r.AddCookie(&http.Cookie{Name: rpc.SessionCookie, Value: "tok"})
		mut(r)
		if rec := do(h, r); rec.Code != http.StatusForbidden || reached {
			t.Fatalf("status = %d，handler 已執行 = %v", rec.Code, reached)
		}
	}
}

func TestCookieCSRFAcceptsTheConfiguredWebOrigin(t *testing.T) {
	t.Parallel()
	web := webOriginOf(t)
	allowed := cookieCSRFOrigins(web, []string{"https://dev.example"})
	if len(allowed) != 2 || allowed[0] != "https://dev.example" || allowed[1] != web.Origin() || web.Origin() == "" {
		t.Fatalf("allowed = %v", allowed)
	}
	if got := cookieCSRFOrigins(rpc.WebOrigin{}, nil); len(got) != 0 {
		t.Fatalf("零值的 web origin 不能產生空字串的名單：%q", got)
	}
}

func webOriginOf(t *testing.T) rpc.WebOrigin {
	t.Helper()
	w, err := rpc.NewWebOrigin("https://www.hawkr.example")
	if err != nil {
		t.Fatal(err)
	}
	return w
}
