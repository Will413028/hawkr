package provider

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"net/http"
	"time"

	"golang.org/x/oauth2"

	"github.com/Will413028/hawkr/backend/internal/modules/identity/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/identity/internal/oidc"
)

const (
	googleAuthURL     = "https://accounts.google.com/o/oauth2/v2/auth"
	googleExchangeURL = "https://oauth2.googleapis.com/token"
	lineIssuer        = "https://access.line.me"
	lineAuthURL       = "https://access.line.me/oauth2/v2.1/authorize"
	lineExchangeURL   = "https://api.line.me/oauth2/v2.1/token"

	// webLoginTimeout 是 web 登入對 provider 的外部呼叫（換 token、抓公鑰）的時間上限。
	webLoginTimeout = 10 * time.Second
)

// WebEndpoints 覆寫 provider 的端點，只給測試指向假的 provider；零值用正式的。
type WebEndpoints struct {
	AuthURL  string
	TokenURL string
	JWKSURL  string
}

// WebLoginConfig 是一個 provider 的 web 登入（authorization code ＋ PKCE）設定。RedirectURL 是這家獨立的 callback 網址。
type WebLoginConfig struct {
	Provider     model.Provider
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Endpoints    WebEndpoints
	HTTPClient   *http.Client
	Now          func() time.Time
	// Context 是 JWKS 快取的生命週期（Google）；nil 時用 Background。
	Context context.Context
}

// WebLogin 是一個 provider 的 web 登入：組授權網址、以 authorization code 換 ID token 並驗證（ADR 2026-10-30-identity-self-hosted-federated-login D3）。
// provider 的 access／refresh token 換完即丟，不保存。
type WebLogin struct {
	provider model.Provider
	issuer   string
	oauth    oauth2.Config
	verifier *oidc.Verifier
	hc       *http.Client
}

// NewWebLogin 建立 Google 或 LINE 的 web 登入。Google 的 ID token 以 JWKS 驗（RS256）；LINE web 的以 channel secret 驗（HS256，
// 計畫 2026-10-30-self-hosted-auth 附件 spikes.md §a）；aud 都是 ClientID。
func NewWebLogin(cfg WebLoginConfig) (*WebLogin, error) {
	if cfg.ClientID == "" || cfg.ClientSecret == "" || cfg.RedirectURL == "" {
		return nil, errors.New("identity：web 登入需要 client ID、client secret 與 redirect URL")
	}
	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: webLoginTimeout}
	}
	w := &WebLogin{provider: cfg.Provider, hc: hc}
	oc := oidc.Config{Audiences: []string{cfg.ClientID}, HTTPClient: hc, Now: cfg.Now, Context: cfg.Context}
	endpoint := oauth2.Endpoint{AuthStyle: oauth2.AuthStyleInParams}
	var scopes []string
	switch cfg.Provider {
	case model.ProviderGoogle:
		w.issuer = oidc.GoogleIssuer
		oc.Issuers = []string{oidc.GoogleIssuer, oidc.GoogleIssuerNoScheme}
		oc.JWKSURL = cmpOr(cfg.Endpoints.JWKSURL, oidc.GoogleJWKS)
		endpoint.AuthURL = cmpOr(cfg.Endpoints.AuthURL, googleAuthURL)
		endpoint.TokenURL = cmpOr(cfg.Endpoints.TokenURL, googleExchangeURL)
		scopes = []string{"openid", "email", "profile"}
	case model.ProviderLine:
		w.issuer = lineIssuer
		oc.Issuers = []string{lineIssuer}
		oc.HMACSecret = []byte(cfg.ClientSecret)
		endpoint.AuthURL = cmpOr(cfg.Endpoints.AuthURL, lineAuthURL)
		endpoint.TokenURL = cmpOr(cfg.Endpoints.TokenURL, lineExchangeURL)
		scopes = []string{"openid", "profile", "email"}
	default:
		return nil, fmt.Errorf("identity：%q 沒有 web 登入", cfg.Provider)
	}
	v, err := oidc.New(oc)
	if err != nil {
		return nil, fmt.Errorf("identity：%s 的 ID token 驗證器：%w", cfg.Provider, err)
	}
	w.verifier = v
	w.oauth = oauth2.Config{ClientID: cfg.ClientID, ClientSecret: cfg.ClientSecret, Endpoint: endpoint, RedirectURL: cfg.RedirectURL, Scopes: scopes}
	return w, nil
}

