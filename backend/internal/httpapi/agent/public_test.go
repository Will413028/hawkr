package agent

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
	"uuid"

	"connectrpc.com/connect"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/protobuf/reflect/protoreflect"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/agent/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/agent/v1alpha1/agentv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	model "github.com/Will413028/hawkr/backend/internal/modules/agent"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

var now = time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)

var (
	inviteA = uuid.MustParse("0198a000-0000-7000-8000-00000000000a")
	inviteB = uuid.MustParse("0198a000-0000-7000-8000-00000000000b")
)

const (
	recipientA = int64(42)
	recipientB = int64(43)
	tokenA     = "session-token-a"
	tokenB     = "session-token-b"
)

type fakeVendors struct {
	previewErr, exchangeErr, verifyErr error
	// preview 是 PreviewInvite 回傳的邀請。
	preview model.InvitePreview
	session model.IssuedSession
	// sessions 是有效的 session token。
	sessions    map[string]model.SessionInfo
	validateErr error

	previewed, exchanged string
	exchangeCalls        int
	verifiedTokens       []string
	verifiedEmail        string
	validated            []string
}

func (f *fakeVendors) PreviewInvite(_ context.Context, secret string) (model.InvitePreview, error) {
	f.previewed = secret
	return f.preview, f.previewErr
}

func (f *fakeVendors) ExchangeInvite(_ context.Context, secret string) (model.IssuedSession, error) {
	f.exchanged = secret
	f.exchangeCalls++
	return f.session, f.exchangeErr
}

func (f *fakeVendors) ValidateSession(_ context.Context, token string) (model.SessionInfo, error) {
	f.validated = append(f.validated, token)
	if f.validateErr != nil {
		return model.SessionInfo{}, f.validateErr
	}
	info, ok := f.sessions[token]
	if !ok {
		return model.SessionInfo{}, apperr.Unauthenticated("登入已失效")
	}
	return info, nil
}

func (f *fakeVendors) VerifySession(_ context.Context, token string, method model.VerificationMethod, email string) (model.SessionInfo, error) {
	f.verifiedTokens = append(f.verifiedTokens, token)
	f.verifiedEmail = email
	if f.verifyErr != nil {
		return model.SessionInfo{}, f.verifyErr
	}
	info := f.sessions[token]
	info.Verification = &model.Verification{Method: method, Email: email, At: now}
	return info, nil
}

type fakeGoogle struct {
	id    identity.Identity
	err   error
	calls int
	token string
	nonce string
}

func (f *fakeGoogle) VerifyGoogle(_ context.Context, idToken, wantNonce string) (identity.Identity, error) {
	f.calls++
	f.token, f.nonce = idToken, wantNonce
	return f.id, f.err
}

type harness struct {
	vendors    *fakeVendors
	google     *fakeGoogle
	public     agentv1alpha1connect.VendorPortalPublicServiceClient
	invitee    agentv1alpha1connect.VendorPortalInviteeServiceClient
	inviteeURL string
	client     *http.Client
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	v := &fakeVendors{
		preview: model.InvitePreview{InviteID: inviteA, RfqRecipientID: recipientA, MaskedEmail: "v***@example.com", ExpiresAt: now.Add(48 * time.Hour)},
		sessions: map[string]model.SessionInfo{
			tokenA: {SessionID: uuid.MustParse("0198a000-0000-7000-8000-000000000001"), InviteID: inviteA, RfqRecipientID: recipientA},
			tokenB: {SessionID: uuid.MustParse("0198a000-0000-7000-8000-000000000002"), InviteID: inviteB, RfqRecipientID: recipientB},
		},
	}
	g := &fakeGoogle{id: identity.Identity{Subject: "g1", Email: "vendor@example.com", EmailVerified: true}}
	guard, err := NewInterceptor(v)
	if err != nil {
		t.Fatal(err)
	}
	base := rpc.NewInterceptor(slog.New(slog.DiscardHandler))
	_, publicHandler := agentv1alpha1connect.NewVendorPortalPublicServiceHandler(NewPublic(v, func() time.Time { return now }), connect.WithInterceptors(base))
	_, inviteeHandler := agentv1alpha1connect.NewVendorPortalInviteeServiceHandler(NewInvitee(v, g, nil), connect.WithInterceptors(base, guard))
	mux := http.NewServeMux()
	mux.Handle("/hawkr.agent.v1alpha1.VendorPortalPublicService/", publicHandler)
	mux.Handle("/hawkr.agent.v1alpha1.VendorPortalInviteeService/", inviteeHandler)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return &harness{
		vendors: v, google: g, client: srv.Client(), inviteeURL: srv.URL,
		public:  agentv1alpha1connect.NewVendorPortalPublicServiceClient(srv.Client(), srv.URL),
		invitee: agentv1alpha1connect.NewVendorPortalInviteeServiceClient(srv.Client(), srv.URL),
	}
}

