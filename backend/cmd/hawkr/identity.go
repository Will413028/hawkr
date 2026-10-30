package main

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	httpidentity "github.com/Will413028/hawkr/backend/internal/httpapi/identity"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/platform/config"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

// identityConfig 是 identity 模組的外部 provider。
type identityConfig struct {
	providers identity.Providers
	// webLogin 是 web 聯合登入已設定的 provider 的 client（Google、LINE）；沒設定的 provider，/api/auth/{provider}/start 回 404。
	webLogin map[identity.Provider]webClient
}

// webClient 是 web 登入在一家 provider 的 OAuth client。
type webClient struct{ id, secret string }

// readIdentityConfig 讀 identity 的設定（ADR 2026-09-27-identity-clerk）。每組設定整組選填：沒有設定時那個功能回
// unavailable 或無法登入；只設了一部分是設定錯誤。
//
//   - CLERK_PUBLISHABLE_KEY、CLERK_SECRET_KEY、CLERK_JWT_KEY：顧客的 Clerk application；CLERK_AUTHORIZED_PARTIES 是 web 的 origin
//     （web 取得的 token 帶 azp）。
//   - CF_ACCESS_TEAM_DOMAIN、CF_ACCESS_ADMIN_AUD：後台的 Access team domain 與 application AUD tag。
//   - DEV_AUTH_KEY_FILE：本機開發的 dev token 的 RSA 私鑰（make dev-key 產生）；設定時另外接受它簽的 token，不能與
//     正式環境的 Clerk（sk_live_）或 Cloudflare Access 同時設定。
//   - GOOGLE_CLIENT_IDS：vendor portal step-up 的 Google ID token 驗證（VendorPortalInviteeService.VerifySession）。
//   - GOOGLE_WEB_CLIENT_ID、GOOGLE_WEB_CLIENT_SECRET：web 登入（/api/auth/google/*）的 Google OAuth client（authorization code ＋ PKCE），與
//     GOOGLE_CLIENT_IDS（vendor portal step-up 驗 ID token 的 aud）無關；兩個都設才啟用，只設一個是設定錯誤。LINE_CHANNEL_ID、
//     LINE_CHANNEL_SECRET：web 登入（/api/auth/line/*）的 LINE Login channel，同樣整組選填；LINE 的 web ID token 以 channel secret 驗（HS256）。
//     redirect URI 是 PUBLIC_WEB_ORIGIN 加 /api/auth/{google,line}/callback，要登記在各家的 console。
//   - APPLE_CLIENT_IDS、APPLE_TEAM_ID、APPLE_KEY_ID、APPLE_PRIVATE_KEY、IDENTITY_TOKEN_KEYS：App 原生登入交來的 Apple 授權碼換成
//     refresh token 加密保存（LinkAppleCredential），刪帳號時撤銷。設了顧客的 Clerk 又沒有 DEV_AUTH_KEY_FILE（部署的環境）時
//     五個都必填；只設一部分是設定錯誤；本機開發（DEV_AUTH_KEY_FILE）可以全部不設，LinkAppleCredential 回 unavailable。
func readIdentityConfig(ctx context.Context, e *config.Env) identityConfig {
	var c identityConfig
	customers := readClerk(e, "CLERK_PUBLISHABLE_KEY", "CLERK_SECRET_KEY", "CLERK_AUTHORIZED_PARTIES")
	if customers != nil {
		c.providers.Customers = append(c.providers.Customers, customers)
	}
	accessConfigured := anySet(e, "CF_ACCESS_TEAM_DOMAIN", "CF_ACCESS_ADMIN_AUD")
	if accessConfigured {
		admin, err := identity.NewAccess(identity.AccessConfig{TeamDomain: e.String("CF_ACCESS_TEAM_DOMAIN"), Audience: e.String("CF_ACCESS_ADMIN_AUD"), Context: ctx})
		if err != nil {
			e.Fail("CF_ACCESS_TEAM_DOMAIN／CF_ACCESS_ADMIN_AUD：%v", err)
		} else {
			c.providers.Admin = admin
		}
	}
	if path := e.StringOr("DEV_AUTH_KEY_FILE", ""); path != "" {
		if liveIdentityConfigured(e) {
			e.Fail("DEV_AUTH_KEY_FILE 不能與正式 Clerk 或 Cloudflare Access 設定同時使用")
		} else {
			key, err := readDevKey(path)
			if err != nil {
				e.Fail("DEV_AUTH_KEY_FILE：%v", err)
			} else if dev, err := identity.NewDevAuth(key); err != nil {
				e.Fail("DEV_AUTH_KEY_FILE：%v", err)
			} else {
				c.providers.Customers = append(c.providers.Customers, dev.Customers())
				c.providers.Admin = dev.Admins()
			}
		}
	}
	if ids := e.List("GOOGLE_CLIENT_IDS"); len(ids) > 0 {
		v, err := identity.NewGoogleVerifier(ids, ctx)
		if err != nil {
			e.Fail("GOOGLE_CLIENT_IDS：%v", err)
		}
		c.providers.Google = v
	}
	c.webLogin = map[identity.Provider]webClient{}
	for _, w := range []struct {
		provider         identity.Provider
		idKey, secretKey string
	}{
		{identity.ProviderGoogle, "GOOGLE_WEB_CLIENT_ID", "GOOGLE_WEB_CLIENT_SECRET"},
		{identity.ProviderLine, "LINE_CHANNEL_ID", "LINE_CHANNEL_SECRET"},
	} {
		if anySet(e, w.idKey, w.secretKey) {
			c.webLogin[w.provider] = webClient{id: e.String(w.idKey), secret: e.String(w.secretKey)}
		}
	}
	appleKeys := []string{"APPLE_CLIENT_IDS", "APPLE_TEAM_ID", "APPLE_KEY_ID", "APPLE_PRIVATE_KEY", "IDENTITY_TOKEN_KEYS"}
	switch {
	case anySet(e, appleKeys...):
		// 設了任何一個就全部必填：只設一部分不會悄悄停用 Apple。
		c.providers.Apple, c.providers.AppleTokens, c.providers.Sealer = readApple(ctx, e)
	case customers != nil && e.StringOr("DEV_AUTH_KEY_FILE", "") == "":
		// 有顧客的 Clerk 又不是本機開發（沒有 DEV_AUTH_KEY_FILE）：Apple 必填。
		for _, k := range appleKeys {
			e.String(k)
		}
	}
	return c
}

