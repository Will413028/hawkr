// Package paymentrelay 將 payment 模組事件轉成尚未搬遷的 Java order 與 aftersales 所需的 RabbitMQ 訊息（B2 步驟 4，之後到 B8、B9 隨它們搬進單體而退場）。
//
// 每個轉送的模組事件在發布事件的 transaction 內直接排入一個 rabbitmq.PublishArgs（transactional outbox）：payload 只由事件決定、不需要讀別的模組，
// 所以沒有第二段 worker。排入失敗時發布的寫入一起回滾；EventID 是排入當下產生的 UUIDv7，同時是 body 的 event_id 與 River 的 unique 鍵。
// 模組事件沒有自己的 ID：一個事件只排一次，所以不需要穩定的 ID；wire 與舊 publisher（payment-service 的 PaymentEventPublisher 與 PaymentDomainEvent）逐欄位相同，
// 差別只有 event_id（舊的是 UUID v4）、occurred_at 的精度與下面列出的欄位語意。
//
// 轉送：
//   - PaymentChanged 的 SUCCEEDED → payment.success、CREATE_FAILED → payment.failed、CANCELED → payment.closed（exchange payment.events，給 order）。
//   - RefundChanged 的 SUCCEEDED → refund.succeeded、FAILED → refund.failed（給 aftersales 與 order）。
//
// 不轉送：
//   - payment.expired：模組只有 CANCELED，不區分逾期與主動關閉；兩者在 Java order 的消費端都只記 log（逾期取消由 order 自己的排程負責），所以以 payment.closed 代表。
//   - payment.conflict、payment.rescue.*：隨 PayUni 的 CONFLICT 與 rescue 刪除（D5 以訂單一方發起的自動退款取代）；自動退款完成的通知由 A8 的 notification 訂閱 AutoRefundCompleted。
//   - MoneyAnomaly：給告警，不是舊服務的事件。
//
// 欄位語意：舊契約的 trade_no 與 order_id 都是訂單編號（一張訂單一筆付款，D8）；退款的 refundNo 是呼叫端建立退款時給的冪等鍵（aftersales 以它對回退款任務）；
// payment_type 是空字串（付款沒有付款方式欄位，order 沿用訂單上的）；provider_trade_no、payuniStatus 與 ATM 日期是空字串（PayUni 專屬或 port 不外露）。
package paymentrelay

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/legacy/rabbitmq"
	"github.com/Will413028/hawkr/backend/internal/modules/payment"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

const exchangePayment = "payment.events"

// aggregateType 是舊的 PaymentAggregateType。
const (
	aggregatePayment = "payment"
	aggregateRefund  = "refund"
)

// localDateTime 是 Java LocalDateTime.toString() 的格式（UTC、沒有時區後綴）：消費端以 LocalDateTime.parse 讀。
const localDateTime = "2006-01-02T15:04:05.999999999"

func at(t time.Time) string { return t.UTC().Format(localDateTime) }

// envelope 是舊 OutboxEventFactory.applyEnvelopeMetadata 補進每則 payload 的五個欄位。
type envelope struct {
	EventID       string `json:"event_id"`
	EventType     string `json:"event_type"`
	AggregateType string `json:"aggregate_type"`
	AggregateID   string `json:"aggregate_id"`
	OccurredAt    string `json:"occurred_at"`
}

// paymentSuccess 是舊 PaymentDomainEvent.PaymentSucceeded 的 payload。
type paymentSuccess struct {
	envelope
	TradeNo         string `json:"trade_no"`
	OrderID         string `json:"order_id"`
	BuyerID         string `json:"buyer_id"`
	Amount          int64  `json:"amount"`
	ProviderTradeNo string `json:"provider_trade_no"`
	PaymentType     string `json:"payment_type"`
	PayTime         string `json:"pay_time"`
}

// paymentFailed 是舊 PaymentFailed 的 payload；provider_message 是最近一次嘗試的失敗原因碼。
type paymentFailed struct {
	envelope
	TradeNo         string `json:"trade_no"`
	OrderID         string `json:"order_id"`
	BuyerID         string `json:"buyer_id"`
	Amount          int64  `json:"amount"`
	ProviderMessage string `json:"provider_message"`
}

// paymentClosed 是舊 PaymentClosed 的 payload。
type paymentClosed struct {
	envelope
	TradeNo  string `json:"trade_no"`
	OrderID  string `json:"order_id"`
	BuyerID  string `json:"buyer_id"`
	Amount   int64  `json:"amount"`
	ClosedAt string `json:"closed_at"`
}

// refundSucceeded 是舊 RefundSucceeded 的 payload（camelCase）。
type refundSucceeded struct {
	envelope
	RefundNo            string `json:"refundNo"`
	TradeNo             string `json:"tradeNo"`
	OrderID             string `json:"orderId"`
	Amount              int64  `json:"amount"`
	ProviderRefundNo    string `json:"providerRefundNo"`
	PayuniStatus        string `json:"payuniStatus"`
	PayuniMessage       string `json:"payuniMessage"`
	EstimatedRefundDate string `json:"estimatedRefundDate"`
	BankDate            string `json:"bankDate"`
	CompletedAt         string `json:"completedAt"`
	OccurredAtCamel     string `json:"occurredAt"`
}

