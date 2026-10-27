package mq

import (
	"context"
	"errors"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"github.com/Will413028/hawkr/invoice-service/internal/usecase"
)

// ---------------------------------------------------------------------------
// consumer 需要的三份事實（§7.1.2 步驟 5）與一個尚未落地的業務出口
// ---------------------------------------------------------------------------

// ErrPeerClientUnavailable 是 peer gRPC client 尚未落地時的 fail-loud 訊號。
//
// 🔴 它刻意是 error 而不是「回零值繼續走」：吞成零值會開出一張沒有品項、金額為 0 的
// 發票，那是不可回收的稅務錯誤。
//
// ⚠️ 但失敗的歸宿**不是 DLQ**（本註解曾經這樣寫，是錯的）：pkg/mq 的 retry 把同一則
// body republish 回原 queue，第二次進 handler 會被 inbox 的去重擋掉並 return nil ⇒
// retry counter 永遠停在 1、`mq_consume_total{result="dlq"}` 對兩支 inbox-backed
// consumer 結構上恆為 0。真正的歸宿是「inbox 列留在 RECEIVED，由 INV.dl 的兜底掃描
// 重跑」（internal/worker/deadline_worker.go），偵測面靠
// `invoice_paid_orders_without_operation`（M8）與 `invoice_inbox_stranded_rows`（M30）。
//
// ⚠️ 這裡曾經寫「M29 invoice_inbox_stuck」——**那個指標從來不存在**（不在設計書 §8 的
// 指標表、不在 sql_exporter.yml、不在 business-alerts.yml，全 repo 只出現在三行註解裡）。
// 註解宣稱一道不存在的機制，比沒有註解更糟：兩位獨立審查者都據此相信「這條路徑有第二道
// 防線」。M30 是 2026-08-31 補上的**真的那一條**（sql_exporter ＋ InvoiceInboxStranded）。
//
// 落地位置是 `internal/client`（§6.2 的四個 peer client），不是本檔。
var ErrPeerClientUnavailable = errors.New("invoice: peer gRPC client is not wired yet (see 58 §6.2)")

// ErrAdjustmentNotImplemented 是 `order.refund.finalized` 的作廢／折讓判定尚未落地的訊號。
//
// 與上面同一條理由：inbox 列已經落庫（去重鍵保住了），但這一則退款**沒有**被套用，
// 所以不得 ACK 成「處理完成」。該列被標成 FAILED 留在候選集合內（同樣不進 DLQ）。
var ErrAdjustmentNotImplemented = errors.New("invoice: refund adjustment (VOID/ALLOWANCE) is not implemented in this wave (see 58 §7.2.2)")

// OrderFacts 是 `ORD.GetOrderInvoiceSnapshotInternal(order_no)` 的回傳（§7.1.2 步驟 5 ①）。
//
// 品項、金額與買家發票偏好**刻意不上 MQ**（§6.3.2.6）：前者必須是「開票當下」的不可變
// 快照，而 MQ 的 at-least-once 重送會讓「當前的 X」在重試時漂掉；後者是敏感資料。
type OrderFacts struct {
	Snapshot        domain.OrderSnapshot
	SnapshotVersion int32

	BuyerTaxID   string
	BuyerTitle   string
	CarrierType  codemeta.InvoiceCarrierType
	CarrierID1   string
	CarrierID2   string
	DonationCode string
	NotifyEmail  string

	// fail-closed 三格中程式推不出來的兩格（§7.1.3）。
	HasPlatformFundedDiscount bool
	IsConsolidatedOrder       bool

	// PreferenceOwnershipMismatch ＝ 解析 buyer_invoice_preference_id 時歸屬驗證失敗
	// （I-47）。它由 ORD adapter 在載具解析當下判定：
	// `WHERE id = ? AND buyer_id = orders.buyer_id AND deleted_at IS NULL` 查不到即成立。
	//
	// 🔴 它必須是**一個獨立訊號**而不是「載具解析成空」：後者會把 IDOR 靜默吞掉，
	// 開出一張沒有載具的發票、當事人永遠不知道有人拿他的常用資料去對別人的訂單試。
	// 呼叫端（usecase.Prepare）把它翻成 AWAITING_MANUAL ＋
	// manual_reason_code = PREFERENCE_OWNERSHIP_MISMATCH。
	PreferenceOwnershipMismatch bool
}