func withCookies[T any](req *connect.Request[T], cookies ...string) *connect.Request[T] {
	req.Header().Set("Cookie", strings.Join(cookies, "; "))
	return req
}

func cookie(recipient int64, token string) string {
	return rpc.VendorPortalCookieName(recipient) + "=" + token
}

func code(err error) connect.Code { return connect.CodeOf(err) }

func TestPreviewInvite(t *testing.T) {
	h := newHarness(t)
	resp, err := h.public.PreviewInvite(t.Context(), connect.NewRequest(&pb.PreviewInviteRequest{Secret: "s3cret"}))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Msg.GetMaskedEmail() != "v***@example.com" || !resp.Msg.GetExpireTime().AsTime().Equal(now.Add(48*time.Hour)) || h.vendors.previewed != "s3cret" {
		t.Fatalf("resp=%v previewed=%q", resp.Msg, h.vendors.previewed)
	}
	if resp.Header().Get("Set-Cookie") != "" {
		t.Fatal("預覽不發 cookie")
	}
}

func TestInviteFailuresAreOneNotFound(t *testing.T) {
	h := newHarness(t)
	h.vendors.previewErr = apperr.NotFound("邀請無法使用")
	h.vendors.exchangeErr = apperr.NotFound("邀請無法使用")
	_, err1 := h.public.PreviewInvite(t.Context(), connect.NewRequest(&pb.PreviewInviteRequest{Secret: "x"}))
	_, err2 := h.public.ExchangeInvite(t.Context(), connect.NewRequest(&pb.ExchangeInviteRequest{Secret: "x"}))
	if code(err1) != connect.CodeNotFound || code(err2) != connect.CodeNotFound {
		t.Fatalf("preview=%v exchange=%v", err1, err2)
	}
	var ce1, ce2 *connect.Error
	if !errors.As(err1, &ce1) || !errors.As(err2, &ce2) || ce1.Message() != ce2.Message() {
		t.Fatalf("兩個方法的訊息要相同：%v / %v", err1, err2)
	}
	if h.vendors.exchangeCalls != 0 {
		t.Fatal("邀請不可用時不得兌換")
	}
}

func TestEmptyRequiredFieldsAreValidationErrors(t *testing.T) {
	h := newHarness(t)
	_, err := h.public.PreviewInvite(t.Context(), connect.NewRequest(&pb.PreviewInviteRequest{}))
	assertViolation(t, err, "secret")
	_, err = h.public.ExchangeInvite(t.Context(), connect.NewRequest(&pb.ExchangeInviteRequest{}))
	assertViolation(t, err, "secret")
	_, err = h.invitee.VerifySession(t.Context(), withCookies(connect.NewRequest(&pb.VerifySessionRequest{RfqRecipientId: "42"}), cookie(recipientA, tokenA)))
	assertViolation(t, err, "google_id_token")
	if h.vendors.previewed != "" || h.vendors.exchanged != "" || h.google.calls != 0 || len(h.vendors.verifiedTokens) != 0 {
		t.Fatal("空的必填欄位不得進到模組或 Google")
	}
}

