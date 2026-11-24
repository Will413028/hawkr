package paymentrelay

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/legacy/rabbitmq"
	"github.com/Will413028/hawkr/backend/internal/modules/payment"
)

type recordingJobs struct {
	args []river.JobArgs
	opts []*river.InsertOpts
}

func (q *recordingJobs) Enqueue(_ context.Context, args river.JobArgs, opts *river.InsertOpts) error {
	q.args = append(q.args, args)
	q.opts = append(q.opts, opts)
	return nil
}

var (
	occurred = time.Date(2026, 11, 24, 17, 30, 15, 123456000, time.FixedZone("TST", 8*3600))
	// wire 上的時間是 Java LocalDateTime.toString()：UTC、沒有時區後綴。
	wireTime = "2026-11-24T09:30:15.123456"
	paidAt   = time.Date(2026, 11, 24, 17, 29, 0, 0, time.FixedZone("TST", 8*3600))
)

// run 把模組事件交給 OnX，取出排入的唯一一個 PublishArgs，回傳它與解成 map 的 body（解成 map[string]any，所以 string 與 number 分得清楚）。
func run[E any](t *testing.T, on func(*recordingJobs) func(context.Context, E) error, event E) (rabbitmq.PublishArgs, map[string]any) {
	t.Helper()
	q := &recordingJobs{}
	if err := on(q)(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	if len(q.args) != 1 {
		t.Fatalf("enqueued %d jobs, want 1", len(q.args))
	}
	a, ok := q.args[0].(rabbitmq.PublishArgs)
	if !ok {
		t.Fatalf("job type = %T", q.args[0])
	}
	if q.opts[0] == nil || q.opts[0].MaxAttempts != rabbitmq.InsertOpts().MaxAttempts {
		t.Fatalf("publish opts = %+v", q.opts[0])
	}
	var payload map[string]any
	if err := json.Unmarshal(a.Body, &payload); err != nil {
		t.Fatal(err)
	}
	if _, err := uuid.Parse(a.EventID); err != nil || a.EventID != payload["event_id"] {
		t.Fatalf("PublishArgs.EventID = %q, body event_id = %v: want the same UUID", a.EventID, payload["event_id"])
	}
	return a, payload
}

func onPayment(q *recordingJobs) func(context.Context, payment.PaymentChanged) error {
	return OnPaymentChanged(q)
}

func onRefund(q *recordingJobs) func(context.Context, payment.RefundChanged) error {
	return OnRefundChanged(q)
}

func check(t *testing.T, a rabbitmq.PublishArgs, payload map[string]any, key string, want map[string]any) {
	t.Helper()
	if a.Exchange != "payment.events" || a.RoutingKey != key {
		t.Errorf("published to %s/%s, want payment.events/%s", a.Exchange, a.RoutingKey, key)
	}
	want["event_id"] = a.EventID
	if !reflect.DeepEqual(payload, want) {
		t.Errorf("%s payload\n got  %#v\n want %#v", key, payload, want)
	}
}

func changed(status payment.PaymentStatus) payment.PaymentChanged {
	return payment.PaymentChanged{
		OccurredAt: occurred, PaymentID: uuid.NewV7(), BuyerID: 7, Amount: 350, Provider: "stripe",
		Orders: []payment.PaymentOrderReceivable{{ID: uuid.NewV7(), OrderNo: "O20261124001", StoreID: 91, Amount: 350}},
		Status: status, Previous: payment.PaymentAwaitingPayment,
	}
}

// 一筆付款對 N 張訂單應收：舊訊息以訂單編號為鍵，每張訂單各送一則，金額是那張訂單的應收（不是付款總額），事件 ID 各自不同。
// mutation：OnPaymentChanged 只送第一張訂單 → 這個測試失敗。
func TestAPaymentOfTwoOrdersRelaysOneMessagePerOrderWithItsOwnAmount(t *testing.T) {
	t.Parallel()
	e := changed(payment.PaymentSucceeded)
	e.PaidAt = &paidAt
	e.Amount = 350
	e.Orders = []payment.PaymentOrderReceivable{{OrderNo: "OA", StoreID: 91, Amount: 100}, {OrderNo: "OB", StoreID: 92, Amount: 250}}
	q := &recordingJobs{}
	if err := OnPaymentChanged(q)(context.Background(), e); err != nil {
		t.Fatal(err)
	}
	if len(q.args) != 2 {
		t.Fatalf("enqueued %d jobs, want one per order", len(q.args))
	}
	seen := map[string]float64{}
	ids := map[string]bool{}
	for _, a := range q.args {
		pa, ok := a.(rabbitmq.PublishArgs)
		if !ok {
			t.Fatalf("job type = %T", a)
		}
		var payload map[string]any
		if err := json.Unmarshal(pa.Body, &payload); err != nil {
			t.Fatal(err)
		}
		seen[payload["order_id"].(string)] = payload["amount"].(float64)
		ids[pa.EventID] = true
	}
	if seen["OA"] != 100 || seen["OB"] != 250 || len(ids) != 2 {
		t.Fatalf("per-order amounts = %v, distinct event ids = %d", seen, len(ids))
	}
}

func TestAPaymentWithoutOrdersIsRefusedInsteadOfRelayingNothing(t *testing.T) {
	t.Parallel()
	e := changed(payment.PaymentSucceeded)
	e.Orders = nil
	if err := OnPaymentChanged(&recordingJobs{})(context.Background(), e); err == nil {
		t.Fatal("a payment without order receivables must not be silently dropped")
	}
}

func TestPaymentSuccessWire(t *testing.T) {
	t.Parallel()
	e := changed(payment.PaymentSucceeded)
	e.PaidAt = &paidAt
	a, got := run(t, onPayment, e)
	check(t, a, got, "payment.success", map[string]any{
		"event_type": "payment.success", "aggregate_type": "payment", "aggregate_id": "O20261124001", "occurred_at": wireTime,
		"trade_no": "O20261124001", "order_id": "O20261124001", "buyer_id": "7", "amount": float64(350),
		"provider_trade_no": "", "payment_type": "", "pay_time": "2026-11-24T09:29:00",
	})
}

func TestPaymentFailedWire(t *testing.T) {
	t.Parallel()
	e := changed(payment.PaymentCreateFailed)
	e.LastError = "amount_too_small"
	a, got := run(t, onPayment, e)
	check(t, a, got, "payment.failed", map[string]any{
		"event_type": "payment.failed", "aggregate_type": "payment", "aggregate_id": "O20261124001", "occurred_at": wireTime,
		"trade_no": "O20261124001", "order_id": "O20261124001", "buyer_id": "7", "amount": float64(350), "provider_message": "amount_too_small",
	})
}

func TestPaymentClosedWire(t *testing.T) {
	t.Parallel()
	a, got := run(t, onPayment, changed(payment.PaymentCanceled))
	check(t, a, got, "payment.closed", map[string]any{
		"event_type": "payment.closed", "aggregate_type": "payment", "aggregate_id": "O20261124001", "occurred_at": wireTime,
		"trade_no": "O20261124001", "order_id": "O20261124001", "buyer_id": "7", "amount": float64(350), "closed_at": wireTime,
	})
}

func refundChanged(status payment.PaymentRefundStatus) payment.RefundChanged {
	return payment.RefundChanged{
		OccurredAt: occurred, RefundID: uuid.NewV7(), PaymentID: uuid.NewV7(), OrderNo: "O20261124001", StoreID: 91, BuyerID: 7, Amount: 120,
		RequestKey: "C77-0", Status: status, Previous: payment.RefundRefunding,
	}
}

func TestRefundSucceededWire(t *testing.T) {
	t.Parallel()
	a, got := run(t, onRefund, refundChanged(payment.RefundSucceeded))
	check(t, a, got, "refund.succeeded", map[string]any{
		"event_type": "refund.succeeded", "aggregate_type": "refund", "aggregate_id": "C77-0", "occurred_at": wireTime,
		"refundNo": "C77-0", "tradeNo": "O20261124001", "orderId": "O20261124001", "amount": float64(120),
		"providerRefundNo": "", "payuniStatus": "", "payuniMessage": "", "estimatedRefundDate": "", "bankDate": "",
		"completedAt": wireTime, "occurredAt": wireTime,
	})
}

func TestRefundFailedWire(t *testing.T) {
	t.Parallel()
	e := refundChanged(payment.RefundFailed)
	e.FailureReason = "provider:expired_or_canceled_card"
	a, got := run(t, onRefund, e)
	check(t, a, got, "refund.failed", map[string]any{
		"event_type": "refund.failed", "aggregate_type": "refund", "aggregate_id": "C77-0", "occurred_at": wireTime,
		"refundNo": "C77-0", "tradeNo": "O20261124001", "orderId": "O20261124001", "amount": float64(120),
		"payuniStatus": "", "payuniMessage": "provider:expired_or_canceled_card", "occurredAt": wireTime,
	})
}

// 不是終局或不是舊事件的狀態不轉送；不認得的狀態是錯誤，不靜默略過。
func TestNonTerminalAndUnknownStatuses(t *testing.T) {
	t.Parallel()
	for _, s := range []payment.PaymentStatus{payment.PaymentCreating, payment.PaymentAwaitingPayment, payment.PaymentProcessing, payment.PaymentAnomaly} {
		q := &recordingJobs{}
		if err := OnPaymentChanged(q)(context.Background(), changed(s)); err != nil || len(q.args) != 0 {
			t.Errorf("%s: err=%v jobs=%d, want none", s, err, len(q.args))
		}
	}
	for _, s := range []payment.PaymentRefundStatus{payment.RefundPending, payment.RefundRefunding, payment.RefundAnomaly} {
		q := &recordingJobs{}
		if err := OnRefundChanged(q)(context.Background(), refundChanged(s)); err != nil || len(q.args) != 0 {
			t.Errorf("%s: err=%v jobs=%d, want none", s, err, len(q.args))
		}
	}
	if err := OnPaymentChanged(&recordingJobs{})(context.Background(), changed("")); err == nil {
		t.Error("an empty payment status must be an error")
	}
	if err := OnRefundChanged(&recordingJobs{})(context.Background(), refundChanged("MYSTERY")); err == nil {
		t.Error("an unknown refund status must be an error")
	}
}

// 每則事件各自一個 event_id：同一個狀態變化發兩次是兩則事件（模組事件沒有自己的 ID）。
func TestEveryEventGetsItsOwnID(t *testing.T) {
	t.Parallel()
	q := &recordingJobs{}
	for range 2 {
		if err := OnPaymentChanged(q)(context.Background(), changed(payment.PaymentCanceled)); err != nil {
			t.Fatal(err)
		}
	}
	if q.args[0].(rabbitmq.PublishArgs).EventID == q.args[1].(rabbitmq.PublishArgs).EventID {
		t.Fatal("two events share an event id")
	}
}

// 送出的每個 routing key 都要是 events.csv 登記的 payment-service 事件（exchange payment.events），而且沒有標成 removed。
func TestRoutingKeysMatchEventsCSV(t *testing.T) {
	t.Parallel()
	f, err := os.Open("../../../docs/plans/2026-09-24-go-modular-monolith/events.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			t.Error(err)
		}
	}()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	live := map[string]bool{}
	for _, r := range rows[1:] {
		if r[1] == "payment.events" && r[2] == "payment-service" && !strings.HasPrefix(r[8], "removed") {
			live[r[0]] = true
		}
	}
	paid := paidAt
	sent := map[string]string{}
	for _, tc := range []struct {
		a rabbitmq.PublishArgs
	}{
		{func() rabbitmq.PublishArgs {
			e := changed(payment.PaymentSucceeded)
			e.PaidAt = &paid
			a, _ := run(t, onPayment, e)
			return a
		}()},
		{func() rabbitmq.PublishArgs { a, _ := run(t, onPayment, changed(payment.PaymentCreateFailed)); return a }()},
		{func() rabbitmq.PublishArgs { a, _ := run(t, onPayment, changed(payment.PaymentCanceled)); return a }()},
		{func() rabbitmq.PublishArgs {
			a, _ := run(t, onRefund, refundChanged(payment.RefundSucceeded))
			return a
		}()},
		{func() rabbitmq.PublishArgs { a, _ := run(t, onRefund, refundChanged(payment.RefundFailed)); return a }()},
	} {
		sent[tc.a.RoutingKey] = tc.a.Exchange
	}
	for key, exchange := range sent {
		if !live[key] || exchange != "payment.events" {
			t.Errorf("%s on %s is not a live payment-service event in events.csv", key, exchange)
		}
	}
	if len(sent) != 5 {
		t.Errorf("relay sent %d distinct routing keys, want 5: %v", len(sent), sent)
	}
}
