package grpc

import (
	"context"
	"errors"
	"strconv"
	"time"

	pb "github.com/Will413028/hawkr/go-pkg/proto/seller"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type vendorPortalHandler struct {
	repo domain.VendorPortalRepository
	now  func() time.Time
}

func newVendorPortalHandler(repo domain.VendorPortalRepository) *vendorPortalHandler {
	return &vendorPortalHandler{repo: repo, now: time.Now}
}

func (h *vendorPortalHandler) EnsureVendorPortalInvite(
	ctx context.Context, req *pb.EnsureVendorPortalInviteRequest,
) (*pb.EnsureVendorPortalInviteResponse, error) {
	prospectID, err := strconv.ParseInt(req.GetProspectId(), 10, 64)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "prospect_id: %v", err)
	}
	recipientID, err := strconv.ParseInt(req.GetRfqRecipientId(), 10, 64)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "rfq_recipient_id: %v", err)
	}
	expiresAt, err := time.Parse(time.RFC3339, req.GetExpiresAt())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "expires_at: %v", err)
	}

	invite, secret, err := h.repo.EnsureInvite(ctx, domain.EnsureInviteInput{
		ProspectID: prospectID, RFQRecipientID: recipientID,
		ExpiresAt: expiresAt.UTC(),
	})
	if errors.Is(err, domain.ErrProspectHasNoEmail) {
		return nil, status.Error(codes.FailedPrecondition, "prospect has no public email")
	}
	if errors.Is(err, domain.ErrInviteSecretAlreadyDelivered) {
		return nil, status.Error(codes.AlreadyExists, "invite secret already delivered")
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "ensure vendor portal invite: %v", err)
	}
	return &pb.EnsureVendorPortalInviteResponse{
		InviteId:     strconv.FormatInt(invite.ID, 10),
		Secret:       secret,
		InvitedEmail: invite.Email,
	}, nil
}

func (h *vendorPortalHandler) MarkVendorPortalInviteSecretDelivered(
	ctx context.Context, req *pb.MarkVendorPortalInviteSecretDeliveredRequest,
) (*pb.MarkVendorPortalInviteSecretDeliveredResponse, error) {
	inviteID, err := strconv.ParseInt(req.GetInviteId(), 10, 64)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invite_id: %v", err)
	}
	if err := h.repo.MarkSecretDelivered(ctx, inviteID, h.now().UTC()); err != nil {
		return nil, status.Errorf(codes.Internal, "mark vendor portal invite secret delivered: %v", err)
	}
	return &pb.MarkVendorPortalInviteSecretDeliveredResponse{Code: 0, Message: "ok"}, nil
}
