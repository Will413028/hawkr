//go:build integration

package identity_test

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"google.golang.org/genproto/googleapis/rpc/errdetails"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/identity/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/identity/v1alpha1/identityv1alpha1connect"
	httpidentity "github.com/Will413028/hawkr/backend/internal/httpapi/identity"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
)

// App 的 ID token：Google 的原生 SDK 簽 RS256（azp 是 Android／iOS client），LINE 原生 SDK 簽 ES256（JWKS）。
func (f *fakeIdP) appToken(nonce string, extra map[string]any) string {
	f.t.Helper()
	now := time.Now()
	claims := map[string]any{
		"iss": f.issuer(), "aud": f.clientID(), "sub": string(f.provider) + "-user-1", "nonce": nonce,
		"iat": now.Add(-time.Minute).Unix(), "exp": now.Add(time.Hour).Unix(), "name": "小明",
	}
	if f.provider == identity.ProviderGoogle {
		claims["azp"], claims["email"], claims["email_verified"] = googleAppClient, "ming@gmail.com", true
	}
	for k, v := range extra {
		if v == nil {
			delete(claims, k)
		} else {
			claims[k] = v
		}
	}
	var key jose.SigningKey
	opts := (&jose.SignerOptions{}).WithType("JWT")
	if f.provider == identity.ProviderLine {
		key = jose.SigningKey{Algorithm: jose.ES256, Key: f.ecKey}
		opts = opts.WithHeader("kid", "e1")
	} else {
		key = jose.SigningKey{Algorithm: jose.RS256, Key: f.key}
		opts = opts.WithHeader("kid", "k1")
	}
	signer, err := jose.NewSigner(key, opts)
	if err != nil {
		f.t.Fatal(err)
	}
	raw, err := jwt.Signed(signer).Claims(claims).Serialize()
	if err != nil {
		f.t.Fatal(err)
	}
	return raw
}

func protoProvider(p identity.Provider) pb.LoginProvider {
	return rpc.EnumToProto[pb.LoginProvider](p)
}

// ipHeader 指定來源 IP：各測試用自己的 IP，不互相佔用限流。
func ipHeader(ip string) connect.ClientOption { return headersOf("X-Test-Client-IP", ip) }

func (f *webFixture) public(opts ...connect.ClientOption) identityv1alpha1connect.IdentityPublicServiceClient {
	return identityv1alpha1connect.NewIdentityPublicServiceClient(http.DefaultClient, f.url, append([]connect.ClientOption{connect.WithProtoJSON()}, opts...)...)
}

func (f *webFixture) app(opts ...connect.ClientOption) identityv1alpha1connect.IdentityAppServiceClient {
	return identityv1alpha1connect.NewIdentityAppServiceClient(http.DefaultClient, f.url, append([]connect.ClientOption{connect.WithProtoJSON()}, opts...)...)
}

func bearer(token string) connect.ClientOption { return headersOf("Authorization", "Bearer "+token) }

// cookieOpt 是瀏覽器以 session cookie 呼叫的同源請求（D6）。
func cookieOpt(token string) connect.ClientOption {
	return headersOf("Cookie", rpc.SessionCookie+"="+token, "Sec-Fetch-Site", "same-origin")
}

func (f *webFixture) issueNonce(t *testing.T, ip string) string {
	t.Helper()
	resp, err := f.public(ipHeader(ip)).IssueAppLoginNonce(t.Context(), connect.NewRequest(&pb.IssueAppLoginNonceRequest{}))
	if err != nil {
		t.Fatalf("IssueAppLoginNonce = %v", err)
	}
	if resp.Msg.GetNonce() == "" || !resp.Msg.GetExpireTime().AsTime().Equal(f.clock.Now().Add(5*time.Minute)) {
		t.Fatalf("nonce 回應 = %v", resp.Msg)
	}
	return resp.Msg.GetNonce()
}

// exchange 發 nonce、讓 provider 簽一個帶它的 ID token（extra 覆寫 claims，nil 表示刪掉），再交換。
func (f *webFixture) exchange(t *testing.T, ip string, p identity.Provider, extra map[string]any, opts ...connect.ClientOption) (*pb.ExchangeIdTokenResponse, error) {
	t.Helper()
	nonce := f.issueNonce(t, ip)
	return f.exchangeToken(t, ip, p, f.idp[p].appToken(nonce, extra), nonce, opts...)
}

func (f *webFixture) exchangeToken(t *testing.T, ip string, p identity.Provider, idToken, nonce string, opts ...connect.ClientOption) (*pb.ExchangeIdTokenResponse, error) {
	t.Helper()
	resp, err := f.public(append([]connect.ClientOption{ipHeader(ip)}, opts...)...).ExchangeIdToken(t.Context(),
		connect.NewRequest(&pb.ExchangeIdTokenRequest{Provider: protoProvider(p), IdToken: idToken, Nonce: nonce}))
	if err != nil {
		return nil, err
	}
	return resp.Msg, nil
}

func (f *webFixture) mustExchange(t *testing.T, ip string, p identity.Provider, opts ...connect.ClientOption) string {
	t.Helper()
	resp, err := f.exchange(t, ip, p, nil, opts...)
	if err != nil {
		t.Fatalf("ExchangeIdToken = %v", err)
	}
	return resp.GetSessionToken()
}

