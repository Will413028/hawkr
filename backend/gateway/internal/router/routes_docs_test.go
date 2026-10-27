package router

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/gateway/internal/config"
	"github.com/Will413028/hawkr/gateway/internal/handler"
	"github.com/gin-gonic/gin"
)

func newDocsRouter(t *testing.T, cfg config.DocsConfig) *gin.Engine {
	t.Helper()
	t.Setenv("APP_ENV", "local")
	gin.SetMode(gin.TestMode)
	r := gin.New()
	registerDocsRoutes(r, handler.NewDocs(cfg))
	return r
}

func docsLogin(r http.Handler, username, password string) *httptest.ResponseRecorder {
	form := url.Values{"username": {username}, "password": {password}}
	req := httptest.NewRequest(http.MethodPost, "/docs/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestDocsRoutes_DisabledWithoutConfig(t *testing.T) {
	cases := map[string]config.DocsConfig{
		"all empty":      {},
		"no username":    {Password: "p", SignKey: "k"},
		"no password":    {Username: "u", SignKey: "k"},
		"no signing key": {Username: "u", Password: "p"},
	}
	for name, cfg := range cases {
		t.Run(name, func(t *testing.T) {
			r := newDocsRouter(t, cfg)
			for _, tc := range []struct{ method, path string }{
				{http.MethodGet, "/docs/"},
				{http.MethodGet, "/docs/login"},
				{http.MethodPost, "/docs/login"},
			} {
				req := httptest.NewRequest(tc.method, tc.path, nil)
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)
				if w.Code != http.StatusNotFound {
					t.Fatalf("%s %s: want 404, got %d", tc.method, tc.path, w.Code)
				}
			}
		})
	}
}

func TestDocsRoutes_Configured(t *testing.T) {
	r := newDocsRouter(t, config.DocsConfig{Username: "viewer", Password: "pw-for-test", SignKey: "key-for-test"})

	req := httptest.NewRequest(http.MethodGet, "/docs/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusFound || w.Header().Get("Location") != "/docs/login" {
		t.Fatalf("no session: want redirect to /docs/login, got %d %q", w.Code, w.Header().Get("Location"))
	}

	w = docsLogin(r, "viewer", "wrong")
	if len(w.Result().Cookies()) != 0 || w.Code == http.StatusFound {
		t.Fatalf("wrong password must not create a session: code=%d cookies=%d", w.Code, len(w.Result().Cookies()))
	}

	w = docsLogin(r, "viewer", "pw-for-test")
	if w.Code != http.StatusFound {
		t.Fatalf("right password: want 302, got %d", w.Code)
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("want one session cookie, got %d", len(cookies))
	}

	req = httptest.NewRequest(http.MethodGet, "/docs/", nil)
	req.AddCookie(cookies[0])
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("with session: want 200, got %d", w.Code)
	}

	other := newDocsRouter(t, config.DocsConfig{Username: "viewer", Password: "pw-for-test", SignKey: "different-key"})
	req = httptest.NewRequest(http.MethodGet, "/docs/", nil)
	req.AddCookie(cookies[0])
	w = httptest.NewRecorder()
	other.ServeHTTP(w, req)
	if w.Code != http.StatusFound {
		t.Fatalf("session signed with another key must be rejected, got %d", w.Code)
	}
}
