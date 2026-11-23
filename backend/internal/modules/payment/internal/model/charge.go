package model

import (
	"time"

	"uuid"
)

// PaymentStatus 是付款的狀態（payments_status_check），只由 provider 的快照推導。
type PaymentStatus string

// 付款的狀態。
const (
	// PaymentCreating 已寫列，provider 那邊的 PaymentIntent 還沒確認建立（含呼叫過、結果不明）。
	PaymentCreating PaymentStatus = "CREATING"
	// PaymentAwaitingPayment 已建立，等買家付款（含需要 3DS 的額外驗證）。
	PaymentAwaitingPayment PaymentStatus = "AWAITING_PAYMENT"
	// PaymentProcessing 買家已送出付款，provider 處理中。
	PaymentProcessing PaymentStatus = "PROCESSING"
	// PaymentSucceeded 已入帳（終局）。
	PaymentSucceeded PaymentStatus = "SUCCEEDED"
	// PaymentCanceled 已取消（provider 的成功仍會勝出）。
	PaymentCanceled PaymentStatus = "CANCELED"
	// PaymentCreateFailed 建立的終局：provider 永久拒絕，或結果不明超過期限。
	PaymentCreateFailed PaymentStatus = "CREATE_FAILED"
)

// PaymentStatusValues 回傳全部付款狀態，與 payments_status_check 相同。
func PaymentStatusValues() []PaymentStatus {
	return []PaymentStatus{PaymentCreating, PaymentAwaitingPayment, PaymentProcessing, PaymentSucceeded, PaymentCanceled, PaymentCreateFailed}
}

// TransferStatus 是轉帳的狀態（transfers_status_check；3b 使用）。
type TransferStatus string

// 轉帳的狀態。
const (
	// TransferPending 已寫列，尚未呼叫或結果不明。
	TransferPending TransferStatus = "PENDING"
	// TransferSucceeded provider 已建立轉帳。
	TransferSucceeded TransferStatus = "SUCCEEDED"
	// TransferFailed 終局失敗，不佔「一筆付款最多一筆有效轉帳」。
	TransferFailed TransferStatus = "FAILED"
)

// TransferStatusValues 回傳全部轉帳狀態，與 transfers_status_check 相同。
func TransferStatusValues() []TransferStatus {
	return []TransferStatus{TransferPending, TransferSucceeded, TransferFailed}
}

// RefundStatus 是退款的狀態（refunds_status_check；3c 使用）。
type RefundStatus string

// 退款的狀態。
const (
	// RefundPending 已寫列，尚未呼叫任何 provider。
	RefundPending RefundStatus = "PENDING"
	// RefundReversing 已轉帳的付款先撤回轉帳（reversal_amount，可能是部分撤回），撤回進行中。
	RefundReversing RefundStatus = "REVERSING"
	// RefundReversed 撤回完成、尚未呼叫退款：等待撤回與已呼叫退款之間的中間狀態，重啟從這裡接著呼叫退款。
	RefundReversed RefundStatus = "REVERSED"
	// RefundRefunding 已呼叫退款，等 provider 的結果。
	RefundRefunding RefundStatus = "REFUNDING"
	// RefundSucceeded 退款成功。
	RefundSucceeded RefundStatus = "SUCCEEDED"
	// RefundFailed 退款失敗，不佔累計額度。
	RefundFailed RefundStatus = "FAILED"
)

// RefundStatusValues 回傳全部退款狀態，與 refunds_status_check 相同。
func RefundStatusValues() []RefundStatus {
	return []RefundStatus{RefundPending, RefundReversing, RefundReversed, RefundRefunding, RefundSucceeded, RefundFailed}
}

// Payment 是買家對一張訂單的付款。金額是整數、不帶幣別（D9）。
type Payment struct {
	ID        uuid.UUID
	OrderNo   string
	StoreID   int64
	BuyerID   int64
	Amount    int64
	Provider  string
	Status    PaymentStatus
	PaidAt    *time.Time
	LastError string
	CreatedAt time.Time
	// ClientSecret 是買家端完成付款的憑證，只在狀態是 AWAITING_PAYMENT 時由 CreatePayment 與 GetPayment 向 provider 讀來回傳，不存。其餘狀態與讀取失敗時是空字串。
	ClientSecret string
}

// PaymentChanged 在付款的狀態改變的 transaction 內發布（寫入快照時狀態與上次不同才發）：訂單一方（過渡期經 relay 給 Java order）據此得知付款成功或取消，
// 並判斷訂單已不可付時發起自動退款（D5 修訂：payment 不讀訂單狀態）。LastError 是最近一次嘗試失敗的 provider 原因碼（沒有時空字串）。
type PaymentChanged struct {
	OccurredAt time.Time
	PaymentID  uuid.UUID
	OrderNo    string
	StoreID    int64
	BuyerID    int64
	Amount     int64
	Provider   string
	Status     PaymentStatus
	// Previous 是這次寫入之前的狀態。
	Previous  PaymentStatus
	PaidAt    *time.Time
	LastError string
}

// ExcessPaid 是一次重複入帳：同一張訂單的付款已經以另一個 PaymentIntent 入帳，這個舊 PaymentIntent 也扣到了款。payment 不自動退款（D5 修訂），給後台與訂單一方處理。
type ExcessPaid struct {
	AttemptID         uuid.UUID
	PaymentID         uuid.UUID
	Provider          string
	ExternalPaymentID string
	ExcessPaidAt      time.Time
}
