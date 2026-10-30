package identity

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

type storedFlow struct {
	provider identity.Provider
	binding  string
	flow     identity.LoginFlow
}

// fakeModule 照 identity 模組的契約實作登入流程：state 單次使用，不論成敗都已刪除；provider 與登入前 cookie 不符回 Unauthenticated。
type fakeModule struct {
	n          int
	flows      map[string]storedFlow
	logins     []identity.LoginInput
	loginErr   error
	loginToken string
	consumed   int
	beginErr   error
	consumeErr error
}

func newFakeModule() *fakeModule {
	return &fakeModule{flows: map[string]storedFlow{}, loginToken: "new-session-token"}
}

func (m *fakeModule) BeginLoginFlow(_ context.Context, in identity.LoginFlowInput) (identity.LoginFlowStart, error) {
	if m.beginErr != nil {
		return identity.LoginFlowStart{}, m.beginErr
	}
	m.n++
	s := identity.LoginFlowStart{
		State: fmt.Sprintf("state-%d", m.n), Nonce: fmt.Sprintf("nonce-%d", m.n),
		CodeVerifier: fmt.Sprintf("verifier-%d", m.n), Binding: fmt.Sprintf("binding-%d", m.n),
	}
	m.flows[s.State] = storedFlow{provider: in.Provider, binding: s.Binding, flow: identity.LoginFlow{
		Provider: in.Provider, CodeVerifier: s.CodeVerifier, Nonce: s.Nonce, ReturnPath: in.ReturnPath, LinkAccountID: in.LinkAccountID,
	}}
	return s, nil
}

func (m *fakeModule) ConsumeLoginFlow(_ context.Context, state, binding string, want identity.Provider) (identity.LoginFlow, error) {
	m.consumed++
	if m.consumeErr != nil {
		return identity.LoginFlow{}, m.consumeErr
	}
	f, ok := m.flows[state]
	delete(m.flows, state)
	if !ok || f.provider != want || subtle.ConstantTimeCompare([]byte(binding), []byte(f.binding)) != 1 {
		return identity.LoginFlow{}, apperr.Unauthenticated("登入流程已失效，請重新登入")
	}
	return f.flow, nil
}

func (m *fakeModule) Login(_ context.Context, in identity.LoginInput) (identity.LoginResult, error) {
	m.logins = append(m.logins, in)
	if m.loginErr != nil {
		return identity.LoginResult{}, m.loginErr
	}
	return identity.LoginResult{Token: m.loginToken, AccountID: 7}, nil
}

type fakeProvider struct {
	p         identity.Provider
	issuer    string
	identity  identity.ProviderIdentity
	err       error
	exchanges []exchange
}

type exchange struct{ code, verifier, nonce string }

func (f *fakeProvider) Provider() identity.Provider { return f.p }
func (f *fakeProvider) CheckIssuer(iss string) bool { return iss == f.issuer }
func (f *fakeProvider) AuthURL(state, nonce, verifier string) string {
	return "https://" + strings.ToLower(string(f.p)) + ".test/auth?" + url.Values{"state": {state}, "nonce": {nonce}, "verifier": {verifier}}.Encode()
}

func (f *fakeProvider) Exchange(_ context.Context, code, verifier, nonce string) (identity.ProviderIdentity, error) {
	f.exchanges = append(f.exchanges, exchange{code, verifier, nonce})
	return f.identity, f.err
}

type env struct {
	m      *fakeModule
	google *fakeProvider
	line   *fakeProvider
	routes map[string]http.Handler
}

func newEnv(t *testing.T, maxAge time.Duration, providers ...identity.Provider) *env {
	t.Helper()
	e := &env{
		m:      newFakeModule(),
		google: &fakeProvider{p: identity.ProviderGoogle, issuer: "https://accounts.google.com", identity: identity.ProviderIdentity{Provider: identity.ProviderGoogle, Subject: "g-1", Email: "a@gmail.com", EmailVerified: true}},
		line:   &fakeProvider{p: identity.ProviderLine, issuer: "https://access.line.me", identity: identity.ProviderIdentity{Provider: identity.ProviderLine, Subject: "l-1"}},
		routes: map[string]http.Handler{},
	}
	var configured []WebProvider
	for _, p := range providers {
		switch p {
		case identity.ProviderGoogle:
			configured = append(configured, e.google)
		case identity.ProviderLine:
			configured = append(configured, e.line)
		}
	}
	w, err := NewWebLogin(WebLoginDeps{Module: e.m, Providers: configured, SessionMaxAge: maxAge})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range w.Routes() {
		e.routes[r.Pattern] = r.Handler
	}
	return e
}