func assertViolation(t *testing.T, err error, field string) {
	t.Helper()
	var ce *connect.Error
	if !errors.As(err, &ce) || ce.Code() != connect.CodeInvalidArgument {
		t.Fatalf("err = %v，預期 invalid_argument", err)
	}
	for _, d := range ce.Details() {
		v, verr := d.Value()
		if verr != nil {
			continue
		}
		if br, ok := v.(*errdetails.BadRequest); ok && len(br.FieldViolations) == 1 && br.FieldViolations[0].Field == field {
			return
		}
	}
	t.Fatalf("沒有 %s 的 field violation：%v", field, ce.Details())
}

func setCookies(t *testing.T, h http.Header) map[string]*http.Cookie {
	t.Helper()
	out := map[string]*http.Cookie{}
	for _, raw := range h.Values("Set-Cookie") {
		c, err := http.ParseSetCookie(raw)
		if err != nil {
			t.Fatal(err)
		}
		if out[c.Name] != nil {
			t.Fatalf("同名的 Set-Cookie 出現兩次：%v", h.Values("Set-Cookie"))
		}
		out[c.Name] = c
	}
	return out
}

func TestExchangeInviteIssuesTheInvitesCookieAndNonce(t *testing.T) {
	h := newHarness(t)
	h.vendors.preview.RfqRecipientID = 9007199254740993
	h.vendors.session = model.IssuedSession{Token: "tok_abc-123", InviteID: inviteA, RfqRecipientID: 9007199254740993, AbsoluteExpiresAt: now.Add(36*time.Hour + 500*time.Millisecond)}
	resp, err := h.public.ExchangeInvite(t.Context(), connect.NewRequest(&pb.ExchangeInviteRequest{Secret: "s3cret"}))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Msg.GetRfqRecipientId() != "9007199254740993" || h.vendors.exchanged != "s3cret" || resp.Msg.GetVerification() != nil {
		t.Fatalf("resp=%v", resp.Msg)
	}
	if resp.Msg.GetGoogleNonce() != GoogleNonce("tok_abc-123") || resp.Msg.GetGoogleNonce() == "" {
		t.Fatalf("nonce = %q", resp.Msg.GetGoogleNonce())
	}
	cookies := setCookies(t, resp.Header())
	c := cookies["__Host-hk_vp_9007199254740993"]
	if len(cookies) != 1 || c == nil {
		t.Fatalf("Set-Cookie = %v", resp.Header().Values("Set-Cookie"))
	}
	if c.Value != "tok_abc-123" || c.Path != "/" || c.Domain != "" || !c.Secure || !c.HttpOnly || c.SameSite != http.SameSiteLaxMode || c.MaxAge != int((36*time.Hour).Seconds()) {
		t.Fatalf("cookie = %+v", c)
	}
}

func TestExchangeInviteReusesTheSameInvitesValidSession(t *testing.T) {
	h := newHarness(t)
	info := h.vendors.sessions[tokenA]
	info.Verification = &model.Verification{Method: model.VerificationGoogle, Email: "vendor@example.com", At: now.Add(-time.Hour)}
	h.vendors.sessions[tokenA] = info
	resp, err := h.public.ExchangeInvite(t.Context(), withCookies(connect.NewRequest(&pb.ExchangeInviteRequest{Secret: "s3cret"}), cookie(recipientA, tokenA)))
	if err != nil {
		t.Fatal(err)
	}
	if h.vendors.exchangeCalls != 0 {
		t.Fatal("沿用時不得兌換（不新發 session）")
	}
	if got := resp.Header().Values("Set-Cookie"); len(got) != 0 {
		t.Fatalf("沿用時不重設 cookie：%v", got)
	}
	v := resp.Msg.GetVerification()
	if resp.Msg.GetRfqRecipientId() != "42" || v.GetMethod() != pb.SessionVerificationMethod_SESSION_VERIFICATION_METHOD_GOOGLE || !v.GetVerifyTime().AsTime().Equal(now.Add(-time.Hour)) || resp.Msg.GetGoogleNonce() != GoogleNonce(tokenA) {
		t.Fatalf("resp=%v", resp.Msg)
	}
}