// PaymentFacts 是 `PAY.GetPaymentFactsInternal(order_no)` 的回傳（§7.1.2 步驟 5 ②、註 2）。
type PaymentFacts struct {
	TradeNo        string
	ProviderPaidAt *time.Time
	PayTime        time.Time
	IsDuplicate    bool
}

// PaidAt 回 `COALESCE(provider_paid_at, pay_time)`（§5.3.4）。
//
// ATM 的 webhook 通常沒有時間欄位 ⇒ `provider_paid_at` 為 NULL 是正常值不是異常，
// COALESCE 必須在這裡做完，不能讓呼叫端各自決定要不要 fallback。
func (f PaymentFacts) PaidAt() time.Time {
	if f.ProviderPaidAt != nil && !f.ProviderPaidAt.IsZero() {
		return f.ProviderPaidAt.UTC()
	}
	return f.PayTime.UTC()
}

// OrderFactsClient 讀訂單發票快照，並在同一次呼叫內完成載具解析。
//
// buyerID **只能取自訂單**（order.paid payload 的 buyer_id，其值由 order-service
// 自 orders.buyer_id 寫出）：它是 I-47 歸屬驗證的述詞之一。把它放進簽章而不是讓
// adapter 自己去撈，是為了讓「buyer_id 從哪來」在型別上看得見。
type OrderFactsClient interface {
	GetOrderInvoiceSnapshot(ctx context.Context, orderNo string, buyerID int64) (OrderFacts, error)
}

// PaymentFactsClient 讀付款事實。
type PaymentFactsClient interface {
	GetPaymentFacts(ctx context.Context, orderNo string) (PaymentFacts, error)
}

// RefundFinalized 是 C2 套用一則退款所需的全部輸入。
//
// 🔴 **這個 struct 是 payload 與業務出口之間唯一的橋**：payload 上有的欄位若不放進來，
// `ApplyRefund` 就永遠看不到它，而缺席是完全靜默的（不編譯錯、不測試紅）。
// §6.3.5.4 步驟 3 逐字：「`RECEIVED` ⇒ 依 `cumulative_refund_amount` vs `invoiced_total`
// 決定 VOID 或 ALLOWANCE」——少了 CumulativeRefundAmount，那一步沒有輸入可判。
//
// 刻意**不**收 `store_id` / `buyer_id` / `event_id` / `correlation_id`：
//   - store 與 buyer 由 `doc`（invoice_documents ⋈ invoice_issuers）決定，事件上那兩欄
//     若與 document 不一致，權威是 document 不是事件
//   - event_id 不得成為任何判斷依據（RefundSyncWorker 每小時換一次）
type RefundFinalized struct {
	OrderNo      string
	RefundRef    string
	RefundSource string

	// RefundOutcome ＝ FULL_REFUND / PARTIAL_REFUND，**僅 AFTERSALE_CASE 來源有值**
	// （E2 自動退款與 E3 賣家取消出貨為空字串）。
	//
	// ⚠️ 它**不能**單獨拿來判「要不要作廢整張發票」：每張新售後單的 refundable_amount
	// 都取自 orders.total_actual_amount、不扣既退 ⇒ 「先 PARTIAL 200、再開一張單 FULL」
	// 是合法路徑，而那一則的 FULL_REFUND 不代表歸零（§6.3.2.3 U9）。全額與否一律用
	// CumulativeRefundAmount 對帳。
	RefundOutcome string

	// RefundAmount 是**這一則**退款的金額（元，整數）。
	RefundAmount int64

	// OrderTotalActualAmount 是訂單實付總額，用來判斷 CumulativeRefundAmount 是否溢出
	// （§6.3.2.3：可能 > 它，此時 clamp 並遞增 invoice_refund_exceeds_invoiced_total）。
	OrderTotalActualAmount int64

	// CumulativeRefundAmount 是 order-service 在同一交易內以
	// `sumAmountByType(order_no, REFUND_ORIGINAL)` 算出的累計退款。
	// 🔴 它是 VOID vs ALLOWANCE 的判準（§6.3.5.4 步驟 3），不是參考值。
	CumulativeRefundAmount int64

	// CaseNo／CancelID 兩者至多一個有值，依 RefundSource 而定（AFTERSALE_CASE ⇒ CaseNo，
	// SELLER_CANCEL ⇒ CancelID，AUTO_REFUND ⇒ 兩者皆空）。
	CaseNo   string
	CancelID string

	// PaymentMethod 是訂單的付款方式，折讓單的稽核欄位。
	PaymentMethod string

	// RefundedAt 是 order-service 認定的「退款落定」時刻。
	// 與 OccurredAt 目前同值（同一個 nowIso），但語意不同：前者是業務事實時間、
	// 後者是事件時間，不得互相冒充（紅線 21）。
	RefundedAt time.Time

	OccurredAt time.Time
}

