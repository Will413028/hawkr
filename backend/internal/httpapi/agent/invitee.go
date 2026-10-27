package agent

import (
	"context"
	"errors"
	"log/slog"
	"strconv"

	"connectrpc.com/connect"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/agent/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/agent/v1alpha1/agentv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	model "github.com/Will413028/hawkr/backend/internal/modules/agent"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// Invitee 實作 VendorPortalInviteeService：session 由 NewInterceptor 驗證後放進 ctx。
type Invitee struct {
	vendors Vendors
	google  Google
	log     *slog.Logger
}

var _ agentv1alpha1connect.VendorPortalInviteeServiceHandler = (*Invitee)(nil)

// NewInvitee 建立 handler；log 是 nil 時不記錄。
func NewInvitee(vendors Vendors, google Google, log *slog.Logger) *Invitee {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &Invitee{vendors: vendors, google: google, log: log}
}

var tokenPaths = rpc.FieldPaths{"GoogleIdToken": "google_id_token"}

func (h *Invitee) GetSession(ctx context.Context, _ *connect.Request[pb.GetSessionRequest]) (*connect.Response[pb.GetSessionResponse], error) {
	sess, err := sessionFrom(ctx)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.GetSessionResponse{Verification: verificationToProto(sess.Info.Verification), GoogleNonce: GoogleNonce(sess.Token)}), nil
}

// errVerifyDenied 是 Google token 無效、nonce 不符、token 太舊、信箱未驗證、信箱不符與 session 不接受這次驗證共用的錯誤：同一個
// code 與同一段訊息，呼叫的人分不出是哪一種。
func errVerifyDenied() error { return apperr.PermissionDenied("無法驗證這個 session") }

func (h *Invitee) VerifySession(ctx context.Context, req *connect.Request[pb.VerifySessionRequest]) (*connect.Response[pb.VerifySessionResponse], error) {
	sess, err := sessionFrom(ctx)
	if err != nil {
		return nil, err
	}
	idToken := req.Msg.GetGoogleIdToken()
	if idToken == "" {
		return nil, rpc.WithFieldPaths(apperr.InvalidInputOn([]string{"GoogleIdToken"}, "缺少 Google ID token"), tokenPaths)
	}
	id, err := h.google.VerifyGoogle(ctx, idToken, GoogleNonce(sess.Token))
	if apperr.KindOf(err) == apperr.KindUnauthenticated {
		return nil, errVerifyDenied()
	}
	if err != nil {
		return nil, err
	}
	if !id.EmailVerified {
		return nil, errVerifyDenied()
	}
	info, err := h.vendors.VerifySession(ctx, sess.Token, model.VerificationGoogle, id.Email)
	if apperr.KindOf(err) == apperr.KindPermissionDenied {
		return nil, errVerifyDenied()
	}
	if err != nil {
		return nil, err
	}
	if info.Verification == nil {
		h.log.ErrorContext(ctx, "httpapi/agent：驗證成功的 session 沒有驗證紀錄", "session_id", info.SessionID)
		return nil, errors.New("httpapi/agent：驗證成功的 session 沒有驗證紀錄")
	}
	return connect.NewResponse(&pb.VerifySessionResponse{
		RfqRecipientId: strconv.FormatInt(info.RfqRecipientID, 10),
		Verification:   verificationToProto(info.Verification),
	}), nil
}
