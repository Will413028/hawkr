package buyer

import (
	"context"
	"math"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	adminpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/buyer/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/buyer/v1alpha1/buyerv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	model "github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

type adminModule interface {
	StatsForAdmin(context.Context) (model.AdminStats, error)
	ListPlans(context.Context) ([]model.Plan, error)
	Plan(context.Context, string) (model.Plan, error)
	UpdatePlan(context.Context, model.UpdatePlanInput) (model.Plan, error)
}

// Admin 是 BuyerAdminService 的 handler。
type Admin struct{ m adminModule }

var _ buyerv1alpha1connect.BuyerAdminServiceHandler = (*Admin)(nil)

// NewAdmin 建立後台 handler。
func NewAdmin(m adminModule) *Admin { return &Admin{m: m} }

func (h *Admin) GetBuyerStats(ctx context.Context, _ *connect.Request[adminpb.GetBuyerStatsRequest]) (*connect.Response[adminpb.GetBuyerStatsResponse], error) {
	stats, err := h.m.StatsForAdmin(ctx)
	if err != nil {
		return nil, err
	}
	if stats.Total > math.MaxInt32 || stats.Total < 0 {
		return nil, apperr.FailedPrecondition("買家總數超過對外契約的範圍")
	}
	return connect.NewResponse(&adminpb.GetBuyerStatsResponse{TotalBuyers: int32(stats.Total)}), nil
}

func toPlan(p model.Plan) *adminpb.MembershipPlan {
	return &adminpb.MembershipPlan{PlanCode: p.PlanCode, DisplayName: p.DisplayName, MonthlyAnswerLimit: p.MonthlyAnswerLimit, MonthlyPrice: p.MonthlyPriceTWD, Purchasable: p.IsPurchasable, CreateTime: timestamppb.New(p.CreatedAt), UpdateTime: timestamppb.New(p.UpdatedAt), Etag: rpc.Etag(p.Version)}
}

func (h *Admin) ListMembershipPlans(ctx context.Context, _ *connect.Request[adminpb.ListMembershipPlansRequest]) (*connect.Response[adminpb.ListMembershipPlansResponse], error) {
	plans, err := h.m.ListPlans(ctx)
	if err != nil {
		return nil, err
	}
	out := &adminpb.ListMembershipPlansResponse{MembershipPlans: make([]*adminpb.MembershipPlan, len(plans))}
	for i, p := range plans {
		out.MembershipPlans[i] = toPlan(p)
	}
	return connect.NewResponse(out), nil
}

func (h *Admin) UpdateMembershipPlan(ctx context.Context, req *connect.Request[adminpb.UpdateMembershipPlanRequest]) (*connect.Response[adminpb.UpdateMembershipPlanResponse], error) {
	msg := req.Msg.GetMembershipPlan()
	if msg == nil {
		return nil, rpc.RequiredField("membership_plan", "membership_plan 必填")
	}
	if msg.GetPlanCode() == "" {
		return nil, rpc.RequiredField("membership_plan.plan_code", "plan_code 必填")
	}
	v, err := rpc.RequiredVersionField(msg.GetEtag(), "etag")
	if err != nil {
		return nil, err
	}
	current, err := h.m.Plan(ctx, msg.GetPlanCode())
	if err != nil {
		return nil, err
	}
	merged := toPlan(current)
	if err := rpc.ApplyUpdateMask(merged, msg, req.Msg.GetUpdateMask()); err != nil {
		return nil, err
	}
	input := model.UpdatePlanInput{PlanCode: current.PlanCode, ExpectedVersion: v, MonthlyAnswerLimit: &merged.MonthlyAnswerLimit, MonthlyPriceTWD: &merged.MonthlyPrice, IsPurchasable: &merged.Purchasable}
	updated, err := h.m.UpdatePlan(ctx, input)
	if err != nil {
		return nil, rpc.WithFieldPaths(err, rpc.FieldPaths{"MonthlyAnswerLimit": "membership_plan.monthly_answer_limit", "MonthlyPriceTWD": "membership_plan.monthly_price"})
	}
	return connect.NewResponse(&adminpb.UpdateMembershipPlanResponse{MembershipPlan: toPlan(updated)}), nil
}
