package notification

import (
	"context"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/coupon"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/registry"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// Validity 是型別在送出前重查來源事實要問的地方（D27）。
type Validity = registry.Validity

type validity struct{ coupons *coupon.Module }

// NewValidity 把 coupon 組成送出前的重查來源：券的狀態與活動是否過期都以 coupon 的 GetVoucher 為準；必填，缺了是組裝錯誤，建構時就 panic。
func NewValidity(c *coupon.Module) Validity {
	if c == nil {
		panic("notification.NewValidity：coupon 必填")
	}
	return validity{coupons: c}
}

func (v validity) VoucherAvailable(ctx context.Context, buyerID int64, voucherID uuid.UUID) (bool, error) {
	view, err := v.coupons.GetVoucher(ctx, buyerID, voucherID)
	if apperr.KindOf(err) == apperr.KindNotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return view.Voucher.Status == coupon.VoucherAvailable && !view.Expired, nil
}
