// Package coupon 是 coupon 模組的公開 API：優惠券活動與買家持有的券（領券、指定發放、補償、鎖定、核銷、釋放、作廢、提前結束）。
//
// 職責、擁有的資料與不變量見同目錄的 AGENTS.md。
package coupon

import (
	"context"
	"embed"
	"io/fs"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/coupon/internal/campaign"
	"github.com/Will413028/hawkr/backend/internal/modules/coupon/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/coupon/internal/qualify"
	"github.com/Will413028/hawkr/backend/internal/modules/coupon/internal/read"
	"github.com/Will413028/hawkr/backend/internal/modules/coupon/internal/voucher"
	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

//go:embed db/migrations/*.sql db/migrations/atlas.sum
var migrationFiles embed.FS

// Migrations 回傳本模組的 migration，給 cmd/hawkr 登記。
func Migrations() fs.FS {
	sub, err := fs.Sub(migrationFiles, "db/migrations")
	if err != nil {
		panic(err) // 只有 embed 的路徑寫錯才會發生，編譯後的 binary 不可能遇到
	}
	return sub
}

// Deps 是模組需要的外部依賴，全部在組裝點提供。
type Deps struct {
	DB      *db.DB
	Stores  Stores
	Members Members
	// NewID 產生活動與券的主鍵，組裝點給 uuid.NewV7（ADR 2026-09-25-row-identity-by-data-kind）。
	NewID func() uuid.UUID
	// Now 是目前時間；測試注入固定值。
	Now func() time.Time
}

type (
	Campaign       = model.Campaign
	Voucher        = model.Voucher
	DiscountType   = model.DiscountType
	TargetType     = model.TargetType
	CampaignStatus = model.CampaignStatus
	VoucherStatus  = model.VoucherStatus
	Origin         = model.Origin
	SourceType     = model.SourceType
	Issuer         = model.Issuer
	CampaignSpec   = model.CampaignSpec
	Reason         = model.Reason

	CreateCampaignInput = campaign.CreateInput
	EndCampaignInput    = campaign.EndInput
	EndCampaignResult   = campaign.EndResult

	ClaimResult        = voucher.ClaimResult
	IssueInput         = voucher.IssueInput
	IssueResult        = voucher.IssueResult
	CompensationInput  = voucher.CompensationInput
	CompensationResult = voucher.CompensationResult
	LockInput          = voucher.LockInput
	LockResult         = voucher.LockResult
	RedeemResult       = voucher.RedeemResult
	ReleaseResult      = voucher.ReleaseResult
	VoidVoucherInput   = voucher.VoidInput

	Checkout           = read.Checkout
	Cursor             = read.Cursor
	VoucherView        = read.VoucherView
	VoucherListInput   = read.VoucherListInput
	VoucherPage        = read.VoucherPage
	ClaimableInput     = read.ClaimableInput
	CampaignPage       = read.CampaignPage
	SellerListInput    = read.SellerListInput
	SellerCampaignPage = read.SellerCampaignPage
	SellerCampaign     = read.SellerCampaign
	CampaignDetail     = read.CampaignDetail
	AdminListInput     = read.AdminListInput
	AdminCampaignPage  = read.AdminCampaignPage
)

const (
	DiscountCash    = model.DiscountCash
	DiscountPercent = model.DiscountPercent

	TargetAll          = model.TargetAll
	TargetMember       = model.TargetMember
	TargetDesignated   = model.TargetDesignated
	TargetCompensation = model.TargetCompensation

	StatusDraft  = model.StatusDraft
	StatusActive = model.StatusActive
	StatusEnded  = model.StatusEnded

	VoucherAvailable = model.VoucherAvailable
	VoucherLocked    = model.VoucherLocked
	VoucherRedeemed  = model.VoucherRedeemed
	VoucherVoided    = model.VoucherVoided

	OriginClaim        = model.OriginClaim
	OriginGrant        = model.OriginGrant
	OriginCompensation = model.OriginCompensation

	SourceAftersalesCase    = model.SourceAftersalesCase
	SourceAdminCompensation = model.SourceAdminCompensation

	DefaultPageSize = read.DefaultPageSize
	MaxPageSize     = read.MaxPageSize

	MaxIssueBatch      = model.MaxIssueBatch
	MaxTargetBuyers    = model.MaxTargetBuyers
	MaxOrderNoLen      = model.MaxOrderNoLen
	MaxOrderAmount     = model.MaxOrderAmount
	DefaultSellerQuota = model.DefaultSellerQuota

	ReasonCampaignNotFound          = model.ReasonCampaignNotFound
	ReasonVoucherNotFound           = model.ReasonVoucherNotFound
	ReasonCampaignNotActive         = model.ReasonCampaignNotActive
	ReasonOutsideWindow             = model.ReasonOutsideWindow
	ReasonNotEligible               = model.ReasonNotEligible
	ReasonStoreSuspended            = model.ReasonStoreSuspended
	ReasonIssueLimitReached         = model.ReasonIssueLimitReached
	ReasonVoucherUnavailable        = model.ReasonVoucherUnavailable
	ReasonVoucherLockedByOtherOrder = model.ReasonVoucherLockedByOtherOrder
	ReasonOrderHoldsAnotherVoucher  = model.ReasonOrderHoldsAnotherVoucher
	ReasonNotApplicableToStore      = model.ReasonNotApplicableToStore
	ReasonBelowThreshold            = model.ReasonBelowThreshold
	ReasonQuotaExceeded             = model.ReasonQuotaExceeded
	ReasonNotDesignated             = model.ReasonNotDesignated
	ReasonInvalidStatus             = model.ReasonInvalidStatus
	ReasonNoVoucherForOrder         = model.ReasonNoVoucherForOrder
	ReasonNoDiscount                = model.ReasonNoDiscount
)

// ReasonOf 回傳 err 帶的 coupon 業務原因；沒有時是空字串。
func ReasonOf(err error) Reason { return model.ReasonOf(err) }

// Module 是 coupon 模組。
type Module struct {
	db       *db.DB
	audit    *adminaudit.Log
	campaign *campaign.Service
	voucher  *voucher.Service
	read     *read.Service
}

// New 建立模組。
func New(d Deps) *Module {
	if d.DB == nil || d.Stores == nil || d.Members == nil || d.NewID == nil || d.Now == nil {
		panic("coupon：DB、Stores、Members、NewID 與 Now 是必要的")
	}
	q := qualify.New(qualify.Deps{DB: d.DB, Stores: d.Stores, Members: d.Members})
	return &Module{
		db:       d.DB,
		audit:    adminaudit.New(d.DB, d.Now),
		campaign: campaign.New(campaign.Deps{DB: d.DB, NewID: d.NewID, Now: d.Now}),
		voucher:  voucher.New(voucher.Deps{DB: d.DB, Qualify: q, NewID: d.NewID, Now: d.Now}),
		read:     read.New(read.Deps{DB: d.DB, Qualify: q, Now: d.Now}),
	}
}

// 下面所有寫入都在 ctx 的 transaction 內執行（ctx 沒有 transaction 時各自開一個），所以呼叫方可以把它們和自己的狀態變更放進同一個
// transaction：ordering 在結帳與訂單狀態轉換的 transaction 內呼叫 Lock、Redeem、Release。

// CreateCampaign 建立活動：賣家的直接 ACTIVE 並檢查配額，admin 的是 DRAFT。
func (m *Module) CreateCampaign(ctx context.Context, in CreateCampaignInput) (Campaign, error) {
	return m.campaign.Create(ctx, in)
}

// PublishCampaign 把 DRAFT 的活動發布成 ACTIVE。
func (m *Module) PublishCampaign(ctx context.Context, id uuid.UUID) error {
	return m.campaign.Publish(ctx, id)
}

// EndCampaign 提前結束活動：擋新的領取與鎖定、作廢 AVAILABLE 的券，LOCKED 的照常核銷或釋放。
func (m *Module) EndCampaign(ctx context.Context, in EndCampaignInput) (EndCampaignResult, error) {
	return m.campaign.End(ctx, in)
}

// SetSellerQuota 設定店家同時進行的活動數上限。
func (m *Module) SetSellerQuota(ctx context.Context, storeID int64, maxCount int32) error {
	return m.campaign.SetQuota(ctx, storeID, maxCount)
}

// Claim 讓買家領取活動的券；已領過回既有的那張。
func (m *Module) Claim(ctx context.Context, campaignID uuid.UUID, buyerID int64) (ClaimResult, error) {
	return m.voucher.Claim(ctx, campaignID, buyerID)
}

// Issue 對 DESIGNATED 活動指定發放。
func (m *Module) Issue(ctx context.Context, in IssueInput) (IssueResult, error) {
	return m.voucher.Issue(ctx, in)
}

// IssueCompensation 建立補償券；(SourceType, SourceID) 冪等。
func (m *Module) IssueCompensation(ctx context.Context, in CompensationInput) (CompensationResult, error) {
	return m.voucher.IssueCompensation(ctx, in)
}

// Quote 驗券但不鎖，回傳折抵金額；與 Lock 同一組檢查。
func (m *Module) Quote(ctx context.Context, in LockInput) (LockResult, error) {
	return m.voucher.Quote(ctx, in)
}

// Lock 把一張券鎖給一筆訂單；同一筆訂單重送冪等，別的訂單失敗。
func (m *Module) Lock(ctx context.Context, in LockInput) (LockResult, error) {
	return m.voucher.Lock(ctx, in)
}

// Redeem 核銷持有這筆訂單的券；重送冪等。
func (m *Module) Redeem(ctx context.Context, orderNo string) (RedeemResult, error) {
	return m.voucher.Redeem(ctx, orderNo)
}

// Release 釋放持有這筆訂單的券；重送冪等。
func (m *Module) Release(ctx context.Context, orderNo string) (ReleaseResult, error) {
	return m.voucher.Release(ctx, orderNo)
}

// VoidVoucher 作廢單張券（admin）。
func (m *Module) VoidVoucher(ctx context.Context, in VoidVoucherInput) error {
	return m.voucher.Void(ctx, in)
}

// ListVouchers 列出買家的券（新到舊）。帶 Checkout 時只回 AVAILABLE 而且現在能用的券，並算預估折抵（與 Lock 同一組檢查）。
func (m *Module) ListVouchers(ctx context.Context, in VoucherListInput) (VoucherPage, error) {
	return m.read.ListVouchers(ctx, in)
}

// ListClaimable 列出買家現在領得到的活動（與 Claim 同一個資格判定）。
func (m *Module) ListClaimable(ctx context.Context, in ClaimableInput) (CampaignPage, error) {
	return m.read.ListClaimable(ctx, in)
}

// ListSellerCampaigns 列出店家自己發的活動，附店家的配額與同時進行中的活動數。
func (m *Module) ListSellerCampaigns(ctx context.Context, in SellerListInput) (SellerCampaignPage, error) {
	return m.read.ListSellerCampaigns(ctx, in)
}

// GetSellerCampaign 讀店家自己的活動明細；不存在與不是這家店的回同一個 NotFound。
func (m *Module) GetSellerCampaign(ctx context.Context, storeID int64, id uuid.UUID) (CampaignDetail, error) {
	return m.read.GetSellerCampaign(ctx, storeID, id)
}

// GetVoucher 讀買家自己的一張券與它的活動；不存在與不是這位買家的回同一個 NotFound。
func (m *Module) GetVoucher(ctx context.Context, buyerID int64, id uuid.UUID) (VoucherView, error) {
	return m.read.GetVoucher(ctx, buyerID, id)
}
