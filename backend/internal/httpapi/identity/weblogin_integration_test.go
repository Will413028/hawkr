//go:build integration

package identity_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"

	"github.com/Will413028/hawkr/backend/internal/httpapi"
	"github.com/Will413028/hawkr/backend/internal/httpapi/characterize"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/identity/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/identity/v1alpha1/identityv1alpha1connect"
	httpidentity "github.com/Will413028/hawkr/backend/internal/httpapi/identity"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/legacy/gatewayproxy"
	"github.com/Will413028/hawkr/backend/internal/modules/accountdeletion"
	"github.com/Will413028/hawkr/backend/internal/modules/agent"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/delegation"
	"github.com/Will413028/hawkr/backend/internal/modules/discovery"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/reviewqueue"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

const (
	webOrigin     = "https://web.example"
	lineSecret    = "line-channel-secret-0123456789ab"
	googleClient  = "google-web-client"
	lineChannelID = "line-channel"
)

// grant 是假 provider 在授權步驟記下的東西：code 綁定 nonce、PKCE challenge 與 redirect URI，token 端點照真的 provider 驗證。
type grant struct {
	nonce, challenge, redirect string
	claims                     map[string]any
}

// fakeIdP 是假的 Google 或 LINE：授權由測試扮演瀏覽器完成（authorize），token 端點與 JWKS 是真的 HTTP。
type fakeIdP struct {
	t        *testing.T
	srv      *httptest.Server
	provider identity.Provider
	key      *rsa.PrivateKey
	mu       sync.Mutex
	codes    map[string]grant
	n        int
}

func newFakeIdP(t *testing.T, p identity.Provider) *fakeIdP {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeIdP{t: t, provider: p, key: key, codes: map[string]grant{}}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /token", f.token)
	mux.HandleFunc("GET /jwks", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &key.PublicKey, KeyID: "k1", Algorithm: "RS256", Use: "sig"}}})
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeIdP) clientID() string {
	if f.provider == identity.ProviderLine {
		return lineChannelID
	}
	return googleClient
}

func (f *fakeIdP) secret() string {
	if f.provider == identity.ProviderLine {
		return lineSecret
	}
	return "google-secret"
}

func (f *fakeIdP) issuer() string {
	if f.provider == identity.ProviderLine {
		return "https://access.line.me"
	}
	return "https://accounts.google.com"
}

// authorize 扮演使用者在 provider 同意授權：檢查授權網址的參數，回傳綁定好的 code。claims 覆寫 ID token 的內容（值為 nil 表示刪掉）。
func (f *fakeIdP) authorize(authURL string, claims map[string]any) (code string) {
	f.t.Helper()
	return f.authorizeWith(authURL, claims, "")
}

