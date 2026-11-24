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
	// PaymentCreateFailed 建立的終局：provider 永久拒絕，或查回證明 provider 沒有這筆。
	PaymentCreateFailed PaymentStatus = "CREATE_FAILED"
	// PaymentAnomaly 建立的結果與 provider 對不起來或查回連續失敗：佔住這張訂單、補掃不再重試，由後台人工結案。
	PaymentAnomaly PaymentStatus = "ANOMALY"
)

// PaymentStatusValues 回傳全部付款狀態，與 payments_status_check 相同。
func PaymentStatusValues() []PaymentStatus {
	return []PaymentStatus{PaymentCreating, PaymentAwaitingPayment, PaymentProcessing, PaymentSucceeded, PaymentCanceled, PaymentCreateFailed, PaymentAnomaly}
}

// TransferStatus 是轉帳的狀態（transfers_status_check）。
type TransferStatus string

// 轉帳的狀態。
const (
	// TransferPending 已寫列，尚未呼叫或結果不明。
	TransferPending TransferStatus = "PENDING"
	// TransferSucceeded provider 已建立轉帳。
	TransferSucceeded TransferStatus = "SUCCEEDED"
	// TransferFailed 終局失敗，不佔「一筆付款最多一筆有效轉帳」。
	TransferFailed TransferStatus = "FAILED"
	// TransferAnomaly 與 provider 對不起來或前置條件不成立：不能確定沒有搬錢，所以仍佔位，補掃不再重試，由後台人工結案。
	TransferAnomaly TransferStatus = "ANOMALY"
)

// TransferStatusValues 回傳全部轉帳狀態，與 transfers_status_check 相同。
func TransferStatusValues() []TransferStatus {
	return []TransferStatus{TransferPending, TransferSucceeded, TransferFailed, TransferAnomaly}
}

// Transfer 是付款轉給賣家的轉帳（transfers 的對外視圖）；ReversedAmount 是累計已撤回的額度。
type Transfer struct {
	ID             uuid.UUID
	PaymentID      uuid.UUID
	StoreID        int64
	Amount         int64
	ReversedAmount int64
	Status         TransferStatus
	// FailureReason 只在 TransferFailed 與 TransferAnomaly 有值。
	FailureReason string
	CreatedAt     time.Time
}

// RefundStatus 是退款的狀態（refunds_status_check）。
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
	// RefundAnomaly 與 provider 對不起來或前置條件不成立：仍佔額度，補掃不再重試，由後台人工結案。
	RefundAnomaly RefundStatus = "ANOMALY"
)

// RefundStatusValues 回傳全部退款狀態，與 refunds_status_check 相同。
func RefundStatusValues() []RefundStatus {
	return []RefundStatus{RefundPending, RefundReversing, RefundReversed, RefundRefunding, RefundSucceeded, RefundFailed, RefundAnomaly}
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

// AnomalyKind 是進入 ANOMALY 的物件種類。
type AnomalyKind string

// 進入 ANOMALY 的物件種類。
const (
	AnomalyPayment  AnomalyKind = "payment"
	AnomalyTransfer AnomalyKind = "transfer"
	AnomalyRefund   AnomalyKind = "refund"
)

// MoneyAnomaly 是告警事件：搬錢的物件（付款、轉帳、退款）與 provider 對不起來而進入 ANOMALY，或 provider 已搬錢本地卻已不在可接收的狀態，需要人工核對。
// 在寫入狀態的同一個 transaction 內發布（沒有狀態可寫的孤兒結果也發布）；Reason 是原因碼與 ID 的簡短說明，不含個資。
type MoneyAnomaly struct {
	OccurredAt time.Time
	Kind       AnomalyKind
	ObjectID   uuid.UUID
	PaymentID  uuid.UUID
	Reason     string
}

// AnomalyOutcome 是人工結案的結果：AnomalySucceeded 是 provider 確實執行了（附 provider 物件 ID 為證據），AnomalyFailed 是確認 provider 沒有執行。
type AnomalyOutcome string

// 人工結案的結果。
const (
	AnomalySucceeded AnomalyOutcome = "SUCCEEDED"
	AnomalyFailed    AnomalyOutcome = "FAILED"
)

// RefundRequest 是建立退款的輸入：OrderNo 指向付款；RequestKey 是呼叫端給的冪等鍵（同一筆付款內唯一，同一把鍵重送得到同一筆退款）；
// Auto 標示自動退款（D5 修訂：由訂單一方判斷與發起，payment 只標記）。
type RefundRequest struct {
	OrderNo    string
	Amount     int64
	RequestKey string
	Auto       bool
}

// Refund 是退款的對外視圖。ReversalAmount 是這筆退款要先從賣家轉帳撤回的額度（沒有轉帳或沒有可撤額度時是 0）。
type Refund struct {
	ID             uuid.UUID
	PaymentID      uuid.UUID
	RequestKey     string
	Amount         int64
	Auto           bool
	Status         RefundStatus
	ReversalAmount int64
	// FailureReason 只在 RefundFailed 與 RefundAnomaly 有值。
	FailureReason string
	CreatedAt     time.Time
}

// RefundChanged 在退款進入終局（SUCCEEDED、FAILED）的 transaction 內發布：訂單與售後一方據此得知退款結果（舊事件 refund.succeeded、refund.failed 的來源）。
// RequestKey 是呼叫端建立退款時給的冪等鍵（售後以案件 ID 對回）。
type RefundChanged struct {
	OccurredAt    time.Time
	RefundID      uuid.UUID
	PaymentID     uuid.UUID
	OrderNo       string
	StoreID       int64
	BuyerID       int64
	Amount        int64
	Auto          bool
	RequestKey    string
	Status        RefundStatus
	Previous      RefundStatus
	FailureReason string
}

// AutoRefundCompleted 在自動退款（Auto 為真）成功的 transaction 內發布（D5）：通知買家與後台可見；失敗的自動退款只發 RefundChanged。
type AutoRefundCompleted struct {
	OccurredAt time.Time
	RefundID   uuid.UUID
	PaymentID  uuid.UUID
	OrderNo    string
	StoreID    int64
	BuyerID    int64
	Amount     int64
}
