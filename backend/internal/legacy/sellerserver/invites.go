package sellerserver

import (
	"context"
	"errors"
	"strconv"
	"time"

	"uuid"

	"connectrpc.com/connect"

	pb "github.com/Will413028/hawkr/backend/internal/legacy/gen/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/agent"
)

// Invites 是供應商入口邀請的來源（agent 模組擁有邀請）：Ensure 建立或重取邀請並回明文 secret，Mark 記下那把 secret 的信已寄出。
type Invites interface {
	EnsureInvite(ctx context.Context, in agent.EnsureInviteInput) (agent.IssuedInvite, error)
	MarkInviteDelivered(ctx context.Context, inviteID uuid.UUID, secret string) error
}

var _ Invites = (*agent.Module)(nil)

// EnsureVendorPortalInvite 的 rfq_recipient_id 就是 notification 事件的 inviteId（ai-service 的收件人），對到
// EnsureInviteInput.RfqRecipientID；回應的 invite_id 是邀請的 UUID 字串。
func (s *Server) EnsureVendorPortalInvite(ctx context.Context, req *connect.Request[pb.EnsureVendorPortalInviteRequest]) (*connect.Response[pb.EnsureVendorPortalInviteResponse], error) {
	prospectID, err := strconv.ParseInt(req.Msg.GetProspectId(), 10, 64)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("prospect_id 不是十進位整數"))
	}
	recipientID, err := strconv.ParseInt(req.Msg.GetRfqRecipientId(), 10, 64)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("rfq_recipient_id 不是十進位整數"))
	}
	expiresAt, err := time.Parse(time.RFC3339, req.Msg.GetExpiresAt())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("expires_at 不是 RFC3339"))
	}
	issued, err := s.invites.EnsureInvite(ctx, agent.EnsureInviteInput{ProspectID: prospectID, RfqRecipientID: recipientID, ExpiresAt: expiresAt.UTC()})
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(&pb.EnsureVendorPortalInviteResponse{
		InviteId: issued.InviteID.String(), Secret: issued.Secret, InvitedEmail: issued.InvitedEmail,
	}), nil
}

// MarkVendorPortalInviteSecretDelivered 帶寄出的那一把 secret：它已不是目前的 secret 時回 Aborted 與 INVITE_SECRET_SUPERSEDED。
func (s *Server) MarkVendorPortalInviteSecretDelivered(ctx context.Context, req *connect.Request[pb.MarkVendorPortalInviteSecretDeliveredRequest]) (*connect.Response[pb.MarkVendorPortalInviteSecretDeliveredResponse], error) {
	inviteID, err := uuid.Parse(req.Msg.GetInviteId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invite_id 不是 UUID"))
	}
	if req.Msg.GetSecret() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("secret 不得為空"))
	}
	if err := s.invites.MarkInviteDelivered(ctx, inviteID, req.Msg.GetSecret()); err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(&pb.MarkVendorPortalInviteSecretDeliveredResponse{Code: 0, Message: "ok"}), nil
}
