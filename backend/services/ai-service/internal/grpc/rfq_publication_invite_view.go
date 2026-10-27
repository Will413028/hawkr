package grpc

import (
	"context"
	"errors"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	aipb "github.com/Will413028/hawkr/go-pkg/proto/ai"
	"google.golang.org/grpc/codes"
)

// InviteSellerViewRepository 是 GetRfqPublicationInviteSellerView 依賴的 repository
// 介面，供 internal/procurement/module.go 以 *repository.Repo 綁定。
type InviteSellerViewRepository interface {
	GetPublicationInviteSellerView(ctx context.Context, inviteID int64) (repository.InviteSellerView, error)
}

type inviteSellerViewHandler struct {
	repo InviteSellerViewRepository
}

func newInviteSellerViewHandler(repo InviteSellerViewRepository) *inviteSellerViewHandler {
	return &inviteSellerViewHandler{repo: repo}
}

func (h *inviteSellerViewHandler) GetRfqPublicationInviteSellerView(ctx context.Context, req *aipb.GetRfqPublicationInviteSellerViewRequest) (*aipb.RfqPublicationInviteSellerViewResponse, error) {
	inviteID, err := parseInt64(req.GetInviteId())
	if err != nil {
		return nil, procurementStatus(codes.InvalidArgument, "", "invalid inviteId")
	}

	view, err := h.repo.GetPublicationInviteSellerView(ctx, inviteID)
	if err != nil {
		if errors.Is(err, repository.ErrInviteNotFound) {
			return nil, procurementStatus(codes.NotFound, "", "invite not found")
		}
		return nil, procurementStatus(codes.Internal, "", "internal error")
	}

	return &aipb.RfqPublicationInviteSellerViewResponse{
		QualityKnowledgeState: view.QualityKnowledgeState,
		Item:                  view.Item,
		ItemLabel:             view.ItemLabel,
		Quantity:              view.Quantity,
		Unit:                  view.Unit,
		Packaging:             view.Packaging,
		DeliveryCity:          view.DeliveryCity,
		DeliveryDate:          view.DeliveryDate,
		ResponseDeadline:      view.ResponseDeadline,
		BuyerNote:             view.BuyerNote,
		PublicationVersion:    view.PublicationVersion,
		ReferencePriceRange:   toSellerFacingReferenceRangeProto(view.ReferencePriceRange),
		BuyerRequestedRange:   toSellerFacingRequestedRangeProto(view.BuyerRequestedRange),
	}, nil
}

func toSellerFacingReferenceRangeProto(r *domain.SellerFacingReferenceRange) *aipb.SellerFacingReferenceRange {
	if r == nil {
		return nil
	}
	return &aipb.SellerFacingReferenceRange{
		LowerBoundAmount: r.LowerBoundAmount,
		UpperBoundAmount: r.UpperBoundAmount,
		BasisQuantity:    r.BasisQuantity,
		BasisUnit:        r.BasisUnit,
	}
}

func toSellerFacingRequestedRangeProto(r *domain.SellerFacingRequestedRange) *aipb.SellerFacingRequestedRange {
	if r == nil {
		return nil
	}
	return &aipb.SellerFacingRequestedRange{
		Currency:         r.Currency,
		PriceComponent:   r.PriceComponent,
		BasisQuantity:    r.BasisQuantity,
		BasisUnit:        r.BasisUnit,
		LowerBoundAmount: r.LowerBoundAmount,
		UpperBoundAmount: r.UpperBoundAmount,
	}
}