func TestExchangeInviteIssuesANewSessionForAnInvalidOrForeignCookie(t *testing.T) {
	issued := model.IssuedSession{Token: "fresh", InviteID: inviteA, RfqRecipientID: recipientA, AbsoluteExpiresAt: now.Add(time.Hour)}
	cases := map[string][]string{
		"這張邀請的 cookie 無效":             {cookie(recipientA, "unknown")},
		"這張邀請的 cookie 是別張邀請的 session": {cookie(recipientA, tokenB)},
		"只有另一張邀請的 cookie":             {cookie(recipientB, tokenB)},
		"沒有 cookie":                   nil,
		"同名兩個":                        {cookie(recipientA, tokenA) + "; " + cookie(recipientA, tokenA)},
	}
	for name, cookies := range cases {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			h.vendors.session = issued
			req := connect.NewRequest(&pb.ExchangeInviteRequest{Secret: "s3cret"})
			if cookies != nil {
				req.Header().Set("Cookie", strings.Join(cookies, "; "))
			}
			resp, err := h.public.ExchangeInvite(t.Context(), req)
			if err != nil {
				t.Fatal(err)
			}
			got := setCookies(t, resp.Header())
			if h.vendors.exchangeCalls != 1 || len(got) != 1 || got[rpc.VendorPortalCookieName(recipientA)].Value != "fresh" {
				t.Fatalf("exchange=%d Set-Cookie=%v", h.vendors.exchangeCalls, resp.Header().Values("Set-Cookie"))
			}
			if resp.Msg.GetGoogleNonce() != GoogleNonce("fresh") || resp.Msg.GetVerification() != nil {
				t.Fatalf("resp=%v", resp.Msg)
			}
		})
	}
}

func TestExchangeInviteSurfacesSessionLookupFailures(t *testing.T) {
	h := newHarness(t)
	h.vendors.validateErr = errors.New("db down")
	_, err := h.public.ExchangeInvite(t.Context(), withCookies(connect.NewRequest(&pb.ExchangeInviteRequest{Secret: "s"}), cookie(recipientA, tokenA)))
	if code(err) != connect.CodeInternal || h.vendors.exchangeCalls != 0 {
		t.Fatalf("err=%v exchange=%d", err, h.vendors.exchangeCalls)
	}
}

func TestExchangeInviteClearsVendorCookiesBeyondTheCap(t *testing.T) {
	for name, reuse := range map[string]bool{"新發": false, "沿用": true} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			h.vendors.session = model.IssuedSession{Token: "fresh", InviteID: inviteA, RfqRecipientID: recipientA, AbsoluteExpiresAt: now.Add(time.Hour)}
			cookies := []string{}
			if reuse {
				cookies = append(cookies, cookie(recipientA, tokenA))
			}
			// 收件人 1..21 之外再加上 recipientA（42）：21 個不同的 vendor cookie（reuse 時含 A）。
			for id := int64(1); len(cookies) < 21; id++ {
				cookies = append(cookies, cookie(id, "x"))
			}
			cookies = append(cookies, "other=keep")
			resp, err := h.public.ExchangeInvite(t.Context(), withCookies(connect.NewRequest(&pb.ExchangeInviteRequest{Secret: "s"}), cookies...))
			if err != nil {
				t.Fatal(err)
			}
			got := setCookies(t, resp.Header())
			cleared := 0
			for n, c := range got {
				if n == rpc.VendorPortalCookieName(recipientA) {
					if c.MaxAge <= 0 {
						t.Fatalf("發出或沿用的 cookie 不得被清掉：%+v", c)
					}
					continue
				}
				if c.MaxAge >= 0 || c.Value != "" {
					t.Fatalf("多出的 cookie 要 Max-Age=0：%+v", c)
				}
				cleared++
			}
			wantCleared := 1
			if !reuse {
				wantCleared = 2 // 新發：請求已有 21 個別張邀請的 cookie，加上這張共 22，清到剩 20。
			}
			if cleared != wantCleared {
				t.Fatalf("清掉 %d 個，預期 %d：%v", cleared, wantCleared, resp.Header().Values("Set-Cookie"))
			}
			// 清掉的是收件人 ID 最小的。
			if got[rpc.VendorPortalCookieName(1)] == nil {
				t.Fatalf("最小的 ID 應該先被清掉：%v", resp.Header().Values("Set-Cookie"))
			}
		})
	}
}

