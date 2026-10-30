package identity

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
	"uuid"

	"connectrpc.com/connect"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/identity/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

var appNow = time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)

// appModule 照 identity 模組的契約實作 App 登入與 session 管理：nonce 單次使用，session 屬於帳號。
type appModule struct {
	nonces   map[string]bool
	consumed []string
	logins   []identity.LoginInput
	loginErr error
	links    []linkCall
	linkErr  error
	sessions []identity.SessionInfo
	revoked  []uuid.UUID
	listErr  error
	linked   map[int64][]identity.LinkedIdentity
}

func newAppModule() *appModule { return &appModule{nonces: map[string]bool{"nonce-1": true}} }

func (m *appModule) IssueAppNonce(context.Context) (string, time.Time, error) {
	return "nonce-1", appNow.Add(5 * time.Minute), nil
}

func (m *appModule) ConsumeAppNonce(_ context.Context, nonce string) error {
	m.consumed = append(m.consumed, nonce)
	if !m.nonces[nonce] {
		return apperr.Unauthenticated("nonce 已失效")
	}
	delete(m.nonces, nonce)
	return nil
}

func (m *appModule) Login(_ context.Context, in identity.LoginInput) (identity.LoginResult, error) {
	m.logins = append(m.logins, in)
	if m.loginErr != nil {
		return identity.LoginResult{}, m.loginErr
	}
	return identity.LoginResult{Token: "session-token", AccountID: 7, AbsoluteExpiresAt: appNow.Add(90 * 24 * time.Hour)}, nil
}

func (m *appModule) LinkIdentity(_ context.Context, accountID int64, id identity.ProviderIdentity) error {
	m.links = append(m.links, linkCall{accountID, id})
	return m.linkErr
}

func (m *appModule) ListSessions(_ context.Context, accountID int64) ([]identity.SessionInfo, error) {
	var out []identity.SessionInfo
	for _, s := range m.sessions {
		if s.AccountID == accountID {
			out = append(out, s)
		}
	}
	return out, m.listErr
}

func (m *appModule) ListLinkedIdentities(_ context.Context, accountID int64) ([]identity.LinkedIdentity, error) {
	return m.linked[accountID], m.listErr
}

func (m *appModule) RevokeSession(_ context.Context, accountID int64, id identity.SessionID) error {
	for i, s := range m.sessions {
		if s.ID == id && s.AccountID == accountID {
			m.sessions = append(m.sessions[:i], m.sessions[i+1:]...)
			m.revoked = append(m.revoked, id)
			return nil
		}
	}
	return apperr.NotFound("session 不存在")
}

func (m *appModule) Account(context.Context, int64) (identity.Account, error) {
	return identity.Account{}, nil
}

// appVerifier 驗證 ID token：只有 "good-token" 有效，nonce 必須等於 wantNonce。
type appVerifier struct {
	p        identity.Provider
	identity identity.ProviderIdentity
	err      error
	calls    int
}

func (v *appVerifier) Provider() identity.Provider { return v.p }

func (v *appVerifier) Verify(_ context.Context, idToken, wantNonce string) (identity.ProviderIdentity, error) {
	v.calls++
	if v.err != nil {
		return identity.ProviderIdentity{}, v.err
	}
	if idToken != "good-token" || wantNonce != "nonce-1" {
		return identity.ProviderIdentity{}, errors.Join(errors.New("fake"), identity.ErrInvalidToken)
	}
	return v.identity, nil
}

type appEnv struct {
	m      *appModule
	google *appVerifier
	public *Public
	app    *App
	now    time.Time
}

func newAppEnv(t *testing.T) *appEnv {
	t.Helper()
	e := &appEnv{m: newAppModule(), now: appNow}
	e.google = &appVerifier{p: identity.ProviderGoogle, identity: identity.ProviderIdentity{Provider: identity.ProviderGoogle, Subject: "g-1", Email: "a@gmail.com", EmailVerified: true}}
	login, err := NewAppLogin(e.m, e.google)
	if err != nil {
		t.Fatal(err)
	}
	e.public = NewPublic(login)
	e.app = NewApp(e.m, nil, login, func() time.Time { return e.now })
	return e
}