// RefundDisposition 是「這則退款不需要任何發票調整」的處置（§6.3.5.4 步驟 3）。
//
// 🔴 它與「處理完成」是**兩件不同的事**：PROCESSED 說「業務動作做完了」，
// SKIPPED 說「刻意不做，理由是 X」。合併成 return nil 之後，事後查
// 「為什麼這筆退款沒有折讓單」只能靠猜——而那正是 invoice_inbox_events 存在的理由。
type RefundDisposition struct {
	// Disposition 是 CodeMeta InvoiceInboxDisposition 的值（可 GROUP BY）。
	Disposition string
	// Reason 是給人看的細節，落 disposition_reason。
	Reason string
}

// AdjustmentApplier 是 `order.refund.finalized` 落到作廢／折讓的業務出口（§7.2.2）。
//
// 回傳值三態：nil ⇒ 建了調整（標 PROCESSED）；非 nil ⇒ 確定不需要調整（標 SKIPPED）；
// error ⇒ 這一輪沒做成（標 FAILED）。
type AdjustmentApplier interface {
	ApplyRefund(ctx context.Context, in RefundFinalized, doc domain.Document) (*RefundDisposition, error)
}

// IssuancePreparer 把 handler 對 usecase 的依賴收斂成一個方法。
//
// 用窄介面而不是直接吃 *usecase.Issuance：handler 的測試不需要 TxManager、
// 不需要 13 個 repository、不需要 AMEGO client。
type IssuancePreparer interface {
	Prepare(ctx context.Context, in usecase.OrderPaidInput) (usecase.PrepareResult, error)
}

// ---------------------------------------------------------------------------
// 尚未落地的預設實作：一律 fail-loud
// ---------------------------------------------------------------------------

// UnavailableOrderFactsClient 在 peer client 落地前佔位。
type UnavailableOrderFactsClient struct{}

// GetOrderInvoiceSnapshot 一律回 ErrPeerClientUnavailable。
func (UnavailableOrderFactsClient) GetOrderInvoiceSnapshot(context.Context, string, int64) (OrderFacts, error) {
	return OrderFacts{}, ErrPeerClientUnavailable
}

// UnavailablePaymentFactsClient 在 peer client 落地前佔位。
type UnavailablePaymentFactsClient struct{}

// GetPaymentFacts 一律回 ErrPeerClientUnavailable。
func (UnavailablePaymentFactsClient) GetPaymentFacts(context.Context, string) (PaymentFacts, error) {
	return PaymentFacts{}, ErrPeerClientUnavailable
}

// UnimplementedAdjustmentApplier 在 §7.2.2 的折讓／作廢 usecase 落地前佔位。
type UnimplementedAdjustmentApplier struct{}

// ApplyRefund 一律回 ErrAdjustmentNotImplemented。
func (UnimplementedAdjustmentApplier) ApplyRefund(
	context.Context, RefundFinalized, domain.Document,
) (*RefundDisposition, error) {
	return nil, ErrAdjustmentNotImplemented
}
