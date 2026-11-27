package registry

import (
	"context"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/text"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// VoucherExpiringParams 是 voucher_expiring 的參數快照，欄位取自 coupon.VoucherExpiring；活動名稱是賣家或平台填的原文，不存。
type VoucherExpiringParams struct {
	EventID    uuid.UUID `json:"event_id"`
	OccurredAt time.Time `json:"occurred_at"`
	BuyerID    int64     `json:"buyer_id"`
	VoucherID  uuid.UUID `json:"voucher_id"`
	CampaignID uuid.UUID `json:"campaign_id"`
	ExpiresAt  time.Time `json:"expires_at"`
}

var exampleCampaignID = uuid.MustParse("0190a000-0000-7000-8000-0000000000d2")

var couponTypes = []Entry{voucherExpiring}

var voucherExpiring = Define(Definition[VoucherExpiringParams]{
	Name:              "voucher_expiring",
	Audiences:         []model.Audience{model.AudienceBuyer},
	Category:          model.CategoryCoupon,
	Presentation:      model.PresentationPromotion,
	Channels:          interactionChannels,
	IdempotencySource: "coupon.VoucherExpiring.VoucherID",
	IdempotencyKey:    func(p VoucherExpiringParams) string { return p.VoucherID.String() },
	OccurredAt:        func(p VoucherExpiringParams) time.Time { return p.OccurredAt },
	Expand: func(ctx context.Context, r Resolver, p VoucherExpiringParams) (*Expansion, error) {
		return BuyerRecipient(ctx, r, p.BuyerID)
	},
	StillValid: func(ctx context.Context, v Validity, p VoucherExpiringParams) (bool, error) {
		return v.VoucherAvailable(ctx, p.BuyerID, p.VoucherID)
	},
	NotAfter: func(p VoucherExpiringParams) (time.Time, bool) { return p.ExpiresAt, true },
	Text: func(l uitext.Locale, p VoucherExpiringParams) (Content, error) {
		return Content{Title: text.VoucherExpiringTitle(l), Body: text.VoucherExpiringBody(l, text.VoucherExpiringBodyArgs{Time: uitext.FormatDateTime(l, p.ExpiresAt)})}, nil
	},
	DeepLink: func(VoucherExpiringParams) string { return "/account/coupons" },
	Example: VoucherExpiringParams{
		EventID: exampleEventID, OccurredAt: exampleAt, BuyerID: exampleBuyerID, VoucherID: exampleObject, CampaignID: exampleCampaignID, ExpiresAt: exampleAt.Add(24 * time.Hour),
	},
})
