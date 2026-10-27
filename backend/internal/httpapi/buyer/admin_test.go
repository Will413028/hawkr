package buyer

import (
	"context"
	"errors"
	"math"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	adminpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/buyer/v1alpha1"
	model "github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

type fakeAdmin struct {
	stats  model.AdminStats
	plans  []model.Plan
	input  model.UpdatePlanInput
	writes int
}

func (f *fakeAdmin) StatsForAdmin(context.Context) (model.AdminStats, error) { return f.stats, nil }
func (f *fakeAdmin) ListPlans(context.Context) ([]model.Plan, error)         { return f.plans, nil }
func (f *fakeAdmin) Plan(_ context.Context, _ string) (model.Plan, error)    { return f.plans[0], nil }
func (f *fakeAdmin) UpdatePlan(_ context.Context, input model.UpdatePlanInput) (model.Plan, error) {
	f.input = input
	f.writes++
	return f.plans[0], nil
}

func TestAdminStatsOverflowAndInt64PlanPrice(t *testing.T) {
	f := &fakeAdmin{stats: model.AdminStats{Total: math.MaxInt32 + 1}, plans: []model.Plan{{PlanCode: "basic", DisplayName: "基本", Version: 3, MonthlyPriceTWD: 100}}}
	h := NewAdmin(f)
	if _, err := h.GetBuyerStats(context.Background(), connect.NewRequest(&adminpb.GetBuyerStatsRequest{})); err == nil {
		t.Fatal("int32 overflow accepted")
	}
	_, err := h.UpdateMembershipPlan(context.Background(), connect.NewRequest(&adminpb.UpdateMembershipPlanRequest{MembershipPlan: &adminpb.MembershipPlan{PlanCode: "basic", Etag: "3", MonthlyPrice: math.MaxInt32 + 1}, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"monthly_price"}}}))
	if err != nil || f.writes != 1 || f.input.MonthlyPriceTWD == nil || *f.input.MonthlyPriceTWD != math.MaxInt32+1 {
		t.Fatalf("int64 price update: input=%+v err=%v", f.input, err)
	}
	_, err = h.UpdateMembershipPlan(context.Background(), connect.NewRequest(&adminpb.UpdateMembershipPlanRequest{MembershipPlan: &adminpb.MembershipPlan{PlanCode: "basic", Etag: "3", DisplayName: "竄改", MonthlyPrice: 200}, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"display_name", "monthly_price"}}}))
	if err != nil || f.input.MonthlyPriceTWD == nil || *f.input.MonthlyPriceTWD != 200 || f.writes != 2 {
		t.Fatalf("valid plan update: input=%+v err=%v", f.input, err)
	}
}

// plan_code 空白與 membership_plan 缺少一樣，是指向欄位的 invalid_argument，不是沒有欄位的錯誤。
func TestUpdateMembershipPlanEmptyPlanCodeNamesTheField(t *testing.T) {
	f := &fakeAdmin{plans: []model.Plan{{PlanCode: "basic"}}}
	_, err := NewAdmin(f).UpdateMembershipPlan(context.Background(), connect.NewRequest(&adminpb.UpdateMembershipPlanRequest{
		MembershipPlan: &adminpb.MembershipPlan{},
	}))
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Kind() != apperr.KindInvalidInput || len(ae.FieldViolations()) != 1 || ae.FieldViolations()[0].Field != "membership_plan.plan_code" {
		t.Fatalf("err = %v，預期指向 membership_plan.plan_code 的 invalid_input", err)
	}
	if f.writes != 0 {
		t.Fatalf("writes = %d", f.writes)
	}
}
