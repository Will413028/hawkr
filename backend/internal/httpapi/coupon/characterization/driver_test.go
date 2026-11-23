// Package characterization 是 coupon 對外 API 的特性測試（ADR 2026-09-24-go-modular-monolith D8 的第一層
// 安全網、ADR 2026-09-26-external-contract-protobuf-connect D4），共用的部分在 internal/httpapi/characterize。
//
// 情境只寫一次（scenarios_test.go），透過 Driver 操作：REST driver 經 gateway 打舊的 Java coupon-service。
//
// 情境鎖定的是 Java coupon-service 當時的行為，沒有驗證那行為是否正確；疑似缺陷處以 BD 編號標明
// （docs/plans/2026-11-23-b7-coupon/defects.md），另案處理。計畫決定要改的行為（D2、BD 標「修」）寫成修正後的
// 斷言，在 REST driver 上以 skipOnLegacy 略過並附編號。coupon-service 刪除時，REST driver、legacy.sh 與
// TestLegacy 一起刪除。
//
// 兩個 driver 的 wire 不同，情境只看業務上的結果：ID 當成不透明的字串、錯誤只比 characterize 的分類與下面的
// coupon 錯誤。
//
// REST 打不到的能力（沒有 gateway 路由，不在這裡覆蓋）：買家驗券（ValidateCoupon）、鎖券、核銷、釋放與補償券發放
// 是 order、aftersales 的內部 gRPC；admin 的補償券與代領是 D6 不保留的能力。
package characterization

import (
	"context"
	"errors"
	"time"

	ch "github.com/Will413028/hawkr/backend/internal/httpapi/characterize"
)

// 種子帳號（infra/seed-data/01-account.sql）：兩家賣家的店家 ID 與兩位買家的帳號、買家 ID。
const (
	StoreSeller  = "1009"
	StoreSeller2 = "1004"

	AccountBuyer  = "2001"
	AccountBuyer2 = "2002"
	BuyerID2      = "52002"
)

// coupon 的業務錯誤：characterize 的分類放不下的失敗。
var (
	ErrQuotaExceeded = errors.New("賣家模板配額已滿")
	ErrSoldOut       = errors.New("發行上限已達")
	ErrWrongStatus   = errors.New("狀態不允許此操作")
	ErrNotActive     = errors.New("活動不是進行中")
	ErrOutOfWindow   = errors.New("不在有效期間")
	ErrNotClaimable  = errors.New("此券不需要領取")
)

type CouponType string

const (
	TypeCash    CouponType = "CASH"
	TypePercent CouponType = "PERCENT"
)

type Target string

const (
	TargetAll          Target = "ALL"
	TargetMember       Target = "MEMBER"
	TargetDesignated   Target = "DESIGNATED"
	TargetCompensation Target = "COMPENSATION"
)

type Status string

const (
	StatusDraft    Status = "DRAFT"
	StatusActive   Status = "ACTIVE"
	StatusInactive Status = "INACTIVE"
	StatusExpired  Status = "EXPIRED"
	StatusRevoked  Status = "REVOKED"
)

type Issuer string

const (
	IssuerAdmin  Issuer = "ADMIN"
	IssuerSeller Issuer = "SELLER"
)

// TemplateInput 是建立模板的宣告；金額與數量 0 表示不送。
type TemplateInput struct {
	Title           string
	Type            CouponType
	Target          Target
	Threshold       int64
	DiscountAmount  int64
	DiscountPercent int
	MaxDiscount     int64
	StartAt         time.Time
	EndAt           time.Time
	// BuyerAccounts 是指定買家的帳號 ID。
	BuyerAccounts []string
	// StoreID 與 TotalIssueLimit、ClaimRequired 只有 admin 能設。
	StoreID         string
	TotalIssueLimit int
	// ClaimRequired 為 nil 時不送，admin 的預設是需要領取。
	ClaimRequired *bool
}

// Template 是模板。TotalIssueLimit 是 -1 表示不限量。
type Template struct {
	ID              string
	Title           string
	Type            CouponType
	Target          Target
	Status          Status
	Issuer          Issuer
	StoreID         string
	IssuerStoreID   string
	Threshold       int64
	DiscountAmount  int64
	DiscountPercent int
	MaxDiscount     int64
	TotalIssueLimit int
	IssuedCount     int
	ClaimRequired   bool
	Claimed         bool
	// TargetBuyers 是指定買家的買家 ID，只有詳情會帶。
	TargetBuyers []string
}

type TemplateList struct {
	Records []Template
	Total   int
}

// Page 是分頁參數；0 表示不送。
type Page struct {
	Number int
	Size   int
}

// ListQuery 是買家自己的券列表的查詢。Status 空字串不送，不認得的值原樣送出。
type ListQuery struct {
	Status string
	Page   Page
}

// Instance 是買家已持有的一張券；Status 原樣保留 wire 的字串。
type Instance struct {
	ID         string
	TemplateID string
	Status     string
}

// MyCoupons 是買家的券列表：Available 是買家資格內、可用或可領的模板，Used 是已使用的券。
type MyCoupons struct {
	Available []Template
	Used      []Instance
	Total     int
}

// AvailableQuery 是結帳頁可用券的查詢；空字串與 0 不送。
type AvailableQuery struct {
	StoreID     string
	OrderAmount int64
	Page        Page
}

type AvailableCoupon struct {
	Template          Template
	EstimatedDiscount int64
}

type AvailableList struct {
	Records []AvailableCoupon
	Total   int
}

// ClaimResult 是領券的結果；VoucherID 是領到的券的 ID（D2 之後才有）。
type ClaimResult struct {
	ClaimedAt string
	VoucherID string
}

// SellerDetail 是賣家看到的模板詳情。
type SellerDetail struct {
	Template Template
	Used     int
	Reserved int
}

// IssueResult 是 admin 指定發放的結果。
type IssueResult struct {
	Issued      int
	Distributed int
}

// Driver 是情境操作 coupon 的方式；錯誤對應到 characterize 的分類與上面的 coupon 錯誤。
type Driver interface {
	// Legacy 回報這是不是對舊的 coupon-service 跑（skipOnLegacy 用）。
	Legacy() bool

	ListMyCoupons(ctx context.Context, c ch.Caller, q ListQuery) (MyCoupons, error)
	ListAvailable(ctx context.Context, c ch.Caller, q AvailableQuery) (AvailableList, error)
	Claim(ctx context.Context, c ch.Caller, templateID string) (ClaimResult, error)

	SellerCreate(ctx context.Context, c ch.Caller, in TemplateInput) (Template, error)
	SellerList(ctx context.Context, c ch.Caller, status Status, p Page) (TemplateList, error)
	SellerDetail(ctx context.Context, c ch.Caller, templateID string) (SellerDetail, error)
	SellerRevoke(ctx context.Context, c ch.Caller, templateID, reason string) error

	AdminCreate(ctx context.Context, c ch.Caller, in TemplateInput) (Template, error)
	AdminPublish(ctx context.Context, c ch.Caller, templateID string) error
	AdminDisable(ctx context.Context, c ch.Caller, templateID string) error
	AdminIssue(ctx context.Context, c ch.Caller, templateID string, buyerAccounts []string, immediate bool) (IssueResult, error)
	AdminVoid(ctx context.Context, c ch.Caller, voucherID, reason string) error
	AdminSetQuota(ctx context.Context, c ch.Caller, storeID string, max int) error
}
