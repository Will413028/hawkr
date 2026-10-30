package provider

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"

	"github.com/Will413028/hawkr/backend/internal/modules/identity/internal/model"
)

var webNow = time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)

// fakeProvider 是假的 Google／LINE：token 端點檢查 authorization code 與 PKCE verifier 後回一個 ID token，JWKS 端點給 Google 的公鑰。
type fakeProvider struct {
	t        *testing.T
	srv      *httptest.Server
	rsaKey   *rsa.PrivateKey
	secret   string
	code     string
	verifier string
	// idToken 組出要回的 ID token；tokenStatus 不是 0 時 token 端點直接回這個狀態。
	idToken     func() string
	tokenStatus int
	form        url.Values
}

func newFakeProvider(t *testing.T) *fakeProvider {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeProvider{t: t, rsaKey: key, secret: "line-channel-secret-0123456789ab", code: "code-1", verifier: "v-0123456789012345678901234567890123456789012"}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /token", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Error(err)
		}
		f.form = r.PostForm
		if f.tokenStatus != 0 {
			http.Error(w, `{"error":"invalid_grant"}`, f.tokenStatus)
			return
		}
		if r.PostForm.Get("code") != f.code || r.PostForm.Get("code_verifier") != f.verifier || r.PostForm.Get("grant_type") != "authorization_code" {
			http.Error(w, `{"error":"invalid_grant"}`, http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "at", "token_type": "Bearer", "refresh_token": "rt", "id_token": f.idToken()})
	})
	mux.HandleFunc("GET /jwks", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &key.PublicKey, KeyID: "k1", Algorithm: "RS256", Use: "sig"}}})
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeProvider) claims(extra map[string]any) map[string]any {
	c := map[string]any{"iat": webNow.Add(-time.Minute).Unix(), "exp": webNow.Add(time.Hour).Unix(), "sub": "sub-1"}
	for k, v := range extra {
		c[k] = v
	}
	return c
}

func (f *fakeProvider) signRS256(claims map[string]any) string {
	f.t.Helper()
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: f.rsaKey}, (&jose.SignerOptions{}).WithHeader("kid", "k1").WithType("JWT"))
	if err != nil {
		f.t.Fatal(err)
	}
	raw, err := jwt.Signed(signer).Claims(claims).Serialize()
	if err != nil {
		f.t.Fatal(err)
	}
	return raw
}

func (f *fakeProvider) signHS256(secret string, claims map[string]any) string {
	f.t.Helper()
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.HS256, Key: []byte(secret)}, (&jose.SignerOptions{}).WithType("JWT"))
	if err != nil {
		f.t.Fatal(err)
	}
	raw, err := jwt.Signed(signer).Claims(claims).Serialize()
	if err != nil {
		f.t.Fatal(err)
	}
	return raw
}

func (f *fakeProvider) web(p model.Provider) *WebLogin {
	f.t.Helper()
	secret := "google-client-secret"
	clientID := "google-web-client"
	if p == model.ProviderLine {
		secret, clientID = f.secret, "line-channel"
	}
	w, err := NewWebLogin(WebLoginConfig{
		Provider: p, ClientID: clientID, ClientSecret: secret, RedirectURL: "https://www.example.test/api/auth/" + string(p) + "/callback",
		Endpoints:  WebEndpoints{AuthURL: f.srv.URL + "/auth", TokenURL: f.srv.URL + "/token", JWKSURL: f.srv.URL + "/jwks"},
		HTTPClient: f.srv.Client(), Now: func() time.Time { return webNow },
	})
	if err != nil {
		f.t.Fatal(err)
	}
	return w
}