// kindOf 回傳錯誤的分類；傳輸層自己判斷的 invalid_argument（rpc.Error）不是 apperr，以 Connect 的錯誤碼字串辨認。
func kindOf(err error) apperr.Kind {
	if strings.HasPrefix(err.Error(), "invalid_argument") {
		return apperr.KindInvalidInput
	}
	return apperr.KindOf(err)
}

func exchangeRequest(provider pb.LoginProvider, idToken, nonce string, header ...string) *connect.Request[pb.ExchangeIdTokenRequest] {
	req := connect.NewRequest(&pb.ExchangeIdTokenRequest{Provider: provider, IdToken: idToken, Nonce: nonce})
	for i := 0; i+1 < len(header); i += 2 {
		req.Header().Set(header[i], header[i+1])
	}
	return req
}

func TestExchangeIdTokenIssuesAnAppSession(t *testing.T) {
	t.Parallel()
	e := newAppEnv(t)
	resp, err := e.public.ExchangeIdToken(t.Context(), exchangeRequest(pb.LoginProvider_LOGIN_PROVIDER_GOOGLE, "good-token", "nonce-1"))
	if err != nil {
		t.Fatal(err)
	}
	msg := resp.Msg
	if msg.GetSessionToken() != "session-token" || !msg.GetAbsoluteExpireTime().AsTime().Equal(appNow.Add(90*24*time.Hour)) || msg.GetIdleTimeout().AsDuration() != 14*24*time.Hour {
		t.Fatalf("回應 = %v", msg)
	}
	if len(e.m.logins) != 1 || e.m.logins[0].Client != identity.ClientApp || e.m.logins[0].Identity.Subject != "g-1" || e.m.logins[0].PreviousToken != "" {
		t.Fatalf("logins = %+v", e.m.logins)
	}
}

func TestExchangeIdTokenReplacesOnlyABearerSession(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		header []string
		want   string
	}{
		"Bearer": {[]string{"Authorization", "Bearer old-token"}, "old-token"},
		"cookie": {[]string{"Cookie", rpc.SessionCookie + "=old-token"}, ""},
		"沒有憑證":   {nil, ""},
	} {
		e := newAppEnv(t)
		if _, err := e.public.ExchangeIdToken(t.Context(), exchangeRequest(pb.LoginProvider_LOGIN_PROVIDER_GOOGLE, "good-token", "nonce-1", tc.header...)); err != nil {
			t.Fatal(err)
		}
		if got := e.m.logins[0].PreviousToken; got != tc.want {
			t.Errorf("%s：PreviousToken = %q，預期 %q", name, got, tc.want)
		}
	}
}

func TestExchangeIdTokenRefusesAReplayedNonce(t *testing.T) {
	t.Parallel()
	e := newAppEnv(t)
	req := func() *connect.Request[pb.ExchangeIdTokenRequest] {
		return exchangeRequest(pb.LoginProvider_LOGIN_PROVIDER_GOOGLE, "good-token", "nonce-1")
	}
	if _, err := e.public.ExchangeIdToken(t.Context(), req()); err != nil {
		t.Fatal(err)
	}
	if _, err := e.public.ExchangeIdToken(t.Context(), req()); apperr.KindOf(err) != apperr.KindUnauthenticated {
		t.Fatalf("重放的 nonce：err = %v，預期 Unauthenticated", err)
	}
	if len(e.m.logins) != 1 {
		t.Fatalf("重放的請求還是登入了：%d 次", len(e.m.logins))
	}
}

func TestExchangeIdTokenRefusesInvalidTokensWithoutSpendingTheNonce(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		idToken, nonce string
	}{
		"ID token 無效": {"bad-token", "nonce-1"},
		"nonce 不符":    {"good-token", "nonce-2"},
	} {
		e := newAppEnv(t)
		_, err := e.public.ExchangeIdToken(t.Context(), exchangeRequest(pb.LoginProvider_LOGIN_PROVIDER_GOOGLE, tc.idToken, tc.nonce))
		if apperr.KindOf(err) != apperr.KindUnauthenticated {
			t.Errorf("%s：err = %v，預期 Unauthenticated", name, err)
		}
		if len(e.m.logins) != 0 || len(e.m.consumed) != 0 || !e.m.nonces["nonce-1"] {
			t.Errorf("%s：被拒絕的 ID token 不能登入，也不能消耗 nonce（consumed=%v）", name, e.m.consumed)
		}
	}
}

