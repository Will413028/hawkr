package domain

import (
	"context"
	"time"
)

type CrmCustomer struct {
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Nickname     string
	Phone        string
	CustomerType CustomerType
	ID           int64
	StoreID      int64
	BuyerID      int64
	TotalSpent   int64
	OrderCount   int
}

// CrmTag 代表賣家自定義的客戶標籤（多租戶）
type CrmTag struct {
	CreatedAt time.Time
	UpdatedAt time.Time
	Name      string
	ID        int64
	StoreID   int64
}

// CrmCustomerTag 代表客戶-標籤關聯
type CrmCustomerTag struct {
	CreatedAt time.Time
	ID        int64
	StoreID   int64
	BuyerID   int64
	TagID     int64
}

// CustomerListParams 客戶列表查詢參數
type CustomerListParams struct {
	Keyword      string
	SortBy       string
	SortOrder    string
	CustomerType string // "" = 全部, "normal" / "vip" / "blocklist"
	StoreID      int64
	TagID        int64
	Page         int32
	PageSize     int32
}

// CustomerCacheUpdate 用於批次更新顧客快取欄位
type CustomerCacheUpdate struct {
	Nickname string
	Phone    string
	BuyerID  int64
}

// LedgerEntry is one immutable, signed row in crm_customer_ledger — the
// source of truth for a customer's total_spent/order_count. See
// docs/superpowers/specs/2026-07-25-crm-customer-ledger-design.md.
type LedgerEntry struct {
	EventID     string
	EventType   string // "order_completed" | "refund_completed_full" | "refund_completed_partial"
	OrderNo     string
	StoreID     int64
	BuyerID     int64
	AmountDelta int64
	CountDelta  int32
}

// CrmRepository 定義 CRM 資料存取介面
type CrmRepository interface {
	// Customer operations
	GetCustomer(ctx context.Context, storeID, buyerID int64) (*CrmCustomer, error)
	ListCustomers(ctx context.Context, params CustomerListParams) ([]*CrmCustomer, error)
	CountCustomers(ctx context.Context, params CustomerListParams) (int64, error)
	CreateCustomer(ctx context.Context, customer *CrmCustomer) error
	UpdateCustomerType(ctx context.Context, storeID, buyerID int64, customerType CustomerType) error
	UpsertCustomerOnOrder(ctx context.Context, id, storeID, buyerID, amount int64, nickname, phone string) error
	BatchUpdateCustomerCache(ctx context.Context, storeID int64, updates []CustomerCacheUpdate) error
	GetCustomerTypesByBuyer(ctx context.Context, buyerID int64, storeIDs []int64) (map[int64]string, error)

	// Tag operations
	GetTag(ctx context.Context, id int64) (*CrmTag, error)
	GetTagByName(ctx context.Context, storeID int64, name string) (*CrmTag, error)
	ListTags(ctx context.Context, storeID int64) ([]*CrmTag, error)
	CreateTag(ctx context.Context, tag *CrmTag) error
	UpdateTag(ctx context.Context, tag *CrmTag) error
	DeleteTag(ctx context.Context, storeID, tagID int64) error

	// Customer-Tag relations
	SearchCustomersByTagIds(ctx context.Context, storeID int64, tagIDs []int64) ([]*CrmCustomer, error)
	GetCustomerTags(ctx context.Context, storeID, buyerID int64) ([]*CrmTag, error)
	AddCustomerTag(ctx context.Context, storeID, buyerID, tagID int64) error
	RemoveCustomerTag(ctx context.Context, storeID, buyerID, tagID int64) error

	// Customer ledger — insert-only, event_id-keyed idempotency (replaces
	// IsOrderProcessed/MarkOrderProcessed's crm_processed_orders dedup table).
	// AppendLedgerEntry returns true only when it inserted a new row (false =
	// duplicate event_id, caller must skip any further accumulation for this call).
	AppendLedgerEntry(ctx context.Context, entry LedgerEntry) (bool, error)
	// ApplyRefundDelta upserts amountDelta (expected negative) into a
	// customer's total_spent, creating the row (order_count=0) if a refund
	// genuinely arrives before this order's own order.completed has ever
	// been recorded — a real, reachable ordering (aftersales cases open at
	// DELIVERED(40), strictly before COMPLETED(50); see spec Background).
	// id is a pre-generated TSID, used only on the insert branch. Returns
	// rows affected as a defensive sanity check — an upsert always affects
	// exactly 1 row; anything else is a genuine anomaly.
	ApplyRefundDelta(ctx context.Context, id, storeID, buyerID, amountDelta int64) (int64, error)

	// RedactCustomerPIIByBuyerID 抹除某買家在所有賣家 CRM 裡的個資快取（#2847）。
	//
	// 與 BatchUpdateCustomerCache 的空值語意刻意分開：那邊在上游給不出值時保留舊值
	// （避免賣家 keyword 搜尋失效），這裡則是買家已行使刪除權、保留舊值即違規。
	// 回傳實際更動的列數：0 代表本來就乾淨（事件重放安全）。
	RedactCustomerPIIByBuyerID(ctx context.Context, buyerID int64) (int64, error)
	// SumLedgerAmountForOrder returns the net amount_delta recorded so far
	// for one order_no. FULL_REFUND nets to exactly this value, negated,
	// unconditionally — see spec §2 attempt 5 for why no other signal is
	// needed (order.completed either already contributed to this sum, or
	// structurally never will).
	SumLedgerAmountForOrder(ctx context.Context, orderNo string) (int64, error)
	// HasFullRefundEntry reports whether order_no already has a
	// refund_completed_full row in the ledger. RecordOrder uses this to
	// avoid contributing a nonzero amount for an order that was already
	// fully refunded before order.completed reached crm's ledger.
	HasFullRefundEntry(ctx context.Context, orderNo string) (bool, error)
	// LockOrderForNetting takes a transaction-scoped Postgres advisory
	// lock keyed on order_no. Call as the first statement inside a
	// transaction in both RecordOrder and RecordRefund — serializes every
	// event for a given order_no across MQ subscriptions/goroutines,
	// closing the concurrent cross-queue snapshot race and the lost-update
	// case where two concurrent FULL_REFUND events both read the same
	// stale state.
	//
	// This correctness depends on the transaction running at READ COMMITTED
	// (Postgres's default, and what database.TxManager's plain pool.Begin
	// uses — no isolation override anywhere). Each statement after the lock
	// is granted takes a fresh snapshot, which is what lets the reads below
	// see a concurrent transaction's commit once this one acquires the lock.
	// Under REPEATABLE READ/SERIALIZABLE, the snapshot would be fixed by the
	// lock-acquiring statement itself (before waiting), reopening the race
	// this lock exists to close — do not raise the isolation level here
	// without re-deriving this reasoning.
	//
	// KNOWN GAP this lock does NOT close (mutual exclusion is not
	// ordering): if a PARTIAL_REFUND is redelivered to the queue tail by
	// MQ retry (e.g. a transient GetOrderByNo failure) and lands *after* a
	// FULL_REFUND for the same order has already zero-netted it, that
	// PARTIAL_REFUND still applies its unconditional -refund_amount,
	// permanently understating total_spent. Requires the events to be
	// out of business order to begin with — accepted, documented residual
	// risk (see spec §2's concurrency paragraph), not fixed here.
	LockOrderForNetting(ctx context.Context, orderNo string) error
}
