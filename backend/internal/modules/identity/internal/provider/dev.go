package provider

import (
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"strings"
	"time"

	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jws"
	"github.com/lestrrat-go/jwx/v3/jwt"

	"github.com/Will413028/hawkr/backend/internal/modules/identity/internal/model"
)

// DevAuth 是本機開發與測試的後台登入：一把 RSA 金鑰，與接受它簽的 dev token 的後台 application，管理員在第一個請求時建立。
// 顧客的 dev 登入是資料庫裡的 dev session（identity.Module.DevSession），不經過這裡。
type DevAuth struct {
	key    *rsa.PrivateKey
	admins model.AdminVerifier
	now    func() time.Time
}

// NewDevAuth 以 key 建立 DevAuth；key 是 nil 時產生一把新的（測試用）。
func NewDevAuth(key *rsa.PrivateKey) (*DevAuth, error) {
	if key == nil {
		var err error
		if key, err = rsa.GenerateKey(rand.Reader, 2048); err != nil {
			return nil, err
		}
	}
	admins, err := NewDevAdmin(&key.PublicKey, nil)
	if err != nil {
		return nil, err
	}
	return &DevAuth{key: key, admins: admins, now: time.Now}, nil //nolint:forbidigo // assigns the clock dependency without calling it
}

// Admins 是接受 AdminToken 的後台 application。
func (d *DevAuth) Admins() model.AdminVerifier { return d.admins }

// devTokenTTL 是測試與本機開發的 token 效期。
const devTokenTTL = 24 * time.Hour

// AdminToken 簽一個後台的 dev token；user 要以 dev_ 開頭。
func (d *DevAuth) AdminToken(user string) (string, error) {
	return d.sign(DevAdminIssuer, user)
}

func (d *DevAuth) sign(issuer, user string) (string, error) {
	var audience []string
	if issuer == DevAdminIssuer {
		audience = []string{devAdminAudience}
	}
	tok, err := signDevToken(d.key, issuer, user, devTokenTTL, d.now(), audience...)
	if err != nil {
		return "", fmt.Errorf("identity：%w", err)
	}
	return tok, nil
}

// devUserPrefix 是 dev 使用者的 ID 前綴。
const devUserPrefix = "dev_"

// signDevToken 以本機的金鑰簽一個 dev token（後台用；測試也用它簽缺 audience 的 token）。userID 要以 dev_ 開頭。
func signDevToken(key *rsa.PrivateKey, issuer, userID string, ttl time.Duration, now time.Time, audience ...string) (string, error) {
	if !strings.HasPrefix(userID, devUserPrefix) {
		return "", fmt.Errorf("dev 使用者的 ID 要以 %s 開頭", devUserPrefix)
	}
	tok := jwt.New()
	tok.Options().Enable(jwt.FlattenAudience)
	for name, value := range map[string]any{"iss": issuer, "sub": userID, "iat": now.Unix(), "nbf": now.Add(-time.Minute).Unix(), "exp": now.Add(ttl).Unix(), "email": userID + "@dev.localhost", "name": userID} {
		if err := tok.Set(name, value); err != nil {
			return "", err
		}
	}
	if len(audience) > 0 {
		if err := tok.Set("aud", audience); err != nil {
			return "", err
		}
	}
	headers := jws.NewHeaders()
	if err := headers.Set(jws.TypeKey, "JWT"); err != nil {
		return "", err
	}
	raw, err := jwt.Sign(tok, jwt.WithKey(jwa.RS256(), key, jws.WithProtectedHeaders(headers)))
	return string(raw), err
}