func (e *env) do(t *testing.T, target string, mut func(*http.Request)) *httptest.ResponseRecorder {
	t.Helper()
	method, path, _ := strings.Cut(target, " ")
	r := httptest.NewRequestWithContext(t.Context(), method, path, nil)
	if mut != nil {
		mut(r)
	}
	h, ok := e.routes["GET "+strings.SplitN(path, "?", 2)[0]]
	if !ok {
		t.Fatalf("沒有路由 %s", path)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

func cookieOf(rec *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func location(t *testing.T, rec *httptest.ResponseRecorder) *url.URL {
	t.Helper()
	if rec.Code != http.StatusFound {
		t.Fatalf("狀態 = %d，預期 302", rec.Code)
	}
	u, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func wantError(t *testing.T, rec *httptest.ResponseRecorder, code string) {
	t.Helper()
	u := location(t, rec)
	if u.Path != "/login" || u.Query().Get("error") != code || u.Host != "" {
		t.Fatalf("Location = %s，預期 /login?error=%s", u, code)
	}
	if c := cookieOf(rec, rpc.SessionCookie); c != nil {
		t.Fatalf("失敗的回應不能設 session cookie：%+v", c)
	}
}

// startFlow 走一次 start，回傳 provider 授權網址上的 state 與登入前 cookie。
func (e *env) startFlow(t *testing.T, provider, returnTo string) (state string, binding *http.Cookie) {
	t.Helper()
	target := "GET /api/auth/" + provider + "/start"
	if returnTo != "" {
		target += "?return_to=" + url.QueryEscape(returnTo)
	}
	rec := e.do(t, target, nil)
	u := location(t, rec)
	binding = cookieOf(rec, LoginCookie)
	if binding == nil {
		t.Fatal("start 沒有設登入前 cookie")
	}
	return u.Query().Get("state"), binding
}

func callbackURL(provider string, q url.Values) string {
	return "GET /api/auth/" + provider + "/callback?" + q.Encode()
}

func TestStartRedirectsToTheProviderWithAShortLivedPreLoginCookie(t *testing.T) {
	t.Parallel()
	e := newEnv(t, 0, identity.ProviderGoogle, identity.ProviderLine)
	rec := e.do(t, "GET /api/auth/google/start?return_to=/account/orders", nil)
	u := location(t, rec)
	if u.Host != "google.test" || u.Query().Get("state") != "state-1" || u.Query().Get("nonce") != "nonce-1" || u.Query().Get("verifier") != "verifier-1" {
		t.Fatalf("授權網址 = %s", u)
	}
	c := cookieOf(rec, LoginCookie)
	if c == nil || c.Value != "binding-1" || c.Path != "/" || c.Domain != "" || !c.Secure || !c.HttpOnly || c.SameSite != http.SameSiteLaxMode || c.MaxAge != 600 {
		t.Fatalf("登入前 cookie = %+v", c)
	}
	if !strings.HasPrefix(c.Name, "__Host-") {
		t.Fatalf("登入前 cookie 沒有 __Host- 前綴：%s", c.Name)
	}
	if rec.Header().Get("Cache-Control") != "no-store" || rec.Header().Get("Content-Security-Policy") != "frame-ancestors 'none'" {
		t.Fatalf("header = %v", rec.Header())
	}
	if got := e.m.flows["state-1"].flow.ReturnPath; got != "/account/orders" {
		t.Fatalf("存下的返回網址 = %q", got)
	}
	if got := e.m.flows["state-1"].flow.LinkAccountID; got != nil {
		t.Fatalf("登入的流程不帶連結帳號：%v", *got)
	}
	// 每家 provider 用自己的路徑與授權端點。
	if u := location(t, e.do(t, "GET /api/auth/line/start", nil)); u.Host != "line.test" {
		t.Fatalf("LINE 的授權網址 = %s", u)
	}
}

func TestStartFallsBackToTheRootForUnsafeReturnPaths(t *testing.T) {
	t.Parallel()
	for _, returnTo := range []string{"//evil.example", "https://evil.example/", "/\\evil.example", "evil", "javascript:alert(1)", "/a\nb", ""} {
		e := newEnv(t, 0, identity.ProviderGoogle)
		state, _ := e.startFlow(t, "google", returnTo)
		if got := e.m.flows[state].flow.ReturnPath; got != "/" {
			t.Errorf("return_to=%q 存成 %q，預期 /", returnTo, got)
		}
	}
}

func TestEveryLoginResponseIsNotCacheableAndNotFrameable(t *testing.T) {
	t.Parallel()
	e := newEnv(t, 0, identity.ProviderGoogle)
	state, binding := e.startFlow(t, "google", "")
	for name, rec := range map[string]*httptest.ResponseRecorder{
		"start":         e.do(t, "GET /api/auth/google/start", nil),
		"callback 成功":   e.do(t, callbackURL("google", url.Values{"state": {state}, "code": {"c"}}), func(r *http.Request) { r.AddCookie(binding) }),
		"callback 失敗":   e.do(t, callbackURL("google", url.Values{"state": {"x"}, "code": {"c"}}), nil),
		"未設定的 provider": e.do(t, "GET /api/auth/line/start", nil),
	} {
		if rec.Header().Get("Cache-Control") != "no-store" || rec.Header().Get("Content-Security-Policy") != "frame-ancestors 'none'" {
			t.Errorf("%s：header = %v", name, rec.Header())
		}
	}
}

func TestUnconfiguredProviderAnswers404AndNonGetIsRefused(t *testing.T) {
	t.Parallel()
	e := newEnv(t, 0, identity.ProviderGoogle)
	for _, path := range []string{"/api/auth/line/start", "/api/auth/line/callback?state=x&code=y"} {
		if rec := e.do(t, "GET "+path, nil); rec.Code != http.StatusNotFound {
			t.Errorf("%s = %d", path, rec.Code)
		}
	}
	if len(e.m.flows) != 0 || e.m.consumed != 0 {
		t.Fatal("未設定的 provider 碰了暫存列")
	}
	for _, method := range []string{http.MethodPost, http.MethodHead, http.MethodPut, http.MethodDelete} {
		for _, path := range []string{"/api/auth/google/start", "/api/auth/google/callback?state=x&code=y"} {
			rec := httptest.NewRecorder()
			e.routes["GET "+strings.SplitN(path, "?", 2)[0]].ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), method, path, nil))
			if rec.Code != http.StatusMethodNotAllowed {
				t.Errorf("%s %s = %d，預期 405", method, path, rec.Code)
			}
		}
	}
	if len(e.m.flows) != 0 || e.m.consumed != 0 {
		t.Fatal("非 GET 碰了暫存列")
	}
}

func TestStartFailureReturnsToTheLoginPage(t *testing.T) {
	t.Parallel()
	e := newEnv(t, 0, identity.ProviderGoogle)
	e.m.beginErr = errors.New("db down")
	rec := e.do(t, "GET /api/auth/google/start", nil)
	wantError(t, rec, ErrorUnavailable)
	if cookieOf(rec, LoginCookie) != nil {
		t.Fatal("start 失敗不能設登入前 cookie")
	}
}

func TestCallbackSignsInAndRedirectsToTheStoredReturnPath(t *testing.T) {
	t.Parallel()
	e := newEnv(t, 0, identity.ProviderGoogle, identity.ProviderLine)
	state, binding := e.startFlow(t, "google", "/account?tab=orders")
	e.google.identity.Name = "小明"
	rec := e.do(t, callbackURL("google", url.Values{"state": {state}, "code": {"auth-code"}, "iss": {"https://accounts.google.com"}, "return_to": {"//evil.example"}}), func(r *http.Request) {
		r.AddCookie(binding)
		r.AddCookie(&http.Cookie{Name: rpc.SessionCookie, Value: "old-token"})
	})
	if u := location(t, rec); u.String() != "/account?tab=orders" {
		t.Fatalf("Location = %s（返回網址只來自暫存列，不來自 callback 的 query）", u)
	}
	if got := e.google.exchanges; len(got) != 1 || got[0] != (exchange{"auth-code", "verifier-1", "nonce-1"}) {
		t.Fatalf("換 token 的參數 = %+v（verifier 與 nonce 要取自暫存列）", got)
	}
	if len(e.m.logins) != 1 {
		t.Fatalf("登入 %d 次", len(e.m.logins))
	}
	in := e.m.logins[0]
	if in.Identity != e.google.identity || in.Client != identity.ClientWeb || in.PreviousToken != "old-token" {
		t.Fatalf("Login 輸入 = %+v", in)
	}
	c := cookieOf(rec, rpc.SessionCookie)
	if c == nil || c.Value != "new-session-token" || c.Name != "__Host-hk_session" || c.Path != "/" || c.Domain != "" || !c.Secure || !c.HttpOnly || c.SameSite != http.SameSiteLaxMode {
		t.Fatalf("session cookie = %+v", c)
	}
	if c.MaxAge != 0 {
		t.Fatalf("沒有設定 Max-Age 時是 session cookie：%d", c.MaxAge)
	}
	if lc := cookieOf(rec, LoginCookie); lc == nil || lc.MaxAge >= 0 {
		t.Fatalf("登入前 cookie 沒有清掉：%+v", lc)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control = %q", rec.Header().Get("Cache-Control"))
	}
	if strings.Contains(rec.Header().Get("Location"), "new-session-token") {
		t.Fatal("token 不能出現在 Location")
	}
}

func TestSessionCookieMaxAgeFollowsTheConfiguration(t *testing.T) {
	t.Parallel()
	e := newEnv(t, 7*24*time.Hour, identity.ProviderGoogle)
	state, binding := e.startFlow(t, "google", "")
	rec := e.do(t, callbackURL("google", url.Values{"state": {state}, "code": {"c"}}), func(r *http.Request) { r.AddCookie(binding) })
	if c := cookieOf(rec, rpc.SessionCookie); c == nil || c.MaxAge != 7*24*3600 {
		t.Fatalf("cookie = %+v", c)
	}
}

func TestCallbackIgnoresABearerTokenAsThePreviousSession(t *testing.T) {
	t.Parallel()
	e := newEnv(t, 0, identity.ProviderGoogle)
	state, binding := e.startFlow(t, "google", "")
	e.do(t, callbackURL("google", url.Values{"state": {state}, "code": {"c"}}), func(r *http.Request) {
		r.AddCookie(binding)
		r.Header.Set("Authorization", "Bearer app-token")
	})
	if got := e.m.logins[0].PreviousToken; got != "" {
		t.Fatalf("PreviousToken = %q：web 登入只換發自己的 cookie session", got)
	}
}

func TestCallbackRefusesAMissingOrWrongPreLoginCookie(t *testing.T) {
	t.Parallel()
	for name, mut := range map[string]func() *http.Request{
		"沒有 cookie": func() *http.Request { return nil },
		"別人的 cookie": func() *http.Request {
			r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
			r.AddCookie(&http.Cookie{Name: LoginCookie, Value: "binding-attacker"})
			return r
		},
		"空的 cookie": func() *http.Request {
			r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
			r.AddCookie(&http.Cookie{Name: LoginCookie, Value: ""})
			return r
		},
	} {
		e := newEnv(t, 0, identity.ProviderGoogle)
		state, binding := e.startFlow(t, "google", "")
		other := mut()
		rec := e.do(t, callbackURL("google", url.Values{"state": {state}, "code": {"c"}}), func(r *http.Request) {
			if other != nil {
				r.Header.Set("Cookie", other.Header.Get("Cookie"))
			}
		})
		wantError(t, rec, ErrorInvalidFlow)
		if len(e.google.exchanges) != 0 || len(e.m.logins) != 0 {
			t.Errorf("%s：拒絕前不能換 token 或登入", name)
		}
		// 流程已作廢：同一個 state 配上正確的 cookie 也不行。
		retry := e.do(t, callbackURL("google", url.Values{"state": {state}, "code": {"c"}}), func(r *http.Request) { r.AddCookie(binding) })
		wantError(t, retry, ErrorInvalidFlow)
	}
}

func TestCallbackRefusesAReplayedState(t *testing.T) {
	t.Parallel()
	e := newEnv(t, 0, identity.ProviderGoogle)
	state, binding := e.startFlow(t, "google", "")
	q := url.Values{"state": {state}, "code": {"c"}}
	if rec := e.do(t, callbackURL("google", q), func(r *http.Request) { r.AddCookie(binding) }); rec.Code != http.StatusFound || cookieOf(rec, rpc.SessionCookie) == nil {
		t.Fatalf("第一次 = %d", rec.Code)
	}
	wantError(t, e.do(t, callbackURL("google", q), func(r *http.Request) { r.AddCookie(binding) }), ErrorInvalidFlow)
	if len(e.m.logins) != 1 || len(e.google.exchanges) != 1 {
		t.Fatalf("重放又登入了：%d 次登入、%d 次換 token", len(e.m.logins), len(e.google.exchanges))
	}
}

func TestCallbackRefusesAFlowStartedForAnotherProvider(t *testing.T) {
	t.Parallel()
	e := newEnv(t, 0, identity.ProviderGoogle, identity.ProviderLine)
	state, binding := e.startFlow(t, "google", "")
	q := url.Values{"state": {state}, "code": {"c"}}
	wantError(t, e.do(t, callbackURL("line", q), func(r *http.Request) { r.AddCookie(binding) }), ErrorInvalidFlow)
	if len(e.line.exchanges) != 0 || len(e.google.exchanges) != 0 || len(e.m.logins) != 0 {
		t.Fatal("mix-up 的請求碰了 provider 或登入")
	}
	wantError(t, e.do(t, callbackURL("google", q), func(r *http.Request) { r.AddCookie(binding) }), ErrorInvalidFlow)
}

func TestCallbackChecksTheIssuerWhenTheProviderSendsOne(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		iss  []string
		want string
	}{
		"沒有 iss":    {nil, ""},
		"正確的 iss":   {[]string{"https://accounts.google.com"}, ""},
		"別家的 iss":   {[]string{"https://access.line.me"}, ErrorInvalidFlow},
		"空的 iss":    {[]string{""}, ErrorInvalidFlow},
		"沒有 scheme": {[]string{"accounts.google.com"}, ErrorInvalidFlow},
	} {
		e := newEnv(t, 0, identity.ProviderGoogle)
		state, binding := e.startFlow(t, "google", "")
		q := url.Values{"state": {state}, "code": {"c"}}
		if tc.iss != nil {
			q["iss"] = tc.iss
		}
		rec := e.do(t, callbackURL("google", q), func(r *http.Request) { r.AddCookie(binding) })
		if tc.want == "" {
			if cookieOf(rec, rpc.SessionCookie) == nil {
				t.Errorf("%s：被拒絕", name)
			}
			continue
		}
		wantError(t, rec, tc.want)
		if len(e.google.exchanges) != 0 || len(e.m.logins) != 0 {
			t.Errorf("%s：iss 不符還是換了 token", name)
		}
	}
}

