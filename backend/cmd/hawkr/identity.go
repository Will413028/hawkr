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
	"time"

	httpidentity "github.com/Will413028/hawkr/backend/internal/httpapi/identity"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/platform/config"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
)

// identityConfig 是 identity 模組的外部 provider。
type identityConfig struct {
	providers identity.Providers
	// webLogin 是 web 聯合登入已設定的 provider 的 client（Google、LINE）；沒設定的 provider，/api/auth/{provider}/start 回 404。
	webLogin map[identity.Provider]webClient
	// googleAppClients 是 App 原生登入的 Google ID token 的 azp 允許清單（Android、iOS 的 client ID）；空的時候 Google 的 App 登入不啟用。
	googleAppClients []string
}

// webClient 是 web 登入在一家 provider 的 OAuth client。
type webClient struct{ id, secret string }

// readIdentityConfig 讀 identity 的設定（ADR 2026-10-30-identity-self-hosted-federated-login）。每組設定整組選填：沒有設定時那個功能回
// unavailable 或無法登入；只設了一部分是設定錯誤。
//
//   - CF_ACCESS_TEAM_DOMAIN、CF_ACCESS_ADMIN_AUD：後台的 Access team domain 與 application AUD tag。
//   - DEV_AUTH_KEY_FILE：本機開發的 dev token 的 RSA 私鑰（make dev-key 產生）；設定時另外接受它簽的後台 token，不能與
//     Cloudflare Access 同時設定。
//   - GOOGLE_CLIENT_IDS：vendor portal step-up 的 Google ID token 驗證（VendorPortalInviteeService.VerifySession）。
//   - GOOGLE_WEB_CLIENT_ID、GOOGLE_WEB_CLIENT_SECRET：web 登入（/api/auth/google/*）的 Google OAuth client（authorization code ＋ PKCE），與
//     GOOGLE_CLIENT_IDS（vendor portal step-up 驗 ID token 的 aud）無關；兩個都設才啟用，只設一個是設定錯誤。LINE_CHANNEL_ID、
//     LINE_CHANNEL_SECRET：web 登入（/api/auth/line/*）的 LINE Login channel，同樣整組選填；LINE 的 web ID token 以 channel secret 驗（HS256）。
//     redirect URI 是 PUBLIC_WEB_ORIGIN 加 /api/auth/{google,line}/callback，要登記在各家的 console。
func readIdentityConfig(ctx context.Context, e *config.Env) identityConfig {
	var c identityConfig
	if accessConfigured(e) {
		admin, err := identity.NewAccess(identity.AccessConfig{TeamDomain: e.String("CF_ACCESS_TEAM_DOMAIN"), Audience: e.String("CF_ACCESS_ADMIN_AUD"), Context: ctx})
		if err != nil {
			e.Fail("CF_ACCESS_TEAM_DOMAIN／CF_ACCESS_ADMIN_AUD：%v", err)
		} else {
			c.providers.Admin = admin
		}
	}
	if path := e.StringOr("DEV_AUTH_KEY_FILE", ""); path != "" {
		if accessConfigured(e) {
			e.Fail("DEV_AUTH_KEY_FILE 不能與 Cloudflare Access 設定同時使用")
		} else {
			key, err := readDevKey(path)
			if err != nil {
				e.Fail("DEV_AUTH_KEY_FILE：%v", err)
			} else if dev, err := identity.NewDevAuth(key); err != nil {
				e.Fail("DEV_AUTH_KEY_FILE：%v", err)
			} else {
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
	c.googleAppClients = e.List("GOOGLE_APP_CLIENT_IDS")
	return c
}

// accessConfigured 是這個環境設定了 Cloudflare Access（正式的後台登入）。DEV_AUTH_KEY_FILE 不能與它同時使用。
func accessConfigured(e *config.Env) bool {
	return anySet(e, "CF_ACCESS_TEAM_DOMAIN", "CF_ACCESS_ADMIN_AUD")
}

// requireDevEnvironment 是只給本機開發的子命令（dev-seed）的保護，前提與 dev-token、serve 接受 dev token 相同：DEV_AUTH_KEY_FILE 指向
// 本機的 dev 金鑰（make dev-key 產生，正式環境沒有），而且沒有 Cloudflare Access 設定。
func requireDevEnvironment(e *config.Env) error {
	path := e.StringOr("DEV_AUTH_KEY_FILE", "")
	if path == "" {
		return errors.New("這個子命令只給本機開發使用：沒有設定 DEV_AUTH_KEY_FILE（make dev-key 產生）")
	}
	if accessConfigured(e) {
		return errors.New("這個子命令只給本機開發使用：DEV_AUTH_KEY_FILE 不能與 Cloudflare Access 設定同時使用")
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
func newIdentity(d *db.DB, bus *events.Bus, c identityConfig, log *slog.Logger) *identity.Module {
	return identity.New(identity.Deps{
		DB: d, Events: bus, Providers: c.providers,
		Now: time.Now, Logger: log,
	})
}

// runDevToken 印出本機開發與特性測試的登入 token，不連任何外部 provider：
//
//   - 顧客：在資料庫為既有的帳號建一個 dev session（App 種類）並印出原文 token；dev 使用者以帳號 ID 識別，不建任何登入身分。
//     帳號不存在時回錯誤（非 0 結束），不帶 -account 時建立一個新帳號（同 serve 一樣建立買家），ID 印到 stderr、token 印到 stdout。
//     需要 DATABASE_URL，而且 requireDevEnvironment 成立（DEV_AUTH_KEY_FILE 指向本機的 dev 金鑰，沒有正式的 Access 設定）。
//     session 的效期同正式 session。
//
//   - 後台（-admin）：以 DEV_AUTH_KEY_FILE 的金鑰簽 24 小時的 JWT，serve 設了同一個 DEV_AUTH_KEY_FILE 才接受它，管理員在第一個請求時建立。
//
//     hawkr dev-token [-account <帳號 ID>]
//     hawkr dev-token -admin [-user dev_<名稱>]
func runDevToken(ctx context.Context, args []string, out, errOut io.Writer) error {
	opts, err := parseDevTokenArgs(args)
	if err != nil {
		return err
	}
	e := config.NewEnv()
	if err := requireDevEnvironment(e); err != nil {
		return err
	}
	if opts.admin {
		key, err := readDevKey(e.StringOr("DEV_AUTH_KEY_FILE", ""))
		if err != nil {
			return fmt.Errorf("DEV_AUTH_KEY_FILE：%w", err)
		}
		dev, err := identity.NewDevAuth(key)
		if err != nil {
			return err
		}
		tok, err := dev.AdminToken(opts.user)
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
	return issueCustomerToken(ctx, identity.New(identity.Deps{DB: d, Events: bus, Now: time.Now, Logger: log}), opts.account, out, errOut)
}

type devTokenOptions struct {
	admin   bool
	user    string // 後台的 dev 使用者，以 dev_ 開頭
	account int64  // 0 是建立新帳號
}

func parseDevTokenArgs(args []string) (devTokenOptions, error) {
	fs := flag.NewFlagSet("dev-token", flag.ContinueOnError)
	admin := fs.Bool("admin", false, "簽後台的 token（否則是顧客的）")
	user := fs.String("user", "dev_buyer", "後台使用者的 ID，以 dev_ 開頭；只用於 -admin")
	account := fs.Int64("account", 0, "顧客：為這個帳號 ID 建 dev session；不帶時建立新帳號")
	if err := fs.Parse(args); err != nil {
		return devTokenOptions{}, err
	}
	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })
	switch {
	case *admin && set["account"]:
		return devTokenOptions{}, errors.New("-account 只用於顧客，不能與 -admin 一起用")
	case !*admin && set["user"]:
		return devTokenOptions{}, errors.New("顧客以帳號 ID 識別：用 -account <帳號 ID>；-user 只用於 -admin")
	case set["account"] && *account <= 0:
		return devTokenOptions{}, fmt.Errorf("-account 要是正的帳號 ID，收到 %d", *account)
	}
	return devTokenOptions{admin: *admin, user: *user, account: *account}, nil
}

// devSessions 是 issueCustomerToken 用到的 identity 能力。
type devSessions interface {
	DevSession(ctx context.Context, accountID int64) (string, error)
	DevNewAccount(ctx context.Context) (int64, error)
}

// issueCustomerToken 為 accountID 的帳號建 dev session 並把 token 印到 out；accountID 是 0 時先建立新帳號並把 ID 印到 errOut。
// 帳號不存在是錯誤，不會默默建立。
func issueCustomerToken(ctx context.Context, m devSessions, accountID int64, out, errOut io.Writer) error {
	if accountID == 0 {
		id, err := m.DevNewAccount(ctx)
		if err != nil {
			return err
		}
		accountID = id
		if _, err := fmt.Fprintf(errOut, "帳號 ID：%d\n", accountID); err != nil {
			return err
		}
	}
	tok, err := m.DevSession(ctx, accountID)
	if err != nil {
		return fmt.Errorf("帳號 %d：%w", accountID, err)
	}
	_, err = fmt.Fprintln(out, tok)
	return err
}

// newWebLogin 建立 web 聯合登入的路由（ADR 2026-10-30-identity-self-hosted-federated-login D3）。redirect URI 是 web 的網域加各家獨立的 callback 路徑。
// session cookie 的 Max-Age 是 identity.WebSessionCookieMaxAge（等於絕對逾時，ADR D1）；閒置逾時由伺服器判斷。
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
	return httpidentity.NewWebLogin(httpidentity.WebLoginDeps{Module: id, Providers: providers, SessionMaxAge: identity.WebSessionCookieMaxAge, Logger: log})
}

// newAppLogin 建立 App 原生登入的 ID token 驗證（ADR 2026-10-30-identity-self-hosted-federated-login D2）：web 登入已設定的 provider 才有，
// Google 另需 azp 允許清單。
func newAppLogin(ctx context.Context, id *identity.Module, c identityConfig) (*httpidentity.AppLogin, error) {
	var verifiers []httpidentity.AppVerifier
	for _, p := range identity.LoginProviders() {
		client, ok := c.webLogin[p]
		if !ok || p == identity.ProviderGoogle && len(c.googleAppClients) == 0 {
			continue
		}
		v, err := identity.NewAppLogin(identity.AppLoginConfig{Provider: p, ClientID: client.id, AuthorizedParties: appParties(p, c), Context: ctx})
		if err != nil {
			return nil, err
		}
		verifiers = append(verifiers, v)
	}
	return httpidentity.NewAppLogin(id, verifiers...)
}

// appParties 是 provider 的 App ID token 的 azp 允許清單：只有 Google 有。
func appParties(p identity.Provider, c identityConfig) []string {
	if p == identity.ProviderGoogle {
		return c.googleAppClients
	}
	return nil
}
