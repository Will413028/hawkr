// Package model 是 coupon 模組的對外型別、常數、錯誤原因與純函式（輸入檢查、折抵計算）。
package model

import (
	"context"
	"errors"
	"math"
	"time"
	"unicode/utf8"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/coupon/internal/text"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/trimtext"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// DiscountType 是折抵方式。值與 DB 的 campaigns_discount_type_check 一致。
type DiscountType string

const (
	DiscountCash    DiscountType = "CASH"
	DiscountPercent DiscountType = "PERCENT"
)

// TargetType 是活動的資格類型。值與 DB 的 campaigns_target_type_check 一致。
type TargetType string

const (
	// TargetAll 是每一位買家（與 Java 的 ALL 的差異見 CARRYOVER.md）。
	TargetAll TargetType = "ALL"
	// TargetMember 是在該店有消費紀錄的買家。
	TargetMember TargetType = "MEMBER"
	// TargetDesignated 是資格名單內的買家。
	TargetDesignated TargetType = "DESIGNATED"
	// TargetCompensation 是補償券：名單只有收補償的那位買家，只由 IssueCompensation 建立。
	TargetCompensation TargetType = "COMPENSATION"
)

// CampaignStatus 是活動的狀態。到期不是狀態：從 EndAt 推導。
type CampaignStatus string

const (
	StatusDraft  CampaignStatus = "DRAFT"
	StatusActive CampaignStatus = "ACTIVE"
	StatusEnded  CampaignStatus = "ENDED"
)

// VoucherStatus 是券的狀態。
type VoucherStatus string

const (
	VoucherAvailable VoucherStatus = "AVAILABLE"
	VoucherLocked    VoucherStatus = "LOCKED"
	VoucherRedeemed  VoucherStatus = "REDEEMED"
	VoucherVoided    VoucherStatus = "VOIDED"
)

// Origin 是券的來源。
type Origin string

const (
	OriginClaim        Origin = "CLAIM"
	OriginGrant        Origin = "GRANT"
	OriginCompensation Origin = "COMPENSATION"
)

// SourceType 是補償券的來源事件種類；(SourceType, SourceID) 是冪等鍵。
type SourceType string

const (
	SourceAftersalesCase    SourceType = "AFTERSALES_CASE"
	SourceAdminCompensation SourceType = "ADMIN_COMPENSATION"
)

// Valid 回報 s 是不是已知的來源種類。
func (s SourceType) Valid() bool {
	return s == SourceAftersalesCase || s == SourceAdminCompensation
}

const (
	// MaxTitleRunes 是活動名稱的長度上限，與 campaigns_title_check 同值。
	MaxTitleRunes = 100
	// MaxReasonRunes 是稽核理由的長度上限，與 redemption_logs_reason_check 同值。
	MaxReasonRunes = 255
	// MaxOrderNoLen 是單號長度上限，與 vouchers_order_no_check 同值。
	MaxOrderNoLen = 128
	// MaxSourceIDLen 是補償來源識別的長度上限。
	MaxSourceIDLen = 128
	// MaxIssueBatch 是 Issue 一次的買家數上限（逐張寫入，單 transaction 需要有界）。
	MaxIssueBatch = 500
	// MaxIssueRequestBuyers 是 Issue 請求裡買家 ID 的原始個數上限（去重前）：先擋掉過大的請求，再去重與查詢。
	MaxIssueRequestBuyers = 2 * MaxIssueBatch
	// MaxTargetBuyers 是建立活動時資格名單的買家數上限。
	MaxTargetBuyers = 500
	// DefaultSellerQuota 是沒有配額列的店家可同時進行的活動數。
	DefaultSellerQuota = 5
	// MaxOrderAmount 讓 amount*percent 不會溢位 int64。
	MaxOrderAmount = math.MaxInt64 / 100
)

// Campaign 是一個活動。
type Campaign struct {
	ID                uuid.UUID
	Title             string
	DiscountType      DiscountType
	DiscountAmount    int64
	DiscountPercent   int32
	MaxDiscountAmount int64
	ThresholdAmount   int64
	TargetType        TargetType
	// StoreID 是適用店家；nil 是全平台。
	StoreID *int64
	// IssuerAdminID 與 IssuerStoreID 恰有一個有值。
	IssuerAdminID *int64
	IssuerStoreID *int64
	StartAt       time.Time
	EndAt         time.Time
	// TotalLimit 是總發行量；nil 是無上限。
	TotalLimit  *int32
	IssuedCount int32
	Status      CampaignStatus
	EndedAt     *time.Time
	CreatedAt   time.Time
}

// Voucher 是買家持有的一張券。
type Voucher struct {
	ID         uuid.UUID
	CampaignID uuid.UUID
	BuyerID    int64
	Origin     Origin
	Status     VoucherStatus
	// OrderNo 是持有（鎖定或核銷）這張券的訂單；其他狀態是空字串。
	OrderNo    string
	LockedAt   *time.Time
	RedeemedAt *time.Time
	VoidedAt   *time.Time
	CreatedAt  time.Time
}

// Expired 回報券在 now 是不是已過期（從活動的 EndAt 推導）。
func (c Campaign) Expired(now time.Time) bool { return now.After(c.EndAt) }

// Reason 是 coupon 的業務原因（Go 端的常數，與 apperr 的分類搭配）。
type Reason string

const (
	ReasonCampaignNotFound          Reason = "COUPON_CAMPAIGN_NOT_FOUND"
	ReasonVoucherNotFound           Reason = "COUPON_VOUCHER_NOT_FOUND"
	ReasonCampaignNotActive         Reason = "COUPON_CAMPAIGN_NOT_ACTIVE"
	ReasonOutsideWindow             Reason = "COUPON_OUTSIDE_WINDOW"
	ReasonNotEligible               Reason = "COUPON_NOT_ELIGIBLE"
	ReasonStoreSuspended            Reason = "COUPON_STORE_SUSPENDED"
	ReasonIssueLimitReached         Reason = "COUPON_ISSUE_LIMIT_REACHED"
	ReasonVoucherUnavailable        Reason = "COUPON_VOUCHER_UNAVAILABLE"
	ReasonVoucherLockedByOtherOrder Reason = "COUPON_VOUCHER_LOCKED_BY_OTHER_ORDER"
	ReasonOrderHoldsAnotherVoucher  Reason = "COUPON_ORDER_HOLDS_ANOTHER_VOUCHER"
	ReasonNotApplicableToStore      Reason = "COUPON_NOT_APPLICABLE_TO_STORE"
	ReasonBelowThreshold            Reason = "COUPON_BELOW_THRESHOLD"
	ReasonQuotaExceeded             Reason = "COUPON_QUOTA_EXCEEDED"
	ReasonNotDesignated             Reason = "COUPON_NOT_DESIGNATED"
	ReasonInvalidStatus             Reason = "COUPON_INVALID_STATUS"
	ReasonNoVoucherForOrder         Reason = "COUPON_NO_VOUCHER_FOR_ORDER"
	ReasonNoDiscount                Reason = "COUPON_NO_DISCOUNT"
)

type reasoned struct {
	err    *apperr.Error
	reason Reason
}

func (e *reasoned) Error() string { return e.err.Error() }
func (e *reasoned) Unwrap() error { return e.err }

// WithReason 回傳帶業務原因的 err。
func WithReason(err *apperr.Error, r Reason) error { return &reasoned{err: err, reason: r} }

// ReasonOf 回傳 err 帶的業務原因；沒有時是空字串。
func ReasonOf(err error) Reason {
	var r *reasoned
	if errors.As(err, &r) {
		return r.reason
	}
	return ""
}

// reasonText 是各原因的使用者面句子（模組的 internal/text）。
var reasonText = map[Reason]func(uitext.Locale) uitext.Message{
	ReasonCampaignNotFound:          text.ReasonCampaignNotFound,
	ReasonVoucherNotFound:           text.ReasonVoucherNotFound,
	ReasonCampaignNotActive:         text.ReasonCampaignNotActive,
	ReasonOutsideWindow:             text.ReasonOutsideWindow,
	ReasonNotEligible:               text.ReasonNotEligible,
	ReasonStoreSuspended:            text.ReasonStoreSuspended,
	ReasonIssueLimitReached:         text.ReasonIssueLimitReached,
	ReasonVoucherUnavailable:        text.ReasonVoucherUnavailable,
	ReasonVoucherLockedByOtherOrder: text.ReasonVoucherLockedByOtherOrder,
	ReasonOrderHoldsAnotherVoucher:  text.ReasonOrderHoldsAnotherVoucher,
	ReasonNotApplicableToStore:      text.ReasonNotApplicableToStore,
	ReasonBelowThreshold:            text.ReasonBelowThreshold,
	ReasonQuotaExceeded:             text.ReasonQuotaExceeded,
	ReasonNotDesignated:             text.ReasonNotDesignated,
	ReasonInvalidStatus:             text.ReasonInvalidStatus,
	ReasonNoVoucherForOrder:         text.ReasonNoVoucherForOrder,
	ReasonNoDiscount:                text.ReasonNoDiscount,
}

func message(ctx context.Context, r Reason) uitext.Message {
	f, ok := reasonText[r]
	if !ok {
		panic("coupon：沒有原因句子：" + string(r))
	}
	return f(uitext.LocaleFrom(ctx))
}

// NotFound 回傳 NotFound 分類、帶業務原因與該原因的句子的錯誤。
func NotFound(ctx context.Context, r Reason) error {
	return WithReason(apperr.NotFound("%s", message(ctx, r)), r)
}

// Precondition 回傳 FailedPrecondition 分類、帶業務原因與該原因的句子的錯誤。
func Precondition(ctx context.Context, r Reason) error {
	return WithReason(apperr.FailedPrecondition("%s", message(ctx, r)), r)
}

// InvalidKey 是欄位錯誤的使用者面句子（模組的 internal/text）。
type InvalidKey string

const (
	InvalidTitleLength                      InvalidKey = "invalid.title_length"
	InvalidReasonLength                     InvalidKey = "invalid.reason_length"
	InvalidCashDiscountPositive             InvalidKey = "invalid.cash_discount_positive"
	InvalidCashDiscountNoPercent            InvalidKey = "invalid.cash_discount_no_percent"
	InvalidDiscountAboveThreshold           InvalidKey = "invalid.discount_above_threshold"
	InvalidPercentOutOfRange                InvalidKey = "invalid.percent_out_of_range"
	InvalidPercentNeedsCap                  InvalidKey = "invalid.percent_needs_cap"
	InvalidPercentNoAmount                  InvalidKey = "invalid.percent_no_amount"
	InvalidUnknownDiscountType              InvalidKey = "invalid.unknown_discount_type"
	InvalidThresholdNegative                InvalidKey = "invalid.threshold_negative"
	InvalidPeriodOrder                      InvalidKey = "invalid.period_order"
	InvalidEndInPast                        InvalidKey = "invalid.end_in_past"
	InvalidIDPositive                       InvalidKey = "invalid.id_positive"
	InvalidIssuerInvalid                    InvalidKey = "invalid.issuer_invalid"
	InvalidTotalLimitNegative               InvalidKey = "invalid.total_limit_negative"
	InvalidStoreOrTargetInvalid             InvalidKey = "invalid.store_or_target_invalid"
	InvalidMemberNeedsStore                 InvalidKey = "invalid.member_needs_store"
	InvalidCompensationNotCreatable         InvalidKey = "invalid.compensation_not_creatable"
	InvalidTargetTypeInvalid                InvalidKey = "invalid.target_type_invalid"
	InvalidTargetListOnlyDesignated         InvalidKey = "invalid.target_list_only_designated"
	InvalidDesignatedNeedsBuyers            InvalidKey = "invalid.designated_needs_buyers"
	InvalidOwnStoreOnly                     InvalidKey = "invalid.own_store_only"
	InvalidQuotaInvalid                     InvalidKey = "invalid.quota_invalid"
	InvalidOrderNoInvalid                   InvalidKey = "invalid.order_no_invalid"
	InvalidBuyerIDInvalid                   InvalidKey = "invalid.buyer_id_invalid"
	InvalidAdminIDInvalid                   InvalidKey = "invalid.admin_id_invalid"
	InvalidBuyerCountInvalid                InvalidKey = "invalid.buyer_count_invalid"
	InvalidSourceInvalid                    InvalidKey = "invalid.source_invalid"
	InvalidBuyerStoreIssuerInvalid          InvalidKey = "invalid.buyer_store_issuer_invalid"
	InvalidAmountInvalid                    InvalidKey = "invalid.amount_invalid"
	InvalidCompensationRuleInvalid          InvalidKey = "invalid.compensation_rule_invalid"
	InvalidSellerCompensationNeedsThreshold InvalidKey = "invalid.seller_compensation_needs_threshold"
	InvalidSourceOtherBuyer                 InvalidKey = "invalid.source_other_buyer"
	InvalidStoreInvalid                     InvalidKey = "invalid.store_invalid"
	InvalidOrderAmountInvalid               InvalidKey = "invalid.order_amount_invalid"
	InvalidBuyerNotFound                    InvalidKey = "invalid.buyer_not_found"
	InvalidStoreNotFound                    InvalidKey = "invalid.store_not_found"
	InvalidStoreClosed                      InvalidKey = "invalid.store_closed"
)

// invalidText 是各欄位錯誤的使用者面句子。
var invalidText = map[InvalidKey]func(uitext.Locale) uitext.Message{
	InvalidTitleLength:                      text.InvalidTitleLength,
	InvalidReasonLength:                     text.InvalidReasonLength,
	InvalidCashDiscountPositive:             text.InvalidCashDiscountPositive,
	InvalidCashDiscountNoPercent:            text.InvalidCashDiscountNoPercent,
	InvalidDiscountAboveThreshold:           text.InvalidDiscountAboveThreshold,
	InvalidPercentOutOfRange:                text.InvalidPercentOutOfRange,
	InvalidPercentNeedsCap:                  text.InvalidPercentNeedsCap,
	InvalidPercentNoAmount:                  text.InvalidPercentNoAmount,
	InvalidUnknownDiscountType:              text.InvalidUnknownDiscountType,
	InvalidThresholdNegative:                text.InvalidThresholdNegative,
	InvalidPeriodOrder:                      text.InvalidPeriodOrder,
	InvalidEndInPast:                        text.InvalidEndInPast,
	InvalidIDPositive:                       text.InvalidIdPositive,
	InvalidIssuerInvalid:                    text.InvalidIssuerInvalid,
	InvalidTotalLimitNegative:               text.InvalidTotalLimitNegative,
	InvalidStoreOrTargetInvalid:             text.InvalidStoreOrTargetInvalid,
	InvalidMemberNeedsStore:                 text.InvalidMemberNeedsStore,
	InvalidCompensationNotCreatable:         text.InvalidCompensationNotCreatable,
	InvalidTargetTypeInvalid:                text.InvalidTargetTypeInvalid,
	InvalidTargetListOnlyDesignated:         text.InvalidTargetListOnlyDesignated,
	InvalidDesignatedNeedsBuyers:            text.InvalidDesignatedNeedsBuyers,
	InvalidOwnStoreOnly:                     text.InvalidOwnStoreOnly,
	InvalidQuotaInvalid:                     text.InvalidQuotaInvalid,
	InvalidOrderNoInvalid:                   text.InvalidOrderNoInvalid,
	InvalidBuyerIDInvalid:                   text.InvalidBuyerIdInvalid,
	InvalidAdminIDInvalid:                   text.InvalidAdminIdInvalid,
	InvalidBuyerCountInvalid:                text.InvalidBuyerCountInvalid,
	InvalidSourceInvalid:                    text.InvalidSourceInvalid,
	InvalidBuyerStoreIssuerInvalid:          text.InvalidBuyerStoreIssuerInvalid,
	InvalidAmountInvalid:                    text.InvalidAmountInvalid,
	InvalidCompensationRuleInvalid:          text.InvalidCompensationRuleInvalid,
	InvalidSellerCompensationNeedsThreshold: text.InvalidSellerCompensationNeedsThreshold,
	InvalidSourceOtherBuyer:                 text.InvalidSourceOtherBuyer,
	InvalidStoreInvalid:                     text.InvalidStoreInvalid,
	InvalidOrderAmountInvalid:               text.InvalidOrderAmountInvalid,
	InvalidBuyerNotFound:                    text.InvalidBuyerNotFound,
	InvalidStoreNotFound:                    text.InvalidStoreNotFound,
	InvalidStoreClosed:                      text.InvalidStoreClosed,
}

// Invalid 回傳指出欄位 field 的 InvalidInput，訊息是 key 的使用者面句子（依 ctx 的語系）。
func Invalid(ctx context.Context, field string, key InvalidKey) error {
	f, ok := invalidText[key]
	if !ok {
		panic("coupon：沒有欄位錯誤句子：" + string(key))
	}
	return apperr.InvalidInputOn([]string{field}, "%s", f(uitext.LocaleFrom(ctx)))
}

// Issuer 是活動的發行者：admin 或賣家店家，恰有一個有值。
type Issuer struct {
	AdminID int64
	StoreID int64
}

// Valid 回報 i 是不是恰有一個正整數的發行者。
func (i Issuer) Valid() bool {
	return (i.AdminID > 0) != (i.StoreID > 0) && i.AdminID >= 0 && i.StoreID >= 0
}

// CampaignSpec 是活動的折抵規則與期間，建立時的輸入。
type CampaignSpec struct {
	Title             string
	DiscountType      DiscountType
	DiscountAmount    int64
	DiscountPercent   int32
	MaxDiscountAmount int64
	ThresholdAmount   int64
	StartAt           time.Time
	EndAt             time.Time
}

// NormalizeTitle 去頭尾空白並檢查長度（1 到 MaxTitleRunes 個字元）。
func NormalizeTitle(ctx context.Context, title string) (string, error) {
	t := trimtext.Trim(title)
	if n := utf8.RuneCountInString(t); n < 1 || n > MaxTitleRunes {
		return "", Invalid(ctx, "Title", InvalidTitleLength)
	}
	return t, nil
}

// NormalizeReason 去頭尾空白；空字串回 ""（沒有理由），超長回 InvalidInput。
func NormalizeReason(ctx context.Context, reason string) (string, error) {
	r := trimtext.Trim(reason)
	if utf8.RuneCountInString(r) > MaxReasonRunes {
		return "", Invalid(ctx, "Reason", InvalidReasonLength)
	}
	return r, nil
}

// Validate 檢查規則自洽，與 DB 的 CHECK 同一組條件（先擋下回 InvalidInput，CHECK 是後備）：折抵欄位依方式、
// 門檻不得為負、現金折抵不得超過門檻（門檻 0 = 無門檻，不套用，BD32）、起日不晚於迄日。回傳去空白後的名稱。
func (s CampaignSpec) Validate(ctx context.Context) (string, error) {
	title, err := NormalizeTitle(ctx, s.Title)
	if err != nil {
		return "", err
	}
	switch s.DiscountType {
	case DiscountCash:
		if s.DiscountAmount <= 0 {
			return "", Invalid(ctx, "DiscountAmount", InvalidCashDiscountPositive)
		}
		if s.DiscountPercent != 0 || s.MaxDiscountAmount != 0 {
			return "", Invalid(ctx, "DiscountPercent", InvalidCashDiscountNoPercent)
		}
		if s.ThresholdAmount != 0 && s.DiscountAmount > s.ThresholdAmount {
			return "", Invalid(ctx, "DiscountAmount", InvalidDiscountAboveThreshold)
		}
	case DiscountPercent:
		if s.DiscountPercent < 1 || s.DiscountPercent > 99 {
			return "", Invalid(ctx, "DiscountPercent", InvalidPercentOutOfRange)
		}
		if s.MaxDiscountAmount <= 0 {
			return "", Invalid(ctx, "MaxDiscountAmount", InvalidPercentNeedsCap)
		}
		if s.DiscountAmount != 0 {
			return "", Invalid(ctx, "DiscountAmount", InvalidPercentNoAmount)
		}
	default:
		return "", Invalid(ctx, "DiscountType", InvalidUnknownDiscountType)
	}
	if s.ThresholdAmount < 0 {
		return "", Invalid(ctx, "ThresholdAmount", InvalidThresholdNegative)
	}
	if s.StartAt.IsZero() || s.EndAt.IsZero() || s.StartAt.After(s.EndAt) {
		return "", Invalid(ctx, "EndAt", InvalidPeriodOrder)
	}
	return title, nil
}

// Discount 回傳活動對 orderAmount 的折抵：現金折抵不超過訂單金額；百分比折抵取 amount*percent/100 與上限的較小者。
func Discount(c Campaign, orderAmount int64) int64 {
	if c.DiscountType == DiscountCash {
		return min(c.DiscountAmount, orderAmount)
	}
	return min(orderAmount*int64(c.DiscountPercent)/100, c.MaxDiscountAmount)
}

// PositiveUnique 檢查 ids 都是正整數並去重（保持第一次出現的順序）；field 是錯誤指出的欄位。
func PositiveUnique(ctx context.Context, field string, ids []int64) ([]int64, error) {
	seen := make(map[int64]struct{}, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return nil, Invalid(ctx, field, InvalidIDPositive)
		}
		if _, dup := seen[id]; !dup {
			seen[id] = struct{}{}
			out = append(out, id)
		}
	}
	return out, nil
}

// ExpiringHandler 是 VoucherExpiring 必須有的 handler 名稱（events.SubscribeNamed）：沒有它，到期提醒不寫提醒紀錄也不發布。
const ExpiringHandler = "coupon.voucher-expiring"

// VoucherExpiring 在一張 AVAILABLE 的券進入到期提醒窗時、與提醒紀錄同一個 transaction 內發布一次（每張券至多一次）；
// VoucherID 是下游冪等鍵。只有業務鍵與資料，使用者面的句子由訂閱的 notification 模組產生。CampaignTitle 是賣家或平台填的活動名稱原文。
type VoucherExpiring struct {
	EventID       uuid.UUID
	OccurredAt    time.Time
	BuyerID       int64
	VoucherID     uuid.UUID
	CampaignID    uuid.UUID
	CampaignTitle string
	// ExpiresAt 是活動的 end_at（UTC）。
	ExpiresAt time.Time
}