// refundFailed 是舊 RefundFailed 的 payload（camelCase，沒有 status：消費端依 routing key 判斷成敗）；payuniMessage 是失敗原因。
type refundFailed struct {
	envelope
	RefundNo        string `json:"refundNo"`
	TradeNo         string `json:"tradeNo"`
	OrderID         string `json:"orderId"`
	Amount          int64  `json:"amount"`
	PayuniStatus    string `json:"payuniStatus"`
	PayuniMessage   string `json:"payuniMessage"`
	OccurredAtCamel string `json:"occurredAt"`
}

func env(id, eventType, aggregateType, aggregateID string, occurred time.Time) envelope {
	return envelope{EventID: id, EventType: eventType, AggregateType: aggregateType, AggregateID: aggregateID, OccurredAt: at(occurred)}
}

// OnPaymentChanged 在發布 PaymentChanged 的 transaction 內把入帳、建立失敗與取消轉成舊的 payment.* 訊息；其他狀態（等待、處理中、建立中、ANOMALY）不轉送。
func OnPaymentChanged(q jobs.Enqueuer) func(context.Context, payment.PaymentChanged) error {
	return func(ctx context.Context, e payment.PaymentChanged) error {
		id := uuid.NewV7().String()
		buyer := fmt.Sprint(e.BuyerID)
		switch e.Status {
		case payment.PaymentSucceeded:
			payTime := ""
			if e.PaidAt != nil {
				payTime = at(*e.PaidAt)
			}
			return enqueuePublish(ctx, q, id, exchangePayment, "payment.success", paymentSuccess{
				envelope: env(id, "payment.success", aggregatePayment, e.OrderNo, e.OccurredAt),
				TradeNo:  e.OrderNo, OrderID: e.OrderNo, BuyerID: buyer, Amount: e.Amount, PayTime: payTime,
			})
		case payment.PaymentCreateFailed:
			return enqueuePublish(ctx, q, id, exchangePayment, "payment.failed", paymentFailed{
				envelope: env(id, "payment.failed", aggregatePayment, e.OrderNo, e.OccurredAt),
				TradeNo:  e.OrderNo, OrderID: e.OrderNo, BuyerID: buyer, Amount: e.Amount, ProviderMessage: e.LastError,
			})
		case payment.PaymentCanceled:
			return enqueuePublish(ctx, q, id, exchangePayment, "payment.closed", paymentClosed{
				envelope: env(id, "payment.closed", aggregatePayment, e.OrderNo, e.OccurredAt),
				TradeNo:  e.OrderNo, OrderID: e.OrderNo, BuyerID: buyer, Amount: e.Amount, ClosedAt: at(e.OccurredAt),
			})
		case payment.PaymentCreating, payment.PaymentAwaitingPayment, payment.PaymentProcessing, payment.PaymentAnomaly:
			return nil
		}
		return fmt.Errorf("paymentrelay：不認得的付款狀態 %q", e.Status)
	}
}

// OnRefundChanged 在發布 RefundChanged 的 transaction 內把退款的終局轉成舊的 refund.succeeded 與 refund.failed 訊息；其他狀態不轉送。
func OnRefundChanged(q jobs.Enqueuer) func(context.Context, payment.RefundChanged) error {
	return func(ctx context.Context, e payment.RefundChanged) error {
		id := uuid.NewV7().String()
		switch e.Status {
		case payment.RefundSucceeded:
			return enqueuePublish(ctx, q, id, exchangePayment, "refund.succeeded", refundSucceeded{
				envelope: env(id, "refund.succeeded", aggregateRefund, e.RequestKey, e.OccurredAt),
				RefundNo: e.RequestKey, TradeNo: e.OrderNo, OrderID: e.OrderNo, Amount: e.Amount,
				CompletedAt: at(e.OccurredAt), OccurredAtCamel: at(e.OccurredAt),
			})
		case payment.RefundFailed:
			return enqueuePublish(ctx, q, id, exchangePayment, "refund.failed", refundFailed{
				envelope: env(id, "refund.failed", aggregateRefund, e.RequestKey, e.OccurredAt),
				RefundNo: e.RequestKey, TradeNo: e.OrderNo, OrderID: e.OrderNo, Amount: e.Amount,
				PayuniMessage: e.FailureReason, OccurredAtCamel: at(e.OccurredAt),
			})
		case payment.RefundPending, payment.RefundRefunding, payment.RefundAnomaly:
			return nil
		}
		return fmt.Errorf("paymentrelay：不認得的退款狀態 %q", e.Status)
	}
}

// enqueuePublish 在呼叫方的 transaction 內排入送出的 job（transactional outbox）；PublishArgs 以 event_id 做 unique。
func enqueuePublish(ctx context.Context, q jobs.Enqueuer, eventID, exchange, key string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return q.Enqueue(ctx, rabbitmq.PublishArgs{EventID: eventID, Exchange: exchange, RoutingKey: key, Body: body}, rabbitmq.InsertOpts())
}