// authorizeWith 與 authorize 相同，challenge 不是空字串時 code 綁定這個 PKCE challenge，而不是授權網址上的（模擬 verifier 對不上）。
func (f *fakeIdP) authorizeWith(authURL string, claims map[string]any, challenge string) (code string) {
	f.t.Helper()
	u, err := url.Parse(authURL)
	if err != nil {
		f.t.Fatal(err)
	}
	q := u.Query()
	if q.Get("response_type") != "code" || q.Get("client_id") != f.clientID() || q.Get("code_challenge_method") != "S256" || q.Get("code_challenge") == "" ||
		q.Get("state") == "" || q.Get("nonce") == "" || !strings.Contains(q.Get("scope"), "openid") ||
		q.Get("redirect_uri") != webOrigin+"/api/auth/"+httpidentity.PathOf(f.provider)+"/callback" {
		f.t.Fatalf("授權網址的參數不對：%s", authURL)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.n++
	code = fmt.Sprintf("code-%d", f.n)
	if challenge == "" {
		challenge = q.Get("code_challenge")
	}
	f.codes[code] = grant{nonce: q.Get("nonce"), challenge: challenge, redirect: q.Get("redirect_uri"), claims: claims}
	return code
}

func (f *fakeIdP) token(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	f.mu.Lock()
	g, ok := f.codes[r.PostForm.Get("code")]
	delete(f.codes, r.PostForm.Get("code"))
	f.mu.Unlock()
	sum := sha256.Sum256([]byte(r.PostForm.Get("code_verifier")))
	if !ok || r.PostForm.Get("grant_type") != "authorization_code" || r.PostForm.Get("client_secret") != f.secret() ||
		r.PostForm.Get("client_id") != f.clientID() || r.PostForm.Get("redirect_uri") != g.redirect ||
		base64.RawURLEncoding.EncodeToString(sum[:]) != g.challenge {
		http.Error(w, `{"error":"invalid_grant"}`, http.StatusBadRequest)
		return
	}
	now := time.Now()
	claims := map[string]any{
		"iss": f.issuer(), "aud": f.clientID(), "sub": string(f.provider) + "-user-1", "nonce": g.nonce,
		"iat": now.Add(-time.Minute).Unix(), "exp": now.Add(time.Hour).Unix(), "name": "小明",
	}
	if f.provider == identity.ProviderGoogle {
		claims["email"], claims["email_verified"] = "ming@gmail.com", true
	}
	for k, v := range g.claims {
		if v == nil {
			delete(claims, k)
		} else {
			claims[k] = v
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "at", "token_type": "Bearer", "id_token": f.sign(claims)})
}

func (f *fakeIdP) sign(claims map[string]any) string {
	f.t.Helper()
	var key jose.SigningKey
	opts := (&jose.SignerOptions{}).WithType("JWT")
	if f.provider == identity.ProviderLine {
		key = jose.SigningKey{Algorithm: jose.HS256, Key: []byte(lineSecret)}
	} else {
		key = jose.SigningKey{Algorithm: jose.RS256, Key: f.key}
		opts = opts.WithHeader("kid", "k1")
	}
	signer, err := jose.NewSigner(key, opts)
	if err != nil {
		f.t.Fatal(err)
	}
	raw, err := jwt.Signed(signer).Claims(claims).Serialize()
	if err != nil {
		f.t.Fatal(err)
	}
	return raw
}

type webFixture struct {
	url    string
	db     *db.DB
	m      *identity.Module
	idp    map[identity.Provider]*fakeIdP
	gwSeen chan http.Header
}

// setupWeb 起 identity、假的 Google 與 LINE、舊 gateway 的替身與單體的入口。來源 IP 由請求的 X-Test-Client-IP 決定，各測試用自己的 IP，不互相佔用 /api/auth/* 的限流。
func setupWeb(t *testing.T) *webFixture {
	t.Helper()
	d := pg.NewDB(t, testdb.Jobs(), testdb.Migrations(
		migrate.Module{Name: "identity", Dir: identity.Migrations()},
		migrate.Module{Name: "buyer", Dir: buyer.Migrations()},
		migrate.Module{Name: "delegation", Dir: delegation.Migrations()},
	))
	jc, err := jobs.NewInsertOnly(d)
	if err != nil {
		t.Fatal(err)
	}
	dev, err := identity.NewDevAuth(nil)
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	bus := events.NewBus()
	m := identity.New(identity.Deps{DB: d, Events: bus, Jobs: jc, Providers: identity.Providers{Admin: dev.Admins()}, Now: time.Now, Logger: log})
	buyers := buyer.New(buyer.Deps{DB: d, Events: bus, Jobs: jc, Now: time.Now, Logger: log})
	events.Subscribe(bus, buyers.OnAccountCreated)
	del := delegation.New(delegation.Deps{DB: d, Identity: m, Events: bus, Seller: &seller.Module{}, Now: time.Now, Logger: log})

	f := &webFixture{db: d, m: m, idp: map[identity.Provider]*fakeIdP{}, gwSeen: make(chan http.Header, 8)}
	var providers []httpidentity.WebProvider
	for _, p := range []identity.Provider{identity.ProviderGoogle, identity.ProviderLine} {
		idp := newFakeIdP(t, p)
		f.idp[p] = idp
		w, err := identity.NewWebLogin(identity.WebLoginConfig{
			Provider: p, ClientID: idp.clientID(), ClientSecret: idp.secret(),
			RedirectURL: webOrigin + "/api/auth/" + httpidentity.PathOf(p) + "/callback",
			Endpoints:   identity.WebEndpoints{AuthURL: idp.srv.URL + "/auth", TokenURL: idp.srv.URL + "/token", JWKSURL: idp.srv.URL + "/jwks"},
			HTTPClient:  idp.srv.Client(),
		})
		if err != nil {
			t.Fatal(err)
		}
		providers = append(providers, w)
	}
	webLogin, err := httpidentity.NewWebLogin(httpidentity.WebLoginDeps{Module: m, Providers: providers, Logger: log})
	if err != nil {
		t.Fatal(err)
	}
	gw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.gwSeen <- r.Header.Clone()
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(gw.Close)
	proxy, err := gatewayproxy.New(gw.URL, func(ctx context.Context) (gatewayproxy.Caller, bool) {
		p, ok := rpc.PrincipalFrom(ctx)
		return gatewayproxy.Caller{Principal: p}, ok
	}, func(*http.Request, int64) (agent.SessionInfo, bool, error) { return agent.SessionInfo{}, false, nil }, noBusiness{}, noBusiness{}, nil, log)
	if err != nil {
		t.Fatal(err)
	}
	h, err := httpapi.NewHandler(httpapi.Deps{
		Social: &social.Module{}, Legacy: proxy, LegacyPrefixes: gatewayproxy.Prefixes, Ready: d.Ping, Identity: m, WebLogin: webLogin,
		Deletion: &accountdeletion.Module{}, Seller: &seller.Module{}, Stats: characterize.Stats(t), WebOrigin: characterize.WebOrigin(t),
		DashboardOrders: noDashboardOrders{}, Discovery: &discovery.Module{}, Agent: &agent.Module{}, ReviewQueue: &reviewqueue.Module{},
		Delegation: del, Origins: []string{webOrigin}, ClientIP: httpapi.ClientIP{Header: "X-Test-Client-IP"}, Logger: log,
	})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	f.url = srv.URL
	return f
}

type noBusiness struct{}

func (noBusiness) CurrentBuyerID(context.Context, int64) (int64, error) { return 0, nil }
func (noBusiness) CurrentStore(context.Context, int64) (seller.Store, error) {
	return seller.Store{}, apperr.NotFound("沒有現任店家")
}

// browser 是不跟轉址、手動管理 cookie 的瀏覽器：Secure 的 cookie 在 http 的測試伺服器上 cookiejar 不會送，所以自己帶。
type browser struct {
	t       *testing.T
	f       *webFixture
	cookies map[string]string
	ip      string
}

var noFollow = &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

func (f *webFixture) browser(t *testing.T, ip string) *browser {
	return &browser{t: t, f: f, cookies: map[string]string{}, ip: ip}
}

// reply 是回應的狀態、header 與 cookie；body 在 get 內讀完關閉，測試不看 body。
type reply struct {
	StatusCode int
	Header     http.Header
	cookies    []*http.Cookie
}

func (r *reply) Cookies() []*http.Cookie { return r.cookies }

func (b *browser) get(path string, header ...string) *reply {
	b.t.Helper()
	req, err := http.NewRequestWithContext(b.t.Context(), http.MethodGet, b.f.url+path, nil)
	if err != nil {
		b.t.Fatal(err)
	}
	req.Header.Set("X-Test-Client-IP", b.ip)
	for i := 0; i+1 < len(header); i += 2 {
		req.Header.Set(header[i], header[i+1])
	}
	for name, value := range b.cookies {
		req.AddCookie(&http.Cookie{Name: name, Value: value})
	}
	resp, err := noFollow.Do(req)
	if err != nil {
		b.t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	out := &reply{StatusCode: resp.StatusCode, Header: resp.Header, cookies: resp.Cookies()}
	for _, c := range out.cookies {
		if c.MaxAge < 0 {
			delete(b.cookies, c.Name)
		} else {
			b.cookies[c.Name] = c.Value
		}
	}
	return out
}

func setCookie(resp *reply, name string) *http.Cookie {
	for _, c := range resp.Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// login 走完 start 與（扮演使用者在 provider 同意之後的）callback；overrides 讓測試改動 callback 的 query 與 ID token 的 claims。
type loginOpts struct {
	returnTo  string
	claims    map[string]any
	callback  func(q url.Values) // 改 callback 的 query
	callbackP identity.Provider  // 打另一家的 callback（mix-up）
}

func (b *browser) login(p identity.Provider, o loginOpts) (start, callback *reply) {
	b.t.Helper()
	path := "/api/auth/" + httpidentity.PathOf(p) + "/start"
	if o.returnTo != "" {
		path += "?return_to=" + url.QueryEscape(o.returnTo)
	}
	start = b.get(path)
	if start.StatusCode != http.StatusFound {
		b.t.Fatalf("start = %d", start.StatusCode)
	}
	loc := start.Header.Get("Location")
	code := b.f.idp[p].authorize(loc, o.claims)
	u, _ := url.Parse(loc)
	q := url.Values{"state": {u.Query().Get("state")}, "code": {code}}
	if p == identity.ProviderGoogle {
		q.Set("iss", "https://accounts.google.com")
	}
	if o.callback != nil {
		o.callback(q)
	}
	target := p
	if o.callbackP != "" {
		target = o.callbackP
	}
	return start, b.get("/api/auth/" + httpidentity.PathOf(target) + "/callback?" + q.Encode())
}

func (f *webFixture) count(t *testing.T, query string) int {
	t.Helper()
	var n int
	if err := f.db.Pool().QueryRow(t.Context(), query).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func (f *webFixture) nothingCreated(t *testing.T) {
	t.Helper()
	for _, table := range []string{"identity.accounts", "identity.sessions", "identity.account_identities"} {
		if n := f.count(t, "SELECT count(*) FROM "+table); n != 0 {
			t.Fatalf("%s 有 %d 列，被拒絕的登入不能留下任何東西", table, n)
		}
	}
}

func wantLoginError(t *testing.T, resp *reply, code string) {
	t.Helper()
	u, err := url.Parse(resp.Header.Get("Location"))
	if resp.StatusCode != http.StatusFound || err != nil || u.Path != "/login" || u.Query().Get("error") != code {
		t.Fatalf("callback = %d %s，預期 302 /login?error=%s", resp.StatusCode, resp.Header.Get("Location"), code)
	}
	if setCookie(resp, rpc.SessionCookie) != nil {
		t.Fatal("失敗的登入設了 session cookie")
	}
}

func (f *webFixture) getSession(t *testing.T, opts ...connect.ClientOption) (*connect.Response[pb.GetSessionResponse], error) {
	t.Helper()
	c := identityv1alpha1connect.NewIdentityAppServiceClient(http.DefaultClient, f.url, append([]connect.ClientOption{connect.WithProtoJSON()}, opts...)...)
	return c.GetSession(t.Context(), connect.NewRequest(&pb.GetSessionRequest{}))
}

// 兩家各一條：start 設登入前 cookie 並轉去 provider，callback 換 session 並轉回返回網址；回應都不可快取；
// 新的 session cookie 與 Bearer 都能 GetSession。
func TestWebLoginSignsInWithEachProvider(t *testing.T) {
	t.Parallel()
	for _, p := range []identity.Provider{identity.ProviderGoogle, identity.ProviderLine} {
		t.Run(string(p), func(t *testing.T) {
			t.Parallel()
			f := setupWeb(t)
			b := f.browser(t, "192.0.2.10")
			start, cb := b.login(p, loginOpts{returnTo: "/account/orders?tab=open"})
			if lc := setCookie(start, httpidentity.LoginCookie); lc == nil || !lc.Secure || !lc.HttpOnly || lc.Path != "/" || lc.SameSite != http.SameSiteLaxMode || lc.MaxAge != 600 {
				t.Fatalf("登入前 cookie = %+v", lc)
			}
			if got := cb.Header.Get("Location"); cb.StatusCode != http.StatusFound || got != "/account/orders?tab=open" {
				t.Fatalf("callback = %d %s", cb.StatusCode, got)
			}
			c := setCookie(cb, rpc.SessionCookie)
			if c == nil || !c.Secure || !c.HttpOnly || c.Path != "/" || c.Domain != "" || c.SameSite != http.SameSiteLaxMode || len(c.Value) < 40 {
				t.Fatalf("session cookie = %+v", c)
			}
			if lc := setCookie(cb, httpidentity.LoginCookie); lc == nil || lc.MaxAge >= 0 {
				t.Fatalf("登入前 cookie 沒有清掉：%+v", lc)
			}
			for name, resp := range map[string]*reply{"start": start, "callback": cb} {
				if resp.Header.Get("Cache-Control") != "no-store" || resp.Header.Get("Content-Security-Policy") != "frame-ancestors 'none'" {
					t.Errorf("%s 的 header = %v", name, resp.Header)
				}
			}
			s, err := f.getSession(t, headersOf("Authorization", "Bearer "+c.Value))
			if err != nil || s.Msg.GetAccount().GetId() == "" {
				t.Fatalf("GetSession = %v, %v", s, err)
			}
			if n := f.count(t, `SELECT count(*) FROM identity.account_identities WHERE provider = '`+string(p)+`'`); n != 1 {
				t.Fatalf("身分列 = %d", n)
			}
		})
	}
}

func headersOf(kv ...string) connect.ClientOption {
	return connect.WithInterceptors(connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			for i := 0; i+1 < len(kv); i += 2 {
				req.Header().Set(kv[i], kv[i+1])
			}
			return next(ctx, req)
		}
	}))
}

// state、nonce、PKCE、iss、登入前 cookie、mix-up 的每一種不符都被拒絕，而且不留帳號、身分或 session。
func TestWebLoginRefusesBrokenFlows(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		opts      loginOpts
		challenge string
		cookie    func(*browser)
		want      string
		// stateKept 是這個 callback 的 state 不屬於任何流程（沒帶或亂填），真正的流程不會因此作廢，留到逾時。
		stateKept bool
	}{
		"state 不是我們發的":                  {opts: loginOpts{callback: func(q url.Values) { q.Set("state", strings.Repeat("A", 43)) }}, want: httpidentity.ErrorInvalidFlow, stateKept: true},
		"沒有 state":                      {opts: loginOpts{callback: func(q url.Values) { q.Del("state") }}, want: httpidentity.ErrorInvalidFlow, stateKept: true},
		"nonce 不符":                      {opts: loginOpts{claims: map[string]any{"nonce": "someone-elses-nonce"}}, want: httpidentity.ErrorInvalidFlow},
		"ID token 沒有 nonce":             {opts: loginOpts{claims: map[string]any{"nonce": nil}}, want: httpidentity.ErrorInvalidFlow},
		"aud 不符":                        {opts: loginOpts{claims: map[string]any{"aud": "another-client"}}, want: httpidentity.ErrorInvalidFlow},
		"iss 不符":                        {opts: loginOpts{claims: map[string]any{"iss": "https://evil.example"}}, want: httpidentity.ErrorInvalidFlow},
		"過期的 ID token":                  {opts: loginOpts{claims: map[string]any{"exp": time.Now().Add(-time.Hour).Unix()}}, want: httpidentity.ErrorInvalidFlow},
		"PKCE 的 verifier 對不上":           {opts: loginOpts{}, challenge: "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM", want: httpidentity.ErrorInvalidFlow},
		"code 不是 provider 發的":           {opts: loginOpts{callback: func(q url.Values) { q.Set("code", "code-that-was-never-issued") }}, want: httpidentity.ErrorInvalidFlow},
		"授權回應的 iss 不符":                  {opts: loginOpts{callback: func(q url.Values) { q.Set("iss", "https://evil.example") }}, want: httpidentity.ErrorInvalidFlow},
		"使用者拒絕授權":                       {opts: loginOpts{callback: func(q url.Values) { q.Del("code"); q.Set("error", "access_denied") }}, want: httpidentity.ErrorAccessDenied},
		"沒有登入前 cookie":                  {opts: loginOpts{}, cookie: func(b *browser) { delete(b.cookies, httpidentity.LoginCookie) }, want: httpidentity.ErrorInvalidFlow},
		"別人的登入前 cookie":                 {opts: loginOpts{}, cookie: func(b *browser) { b.cookies[httpidentity.LoginCookie] = strings.Repeat("B", 43) }, want: httpidentity.ErrorInvalidFlow},
		"A 家的流程打 B 家的 callback（mix-up）": {opts: loginOpts{callbackP: identity.ProviderLine}, want: httpidentity.ErrorInvalidFlow},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := setupWeb(t)
			b := f.browser(t, "192.0.2.11")
			p := identity.ProviderGoogle
			start := b.get("/api/auth/google/start")
			loc := start.Header.Get("Location")
			code := f.idp[p].authorizeWith(loc, tc.opts.claims, tc.challenge)
			u, _ := url.Parse(loc)
			q := url.Values{"state": {u.Query().Get("state")}, "code": {code}, "iss": {"https://accounts.google.com"}}
			if tc.opts.callback != nil {
				tc.opts.callback(q)
			}
			if tc.cookie != nil {
				tc.cookie(b)
			}
			target := p
			if tc.opts.callbackP != "" {
				target = tc.opts.callbackP
			}
			cb := b.get("/api/auth/" + httpidentity.PathOf(target) + "/callback?" + q.Encode())
			wantLoginError(t, cb, tc.want)
			f.nothingCreated(t)
			// 帶著屬於某個流程的 state 的失敗都已作廢該流程（單次使用）。
			want := 0
			if tc.stateKept {
				want = 1
			}
			if n := f.count(t, "SELECT count(*) FROM identity.login_flows"); n != want {
				t.Fatalf("失敗之後 login_flows 有 %d 列，預期 %d", n, want)
			}
		})
	}
}