func reasonOf(err error) string {
	var ce *connect.Error
	if !errors.As(err, &ce) {
		return ""
	}
	for _, d := range ce.Details() {
		v, err := d.Value()
		if err != nil {
			continue
		}
		if info, ok := v.(*errdetails.ErrorInfo); ok {
			return info.GetReason()
		}
	}
	return ""
}

func (f *webFixture) sessionAccountID(t *testing.T, token string) string {
	t.Helper()
	s, err := f.getSession(t, bearer(token))
	if err != nil {
		t.Fatalf("GetSession = %v", err)
	}
	return s.Msg.GetAccount().GetId()
}

func (f *webFixture) nonceRows(t *testing.T) int {
	t.Helper()
	return f.count(t, "SELECT count(*) FROM identity.app_login_nonces")
}

// 兩家各一條：nonce → 原生 ID token → session（App 種類）；token 可以 GetSession，nonce 用掉了。
func TestAppExchangeSignsInWithEachProvider(t *testing.T) {
	t.Parallel()
	for i, p := range []identity.Provider{identity.ProviderGoogle, identity.ProviderLine} {
		t.Run(string(p), func(t *testing.T) {
			t.Parallel()
			f := setupWeb(t)
			ip := fmt.Sprintf("198.51.100.%d", 10+i)
			resp, err := f.exchange(t, ip, p, nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(resp.GetSessionToken()) < 40 || !resp.GetAbsoluteExpireTime().AsTime().Equal(f.clock.Now().Add(90*24*time.Hour)) || resp.GetIdleTimeout().AsDuration() != 14*24*time.Hour {
				t.Fatalf("回應 = %v", resp)
			}
			if f.sessionAccountID(t, resp.GetSessionToken()) == "" {
				t.Fatal("GetSession 沒有帳號")
			}
			if n := f.count(t, `SELECT count(*) FROM identity.sessions WHERE client = 'APP'`); n != 1 {
				t.Fatalf("APP session = %d", n)
			}
			if n := f.count(t, `SELECT count(*) FROM identity.account_identities WHERE provider = '`+string(p)+`'`); n != 1 {
				t.Fatalf("身分列 = %d", n)
			}
			if n := f.nonceRows(t); n != 0 {
				t.Fatalf("用掉的 nonce 還在：%d", n)
			}
		})
	}
}

// 每一種不符都被拒絕、不留帳號、身分或 session；ID token 無效時 nonce 不被消耗（別人不能用垃圾請求作廢它）。
func TestAppExchangeRefusesBadIDTokens(t *testing.T) {
	t.Parallel()
	type tc struct {
		p     identity.Provider
		extra map[string]any
		want  connect.Code
	}
	cases := map[string]tc{
		"Google：token 的 nonce 不是請求的": {identity.ProviderGoogle, map[string]any{"nonce": "another-nonce"}, connect.CodeUnauthenticated},
		"Google：token 沒有 nonce":      {identity.ProviderGoogle, map[string]any{"nonce": nil}, connect.CodeUnauthenticated},
		"Google：azp 不在允許清單":          {identity.ProviderGoogle, map[string]any{"azp": "evil-client"}, connect.CodeUnauthenticated},
		"Google：沒有 azp":              {identity.ProviderGoogle, map[string]any{"azp": nil}, connect.CodeUnauthenticated},
		"Google：aud 不是 web client":   {identity.ProviderGoogle, map[string]any{"aud": "other-client"}, connect.CodeUnauthenticated},
		"Google：iss 不對":              {identity.ProviderGoogle, map[string]any{"iss": "https://evil.example"}, connect.CodeUnauthenticated},
		"Google：過期":                  {identity.ProviderGoogle, map[string]any{"exp": time.Now().Add(-time.Hour).Unix()}, connect.CodeUnauthenticated},
		"LINE：token 的 nonce 不是請求的":   {identity.ProviderLine, map[string]any{"nonce": "another-nonce"}, connect.CodeUnauthenticated},
		"LINE：aud 不是 channel":        {identity.ProviderLine, map[string]any{"aud": "other-channel"}, connect.CodeUnauthenticated},
		"LINE：iss 不對":                {identity.ProviderLine, map[string]any{"iss": "https://evil.example"}, connect.CodeUnauthenticated},
		"LINE：過期":                    {identity.ProviderLine, map[string]any{"exp": time.Now().Add(-time.Hour).Unix()}, connect.CodeUnauthenticated},
	}
	i := 0
	for name, c := range cases {
		i++
		ip := fmt.Sprintf("198.51.100.%d", 20+i)
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := setupWeb(t)
			nonce := f.issueNonce(t, ip)
			extra := map[string]any{}
			for k, v := range c.extra {
				extra[k] = v
			}
			if _, overridden := extra["nonce"]; !overridden {
				extra["nonce"] = nonce
			}
			_, err := f.exchangeToken(t, ip, c.p, f.idp[c.p].appToken(nonce, extra), nonce)
			wantCode(t, err, c.want)
			f.nothingCreated(t)
			if n := f.nonceRows(t); n != 1 {
				t.Fatalf("nonce 列 = %d，無效的 ID token 不能消耗 nonce", n)
			}
		})
	}
	t.Run("LINE：用 web 的 HS256 簽", func(t *testing.T) {
		t.Parallel()
		f := setupWeb(t)
		nonce := f.issueNonce(t, "198.51.100.60")
		line := f.idp[identity.ProviderLine]
		hs := line.sign(map[string]any{
			"iss": line.issuer(), "aud": line.clientID(), "sub": "LINE-user-1", "nonce": nonce,
			"iat": time.Now().Add(-time.Minute).Unix(), "exp": time.Now().Add(time.Hour).Unix(),
		})
		_, err := f.exchangeToken(t, "198.51.100.60", identity.ProviderLine, hs, nonce)
		wantCode(t, err, connect.CodeUnauthenticated)
		f.nothingCreated(t)
	})
	t.Run("nonce 不是伺服器發的", func(t *testing.T) {
		t.Parallel()
		f := setupWeb(t)
		forged := "forged-nonce-0123456789012345678901234567890123"
		_, err := f.exchangeToken(t, "198.51.100.61", identity.ProviderGoogle, f.idp[identity.ProviderGoogle].appToken(forged, nil), forged)
		wantCode(t, err, connect.CodeUnauthenticated)
		f.nothingCreated(t)
	})
	t.Run("沒有 provider", func(t *testing.T) {
		t.Parallel()
		f := setupWeb(t)
		_, err := f.public(ipHeader("198.51.100.62")).ExchangeIdToken(t.Context(), connect.NewRequest(&pb.ExchangeIdTokenRequest{IdToken: "x", Nonce: "y"}))
		wantCode(t, err, connect.CodeInvalidArgument)
	})
}