func cmpOr(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

// Provider 回傳這個 web 登入屬於哪一家。
func (w *WebLogin) Provider() model.Provider { return w.provider }

// AuthURL 回傳 provider 授權端點的網址：帶 state、nonce 與 PKCE 的 S256 challenge（verifier 留在伺服器端的登入流程）。
func (w *WebLogin) AuthURL(state, nonce, verifier string) string {
	return w.oauth.AuthCodeURL(state, oauth2.S256ChallengeOption(verifier), oauth2.SetAuthURLParam("nonce", nonce))
}

// CheckIssuer 回報授權回應的 iss 參數是不是這家 provider 的 issuer（RFC 9207）。
func (w *WebLogin) CheckIssuer(iss string) bool {
	return subtle.ConstantTimeCompare([]byte(iss), []byte(w.issuer)) == 1
}

// RequiresIssuer 回報這家 provider 的授權回應一定帶 iss：Google 的 discovery 宣告 authorization_response_iss_parameter_supported，
// LINE 沒有（計畫 2026-10-30-self-hosted-auth 附件 spikes.md §a、§c；RFC 9207 §2.4）。
func (w *WebLogin) RequiresIssuer() bool { return w.provider == model.ProviderGoogle }

// Exchange 以 authorization code（帶 PKCE verifier）換 token，驗證 ID token（簽章、iss、aud、exp，nonce 必須等於 wantNonce），
// 回傳 provider 的身分。ID token 無效、nonce 不符回含 model.ErrInvalidToken 的錯誤；provider 拒絕這個 code（4xx）回含 model.ErrRejected 的錯誤；
// 其他（連不上、5xx）是沒有分類的錯誤。access／refresh token 不外流。
func (w *WebLogin) Exchange(ctx context.Context, code, verifier, wantNonce string) (model.ProviderIdentity, error) {
	if wantNonce == "" {
		return model.ProviderIdentity{}, errors.New("identity：web 登入的 nonce 是空的")
	}
	ctx, cancel := context.WithTimeout(ctx, webLoginTimeout)
	defer cancel()
	ctx = context.WithValue(ctx, oauth2.HTTPClient, w.hc)
	tok, err := w.oauth.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		var re *oauth2.RetrieveError
		if errors.As(err, &re) && re.Response != nil && re.Response.StatusCode >= 400 && re.Response.StatusCode < 500 {
			return model.ProviderIdentity{}, errors.Join(err, model.ErrRejected)
		}
		return model.ProviderIdentity{}, fmt.Errorf("identity：%s 換 token：%w", w.provider, err)
	}
	raw, _ := tok.Extra("id_token").(string)
	if raw == "" {
		return model.ProviderIdentity{}, fmt.Errorf("identity：%s 的 token 回應沒有 ID token：%w", w.provider, model.ErrInvalidToken)
	}
	c, err := w.verifier.Verify(ctx, raw)
	if errors.Is(err, oidc.ErrInvalid) {
		return model.ProviderIdentity{}, errors.Join(err, model.ErrInvalidToken)
	}
	if err != nil {
		return model.ProviderIdentity{}, fmt.Errorf("identity：%s 驗證 ID token：%w", w.provider, err)
	}
	if subtle.ConstantTimeCompare([]byte(c.Nonce), []byte(wantNonce)) != 1 {
		return model.ProviderIdentity{}, fmt.Errorf("identity：%s 的 ID token nonce 不符：%w", w.provider, model.ErrInvalidToken)
	}
	return model.ProviderIdentity{
		Provider: w.provider, Subject: c.Subject, Email: c.Email, EmailVerified: c.EmailVerified,
		HostedDomain: c.HostedDomain, Name: c.Name,
	}, nil
}