func TestGoogleAuthURLCarriesStateNonceAndS256Challenge(t *testing.T) {
	f := newFakeProvider(t)
	w := f.web(model.ProviderGoogle)
	u, err := url.Parse(w.AuthURL("state-1", "nonce-1", f.verifier))
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	sum := sha256.Sum256([]byte(f.verifier))
	want := map[string]string{
		"response_type": "code", "client_id": "google-web-client", "state": "state-1", "nonce": "nonce-1",
		"code_challenge": base64.RawURLEncoding.EncodeToString(sum[:]), "code_challenge_method": "S256",
		"redirect_uri": "https://www.example.test/api/auth/GOOGLE/callback", "scope": "openid email profile",
	}
	for k, v := range want {
		if q.Get(k) != v {
			t.Errorf("%s = %q，預期 %q", k, q.Get(k), v)
		}
	}
	if u.Host != f.srv.Listener.Addr().String() {
		t.Errorf("授權端點 = %s", u.Host)
	}
}

func TestGoogleExchangeVerifiesTheIDToken(t *testing.T) {
	f := newFakeProvider(t)
	w := f.web(model.ProviderGoogle)
	good := map[string]any{"iss": "https://accounts.google.com", "aud": "google-web-client", "nonce": "n1", "email": "a@corp.example", "email_verified": true, "hd": "corp.example", "name": "小明"}
	f.idToken = func() string { return f.signRS256(f.claims(good)) }
	id, err := w.Exchange(context.Background(), f.code, f.verifier, "n1")
	if err != nil {
		t.Fatal(err)
	}
	want := model.ProviderIdentity{Provider: model.ProviderGoogle, Subject: "sub-1", Email: "a@corp.example", EmailVerified: true, HostedDomain: "corp.example", Name: "小明"}
	if id != want {
		t.Fatalf("= %+v", id)
	}
	if f.form.Get("client_secret") != "google-client-secret" {
		t.Errorf("client_secret 沒有帶在 token 請求")
	}
	if f.form.Get("redirect_uri") != "https://www.example.test/api/auth/GOOGLE/callback" {
		t.Errorf("redirect_uri = %q", f.form.Get("redirect_uri"))
	}
	bad := map[string]map[string]any{
		"nonce 不符": {"nonce": "other"},
		"沒有 nonce": {"nonce": nil},
		"aud 不符":   {"aud": "someone-else"},
		"iss 不符":   {"iss": "https://evil.example"},
		"過期":       {"exp": webNow.Add(-time.Minute).Unix()},
	}
	for name, override := range bad {
		claims := map[string]any{}
		for k, v := range good {
			claims[k] = v
		}
		for k, v := range override {
			if v == nil {
				delete(claims, k)
			} else {
				claims[k] = v
			}
		}
		f.idToken = func() string { return f.signRS256(f.claims(claims)) }
		if _, err := w.Exchange(context.Background(), f.code, f.verifier, "n1"); !errors.Is(err, model.ErrInvalidToken) {
			t.Errorf("%s：err = %v，預期 ErrInvalidToken", name, err)
		}
	}
}

func TestExchangeSendsThePKCEVerifierAndMapsProviderFailures(t *testing.T) {
	f := newFakeProvider(t)
	w := f.web(model.ProviderGoogle)
	f.idToken = func() string {
		return f.signRS256(f.claims(map[string]any{"iss": "https://accounts.google.com", "aud": "google-web-client", "nonce": "n1"}))
	}
	if _, err := w.Exchange(context.Background(), f.code, "a-different-verifier-0123456789012345678901", "n1"); !errors.Is(err, model.ErrRejected) {
		t.Fatalf("verifier 不符 err = %v，預期 ErrRejected", err)
	}
	f.tokenStatus = http.StatusBadRequest
	if _, err := w.Exchange(context.Background(), f.code, f.verifier, "n1"); !errors.Is(err, model.ErrRejected) {
		t.Fatalf("4xx err = %v，預期 ErrRejected", err)
	}
	f.tokenStatus = http.StatusBadGateway
	_, err := w.Exchange(context.Background(), f.code, f.verifier, "n1")
	if err == nil || errors.Is(err, model.ErrRejected) || errors.Is(err, model.ErrInvalidToken) {
		t.Fatalf("5xx err = %v，預期沒有分類的錯誤", err)
	}
	if _, err := w.Exchange(context.Background(), f.code, f.verifier, ""); err == nil {
		t.Fatal("空的 nonce 也能換")
	}
}