func TestWebLoginRefusesAReplayedCallback(t *testing.T) {
	t.Parallel()
	f := setupWeb(t)
	b := f.browser(t, "192.0.2.12")
	start := b.get("/api/auth/google/start")
	loc := start.Header.Get("Location")
	u, _ := url.Parse(loc)
	q := url.Values{"state": {u.Query().Get("state")}, "code": {f.idp[identity.ProviderGoogle].authorize(loc, nil)}}
	binding := b.cookies[httpidentity.LoginCookie]
	first := b.get("/api/auth/google/callback?" + q.Encode())
	if setCookie(first, rpc.SessionCookie) == nil {
		t.Fatalf("第一次 = %d %s", first.StatusCode, first.Header.Get("Location"))
	}
	// 重放：連登入前 cookie 也放回去，只剩 state 單次使用擋住它。
	b.cookies[httpidentity.LoginCookie] = binding
	replay := b.get("/api/auth/google/callback?" + q.Encode())
	wantLoginError(t, replay, httpidentity.ErrorInvalidFlow)
	if n := f.count(t, "SELECT count(*) FROM identity.sessions"); n != 1 {
		t.Fatalf("sessions = %d", n)
	}
}

func TestWebLoginReturnPathsStaySameSite(t *testing.T) {
	t.Parallel()
	f := setupWeb(t)
	for i, tc := range []struct{ returnTo, want string }{
		{"/seller-center", "/seller-center"},
		{"//evil.example", "/"},
		{"https://evil.example/", "/"},
		{"/\\evil.example", "/"},
		{"javascript:alert(1)", "/"},
		{"", "/"},
	} {
		b := f.browser(t, fmt.Sprintf("192.0.2.%d", 20+i))
		_, cb := b.login(identity.ProviderGoogle, loginOpts{returnTo: tc.returnTo})
		if got := cb.Header.Get("Location"); got != tc.want {
			t.Errorf("return_to=%q → %q，預期 %q", tc.returnTo, got, tc.want)
		}
	}
}