func TestCallbackProviderErrorsVoidTheFlow(t *testing.T) {
	t.Parallel()
	for provider, code := range map[string]string{"access_denied": ErrorAccessDenied, "server_error": ErrorProvider, "<script>": ErrorProvider} {
		e := newEnv(t, 0, identity.ProviderGoogle)
		state, binding := e.startFlow(t, "google", "")
		rec := e.do(t, callbackURL("google", url.Values{"state": {state}, "error": {provider}, "error_description": {"x"}}), func(r *http.Request) { r.AddCookie(binding) })
		wantError(t, rec, code)
		if strings.Contains(rec.Header().Get("Location"), "script") {
			t.Errorf("provider 的文字被反映到 Location：%s", rec.Header().Get("Location"))
		}
		if len(e.m.flows) != 0 {
			t.Errorf("%s：流程沒有作廢", provider)
		}
		if len(e.google.exchanges) != 0 || len(e.m.logins) != 0 {
			t.Errorf("%s：出錯的回應還是換了 token", provider)
		}
	}
}

func TestCallbackRefusesMissingParametersAndLinkFlows(t *testing.T) {
	t.Parallel()
	e := newEnv(t, 0, identity.ProviderGoogle)
	state, binding := e.startFlow(t, "google", "")
	wantError(t, e.do(t, callbackURL("google", url.Values{"state": {state}}), func(r *http.Request) { r.AddCookie(binding) }), ErrorInvalidFlow)
	wantError(t, e.do(t, callbackURL("google", url.Values{"code": {"c"}}), func(r *http.Request) { r.AddCookie(binding) }), ErrorInvalidFlow)

	state, binding = e.startFlow(t, "google", "")
	account := int64(9)
	f := e.m.flows[state]
	f.flow.LinkAccountID = &account
	e.m.flows[state] = f
	wantError(t, e.do(t, callbackURL("google", url.Values{"state": {state}, "code": {"c"}}), func(r *http.Request) { r.AddCookie(binding) }), ErrorInvalidFlow)
	if len(e.google.exchanges) != 0 || len(e.m.logins) != 0 {
		t.Fatal("缺參數或連結流程還是換了 token")
	}
}

