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

type identityConfig struct {
	providers        identity.Providers
	webLogin         map[identity.Provider]webClient
	googleAppClients []string
}

type webClient struct{ id, secret string }

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

func accessConfigured(e *config.Env) bool {
	return anySet(e, "CF_ACCESS_TEAM_DOMAIN", "CF_ACCESS_ADMIN_AUD")
}

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

func newIdentity(d *db.DB, bus *events.Bus, c identityConfig, log *slog.Logger) *identity.Module {
	return identity.New(identity.Deps{
		DB: d, Events: bus, Providers: c.providers,
		Now: time.Now, Logger: log,
	})
}

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
	user    string
	account int64
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

type devSessions interface {
	DevSession(ctx context.Context, accountID int64) (string, error)
	DevNewAccount(ctx context.Context) (int64, error)
}

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

func appParties(p identity.Provider, c identityConfig) []string {
	if p == identity.ProviderGoogle {
		return c.googleAppClients
	}
	return nil
}
