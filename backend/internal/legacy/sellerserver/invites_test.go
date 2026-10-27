package sellerserver

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"uuid"

	"connectrpc.com/connect"
	"google.golang.org/genproto/googleapis/rpc/errdetails"

	pb "github.com/Will413028/hawkr/backend/internal/legacy/gen/seller"
	"github.com/Will413028/hawkr/backend/internal/legacy/gen/seller/sellerconnect"
	"github.com/Will413028/hawkr/backend/internal/modules/agent"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

type fakeInvites struct {
	ensureIn    agent.EnsureInviteInput
	ensureOut   agent.IssuedInvite
	ensureErr   error
	markID      uuid.UUID
	markSecret  string
	markErr     error
	markCalled  bool
	ensureCalls int
}

func (f *fakeInvites) EnsureInvite(_ context.Context, in agent.EnsureInviteInput) (agent.IssuedInvite, error) {
	f.ensureCalls++
	f.ensureIn = in
	return f.ensureOut, f.ensureErr
}

func (f *fakeInvites) MarkInviteDelivered(_ context.Context, id uuid.UUID, secret string) error {
	f.markCalled = true
	f.markID, f.markSecret = id, secret
	return f.markErr
}

func newInviteServer(t *testing.T, inv Invites) *Server {
	t.Helper()
	return &Server{invites: inv}
}

func reasonOf(t *testing.T, err error) (string, string) {
	t.Helper()
	var ce *connect.Error
	if !errors.As(err, &ce) {
		t.Fatalf("不是 connect 錯誤：%v", err)
	}
	for _, d := range ce.Details() {
		v, verr := d.Value()
		if verr != nil {
			t.Fatal(verr)
		}
		if info, ok := v.(*errdetails.ErrorInfo); ok {
			return info.GetReason(), info.GetDomain()
		}
	}
	return "", ""
}

func codeOf(t *testing.T, err error) connect.Code {
	t.Helper()
	if err == nil {
		t.Fatal("預期錯誤")
	}
	return connect.CodeOf(err)
}

// 每個 agent 的結果對應各自的狀態碼與 reason；reason 一律是 proto 列舉的名稱，不是另寫的字串。
func TestInviteErrorMapping(t *testing.T) {
	const prefix = "VENDOR_PORTAL_INVITE_ERROR_REASON_"
	for _, tc := range []struct {
		name       string
		err        error
		code       connect.Code
		wantReason string
	}{
		{"已撤銷", agent.ErrInviteRevoked, connect.CodeFailedPrecondition, prefix + "INVITE_REVOKED"},
		{"業者不是進行中", agent.ErrProspectNotActive, connect.CodeFailedPrecondition, prefix + "PROSPECT_NOT_ACTIVE"},
		{"沒有可寄的 email", agent.ErrNoDeliverableEmail, connect.CodeFailedPrecondition, prefix + "NO_DELIVERABLE_EMAIL"},
		{"secret 過時", agent.ErrInviteSecretSuperseded, connect.CodeAborted, prefix + "INVITE_SECRET_SUPERSEDED"},
		{"已寄出", agent.ErrInviteAlreadyDelivered, connect.CodeAlreadyExists, ""},
		{"輸入錯誤", apperr.InvalidInput("業者與邀請不符"), connect.CodeInvalidArgument, ""},
		{"其他錯誤", errors.New("db down"), connect.CodeInternal, ""},
		{"Ensure 的業者不存在", apperr.NotFound("業者不存在"), connect.CodeNotFound, ""},
		{"Mark 的邀請不存在", apperr.NotFound("邀請不存在"), connect.CodeNotFound, ""},
		{"Unavailable", apperr.Unavailable("暫時不可用"), connect.CodeUnavailable, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := connectError(tc.err)
			if codeOf(t, got) != tc.code {
				t.Fatalf("code = %v，預期 %v", connect.CodeOf(got), tc.code)
			}
			reason, domain := reasonOf(t, got)
			if reason != tc.wantReason {
				t.Fatalf("reason = %q，預期 %q", reason, tc.wantReason)
			}
			if tc.wantReason != "" && domain != "hawkr" {
				t.Fatalf("domain = %q", domain)
			}
		})
	}
}