func TestExchangeIdTokenMapsProviderOutagesAndMissingProviders(t *testing.T) {
	t.Parallel()
	e := newAppEnv(t)
	e.google.err = errors.New("抓不到 JWKS")
	if _, err := e.public.ExchangeIdToken(t.Context(), exchangeRequest(pb.LoginProvider_LOGIN_PROVIDER_GOOGLE, "good-token", "nonce-1")); apperr.KindOf(err) != apperr.KindUnavailable {
		t.Errorf("provider 故障：err = %v，預期 Unavailable", err)
	}
	if _, err := e.public.ExchangeIdToken(t.Context(), exchangeRequest(pb.LoginProvider_LOGIN_PROVIDER_LINE, "good-token", "nonce-1")); apperr.KindOf(err) != apperr.KindUnavailable {
		t.Errorf("沒有設定的 provider：err = %v，預期 Unavailable", err)
	}
	if len(e.m.logins) != 0 || len(e.m.consumed) != 0 {
		t.Error("故障時不能登入或消耗 nonce")
	}
}

func TestExchangeIdTokenRefusesIncompleteRequests(t *testing.T) {
	t.Parallel()
	for name, req := range map[string]*connect.Request[pb.ExchangeIdTokenRequest]{
		"沒有 provider":  exchangeRequest(pb.LoginProvider_LOGIN_PROVIDER_UNSPECIFIED, "good-token", "nonce-1"),
		"未知的 provider": exchangeRequest(pb.LoginProvider(99), "good-token", "nonce-1"),
		"沒有 ID token":  exchangeRequest(pb.LoginProvider_LOGIN_PROVIDER_GOOGLE, "", "nonce-1"),
		"沒有 nonce":     exchangeRequest(pb.LoginProvider_LOGIN_PROVIDER_GOOGLE, "good-token", ""),
	} {
		e := newAppEnv(t)
		_, err := e.public.ExchangeIdToken(t.Context(), req)
		if kindOf(err) != apperr.KindInvalidInput {
			t.Errorf("%s：err = %v，預期 InvalidInput", name, err)
		}
		if e.google.calls != 0 || len(e.m.logins) != 0 {
			t.Errorf("%s：不完整的請求還是碰了驗證或登入", name)
		}
	}
}

func TestExchangeIdTokenPassesTheModulesLoginRefusals(t *testing.T) {
	t.Parallel()
	e := newAppEnv(t)
	e.m.loginErr = apperr.Unauthenticated("這個登入身分的帳號已刪除")
	if _, err := e.public.ExchangeIdToken(t.Context(), exchangeRequest(pb.LoginProvider_LOGIN_PROVIDER_GOOGLE, "good-token", "nonce-1")); apperr.KindOf(err) != apperr.KindUnauthenticated {
		t.Fatalf("err = %v，預期 Unauthenticated", err)
	}
}

func TestIssueAppLoginNonce(t *testing.T) {
	t.Parallel()
	e := newAppEnv(t)
	resp, err := e.public.IssueAppLoginNonce(t.Context(), connect.NewRequest(&pb.IssueAppLoginNonceRequest{}))
	if err != nil || resp.Msg.GetNonce() != "nonce-1" || !resp.Msg.GetExpireTime().AsTime().Equal(appNow.Add(5*time.Minute)) {
		t.Fatalf("resp = %v, err = %v", resp, err)
	}
}

func TestNewAppLoginRejectsDuplicateOrUnknownProviders(t *testing.T) {
	t.Parallel()
	m := newAppModule()
	g := &appVerifier{p: identity.ProviderGoogle}
	if _, err := NewAppLogin(m, g, &appVerifier{p: identity.ProviderGoogle}); err == nil {
		t.Error("同一家設定兩次沒有回錯")
	}
	if _, err := NewAppLogin(m, &appVerifier{p: identity.Provider("FACEBOOK")}); err == nil {
		t.Error("未知的 provider 沒有回錯")
	}
}