func TestCallbackMapsProviderAndLoginFailures(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		exchangeErr error
		loginErr    error
		consumeErr  error
		want        string
	}{
		"ID token 無效":      {exchangeErr: errors.Join(errors.New("bad"), identity.ErrInvalidToken), want: ErrorInvalidFlow},
		"provider 拒絕 code": {exchangeErr: errors.Join(errors.New("bad"), identity.ErrRejected), want: ErrorInvalidFlow},
		"provider 連不上":     {exchangeErr: errors.New("timeout"), want: ErrorUnavailable},
		"帳號已刪除":            {loginErr: apperr.Unauthenticated("帳號已刪除"), want: ErrorLoginFailed},
		"登入失敗":             {loginErr: errors.New("db down"), want: ErrorUnavailable},
		"讀暫存列失敗":           {consumeErr: errors.New("db down"), want: ErrorUnavailable},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			e := newEnv(t, 0, identity.ProviderGoogle)
			state, binding := e.startFlow(t, "google", "")
			e.google.err, e.m.loginErr, e.m.consumeErr = tc.exchangeErr, tc.loginErr, tc.consumeErr
			wantError(t, e.do(t, callbackURL("google", url.Values{"state": {state}, "code": {"c"}}), func(r *http.Request) { r.AddCookie(binding) }), tc.want)
		})
	}
}