// 登入換發新 session，請求帶的舊 session cookie 在同一個登入刪除；舊 token 之後不能再用。
func TestWebLoginReplacesThePreviousSession(t *testing.T) {
	t.Parallel()
	f := setupWeb(t)
	b := f.browser(t, "192.0.2.30")
	_, first := b.login(identity.ProviderGoogle, loginOpts{})
	old := setCookie(first, rpc.SessionCookie).Value
	if _, err := f.getSession(t, headersOf("Authorization", "Bearer "+old)); err != nil {
		t.Fatalf("舊 session = %v", err)
	}
	_, second := b.login(identity.ProviderGoogle, loginOpts{})
	fresh := setCookie(second, rpc.SessionCookie).Value
	if fresh == old {
		t.Fatal("登入沒有換發 token")
	}
	_, err := f.getSession(t, headersOf("Authorization", "Bearer "+old))
	wantCode(t, err, connect.CodeUnauthenticated)
	if _, err := f.getSession(t, headersOf("Authorization", "Bearer "+fresh)); err != nil {
		t.Fatalf("新 session = %v", err)
	}
	if n := f.count(t, "SELECT count(*) FROM identity.sessions"); n != 1 {
		t.Fatalf("sessions = %d，預期只剩新的一個", n)
	}
}

func TestWebLoginRoutesAreRateLimited(t *testing.T) {
	t.Parallel()
	f := setupWeb(t)
	b := f.browser(t, "192.0.2.40")
	var limited *reply
	for i := 0; i < 40 && limited == nil; i++ {
		if resp := b.get("/api/auth/google/start"); resp.StatusCode == http.StatusTooManyRequests {
			limited = resp
		}
	}
	if limited == nil || limited.Header.Get("Retry-After") == "" || limited.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("連續請求沒有被限流：%v", limited)
	}
	if resp := f.browser(t, "192.0.2.41").get("/api/auth/google/start"); resp.StatusCode != http.StatusFound {
		t.Fatalf("另一個 IP = %d", resp.StatusCode)
	}
}