func TestExchangeInviteKeepsAllCookiesAtTheCap(t *testing.T) {
	h := newHarness(t)
	h.vendors.session = model.IssuedSession{Token: "fresh", InviteID: inviteA, RfqRecipientID: recipientA, AbsoluteExpiresAt: now.Add(time.Hour)}
	var cookies []string
	for id := int64(100); len(cookies) < 19; id++ {
		cookies = append(cookies, cookie(id, "x"))
	}
	resp, err := h.public.ExchangeInvite(t.Context(), withCookies(connect.NewRequest(&pb.ExchangeInviteRequest{Secret: "s"}), cookies...))
	if err != nil {
		t.Fatal(err)
	}
	if got := setCookies(t, resp.Header()); len(got) != 1 {
		t.Fatalf("19 個加上新發的剛好 20 個，不清：%v", resp.Header().Values("Set-Cookie"))
	}
}

// Max-Age 是到絕對期限的剩餘秒數，至少 1：剩不到 1 秒（或已經過了）不能是 0 或負數，那會讓瀏覽器立刻刪除 cookie。
func TestSessionCookieMaxAgeHasAFloorOfOneSecond(t *testing.T) {
	for name, remaining := range map[string]time.Duration{"剩 400ms": 400 * time.Millisecond, "剛好到期": 0, "已過期": -time.Minute, "剩 1 秒": time.Second} {
		if got := rpc.NewVendorPortalCookie(1, "t", now.Add(remaining), now).MaxAge; got != 1 {
			t.Errorf("%s：Max-Age = %d，預期 1", name, got)
		}
	}
	if got := rpc.NewVendorPortalCookie(1, "t", now.Add(90*time.Second), now).MaxAge; got != 90 {
		t.Errorf("Max-Age = %d", got)
	}
}

func verifyReq(recipient int64, cookies ...string) *connect.Request[pb.VerifySessionRequest] {
	return withCookies(connect.NewRequest(&pb.VerifySessionRequest{RfqRecipientId: strconv.FormatInt(recipient, 10), GoogleIdToken: "idtoken"}), cookies...)
}

func TestInviteeMethodsWithoutAUsableCookieAreUnauthenticated(t *testing.T) {
	h := newHarness(t)
	cases := map[string][]string{
		"沒有 cookie":    nil,
		"未知的 session":  {cookie(recipientA, "unknown")},
		"別的 cookie 名稱": {"session=" + tokenA},
		"舊的單一 cookie":  {"__Host-hk_vendor_portal=" + tokenA},
		// A 的 cookie 送到指名 B 的請求：cookie 以請求指名的收件人選，B 沒有 cookie。
		"收件人 A 的 cookie 送給 B": {cookie(recipientA, tokenA)},
		// 名稱是 B、值是 A 的 session：session 的收件人與指名的不同。
		"B 的名稱放 A 的 session": {cookie(recipientB, tokenA)},
		"同名兩個":               {cookie(recipientB, tokenB) + "; " + cookie(recipientB, tokenB)},
	}
	for name, cookies := range cases {
		_, verr := h.invitee.VerifySession(t.Context(), verifyReq(recipientB, cookies...))
		_, gerr := h.invitee.GetSession(t.Context(), withCookies(connect.NewRequest(&pb.GetSessionRequest{RfqRecipientId: "43"}), cookies...))
		if code(verr) != connect.CodeUnauthenticated || code(gerr) != connect.CodeUnauthenticated {
			t.Errorf("%s：verify=%v get=%v", name, verr, gerr)
		}
	}
	if h.google.calls != 0 || len(h.vendors.verifiedTokens) != 0 {
		t.Fatal("session 無效時不得呼叫 Google 或模組")
	}
}