func TestLinkIdentityLinksToTheCallersAccount(t *testing.T) {
	t.Parallel()
	e := newAppEnv(t)
	ctx := callerContext(t, 42, uuid.New(), rpc.CredentialBearer)
	req := connect.NewRequest(&pb.LinkIdentityRequest{Provider: pb.LoginProvider_LOGIN_PROVIDER_GOOGLE, IdToken: "good-token", Nonce: "nonce-1"})
	if _, err := e.app.LinkIdentity(ctx, req); err != nil {
		t.Fatal(err)
	}
	if len(e.m.links) != 1 || e.m.links[0].accountID != 42 || e.m.links[0].identity.Subject != "g-1" || len(e.m.logins) != 0 {
		t.Fatalf("links = %+v logins = %+v", e.m.links, e.m.logins)
	}
	if _, err := e.app.LinkIdentity(ctx, req); apperr.KindOf(err) != apperr.KindUnauthenticated {
		t.Fatalf("重放的 nonce：err = %v，預期 Unauthenticated", err)
	}
}

func TestLinkIdentityConflictCarriesTheReason(t *testing.T) {
	t.Parallel()
	e := newAppEnv(t)
	e.m.linkErr = apperr.AlreadyExists("x").WithReason(apperr.ReasonIdentityAlreadyLinked)
	req := connect.NewRequest(&pb.LinkIdentityRequest{Provider: pb.LoginProvider_LOGIN_PROVIDER_GOOGLE, IdToken: "good-token", Nonce: "nonce-1"})
	_, err := e.app.LinkIdentity(callerContext(t, 42, uuid.New(), rpc.CredentialCookie), req)
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Kind() != apperr.KindAlreadyExists || ae.Reason != apperr.ReasonIdentityAlreadyLinked {
		t.Fatalf("err = %v", err)
	}
}

func callerContext(t *testing.T, account int64, session uuid.UUID, source rpc.CredentialSource) context.Context {
	t.Helper()
	ctx := rpc.WithPrincipal(t.Context(), identity.Principal{Kind: identity.PrincipalAccount, ID: account})
	return rpc.WithSession(ctx, rpc.Session{ID: session, Client: identity.ClientWeb, Source: source})
}

func TestHandlersRefuseARequestWithoutASession(t *testing.T) {
	t.Parallel()
	e := newAppEnv(t)
	withPrincipalOnly := rpc.WithPrincipal(t.Context(), identity.Principal{Kind: identity.PrincipalAccount, ID: 1})
	for name, call := range map[string]func() error{
		"SignOut": func() error {
			_, err := e.app.SignOut(withPrincipalOnly, connect.NewRequest(&pb.SignOutRequest{}))
			return err
		},
		"ListSessions": func() error {
			_, err := e.app.ListSessions(withPrincipalOnly, connect.NewRequest(&pb.ListSessionsRequest{}))
			return err
		},
		"ListLinkedIdentities": func() error {
			_, err := e.app.ListLinkedIdentities(withPrincipalOnly, connect.NewRequest(&pb.ListLinkedIdentitiesRequest{}))
			return err
		},
		"RevokeSession": func() error {
			_, err := e.app.RevokeSession(withPrincipalOnly, connect.NewRequest(&pb.RevokeSessionRequest{}))
			return err
		},
	} {
		if err := call(); err == nil || apperr.KindOf(err) != apperr.KindInternal && !strings.Contains(err.Error(), "interceptor") {
			t.Errorf("%s：err = %v，沒有 session 是組裝錯誤", name, err)
		}
	}
}