// readApple 讀 Sign in with Apple 的設定（全部必填）：ID token 驗證、authorization code 的交換與撤銷、保存 token 的加密。
func readApple(ctx context.Context, e *config.Env) (identity.IDTokenVerifier, identity.AppleTokens, identity.Sealer) {
	v, err := identity.NewAppleVerifier(e.List("APPLE_CLIENT_IDS"), ctx)
	if err != nil {
		e.Fail("APPLE_CLIENT_IDS：%v", err)
	}
	cfg := identity.AppleConfig{
		TeamID:        e.String("APPLE_TEAM_ID"),
		KeyID:         e.String("APPLE_KEY_ID"),
		PrivateKeyPEM: e.String("APPLE_PRIVATE_KEY"),
		RedirectURIs:  map[string]string{},
	}
	for _, kv := range e.List("APPLE_REDIRECT_URIS") {
		clientID, uri, ok := strings.Cut(kv, "=")
		if !ok {
			e.Fail("APPLE_REDIRECT_URIS 的格式是 client_id=redirect_uri，以逗號分隔")
			continue
		}
		cfg.RedirectURIs[clientID] = uri
	}
	keys := e.String("IDENTITY_TOKEN_KEYS")
	tokens, err := identity.NewAppleTokens(cfg)
	if err != nil {
		e.Fail("APPLE_PRIVATE_KEY：%v", err)
	}
	sealer, err := identity.NewSealer(keys)
	if err != nil {
		e.Fail("IDENTITY_TOKEN_KEYS：%v", err)
	}
	return v, tokens, sealer
}

// readClerk 讀一個 Clerk application 的設定；所有 key 都沒有設定時是 nil。
func readClerk(e *config.Env, publishable, secret, parties string) identity.Clerk {
	if !anySet(e, publishable, secret, "CLERK_JWT_KEY") {
		return nil
	}
	c, err := identity.NewClerk(identity.ClerkConfig{
		PublishableKey: e.String(publishable), SecretKey: e.String(secret), JWTKey: e.String("CLERK_JWT_KEY"), AuthorizedParties: e.List(parties),
	})
	if err != nil {
		e.Fail("%s：%v", publishable, err)
		return nil
	}
	return c
}

// liveIdentityConfigured 是這個環境設定了正式的登入：Clerk 的 live key 或 Cloudflare Access。DEV_AUTH_KEY_FILE 不能與它同時使用。
func liveIdentityConfigured(e *config.Env) bool {
	return strings.HasPrefix(e.StringOr("CLERK_SECRET_KEY", ""), "sk_live_") || anySet(e, "CF_ACCESS_TEAM_DOMAIN", "CF_ACCESS_ADMIN_AUD")
}

// requireDevEnvironment 是只給本機開發的子命令（dev-seed）的保護，前提與 dev-token、serve 接受 dev token 相同：DEV_AUTH_KEY_FILE 指向
// 本機的 dev 金鑰（make dev-key 產生，正式環境沒有），而且沒有正式的 Clerk 或 Cloudflare Access 設定。
func requireDevEnvironment(e *config.Env) error {
	path := e.StringOr("DEV_AUTH_KEY_FILE", "")
	if path == "" {
		return errors.New("這個子命令只給本機開發使用：沒有設定 DEV_AUTH_KEY_FILE（make dev-key 產生）")
	}
	if liveIdentityConfigured(e) {
		return errors.New("這個子命令只給本機開發使用：DEV_AUTH_KEY_FILE 不能與正式 Clerk 或 Cloudflare Access 設定同時使用")
	}
	if _, err := readDevKey(path); err != nil {
		return fmt.Errorf("DEV_AUTH_KEY_FILE：%w", err)
	}
	return nil
}

