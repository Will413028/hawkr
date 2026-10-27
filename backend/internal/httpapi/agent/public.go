// Package agent 是 agent 模組的 Connect handler：前台的 hawkr.agent.v1alpha1（vendor portal）。這裡只做傳輸：cookie、
// Google token 的驗證接線、nonce 與欄位轉換，邀請、session 與驗證紀錄的規則在 internal/modules/agent。
package agent

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/agent/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/agent/v1alpha1/agentv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	model "github.com/Will413028/hawkr/backend/internal/modules/agent"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// Vendors 是邀請與 session 的來源（*agent.Module）。
type Vendors interface {
	Sessions
	PreviewInvite(ctx context.Context, secret string) (model.InvitePreview, error)
	ExchangeInvite(ctx context.Context, secret string) (model.IssuedSession, error)
	VerifySession(ctx context.Context, token string, method model.VerificationMethod, verifiedEmail string) (model.SessionInfo, error)
}

// Google 驗證 Google 的 ID token（*identity.Module）。
type Google interface {
	VerifyGoogle(ctx context.Context, idToken, wantNonce string) (identity.Identity, error)
}

// Public 實作 VendorPortalPublicService。
type Public struct {
	vendors Vendors
	now     func() time.Time
}

var _ agentv1alpha1connect.VendorPortalPublicServiceHandler = (*Public)(nil)

// NewPublic 建立 handler；now 是 nil 時用 time.Now。
func NewPublic(vendors Vendors, now func() time.Time) *Public {
	if now == nil {
		now = time.Now
	}
	return &Public{vendors: vendors, now: now}
}

var secretPaths = rpc.FieldPaths{"Secret": "secret"}

func (h *Public) PreviewInvite(ctx context.Context, req *connect.Request[pb.PreviewInviteRequest]) (*connect.Response[pb.PreviewInviteResponse], error) {
	secret := req.Msg.GetSecret()
	if secret == "" {
		return nil, rpc.WithFieldPaths(apperr.InvalidInputOn([]string{"Secret"}, "缺少邀請 secret"), secretPaths)
	}
	p, err := h.vendors.PreviewInvite(ctx, secret)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.PreviewInviteResponse{MaskedEmail: p.MaskedEmail, ExpireTime: timestamppb.New(p.ExpiresAt)}), nil
}

// ExchangeInvite 先以 PreviewInvite 解出 secret 對應的邀請（不可用與兌換同一個 not_found），請求帶著這張邀請的 cookie 而且它是這張邀請
// 的有效 session 就沿用：不發新的 session，也不重設 cookie（它的 Max-Age 已經是同一個絕對期限）；沒有或無效時才兌換。
// 另一張邀請的 cookie 不讀、不動。請求帶的 vendor cookie 超過上限時清掉多出的（rpc.ExcessVendorPortalCookies），不含這張邀請的。
func (h *Public) ExchangeInvite(ctx context.Context, req *connect.Request[pb.ExchangeInviteRequest]) (*connect.Response[pb.ExchangeInviteResponse], error) {
	secret := req.Msg.GetSecret()
	if secret == "" {
		return nil, rpc.WithFieldPaths(apperr.InvalidInputOn([]string{"Secret"}, "缺少邀請 secret"), secretPaths)
	}
	invite, err := h.vendors.PreviewInvite(ctx, secret)
	if err != nil {
		return nil, err
	}
	resp := connect.NewResponse(&pb.ExchangeInviteResponse{RfqRecipientId: strconv.FormatInt(invite.RfqRecipientID, 10)})
	reused, err := h.reusable(ctx, req.Header(), invite)
	if err != nil {
		return nil, err
	}
	if reused != nil {
		resp.Msg.Verification = verificationToProto(reused.Info.Verification)
		resp.Msg.GoogleNonce = GoogleNonce(reused.Token)
	} else {
		s, err := h.vendors.ExchangeInvite(ctx, secret)
		if err != nil {
			return nil, err
		}
		resp.Msg.GoogleNonce = GoogleNonce(s.Token)
		resp.Header().Add("Set-Cookie", rpc.NewVendorPortalCookie(s.RfqRecipientID, s.Token, s.AbsoluteExpiresAt, h.now()).String())
	}
	for _, name := range rpc.ExcessVendorPortalCookies(req.Header(), invite.RfqRecipientID) {
		resp.Header().Add("Set-Cookie", rpc.ExpiredVendorPortalCookie(name).String())
	}
	return resp, nil
}

// reusable 回傳請求帶的、屬於 invite 的有效 session；沒有或不可用回 nil。
func (h *Public) reusable(ctx context.Context, header http.Header, invite model.InvitePreview) (*Session, error) {
	token := rpc.VendorPortalCookieToken(header, invite.RfqRecipientID)
	if token == "" {
		return nil, nil //nolint:nilnil // nil 表示沒有可沿用的 session
	}
	info, err := h.vendors.ValidateSession(ctx, token)
	if apperr.KindOf(err) == apperr.KindUnauthenticated {
		return nil, nil //nolint:nilnil // 無效的 cookie 之後會被新的 session 蓋掉
	}
	if err != nil {
		return nil, err
	}
	if info.InviteID != invite.InviteID {
		return nil, nil //nolint:nilnil // cookie 的 session 不是這張邀請的
	}
	return &Session{Token: token, Info: info}, nil
}

func verificationToProto(v *model.Verification) *pb.SessionVerification {
	if v == nil {
		return nil
	}
	return &pb.SessionVerification{Method: rpc.EnumToProto[pb.SessionVerificationMethod](v.Method), VerifyTime: timestamppb.New(v.At)}
}
