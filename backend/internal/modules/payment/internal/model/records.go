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