// nonce 單次使用：同一個 nonce 簽出的第二個（有效的）ID token 被拒絕。
func TestAppExchangeNonceIsSingleUse(t *testing.T) {
	t.Parallel()
	f := setupWeb(t)
	ip := "198.51.100.70"
	nonce := f.issueNonce(t, ip)
	idp := f.idp[identity.ProviderGoogle]
	if _, err := f.exchangeToken(t, ip, identity.ProviderGoogle, idp.appToken(nonce, nil), nonce); err != nil {
		t.Fatal(err)
	}
	_, err := f.exchangeToken(t, ip, identity.ProviderGoogle, idp.appToken(nonce, nil), nonce)
	wantCode(t, err, connect.CodeUnauthenticated)
	if n := f.count(t, "SELECT count(*) FROM identity.sessions"); n != 1 {
		t.Fatalf("sessions = %d，重放不能再發 session", n)
	}
}

// 過期的 nonce 被拒絕（注入時鐘）。
func TestAppExchangeNonceExpiresAfterFiveMinutes(t *testing.T) {
	t.Parallel()
	f := setupWeb(t)
	ip := "198.51.100.71"
	nonce := f.issueNonce(t, ip)
	f.clock.Advance(5*time.Minute + time.Second)
	_, err := f.exchangeToken(t, ip, identity.ProviderGoogle, f.idp[identity.ProviderGoogle].appToken(nonce, nil), nonce)
	wantCode(t, err, connect.CodeUnauthenticated)
	f.nothingCreated(t)
}

// 已刪帳號的身分（墓碑）從 App 登入也回 unauthenticated，不建新帳號、不發 session。
func TestAppExchangeRefusesADeletedAccountsIdentity(t *testing.T) {
	t.Parallel()
	f := setupWeb(t)
	ip := "198.51.100.72"
	token := f.mustExchange(t, ip, identity.ProviderLine)
	id, err := f.m.Authenticate(t.Context(), token)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.m.DeleteAccount(t.Context(), id.Principal.ID); err != nil {
		t.Fatal(err)
	}
	_, err = f.exchange(t, ip, identity.ProviderLine, nil)
	wantCode(t, err, connect.CodeUnauthenticated)
	if a, s := f.count(t, "SELECT count(*) FROM identity.accounts"), f.count(t, "SELECT count(*) FROM identity.sessions"); a != 1 || s != 0 {
		t.Fatalf("accounts = %d、sessions = %d，預期墓碑 1、session 0", a, s)
	}
}

// 登入換發新 session：請求帶 Bearer 的舊 session 在同一個登入刪除；cookie 帶的不算。
func TestAppExchangeReplacesTheBearerSession(t *testing.T) {
	t.Parallel()
	f := setupWeb(t)
	ip := "198.51.100.73"
	old := f.mustExchange(t, ip, identity.ProviderGoogle)
	fresh := f.mustExchange(t, ip, identity.ProviderGoogle, bearer(old))
	if fresh == old {
		t.Fatal("沒有換發 token")
	}
	_, err := f.getSession(t, bearer(old))
	wantCode(t, err, connect.CodeUnauthenticated)
	f.sessionAccountID(t, fresh)
	if n := f.count(t, "SELECT count(*) FROM identity.sessions"); n != 1 {
		t.Fatalf("sessions = %d", n)
	}
}

