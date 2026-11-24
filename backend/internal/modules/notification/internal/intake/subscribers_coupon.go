package intake

import (
	"context"

	"github.com/Will413028/hawkr/backend/internal/modules/coupon"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/registry"
)

// OnVoucherExpiring 訂閱 coupon.VoucherExpiring：每張券一個意圖，冪等鍵是 voucher_id；活動名稱原文不進參數。
func (s *Service) OnVoucherExpiring(ctx context.Context, e coupon.VoucherExpiring) error {
	return s.Accept(ctx, "voucher_expiring", registry.VoucherExpiringParams{
		EventID: e.EventID, OccurredAt: e.OccurredAt, BuyerID: e.BuyerID, VoucherID: e.VoucherID, CampaignID: e.CampaignID, ExpiresAt: e.ExpiresAt,
	})
}
