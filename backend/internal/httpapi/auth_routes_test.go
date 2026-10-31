package httpapi

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	identityconnect "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/identity/v1alpha1/identityv1alpha1connect"
	httpidentity "github.com/Will413028/hawkr/backend/internal/httpapi/identity"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/legacy/gatewayproxy"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// authModule 與 authProvider 是 web 登入路由的最小替身：這裡驗的是入口中介層對 /api/auth/* 的處理，不是登入流程本身
// （後者在 httpapi/identity 的測試與 integration 測試）。
type authModule struct{}

func (authModule) BeginLoginFlow(context.Context, identity.LoginFlowInput) (identity.LoginFlowStart, error) {
	return identity.LoginFlowStart{State: "s", Nonce: "n", CodeVerifier: "v", Binding: "b"}, nil
}

func (authModule) ConsumeLoginFlow(context.Context, string, string, identity.Provider) (identity.LoginFlow, error) {
	return identity.LoginFlow{}, apperr.Unauthenticated("失效")
}

func (authModule) LinkIdentity(context.Context, int64, identity.ProviderIdentity) error {
	return apperr.Unauthenticated("失效")
}

func (authModule) Login(context.Context, identity.LoginInput) (identity.LoginResult, error) {
	return identity.LoginResult{}, apperr.Unauthenticated("失效")
}

type authProvider struct{ p identity.Provider }

func (a authProvider) Provider() identity.Provider { return a.p }
func (authProvider) CheckIssuer(string) bool       { return true }
func (authProvider) RequiresIssuer() bool          { return false }
func (a authProvider) AuthURL(string, string, string) string {
	return "https://" + strings.ToLower(string(a.p)) + ".test/auth"
}

func (authProvider) Exchange(context.Context, string, string, string) (identity.ProviderIdentity, error) {
	return identity.ProviderIdentity{}, apperr.Unauthenticated("失效")
}