func TestAppLoginProceduresAreRateLimitedPerIP(t *testing.T) {
	t.Parallel()
	f := setupWeb(t)
	var limited error
	for i := 0; i < 40 && limited == nil; i++ {
		_, limited = f.public(ipHeader("198.51.100.80")).IssueAppLoginNonce(t.Context(), connect.NewRequest(&pb.IssueAppLoginNonceRequest{}))
	}
	wantCode(t, limited, connect.CodeResourceExhausted)
	// ExchangeIdToken 與 IssueAppLoginNonce 共用同一個額度；另一個 IP 不受影響。
	_, err := f.exchangeToken(t, "198.51.100.80", identity.ProviderGoogle, "x", "y")
	wantCode(t, err, connect.CodeResourceExhausted)
	f.issueNonce(t, "198.51.100.81")
}

// SignOut 刪目前的 session，之後同一個 token 是 unauthenticated；cookie 的回應清掉 cookie，Bearer 的不用；別的 session 不受影響。
func TestSignOutRevokesTheCurrentSession(t *testing.T) {
	t.Parallel()
	f := setupWeb(t)
	b := f.browser(t, "198.51.100.90")
	_, cb := b.login(identity.ProviderGoogle, loginOpts{})
	web := setCookie(cb, rpc.SessionCookie).Value
	app := f.mustExchange(t, "198.51.100.91", identity.ProviderGoogle)

	resp, err := f.app(cookieOpt(web)).SignOut(t.Context(), connect.NewRequest(&pb.SignOutRequest{}))
	if err != nil {
		t.Fatalf("cookie 的 SignOut = %v", err)
	}
	var cleared *http.Cookie
	for _, c := range (&http.Response{Header: resp.Header()}).Cookies() {
		if c.Name == rpc.SessionCookie {
			cleared = c
		}
	}
	if cleared == nil || cleared.MaxAge >= 0 || cleared.Value != "" || !cleared.Secure || !cleared.HttpOnly || cleared.Path != "/" {
		t.Fatalf("清除用的 cookie = %+v", cleared)
	}
	_, err = f.getSession(t, bearer(web))
	wantCode(t, err, connect.CodeUnauthenticated)
	f.sessionAccountID(t, app)

	resp, err = f.app(bearer(app)).SignOut(t.Context(), connect.NewRequest(&pb.SignOutRequest{}))
	if err != nil {
		t.Fatalf("Bearer 的 SignOut = %v", err)
	}
	if resp.Header().Get("Set-Cookie") != "" {
		t.Errorf("Bearer 的 SignOut 不該動 cookie：%v", resp.Header()["Set-Cookie"])
	}
	_, err = f.getSession(t, bearer(app))
	wantCode(t, err, connect.CodeUnauthenticated)
	if n := f.count(t, "SELECT count(*) FROM identity.sessions"); n != 0 {
		t.Fatalf("sessions = %d", n)
	}
	// 沒有登入時 SignOut 是 unauthenticated。
	_, err = f.app().SignOut(t.Context(), connect.NewRequest(&pb.SignOutRequest{}))
	wantCode(t, err, connect.CodeUnauthenticated)
}

// 登出用 GET 被拒（cookie 與 Bearer），session 還在。
func TestSignOutByGETIsRefused(t *testing.T) {
	t.Parallel()
	f := setupWeb(t)
	b := f.browser(t, "198.51.100.92")
	_, cb := b.login(identity.ProviderGoogle, loginOpts{})
	web := setCookie(cb, rpc.SessionCookie).Value
	app := f.mustExchange(t, "198.51.100.93", identity.ProviderGoogle)
	for name, header := range map[string][2]string{
		"cookie": {"Cookie", rpc.SessionCookie + "=" + web},
		"Bearer": {"Authorization", "Bearer " + app},
	} {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, f.url+identityv1alpha1connect.IdentityAppServiceSignOutProcedure+"?connect=v1&encoding=json&message=%7B%7D", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set(header[0], header[1])
		req.Header.Set("Sec-Fetch-Site", "same-origin")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("%s：GET SignOut = %d，預期 405", name, resp.StatusCode)
		}
	}
	if n := f.count(t, "SELECT count(*) FROM identity.sessions"); n != 2 {
		t.Fatalf("sessions = %d，GET 不能登出", n)
	}
}

