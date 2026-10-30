//go:build integration

package identity_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"

	"github.com/Will413028/hawkr/backend/internal/httpapi"
	"github.com/Will413028/hawkr/backend/internal/httpapi/characterize"
	adminpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/identity/v1alpha1"
	adminconnect "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/identity/v1alpha1/identityv1alpha1connect"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/identity/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/identity/v1alpha1/identityv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	httpseller "github.com/Will413028/hawkr/backend/internal/httpapi/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/accountdeletion"
	"github.com/Will413028/hawkr/backend/internal/modules/agent"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/delegation"
	"github.com/Will413028/hawkr/backend/internal/modules/discovery"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/reviewqueue"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

type fixture struct {
	url    string
	m      *identity.Module
	db     *db.DB
	dev    *identity.DevAuth
	buyers *buyer.Module

	mu       sync.Mutex
	accounts map[string]int64 // customerToken 的名稱 → 帳號 ID：同一個名稱是同一個帳號
}

type options struct {
	access identity.AdminVerifier
	// legacy 是轉給舊 gateway 的 handler；nil 時不掛。
	legacy http.Handler
}

// setup 起 identity 與單體的入口，後台的身分用本機金鑰簽的 dev token，顧客的身分是 dev session。
func setup(t *testing.T, o options) *fixture {
	t.Helper()
	d := pg.NewDB(t, testdb.Jobs(), testdb.Migrations(
		migrate.Module{Name: "identity", Dir: identity.Migrations()},
		migrate.Module{Name: "buyer", Dir: buyer.Migrations()},
		migrate.Module{Name: "delegation", Dir: delegation.Migrations()},
	))
	jc, err := jobs.NewInsertOnly(d)
	if err != nil {
		t.Fatal(err)
	}
	dev, err := identity.NewDevAuth(nil)
	if err != nil {
		t.Fatal(err)
	}
	providers := identity.Providers{Admin: dev.Admins()}
	if o.access != nil {
		providers.Admin = o.access
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	bus := events.NewBus()
	m := identity.New(identity.Deps{
		DB: d, Events: bus, Providers: providers,
		Now: time.Now, Logger: log,
	})
	// 與正式組裝相同：buyer 在建帳的 transaction 內建立買家並授予 BUYER（cmd/hawkr 的 subscribeBuyerLifecycle）。
	buyers := buyer.New(buyer.Deps{DB: d, Events: bus, Jobs: jc, Now: time.Now, Logger: log})
	events.Subscribe(bus, buyers.OnAccountCreated)
	del := delegation.New(delegation.Deps{DB: d, Identity: m, Events: bus, Seller: &seller.Module{}, Now: time.Now, Logger: log})
	events.Subscribe(bus, del.OnAdminRoleRevoked)
	h, err := httpapi.NewHandler(httpapi.Deps{Social: &social.Module{}, Legacy: o.legacy, LegacyPrefixes: []string{"/api/v1/"}, Ready: d.Ping, Identity: m, Deletion: &accountdeletion.Module{}, Seller: &seller.Module{}, Stats: characterize.Stats(t), WebOrigin: characterize.WebOrigin(t), DashboardOrders: noDashboardOrders{}, Discovery: &discovery.Module{}, Agent: &agent.Module{}, ReviewQueue: &reviewqueue.Module{}, Delegation: del, Origins: []string{"https://web.example"}, Logger: log})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return &fixture{url: srv.URL, m: m, db: d, dev: dev, buyers: buyers}
}

func headers(kv ...string) connect.ClientOption {
	return connect.WithInterceptors(connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			for i := 0; i+1 < len(kv); i += 2 {
				req.Header().Set(kv[i], kv[i+1])
			}
			return next(ctx, req)
		}
	}))
}

