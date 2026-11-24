package model

import (
	"time"

	"uuid"
)

// 後台清單每頁筆數：未指定用 DefaultRecordPageSize，上限 MaxRecordPageSize。
const (
	DefaultRecordPageSize int32 = 20
	MaxRecordPageSize     int32 = 100
)

// RecordCursor 是後台清單的排序鍵：主鍵是 UUIDv7，依 ID 由大到小就是建立時間由新到舊。
type RecordCursor struct {
	ID uuid.UUID `json:"id"`
}

// Valid 回傳排序鍵是否有值。
func (c RecordCursor) Valid() bool { return c.ID != uuid.UUID{} }

// PaymentRecord 是後台看到的付款：對外視圖加上 provider 的外部 ID、失敗原因與更新時間（不含 client secret）。
type PaymentRecord struct {
	Payment
	ExternalPaymentID string
	// FailureReason 只在 CREATE_FAILED 與 ANOMALY 有值。
	FailureReason string
	UpdatedAt     time.Time
}

// PaymentFilter 是付款清單的篩選：Status 與 OrderNo 空字串是不篩。
type PaymentFilter struct {
	Status  PaymentStatus
	OrderNo string
}

// PaymentRecordPage 是一頁付款；Next 非 nil 表示還有下一頁。
type PaymentRecordPage struct {
	Records []PaymentRecord
	Next    *RecordCursor
}

// TransferRecord 是後台看到的轉帳：對外視圖加上所屬付款、訂單應收的訂單編號與 provider 的外部 ID。
type TransferRecord struct {
	Transfer
	PaymentID          uuid.UUID
	OrderNo            string
	ExternalTransferID string
}

// TransferFilter 是轉帳清單的篩選：Status 與 OrderNo 空字串是不篩。
type TransferFilter struct {
	Status  TransferStatus
	OrderNo string
}

// TransferRecordPage 是一頁轉帳；Next 非 nil 表示還有下一頁。
type TransferRecordPage struct {
	Records []TransferRecord
	Next    *RecordCursor
}

// ReversalRecord 是退款向賣家收回的一次 provider 撤回呼叫（後台人工結案撤回要用它的 ID）。
type ReversalRecord struct {
	ID         uuid.UUID
	Seq        int32
	Amount     int64
	Status     ReversalStatus
	Reason     string
	ExternalID string
	CreatedAt  time.Time
}

// RefundRecord 是後台看到的退款：對外視圖加上所屬付款與訂單編號、provider 的外部 ID 與撤回列。
type RefundRecord struct {
	Refund
	PaymentID        uuid.UUID
	OrderNo          string
	ExternalRefundID string
	Reversals        []ReversalRecord
}

// RefundFilter 是退款清單的篩選：Status 與 OrderNo 空字串是不篩；AnomalyOnly 為真時只列退款、收回或補差額任一個在 ANOMALY 的退款。
type RefundFilter struct {
	Status      RefundStatus
	OrderNo     string
	AnomalyOnly bool
}

// RefundRecordPage 是一頁退款；Next 非 nil 表示還有下一頁。
type RefundRecordPage struct {
	Records []RefundRecord
	Next    *RecordCursor
}

// EarningsType 是賣家入帳帳本一列的種類。
type EarningsType string

// 入帳帳本的種類。
const (
	// EarningsTransfer 平台把訂單的款項轉給賣家（金額為正）。
	EarningsTransfer EarningsType = "TRANSFER"
	// EarningsReversal 退款時向賣家收回已轉出的款項（金額為負）。
	EarningsReversal EarningsType = "REVERSAL"
	// EarningsTopUp 補差額：退款失敗後把少收的款項補給賣家（金額為正）。
	EarningsTopUp EarningsType = "TOP_UP"
)

// EarningsTypeValues 回傳全部入帳帳本的種類。
func EarningsTypeValues() []EarningsType {
	return []EarningsType{EarningsTransfer, EarningsReversal, EarningsTopUp}
}

// EarningsStatus 是賣家入帳帳本一列的狀態，與 provider 無關：各來源的狀態收斂成這三種。
type EarningsStatus string

// 入帳帳本的狀態。
const (
	// EarningsPending 處理中（含待營運人工處理的列：賣家沒有要做的事）。
	EarningsPending EarningsStatus = "PENDING"
	// EarningsCompleted 已完成。
	EarningsCompleted EarningsStatus = "COMPLETED"
	// EarningsFailed 失敗（終局），款項沒有轉出。
	EarningsFailed EarningsStatus = "FAILED"
)

// EarningsStatusValues 回傳全部入帳帳本的狀態。
func EarningsStatusValues() []EarningsStatus {
	return []EarningsStatus{EarningsPending, EarningsCompleted, EarningsFailed}
}

// EarningsEntry 是賣家入帳帳本的一列。ID 是來源紀錄的 ID（轉帳、撤回列，補差額是所屬退款）；Amount 帶正負：轉入與補差額為正，收回為負。
type EarningsEntry struct {
	ID        uuid.UUID
	Type      EarningsType
	OrderNo   string
	Amount    int64
	Status    EarningsStatus
	CreatedAt time.Time
}

// EarningsFilter 是入帳帳本的篩選：OrderNo 空字串不篩；From（含）與 To（不含）篩列的記錄時間，nil 不限。
type EarningsFilter struct {
	OrderNo string
	From    *time.Time
	To      *time.Time
}

// EarningsPage 是一頁入帳列；Next 非 nil 表示還有下一頁。
type EarningsPage struct {
	Entries []EarningsEntry
	Next    *RecordCursor
}

// OrderEarnings 是一張訂單的全部入帳列（新的在前）與已完成列的淨額（只加總 EarningsCompleted 的列）。
type OrderEarnings struct {
	Entries   []EarningsEntry
	NetAmount int64
}