func TestNewWebLoginRejectsDuplicateProviders(t *testing.T) {
	t.Parallel()
	m := newFakeModule()
	g := &fakeProvider{p: identity.ProviderGoogle}
	if _, err := NewWebLogin(WebLoginDeps{Module: m, Providers: []WebProvider{g, g}}); err == nil {
		t.Fatal("同一家設定兩次也能建立")
	}
	if _, err := NewWebLogin(WebLoginDeps{Module: m, Providers: []WebProvider{&fakeProvider{p: "FACEBOOK"}}}); err == nil {
		t.Fatal("未知的 provider 也能建立")
	}
	if _, err := NewWebLogin(WebLoginDeps{}); err == nil {
		t.Fatal("沒有模組也能建立")
	}
}

func TestRoutesAreOneStartAndOneCallbackPerProviderAndGetOnly(t *testing.T) {
	t.Parallel()
	e := newEnv(t, 0)
	var got []string
	for pattern := range e.routes {
		got = append(got, pattern)
		if !strings.HasPrefix(pattern, "GET "+AuthPathPrefix) {
			t.Errorf("路由 %q 不是 GET /api/auth/*", pattern)
		}
	}
	if len(got) != 2*len(identity.LoginProviders()) {
		t.Fatalf("路由 %v 的數量不是每家 provider 兩條", got)
	}
	for _, p := range identity.LoginProviders() {
		for _, suffix := range []string{"/start", "/callback"} {
			if _, ok := e.routes["GET "+AuthPathPrefix+PathOf(p)+suffix]; !ok {
				t.Errorf("缺 %s%s", PathOf(p), suffix)
			}
		}
	}
}

func TestEachProviderSignsInThroughItsOwnRoutes(t *testing.T) {
	t.Parallel()
	e := newEnv(t, 0, identity.ProviderGoogle, identity.ProviderLine)
	state, binding := e.startFlow(t, "line", "/seller-center")
	rec := e.do(t, callbackURL("line", url.Values{"state": {state}, "code": {"line-code"}}), func(r *http.Request) { r.AddCookie(binding) })
	if u := location(t, rec); u.String() != "/seller-center" {
		t.Fatalf("Location = %s", u)
	}
	if len(e.google.exchanges) != 0 || len(e.line.exchanges) != 1 || e.line.exchanges[0].code != "line-code" {
		t.Fatalf("google %v，line %v", e.google.exchanges, e.line.exchanges)
	}
	if len(e.m.logins) != 1 || e.m.logins[0].Identity.Provider != identity.ProviderLine {
		t.Fatalf("Login = %+v", e.m.logins)
	}
}