func TestExchangeWithoutAnIDTokenIsInvalid(t *testing.T) {
	f := newFakeProvider(t)
	w := f.web(model.ProviderGoogle)
	f.idToken = func() string { return "" }
	if _, err := w.Exchange(context.Background(), f.code, f.verifier, "n1"); !errors.Is(err, model.ErrInvalidToken) {
		t.Fatalf("err = %v", err)
	}
}

func TestLineExchangeVerifiesTheHS256IDTokenWithTheChannelSecret(t *testing.T) {
	f := newFakeProvider(t)
	w := f.web(model.ProviderLine)
	good := map[string]any{"iss": "https://access.line.me", "aud": "line-channel", "nonce": "n1", "name": "小明", "email": "a@gmail.com"}
	f.idToken = func() string { return f.signHS256(f.secret, f.claims(good)) }
	id, err := w.Exchange(context.Background(), f.code, f.verifier, "n1")
	if err != nil {
		t.Fatal(err)
	}
	want := model.ProviderIdentity{Provider: model.ProviderLine, Subject: "sub-1", Email: "a@gmail.com", EmailVerified: false, Name: "小明"}
	if id != want {
		t.Fatalf("= %+v", id)
	}
	f.idToken = func() string { return f.signHS256("another-secret-0123456789abcdef01", f.claims(good)) }
	if _, err := w.Exchange(context.Background(), f.code, f.verifier, "n1"); !errors.Is(err, model.ErrInvalidToken) {
		t.Fatalf("別的 secret 簽的 err = %v", err)
	}
	f.idToken = func() string { return f.signRS256(f.claims(good)) }
	if _, err := w.Exchange(context.Background(), f.code, f.verifier, "n1"); !errors.Is(err, model.ErrInvalidToken) {
		t.Fatalf("RS256 的 err = %v", err)
	}
	f.idToken = func() string { return f.signHS256(f.secret, f.claims(good)) }
	if _, err := w.Exchange(context.Background(), f.code, f.verifier, "other"); !errors.Is(err, model.ErrInvalidToken) {
		t.Fatalf("nonce 不符 err = %v", err)
	}
	u, _ := url.Parse(w.AuthURL("s", "n", f.verifier))
	if u.Query().Get("scope") != "openid profile email" {
		t.Errorf("scope = %q", u.Query().Get("scope"))
	}
}

func TestCheckIssuerIsPerProvider(t *testing.T) {
	f := newFakeProvider(t)
	g, l := f.web(model.ProviderGoogle), f.web(model.ProviderLine)
	if !g.CheckIssuer("https://accounts.google.com") || g.CheckIssuer("accounts.google.com") || g.CheckIssuer("https://access.line.me") || g.CheckIssuer("") {
		t.Error("Google 的 issuer 判斷不對")
	}
	if !l.CheckIssuer("https://access.line.me") || l.CheckIssuer("https://accounts.google.com") {
		t.Error("LINE 的 issuer 判斷不對")
	}
}

func TestNewWebLoginRejectsIncompleteOrUnknownConfig(t *testing.T) {
	for name, cfg := range map[string]WebLoginConfig{
		"沒有 secret":    {Provider: model.ProviderGoogle, ClientID: "c", RedirectURL: "https://x.test/cb"},
		"沒有 redirect":  {Provider: model.ProviderLine, ClientID: "c", ClientSecret: "s"},
		"未知的 provider": {Provider: "FACEBOOK", ClientID: "c", ClientSecret: "s", RedirectURL: "https://x.test/cb"},
		"空的 provider":  {ClientID: "c", ClientSecret: "s", RedirectURL: "https://x.test/cb"},
	} {
		if _, err := NewWebLogin(cfg); err == nil {
			t.Errorf("%s：建立成功", name)
		}
	}
}
