package provider

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/identity/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/identity/internal/oidc"
)

// AppLoginConfig 是一個 provider 的 App 原生登入（驗 SDK 簽發的 ID token）設定。
type AppLoginConfig struct {
	Provider model.Provider
	// ClientID 是 ID token 的 aud：Google 是 web client ID（Android 的 requestIdToken、iOS 的 serverClientID 都設成它，aud 因此統一，
	// spikes.md §c）；LINE 是 channel ID。
	ClientID string
	// AuthorizedParties 是 Google ID token 的 azp 允許清單（取得 token 的 Android、iOS client ID）；Google 必填，LINE 不能設。
	AuthorizedParties []string
	// JWKSURL 覆寫公鑰端點，只給測試指向假的 provider；空字串用正式的。
	JWKSURL    string
	HTTPClient *http.Client
	Now        func() time.Time
	// Context 是 JWKS 快取的生命週期；nil 時用 Background。
	Context context.Context
}

// AppLogin 驗證一個 provider 的原生 SDK 簽發給 App 的 ID token（ADR 2026-10-30-identity-self-hosted-federated-login D2）。
type AppLogin struct {
	provider model.Provider
	parties  []string
	verifier *oidc.Verifier
}

// NewAppLogin 建立 Google 或 LINE 的 App 登入驗證：Google 的 ID token 以 JWKS 驗（RS256），iss 接受 https://accounts.google.com 與
// accounts.google.com，aud 是 web client ID，azp 必須在允許清單；LINE 原生 SDK 的以 JWKS 驗（ES256，spikes.md §a），aud 是 channel ID。
func NewAppLogin(cfg AppLoginConfig) (*AppLogin, error) {
	if cfg.ClientID == "" {
		return nil, errors.New("identity：App 登入需要 client ID")
	}
	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: webLoginTimeout}
	}
	oc := oidc.Config{Audiences: []string{cfg.ClientID}, HTTPClient: hc, Now: cfg.Now, Context: cfg.Context}
	a := &AppLogin{provider: cfg.Provider}
	switch cfg.Provider {
	case model.ProviderGoogle:
		if len(cfg.AuthorizedParties) == 0 || slices.Contains(cfg.AuthorizedParties, "") {
			return nil, errors.New("identity：Google 的 App 登入需要 azp 允許清單")
		}
		a.parties = slices.Clone(cfg.AuthorizedParties)
		oc.Issuers = []string{oidc.GoogleIssuer, oidc.GoogleIssuerNoScheme}
		oc.JWKSURL = cmpOr(cfg.JWKSURL, oidc.GoogleJWKS)
	case model.ProviderLine:
		if len(cfg.AuthorizedParties) != 0 {
			return nil, errors.New("identity：LINE 的 App 登入沒有 azp")
		}
		oc.Issuers = []string{lineIssuer}
		oc.JWKSURL = cmpOr(cfg.JWKSURL, oidc.LineJWKS)
		oc.Algorithm = "ES256"
	default:
		return nil, fmt.Errorf("identity：%q 沒有 App 登入", cfg.Provider)
	}
	v, err := oidc.New(oc)
	if err != nil {
		return nil, fmt.Errorf("identity：%s 的 App ID token 驗證器：%w", cfg.Provider, err)
	}
	a.verifier = v
	return a, nil
}

// Provider 回傳這個驗證屬於哪一家。
func (a *AppLogin) Provider() model.Provider { return a.provider }

// Verify 驗證 ID token（簽章、iss、aud、exp；Google 另驗 azp），nonce 必須等於 wantNonce，回傳 provider 的身分。無效的 token 與 nonce 不符回含
// model.ErrInvalidToken 的錯誤；抓不到公鑰是沒有分類的錯誤。nonce 的單次使用由呼叫方負責。
func (a *AppLogin) Verify(ctx context.Context, idToken, wantNonce string) (model.ProviderIdentity, error) {
	if wantNonce == "" {
		return model.ProviderIdentity{}, errors.New("identity：App 登入的 nonce 是空的")
	}
	c, err := a.verifier.Verify(ctx, idToken)
	if errors.Is(err, oidc.ErrInvalid) {
		return model.ProviderIdentity{}, errors.Join(err, model.ErrInvalidToken)
	}
	if err != nil {
		return model.ProviderIdentity{}, fmt.Errorf("identity：%s 驗證 App 的 ID token：%w", a.provider, err)
	}
	if subtle.ConstantTimeCompare([]byte(c.Nonce), []byte(wantNonce)) != 1 {
		return model.ProviderIdentity{}, fmt.Errorf("identity：%s 的 ID token nonce 不符：%w", a.provider, model.ErrInvalidToken)
	}
	if a.provider == model.ProviderGoogle && !slices.Contains(a.parties, c.AuthorizedParty) {
		return model.ProviderIdentity{}, fmt.Errorf("identity：Google 的 ID token azp %q 不在允許清單：%w", c.AuthorizedParty, model.ErrInvalidToken)
	}
	return model.ProviderIdentity{
		Provider: a.provider, Subject: c.Subject, Email: c.Email, EmailVerified: c.EmailVerified,
		HostedDomain: c.HostedDomain, Name: c.Name,
	}, nil
}