// 停權的帳號仍能登入、SignOut、ListSessions、RevokeSession、LinkIdentity（自助／讀取，A7-D18）；寫入回 ACCOUNT_SUSPENDED。
func TestSuspendedAccountsCanStillManageTheirSessions(t *testing.T) {
	t.Parallel()
	f := setupWeb(t)
	ip := "198.51.100.94"
	first := f.mustExchange(t, ip, identity.ProviderGoogle)
	a, err := f.m.Authenticate(t.Context(), first)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Pool().Exec(t.Context(), `UPDATE identity.accounts SET suspended_at = $2 WHERE id = $1`, a.Principal.ID, f.clock.Now()); err != nil {
		t.Fatal(err)
	}
	second := f.mustExchange(t, ip, identity.ProviderGoogle, bearer(first)) // 停權的帳號照樣登入，登入換發 session
	other := f.mustExchange(t, ip, identity.ProviderGoogle)                 // 另一個 session
	c := f.app(bearer(second))
	list, err := c.ListSessions(t.Context(), connect.NewRequest(&pb.ListSessionsRequest{}))
	if err != nil || len(list.Msg.GetSessions()) != 2 {
		t.Fatalf("ListSessions = %v, %v", list, err)
	}
	var target string
	for _, s := range list.Msg.GetSessions() {
		if !s.GetCurrent() {
			target = s.GetId()
		}
	}
	nonce := f.issueNonce(t, ip)
	if _, err := c.LinkIdentity(t.Context(), connect.NewRequest(&pb.LinkIdentityRequest{
		Provider: pb.LoginProvider_LOGIN_PROVIDER_LINE, IdToken: f.idp[identity.ProviderLine].appToken(nonce, nil), Nonce: nonce,
	})); err != nil {
		t.Fatalf("停權者 LinkIdentity = %v", err)
	}
	if _, err := c.RevokeSession(t.Context(), connect.NewRequest(&pb.RevokeSessionRequest{SessionId: target})); err != nil {
		t.Fatalf("停權者 RevokeSession = %v", err)
	}
	_, err = f.getSession(t, bearer(other))
	wantCode(t, err, connect.CodeUnauthenticated)
	_, err = c.LinkAppleCredential(t.Context(), connect.NewRequest(&pb.LinkAppleCredentialRequest{IdToken: "x", AuthorizationCode: "y"}))
	wantCode(t, err, connect.CodePermissionDenied)
	if got := reasonOf(err); got != "ACCOUNT_SUSPENDED" {
		t.Fatalf("寫入的 reason = %q", got)
	}
	if _, err := c.SignOut(t.Context(), connect.NewRequest(&pb.SignOutRequest{})); err != nil {
		t.Fatalf("停權者 SignOut = %v", err)
	}
	_, err = f.getSession(t, bearer(second))
	wantCode(t, err, connect.CodeUnauthenticated)
}

func (f *webFixture) linkOverApp(t *testing.T, client identityv1alpha1connect.IdentityAppServiceClient, ip string, p identity.Provider) error {
	t.Helper()
	nonce := f.issueNonce(t, ip)
	_, err := client.LinkIdentity(t.Context(), connect.NewRequest(&pb.LinkIdentityRequest{Provider: protoProvider(p), IdToken: f.idp[p].appToken(nonce, nil), Nonce: nonce}))
	return err
}

// App 連結：成功之後用那個身分登入就是同一個帳號；已屬於別的帳號（含墓碑）回 AlreadyExists 與 IDENTITY_ALREADY_LINKED，什麼都不改。
func TestLinkIdentityOverApp(t *testing.T) {
	t.Parallel()
	f := setupWeb(t)
	ip := "198.51.100.100"
	google := f.mustExchange(t, ip, identity.ProviderGoogle)
	owner := f.sessionAccountID(t, google)
	client := f.app(bearer(google))
	if err := f.linkOverApp(t, client, ip, identity.ProviderLine); err != nil {
		t.Fatalf("LinkIdentity = %v", err)
	}
	line := f.mustExchange(t, ip, identity.ProviderLine)
	if got := f.sessionAccountID(t, line); got != owner {
		t.Fatalf("連結的 LINE 登入到帳號 %s，預期 %s", got, owner)
	}
	// 重複連結自己的身分：成功。
	if err := f.linkOverApp(t, client, ip, identity.ProviderLine); err != nil {
		t.Fatalf("重複連結 = %v", err)
	}
	if n := f.count(t, `SELECT count(*) FROM identity.account_identities`); n != 2 {
		t.Fatalf("身分列 = %d", n)
	}
}

func TestLinkIdentityConflictsOverApp(t *testing.T) {
	t.Parallel()
	for name, tombstone := range map[string]bool{"身分屬於別的帳號": false, "身分屬於已刪除的帳號": true} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := setupWeb(t)
			ip := "198.51.100.101"
			bob := f.mustExchange(t, ip, identity.ProviderLine)
			bobAccount, err := f.m.Authenticate(t.Context(), bob)
			if err != nil {
				t.Fatal(err)
			}
			if tombstone {
				if err := f.m.DeleteAccount(t.Context(), bobAccount.Principal.ID); err != nil {
					t.Fatal(err)
				}
			}
			// 另一個帳號（Google）想連結 LINE-user-1。
			alice := f.mustExchange(t, ip, identity.ProviderGoogle)
			err = f.linkOverApp(t, f.app(bearer(alice)), ip, identity.ProviderLine)
			wantCode(t, err, connect.CodeAlreadyExists)
			if got := reasonOf(err); got != "IDENTITY_ALREADY_LINKED" {
				t.Fatalf("reason = %q", got)
			}
			var owner int64
			if err := f.db.Pool().QueryRow(t.Context(), `SELECT account_id FROM identity.account_identities WHERE provider = 'LINE'`).Scan(&owner); err != nil || owner != bobAccount.Principal.ID {
				t.Fatalf("LINE 身分的擁有者 = %d, %v，預期仍是 %d", owner, err, bobAccount.Principal.ID)
			}
		})
	}
}

