package mq

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// 入向 payload（§6.3.2）
// ---------------------------------------------------------------------------

// orderPaidPayload 是 `order.paid` 的 wire 形狀（Java `OrderPaidPayload`，snake_case）。
//
// 只宣告本服務用得到的欄位；未知欄位由 encoding/json 忽略，這是刻意的（上游加欄不得
// 讓發票線 fail）。
//
// ⚠️ `event_type` **不在** payload 裡——它由 ObsRabbitPublisher 寫進 AMQP header。
// handler 要分支請用 routingKey（HandlerFunc 的第二個參數），不要從 payload 讀。
//
// ⚠️ TSID 以 `int64` 承接沒有精度問題：encoding/json 只有在解進 `map[string]any` /
// `any` 時才用 float64。目標欄位是 typed int64 時走精確解析。
type orderPaidPayload struct {
	EventID               string `json:"event_id"`
	OrderNo               string `json:"order_no"`
	OccurredAt            string `json:"occurred_at"`
	BuyerID               int64  `json:"buyer_id"`
	StoreID               int64  `json:"store_id"`
	NotifyBuyerAccountID  int64  `json:"notify_buyer_account_id"`
	TotalActualAmount     int64  `json:"total_actual_amount"`
	ShippingMethod        string `json:"shipping_method"`
	CorrelationIDMetadata string `json:"correlation_id"`
}

// orderRefundFinalizedPayload 是 `order.refund.finalized` 的 wire 形狀（§6.3.2.3）。
//
// 🔴 **每一欄都是 `string`，因為 producer 每一欄都是 `String`。**
// 權威是 order-service 的 `OrderRefundFinalizedPayload`（record，16 個 component 全部
// `String`，class 上帶 `@JsonNaming(SnakeCaseStrategy)`）；設計書 §6.3.2.3 逐字寫
// 「金額與 TSID 一律 string——`OrderPaidPayload` 用 `Long` 是既有債，新事件不跟進」。
//
// 這個 struct 曾把 `store_id` / `buyer_id` / `refund_amount` 宣告成 `int64`：
// `json.Unmarshal` 對 `"2685914682"` → `int64` 直接回錯，而當時的慣例①把錯誤 ACK 丟棄
// ⇒ **每一則退款事件在 5.4 秒內被吃掉 40 則、DB 零痕跡**。所以「型別對齊 wire」不是
// 風格問題：它是這條線唯一能收到訊息的前提。要數字請走 parseWireAmount（在 struct
// 之外解析、失敗 fail-loud），不要把型別寫回 struct 上。
//
// 欄位是**全 16 欄**不是 9 欄。少宣告的那 7 欄不會報錯、不會測試紅——它們只是
// 「consumer 永遠看不到 refund_outcome / cumulative_refund_amount」，而後者正是
// §6.3.5.4 步驟 3 決定「作廢還是折讓」的那個值。
//
// `refund_ref` ＝ `financial_logs.external_txn_id`，是「這筆退款」的全域唯一識別。
// 🔴 **絕對不要退回 `event_id` 當冪等鍵**：aftersales 的 `RefundSyncWorker` 每小時
// 把任何 1 小時前的 DONE refund task 重發一次、每次換一個新 UUID ⇒ 用 event_id
// 去重等於每小時對同一筆退款開一張新折讓單。
//
// Java 端把「沒有值」寫成 JSON `null`（`blankToNull`）；`encoding/json` 對 `null` →
// `string` 是 no-op（保持零值 ""），不是錯誤 ⇒ `case_no` / `cancel_id` / `refund_outcome`
// / `correlation_id` 為空是正常值。
type orderRefundFinalizedPayload struct {
	EventID                string `json:"event_id"`
	OrderNo                string `json:"order_no"`
	OccurredAt             string `json:"occurred_at"`
	StoreID                string `json:"store_id"`
	BuyerID                string `json:"buyer_id"`
	RefundRef              string `json:"refund_ref"`
	RefundSource           string `json:"refund_source"`
	RefundOutcome          string `json:"refund_outcome"`
	RefundAmount           string `json:"refund_amount"`
	OrderTotalActualAmount string `json:"order_total_actual_amount"`
	CumulativeRefundAmount string `json:"cumulative_refund_amount"`
	CaseNo                 string `json:"case_no"`
	CancelID               string `json:"cancel_id"`
	PaymentMethod          string `json:"payment_method"`
	RefundedAt             string `json:"refunded_at"`
	CorrelationIDMetadata  string `json:"correlation_id"`
}

