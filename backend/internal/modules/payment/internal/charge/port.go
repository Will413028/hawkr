// Package charge 是付款：provider 中立的 port（PaymentIntent、退款、轉帳與撤回、付款事件）、付款的本地狀態、建立付款（請求內建立、job 補做）、
// 同步與補掃，以及 platform inbox 的付款事件處理（計畫 2026-11-23-b2-payment 步驟 3a）。
//
// 結構（backend AGENTS.md 規則 4、5）：
//   - 建立付款：transaction 內只寫「建立中」的列與補做 job；提交之後在請求內、transaction 外帶列上的冪等鍵向 provider 建立 PaymentIntent 並回 client_secret（D7 修訂）。
//     失敗或逾時回 pending，補做 job 與週期掃描以同一把鍵重送；呼叫過、沒有外部 ID 的列在期限（UnresolvedDeadline）內重送，超過進終局。
//   - 付款事件：webhook 先寫 inbox；handler 只排以付款為唯一鍵的同步 job，job 在 transaction 外重讀 PaymentIntent（不信事件內容），在 transaction 內改狀態並發 PaymentChanged。
//   - 退款、轉帳與撤回的 port 方法與 schema 由 3b、3c 的 use case 使用；本步只定簽章並讓 Fake 能被測試驅動。
package charge

import (
	"context"
	"errors"
	"net/http"
	"time"

	"uuid"
)

// State 是 provider 回報的 PaymentIntent 的中立狀態，由 adapter 從回應對應而來；不認得的值對應成 StateUnknown（空值同），不是有效值：同步遇到它不寫入。
type State string

// PaymentIntent 的中立狀態。
const (
	// StateAwaitingPayment 等買家付款（requires_payment_method、requires_confirmation、requires_action 都算）。
	StateAwaitingPayment State = "awaiting_payment"
	// StateProcessing provider 處理中。
	StateProcessing State = "processing"
	// StateSucceeded 已入帳。
	StateSucceeded State = "succeeded"
	// StateCanceled 已取消。
	StateCanceled State = "canceled"
	// StateUnknown provider 回了不認得的狀態。
	StateUnknown State = "unknown"
)

// Valid 回傳 s 是否是四個已知狀態之一（StateUnknown 與空值都不是）。
func (s State) Valid() bool {
	switch s {
	case StateAwaitingPayment, StateProcessing, StateSucceeded, StateCanceled:
		return true
	case StateUnknown:
		return false
	}
	return false
}

// Snapshot 是 PaymentIntent 某一刻的狀態，由 adapter 整理成中立的欄位。Amount 是 provider 那邊的金額（整數，不帶幣別，D9）：與列上的金額不符時同步不信它。
// ChargeID 與 PaidAt 只在 StateSucceeded 有意義（ChargeID 是轉帳的 source_transaction）。LastError 是最近一次付款嘗試失敗的 provider 原因碼。
// ClientSecret 是買家端完成付款的憑證：由 adapter 在能讀到時帶上，不存、不寫 log。
type Snapshot struct {
	ExternalID   string
	State        State
	Amount       int64
	ChargeID     string
	PaidAt       time.Time
	LastError    string
	ClientSecret string
}

// CreateInput 是建立 PaymentIntent 的輸入。IdempotencyKey 是付款列上保存的 UUID：每次重送都用它，期限內得到同一個 PaymentIntent。
// TransferGroup 是訂單號（一筆付款只對一家店，之後的轉帳以它關聯）；StoreID 與 PaymentID 只作 metadata 對帳；Amount 是整數，幣別由 adapter 的設定決定（D9）。
type CreateInput struct {
	PaymentID      uuid.UUID
	OrderNo        string
	StoreID        int64
	Amount         int64
	Description    string
	TransferGroup  string
	IdempotencyKey string
}

// RefundInput 是建立退款的輸入（3c）。ExternalPaymentID 是付款的 PaymentIntent；IdempotencyKey 是退款列上保存的 UUID。
type RefundInput struct {
	ExternalPaymentID string
	Amount            int64
	IdempotencyKey    string
}

// RefundState 是退款的中立狀態；不認得的值是 RefundUnknown，不是有效值。
type RefundState string

// 退款的中立狀態。
const (
	// RefundPending provider 受理、還沒有結果。
	RefundPending RefundState = "pending"
	// RefundSucceeded 退款成功。
	RefundSucceeded RefundState = "succeeded"
	// RefundFailed 退款失敗（終局）。
	RefundFailed RefundState = "failed"
	// RefundUnknown provider 回了不認得的狀態。
	RefundUnknown RefundState = "unknown"
)

// Valid 回傳 s 是否是三個已知狀態之一。
func (s RefundState) Valid() bool {
	switch s {
	case RefundPending, RefundSucceeded, RefundFailed:
		return true
	case RefundUnknown:
		return false
	}
	return false
}

// RefundSnapshot 是退款某一刻的狀態。
type RefundSnapshot struct {
	ExternalID string
	State      RefundState
	Amount     int64
	// FailureReason 是 RefundFailed 時 provider 的原因碼。
	FailureReason string
}

// TransferInput 是建立轉帳的輸入（3b）：付款的入帳（SourceChargeID）與賣家的外部帳戶（DestinationAccountID），TransferGroup 同付款。
type TransferInput struct {
	Amount               int64
	SourceChargeID       string
	DestinationAccountID string
	TransferGroup        string
	IdempotencyKey       string
}

