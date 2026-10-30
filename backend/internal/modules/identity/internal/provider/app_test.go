package provider

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"

	"github.com/Will413028/hawkr/backend/internal/modules/identity/internal/model"
)

// appIdP 是假的 Google 與 LINE 的公鑰端點：Google 發 RS256，LINE 原生 SDK 發 ES256。
type appIdP struct {
	t      *testing.T
	srv    *httptest.Server
	rsaKey *rsa.PrivateKey
	ecKey  *ecdsa.PrivateKey
}

func newAppIdP(t *testing.T) *appIdP {
	t.Helper()
	rk, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	ek, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	p := &appIdP{t: t, rsaKey: rk, ecKey: ek}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /google", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &rk.PublicKey, KeyID: "g1", Algorithm: "RS256", Use: "sig"}}})
	})
	mux.HandleFunc("GET /line", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &ek.PublicKey, KeyID: "l1", Algorithm: "ES256", Use: "sig"}}})
	})
	p.srv = httptest.NewServer(mux)
	t.Cleanup(p.srv.Close)
	return p
}

func (p *appIdP) sign(alg jose.SignatureAlgorithm, key any, kid string, claims map[string]any) string {
	p.t.Helper()
	opts := (&jose.SignerOptions{}).WithType("JWT")
	if kid != "" {
		opts = opts.WithHeader("kid", kid)
	}
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: alg, Key: key}, opts)
	if err != nil {
		p.t.Fatal(err)
	}
	raw, err := jwt.Signed(signer).Claims(claims).Serialize()
	if err != nil {
		p.t.Fatal(err)
	}
	return raw
}

// appClaims 合併 claims，後面的覆寫前面的，值為 nil 表示刪掉。
func appClaims(layers ...map[string]any) map[string]any {
	c := map[string]any{"iat": webNow.Add(-time.Minute).Unix(), "exp": webNow.Add(time.Hour).Unix(), "sub": "sub-1", "nonce": "n1"}
	for _, layer := range layers {
		for k, v := range layer {
			if v == nil {
				delete(c, k)
			} else {
				c[k] = v
			}
		}
	}
	return c
}