func TestInviteeRecipientIDMustBeADecimalPositiveInteger(t *testing.T) {
	h := newHarness(t)
	for _, id := range []string{"", "abc", "0", "-1", "042", "+42", "42 "} {
		_, err := h.invitee.GetSession(t.Context(), withCookies(connect.NewRequest(&pb.GetSessionRequest{RfqRecipientId: id}), cookie(recipientA, tokenA)))
		assertViolation(t, err, "rfq_recipient_id")
	}
	if len(h.vendors.validated) != 0 {
		t.Fatal("格式不對的收件人不得查 session")
	}
}

// 兩張邀請的 cookie 在同一個請求：每個方法只動指名的那張。
func TestTwoCookiesInOneRequestEachMethodTouchesOnlyTheNamedInvite(t *testing.T) {
	h := newHarness(t)
	both := []string{cookie(recipientA, tokenA), cookie(recipientB, tokenB)}
	if _, err := h.invitee.VerifySession(t.Context(), verifyReq(recipientA, both...)); err != nil {
		t.Fatal(err)
	}
	if len(h.vendors.verifiedTokens) != 1 || h.vendors.verifiedTokens[0] != tokenA {
		t.Fatalf("只該驗證 A 的 session：%v", h.vendors.verifiedTokens)
	}
	if h.google.nonce != GoogleNonce(tokenA) {
		t.Fatalf("nonce 要由 A 的 token 推導：%q", h.google.nonce)
	}
	resp, err := h.invitee.GetSession(t.Context(), withCookies(connect.NewRequest(&pb.GetSessionRequest{RfqRecipientId: "43"}), both...))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Msg.GetVerification() != nil || resp.Msg.GetGoogleNonce() != GoogleNonce(tokenB) {
		t.Fatalf("B 還沒驗證：%v", resp.Msg)
	}
	for _, tok := range h.vendors.validated {
		if tok != tokenA && tok != tokenB {
			t.Fatalf("validated %q", tok)
		}
	}
}

func TestGetSessionReportsVerificationAndNonce(t *testing.T) {
	h := newHarness(t)
	info := h.vendors.sessions[tokenA]
	info.Verification = &model.Verification{Method: model.VerificationGoogle, Email: "vendor@example.com", At: now}
	h.vendors.sessions[tokenA] = info
	resp, err := h.invitee.GetSession(t.Context(), withCookies(connect.NewRequest(&pb.GetSessionRequest{RfqRecipientId: "42"}), cookie(recipientA, tokenA)))
	if err != nil {
		t.Fatal(err)
	}
	v := resp.Msg.GetVerification()
	if v.GetMethod() != pb.SessionVerificationMethod_SESSION_VERIFICATION_METHOD_GOOGLE || !v.GetVerifyTime().AsTime().Equal(now) || resp.Msg.GetGoogleNonce() != GoogleNonce(tokenA) {
		t.Fatalf("resp = %v", resp.Msg)
	}
}

func TestGoogleNonceIsDerivedAndDiffersFromTheStoredHash(t *testing.T) {
	first, second := GoogleNonce("a"), GoogleNonce("a")
	if GoogleNonce("a") == GoogleNonce("b") || first != second {
		t.Fatal("nonce 要由 token 決定、不同 token 不同")
	}
	token := base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	raw := make([]byte, 32)
	stored := sha256.Sum256(raw)
	if GoogleNonce(token) == base64.RawURLEncoding.EncodeToString(stored[:]) {
		t.Fatal("nonce 不得等於資料庫存的 token 雜湊")
	}
	if strings.ContainsAny(GoogleNonce(token), "=+/") {
		t.Fatal("nonce 是 base64url、沒有 padding")
	}
}