// TransferSnapshot 是轉帳某一刻的狀態；ReversedAmount 是 provider 那邊累計已撤回的額度。
type TransferSnapshot struct {
	ExternalID     string
	Amount         int64
	ReversedAmount int64
}

// ReversalInput 是撤回轉帳（可以部分）的輸入（3b、3c）：Amount 不得超過轉帳的剩餘可撤額度。
type ReversalInput struct {
	ExternalTransferID string
	Amount             int64
	IdempotencyKey     string
}

// ReversalSnapshot 是一次撤回的結果。
type ReversalSnapshot struct {
	ExternalID string
	Amount     int64
}

// EventKind 是付款事件指的物件種類。3a 只處理付款（PaymentIntent）；退款與轉帳的事件由 3b、3c 加入，之前 ParseEvent 對它們回 ErrIgnoredEvent。
type EventKind string

// 事件指的物件種類。
const (
	// EventPayment 指向一個 PaymentIntent。
	EventPayment EventKind = "payment"
)

// Event 是驗過簽章的 provider 事件：事件 ID（inbox 的唯一鍵）與它指的物件。事件內容不攜帶狀態：處理者一律重讀物件。
type Event struct {
	ID       string
	Kind     EventKind
	ObjectID string
}

// Provider 是付款的 port。真正的實作是 Stripe（3d），測試與本機用 Fake。傳輸錯誤（連線、逾時、5xx、429）回一般錯誤，job 重試；provider 以業務理由拒絕、
// 重試也不會成功時包 ErrPermanent。實作不得把傳輸錯誤轉成業務結果。每個呼叫都有 deadline，而且不得在 DB transaction 內呼叫（規則 5）。
type Provider interface {
	// Name 是 provider 的代碼（小寫英數與底線），存在付款列的 provider 欄，也是 inbox 的 provider 名稱：必須與收款帳戶的 provider 名稱不同
	// （inbox 同名登記會 panic；一個事件 destination 一個 endpoint 一把簽章密鑰）。
	Name() string
	// CreatePayment 以 IdempotencyKey 冪等地建立 PaymentIntent 並回傳它的快照（含 ClientSecret）。同一把鍵在保存期內重送得到同一個 PaymentIntent。
	CreatePayment(ctx context.Context, in CreateInput) (Snapshot, error)
	// GetPayment 讀 PaymentIntent 目前的狀態（含 ClientSecret，讀得到時）；不存在是永久失敗。
	GetPayment(ctx context.Context, externalID string) (Snapshot, error)
	// CancelPayment 取消尚未入帳的 PaymentIntent 並回傳取消後的快照；冪等：已取消視為成功。已入帳的回傳入帳的快照（State 是 StateSucceeded，不是錯誤），
	// 由呼叫方依狀態處理。
	CancelPayment(ctx context.Context, externalID string) (Snapshot, error)
	// CreateRefund 以 IdempotencyKey 冪等地建立退款（3c）。
	CreateRefund(ctx context.Context, in RefundInput) (RefundSnapshot, error)
	// GetRefund 讀退款目前的狀態（3c）。
	GetRefund(ctx context.Context, externalID string) (RefundSnapshot, error)
	// CreateTransfer 以 IdempotencyKey 冪等地建立帶 source_transaction 的轉帳（3b）；平台餘額不足是可重試的一般錯誤（spikes.md #8），不是 ErrPermanent。
	CreateTransfer(ctx context.Context, in TransferInput) (TransferSnapshot, error)
	// ReverseTransfer 以 IdempotencyKey 冪等地撤回轉帳的一部分或全部（3b、3c）；賣家餘額不足不是錯誤（spikes.md #9）。
	ReverseTransfer(ctx context.Context, in ReversalInput) (ReversalSnapshot, error)
	// ParseEvent 驗簽並取出事件 ID 與物件；簽章不符或格式不合回 ErrInvalidEvent，驗過簽章但不是要處理的事件回 ErrIgnoredEvent。
	ParseEvent(header http.Header, body []byte) (Event, error)
}

var (
	// ErrPermanent 標示重試也不會成功的失敗。
	ErrPermanent = errors.New("charge: permanent failure")
	// ErrInvalidEvent 標示事件的簽章或格式不合，不處理。
	ErrInvalidEvent = errors.New("charge: invalid event")
	// ErrIgnoredEvent 標示驗過簽章、但不是要處理的事件：回應 2xx，不寫 inbox、不排 job。
	ErrIgnoredEvent = errors.New("charge: ignored event")
)

// Order 是建立付款當下訂單一方回的事實：付款依它寫成快照。
type Order struct {
	OrderNo string
	BuyerID int64
	StoreID int64
	// Amount 是訂單應付金額（整數，不帶幣別，D9）。
	Amount int64
	// Payable 為真表示訂單現在可以付款（待付款、沒有取消或過期）。
	Payable bool
	// Description 是給 provider 的付款說明（商品名稱）。
	Description string
}

// Orders 是訂單一方的 port，由組裝點注入（步驟 4 以 Java order 的 GetOrderForPayment 實作，B8 換成 ordering 的 in-process 呼叫）。呼叫在 transaction 外。
// 訂單不存在回 apperr.NotFound。
type Orders interface {
	OrderForPayment(ctx context.Context, orderNo string) (Order, error)
}