func (p *appIdP) google(t *testing.T, parties ...string) *AppLogin {
	t.Helper()
	if parties == nil {
		parties = []string{"android-client", "ios-client"}
	}
	a, err := NewAppLogin(AppLoginConfig{Provider: model.ProviderGoogle, ClientID: "google-web-client", AuthorizedParties: parties, JWKSURL: p.srv.URL + "/google", HTTPClient: p.srv.Client(), Now: func() time.Time { return webNow }})
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func (p *appIdP) line(t *testing.T) *AppLogin {
	t.Helper()
	a, err := NewAppLogin(AppLoginConfig{Provider: model.ProviderLine, ClientID: "line-channel", JWKSURL: p.srv.URL + "/line", HTTPClient: p.srv.Client(), Now: func() time.Time { return webNow }})
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func (p *appIdP) googleToken(extra map[string]any) string {
	base := map[string]any{"iss": "https://accounts.google.com", "aud": "google-web-client", "azp": "android-client", "email": "a@gmail.com", "email_verified": true, "name": "小明"}
	return p.sign(jose.RS256, p.rsaKey, "g1", appClaims(base, extra))
}

func (p *appIdP) lineToken(extra map[string]any) string {
	base := map[string]any{"iss": "https://access.line.me", "aud": "line-channel", "name": "小明"}
	return p.sign(jose.ES256, p.ecKey, "l1", appClaims(base, extra))
}

func TestGoogleAppLoginVerifiesTheIDToken(t *testing.T) {
	t.Parallel()
	p := newAppIdP(t)
	a := p.google(t)
	for name, iss := range map[string]string{"帶 scheme": "https://accounts.google.com", "不帶 scheme": "accounts.google.com"} {
		id, err := a.Verify(t.Context(), p.googleToken(map[string]any{"iss": iss}), "n1")
		if err != nil {
			t.Fatalf("%s：%v", name, err)
		}
		if id.Provider != model.ProviderGoogle || id.Subject != "sub-1" || id.Email != "a@gmail.com" || !id.EmailVerified || id.Name != "小明" {
			t.Errorf("%s：id = %+v", name, id)
		}
	}
	for name, tc := range map[string]struct {
		token string
		nonce string
	}{
		"nonce 不符":          {p.googleToken(nil), "other"},
		"token 沒有 nonce":    {p.googleToken(map[string]any{"nonce": nil}), "n1"},
		"azp 不在允許清單":        {p.googleToken(map[string]any{"azp": "evil-client"}), "n1"},
		"沒有 azp":            {p.googleToken(map[string]any{"azp": nil}), "n1"},
		"aud 不是 web client": {p.googleToken(map[string]any{"aud": "android-client"}), "n1"},
		"iss 不對":            {p.googleToken(map[string]any{"iss": "https://evil.example"}), "n1"},
		"過期":                {p.googleToken(map[string]any{"exp": webNow.Add(-time.Hour).Unix()}), "n1"},
		"別人的金鑰簽章":           {p.sign(jose.RS256, mustRSA(t), "g1", appClaims(map[string]any{"iss": "https://accounts.google.com", "aud": "google-web-client", "azp": "android-client"})), "n1"},
		"HS256":             {p.sign(jose.HS256, []byte("0123456789abcdef0123456789abcdef"), "g1", appClaims(map[string]any{"iss": "https://accounts.google.com", "aud": "google-web-client", "azp": "android-client"})), "n1"},
		"不是 JWT":            {"garbage", "n1"},
	} {
		if _, err := a.Verify(t.Context(), tc.token, tc.nonce); !errors.Is(err, model.ErrInvalidToken) {
			t.Errorf("%s：err = %v，預期 ErrInvalidToken", name, err)
		}
	}
	if _, err := a.Verify(t.Context(), p.googleToken(nil), ""); err == nil || errors.Is(err, model.ErrInvalidToken) {
		t.Errorf("空的 nonce：err = %v，預期程式錯誤", err)
	}
}

func TestLineAppLoginVerifiesTheES256IDTokenWithTheJWKS(t *testing.T) {
	t.Parallel()
	p := newAppIdP(t)
	a := p.line(t)
	id, err := a.Verify(t.Context(), p.lineToken(map[string]any{"email": "a@example.com"}), "n1")
	if err != nil {
		t.Fatal(err)
	}
	if id.Provider != model.ProviderLine || id.Subject != "sub-1" || id.Email != "a@example.com" || id.EmailVerified {
		t.Errorf("id = %+v，LINE 的 email 一律不是已驗證", id)
	}
	other, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	web := appClaims(map[string]any{"iss": "https://access.line.me", "aud": "line-channel"})
	for name, tc := range map[string]struct {
		token string
		nonce string
	}{
		"nonce 不符":         {p.lineToken(nil), "other"},
		"aud 不是 channel":   {p.lineToken(map[string]any{"aud": "other-channel"}), "n1"},
		"iss 不對":           {p.lineToken(map[string]any{"iss": "https://accounts.google.com"}), "n1"},
		"過期":               {p.lineToken(map[string]any{"exp": webNow.Add(-time.Hour).Unix()}), "n1"},
		"別人的金鑰簽章":          {p.sign(jose.ES256, other, "l1", web), "n1"},
		"不認得的 kid":         {p.sign(jose.ES256, p.ecKey, "unknown", web), "n1"},
		"HS256（web 用的演算法）": {p.sign(jose.HS256, []byte("line-channel-secret-0123456789ab"), "l1", web), "n1"},
		"RS256":            {p.sign(jose.RS256, p.rsaKey, "l1", web), "n1"},
	} {
		if _, err := a.Verify(t.Context(), tc.token, tc.nonce); !errors.Is(err, model.ErrInvalidToken) {
			t.Errorf("%s：err = %v，預期 ErrInvalidToken", name, err)
		}
	}
}

func TestAppLoginWithUnreachableKeysIsNotAnInvalidToken(t *testing.T) {
	t.Parallel()
	p := newAppIdP(t)
	a, err := NewAppLogin(AppLoginConfig{Provider: model.ProviderLine, ClientID: "line-channel", JWKSURL: p.srv.URL + "/missing", HTTPClient: p.srv.Client(), Now: func() time.Time { return webNow }})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Verify(t.Context(), p.lineToken(nil), "n1"); err == nil || errors.Is(err, model.ErrInvalidToken) {
		t.Errorf("err = %v，抓不到公鑰是 provider 故障，不是 token 無效", err)
	}
}

func TestNewAppLoginRejectsIncompleteOrUnknownConfig(t *testing.T) {
	t.Parallel()
	for name, cfg := range map[string]AppLoginConfig{
		"沒有 client ID":        {Provider: model.ProviderLine},
		"Google 沒有 azp 清單":    {Provider: model.ProviderGoogle, ClientID: "c"},
		"Google 的 azp 清單有空字串": {Provider: model.ProviderGoogle, ClientID: "c", AuthorizedParties: []string{""}},
		"LINE 設了 azp":         {Provider: model.ProviderLine, ClientID: "c", AuthorizedParties: []string{"a"}},
		"未知的 provider":        {Provider: model.Provider("FACEBOOK"), ClientID: "c"},
		"空的 provider":         {ClientID: "c"},
	} {
		if _, err := NewAppLogin(cfg); err == nil {
			t.Errorf("%s：沒有回錯", name)
		}
	}
}

func mustRSA(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return k
}
