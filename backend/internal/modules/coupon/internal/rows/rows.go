// Package rows 把 sqlc 的資料列轉成 model 的對外型別。
package rows

import (
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/coupon/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/coupon/internal/store"
)

// Campaign 把活動列轉成對外型別。
func Campaign(r store.CouponCouponCampaign) model.Campaign {
	c := model.Campaign{
		ID: r.ID, Title: r.Title, DiscountType: model.DiscountType(r.DiscountType), ThresholdAmount: r.ThresholdAmount,
		TargetType: model.TargetType(r.TargetType), StoreID: r.StoreID, IssuerAdminID: r.IssuerAdminID, IssuerStoreID: r.IssuerStoreID,
		StartAt: r.StartAt, EndAt: r.EndAt, TotalLimit: r.TotalLimit, IssuedCount: r.IssuedCount,
		Status: model.CampaignStatus(r.Status), EndedAt: r.EndedAt, CreatedAt: r.CreatedAt,
	}
	if r.DiscountAmount != nil {
		c.DiscountAmount = *r.DiscountAmount
	}
	if r.DiscountPercent != nil {
		c.DiscountPercent = *r.DiscountPercent
	}
	if r.MaxDiscountAmount != nil {
		c.MaxDiscountAmount = *r.MaxDiscountAmount
	}
	return c
}

// Voucher 把券列轉成對外型別。
func Voucher(r store.CouponCouponVoucher) model.Voucher {
	v := model.Voucher{
		ID: r.ID, CampaignID: r.CampaignID, BuyerID: r.BuyerID, Origin: model.Origin(r.Origin), Status: model.VoucherStatus(r.Status),
		LockedAt: r.LockedAt, RedeemedAt: r.RedeemedAt, VoidedAt: r.VoidedAt, CreatedAt: r.CreatedAt,
	}
	if r.OrderNo != nil {
		v.OrderNo = *r.OrderNo
	}
	return v
}

// Ptr 回傳 v 的指標，給 sqlc 的可為 NULL 欄位。
func Ptr[T any](v T) *T { return &v }

// TimePtr 回傳 t 的指標。
func TimePtr(t time.Time) *time.Time { return &t }