// orderCancelledPayload 是 `order.cancelled` 的 wire 形狀（§6.3.2.4，#2904）。
//
// 🔴 **`buyer_id` 是 `int64` 而不是 `string` —— 這與正上方的 refund payload 相反，
// 而兩者都是對的。** 權威是 producer：order-service 的 `OrderCancelledPayload`
// （record，帶 `@JsonNaming(SnakeCaseStrategy)`）把 `buyerId` 宣告成 **`Long`**
// ⇒ wire 上是 JSON 數字；`OrderRefundFinalizedPayload` 的 16 個 component 全是
// `String` ⇒ wire 上是字串。同一個 producer 服務、同一個欄名、相反的答案。
//
// ⚠️ 所以「本 repo 的 payload 一律 string」這種推論在這裡會直接把 consumer 弄壞：
// 宣告成 string 時 `json.Unmarshal` 對**每一則**訊息回錯。判別方式只有一個——
// 去讀 producer，而 producer_contract_test.go 就是把那件事寫成測試。
//
// 🔴 **`items` 刻意不宣告。** 它是 `List<StockItemPayload>`（巢狀 record，
// `sku_id` / `quantity` 兩欄），發票線完全用不到（庫存歸還是 product-service 的事）。
// 宣告一個用不到的巢狀型別只會多一個「上游改它就可能解析失敗」的接觸面。
// 未宣告的欄位由 encoding/json 忽略，而「為什麼不讀」逐欄登記在
// producer_contract_test.go 的 deliberatelyIgnored ——沒登記那支測試會紅。
//
// ⚠️ `occurred_at` 的格式與 C1 同一種、與 C2 不同：producer 寫的是
// `DateTimeUtils.nowUtc().toString()`（`LocalDateTime.toString()`）⇒ **無 Z、無時區**，
// 秒與奈秒同為 0 時連秒都省略。只用 RFC3339 解必定失敗 ⇒ 一律走 parseEventTime。
//
// ⚠️ `cancel_reason` 可能是 JSON `null`（買家自願取消未付款訂單時 producer 傳 null）。
// `encoding/json` 對 `null` → `string` 是 no-op（保持 ""），不是錯誤 ⇒ 空字串是正常值，
// **不得**拿它當 fail-loud 的依據。
type orderCancelledPayload struct {
	EventID      string `json:"event_id"`
	OrderNo      string `json:"order_no"`
	OccurredAt   string `json:"occurred_at"`
	BuyerID      int64  `json:"buyer_id"`
	CancelReason string `json:"cancel_reason"`
}

// ---------------------------------------------------------------------------
// wire string → int64（§6.3.5.4 慣例②：業務欄位解析失敗 → error，禁止吞成零值）
// ---------------------------------------------------------------------------

// parseWireAmount 解析一個「元的整數」金額欄。
//
// 🔴 空字串**不是** 0。三個金額欄在 producer 端都走 `String.valueOf(long)` ⇒ 必然非空；
// 收到空的代表 producer 換了形狀或 payload 被截斷，而把它讀成 0 的後果是
// `cumulative_refund_amount = 0` ⇒ §6.3.5.4 步驟 3 判成「沒退過」⇒ 該退的錢不作廢也不折讓，
// 且沒有任何錯誤訊號。要有訊號就只能在這裡回 error。
//
// 負值同樣擋掉：退款金額為負是資料錯誤，讓它往下走會產生一張負額折讓單。
func parseWireAmount(field, raw string) (int64, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return 0, fmt.Errorf("invoice: %s is empty (wire 上必為十進位整數字串)", field)
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invoice: %s %q is not an integer: %w", field, raw, err)
	}
	if v < 0 {
		return 0, fmt.Errorf("invoice: %s %q is negative", field, raw)
	}
	return v, nil
}

// toRefundFinalized 是 payload → 業務出口輸入的**唯一**轉換點。
//
// 🔴 兩條路徑都必須走它：handler 直接處理（RefundFinalizedConsumer.Handle）與亂序後的
// 反向認領（OrderPaidConsumer.refundFromInbox）。兩份各自組裝的話，新增一個 payload 欄位
// 只會被接上其中一條——而漏掉的那條是「退款先到、付款後到」的路徑，正是最難在測試裡
// 想起來的那一條。
//
// occurredAt 由呼叫端傳入而不是在這裡解析：反向認領路徑的權威是 inbox 列上的
// `occurred_at`（落庫當下解析過的值），不是 payload 字串（可能已被 90 天保留規則覆寫）。
func (p orderRefundFinalizedPayload) toRefundFinalized(occurredAt time.Time) (RefundFinalized, error) {
	refundAmount, err := parseWireAmount("refund_amount", p.RefundAmount)
	if err != nil {
		return RefundFinalized{}, err
	}
	orderTotal, err := parseWireAmount("order_total_actual_amount", p.OrderTotalActualAmount)
	if err != nil {
		return RefundFinalized{}, err
	}
	cumulative, err := parseWireAmount("cumulative_refund_amount", p.CumulativeRefundAmount)
	if err != nil {
		return RefundFinalized{}, err
	}
	// refunded_at 與 occurred_at 目前同值，但語意不同（業務事實時間 vs 事件時間），
	// 不得互相冒充 ⇒ 各自解析、各自 fail-loud。
	refundedAt, err := parseEventTime(p.RefundedAt)
	if err != nil {
		return RefundFinalized{}, fmt.Errorf("refunded_at: %w", err)
	}
	return RefundFinalized{
		OrderNo:                p.OrderNo,
		RefundRef:              p.RefundRef,
		RefundSource:           p.RefundSource,
		RefundOutcome:          p.RefundOutcome,
		RefundAmount:           refundAmount,
		OrderTotalActualAmount: orderTotal,
		CumulativeRefundAmount: cumulative,
		CaseNo:                 p.CaseNo,
		CancelID:               p.CancelID,
		PaymentMethod:          p.PaymentMethod,
		RefundedAt:             refundedAt,
		OccurredAt:             occurredAt,
	}, nil
}