func TestLinkIdentityOverAppNeedsASignedInAccountAndAValidToken(t *testing.T) {
	t.Parallel()
	f := setupWeb(t)
	ip := "198.51.100.102"
	err := f.linkOverApp(t, f.app(), ip, identity.ProviderLine)
	wantCode(t, err, connect.CodeUnauthenticated)
	google := f.mustExchange(t, ip, identity.ProviderGoogle)
	nonce := f.issueNonce(t, ip)
	_, err = f.app(bearer(google)).LinkIdentity(t.Context(), connect.NewRequest(&pb.LinkIdentityRequest{
		Provider: pb.LoginProvider_LOGIN_PROVIDER_LINE, IdToken: f.idp[identity.ProviderLine].appToken("another-nonce", nil), Nonce: nonce,
	}))
	wantCode(t, err, connect.CodeUnauthenticated)
	if n := f.count(t, `SELECT count(*) FROM identity.account_identities WHERE provider = 'LINE'`); n != 0 {
		t.Fatalf("LINE 身分 = %d，無效的 token 不能連結", n)
	}
}

// web 連結：start?link=1 把已登入的帳號寫進暫存列，callback 連結它，不換發 session。
func TestWebLinkAddsTheIdentityToTheSignedInAccount(t *testing.T) {
	t.Parallel()
	f := setupWeb(t)
	b := f.browser(t, "198.51.100.110")
	_, cb := b.login(identity.ProviderGoogle, loginOpts{})
	session := setCookie(cb, rpc.SessionCookie).Value
	owner := f.sessionAccountID(t, session)

	start := b.get("/api/auth/line/start?link=1&return_to="+url.QueryEscape("/account/settings"), "Sec-Fetch-Site", "none")
	if start.StatusCode != http.StatusFound || setCookie(start, httpidentity.LoginCookie) == nil {
		t.Fatalf("start = %d", start.StatusCode)
	}
	loc := start.Header.Get("Location")
	code := f.idp[identity.ProviderLine].authorize(loc, nil)
	u, _ := url.Parse(loc)
	done := b.get("/api/auth/line/callback?" + url.Values{"state": {u.Query().Get("state")}, "code": {code}}.Encode())
	if got := done.Header.Get("Location"); done.StatusCode != http.StatusFound || got != "/account/settings" {
		t.Fatalf("callback = %d %s", done.StatusCode, got)
	}
	if setCookie(done, rpc.SessionCookie) != nil {
		t.Fatal("連結不能換發 session")
	}
	if got := f.sessionAccountID(t, session); got != owner {
		t.Fatalf("session 的帳號變了：%s → %s", owner, got)
	}
	line := f.mustExchange(t, "198.51.100.111", identity.ProviderLine)
	if got := f.sessionAccountID(t, line); got != owner {
		t.Fatalf("以連結的 LINE 登入到帳號 %s，預期 %s", got, owner)
	}
	if n := f.count(t, `SELECT count(*) FROM identity.accounts`); n != 1 {
		t.Fatalf("accounts = %d", n)
	}
}

func TestWebLinkConflictComesBackOnTheReturnPath(t *testing.T) {
	t.Parallel()
	f := setupWeb(t)
	bob := f.mustExchange(t, "198.51.100.120", identity.ProviderLine)
	bobAccount := f.sessionAccountID(t, bob)
	b := f.browser(t, "198.51.100.121")
	_, cb := b.login(identity.ProviderGoogle, loginOpts{})
	start := b.get("/api/auth/line/start?link=1&return_to=" + url.QueryEscape("/account/settings"))
	loc := start.Header.Get("Location")
	code := f.idp[identity.ProviderLine].authorize(loc, nil)
	u, _ := url.Parse(loc)
	done := b.get("/api/auth/line/callback?" + url.Values{"state": {u.Query().Get("state")}, "code": {code}}.Encode())
	back, err := url.Parse(done.Header.Get("Location"))
	if err != nil || done.StatusCode != http.StatusFound || back.Path != "/account/settings" || back.Query().Get(httpidentity.LinkErrorParam) != httpidentity.LinkErrorAlreadyLinked {
		t.Fatalf("callback = %d %s", done.StatusCode, done.Header.Get("Location"))
	}
	if got := f.sessionAccountID(t, setCookie(cb, rpc.SessionCookie).Value); got == bobAccount {
		t.Fatal("連結衝突不能把 session 換成別人的帳號")
	}
	var owner string
	if err := f.db.Pool().QueryRow(t.Context(), `SELECT account_id::text FROM identity.account_identities WHERE provider = 'LINE'`).Scan(&owner); err != nil || owner != bobAccount {
		t.Fatalf("LINE 身分的擁有者 = %s, %v，預期仍是 %s", owner, err, bobAccount)
	}
}

