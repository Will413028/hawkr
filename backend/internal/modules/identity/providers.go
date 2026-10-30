package identity

import (
	"context"
	"crypto/rsa"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/identity/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/identity/internal/provider"
)

// AccessConfig 是後台 Access JWT 驗證的設定。
type AccessConfig = model.AccessConfig

// DevAuth 是本機開發與測試的後台登入；顧客的 dev 登入是資料庫裡的 dev session，不經過這裡。
type DevAuth = provider.DevAuth

const DevAdminIssuer = provider.DevAdminIssuer

// WebLogin 是一個 provider 的 web 登入（授權網址、換 code、驗 ID token）；WebLoginConfig 是它的設定。
type (
	WebLogin       = provider.WebLogin
	WebLoginConfig = provider.WebLoginConfig
	WebEndpoints   = provider.WebEndpoints
)

// NewWebLogin 建立 Google 或 LINE 的 web 登入；cfg.Provider 指定哪一家。
func NewWebLogin(cfg WebLoginConfig) (*WebLogin, error) { return provider.NewWebLogin(cfg) }

// AppLogin 驗證一個 provider 的原生 SDK 簽發給 App 的 ID token；AppLoginConfig 是它的設定。
type (
	AppLogin       = provider.AppLogin
	AppLoginConfig = provider.AppLoginConfig
)

// NewAppLogin 建立 Google 或 LINE 的 App 登入驗證；cfg.Provider 指定哪一家。
func NewAppLogin(cfg AppLoginConfig) (*AppLogin, error) { return provider.NewAppLogin(cfg) }

// NewGoogleVerifier 建立 vendor portal step-up 的 Google ID token verifier。
func NewGoogleVerifier(audiences []string, lifetime ...context.Context) (IDTokenVerifier, error) {
	return provider.NewGoogleVerifier(audiences, lifetime...)
}

// NewAccess 建立後台 Access JWT verifier。
func NewAccess(cfg AccessConfig) (AdminVerifier, error) { return provider.NewAccess(cfg) }

// NewDevAdmin 建立 dev admin token verifier。
func NewDevAdmin(key *rsa.PublicKey, now func() time.Time) (AdminVerifier, error) {
	return provider.NewDevAdmin(key, now)
}

// NewDevAuth 建立本機 dev 登入；key 為 nil 時產生測試金鑰。
func NewDevAuth(key *rsa.PrivateKey) (*DevAuth, error) { return provider.NewDevAuth(key) }