func TestSignOutRevokesTheCurrentSessionAndClearsOnlyTheCookie(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		source      rpc.CredentialSource
		wantCleared bool
	}{
		"cookie": {rpc.CredentialCookie, true},
		"Bearer": {rpc.CredentialBearer, false},
	} {
		e := newAppEnv(t)
		cur, other := uuid.New(), uuid.New()
		e.m.sessions = []identity.SessionInfo{{ID: cur, AccountID: 42}, {ID: other, AccountID: 42}}
		resp, err := e.app.SignOut(callerContext(t, 42, cur, tc.source), connect.NewRequest(&pb.SignOutRequest{}))
		if err != nil {
			t.Fatal(err)
		}
		if len(e.m.sessions) != 1 || e.m.sessions[0].ID != other {
			t.Errorf("%s：剩下的 session = %+v，只該撤銷目前這個", name, e.m.sessions)
		}
		var cleared *http.Cookie
		for _, c := range (&http.Response{Header: resp.Header()}).Cookies() {
			if c.Name == rpc.SessionCookie {
				cleared = c
			}
		}
		if tc.wantCleared != (cleared != nil) {
			t.Errorf("%s：cookie = %+v", name, cleared)
		}
		if cleared != nil && (cleared.MaxAge >= 0 || cleared.Value != "" || !cleared.Secure || !cleared.HttpOnly || cleared.Path != "/") {
			t.Errorf("%s：清除用的 cookie = %+v", name, cleared)
		}
	}
}

func TestSignOutSucceedsWhenTheSessionIsAlreadyGone(t *testing.T) {
	t.Parallel()
	e := newAppEnv(t)
	if _, err := e.app.SignOut(callerContext(t, 42, uuid.New(), rpc.CredentialBearer), connect.NewRequest(&pb.SignOutRequest{})); err != nil {
		t.Fatal(err)
	}
}

func sessionAt(account int64, created time.Time, last time.Time) identity.SessionInfo {
	return identity.SessionInfo{
		ID: uuid.NewV7(), AccountID: account, Client: identity.ClientWeb, CreatedAt: created, LastUsedAt: last,
		AbsoluteExpiresAt: created.Add(90 * 24 * time.Hour), AuthTime: created,
	}
}