func (f *fixture) customerToken(t *testing.T, user string) string {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	id, ok := f.accounts[user]
	if !ok {
		var err error
		if id, err = f.m.DevNewAccount(t.Context()); err != nil {
			t.Fatal(err)
		}
		// dev 帳號沒有登入身分，所以沒有 email；後台列表與 email 篩選讀 accounts.email（provider 回報的 email 的位置），這裡直接寫入，
		// 讓測試以 user 名稱認得帳號。
		if _, err := f.db.Pool().Exec(t.Context(), `UPDATE identity.accounts SET email = $1 WHERE id = $2`, user+"@dev.localhost", id); err != nil {
			t.Fatal(err)
		}
		if f.accounts == nil {
			f.accounts = map[string]int64{}
		}
		f.accounts[user] = id
	}
	tok, err := f.m.DevSession(t.Context(), id)
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func (f *fixture) adminToken(t *testing.T, user string) string {
	t.Helper()
	tok, err := f.dev.AdminToken(user)
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func (f *fixture) app(opts ...connect.ClientOption) identityv1alpha1connect.IdentityAppServiceClient {
	return identityv1alpha1connect.NewIdentityAppServiceClient(http.DefaultClient, f.url, append([]connect.ClientOption{connect.WithProtoJSON()}, opts...)...)
}

func (f *fixture) admin(opts ...connect.ClientOption) adminconnect.IdentityAdminServiceClient {
	return adminconnect.NewIdentityAdminServiceClient(http.DefaultClient, f.url, append([]connect.ClientOption{connect.WithProtoJSON()}, opts...)...)
}

func wantCode(t *testing.T, err error, code connect.Code) {
	t.Helper()
	var ce *connect.Error
	if !errors.As(err, &ce) || ce.Code() != code {
		t.Fatalf("err = %v，預期 %v", err, code)
	}
}

// 建帳就是買家（ADR 2026-09-30-account-created-provisions-buyer）：第一次 GetSession 建帳時同一個 transaction 就有
// 買家；買家不是角色（W5），roles 是空的。帶 X-Active-Role: SELLER 首次登入也一樣：Connect 不讀它，帳號還不是賣家。
func TestGetSessionCreatesTheAccountAsABuyer(t *testing.T) {
	t.Parallel()
	f := setup(t, options{})
	ctx := context.Background()
	for _, tc := range []struct{ user, hint string }{{"dev_buyer", ""}, {"dev_seller", "SELLER"}} {
		kv := []string{"Authorization", "Bearer " + f.customerToken(t, tc.user)}
		if tc.hint != "" {
			kv = append(kv, "X-Active-Role", tc.hint)
		}
		s, err := f.app(headers(kv...)).GetSession(ctx, connect.NewRequest(&pb.GetSessionRequest{}))
		if err != nil {
			t.Fatalf("%s：GetSession = %v", tc.user, err)
		}
		a := s.Msg.GetAccount()
		if a.GetId() == "" || len(a.GetRoles()) != 0 {
			t.Fatalf("%s：GetSession = %v", tc.user, s)
		}
		accountID, err := strconv.ParseInt(a.GetId(), 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		p, err := f.buyers.Profile(ctx, accountID)
		if err != nil || p.AccountID != accountID || p.DisplayName != "dev" {
			t.Fatalf("%s：買家 = %+v, %v", tc.user, p, err)
		}
	}
	var buyers int
	if err := f.db.Pool().QueryRow(ctx, `SELECT count(*) FROM buyer.buyers`).Scan(&buyers); err != nil || buyers != 2 {
		t.Fatalf("買家列數 = %d, %v", buyers, err)
	}
	// 再次登入不會再建。
	if _, err := f.app(headers("Authorization", "Bearer "+f.customerToken(t, "dev_buyer"))).GetSession(ctx, connect.NewRequest(&pb.GetSessionRequest{})); err != nil {
		t.Fatal(err)
	}
	if err := f.db.Pool().QueryRow(ctx, `SELECT count(*) FROM buyer.buyers`).Scan(&buyers); err != nil || buyers != 2 {
		t.Fatalf("再次登入後買家列數 = %d, %v", buyers, err)
	}
	// 成為賣家之後：roles 只列 SELLER；Connect 不讀 X-Active-Role，帶不帶回應都一樣。
	seller := f.customerToken(t, "dev_seller")
	s, err := f.app(headers("Authorization", "Bearer "+seller)).GetSession(ctx, connect.NewRequest(&pb.GetSessionRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	sellerID, _ := strconv.ParseInt(s.Msg.GetAccount().GetId(), 10, 64)
	if err := f.m.GrantRole(ctx, identity.GrantRoleInput{AccountID: sellerID, Role: identity.RoleSeller}); err != nil {
		t.Fatal(err)
	}
	for _, hint := range []string{"SELLER", ""} {
		kv := []string{"Authorization", "Bearer " + seller}
		if hint != "" {
			kv = append(kv, "X-Active-Role", hint)
		}
		s, err := f.app(headers(kv...)).GetSession(ctx, connect.NewRequest(&pb.GetSessionRequest{}))
		if err != nil {
			t.Fatalf("賣家 hint=%q：GetSession = %v", hint, err)
		}
		if a := s.Msg.GetAccount(); !slices.Equal(a.GetRoles(), []pb.Role{pb.Role_ROLE_SELLER}) {
			t.Fatalf("賣家 hint=%q：GetSession = %v, %v", hint, s, err)
		}
	}
}

func TestConnectIgnoresTheOldSessionCookieAndCrossOriginWriteIsDenied(t *testing.T) {
	t.Parallel()
	f := setup(t, options{})
	ctx := context.Background()
	tok := f.customerToken(t, "dev_web")
	web := f.app(headers("Cookie", "__session="+tok))
	_, err := web.GetSession(ctx, connect.NewRequest(&pb.GetSessionRequest{}))
	wantCode(t, err, connect.CodeUnauthenticated)
	withBearer := f.app(headers("Authorization", "Bearer "+tok))
	if s, err := withBearer.GetSession(ctx, connect.NewRequest(&pb.GetSessionRequest{})); err != nil || s.Msg.GetAccount().GetId() == "" {
		t.Fatalf("Bearer 的 GetSession = %v, %v", s, err)
	}
	crossSite := f.app(headers("Authorization", "Bearer "+tok, "Sec-Fetch-Site", "cross-site", "Origin", "https://evil.example"))
	_, err = crossSite.SignOut(ctx, connect.NewRequest(&pb.SignOutRequest{}))
	wantCode(t, err, connect.CodePermissionDenied) // CSRF（http.CrossOriginProtection）回 403
	sameOrigin := f.app(headers("Authorization", "Bearer "+tok, "Sec-Fetch-Site", "same-origin"))
	if _, err := sameOrigin.SignOut(ctx, connect.NewRequest(&pb.SignOutRequest{})); err != nil {
		t.Fatalf("同源的寫入 = %v", err)
	}
}

func TestServicesAreSeparatedByAudience(t *testing.T) {
	t.Parallel()
	f := setup(t, options{})
	ctx := context.Background()
	_, err := f.app().GetSession(ctx, connect.NewRequest(&pb.GetSessionRequest{}))
	wantCode(t, err, connect.CodeUnauthenticated)
	_, err = f.app(headers("Authorization", "Bearer not-a-token")).GetSession(ctx, connect.NewRequest(&pb.GetSessionRequest{}))
	wantCode(t, err, connect.CodeUnauthenticated)
	// 管理員不能呼叫登入後（買家、賣家）的 service；買家不能呼叫後台。
	_, err = f.app(headers("Authorization", "Bearer "+f.adminToken(t, "dev_ops"))).GetSession(ctx, connect.NewRequest(&pb.GetSessionRequest{}))
	wantCode(t, err, connect.CodePermissionDenied)
	_, err = f.admin(headers("Authorization", "Bearer "+f.customerToken(t, "dev_buyer"))).ListAccounts(ctx, connect.NewRequest(&adminpb.ListAccountsRequest{}))
	wantCode(t, err, connect.CodePermissionDenied)
}

func TestAccessJWTRejectsWrongAudienceIssuerExpiryAndNotBefore(t *testing.T) {
	t.Parallel()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	jwk := jose.JSONWebKey{Key: &key.PublicKey, KeyID: "local-test", Algorithm: "RS256", Use: "sig"}
	keys, err := json.Marshal(map[string]any{"keys": []jose.JSONWebKey{jwk}})
	if err != nil {
		t.Fatal(err)
	}
	certs := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/cdn-cgi/access/certs" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(keys)
	}))
	t.Cleanup(certs.Close)
	const audience = "admin-application-aud"
	access, err := identity.NewAccess(identity.AccessConfig{TeamDomain: certs.URL, Audience: audience, HTTPClient: certs.Client()})
	if err != nil {
		t.Fatal(err)
	}
	f := setup(t, options{access: access})
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: key}, (&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", "local-test"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	sign := func(issuer, aud string, expiry, notBefore time.Time) string {
		t.Helper()
		raw, err := jwt.Signed(signer).Claims(jwt.Claims{
			Issuer: issuer, Subject: "access-subject-1", Audience: jwt.Audience{aud},
			IssuedAt: jwt.NewNumericDate(now), NotBefore: jwt.NewNumericDate(notBefore), Expiry: jwt.NewNumericDate(expiry),
		}).Claims(map[string]any{"email": "ops@gmail.com"}).Serialize()
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	ctx := context.Background()
	for name, raw := range map[string]string{
		"wrong audience": sign(certs.URL, "another-application", now.Add(time.Hour), now.Add(-time.Minute)),
		"wrong issuer":   sign("https://another-team.example", audience, now.Add(time.Hour), now.Add(-time.Minute)),
		"expired":        sign(certs.URL, audience, now.Add(-time.Hour), now.Add(-2*time.Hour)),
		"future nbf":     sign(certs.URL, audience, now.Add(2*time.Hour), now.Add(time.Hour)),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := f.admin(headers("Authorization", "Bearer "+raw)).GetSession(ctx, connect.NewRequest(&adminpb.GetSessionRequest{}))
			wantCode(t, err, connect.CodeUnauthenticated)
		})
	}
	good := sign(certs.URL, audience, now.Add(time.Hour), now.Add(-time.Minute))
	admin := f.admin(headers("Authorization", "Bearer "+good))
	first, err := admin.GetSession(ctx, connect.NewRequest(&adminpb.GetSessionRequest{}))
	if err != nil || first.Msg.GetAdmin().GetEmail() != "ops@gmail.com" {
		t.Fatalf("第一次認證 = %v, %v", first, err)
	}
	second, err := admin.GetSession(ctx, connect.NewRequest(&adminpb.GetSessionRequest{}))
	if err != nil || second.Msg.GetAdmin().GetId() != first.Msg.GetAdmin().GetId() {
		t.Fatalf("第二次認證 = %v, %v", second, err)
	}
	var rows int
	if err := f.db.Pool().QueryRow(ctx, `SELECT count(*) FROM identity.admins WHERE access_subject = $1`, "access-subject-1").Scan(&rows); err != nil || rows != 1 {
		t.Fatalf("管理員列數 = %d, %v", rows, err)
	}
	_, err = f.app(headers("Authorization", "Bearer "+good)).GetSession(ctx, connect.NewRequest(&pb.GetSessionRequest{}))
	wantCode(t, err, connect.CodePermissionDenied)
}

