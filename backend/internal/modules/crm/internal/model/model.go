// Package model 是 crm 模組的對外型別、常數、投影事件與錯誤原因。
package model

import (
	"errors"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// RequiredCapability 是 crm 的店家寫入與讀取需要的賣家能力（行銷經營：停權時被擋）。能力宣告表（httpapi/crm）引用它，檢查只在模組裡。
const RequiredCapability = seller.CapMarketing

const (
	// MaxTagNameRunes 是標籤名稱的長度上限，以字元（rune）計；DB 的 tags_name_check 同值。
	MaxTagNameRunes = 50
	// MaxTagsPerCall 是 AddCustomerTags 一次的名稱數上限。
	MaxTagsPerCall = 20
	// MaxTagsPerStore 與 MaxTagsPerCustomer 是標籤數上限（A6-D18 W4）：ListTags 不分頁、AddCustomerTags 單 transaction，需要有界。
	MaxTagsPerStore    = 200
	MaxTagsPerCustomer = 20
	// MaxStoreIDs 是 VIPStatuses 一次的店家數上限（同 buyer.MaxBatchSize 的量級）。
	MaxStoreIDs = 100
	// DefaultPageSize 與 MaxPageSize 是客戶列表的預設與上限（AIP-158）。
	DefaultPageSize = 20
	MaxPageSize     = 100
	// MaxEventKeyLen 是事件冪等鍵（order.completed 的 event_id、退款的 external_ref_id 或 case_no）的長度上限：
	// ledger.dedup_key 上限 128，扣掉命名空間前綴 "refund:"（最長的前綴）。
	MaxEventKeyLen = 120
	// MaxOrderNoLen 是單號長度上限，與 order_nets_order_no_check 同值。
	MaxOrderNoLen = 128
)

// CustomerType 是店家對買家的分類。值與 DB 的 customers_type_check 一致；BLOCKLIST 不存在於任何層（封鎖只在店家封鎖名單，A6-D2 (i)）。
type CustomerType string

const (
	TypeNormal CustomerType = "NORMAL"
	TypeVIP    CustomerType = "VIP"
)

// Valid 回報 t 是不是已知的分類；空字串不是。
func (t CustomerType) Valid() bool { return t == TypeNormal || t == TypeVIP }

// SortBy 是客戶列表的排序欄位。
type SortBy int

const (
	// SortFirstOrderAt 是預設（零值）：首筆完成訂單的時間。
	SortFirstOrderAt SortBy = iota
	SortTotalSpent
	SortOrderCount
)

// Valid 回報 s 是不是已知的排序欄位。
func (s SortBy) Valid() bool { return s >= SortFirstOrderAt && s <= SortOrderCount }

// Tag 是店家的標籤。
type Tag struct {
	ID   uuid.UUID
	Name string
}

// Customer 是店家的一位客戶。沒有任何聯絡欄位：暱稱與頭像由 httpapi 經 buyer 補，電話、Email、地址不在 crm（A6-D8）。
type Customer struct {
	StoreID      int64
	BuyerID      int64
	Type         CustomerType
	TotalSpent   int64
	OrderCount   int32
	FirstOrderAt time.Time
	Tags         []Tag
}

// CustomerCursor 是客戶列表的 keyset 游標：上一頁最後一列的排序鍵與 BuyerID。只有目前排序欄位對應的鍵有意義
// （SortTotalSpent、SortOrderCount 用 Number，SortFirstOrderAt 用 At）；游標與排序、方向不符時由呼叫者負責（httpapi 把它們編進同一個 token）。
type CustomerCursor struct {
	Number  int64
	At      time.Time
	BuyerID int64
}

// CustomerListInput 是客戶列表的條件。Desc 為 true 時大的在前。
type CustomerListInput struct {
	StoreID int64
	Type    *CustomerType
	TagID   *uuid.UUID
	Sort    SortBy
	Desc    bool
	After   *CustomerCursor
	// Size 為 0 用 DefaultPageSize，超過 MaxPageSize 以上限計。
	Size int32
}

// CustomerPage 是一頁客戶；Next 為 nil 表示沒有下一頁。不回總數。
type CustomerPage struct {
	Customers []Customer
	Next      *CustomerCursor
}

// OrderCompleted 是 order.completed 事件在投影用到的欄位。EventID 是冪等鍵（Java outbox 重試間不變）。
type OrderCompleted struct {
	EventID string
	OrderNo string
	StoreID int64
	BuyerID int64
	// Amount 是訂單的實付總額（新台幣元），不得為負。
	Amount int64
}

// RefundOutcome 是退款的結果。
type RefundOutcome string

const (
	RefundPartial RefundOutcome = "PARTIAL_REFUND"
	RefundFull    RefundOutcome = "FULL_REFUND"
)

// Valid 回報 o 是不是已知的退款結果。
func (o RefundOutcome) Valid() bool { return o == RefundPartial || o == RefundFull }

// Refund 是 aftersales.refund.completed 事件解析後的欄位。Key 是冪等鍵：external_ref_id，缺時 "AS_REFUND_"+case_no
// （Java 每次重發退款事件都換新的 event_id，所以 event_id 不能當鍵）。StoreID、BuyerID 由訂單解析而來（事件本身沒有）。
type Refund struct {
	Key     string
	OrderNo string
	StoreID int64
	BuyerID int64
	Outcome RefundOutcome
	// Amount 只在 PARTIAL_REFUND 使用（必須為正）；FULL_REFUND 不信上游的金額，以自己記錄的淨額歸零。
	Amount int64
}

// Reason 是 crm 的業務原因（Go 端的常數，與 apperr 的分類搭配；對外的 ErrorReason 由 A6-5 加進 proto）。
type Reason string

const (
	ReasonCustomerNotFound Reason = "CRM_CUSTOMER_NOT_FOUND"
	ReasonTagNotFound      Reason = "CRM_TAG_NOT_FOUND"
	ReasonTagNameTaken     Reason = "CRM_TAG_NAME_TAKEN"
	ReasonTagLimit         Reason = "CRM_TAG_LIMIT"
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

// ErrInvalidEvent 是投影事件的欄位不合法（金額為負、缺鍵、鍵過長、ID 非正數、未知退款結果）：重試不會變好，consumer 以它分類為 permanent。
var ErrInvalidEvent = errors.New("crm：投影事件不合法")

// ErrPartiesMismatch 是同一單號的事件帶著不同的買家或店家：上游自相矛盾，重試不會變好，consumer 以它分類為 permanent。
var ErrPartiesMismatch = errors.New("crm：同一單號的買家或店家與已記錄的不同")
