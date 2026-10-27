package provider

import (
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/identity/internal/clerk"
	"github.com/Will413028/hawkr/backend/internal/modules/identity/internal/model"
)

// DevAuth 是本機開發與測試的登入（ADR 2026-09-27-identity-clerk）：一把 RSA 金鑰，與接受它簽的 dev token 的顧客、
// 後台 application。不連 Clerk；dev 使用者是 dev_ 開頭的 ID，第一個請求時建立帳號或管理員。
type DevAuth struct {
	key       *rsa.PrivateKey
	customers model.Clerk
	admins    model.AdminVerifier
	now       func() time.Time
}

// NewDevAuth 以 key 建立 DevAuth；key 是 nil 時產生一把新的（測試用）。
func NewDevAuth(key *rsa.PrivateKey) (*DevAuth, error) {
	if key == nil {
		var err error
		if key, err = rsa.GenerateKey(rand.Reader, 2048); err != nil {
			return nil, err
		}
	}
	customers, err := NewDevClerk(&key.PublicKey, nil)
	if err != nil {
		return nil, err
	}
	admins, err := NewDevAdmin(&key.PublicKey, nil)
	if err != nil {
		return nil, err
	}
	return &DevAuth{key: key, customers: customers, admins: admins, now: time.Now}, nil //nolint:forbidigo // assigns the clock dependency without calling it
}

// Customers 是接受 CustomerToken 的顧客 application；Admins 是接受 AdminToken 的後台 application。
func (d *DevAuth) Customers() model.Clerk      { return d.customers }
func (d *DevAuth) Admins() model.AdminVerifier { return d.admins }

// devTokenTTL 是測試與本機開發的 token 效期。
const devTokenTTL = 24 * time.Hour

// CustomerToken 簽一個買家、賣家的 dev token；user 要以 dev_ 開頭。
func (d *DevAuth) CustomerToken(user string) (string, error) {
	return d.sign(DevIssuer, user)
}

// AdminToken 簽一個後台的 dev token；user 要以 dev_ 開頭。
func (d *DevAuth) AdminToken(user string) (string, error) {
	return d.sign(DevAdminIssuer, user)
}

func (d *DevAuth) sign(issuer, user string) (string, error) {
	var audience []string
	if issuer == DevAdminIssuer {
		audience = []string{devAdminAudience}
	}
	tok, err := clerk.SignDev(d.key, issuer, user, devTokenTTL, d.now(), audience...)
	if err != nil {
		return "", fmt.Errorf("identity：%w", err)
	}
	return tok, nil
}
