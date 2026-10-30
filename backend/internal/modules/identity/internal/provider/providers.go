package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/Will413028/hawkr/backend/internal/modules/identity/internal/apple"
	"github.com/Will413028/hawkr/backend/internal/modules/identity/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/identity/internal/oidc"
	"github.com/Will413028/hawkr/backend/internal/platform/seal"
)

// 以下是真正的 provider，給組裝點用；測試以假的實作取代 Providers 的欄位。

// 本機開發的後台 token 的 issuer（hawkr dev-token -admin）。
const DevAdminIssuer = "hawkr-dev-admin"

// NewGoogleVerifier 驗證 Google 的 ID token。audiences 是接受的 OAuth client ID；給 vendor portal 的 step-up。
func NewGoogleVerifier(audiences []string, lifetime ...context.Context) (model.IDTokenVerifier, error) {
	cfg := googleConfig(audiences)
	if len(lifetime) > 0 {
		cfg.Context = lifetime[0]
	}
	v, err := oidc.New(cfg)
	if err != nil {
		return nil, err
	}
	return verifier{v}, nil
}

func googleConfig(audiences []string) oidc.Config {
	return oidc.Config{Issuers: []string{oidc.GoogleIssuer, oidc.GoogleIssuerNoScheme}, JWKSURL: oidc.GoogleJWKS, Audiences: audiences}
}

// NewAppleVerifier 驗證 Sign in with Apple 的 ID token。audiences 是兩支 App 的 bundle ID 與 web 的 Services ID。
func NewAppleVerifier(audiences []string, lifetime ...context.Context) (model.IDTokenVerifier, error) {
	cfg := oidc.Config{Issuers: []string{oidc.AppleIssuer}, JWKSURL: oidc.AppleJWKS, Audiences: audiences}
	if len(lifetime) > 0 {
		cfg.Context = lifetime[0]
	}
	v, err := oidc.New(cfg)
	if err != nil {
		return nil, err
	}
	return verifier{v}, nil
}

type verifier struct{ v *oidc.Verifier }

func (v verifier) Verify(ctx context.Context, token string) (model.Identity, error) {
	c, err := v.v.Verify(ctx, token)
	if errors.Is(err, oidc.ErrInvalid) {
		return model.Identity{}, errors.Join(err, model.ErrInvalidToken)
	}
	if err != nil {
		return model.Identity{}, err
	}
	return model.Identity{Subject: c.Subject, Email: c.Email, EmailVerified: c.EmailVerified, Audience: c.Audience, Nonce: c.Nonce, IssuedAt: c.IssuedAt}, nil
}

// NewAppleTokens 以 authorization code 換 refresh token、並撤銷它。
func NewAppleTokens(cfg model.AppleConfig) (model.AppleTokens, error) {
	c, err := apple.New(apple.Config{TeamID: cfg.TeamID, KeyID: cfg.KeyID, PrivateKeyPEM: cfg.PrivateKeyPEM, RedirectURIs: cfg.RedirectURIs})
	if err != nil {
		return nil, err
	}
	return appleTokens{c}, nil
}

type appleTokens struct{ c *apple.Client }

func (a appleTokens) Exchange(ctx context.Context, clientID, code string) (model.AppleGrant, error) {
	rt, idToken, err := a.c.Exchange(ctx, clientID, code)
	if errors.Is(err, apple.ErrRejected) {
		return model.AppleGrant{}, errors.Join(err, model.ErrRejected)
	}
	return model.AppleGrant{RefreshToken: rt, IDToken: idToken}, err
}

func (a appleTokens) Revoke(ctx context.Context, clientID, token string) error {
	err := a.c.Revoke(ctx, clientID, token)
	if errors.Is(err, apple.ErrRejected) {
		return errors.Join(err, model.ErrRejected)
	}
	return err
}

// NewSealer 以 AES-256-GCM 加密排入 job 的第三方憑證。keys 是逗號分隔的 id:base64(32 bytes)，第一把是目前的金鑰；
// 換金鑰時把新的放在最前面、舊的留到它加密的資料都不在了。
func NewSealer(keys string) (model.Sealer, error) {
	k, err := seal.Parse(keys)
	if err != nil {
		return nil, fmt.Errorf("identity：%w", err)
	}
	return sealer{k}, nil
}

// sealer 把金鑰環接成 model.Sealer；AAD 由呼叫方依用途與帳號給。
type sealer struct{ k *seal.Keyring }

func (s sealer) Seal(p, aad []byte) ([]byte, error) { return s.k.Seal(p, aad) }
func (s sealer) Open(c, aad []byte) ([]byte, error) { return s.k.Open(c, aad) }
func (s sealer) Has(id string) bool                 { return s.k.Has(id) }