func TestVerifySessionDeniedCasesShareOneCodeAndMessage(t *testing.T) {
	cases := map[string]func(h *harness){
		"Google token 無效或 nonce 不符": func(h *harness) { h.google.err = apperr.Unauthenticated("Google 驗證失敗") },
		"信箱未驗證":                     func(h *harness) { h.google.id.EmailVerified = false },
		"信箱不符":                      func(h *harness) { h.vendors.verifyErr = apperr.PermissionDenied("無法驗證這個 session") },
		"模組以別的訊息拒絕":                 func(h *harness) { h.vendors.verifyErr = apperr.PermissionDenied("another message") },
	}
	var messages []string
	for name, setup := range cases {
		h := newHarness(t)
		setup(h)
		_, err := h.invitee.VerifySession(t.Context(), verifyReq(recipientA, cookie(recipientA, tokenA)))
		var ce *connect.Error
		if !errors.As(err, &ce) || ce.Code() != connect.CodePermissionDenied {
			t.Fatalf("%s：err = %v", name, err)
		}
		messages = append(messages, ce.Message())
		if name == "信箱未驗證" && len(h.vendors.verifiedTokens) != 0 {
			t.Fatal("email_verified=false 不得進到模組")
		}
	}
	for _, m := range messages[1:] {
		if m != messages[0] {
			t.Fatalf("訊息不一致：%q", messages)
		}
	}
}

func TestVerifySessionGoogleUnavailableIsUnavailable(t *testing.T) {
	h := newHarness(t)
	h.google.err = apperr.Unavailable("Google 驗證目前無法使用")
	_, err := h.invitee.VerifySession(t.Context(), verifyReq(recipientA, cookie(recipientA, tokenA)))
	if code(err) != connect.CodeUnavailable || len(h.vendors.verifiedTokens) != 0 {
		t.Fatalf("err = %v", err)
	}
}

func TestVerifySessionGoogleProgrammingErrorIsInternal(t *testing.T) {
	h := newHarness(t)
	h.google.err = errors.New("identity：VerifyGoogle 需要 nonce")
	_, err := h.invitee.VerifySession(t.Context(), verifyReq(recipientA, cookie(recipientA, tokenA)))
	if code(err) != connect.CodeInternal {
		t.Fatalf("err = %v", err)
	}
}

func TestVerifySessionStoresTheEmailGoogleVerified(t *testing.T) {
	h := newHarness(t)
	resp, err := h.invitee.VerifySession(t.Context(), verifyReq(recipientA, cookie(recipientA, tokenA)))
	if err != nil {
		t.Fatal(err)
	}
	if h.google.token != "idtoken" || h.google.nonce != GoogleNonce(tokenA) || h.vendors.verifiedTokens[0] != tokenA || h.vendors.verifiedEmail != "vendor@example.com" {
		t.Fatalf("google=%q nonce=%q token=%v email=%q", h.google.token, h.google.nonce, h.vendors.verifiedTokens, h.vendors.verifiedEmail)
	}
	v := resp.Msg.GetVerification()
	if resp.Msg.GetRfqRecipientId() != "42" || v.GetMethod() != pb.SessionVerificationMethod_SESSION_VERIFICATION_METHOD_GOOGLE || !v.GetVerifyTime().AsTime().Equal(now) {
		t.Fatalf("resp = %v", resp.Msg)
	}
}

func TestEveryVerificationMethodHasAContractValue(t *testing.T) {
	if m := rpc.EnumMismatches[pb.SessionVerificationMethod](model.VerificationMethodValues()); len(m) > 0 {
		t.Fatal(m)
	}
}

// 授權跟著 service 的邊界：interceptor 以請求的 rfq_recipient_id 選 cookie，所以 service 的每個方法的請求都要有這個欄位。
func TestEveryInviteeRequestNamesTheRecipient(t *testing.T) {
	svc := pb.File_hawkr_agent_v1alpha1_agent_proto.Services().ByName("VendorPortalInviteeService")
	if svc == nil {
		t.Fatal("找不到 service")
	}
	for i := range svc.Methods().Len() {
		m := svc.Methods().Get(i)
		f := m.Input().Fields().ByName("rfq_recipient_id")
		if f == nil || f.Kind() != protoreflect.StringKind {
			t.Errorf("%s 的請求沒有字串欄位 rfq_recipient_id", m.FullName())
		}
	}
}