func TestListSessionsMarksTheCurrentOneAndSkipsExpiredOnes(t *testing.T) {
	t.Parallel()
	e := newAppEnv(t)
	oldest := sessionAt(42, appNow.Add(-30*24*time.Hour), appNow.Add(-20*24*time.Hour)) // 閒置超過 7 天
	idle := sessionAt(42, appNow.Add(-3*24*time.Hour), appNow.Add(-2*time.Hour))
	cur := sessionAt(42, appNow.Add(-time.Hour), appNow.Add(-time.Minute))
	cur.Client = identity.ClientApp
	someoneElse := sessionAt(43, appNow, appNow)
	e.m.sessions = []identity.SessionInfo{cur, idle, oldest, someoneElse}
	resp, err := e.app.ListSessions(callerContext(t, 42, cur.ID, rpc.CredentialBearer), connect.NewRequest(&pb.ListSessionsRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	got := resp.Msg.GetSessions()
	if len(got) != 2 || got[0].GetId() != cur.ID.String() || got[1].GetId() != idle.ID.String() {
		t.Fatalf("sessions = %v，預期目前的與閒置中但未逾時的，新到舊", got)
	}
	if !got[0].GetCurrent() || got[1].GetCurrent() || got[0].GetClientKind() != pb.ClientKind_CLIENT_KIND_APP || got[1].GetClientKind() != pb.ClientKind_CLIENT_KIND_WEB {
		t.Errorf("current／client_kind 不對：%v", got)
	}
	if !got[0].GetLastUseTime().AsTime().Equal(cur.LastUsedAt) || !got[0].GetCreateTime().AsTime().Equal(cur.CreatedAt) || resp.Msg.GetNextPageToken() != "" {
		t.Errorf("時間或分頁不對：%v", resp.Msg)
	}
}

func TestListSessionsPagesWithoutRepeatsOrGaps(t *testing.T) {
	t.Parallel()
	e := newAppEnv(t)
	var all []identity.SessionInfo
	for i := range 5 {
		all = append(all, sessionAt(42, appNow.Add(-time.Duration(i)*time.Hour), appNow))
	}
	// 同一時間建立的兩個：以 ID 排序。
	same := appNow.Add(-10 * time.Hour)
	a, b := sessionAt(42, same, appNow), sessionAt(42, same, appNow)
	if a.ID.Compare(b.ID) < 0 {
		a, b = b, a
	}
	all = append(all, a, b)
	e.m.sessions = all
	ctx := callerContext(t, 42, all[0].ID, rpc.CredentialBearer)
	var seen []string
	token := ""
	for range 10 {
		resp, err := e.app.ListSessions(ctx, connect.NewRequest(&pb.ListSessionsRequest{PageSize: 3, PageToken: token}))
		if err != nil {
			t.Fatal(err)
		}
		for _, s := range resp.Msg.GetSessions() {
			seen = append(seen, s.GetId())
		}
		if token = resp.Msg.GetNextPageToken(); token == "" {
			break
		}
	}
	if len(seen) != 7 {
		t.Fatalf("看到 %d 筆：%v", len(seen), seen)
	}
	want := []string{all[0].ID.String(), all[1].ID.String(), all[2].ID.String(), all[3].ID.String(), all[4].ID.String(), a.ID.String(), b.ID.String()}
	for i := range want {
		if seen[i] != want[i] {
			t.Fatalf("第 %d 筆 = %s，預期 %s（新到舊，同時間 ID 大的在前）", i, seen[i], want[i])
		}
	}
}

func TestListSessionsRefusesBadPagingInput(t *testing.T) {
	t.Parallel()
	e := newAppEnv(t)
	ctx := callerContext(t, 42, uuid.New(), rpc.CredentialBearer)
	if _, err := e.app.ListSessions(ctx, connect.NewRequest(&pb.ListSessionsRequest{PageSize: -1})); kindOf(err) != apperr.KindInvalidInput {
		t.Errorf("負的 page_size：err = %v", err)
	}
	if _, err := e.app.ListSessions(ctx, connect.NewRequest(&pb.ListSessionsRequest{PageToken: "garbage"})); kindOf(err) != apperr.KindInvalidInput {
		t.Errorf("亂的 page_token：err = %v", err)
	}
}

func TestRevokeSessionNeedsRecentAuthentication(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		age  time.Duration
		deny bool
	}{
		"剛登入":       {0, false},
		"14 分 59 秒": {identity.ReauthWindow - time.Second, false},
		"剛好 15 分鐘":  {identity.ReauthWindow, false},
		"15 分 01 秒": {identity.ReauthWindow + time.Second, true},
		"登入後一天":     {24 * time.Hour, true},
	} {
		e := newAppEnv(t)
		cur := sessionAt(42, appNow.Add(-48*time.Hour), appNow)
		cur.AuthTime = appNow.Add(-tc.age)
		target := sessionAt(42, appNow.Add(-time.Hour), appNow)
		e.m.sessions = []identity.SessionInfo{cur, target}
		_, err := e.app.RevokeSession(callerContext(t, 42, cur.ID, rpc.CredentialBearer), connect.NewRequest(&pb.RevokeSessionRequest{SessionId: target.ID.String()}))
		if tc.deny {
			var ae *apperr.Error
			if !errors.As(err, &ae) || ae.Kind() != apperr.KindFailedPrecondition || ae.Reason != apperr.ReasonReauthenticationRequired {
				t.Errorf("%s：err = %v，預期 FailedPrecondition 與 REAUTHENTICATION_REQUIRED", name, err)
			}
			if len(e.m.revoked) != 0 {
				t.Errorf("%s：未重新驗證還是撤銷了 session", name)
			}
			continue
		}
		if err != nil || len(e.m.revoked) != 1 || e.m.revoked[0] != target.ID {
			t.Errorf("%s：err = %v，revoked = %v", name, err, e.m.revoked)
		}
	}
}

func TestRevokeSessionRefusesOtherAccountsAndBadIDs(t *testing.T) {
	t.Parallel()
	e := newAppEnv(t)
	cur := sessionAt(42, appNow, appNow)
	others := sessionAt(43, appNow, appNow)
	e.m.sessions = []identity.SessionInfo{cur, others}
	ctx := callerContext(t, 42, cur.ID, rpc.CredentialBearer)
	if _, err := e.app.RevokeSession(ctx, connect.NewRequest(&pb.RevokeSessionRequest{SessionId: others.ID.String()})); apperr.KindOf(err) != apperr.KindNotFound {
		t.Errorf("別人的 session：err = %v，預期 NotFound", err)
	}
	if _, err := e.app.RevokeSession(ctx, connect.NewRequest(&pb.RevokeSessionRequest{SessionId: "not-a-uuid"})); kindOf(err) != apperr.KindInvalidInput {
		t.Errorf("格式不對的 ID：err = %v", err)
	}
	if len(e.m.sessions) != 2 {
		t.Error("被拒絕的請求不能改資料")
	}
}

func TestRevokeSessionWhenTheCurrentSessionIsGone(t *testing.T) {
	t.Parallel()
	e := newAppEnv(t)
	target := sessionAt(42, appNow, appNow)
	e.m.sessions = []identity.SessionInfo{target}
	if _, err := e.app.RevokeSession(callerContext(t, 42, uuid.New(), rpc.CredentialBearer), connect.NewRequest(&pb.RevokeSessionRequest{SessionId: target.ID.String()})); apperr.KindOf(err) != apperr.KindUnauthenticated {
		t.Fatalf("err = %v，預期 Unauthenticated", err)
	}
}

func TestEnumsMatchTheContract(t *testing.T) {
	t.Parallel()
	if m := rpc.EnumMismatches[pb.LoginProvider](identity.LoginProviders()); len(m) != 0 {
		t.Error(m)
	}
	if m := rpc.EnumMismatches[pb.ClientKind](identity.ClientKinds()); len(m) != 0 {
		t.Error(m)
	}
}

func TestListLinkedIdentitiesReturnsOnlyTheCallersOwn(t *testing.T) {
	t.Parallel()
	e := newAppEnv(t)
	t1, t2, t3 := appNow.Add(-48*time.Hour), appNow.Add(-24*time.Hour), appNow.Add(-time.Hour)
	e.m.linked = map[int64][]identity.LinkedIdentity{
		42: {{Provider: identity.ProviderGoogle, LinkedAt: t1}, {Provider: identity.ProviderLine, LinkedAt: t2}, {Provider: identity.ProviderGoogle, LinkedAt: t3}},
		43: {{Provider: identity.ProviderLine, LinkedAt: t1}},
	}
	resp, err := e.app.ListLinkedIdentities(callerContext(t, 42, uuid.New(), rpc.CredentialBearer), connect.NewRequest(&pb.ListLinkedIdentitiesRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	got := resp.Msg.GetIdentities()
	want := []struct {
		p pb.LoginProvider
		t time.Time
	}{{pb.LoginProvider_LOGIN_PROVIDER_GOOGLE, t1}, {pb.LoginProvider_LOGIN_PROVIDER_LINE, t2}, {pb.LoginProvider_LOGIN_PROVIDER_GOOGLE, t3}}
	if len(got) != len(want) {
		t.Fatalf("identities = %v，預期呼叫者自己的 3 筆（同一 provider 可多筆）", got)
	}
	for i, w := range want {
		if got[i].GetProvider() != w.p || !got[i].GetLinkTime().AsTime().Equal(w.t) {
			t.Errorf("identities[%d] = %v，預期 %v %v", i, got[i], w.p, w.t)
		}
	}
	none, err := e.app.ListLinkedIdentities(callerContext(t, 99, uuid.New(), rpc.CredentialBearer), connect.NewRequest(&pb.ListLinkedIdentitiesRequest{}))
	if err != nil || len(none.Msg.GetIdentities()) != 0 {
		t.Errorf("沒有連結的帳號：%v %v", none, err)
	}
}

func TestListLinkedIdentitiesRefusesAnUnknownProvider(t *testing.T) {
	t.Parallel()
	e := newAppEnv(t)
	e.m.linked = map[int64][]identity.LinkedIdentity{42: {{Provider: "FACEBOOK", LinkedAt: appNow}}}
	_, err := e.app.ListLinkedIdentities(callerContext(t, 42, uuid.New(), rpc.CredentialBearer), connect.NewRequest(&pb.ListLinkedIdentitiesRequest{}))
	if err == nil || apperr.KindOf(err) != apperr.KindInternal {
		t.Fatalf("不認得的 provider 要回內部錯誤，err = %v", err)
	}
}
