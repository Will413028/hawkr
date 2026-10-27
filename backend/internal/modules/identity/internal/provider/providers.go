package provider

import (
	"context"
	"crypto/rsa"
	"errors"
	"fmt"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/identity/internal/apple"
	"github.com/Will413028/hawkr/backend/internal/modules/identity/internal/clerk"
	"github.com/Will413028/hawkr/backend/internal/modules/identity/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/identity/internal/oidc"
	"github.com/Will413028/hawkr/backend/internal/platform/seal"
)

// 以下是真正的 provider，給組裝點用；測試以假的實作取代 Providers 的欄位。

// NewClerk 建立一個 Clerk application 的 client。
func NewClerk(cfg model.ClerkConfig) (model.Clerk, error) {
	c, err := clerk.New(clerk.Config{PublishableKey: cfg.PublishableKey, SecretKey: cfg.SecretKey, JWTKey: cfg.JWTKey, AuthorizedParties: cfg.AuthorizedParties})
	if err != nil {
		return nil, fmt.Errorf("identity：%w", err)
	}
	return clerkApp{c}, nil
}

// 本機開發的 token 的 issuer：顧客與後台各一個（hawkr dev-token）。
const (
	DevIssuer      = clerk.DevIssuer
	DevAdminIssuer = "hawkr-dev-admin"
)

// NewDevClerk 建立本機開發用的顧客 application：接受 key 簽的 dev token（issuer 是 DevIssuer），
// 使用者是 dev_ 開頭的 ID，不連 Clerk。只在設定明確指定時使用。
func NewDevClerk(key *rsa.PublicKey, now func() time.Time) (model.Clerk, error) {
	d, err := clerk.NewDev(key, now)
	if err != nil {
		return nil, fmt.Errorf("identity：%w", err)
	}
	return clerkApp{d}, nil
}

// SignDevToken 以本機的金鑰簽一個 dev token（hawkr dev-token 與測試用）。userID 要以 dev_ 開頭。
func SignDevToken(key *rsa.PrivateKey, issuer, userID string, ttl time.Duration, now time.Time) (string, error) {
	return clerk.SignDev(key, issuer, userID, ttl, now)
}

// clerkBackend 是 clerk.Client 與 clerk.Dev 共同的方法。
type clerkBackend interface {
	Issuer() string
	OwnsUser(id string) bool
	Verify(ctx context.Context, token string) (oidc.Claims, error)
	BanUser(ctx context.Context, id string) error
	DeleteUser(ctx context.Context, id string) error
}

// clerkApp 把 clerk 套件的錯誤換成本模組的（model.ErrInvalidToken、model.ErrRejected）。
type clerkApp struct{ c clerkBackend }

func (a clerkApp) Issuer() string          { return a.c.Issuer() }
func (a clerkApp) OwnsUser(id string) bool { return a.c.OwnsUser(id) }

func (a clerkApp) Verify(ctx context.Context, token string) (model.ClerkIdentity, error) {
	c, err := a.c.Verify(ctx, token)
	if errors.Is(err, clerk.ErrInvalid) {
		return model.ClerkIdentity{}, errors.Join(err, model.ErrInvalidToken)
	}
	return model.ClerkIdentity{Subject: c.Subject, Email: c.Email, Name: c.Name}, err
}

func (a clerkApp) BanUser(ctx context.Context, id string) error {
	return rejected(a.c.BanUser(ctx, id))
}
func (a clerkApp) DeleteUser(ctx context.Context, id string) error {
	return rejected(a.c.DeleteUser(ctx, id))
}

func rejected(err error) error {
	if errors.Is(err, clerk.ErrRejected) {
		return errors.Join(err, model.ErrRejected)
	}
	return err
}

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
