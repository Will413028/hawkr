package identity

import (
	"context"
	"crypto/rsa"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/identity/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/identity/internal/provider"
)

// ClerkConfig 是 Clerk application 的設定。
type ClerkConfig = model.ClerkConfig

// AccessConfig 是後台 Access JWT 驗證的設定。
type AccessConfig = model.AccessConfig

// AppleConfig 是 Sign in with Apple 金鑰設定。
type AppleConfig = model.AppleConfig

// DevAuth 是本機的顧客與後台測試登入。
type DevAuth = provider.DevAuth

const (
	DevIssuer      = provider.DevIssuer
	DevAdminIssuer = provider.DevAdminIssuer
)

// NewClerk 建立 Clerk application 的 client。
func NewClerk(cfg ClerkConfig) (Clerk, error) { return provider.NewClerk(cfg) }

// NewDevClerk 建立接受 dev token 的顧客 application，不連 Clerk。
func NewDevClerk(key *rsa.PublicKey, now func() time.Time) (Clerk, error) {
	return provider.NewDevClerk(key, now)
}

// SignDevToken 以本機金鑰簽 dev token；userID 必須以 dev_ 開頭。
func SignDevToken(key *rsa.PrivateKey, issuer, userID string, ttl time.Duration, now time.Time) (string, error) {
	return provider.SignDevToken(key, issuer, userID, ttl, now)
}

// NewGoogleVerifier 建立 vendor portal step-up 的 Google ID token verifier。
func NewGoogleVerifier(audiences []string, lifetime ...context.Context) (IDTokenVerifier, error) {
	return provider.NewGoogleVerifier(audiences, lifetime...)
}

// NewAppleVerifier 建立 Sign in with Apple ID token verifier。
func NewAppleVerifier(audiences []string, lifetime ...context.Context) (IDTokenVerifier, error) {
	return provider.NewAppleVerifier(audiences, lifetime...)
}

// NewAppleTokens 建立 authorization code 交換與撤銷的 client。
func NewAppleTokens(cfg AppleConfig) (AppleTokens, error) { return provider.NewAppleTokens(cfg) }

// NewSealer 建立 job 憑證的 AES-256-GCM 加密器。
func NewSealer(keys string) (Sealer, error) { return provider.NewSealer(keys) }

// NewAccess 建立後台 Access JWT verifier。
func NewAccess(cfg AccessConfig) (AdminVerifier, error) { return provider.NewAccess(cfg) }

// NewDevAdmin 建立 dev admin token verifier。
func NewDevAdmin(key *rsa.PublicKey, now func() time.Time) (AdminVerifier, error) {
	return provider.NewDevAdmin(key, now)
}

// NewDevAuth 建立本機 dev 登入；key 為 nil 時產生測試金鑰。
func NewDevAuth(key *rsa.PrivateKey) (*DevAuth, error) { return provider.NewDevAuth(key) }