// 憑證只收 Authorization: Bearer 與 session cookie：query、舊的 __session cookie、其他 header 都不收。
func TestOnlyBearerAndTheSessionCookieAreCredentials(t *testing.T) {
	t.Parallel()
	f := setupWeb(t)
	b := f.browser(t, "192.0.2.50")
	_, cb := b.login(identity.ProviderGoogle, loginOpts{})
	tok := setCookie(cb, rpc.SessionCookie).Value
	getSession := func(opts ...connect.ClientOption) error {
		_, err := f.getSession(t, append([]connect.ClientOption{connect.WithHTTPGet()}, opts...)...)
		return err
	}
	if err := getSession(headersOf("Authorization", "Bearer "+tok)); err != nil {
		t.Fatalf("Bearer 的 GetSession = %v", err)
	}
	if err := getSession(headersOf("Cookie", rpc.SessionCookie+"="+tok)); err != nil {
		t.Fatalf("cookie 的 GetSession（GET、NO_SIDE_EFFECTS）= %v", err)
	}
	t.Run("其他位置", func(t *testing.T) {
		for name, opt := range map[string]connect.ClientOption{
			"舊的 __session cookie": headersOf("Cookie", "__session="+tok),
			"token header":        headersOf("Token", tok, "X-Session-Token", tok),
			"Basic":               headersOf("Authorization", "Basic "+tok),
		} {
			if err := getSession(opt); connect.CodeOf(err) != connect.CodeUnauthenticated {
				t.Errorf("%s：err = %v，預期 Unauthenticated", name, err)
			}
		}
	})
	// query：token 放在網址上不收。
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, f.url+identityv1alpha1connect.IdentityAppServiceGetSessionProcedure+"?connect=v1&encoding=json&message=%7B%7D&token="+tok+"&"+rpc.SessionCookie+"="+tok, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "unauthenticated") {
		t.Fatalf("token 放在 query 被收了：%d %s", resp.StatusCode, body)
	}
}