func TestAdminSessionAccountListAndSignOut(t *testing.T) {
	t.Parallel()
	f := setup(t, options{})
	ctx := context.Background()
	for _, user := range []string{"dev_a", "dev_b", "dev_c"} {
		if _, err := f.app(headers("Authorization", "Bearer "+f.customerToken(t, user))).GetSession(ctx, connect.NewRequest(&pb.GetSessionRequest{})); err != nil {
			t.Fatal(err)
		}
	}
	characterize.GrantAdminOwner(t, f.m, f.adminToken(t, "dev_ops"))
	admin := f.admin(headers("Authorization", "Bearer "+f.adminToken(t, "dev_ops")))
	s, err := admin.GetSession(ctx, connect.NewRequest(&adminpb.GetSessionRequest{}))
	if a := s.Msg.GetAdmin(); err != nil || a.GetEmail() != "dev_ops@dev.localhost" {
		t.Fatalf("管理員的 session = %v, %v", s, err)
	}
	// 每個帳號都是買家：不帶角色篩選（ROLE_UNSPECIFIED）就是全部的買家。
	page, err := admin.ListAccounts(ctx, connect.NewRequest(&adminpb.ListAccountsRequest{PageSize: 2}))
	if err != nil || len(page.Msg.GetAccounts()) != 2 || page.Msg.GetTotalSize() != 3 || page.Msg.GetNextPageToken() == "" {
		t.Fatalf("第一頁 = %v, %v", page, err)
	}
	next, err := admin.ListAccounts(ctx, connect.NewRequest(&adminpb.ListAccountsRequest{PageSize: 2, PageToken: page.Msg.GetNextPageToken()}))
	if err != nil || len(next.Msg.GetAccounts()) != 1 || next.Msg.GetNextPageToken() != "" {
		t.Fatalf("第二頁 = %v, %v", next, err)
	}
	if got := next.Msg.GetAccounts()[0].GetEmail(); got != "dev_a@dev.localhost" {
		t.Fatalf("最舊的帳號的 email = %q", got)
	}
	adminID, err := strconv.ParseInt(s.Msg.GetAdmin().GetId(), 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.db.Pool().Exec(ctx, `INSERT INTO delegation.delegated_operation_sessions (
		display_code, credential_hash, actor_admin_id, actor_display_name, target_store_id,
		target_seller_account_id, target_store_name, reason_code, reason_label, capabilities,
		issued_at, expires_at, last_active_at, status, idempotency_key, create_request_id, create_correlation_id
	) VALUES ('OPS-0000000000001', decode(repeat('11', 32), 'hex'), $1, 'ops', 1, 2, 'store',
		'SELLER_REQUEST', 'seller request', ARRAY['products.read'],
		date_trunc('second', now()), date_trunc('second', now()) + interval '30 minutes',
		date_trunc('second', now()), 'ACTIVE', 'signout-test', 'signout-test', 'signout-test')`, adminID)
	if err != nil {
		t.Fatal(err)
	}
	// email 完全相符（不分大小寫）：只找到那一個帳號。
	exact, err := admin.ListAccounts(ctx, connect.NewRequest(&adminpb.ListAccountsRequest{Email: " DEV_A@dev.localhost "}))
	if err != nil || len(exact.Msg.GetAccounts()) != 1 || exact.Msg.GetAccounts()[0].GetEmail() != "dev_a@dev.localhost" || exact.Msg.GetTotalSize() != 1 {
		t.Fatalf("email 完全相符 = %v, %v", exact, err)
	}
	if none, err := admin.ListAccounts(ctx, connect.NewRequest(&adminpb.ListAccountsRequest{Email: "dev_a@dev"})); err != nil || len(none.Msg.GetAccounts()) != 0 {
		t.Fatalf("email 部分相符不該命中 = %v, %v", none, err)
	}
	// 換了 email 篩選也不能沿用 page token。
	_, err = admin.ListAccounts(ctx, connect.NewRequest(&adminpb.ListAccountsRequest{PageSize: 2, Email: "x@y.z", PageToken: page.Msg.GetNextPageToken()}))
	wantCode(t, err, connect.CodeInvalidArgument)
	// 換了篩選就不能沿用 page token。
	_, err = admin.ListAccounts(ctx, connect.NewRequest(&adminpb.ListAccountsRequest{PageSize: 2, Query: "x", PageToken: page.Msg.GetNextPageToken()}))
	wantCode(t, err, connect.CodeInvalidArgument)
	// 登出前撤銷代行；沒有代行時也成功。
	if _, err := admin.SignOut(ctx, connect.NewRequest(&adminpb.SignOutRequest{})); err != nil {
		t.Fatal(err)
	}
	var status, reason string
	if err := f.db.Pool().QueryRow(ctx, `SELECT status, revoke_reason FROM delegation.delegated_operation_sessions WHERE actor_admin_id = $1`, adminID).Scan(&status, &reason); err != nil {
		t.Fatal(err)
	}
	if status != "REVOKED" || reason != "admin_logout" {
		t.Fatalf("登出後代行狀態 = %s, %s", status, reason)
	}
}

func TestEnumsMatchTheModule(t *testing.T) {
	t.Parallel()
	for _, s := range rpc.EnumMismatches[pb.Role](identity.Roles()) {
		t.Error(s)
	}
}

// noDashboardOrders 是這個測試不讀儀表板的訂單時的替身：NewHandler 要求提供儀表板訂單的來源。
type noDashboardOrders struct{}

func (noDashboardOrders) SalesStats(context.Context, int64, []httpseller.SalesWindow) ([]httpseller.SalesStats, error) {
	return nil, errors.New("not used")
}

func (noDashboardOrders) StatusCounts(context.Context, int64) (httpseller.OrderStatusCounts, error) {
	return httpseller.OrderStatusCounts{}, errors.New("not used")
}

// GetAccount 以 ID 讀一個存活的帳號；不存在與已刪除（墓碑）的都回 not_found，與 ListAccounts 只列存活帳號一致。
func TestAdminGetAccountFindsALiveAccountAndHidesTheDeadOnes(t *testing.T) {
	t.Parallel()
	f := setup(t, options{})
	ctx := context.Background()
	for _, user := range []string{"dev_a", "dev_b"} {
		if _, err := f.app(headers("Authorization", "Bearer "+f.customerToken(t, user))).GetSession(ctx, connect.NewRequest(&pb.GetSessionRequest{})); err != nil {
			t.Fatal(err)
		}
	}
	characterize.GrantAdminOwner(t, f.m, f.adminToken(t, "dev_ops"))
	admin := f.admin(headers("Authorization", "Bearer "+f.adminToken(t, "dev_ops")))
	page, err := admin.ListAccounts(ctx, connect.NewRequest(&adminpb.ListAccountsRequest{PageSize: 10}))
	if err != nil {
		t.Fatal(err)
	}
	var aID string
	for _, a := range page.Msg.GetAccounts() {
		if a.GetEmail() == "dev_a@dev.localhost" {
			aID = a.GetId()
		}
	}
	if aID == "" {
		t.Fatalf("列表裡找不到 dev_a：%v", page.Msg.GetAccounts())
	}
	// 存在：回傳的是同一個帳號。
	got, err := admin.GetAccount(ctx, connect.NewRequest(&adminpb.GetAccountRequest{AccountId: aID}))
	if err != nil || got.Msg.GetAccount().GetId() != aID || got.Msg.GetAccount().GetEmail() != "dev_a@dev.localhost" {
		t.Fatalf("存活的帳號 = %v, %v", got, err)
	}
	// 不存在。
	_, err = admin.GetAccount(ctx, connect.NewRequest(&adminpb.GetAccountRequest{AccountId: "999999999"}))
	wantCode(t, err, connect.CodeNotFound)
	// 已刪除：墓碑由 identity.DeleteAccount 寫入（accountdeletion 完成刪除時呼叫的也是它）。這個套件的 fixture 把 accountdeletion
	// 接成零值替身，所以不走刪除申請的 HTTP 路徑；刪除流程本身由 accountdeletion 與 httpapi/seller 的整合測試守。
	id, err := strconv.ParseInt(aID, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.m.DeleteAccount(ctx, id); err != nil {
		t.Fatal(err)
	}
	_, err = admin.GetAccount(ctx, connect.NewRequest(&adminpb.GetAccountRequest{AccountId: aID}))
	wantCode(t, err, connect.CodeNotFound)
}