func webLogin(t *testing.T) *httpidentity.WebLogin {
	t.Helper()
	w, err := httpidentity.NewWebLogin(httpidentity.WebLoginDeps{
		Module:    authModule{},
		Providers: []httpidentity.WebProvider{authProvider{identity.ProviderGoogle}, authProvider{identity.ProviderLine}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return w
}

func authEntry(t *testing.T, now func() time.Time) http.Handler {
	t.Helper()
	h, err := assemble(Deps{
		WebLogin: webLogin(t), Now: now, Origins: []string{testOrigin},
		OriginAuth: OriginAuth{Main: []byte("main-secret")}, Logger: slog.New(slog.DiscardHandler),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func authRequest(t *testing.T, method, path, ip string) *http.Request {
	t.Helper()
	r := httptest.NewRequestWithContext(t.Context(), method, path, nil)
	r.RemoteAddr = ip + ":4000"
	return r
}

type suspendedAccountAuth struct{}

func (suspendedAccountAuth) Authenticate(context.Context, string) (identity.Authenticated, error) {
	at := entryNow.Add(-time.Hour)
	return identity.Authenticated{Principal: identity.Principal{Kind: identity.PrincipalAccount, ID: 7}, SuspendedAt: &at}, nil
}

// /api/auth/* 是純 HTTP 的 GET 路由：不是 Connect 的 procedure，所以 withSession 的動作性質判斷（customerEffects）不套用，停權的帳號也能到登入頁
// （GET 放行）；任何會寫入的 method 都到不了它們。路由的清單來自 WebLogin.Routes()，每家 provider 一組 start、callback。
func TestAuthRoutesAreGetOnlyAndOutsideTheEffectDeclarations(t *testing.T) {
	t.Parallel()
	w := webLogin(t)
	routes := w.Routes()
	if len(routes) != 2*len(identity.LoginProviders()) {
		t.Fatalf("路由數 = %d；每家 provider 一個 start、一個 callback（新增 provider 要在這裡確認它不經寫入路徑）", len(routes))
	}
	inner := http.NewServeMux()
	for _, route := range routes {
		inner.Handle(route.Pattern, route.Handler)
	}
	h := withSession(suspendedAccountAuth{}, slog.New(slog.DiscardHandler), inner)
	for _, route := range routes {
		method, path, _ := strings.Cut(route.Pattern, " ")
		if method != http.MethodGet || !strings.HasPrefix(path, httpidentity.AuthPathPrefix) {
			t.Errorf("路由 %q 不是 GET /api/auth/*", route.Pattern)
		}
		if _, connectPath := rpc.AudienceOf(path); connectPath {
			t.Errorf("%s 被當成 Connect 的路徑，會套用 customerEffects", path)
		}
		if _, declared := customerEffects[path]; declared {
			t.Errorf("%s 不該出現在 customerEffects", path)
		}
		get := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
		get.Header.Set("Authorization", "Bearer suspended")
		if rec := do(h, get); rec.Code == http.StatusForbidden {
			t.Errorf("停權的帳號 GET %s = 403，登入頁必須能到", path)
		}
		for _, m := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
			r := httptest.NewRequestWithContext(t.Context(), m, path, nil)
			r.Header.Set("Authorization", "Bearer suspended")
			if rec := do(h, r); rec.Code < 400 {
				t.Errorf("%s %s = %d：寫入的 method 不能到 handler", m, path, rec.Code)
			}
		}
	}
	for _, path := range []string{"/api/auth/apple/start", "/api/auth/google", "/api/auth/google/start/extra", "/api/auth/google/logout"} {
		if rec := do(authEntry(t, func() time.Time { return entryNow }), authRequest(t, http.MethodGet, path, "192.0.2.1")); rec.Code != http.StatusNotFound {
			t.Errorf("GET %s = %d，預期 404（沒有宣告的 /api/auth/* 路徑不存在）", path, rec.Code)
		}
	}
}

// /api/auth/* 每個連線端 IP 一分鐘 30 個請求（start 與 callback 共用一個計數器）；超過回 429 加 Retry-After，不可快取；另一個 IP 與下一個時間窗不受影響。
func TestAuthRoutesAreRateLimitedPerClientIP(t *testing.T) {
	t.Parallel()
	now := entryNow
	h := authEntry(t, func() time.Time { return now })
	paths := []string{"/api/auth/google/start", "/api/auth/line/start", "/api/auth/google/callback?state=x&code=y"}
	for i := range tierAuth.visitor {
		if rec := do(h, authRequest(t, http.MethodGet, paths[i%len(paths)], "192.0.2.1")); rec.Code == http.StatusTooManyRequests {
			t.Fatalf("第 %d 個請求就被限流", i+1)
		}
	}
	rec := do(h, authRequest(t, http.MethodGet, paths[0], "192.0.2.1"))
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") == "" || rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("超過額度 = %d %v", rec.Code, rec.Header())
	}
	if rec := do(h, authRequest(t, http.MethodGet, paths[2], "192.0.2.1")); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("callback 與 start 共用一個計數器：%d", rec.Code)
	}
	if rec := do(h, authRequest(t, http.MethodGet, paths[0], "192.0.2.2")); rec.Code == http.StatusTooManyRequests {
		t.Fatal("另一個 IP 被限流")
	}
	now = now.Add(rateWindow)
	if rec := do(h, authRequest(t, http.MethodGet, paths[0], "192.0.2.1")); rec.Code == http.StatusTooManyRequests {
		t.Fatal("下一個時間窗還在限流")
	}
}

// 限流以連線端 IP 計，不信 origin-auth 的身分：持有 SSR 身分的請求（代表很多人）也在同一個 IP 的額度內。
func TestAuthRateLimitIgnoresOriginAuthSubjects(t *testing.T) {
	t.Parallel()
	h := authEntry(t, func() time.Time { return entryNow })
	for i := range tierAuth.visitor + 1 {
		r := authRequest(t, http.MethodGet, "/api/auth/google/start", "192.0.2.9")
		r.Header.Set(originAuthHeader, "invalid."+fmt.Sprint(i))
		rec := do(h, r)
		if i == tierAuth.visitor && rec.Code != http.StatusTooManyRequests {
			t.Fatalf("第 %d 個 = %d", i+1, rec.Code)
		}
	}
}

func TestGatewayProxyStripsTheSameCookieTheMonolithSets(t *testing.T) {
	t.Parallel()
	if gatewayproxy.SessionCookie != rpc.SessionCookie {
		t.Fatalf("gatewayproxy.SessionCookie = %q，rpc.SessionCookie = %q", gatewayproxy.SessionCookie, rpc.SessionCookie)
	}
	for _, name := range []string{rpc.SessionCookie, httpidentity.LoginCookie} {
		if !strings.HasPrefix(name, "__Host-") {
			t.Errorf("cookie %q 沒有 __Host- 前綴", name)
		}
	}
}

// 只有宣告 NO_SIDE_EFFECTS 的 procedure 能走 GET，不論憑證在 cookie 或 Bearer（ADR 2026-10-30-identity-self-hosted-federated-login D6、D7）：這條由
// connect-go 執行（沒有宣告的 procedure 收到 GET 回 405，handler 不會被呼叫），effects 與 proto 宣告的一致由
// TestEffectDeclarationsAgreeWithTheIdempotencyLevels 守。登出、連結、終止 session 用 GET 被拒。
func TestGETOnAWriteProcedureNeverReachesTheHandler(t *testing.T) {
	t.Parallel()
	path, handler := identityconnect.NewIdentityAppServiceHandler(httpidentity.NewApp(nil, nil, nil, time.Now))
	inner := http.NewServeMux()
	mount(inner, path, handler)
	h := withSession(fakeAuth{}, slog.New(slog.DiscardHandler), inner)
	credentials := map[string]func(*http.Request){
		"cookie": func(r *http.Request) { r.AddCookie(&http.Cookie{Name: rpc.SessionCookie, Value: "good"}) },
		"Bearer": func(r *http.Request) { r.Header.Set("Authorization", "Bearer good") },
	}
	for _, procedure := range []string{"DeleteAccount", "SignOut", "LinkIdentity", "RevokeSession"} {
		for name, credential := range credentials {
			r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path+procedure+"?connect=v1&encoding=json&message=%7B%7D", nil)
			credential(r)
			if rec := do(h, r); rec.Code != http.StatusMethodNotAllowed {
				t.Errorf("GET %s（%s）= %d，預期 405", procedure, name, rec.Code)
			}
		}
	}
}
