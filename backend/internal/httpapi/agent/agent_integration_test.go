//go:build integration

package agent_test

import (
	"context"
	"errors"
	"github.com/Will413028/hawkr/backend/internal/httpapi/characterize"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"

	"github.com/Will413028/hawkr/backend/internal/httpapi"
	httpagent "github.com/Will413028/hawkr/backend/internal/httpapi/agent"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/agent/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/agent/v1alpha1/agentv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	httpseller "github.com/Will413028/hawkr/backend/internal/httpapi/seller"
	"github.com/Will413028/hawkr/backend/internal/legacy/gatewayproxy"
	"github.com/Will413028/hawkr/backend/internal/modules/accountdeletion"
	"github.com/Will413028/hawkr/backend/internal/modules/agent"
	"github.com/Will413028/hawkr/backend/internal/modules/delegation"
	"github.com/Will413028/hawkr/backend/internal/modules/discovery"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/reviewqueue"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

var clock = time.Date(2026, 10, 5, 4, 0, 0, 0, time.UTC)

// prospects 是 seller 的 ProspectContact port 的替身。
type prospects struct{ email string }

func (p prospects) ProspectContact(_ context.Context, id int64) (seller.ProspectContact, error) {
	return seller.ProspectContact{ProspectID: id, DisplayName: "業者", Email: &p.email, Status: seller.ProspectStatusActive}, nil
}

// googleDouble 是 Google ID token 驗證的替身：token 的格式是 kind|nonce|分鐘前，kind 決定結果，nonce 與 iat 是 token 內的 claim
// （由 identity 的 VerifyGoogle 比對 session 推導的 nonce 與新鮮度）。
type googleDouble struct{}

func idToken(kind, nonce string, minutesAgo int) string {
	return kind + "|" + nonce + "|" + strconv.Itoa(minutesAgo)
}

func (googleDouble) Verify(_ context.Context, token string) (identity.Identity, error) {
	parts := strings.SplitN(token, "|", 3)
	if len(parts) != 3 {
		return identity.Identity{}, identity.ErrInvalidToken
	}
	minutes, err := strconv.Atoi(parts[2])
	if err != nil {
		return identity.Identity{}, identity.ErrInvalidToken
	}
	claims := identity.Identity{Subject: "g1", Nonce: parts[1], IssuedAt: clock.Add(-time.Duration(minutes) * time.Minute)}
	switch parts[0] {
	case "good":
		claims.Email, claims.EmailVerified = " Vendor@Example.com ", true
	case "unverified":
		claims.Email = "vendor@example.com"
	case "other":
		claims.Email, claims.EmailVerified = "someone@other.example", true
	case "jwks-down":
		return identity.Identity{}, errors.New("jwks unreachable")
	default:
		return identity.Identity{}, identity.ErrInvalidToken
	}
	return claims, nil
}

type noBusiness struct{}

func (noBusiness) CurrentBuyerID(context.Context, int64) (int64, error) { return 0, nil }
func (noBusiness) CurrentStore(context.Context, int64) (seller.Store, error) {
	return seller.Store{}, apperr.NotFound("沒有現任店家")
}

type noDashboardOrders struct{}

func (noDashboardOrders) SalesStats(context.Context, int64, []httpseller.SalesWindow) ([]httpseller.SalesStats, error) {
	return nil, errors.New("not used")
}

func (noDashboardOrders) StatusCounts(context.Context, int64) (httpseller.OrderStatusCounts, error) {
	return httpseller.OrderStatusCounts{}, errors.New("not used")
}

// gatewayStub 記下 gateway 收到的 vendor portal header。
type gatewayStub struct {
	mu   sync.Mutex
	seen []string
}

func (g *gatewayStub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var out []string
	for k, v := range r.Header {
		if strings.HasPrefix(strings.ToLower(k), "x-vendor-portal-") {
			out = append(out, k+"="+strings.Join(v, ","))
		}
	}
	slices.Sort(out)
	g.mu.Lock()
	g.seen = out
	g.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

func (g *gatewayStub) headers() []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.seen
}

func cookieHeader(pairs ...string) string { return strings.Join(pairs, "; ") }

func named(recipient int64, token string) string {
	return rpc.VendorPortalCookieName(recipient) + "=" + token
}

// 以 agent 的 EnsureInvite 種兩張邀請，經完整的 HTTP 層（httpapi.NewHandler）走 Preview → Exchange → GetSession → VerifySession，
// 再發轉給 gateway 的 RFQ 請求：瀏覽器同時持有兩張邀請的 cookie，每個請求只用它指名的那張；入口驗證 cookie，把收件人、session
// 與驗證紀錄交給 gateway。
func TestVendorPortalFromInviteToTheGateway(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	d := pg.NewDB(t, testdb.Jobs(), testdb.Migrations(migrate.Module{Name: "identity", Dir: identity.Migrations()}, migrate.Module{Name: "agent", Dir: agent.Migrations()}))
	jc, err := jobs.NewInsertOnly(d)
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	now := func() time.Time { return clock }
	agents := agent.New(agent.Deps{DB: d, Prospects: prospects{email: "vendor@example.com"}, Now: now})
	id := identity.New(identity.Deps{
		DB: d, Events: events.NewBus(), Jobs: jc, Now: now, Logger: log,
		Providers: identity.Providers{Google: googleDouble{}},
	})
	inviteA, err := agents.EnsureInvite(ctx, agent.EnsureInviteInput{ProspectID: 7, RfqRecipientID: 42, ExpiresAt: clock.Add(24 * time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	inviteB, err := agents.EnsureInvite(ctx, agent.EnsureInviteInput{ProspectID: 8, RfqRecipientID: 43, ExpiresAt: clock.Add(24 * time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	sessionRows := func() int {
		t.Helper()
		var n int
		if err := d.Conn(ctx).QueryRow(ctx, "SELECT count(*) FROM agent.vendor_portal_sessions").Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	stub := &gatewayStub{}
	gw := httptest.NewServer(stub)
	t.Cleanup(gw.Close)
	proxy, err := gatewayproxy.New(gw.URL, func(ctx context.Context) (gatewayproxy.Caller, bool) {
		p, ok := rpc.PrincipalFrom(ctx)
		return gatewayproxy.Caller{Principal: p}, ok
	}, httpagent.SessionOf(agents), noBusiness{}, noBusiness{}, nil, log)
	if err != nil {
		t.Fatal(err)
	}
	h, err := httpapi.NewHandler(httpapi.Deps{Social: &social.Module{},
		Ready: d.Ping, Identity: id, Agent: agents, Deletion: &accountdeletion.Module{}, Seller: &seller.Module{},
		Stats: characterize.Stats(t), WebOrigin: characterize.WebOrigin(t), DashboardOrders: noDashboardOrders{}, Discovery: &discovery.Module{}, ReviewQueue: &reviewqueue.Module{},
		Delegation: delegation.New(delegation.Deps{DB: d, Identity: id, Seller: &seller.Module{}, Now: now, Logger: log}),
		Legacy:     proxy, LegacyPrefixes: gatewayproxy.Prefixes, Now: now, Logger: log,
	})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	public := agentv1alpha1connect.NewVendorPortalPublicServiceClient(srv.Client(), srv.URL, connect.WithProtoJSON())
	invitee := agentv1alpha1connect.NewVendorPortalInviteeServiceClient(srv.Client(), srv.URL, connect.WithProtoJSON())
	inviteeGET := agentv1alpha1connect.NewVendorPortalInviteeServiceClient(srv.Client(), srv.URL, connect.WithProtoJSON(), connect.WithHTTPGet())

	preview, err := public.PreviewInvite(ctx, connect.NewRequest(&pb.PreviewInviteRequest{Secret: inviteA.Secret}))
	if err != nil {
		t.Fatal(err)
	}
	if preview.Msg.GetMaskedEmail() != "v***@example.com" || !preview.Msg.GetExpireTime().AsTime().Equal(clock.Add(24*time.Hour)) {
		t.Fatalf("preview = %v", preview.Msg)
	}
	if _, err := public.PreviewInvite(ctx, connect.NewRequest(&pb.PreviewInviteRequest{Secret: "nope"})); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("未知的 secret：err = %v", err)
	}

	exchange := func(invite agent.IssuedInvite, cookies ...string) *connect.Response[pb.ExchangeInviteResponse] {
		t.Helper()
		req := connect.NewRequest(&pb.ExchangeInviteRequest{Secret: invite.Secret})
		if len(cookies) > 0 {
			req.Header().Set("Cookie", cookieHeader(cookies...))
		}
		resp, err := public.ExchangeInvite(ctx, req)
		if err != nil {
			t.Fatal(err)
		}
		if cc := resp.Header().Get("Cache-Control"); cc != "private, no-store" {
			t.Fatalf("Cache-Control = %q", cc)
		}
		return resp
	}
	issued := func(resp *connect.Response[pb.ExchangeInviteResponse], recipient string) *http.Cookie {
		t.Helper()
		raw := resp.Header().Values("Set-Cookie")
		if len(raw) != 1 {
			t.Fatalf("Set-Cookie = %v", raw)
		}
		c, err := http.ParseSetCookie(raw[0])
		if err != nil {
			t.Fatal(err)
		}
		// session 的絕對期限（7 天）被邀請的到期時間（24 小時）截斷，Max-Age 取截斷後的剩餘秒數。
		if c.Name != "__Host-hk_vp_"+recipient || c.Value == "" || c.Path != "/" || c.Domain != "" ||
			!c.Secure || !c.HttpOnly || c.SameSite != http.SameSiteLaxMode || c.MaxAge != 24*60*60 {
			t.Fatalf("cookie = %+v（%s）", c, raw[0])
		}
		return c
	}

	firstA := exchange(inviteA)
	if firstA.Msg.GetRfqRecipientId() != "42" || firstA.Msg.GetVerification() != nil || firstA.Msg.GetGoogleNonce() == "" {
		t.Fatalf("exchange = %v", firstA.Msg)
	}
	cookieA := issued(firstA, "42")
	nonceA := firstA.Msg.GetGoogleNonce()
	if nonceA != httpagent.GoogleNonce(cookieA.Value) {
		t.Fatal("nonce 要由 cookie 的 session token 推導")
	}
	if _, err := agents.ValidateSession(ctx, cookieA.Value); err != nil {
		t.Fatal(err)
	}

	// 同一個瀏覽器再開另一張邀請：新發 B 的 cookie，A 的 session 與 cookie 不動。
	exchangedB := exchange(inviteB, named(42, cookieA.Value))
	cookieB := issued(exchangedB, "43")
	if sessionRows() != 2 {
		t.Fatalf("session = %d", sessionRows())
	}
	if _, err := agents.ValidateSession(ctx, cookieA.Value); err != nil {
		t.Fatalf("兌換 B 不得動到 A 的 session：%v", err)
	}
	infoB, err := agents.ValidateSession(ctx, cookieB.Value)
	if err != nil {
		t.Fatal(err)
	}

	// A 的有效 cookie 再兌換 A：沿用，不新增 session，也不重設 cookie。
	again := exchange(inviteA, named(42, cookieA.Value), named(43, cookieB.Value))
	if len(again.Header().Values("Set-Cookie")) != 0 || sessionRows() != 2 || again.Msg.GetGoogleNonce() != nonceA || again.Msg.GetVerification() != nil {
		t.Fatalf("沿用：Set-Cookie=%v rows=%d msg=%v", again.Header().Values("Set-Cookie"), sessionRows(), again.Msg)
	}
	// A 的 cookie 無效：發新的 session，nonce 跟著換。
	renewed := exchange(inviteA, named(42, "bogus"))
	renewedCookie := issued(renewed, "42")
	if sessionRows() != 3 || renewed.Msg.GetGoogleNonce() == nonceA || renewedCookie.Value == cookieA.Value {
		t.Fatalf("無效的 cookie：rows=%d", sessionRows())
	}
	cookieA, nonceA = renewedCookie, renewed.Msg.GetGoogleNonce()
	infoA, err := agents.ValidateSession(ctx, cookieA.Value)
	if err != nil {
		t.Fatal(err)
	}
	both := cookieHeader(named(42, cookieA.Value), named(43, cookieB.Value))

	forward := func(path, cookies string) []string {
		t.Helper()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/v1/public/vendor_portal/rfqs/"+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		if cookies != "" {
			req.Header.Set("Cookie", cookies)
		}
		req.Header.Set("X-Vendor-Portal-Rfq-Recipient-Id", "999")
		req.Header.Set("X-Vendor-Portal-Verified-Email", "attacker@example.com")
		resp, err := srv.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("gateway 的回應 = %d", resp.StatusCode)
		}
		return stub.headers()
	}
	unverified := func(recipient string, info agent.SessionInfo) []string {
		return []string{"X-Vendor-Portal-Rfq-Recipient-Id=" + recipient, "X-Vendor-Portal-Session-Id=" + info.SessionID.String()}
	}
	if got := forward("42/response_draft", both); !slices.Equal(got, unverified("42", infoA)) {
		t.Fatalf("A：gateway 收到 %v", got)
	}
	if got := forward("43", both); !slices.Equal(got, unverified("43", infoB)) {
		t.Fatalf("B：gateway 收到 %v", got)
	}
	malformed := cookieHeader("foo", named(42, cookieA.Value), "bad=caf\u00e9")
	if got := forward("42", malformed); !slices.Equal(got, unverified("42", infoA)) {
		t.Fatalf("不相干的壞 cookie 不得讓 A 失效：gateway 收到 %v", got)
	}
	if got := forward("43", named(42, cookieA.Value)); len(got) != 0 {
		t.Fatalf("只有 A 的 cookie 不得為 B 帶 header：%v", got)
	}
	for name, c := range map[string]string{"無效的 cookie": named(42, "bogus"), "沒有 cookie": "", "舊的單一 cookie": "__Host-hk_vendor_portal=" + cookieA.Value} {
		if got := forward("42", c); len(got) != 0 {
			t.Fatalf("%s 不該帶任何 vendor header：%v", name, got)
		}
	}
	if got := forward("042", both); len(got) != 0 {
		t.Fatalf("路徑的 ID 格式不對不該帶 header：%v", got)
	}

	// GetSession（GET）：每張邀請各自的狀態，回應不可快取。
	getReq := func(recipient string, cookies string) *connect.Request[pb.GetSessionRequest] {
		req := connect.NewRequest(&pb.GetSessionRequest{RfqRecipientId: recipient})
		if cookies != "" {
			req.Header().Set("Cookie", cookies)
		}
		return req
	}
	got, err := inviteeGET.GetSession(ctx, getReq("42", both))
	if err != nil {
		t.Fatal(err)
	}
	if got.Msg.GetVerification() != nil || got.Msg.GetGoogleNonce() != nonceA {
		t.Fatalf("GetSession = %v", got.Msg)
	}
	if cc := got.Header().Get("Cache-Control"); cc != "private, no-store" || got.Header().Get("ETag") != "" {
		t.Fatalf("Cache-Control = %q，ETag = %q", cc, got.Header().Get("ETag"))
	}
	if got, err := invitee.GetSession(ctx, getReq("42", malformed)); err != nil || got.Msg.GetGoogleNonce() != nonceA {
		t.Fatalf("不相干的壞 cookie 不得讓 interceptor 找不到 A 的 cookie：%v %v", got, err)
	}
	if _, err := invitee.GetSession(ctx, getReq("42", "")); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("沒有 cookie：err = %v", err)
	}
	if _, err := invitee.GetSession(ctx, getReq("43", named(42, cookieA.Value))); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("A 的 cookie 指名 B：err = %v", err)
	}

	verifyReq := func(recipient string, token string, cookies string) *connect.Request[pb.VerifySessionRequest] {
		req := connect.NewRequest(&pb.VerifySessionRequest{RfqRecipientId: recipient, GoogleIdToken: token})
		if cookies != "" {
			req.Header().Set("Cookie", cookies)
		}
		return req
	}
	if _, err := invitee.VerifySession(ctx, verifyReq("42", idToken("good", nonceA, 1), "")); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("沒有 cookie：err = %v", err)
	}
	var denied []string
	for name, token := range map[string]string{
		"信箱未驗證":           idToken("unverified", nonceA, 1),
		"別人的信箱":           idToken("other", nonceA, 1),
		"無效的 token":       idToken("invalid", nonceA, 1),
		"nonce 不符（別張邀請的）": idToken("good", httpagent.GoogleNonce(cookieB.Value), 1),
		"沒有 nonce":        idToken("good", "", 1),
		"token 太舊":        idToken("good", nonceA, 11),
	} {
		_, err := invitee.VerifySession(ctx, verifyReq("42", token, both))
		var ce *connect.Error
		if !errors.As(err, &ce) || ce.Code() != connect.CodePermissionDenied {
			t.Fatalf("%s：err = %v", name, err)
		}
		denied = append(denied, ce.Message())
	}
	for _, m := range denied[1:] {
		if m != denied[0] {
			t.Fatalf("拒絕的訊息要一致：%q", denied)
		}
	}
	if _, err := invitee.VerifySession(ctx, verifyReq("42", idToken("jwks-down", nonceA, 1), both)); connect.CodeOf(err) != connect.CodeUnavailable {
		t.Fatalf("公鑰問不到：err = %v", err)
	}
	if info, err := agents.ValidateSession(ctx, cookieA.Value); err != nil || info.Verification != nil {
		t.Fatalf("失敗的驗證不得留下紀錄：%+v %v", info, err)
	}

	// 兩張 cookie 在同一個請求：驗證 A 只動 A 的 session。
	verified, err := invitee.VerifySession(ctx, verifyReq("42", idToken("good", nonceA, 9), both))
	if err != nil {
		t.Fatal(err)
	}
	v := verified.Msg.GetVerification()
	if verified.Msg.GetRfqRecipientId() != "42" || v.GetMethod() != pb.SessionVerificationMethod_SESSION_VERIFICATION_METHOD_GOOGLE || !v.GetVerifyTime().AsTime().Equal(clock) {
		t.Fatalf("verify = %v", verified.Msg)
	}
	if cc := verified.Header().Get("Cache-Control"); cc != "private, no-store" {
		t.Fatalf("Cache-Control = %q", cc)
	}
	if info, err := agents.ValidateSession(ctx, cookieB.Value); err != nil || info.Verification != nil {
		t.Fatalf("驗證 A 不得動到 B：%+v %v", info, err)
	}
	gotB, err := invitee.GetSession(ctx, getReq("43", both))
	if err != nil || gotB.Msg.GetVerification() != nil {
		t.Fatalf("B 還沒驗證：%v %v", gotB, err)
	}
	gotA, err := invitee.GetSession(ctx, getReq("42", both))
	if err != nil || gotA.Msg.GetVerification() == nil {
		t.Fatalf("A 已驗證：%v %v", gotA, err)
	}
	// 重新兌換 A（沿用）回報已驗證的狀態。
	if re := exchange(inviteA, both); re.Msg.GetVerification() == nil || sessionRows() != 3 {
		t.Fatalf("沿用已驗證的 session：%v rows=%d", re.Msg, sessionRows())
	}

	if got := forward("42", both); !slices.Equal(got, []string{
		"X-Vendor-Portal-Rfq-Recipient-Id=42",
		"X-Vendor-Portal-Session-Id=" + infoA.SessionID.String(),
		"X-Vendor-Portal-Verification-Method=GOOGLE",
		"X-Vendor-Portal-Verified-At=2026-10-05T04:00:00Z",
		"X-Vendor-Portal-Verified-Email=Vendor@Example.com",
	}) {
		t.Fatalf("已驗證的 A：gateway 收到 %v", got)
	}
	if got := forward("43", both); !slices.Equal(got, unverified("43", infoB)) {
		t.Fatalf("B 仍未驗證：gateway 收到 %v", got)
	}
}