// 四個 reason 兩兩不同，且都不是 UNSPECIFIED 或空字串。
// 每個 apperr 分類都有狀態碼，沒有分類的錯誤是 Internal。
func TestKindCodes(t *testing.T) {
	for _, tc := range []struct {
		kind       apperr.Kind
		err        error
		want       connect.Code
		wantReason string
	}{
		{apperr.KindInternal, errors.New("x"), connect.CodeInternal, ""},
		{apperr.KindInvalidInput, apperr.InvalidInput("x"), connect.CodeInvalidArgument, ""},
		{apperr.KindNotFound, apperr.NotFound("x"), connect.CodeNotFound, ""},
		{apperr.KindVersionConflict, apperr.VersionConflict("x"), connect.CodeAborted, ""},
		{apperr.KindAlreadyExists, apperr.AlreadyExists("x"), connect.CodeAlreadyExists, ""},
		{apperr.KindFailedPrecondition, apperr.FailedPrecondition("x"), connect.CodeFailedPrecondition, ""},
		{apperr.KindUnauthenticated, apperr.Unauthenticated("x"), connect.CodeUnauthenticated, ""},
		{apperr.KindPermissionDenied, apperr.PermissionDenied("x"), connect.CodePermissionDenied, ""},
		{apperr.KindUnavailable, apperr.Unavailable("x"), connect.CodeUnavailable, ""},
		{apperr.KindResourceExhausted, apperr.ResourceExhausted(0, "x"), connect.CodeResourceExhausted, ""},
	} {
		if got := apperr.KindOf(tc.err); got != tc.kind {
			t.Errorf("%v：建構函式的分類 = %v", tc.kind, got)
		}
		err := connectError(tc.err)
		if got := connect.CodeOf(err); got != tc.want {
			t.Errorf("%v：code = %v，預期 %v", tc.kind, got, tc.want)
		}
		if reason, _ := reasonOf(t, err); reason != tc.wantReason {
			t.Errorf("%v：reason = %q，預期 %q（舊 sellerserver 的 connectError 只送邀請的 sentinel reason，不轉 apperr.Reason）", tc.kind, reason, tc.wantReason)
		}
	}
	for err, want := range map[error]connect.Code{
		context.Canceled:                        connect.CodeCanceled,
		context.DeadlineExceeded:                connect.CodeDeadlineExceeded,
		fmt.Errorf("讀取失敗：%w", context.Canceled): connect.CodeCanceled,
	} {
		if got := connect.CodeOf(connectError(err)); got != want {
			t.Errorf("%v：code = %v，預期 %v", err, got, want)
		}
	}
	if got := connect.CodeOf(connectError(errors.New("plain"))); got != connect.CodeInternal {
		t.Errorf("非 apperr 的錯誤 code = %v", got)
	}
}

func TestInviteReasonsAreDistinctAndSpecified(t *testing.T) {
	seen := map[string]bool{}
	for _, r := range inviteReasons {
		name := r.reason.String()
		if r.reason == pb.VendorPortalInviteErrorReason_VENDOR_PORTAL_INVITE_ERROR_REASON_UNSPECIFIED || name == "" {
			t.Fatalf("%v 的 reason 無效", r.err)
		}
		if seen[name] {
			t.Fatalf("reason %q 重複", name)
		}
		seen[name] = true
	}
	if len(seen) != 4 {
		t.Fatalf("reason 數量 = %d", len(seen))
	}
}

func TestEnsureVendorPortalInviteParsesTheRequest(t *testing.T) {
	id := uuid.New()
	f := &fakeInvites{ensureOut: agent.IssuedInvite{InviteID: id, Secret: "s3cret", InvitedEmail: "v@example.com"}}
	s := newInviteServer(t, f)
	r, err := s.EnsureVendorPortalInvite(context.Background(), connect.NewRequest(&pb.EnsureVendorPortalInviteRequest{
		ProspectId: "101", RfqRecipientId: "202", ExpiresAt: "2026-10-12T12:00:00+08:00",
	}))
	if err != nil {
		t.Fatal(err)
	}
	wantAt := time.Date(2026, 10, 12, 4, 0, 0, 0, time.UTC)
	if f.ensureIn.ProspectID != 101 || f.ensureIn.RfqRecipientID != 202 || !f.ensureIn.ExpiresAt.Equal(wantAt) || f.ensureIn.ExpiresAt.Location() != time.UTC {
		t.Fatalf("input = %+v", f.ensureIn)
	}
	if r.Msg.GetInviteId() != id.String() || r.Msg.GetSecret() != "s3cret" || r.Msg.GetInvitedEmail() != "v@example.com" {
		t.Fatalf("response = %v", r.Msg)
	}
}

