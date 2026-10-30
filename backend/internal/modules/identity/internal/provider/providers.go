package provider

import (
	"context"
	"errors"

	"github.com/Will413028/hawkr/backend/internal/modules/identity/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/identity/internal/oidc"
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

type verifier struct{ v *oidc.Verifier }

func (v verifier) Verify(ctx context.Context, token string) (model.Identity, error) {
	c, err := v.v.Verify(ctx, token)
	if errors.Is(err, oidc.ErrInvalid) {
		return model.Identity{}, errors.Join(err, model.ErrInvalidToken)
	}
	if err != nil {
		return model.Identity{}, err
	}
	return model.Identity{Subject: c.Subject, Email: c.Email, EmailVerified: c.EmailVerified, Nonce: c.Nonce, IssuedAt: c.IssuedAt}, nil
}
