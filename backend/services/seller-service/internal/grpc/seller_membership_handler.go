package grpc

import (
	"context"
	"strconv"

	pb "github.com/Will413028/hawkr/go-pkg/proto/seller"

	"github.com/Will413028/hawkr/seller-service/internal/usecase"
)

// Admin 賣家會員分級方案 gRPC handlers（子系統 1）。同 suspension_handler.go
// 「不回 grpc.Status，回 proto code field」風格。時間欄位依 §6 走 ISO 8601 UTC 字串
// （formatTime，mapper.go），非 ms 數字。

// toSellerMembershipPlanData converts the usecase read model to its proto wire
// shape. tier_rank / monthly_order_cap cross the boundary as strings (§4 —
// numeric/ID/count fields are string on the wire).
func toSellerMembershipPlanData(v usecase.SellerMembershipView) *pb.SellerMembershipPlanData {
	return &pb.SellerMembershipPlanData{
		StoreId:         formatID(v.StoreID),
		PlanCode:        v.PlanCode,
		DisplayName:     v.DisplayName,
		TierRank:        strconv.Itoa(v.TierRank),
		MonthlyOrderCap: strconv.Itoa(v.MonthlyOrderCap),
		EffectiveAt:     formatTime(v.EffectiveAt),
	}
}

func (s *SellerServer) AssignSellerMembershipPlan(ctx context.Context, req *pb.AssignSellerMembershipPlanRequest) (*pb.SellerMembershipPlanResponse, error) {
	view, err := s.membershipUC.AssignMembership(ctx, usecase.AssignMembershipRequest{
		StoreID:            parseInt64(req.GetStoreId()),
		PlanCode:           req.GetPlanCode(),
		Reason:             req.GetReason(),
		GrantedByAccountID: parseInt64(req.GetGrantedByAccountId()),
		RequestID:          req.GetRequestId(),
	})
	if err != nil {
		c, m := errToCode(err)
		return &pb.SellerMembershipPlanResponse{Code: c, Message: m}, nil
	}
	return &pb.SellerMembershipPlanResponse{Data: toSellerMembershipPlanData(view)}, nil
}

func (s *SellerServer) GetSellerMembershipPlan(ctx context.Context, req *pb.GetSellerMembershipPlanRequest) (*pb.SellerMembershipPlanResponse, error) {
	view, err := s.membershipUC.GetCurrentPlan(ctx, parseInt64(req.GetStoreId()))
	if err != nil {
		c, m := errToCode(err)
		return &pb.SellerMembershipPlanResponse{Code: c, Message: m}, nil
	}
	return &pb.SellerMembershipPlanResponse{Data: toSellerMembershipPlanData(view)}, nil
}

// toSellerMembershipPlanCatalogEntry converts the catalog read model to its
// proto wire shape. tier_rank/monthly_order_cap/monthly_price_twd cross the
// boundary as strings (§4); tier_rank is present for admin display only —
// there is no write path for it here (see UpdateSellerMembershipPlan).
func toSellerMembershipPlanCatalogEntry(p usecase.SellerMembershipPlanCatalogEntry) *pb.SellerMembershipPlanCatalogEntry {
	return &pb.SellerMembershipPlanCatalogEntry{
		PlanCode:        p.PlanCode,
		DisplayName:     p.DisplayName,
		TierRank:        strconv.Itoa(p.TierRank),
		MonthlyOrderCap: strconv.Itoa(p.MonthlyOrderCap),
		MonthlyPriceTwd: strconv.Itoa(p.MonthlyPriceTWD),
		IsPurchasable:   p.IsPurchasable,
		CreatedAt:       formatTime(p.CreatedAt),
		UpdatedAt:       formatTime(p.UpdatedAt),
	}
}

func (s *SellerServer) ListSellerMembershipPlans(ctx context.Context, _ *pb.ListSellerMembershipPlansRequest) (*pb.ListSellerMembershipPlansResponse, error) {
	plans, err := s.membershipUC.ListMembershipPlanCatalog(ctx)
	if err != nil {
		c, m := errToCode(err)
		return &pb.ListSellerMembershipPlansResponse{Code: c, Message: m}, nil
	}
	out := make([]*pb.SellerMembershipPlanCatalogEntry, len(plans))
	for i, p := range plans {
		out[i] = toSellerMembershipPlanCatalogEntry(p)
	}
	return &pb.ListSellerMembershipPlansResponse{Plans: out}, nil
}

func (s *SellerServer) UpdateSellerMembershipPlan(ctx context.Context, req *pb.UpdateSellerMembershipPlanRequest) (*pb.UpdateSellerMembershipPlanResponse, error) {
	in := usecase.UpdateMembershipPlanCatalogRequest{PlanCode: req.GetPlanCode()}
	if req.GetHasMonthlyOrderCap() {
		v := int(parseInt64(req.GetMonthlyOrderCap()))
		in.MonthlyOrderCap = &v
	}
	if req.GetHasMonthlyPriceTwd() {
		v := int(parseInt64(req.GetMonthlyPriceTwd()))
		in.MonthlyPriceTWD = &v
	}
	if req.GetHasIsPurchasable() {
		v := req.GetIsPurchasable()
		in.IsPurchasable = &v
	}

	_, err := s.membershipUC.UpdateMembershipPlanCatalog(ctx, in)
	if err != nil {
		c, m := errToCode(err)
		return &pb.UpdateSellerMembershipPlanResponse{Code: c, Message: m}, nil
	}
	return &pb.UpdateSellerMembershipPlanResponse{}, nil
}
