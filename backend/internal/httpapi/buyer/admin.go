package buyer

import (
	"context"
	"math"

	"connectrpc.com/connect"

	adminpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/buyer/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/buyer/v1alpha1/buyerv1alpha1connect"
	model "github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

type adminModule interface {
	StatsForAdmin(context.Context) (model.AdminStats, error)
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