// 轉給舊 gateway 的請求沒有 Bearer 與 session cookie，其他 cookie 還在；身分只有單體解析出的 X-Account-Id。
func TestGatewayReceivesNoSessionCredential(t *testing.T) {
	t.Parallel()
	f := setupWeb(t)
	b := f.browser(t, "192.0.2.60")
	_, cb := b.login(identity.ProviderGoogle, loginOpts{})
	tok := setCookie(cb, rpc.SessionCookie).Value
	for name, header := range map[string][]string{
		"Bearer": {"Authorization", "Bearer " + tok, "Cookie", "theme=dark"},
		"cookie": {"Cookie", rpc.SessionCookie + "=" + tok + "; theme=dark"},
	} {
		f.browser(t, "192.0.2.61").get("/api/v1/app/buyers/me", header...)
		select {
		case seen := <-f.gwSeen:
			if seen.Get("Authorization") != "" {
				t.Errorf("%s：gateway 收到 Authorization", name)
			}
			for _, c := range seen.Values("Cookie") {
				if strings.Contains(c, rpc.SessionCookie) || strings.Contains(c, tok) {
					t.Errorf("%s：gateway 收到 session cookie：%q", name, c)
				}
			}
			if !slices.Equal(seen.Values("Cookie"), []string{"theme=dark"}) {
				t.Errorf("%s：其他 cookie 沒有原樣轉出：%q", name, seen.Values("Cookie"))
			}
			if seen.Get(gatewayproxy.HeaderAccountID) == "" {
				t.Errorf("%s：gateway 沒有收到單體解析的身分", name)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("%s：gateway 沒有收到請求", name)
		}
	}
}