func anySet(e *config.Env, keys ...string) bool {
	for _, k := range keys {
		if e.StringOr(k, "") != "" {
			return true
		}
	}
	return false
}

// readDevKey 讀 dev token 的 RSA 私鑰（PKCS#8 或 PKCS#1 的 PEM）。
func readDevKey(path string) (*rsa.PrivateKey, error) {
	raw, err := os.ReadFile(path) //nolint:gosec // path is the operator-configured local dev key file
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, errors.New("不是 PEM")
	}
	if k, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		if rk, ok := k.(*rsa.PrivateKey); ok {
			return rk, nil
		}
		return nil, errors.New("不是 RSA 私鑰")
	}
	return x509.ParsePKCS1PrivateKey(block.Bytes)
}

// newIdentity 建立 identity 模組。
func newIdentity(d *db.DB, bus *events.Bus, jc jobs.Enqueuer, c identityConfig, log *slog.Logger) *identity.Module {
	return identity.New(identity.Deps{
		DB: d, Events: bus, Jobs: jc, Providers: c.providers,
		Now: time.Now, Logger: log,
	})
}

// runDevToken 印出本機開發與特性測試的登入 token，不連任何外部 provider：
//
//   - 顧客：在資料庫建一個 dev session（App 種類，帳號沒有時建立，同 serve 一樣建立買家）並印出原文 token；需要 DATABASE_URL，
//     而且 requireDevEnvironment 成立（DEV_AUTH_KEY_FILE 指向本機的 dev 金鑰，沒有正式的 Access 設定）。session 的效期同正式 session。
//
//   - 後台（-admin）：以 DEV_AUTH_KEY_FILE 的金鑰簽 24 小時的 JWT，serve 設了同一個 DEV_AUTH_KEY_FILE 才接受它，管理員在第一個請求時建立。
//
//     hawkr dev-token [-admin] [-user dev_<名稱>]
func runDevToken(ctx context.Context, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("dev-token", flag.ContinueOnError)
	admin := fs.Bool("admin", false, "簽後台的 token（否則是買家、賣家的）")
	user := fs.String("user", "dev_buyer", "使用者的 ID，以 dev_ 開頭；本機 seed 的帳號是 dev_account_<帳號 ID>")
	if err := fs.Parse(args); err != nil {
		return err
	}
	e := config.NewEnv()
	if err := requireDevEnvironment(e); err != nil {
		return err
	}
	if *admin {
		key, err := readDevKey(e.StringOr("DEV_AUTH_KEY_FILE", ""))
		if err != nil {
			return fmt.Errorf("DEV_AUTH_KEY_FILE：%w", err)
		}
		dev, err := identity.NewDevAuth(key)
		if err != nil {
			return err
		}
		tok, err := dev.AdminToken(*user)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(out, tok)
		return err
	}
	dbc := dbConfig(e)
	if err := e.Err(); err != nil {
		return err
	}
	d, err := db.Open(ctx, dbc)
	if err != nil {
		return err
	}
	defer d.Close()
	log := newLogger()
	bus := events.NewBus()
	buyers := buyer.New(buyer.Deps{DB: d, Events: bus, Now: time.Now, Logger: log})
	subscribeBuyerLifecycle(bus, buyers)
	tok, err := identity.New(identity.Deps{DB: d, Events: bus, Now: time.Now, Logger: log}).DevSession(ctx, *user)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, tok)
	return err
}

// newWebLogin 建立 web 聯合登入的路由（ADR 2026-10-30-identity-self-hosted-federated-login D3）。redirect URI 是 web 的網域加各家獨立的 callback 路徑。
// session cookie 沒有設 Max-Age（瀏覽器的 session cookie），伺服器端的閒置與絕對逾時照 session 的規則。
func newWebLogin(ctx context.Context, id *identity.Module, web rpc.WebOrigin, c identityConfig, log *slog.Logger) (*httpidentity.WebLogin, error) {
	var providers []httpidentity.WebProvider
	for _, p := range identity.LoginProviders() {
		client, ok := c.webLogin[p]
		if !ok {
			continue
		}
		redirect, err := web.URL(httpidentity.AuthPathPrefix + httpidentity.PathOf(p) + "/callback")
		if err != nil {
			return nil, err
		}
		w, err := identity.NewWebLogin(identity.WebLoginConfig{Provider: p, ClientID: client.id, ClientSecret: client.secret, RedirectURL: redirect, Context: ctx})
		if err != nil {
			return nil, err
		}
		providers = append(providers, w)
	}
	return httpidentity.NewWebLogin(httpidentity.WebLoginDeps{Module: id, Providers: providers, Logger: log})
}