func TestVendorPortalInviteBadInputIsInvalidArgument(t *testing.T) {
	f := &fakeInvites{}
	s := newInviteServer(t, f)
	ctx := context.Background()
	ok := &pb.EnsureVendorPortalInviteRequest{ProspectId: "1", RfqRecipientId: "2", ExpiresAt: "2026-10-12T12:00:00Z"}
	for name, mutate := range map[string]func(r *pb.EnsureVendorPortalInviteRequest){
		"prospect_id":      func(r *pb.EnsureVendorPortalInviteRequest) { r.ProspectId = "abc" },
		"prospect 空":       func(r *pb.EnsureVendorPortalInviteRequest) { r.ProspectId = "" },
		"rfq_recipient_id": func(r *pb.EnsureVendorPortalInviteRequest) { r.RfqRecipientId = "1.5" },
		"expires_at":       func(r *pb.EnsureVendorPortalInviteRequest) { r.ExpiresAt = "tomorrow" },
	} {
		req := &pb.EnsureVendorPortalInviteRequest{ProspectId: ok.ProspectId, RfqRecipientId: ok.RfqRecipientId, ExpiresAt: ok.ExpiresAt}
		mutate(req)
		_, err := s.EnsureVendorPortalInvite(ctx, connect.NewRequest(req))
		if codeOf(t, err) != connect.CodeInvalidArgument {
			t.Errorf("%s：code = %v", name, connect.CodeOf(err))
		}
	}
	if f.ensureCalls != 0 {
		t.Fatal("輸入錯誤不得呼叫 agent")
	}
	for name, req := range map[string]*pb.MarkVendorPortalInviteSecretDeliveredRequest{
		"invite_id 不是 UUID": {InviteId: "12345", Secret: "s"},
		"invite_id 空":       {InviteId: "", Secret: "s"},
		"secret 空":          {InviteId: uuid.New().String(), Secret: ""},
	} {
		_, err := s.MarkVendorPortalInviteSecretDelivered(ctx, connect.NewRequest(req))
		if codeOf(t, err) != connect.CodeInvalidArgument {
			t.Errorf("%s：code = %v", name, connect.CodeOf(err))
		}
	}
	if f.markCalled {
		t.Fatal("輸入錯誤不得呼叫 agent")
	}
}

func TestMarkVendorPortalInvitePassesTheSecretThrough(t *testing.T) {
	id := uuid.New()
	f := &fakeInvites{}
	s := newInviteServer(t, f)
	r, err := s.MarkVendorPortalInviteSecretDelivered(context.Background(), connect.NewRequest(&pb.MarkVendorPortalInviteSecretDeliveredRequest{InviteId: id.String(), Secret: "the-secret"}))
	if err != nil || r.Msg.GetCode() != 0 {
		t.Fatalf("%v %v", r, err)
	}
	if f.markID != id || f.markSecret != "the-secret" {
		t.Fatalf("mark = %v %q", f.markID, f.markSecret)
	}
}

// gRPC 協定走線：client 讀得到 ErrorInfo（grpc-status-details-bin），代表細節與 grpc-go 的 status 相容。
func TestInviteErrorsCarryErrorInfoOverTheGRPCWire(t *testing.T) {
	f := &fakeInvites{}
	mux := http.NewServeMux()
	mux.Handle(sellerconnect.NewSellerServiceHandler(newInviteServer(t, f)))
	srv := httptest.NewUnstartedServer(mux)
	srv.EnableHTTP2 = true
	srv.StartTLS()
	defer srv.Close()
	client := sellerconnect.NewSellerServiceClient(srv.Client(), srv.URL, connect.WithGRPC())
	ctx := context.Background()

	f.ensureErr = agent.ErrProspectNotActive
	_, err := client.EnsureVendorPortalInvite(ctx, connect.NewRequest(&pb.EnsureVendorPortalInviteRequest{ProspectId: "1", RfqRecipientId: "2", ExpiresAt: "2026-10-12T12:00:00Z"}))
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("code = %v (%v)", connect.CodeOf(err), err)
	}
	if reason, domain := reasonOf(t, err); reason != pb.VendorPortalInviteErrorReason_VENDOR_PORTAL_INVITE_ERROR_REASON_PROSPECT_NOT_ACTIVE.String() || domain != "hawkr" {
		t.Fatalf("reason = %q domain = %q", reason, domain)
	}

	f.markErr = agent.ErrInviteSecretSuperseded
	_, err = client.MarkVendorPortalInviteSecretDelivered(ctx, connect.NewRequest(&pb.MarkVendorPortalInviteSecretDeliveredRequest{InviteId: uuid.New().String(), Secret: "s"}))
	if connect.CodeOf(err) != connect.CodeAborted {
		t.Fatalf("code = %v (%v)", connect.CodeOf(err), err)
	}
	if reason, _ := reasonOf(t, err); reason != pb.VendorPortalInviteErrorReason_VENDOR_PORTAL_INVITE_ERROR_REASON_INVITE_SECRET_SUPERSEDED.String() {
		t.Fatalf("reason = %q", reason)
	}
}