func TestInterceptorRefusesARequestThatDoesNotNameTheRecipient(t *testing.T) {
	i := &interceptor{sessions: &fakeVendors{}}
	if _, err := i.enter(t.Context(), "/x/Y", http.Header{}, &pb.PreviewInviteRequest{Secret: "s"}); err == nil || connect.CodeOf(err) != connect.CodeUnknown {
		t.Fatalf("err = %v，預期沒有分類的錯誤（internal）", err)
	}
}

func TestSessionOf(t *testing.T) {
	h := newHarness(t)
	of := SessionOf(h.vendors)
	req := func(cookies ...string) *http.Request {
		r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
		if cookies != nil {
			r.Header["Cookie"] = cookies
		}
		return r
	}
	if info, ok, err := of(req(cookie(recipientA, tokenA)), recipientA); err != nil || !ok || info.RfqRecipientID != recipientA {
		t.Fatalf("有效 = %+v %v %v", info, ok, err)
	}
	if info, ok, err := of(req(cookie(recipientA, tokenA), cookie(recipientB, tokenB)), recipientB); err != nil || !ok || info.RfqRecipientID != recipientB || info.SessionID != h.vendors.sessions[tokenB].SessionID {
		t.Fatalf("兩張 cookie 以指名的為準 = %+v %v %v", info, ok, err)
	}
	for name, c := range map[string]struct {
		cookies []string
		id      int64
	}{
		"沒有":                 {nil, recipientA},
		"未知":                 {[]string{cookie(recipientA, "nope")}, recipientA},
		"別名":                 {[]string{"hk_vp_42=" + tokenA}, recipientA},
		"壞格式":                {[]string{"=;;"}, recipientA},
		"只有別張邀請的 cookie":     {[]string{cookie(recipientA, tokenA)}, recipientB},
		"名稱與 session 的收件人不同": {[]string{cookie(recipientB, tokenA)}, recipientB},
		"同名多個":               {[]string{cookie(recipientA, tokenA), cookie(recipientA, tokenA)}, recipientA},
	} {
		if _, ok, err := of(req(c.cookies...), c.id); ok || err != nil {
			t.Errorf("%s：ok=%v err=%v", name, ok, err)
		}
	}
	h.vendors.validateErr = errors.New("db down")
	if _, ok, err := of(req(cookie(recipientA, tokenA)), recipientA); ok || err == nil {
		t.Fatalf("資料庫錯誤要回 error：ok=%v err=%v", ok, err)
	}
}

func TestSessionIsFoundDespiteMalformedOrSplitCookies(t *testing.T) {
	h := newHarness(t)
	req := verifyReq(recipientA)
	req.Header().Set("Cookie", "foo; "+cookie(recipientA, tokenA))
	if _, err := h.invitee.VerifySession(t.Context(), req); err != nil {
		t.Fatalf("Connect，壞 cookie 在前：%v", err)
	}
	req = verifyReq(recipientA)
	req.Header().Add("Cookie", "a=b")
	req.Header().Add("Cookie", cookie(recipientA, tokenA))
	if _, err := h.invitee.VerifySession(t.Context(), req); err != nil {
		t.Fatalf("Connect，第二行 Cookie：%v", err)
	}
	of := SessionOf(h.vendors)
	for name, lines := range map[string][]string{
		"gateway，壞 cookie 在前": {"foo; " + cookie(recipientA, tokenA)},
		"gateway，第二行":         {"a=b", cookie(recipientA, tokenA)},
	} {
		r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
		r.Header["Cookie"] = lines
		if _, ok, err := of(r, recipientA); !ok || err != nil {
			t.Errorf("%s：ok=%v err=%v", name, ok, err)
		}
	}
}