// 連結要有登入、callback 時的登入帳號要與發起的相同、登入前 cookie 與 state 照樣檢查。
func TestWebLinkRefusals(t *testing.T) {
	t.Parallel()
	t.Run("沒有登入", func(t *testing.T) {
		t.Parallel()
		f := setupWeb(t)
		b := f.browser(t, "198.51.100.130")
		resp := b.get("/api/auth/line/start?link=1")
		wantLoginError(t, resp, httpidentity.ErrorLoginRequired)
		if n := f.count(t, "SELECT count(*) FROM identity.login_flows"); n != 0 || setCookie(resp, httpidentity.LoginCookie) != nil {
			t.Fatalf("login_flows = %d，被拒絕的連結不能留下暫存列或登入前 cookie", n)
		}
	})
	t.Run("callback 時已換了帳號", func(t *testing.T) {
		t.Parallel()
		f := setupWeb(t)
		b := f.browser(t, "198.51.100.131")
		_, _ = b.login(identity.ProviderGoogle, loginOpts{})
		start := b.get("/api/auth/line/start?link=1&return_to=/account/settings")
		loc := start.Header.Get("Location")
		code := f.idp[identity.ProviderLine].authorize(loc, nil)
		u, _ := url.Parse(loc)
		// 瀏覽器在 provider 頁面時，session 換成另一個帳號（LINE 先建的帳號）。
		other := f.mustExchange(t, "198.51.100.132", identity.ProviderLine)
		b.cookies[rpc.SessionCookie] = other
		done := b.get("/api/auth/line/callback?" + url.Values{"state": {u.Query().Get("state")}, "code": {code}}.Encode())
		back, _ := url.Parse(done.Header.Get("Location"))
		if back.Query().Get(httpidentity.LinkErrorParam) != httpidentity.ErrorInvalidFlow {
			t.Fatalf("callback = %d %s", done.StatusCode, done.Header.Get("Location"))
		}
		if n := f.count(t, `SELECT count(*) FROM identity.account_identities WHERE provider = 'LINE'`); n != 1 {
			t.Fatalf("LINE 身分 = %d，不該多連一個", n)
		}
	})
	t.Run("沒有登入前 cookie（跨站導覽到 callback）", func(t *testing.T) {
		t.Parallel()
		f := setupWeb(t)
		b := f.browser(t, "198.51.100.133")
		_, _ = b.login(identity.ProviderGoogle, loginOpts{})
		start := b.get("/api/auth/line/start?link=1")
		loc := start.Header.Get("Location")
		code := f.idp[identity.ProviderLine].authorize(loc, nil)
		u, _ := url.Parse(loc)
		delete(b.cookies, httpidentity.LoginCookie)
		done := b.get("/api/auth/line/callback?" + url.Values{"state": {u.Query().Get("state")}, "code": {code}}.Encode())
		wantLoginError(t, done, httpidentity.ErrorInvalidFlow)
		if n := f.count(t, `SELECT count(*) FROM identity.account_identities WHERE provider = 'LINE'`); n != 0 {
			t.Fatalf("LINE 身分 = %d，沒有登入前 cookie 不能連結", n)
		}
	})
}

// ListSessions 標出目前的 session、只列有效的（閒置逾時的不列）、只列自己的。
func TestListSessionsMarksTheCurrentSession(t *testing.T) {
	t.Parallel()
	f := setupWeb(t)
	ip := "198.51.100.140"
	b := f.browser(t, ip)
	_, cb := b.login(identity.ProviderGoogle, loginOpts{})
	web := setCookie(cb, rpc.SessionCookie).Value
	f.clock.Advance(time.Hour)
	app := f.mustExchange(t, ip, identity.ProviderGoogle)
	_ = f.mustExchange(t, ip, identity.ProviderLine) // 別的帳號

	check := func(client identityv1alpha1connect.IdentityAppServiceClient, current pb.ClientKind, want int) {
		t.Helper()
		resp, err := client.ListSessions(t.Context(), connect.NewRequest(&pb.ListSessionsRequest{}))
		if err != nil {
			t.Fatal(err)
		}
		got := resp.Msg.GetSessions()
		if len(got) != want {
			t.Fatalf("sessions = %v，預期 %d 個", got, want)
		}
		n := 0
		for _, s := range got {
			if s.GetCurrent() {
				n++
				if s.GetClientKind() != current {
					t.Errorf("目前的 session 種類 = %v，預期 %v", s.GetClientKind(), current)
				}
			}
			if s.GetId() == "" || s.GetCreateTime() == nil || s.GetLastUseTime() == nil {
				t.Errorf("session 欄位不完整：%v", s)
			}
			if s.GetId() == web || s.GetId() == app {
				t.Error("id 不能是 token")
			}
		}
		if n != 1 {
			t.Errorf("標為目前的有 %d 個", n)
		}
	}
	check(f.app(bearer(app)), pb.ClientKind_CLIENT_KIND_APP, 2)
	check(f.app(cookieOpt(web)), pb.ClientKind_CLIENT_KIND_WEB, 2)

	// web 閒置超過 7 天就失效，不再列出；App 的 14 天還沒到。
	f.clock.Advance(7 * 24 * time.Hour)
	resp, err := f.app(bearer(app)).ListSessions(t.Context(), connect.NewRequest(&pb.ListSessionsRequest{}))
	if err != nil || len(resp.Msg.GetSessions()) != 1 || resp.Msg.GetSessions()[0].GetClientKind() != pb.ClientKind_CLIENT_KIND_APP {
		t.Fatalf("閒置逾時後 = %v, %v", resp, err)
	}
	// ListSessions 可以用 GET（NO_SIDE_EFFECTS），cookie 認證也行。
	if _, err := f.app(connect.WithHTTPGet(), bearer(app)).ListSessions(t.Context(), connect.NewRequest(&pb.ListSessionsRequest{})); err != nil {
		t.Fatalf("GET ListSessions = %v", err)
	}
}