// ---------------------------------------------------------------------------
// 冪等鍵（§6.3.5.2）
// ---------------------------------------------------------------------------

const (
	// dedupPrefixOrderPaid ＋ order_no ＝ C1 的 business_dedup_key。
	// 用 order_no 而不是 event_id：一張訂單只會發一次 paid（0→10 CAS 守門），
	// 而 order_no 連「未來多加一個 publisher」都擋得住，成本為零。
	dedupPrefixOrderPaid = "ORDER_PAID:"
	// dedupPrefixRefund ＋ refund_ref ＝ C2 的 business_dedup_key。
	dedupPrefixRefund = "REFUND:"

	// dedupPrefixOrderCancelled ＋ order_no ＝ C3 的 business_dedup_key（#2904）。
	//
	// 用 order_no 而不是 event_id，理由與 C1 同一條、但在這裡更硬：`order.cancelled`
	// 走 order_outbox，而 outbox 的重投**保留原 event_id**（`OrderOutboxPublisher`
	// 的 enrich 逐一用完整建構子把 `core.eventId()` 帶回去，註解逐字寫「跨重試冪等鍵
	// 穩定」）⇒ event_id 在重投時是穩定的。即使如此仍用 order_no：一張訂單只會被
	// 取消一次，而 order_no 連「未來多一個 publisher」都擋得住，成本為零。
	//
	// 🔴 前綴必須與 C1 不同。`uq_invinbox_business_key` 是 (event_type, business_dedup_key)
	// 複合唯一鍵 ⇒ 光靠 event_type 不同就已經不會撞；前綴是給人看的第二道，
	// 讓 ADM 在表上一眼看得出這一列是哪一種事件，不必回頭查 event_type。
	dedupPrefixOrderCancelled = "ORDER_CANCELLED:"

	// dedupKeyMaxLen 是 invoice_inbox_events.business_dedup_key 的欄寬（varchar(160)，§6.3.5.3）。
	// 超長不是 CHECK violation 而是**靜默截斷後撞錯鍵**，所以在寫入前 fail-loud。
	dedupKeyMaxLen = 160
)

func orderPaidDedupKey(orderNo string) string { return dedupPrefixOrderPaid + orderNo }

func refundDedupKey(refundRef string) string { return dedupPrefixRefund + refundRef }

func orderCancelledDedupKey(orderNo string) string { return dedupPrefixOrderCancelled + orderNo }

// checkDedupKey 在寫入前擋掉超長鍵。回 error ⇒ handler fail-loud ⇒ DLQ（看得見）。
func checkDedupKey(key string) error {
	if len(key) > dedupKeyMaxLen {
		return fmt.Errorf("invoice: business_dedup_key %q exceeds %d bytes", key, dedupKeyMaxLen)
	}
	return nil
}

// ---------------------------------------------------------------------------
// occurred_at 的解析：兩條 queue 格式不同（§6.3.2.2 陷阱表第 1 列）
// ---------------------------------------------------------------------------

// eventTimeLayouts 由嚴到寬排列。
//
// C2（`order.refund.finalized`）走 Java 的 formatIso8601 ⇒ 帶 `Z`、RFC3339 可解。
// C1（`order.paid`）走 `LocalDateTime.toString()` ⇒ **無 Z、無時區**，而且秒與奈秒
// 同時為 0 時連秒都省略（`2026-08-25T09:41`）⇒ 只用 RFC3339 解**必定失敗**。
//
// 無時區的那三個 layout 一律以 UTC 解讀：上游的 LocalDateTime 值語意就是 UTC
// （DateTimeUtils.nowUtc），不是 server local time。
var eventTimeLayouts = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02T15:04:05.999999999",
	"2006-01-02T15:04:05",
	"2006-01-02T15:04",
}

// parseEventTime 解析 occurred_at。
//
// 🔴 解析失敗**必須** fail-loud（回 error → retry → DLQ），不得吞成零值：occurred_at
// 是 issue_not_before 與 statutory_deadline_at 的計算基準，吞成零值 ⇒ 期限落在 1970
// ⇒ due-date worker 立刻判定逾期，而且沒有任何錯誤訊號。
func parseEventTime(raw string) (time.Time, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return time.Time{}, fmt.Errorf("invoice: occurred_at is empty")
	}
	for _, layout := range eventTimeLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("invoice: occurred_at %q matches none of %d known layouts", s, len(eventTimeLayouts))
}
