package provider

import (
	"context"
	"crypto/rsa"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/identity/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/identity/internal/oidc"
)

const devAdminAudience = "hawkr-dev-admin-application"

type accessVerifier struct {
	issuer string
	v      *oidc.Verifier
}

// NewAccess 使用 team domain 的 JWKS 驗證 Access JWT。
func NewAccess(cfg model.AccessConfig) (model.AdminVerifier, error) {
	u, err := url.Parse(cfg.TeamDomain)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || strings.TrimSpace(cfg.Audience) == "" {
		return nil, errors.New("identity：Access 需要 HTTPS team domain 與 application AUD tag")
	}
	issuer := u.String()
	return newAccessVerifier(oidc.Config{
		Issuers: []string{issuer}, JWKSURL: issuer + "/cdn-cgi/access/certs", Audiences: []string{cfg.Audience}, HTTPClient: cfg.HTTPClient, Context: cfg.Context,
	})
}

func newAccessVerifier(cfg oidc.Config) (model.AdminVerifier, error) {
	v, err := oidc.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("identity：Access：%w", err)
	}
	return accessVerifier{issuer: cfg.Issuers[0], v: v}, nil
}

func (a accessVerifier) Issuer() string { return a.issuer }

func (a accessVerifier) Verify(ctx context.Context, token string) (model.AdminIdentity, error) {
	c, err := a.v.Verify(ctx, token)
	if errors.Is(err, oidc.ErrInvalid) {
		return model.AdminIdentity{}, errors.Join(err, model.ErrInvalidToken)
	}
	if err != nil {
		return model.AdminIdentity{}, err
	}
	if strings.TrimSpace(c.Email) == "" {
		return model.AdminIdentity{}, errors.Join(errors.New("access token 沒有 email"), model.ErrInvalidToken)
	}
	return model.AdminIdentity{Subject: c.Subject, Email: c.Email}, nil
}

// NewDevAdmin 用 Access 驗證路徑與本機公鑰驗證 dev admin token。
func NewDevAdmin(key *rsa.PublicKey, now func() time.Time) (model.AdminVerifier, error) {
	return newAccessVerifier(oidc.Config{
		Issuers:   []string{DevAdminIssuer},
		PublicKey: key,
		Audiences: []string{devAdminAudience},
		Now:       now,
	})
}