func (f *webFixture) sessionIDs(t *testing.T, token string) (current string, others []string) {
	t.Helper()
	resp, err := f.app(bearer(token)).ListSessions(t.Context(), connect.NewRequest(&pb.ListSessionsRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range resp.Msg.GetSessions() {
		if s.GetCurrent() {
			current = s.GetId()
		} else {
			others = append(others, s.GetId())
		}
	}
	return current, others
}

// RevokeSession 要求目前的 session 在 15 分鐘內以 provider 驗證過；重新登入（換發新 session）之後放行；只能終止自己的。
func TestRevokeSessionNeedsRecentAuthentication(t *testing.T) {
	t.Parallel()
	f := setupWeb(t)
	ip := "198.51.100.150"
	b := f.browser(t, ip)
	_, cb := b.login(identity.ProviderGoogle, loginOpts{})
	web := setCookie(cb, rpc.SessionCookie).Value
	app := f.mustExchange(t, ip, identity.ProviderGoogle)
	_, others := f.sessionIDs(t, app)
	if len(others) != 1 {
		t.Fatalf("others = %v", others)
	}
	target := others[0]
	revoke := func(token, id string) error {
		_, err := f.app(bearer(token)).RevokeSession(t.Context(), connect.NewRequest(&pb.RevokeSessionRequest{SessionId: id}))
		return err
	}

	// 登入後 14 分鐘：放行的邊界內；先用不存在的 ID 確認通過重新驗證之後才查擁有者。
	f.clock.Advance(14 * time.Minute)
	wantCode(t, revoke(app, "01930000-0000-7000-8000-00000000dead"), connect.CodeNotFound)

	f.clock.Advance(2 * time.Minute) // 16 分鐘
	err := revoke(app, target)
	wantCode(t, err, connect.CodeFailedPrecondition)
	if got := reasonOf(err); got != "REAUTHENTICATION_REQUIRED" {
		t.Fatalf("reason = %q", got)
	}
	if n := f.count(t, "SELECT count(*) FROM identity.sessions"); n != 2 {
		t.Fatalf("sessions = %d，未重新驗證不能終止", n)
	}

	// 重新登入（換發新 session，auth_time 是現在）之後放行。
	fresh := f.mustExchange(t, ip, identity.ProviderGoogle, bearer(app))
	if err := revoke(fresh, target); err != nil {
		t.Fatalf("重新驗證後的 RevokeSession = %v", err)
	}
	_, err = f.getSession(t, bearer(web))
	wantCode(t, err, connect.CodeUnauthenticated)
	f.sessionAccountID(t, fresh)

	// 別人的 session 與不存在的 ID：NotFound，不洩漏存在與否。
	other := f.mustExchange(t, ip, identity.ProviderLine)
	otherCurrent, _ := f.sessionIDs(t, other)
	wantCode(t, revoke(fresh, otherCurrent), connect.CodeNotFound)
	f.sessionAccountID(t, other)
	wantCode(t, revoke(fresh, "not-a-uuid"), connect.CodeInvalidArgument)
}

// 重新驗證也可以走 web 的登入：cookie 的 session 被換發，auth_time 重新計算。
func TestRevokeSessionAfterWebReauthentication(t *testing.T) {
	t.Parallel()
	f := setupWeb(t)
	ip := "198.51.100.151"
	b := f.browser(t, ip)
	_, first := b.login(identity.ProviderGoogle, loginOpts{})
	f.mustExchange(t, ip, identity.ProviderGoogle) // 另一個 session（App）
	f.clock.Advance(20 * time.Minute)

	revokeOthers := func(token string) error {
		resp, err := f.app(cookieOpt(token)).ListSessions(t.Context(), connect.NewRequest(&pb.ListSessionsRequest{}))
		if err != nil {
			t.Fatal(err)
		}
		for _, s := range resp.Msg.GetSessions() {
			if s.GetCurrent() {
				continue
			}
			if _, err := f.app(cookieOpt(token)).RevokeSession(t.Context(), connect.NewRequest(&pb.RevokeSessionRequest{SessionId: s.GetId()})); err != nil {
				return err
			}
		}
		return nil
	}
	wantCode(t, revokeOthers(setCookie(first, rpc.SessionCookie).Value), connect.CodeFailedPrecondition)
	_, again := b.login(identity.ProviderGoogle, loginOpts{})
	if err := revokeOthers(setCookie(again, rpc.SessionCookie).Value); err != nil {
		t.Fatalf("web 重新登入後 RevokeSession = %v", err)
	}
	if n := f.count(t, "SELECT count(*) FROM identity.sessions"); n != 1 {
		t.Fatalf("sessions = %d，預期只剩重新登入的這個", n)
	}
}
