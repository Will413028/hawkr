package mq

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"testing"
	"time"

	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	pkgmq "github.com/Will413028/hawkr/go-pkg/mq"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"github.com/Will413028/hawkr/invoice-service/internal/usecase"
)

// ---------------------------------------------------------------------------
// fakes
// ---------------------------------------------------------------------------

// fakeInbox 模擬 invoice_inbox_events 的兩道唯一鍵。
//
// 兩把鍵都要：uq_invinbox_event_id 擋 RabbitMQ 重投，
// uq_invinbox_business_key(event_type, business_dedup_key) 擋業務層重複。
// 只做其中一把的 fake 會讓「換一個 event_id 重送」這個真實情境測不到，
// 而那正是 aftersales RefundSyncWorker 的行為。
type fakeInbox struct {
	byEventID map[string]bool
	byBizKey  map[string]bool
	rows      []domain.InboxEvent
	nextID    int64
	inserts   int
	processed []int64
	failed    []int64
	insertErr error
}

func newFakeInbox() *fakeInbox {
	return &fakeInbox{byEventID: map[string]bool{}, byBizKey: map[string]bool{}, nextID: 1}
}

func (f *fakeInbox) Insert(_ context.Context, in domain.NewInboxEvent) (int64, bool, error) {
	if f.insertErr != nil {
		return 0, false, f.insertErr
	}
	f.inserts++
	bizKey := in.EventType + "\x00" + in.BusinessDedupKey
	if f.byEventID[in.EventID] || f.byBizKey[bizKey] {
		return 0, false, nil
	}
	f.byEventID[in.EventID] = true
	f.byBizKey[bizKey] = true
	id := f.nextID
	f.rows = append(f.rows, domain.InboxEvent{
		ID: id, EventID: in.EventID, EventType: in.EventType,
		BusinessDedupKey: in.BusinessDedupKey, AggregateRef: in.AggregateRef,
		Payload: in.Payload, Status: in.Status, OccurredAt: in.OccurredAt,
		// 🔴 NextAttemptAt 必須留下來：它是本表的**租約欄**，而「handler 還在跑的時候
		// 這一列到期了沒有」正是 INV.dl 接上之後唯一會出事的地方。不記它的 fake 會讓
		// handlerLease 那行改回 `now` 也照樣全綠（見 TestOrderPaid_InsertReservesHandlerLease）。
		NextAttemptAt: in.NextAttemptAt,
	})
	f.nextID++
	return id, true, nil
}

func (f *fakeInbox) FetchDue(context.Context, time.Time, int32) ([]domain.InboxEvent, error) {
	return nil, nil
}

// FetchDueByType 鏡射 FetchDueInboxEventsByType 的述詞：狀態三值 ＋ 型別相等 ＋ 已到期。
//
// 它不是佔位：INV.dl 的候選集合就是這個述詞，而 consumer 寫進 next_attempt_at 的值
// （now＋handlerLease）決定了一列剛落庫的事件會不會馬上被兜底掃描搶走。回 nil 的 fake
// 會讓那條不變式沒有任何測試守著。
func (f *fakeInbox) FetchDueByType(_ context.Context, eventType string, now time.Time, limit int32) ([]domain.InboxEvent, error) {
	var out []domain.InboxEvent
	for _, r := range f.rows {
		// 三個狀態逐字取自 SQL（"FAILED" 在 production code 沒有具名常數，
		// 它只出現在 MarkInboxEventFailed 的 SET 子句裡）。
		switch r.Status {
		case inboxStatusReceived, inboxStatusDeferred, "FAILED":
		default:
			continue
		}
		if r.EventType != eventType || r.NextAttemptAt.After(now) {
			continue
		}
		out = append(out, r)
		if int32(len(out)) >= limit {
			break
		}
	}
	return out, nil
}

// FetchDueActionableByType 是 FetchDueByType 的變體，**排除 DEFERRED**（#3073）。
//
// 兩者的差別只有那一格狀態，而那正是本方法存在的理由：DEFERRED 的歸宿是反向認領，
// 兜底掃描碰它會在失敗時翻成 FAILED，反向認領的述詞從此永久不成立。fake 逐字鏡射
// SQL 的狀態集合，不要用「呼叫 FetchDueByType 再過濾」實作——那會讓兩者的差異
// 消失在 fake 裡，而測試就驗不到它了。
func (f *fakeInbox) FetchDueActionableByType(_ context.Context, eventType string, now time.Time, limit int32) ([]domain.InboxEvent, error) {
	var out []domain.InboxEvent
	for _, r := range f.rows {
		switch r.Status {
		case inboxStatusReceived, "FAILED":
		default:
			continue
		}
		if r.EventType != eventType || r.NextAttemptAt.After(now) {
			continue
		}
		out = append(out, r)
		if int32(len(out)) >= limit {
			break
		}
	}
	return out, nil
}

func (f *fakeInbox) UpdateAttempt(_ context.Context, id int64, attemptCount int32, nextAttemptAt time.Time, lastError *string) (bool, error) {
	for i := range f.rows {
		if f.rows[i].ID != id {
			continue
		}
		f.rows[i].AttemptCount = attemptCount
		f.rows[i].NextAttemptAt = nextAttemptAt
		f.rows[i].LastError = lastError
		return true, nil
	}
	return false, nil
}
func (f *fakeInbox) Claim(context.Context, int64, time.Time, time.Time) (bool, error) {
	return true, nil
}
func (f *fakeInbox) MarkProcessed(_ context.Context, id int64, _ time.Time) (bool, error) {
	f.processed = append(f.processed, id)
	f.setStatus(id, "PROCESSED")
	return true, nil
}
func (f *fakeInbox) MarkSkipped(_ context.Context, id int64, _, _ string, _ time.Time) (bool, error) {
	f.setStatus(id, "SKIPPED")
	return true, nil
}
func (f *fakeInbox) MarkDeferred(_ context.Context, id int64, deferredUntil time.Time) (bool, error) {
	f.setStatus(id, inboxStatusDeferred)
	f.setNextAttempt(id, deferredUntil)
	return true, nil
}
func (f *fakeInbox) MarkFailed(_ context.Context, id int64, _ string, nextAttemptAt time.Time) (bool, error) {
	f.failed = append(f.failed, id)
	f.setStatus(id, "FAILED")
	f.setNextAttempt(id, nextAttemptAt)
	return true, nil
}

// setStatus／setNextAttempt 讓 fake 的候選集合會隨標記收縮，與 SQL 的 allowed-from 一致。
// 少了它們，一列被標成 PROCESSED 之後仍會出現在 FetchDueByType 的結果裡 ⇒ 任何
// 「標記完就不該再被撈到」的斷言都會是假綠。
func (f *fakeInbox) setStatus(id int64, status string) {
	for i := range f.rows {
		if f.rows[i].ID == id {
			f.rows[i].Status = status
			return
		}
	}
}

func (f *fakeInbox) setNextAttempt(id int64, at time.Time) {
	for i := range f.rows {
		if f.rows[i].ID == id {
			f.rows[i].NextAttemptAt = at
			return
		}
	}
}
func (f *fakeInbox) ListDeferredByAggregateRef(_ context.Context, ref string) ([]domain.InboxEvent, error) {
	var out []domain.InboxEvent
	for _, r := range f.rows {
		if r.Status == inboxStatusDeferred && r.AggregateRef != nil && *r.AggregateRef == ref {
			out = append(out, r)
		}
	}
	return out, nil
}
func (f *fakeInbox) CountStaleDeferred(context.Context, time.Time) (int64, error) { return 0, nil }
func (f *fakeInbox) PurgeProcessedPayloads(context.Context, time.Time, time.Time, int32) (int64, error) {
	return 0, nil
}

// fakeDocs 只實作 handler 用得到的那兩支，其餘回 not-implemented（呼叫到就是設計偏了）。
type fakeDocs struct {
	domain.DocumentRepository
	bySourceRef map[string]domain.Document
}

func (f *fakeDocs) GetBySourceRef(_ context.Context, ref string) (domain.Document, error) {
	if d, ok := f.bySourceRef[ref]; ok {
		return d, nil
	}
	return domain.Document{}, pkgerrors.NewBusiness(bizcode.InvoiceDocumentNotFound, "not found")
}

type fakePreparer struct {
	calls int
	res   usecase.PrepareResult
	err   error
}

func (f *fakePreparer) Prepare(context.Context, usecase.OrderPaidInput) (usecase.PrepareResult, error) {
	f.calls++
	return f.res, f.err
}

type fakeOrderFacts struct {
	calls int
	facts OrderFacts
	err   error
	// gotBuyerID 記下 consumer 傳下來的 buyer_id：I-47 的歸屬驗證靠它，
	// 而「有沒有真的把訂單上的 buyer_id 傳下去」只有記下來才驗得到。
	gotBuyerID int64
}

func (f *fakeOrderFacts) GetOrderInvoiceSnapshot(_ context.Context, _ string, buyerID int64) (OrderFacts, error) {
	f.calls++
	f.gotBuyerID = buyerID
	return f.facts, f.err
}

type fakePayFacts struct {
	facts PaymentFacts
	err   error
}

func (f *fakePayFacts) GetPaymentFacts(context.Context, string) (PaymentFacts, error) {
	return f.facts, f.err
}

type fakeApplier struct {
	calls int
	err   error
	// skip 非 nil ⇒ 模擬「確定不需要調整」，呼叫端應標 SKIPPED 而不是 PROCESSED。
	skip *RefundDisposition
}

func (f *fakeApplier) ApplyRefund(
	context.Context, RefundFinalized, domain.Document,
) (*RefundDisposition, error) {
	f.calls++
	return f.skip, f.err
}

// recordingApplier 記下 handler 真的傳下來的那一份輸入。
//
// 只數 calls 的 fake 驗不出「解得開但欄位全是零值」—— 而那正是漏宣告 7 欄之後的形狀。
type recordingApplier struct {
	calls int
	last  RefundFinalized
	err   error
	skip  *RefundDisposition
}

func (f *recordingApplier) ApplyRefund(
	_ context.Context, in RefundFinalized, _ domain.Document,
) (*RefundDisposition, error) {
	f.calls++
	f.last = in
	return f.skip, f.err
}

// ---------------------------------------------------------------------------
// C1：order.paid
// ---------------------------------------------------------------------------

func orderPaidBody(t *testing.T, eventID, orderNo, occurredAt string) []byte {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"event_id": eventID, "order_no": orderNo, "occurred_at": occurredAt,
		"buyer_id": 2623900520, "store_id": 2685914682, "total_actual_amount": 100,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

func newPaidConsumer(inbox *fakeInbox, prep *fakePreparer, of *fakeOrderFacts, docs *fakeDocs, ap *fakeApplier) *OrderPaidConsumer {
	return NewOrderPaidConsumer(inbox, prep, of, &fakePayFacts{facts: PaymentFacts{PayTime: time.Now().UTC()}}, ap, docs, nil)
}

// TestOrderPaid_Idempotent 是本檔最重要的一支：同一則 order.paid 重送，**只能**建一份
// document。用「Prepare 被呼叫幾次」與「取快照被呼叫幾次」兩個計數器斷言，而不是看
// 回傳值——回傳值在去重時和第一次成功時長得一樣。
func TestOrderPaid_Idempotent(t *testing.T) {
	inbox := newFakeInbox()
	prep := &fakePreparer{res: usecase.PrepareResult{DocumentID: 7, OperationID: 8}}
	of := &fakeOrderFacts{}
	c := newPaidConsumer(inbox, prep, of, &fakeDocs{bySourceRef: map[string]domain.Document{}}, &fakeApplier{})

	body := orderPaidBody(t, "evt-1", "O000000001", "2026-08-25T09:41")
	for i := range 3 {
		if err := c.Handle(context.Background(), "order.paid", body); err != nil {
			t.Fatalf("第 %d 次 Handle: %v", i+1, err)
		}
	}
	if prep.calls != 1 {
		t.Fatalf("Prepare 被呼叫 %d 次，want 1（重送必須被 inbox 去重擋下）", prep.calls)
	}
	if of.calls != 1 {
		t.Fatalf("GetOrderInvoiceSnapshot 被呼叫 %d 次，want 1", of.calls)
	}
}

// TestOrderPaid_IdempotentAcrossEventIDs 守業務鍵那一半。
//
// 上游若換了一個 event_id 重送（RefundSyncWorker 就是這樣做的），只靠 UNIQUE(event_id)
// 的去重會整個失效。這一支把 event_id 換掉、order_no 不變。
func TestOrderPaid_IdempotentAcrossEventIDs(t *testing.T) {
	inbox := newFakeInbox()
	prep := &fakePreparer{}
	c := newPaidConsumer(inbox, prep, &fakeOrderFacts{}, &fakeDocs{bySourceRef: map[string]domain.Document{}}, &fakeApplier{})

	for i, id := range []string{"evt-a", "evt-b", "evt-c"} {
		if err := c.Handle(context.Background(), "order.paid", orderPaidBody(t, id, "O000000002", "2026-08-25T09:41:02")); err != nil {
			t.Fatalf("第 %d 次 Handle: %v", i+1, err)
		}
	}
	if prep.calls != 1 {
		t.Fatalf("Prepare 被呼叫 %d 次，want 1（business_dedup_key 必須擋住換 event_id 的重送）", prep.calls)
	}
}

// TestOrderPaid_MalformedBodyGoesToDLQ 守修正後的慣例①：反序列化失敗 → return error
// （retry → DLQ），**不是** ACK 丟棄。
//
// 一則被丟棄的 order.paid ＝ 一張已收款訂單永遠不開發票（法定義務），而且連 inbox 列
// 都沒有 ⇒ M8 也看不到。回 error 之後訊息原文留在 invoice.order.paid.dlq，可查可重放。
func TestOrderPaid_MalformedBodyGoesToDLQ(t *testing.T) {
	inbox := newFakeInbox()
	c := newPaidConsumer(inbox, &fakePreparer{}, &fakeOrderFacts{}, &fakeDocs{}, &fakeApplier{})
	if err := c.Handle(context.Background(), "order.paid", []byte("{not json")); err == nil {
		t.Fatal("反序列化失敗必須 return error（→ retry → DLQ）；ACK 丟棄等於一張已收款訂單永遠不開票且無法重放")
	}
	if inbox.inserts != 0 {
		t.Fatal("壞掉的 payload 不該落 inbox")
	}
	if ConsumerMaxRetries <= 0 {
		t.Fatalf("ConsumerMaxRetries = %d：回 error 只有在 MaxRetries > 0 時才有上界", ConsumerMaxRetries)
	}
}

// TestOrderPaid_FailLoudOnUnparseableFields 守慣例②：業務欄位壞掉必須 return error
// 走 retry → DLQ，**禁止吞成零值**。occurred_at 吞成零值 ⇒ 法定期限落在 1970 ⇒
// due-date worker 立刻判定逾期，而且沒有任何訊號。
func TestOrderPaid_FailLoudOnUnparseableFields(t *testing.T) {
	cases := map[string][]byte{
		"occurred_at 無法解析": orderPaidBody(t, "e1", "O1", "yesterday"),
		"occurred_at 空":    orderPaidBody(t, "e2", "O2", ""),
		"order_no 空":       orderPaidBody(t, "e3", "", "2026-08-25T09:41:02Z"),
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			c := newPaidConsumer(newFakeInbox(), &fakePreparer{}, &fakeOrderFacts{}, &fakeDocs{}, &fakeApplier{})
			if err := c.Handle(context.Background(), "order.paid", body); err == nil {
				t.Fatal("必須 return error（fail-loud），got nil")
			}
		})
	}
}

// TestOrderPaid_PeerClientFailureGoesToDLQ 守「peer client 未接線時的行為是看得見的失敗」，
// 不是「靜默開出一張空發票」。
func TestOrderPaid_PeerClientFailureGoesToDLQ(t *testing.T) {
	c := NewOrderPaidConsumer(newFakeInbox(), &fakePreparer{},
		UnavailableOrderFactsClient{}, UnavailablePaymentFactsClient{},
		&fakeApplier{}, &fakeDocs{}, nil)
	err := c.Handle(context.Background(), "order.paid", orderPaidBody(t, "e", "O9", "2026-08-25T09:41:02Z"))
	if !errors.Is(err, ErrPeerClientUnavailable) {
		t.Fatalf("want ErrPeerClientUnavailable, got %v", err)
	}
}

// TestOrderPaid_ReclaimsDeferredRefund 守 §6.3.6 的反向認領：退款先到、付款後到時，
// 開票成功之後必須把那幾列 DEFERRED 撈回來套用並推進 PROCESSED。
func TestOrderPaid_ReclaimsDeferredRefund(t *testing.T) {
	ctx := context.Background()
	orderNo := "O000000003"
	inbox := newFakeInbox()
	docs := &fakeDocs{bySourceRef: map[string]domain.Document{}}
	applier := &fakeApplier{}

	// 退款先到：document 還不存在 ⇒ DEFERRED。
	refund := NewRefundFinalizedConsumer(inbox, docs, applier, nil)
	refundBody := producerShapedRefundBody(t, map[string]any{
		"event_id": "r-1", "order_no": orderNo, "occurred_at": "2026-08-25T09:40:00Z",
		"refunded_at": "2026-08-25T09:40:00Z", "refund_ref": "TXN-001", "refund_amount": "100",
	})
	if err := refund.Handle(ctx, "order.refund.finalized", refundBody); err != nil {
		t.Fatalf("refund handle: %v", err)
	}
	if applier.calls != 0 {
		t.Fatal("查不到 document 時不得套用調整")
	}
	if inbox.rows[0].Status != inboxStatusDeferred {
		t.Fatalf("退款先到時 status 應為 DEFERRED，got %s", inbox.rows[0].Status)
	}

	// 付款後到：Prepare 成功之後 document 出現，反向認領應該把那一列處理掉。
	docs.bySourceRef[orderNo] = domain.Document{ID: 42}
	paid := newPaidConsumer(inbox, &fakePreparer{res: usecase.PrepareResult{DocumentID: 42}}, &fakeOrderFacts{}, docs, applier)
	if err := paid.Handle(ctx, "order.paid", orderPaidBody(t, "p-1", orderNo, "2026-08-25T09:41:02")); err != nil {
		t.Fatalf("paid handle: %v", err)
	}
	if applier.calls != 1 {
		t.Fatalf("反向認領應套用 1 筆退款，實際 %d", applier.calls)
	}
	// 兩列都要被推出兜底候選集合，順序即發生順序：
	//   id=2 是本次的 ORDER_PAID 列（Prepare 成功之後標記）
	//   id=1 是先到的 DEFERRED 退款列（反向認領套用之後標記）
	// 🔴 少了 id=2 那一次，ORDER_PAID 列會永遠停在 RECEIVED ＋ next_attempt_at=now，
	//    也就是永久留在 idx_invinbox_due 的候選集合裡，INV.dl 每一輪都白撈它一次。
	if want := []int64{2, 1}; !slices.Equal(inbox.processed, want) {
		t.Fatalf("兩列都應被推進 PROCESSED，want %v 實際 %v", want, inbox.processed)
	}
}

// ---------------------------------------------------------------------------
// C2：order.refund.finalized
// ---------------------------------------------------------------------------

// producerShapedRefundBody 造一則**與 order-service 實際發出的形狀相同**的
// `order.refund.finalized` body。
//
// 🔴 每一個值都是 JSON 字串，因為 `OrderRefundFinalizedPayload` 的 16 個 component
// 全是 `String`（`@JsonNaming(SnakeCaseStrategy)`）：
//   - 金額三欄走 `String.valueOf(long)`（OrderRefundFinalizedOutboxWriter.write）
//   - store_id / buyer_id 走 `str(Long)`
//   - occurred_at 與 refunded_at 同值，走 `DateTimeUtils.formatIso8601(nowUtc())` ⇒ 帶 Z
//   - refund_outcome / case_no / cancel_id / correlation_id 走 `blankToNull` ⇒ 可能是 JSON null
//
// **欄位集合由 TestRefundFixtureCoversEveryProducerField 對 Java 原始碼逐欄核對**，
// 不是憑印象列的 —— 這四個缺陷能活下來，正是因為 fixture 與 struct 出自同一份想像。
//
// overrides 用來改單一欄；值傳 nil 代表送 JSON null（模擬 blankToNull）。
func producerShapedRefundBody(t *testing.T, overrides map[string]any) []byte {
	t.Helper()
	body := map[string]any{
		"event_id":                  "8f1c9a3e-0000-4b2a-9c31-000000000001",
		"order_no":                  "O000000001",
		"occurred_at":               "2026-08-30T09:41:02Z",
		"store_id":                  "2685914682",
		"buyer_id":                  "2623900520",
		"refund_ref":                "AS_REFUND_C2026083000001",
		"refund_source":             "AFTERSALE_CASE",
		"refund_outcome":            "PARTIAL_REFUND",
		"refund_amount":             "200",
		"order_total_actual_amount": "1000",
		"cumulative_refund_amount":  "350",
		"case_no":                   "C2026083000001",
		"cancel_id":                 nil,
		"payment_method":            "CREDIT_CARD",
		"refunded_at":               "2026-08-30T09:41:02Z",
		"correlation_id":            "0af7651916cd43dd8448eb211c80319c",
	}
	for k, v := range overrides {
		body[k] = v
	}
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

// TestRefund_RealProducerPayloadIsConsumed 是 P0-2 的端到端證明。
//
// 🔴 這一支斷言的**不是**「不 panic」，而是三件事同時成立：
//  1. producer 形狀的 body 解得開（原本每一則都解不開）
//  2. inbox 真的落了一列（原本 DB 零痕跡）
//  3. applier 收到的每一個業務值都對，**含原本根本沒宣告的 7 欄**
//
// 第 3 點是關鍵：只驗「不再回錯」會讓「解得開但欄位全是零值」照樣通過，
// 而 cumulative_refund_amount 讀成 0 的後果就是該作廢的發票不作廢。
func TestRefund_RealProducerPayloadIsConsumed(t *testing.T) {
	ctx := context.Background()
	orderNo := "O000000001"
	inbox := newFakeInbox()
	docs := &fakeDocs{bySourceRef: map[string]domain.Document{orderNo: {ID: 77}}}
	applier := &recordingApplier{}
	c := NewRefundFinalizedConsumer(inbox, docs, applier, nil)

	if err := c.Handle(ctx, "order.refund.finalized", producerShapedRefundBody(t, nil)); err != nil {
		t.Fatalf("真實 producer 形狀的 payload 必須被消費，卻回了：%v", err)
	}
	if len(inbox.rows) != 1 {
		t.Fatalf("inbox 應落 1 列，實際 %d —— 解不開的事件連列都沒有，那正是原缺陷「DB 零痕跡」的形狀", len(inbox.rows))
	}
	if applier.calls != 1 {
		t.Fatalf("applier 被呼叫 %d 次，want 1", applier.calls)
	}

	got := applier.last
	want := RefundFinalized{
		OrderNo:                orderNo,
		RefundRef:              "AS_REFUND_C2026083000001",
		RefundSource:           "AFTERSALE_CASE",
		RefundOutcome:          "PARTIAL_REFUND",
		RefundAmount:           200,
		OrderTotalActualAmount: 1000,
		CumulativeRefundAmount: 350,
		CaseNo:                 "C2026083000001",
		CancelID:               "",
		PaymentMethod:          "CREDIT_CARD",
		RefundedAt:             time.Date(2026, 8, 30, 9, 41, 2, 0, time.UTC),
		OccurredAt:             time.Date(2026, 8, 30, 9, 41, 2, 0, time.UTC),
	}
	if !got.RefundedAt.Equal(want.RefundedAt) || !got.OccurredAt.Equal(want.OccurredAt) {
		t.Errorf("時間欄不符：got refunded_at=%v occurred_at=%v", got.RefundedAt, got.OccurredAt)
	}
	got.RefundedAt, got.OccurredAt = want.RefundedAt, want.OccurredAt
	if got != want {
		t.Errorf("applier 收到的輸入不符\n got  = %+v\n want = %+v", got, want)
	}
}

// TestRefund_UnmarshalFailureGoesToDLQNotSilentDiscard 釘住失敗的**歸宿**。
//
// 原行為是 `return nil`（ACK 丟棄）＝ payload 永久消失、無法重放，只留 M30 一個計數。
// 現在回 error ⇒ pkg/mq 走 retry（MaxRetries=3，與 topology 對拍）⇒ 三次後直投
// `invoice.order.refund.finalized.dlq`，原文留在 broker 上可查可重放。
//
// ⚠️ 「毒訊息無限重投」只在 MaxRetries == 0 才成立（pkg/mq 的 legacy 路徑），
// 本 subscription 是 3 ⇒ 有上界。
func TestRefund_UnmarshalFailureGoesToDLQNotSilentDiscard(t *testing.T) {
	inbox := newFakeInbox()
	c := NewRefundFinalizedConsumer(inbox, &fakeDocs{}, &fakeApplier{}, nil)
	err := c.Handle(context.Background(), "order.refund.finalized", []byte(`{"order_no": [破的}`))
	if err == nil {
		t.Fatal("解不開的 payload 必須回 error（→ retry → DLQ）；ACK 丟棄等於一筆真實退款永遠不產生折讓單且無法重放")
	}
	if len(inbox.rows) != 0 {
		t.Fatalf("解不開時不該落 inbox 列，實際 %d", len(inbox.rows))
	}
	if ConsumerMaxRetries <= 0 {
		t.Fatalf("ConsumerMaxRetries = %d：回 error 只有在 MaxRetries > 0 時才有上界，"+
			"否則 pkg/mq 走 legacy 無限 requeue ⇒ 毒訊息熱迴圈", ConsumerMaxRetries)
	}
}

// TestRefund_MissingAmountFieldsFailLoud 守慣例②：業務欄位解析失敗禁止吞成零值。
//
// 🔴 三個金額欄各測一次而不是只測一個：它們在 producer 端都必然非空，
// 而讀成 0 的後果各不相同 —— cumulative 讀 0 ⇒「判成沒退過」，
// order_total 讀 0 ⇒「累計必定超額」，refund_amount 讀 0 ⇒「開一張 0 元折讓單」。
func TestRefund_MissingAmountFieldsFailLoud(t *testing.T) {
	for _, field := range []string{"refund_amount", "order_total_actual_amount", "cumulative_refund_amount"} {
		t.Run(field+"=null", func(t *testing.T) {
			inbox := newFakeInbox()
			docs := &fakeDocs{bySourceRef: map[string]domain.Document{"O000000001": {ID: 1}}}
			c := NewRefundFinalizedConsumer(inbox, docs, &fakeApplier{}, nil)
			body := producerShapedRefundBody(t, map[string]any{field: nil})
			if err := c.Handle(context.Background(), "order.refund.finalized", body); err == nil {
				t.Fatalf("%s 缺值必須 fail-loud，不得吞成 0", field)
			}
			if len(inbox.rows) != 0 {
				t.Fatalf("解析失敗必須發生在 Insert 之前（否則去重鍵已落庫、重投會被擋掉、"+
					"這則事件連 retry 都用不上），實際落了 %d 列", len(inbox.rows))
			}
		})
	}
}

// TestRefund_ReclaimUsesTheSameConversion 守「反向認領」那條路徑也拿得到新欄位。
//
// 兩條路徑各自組裝 RefundFinalized 時，新增欄位只會被接上其中一條 —— 而漏掉的
// 通常是這一條（退款先到、付款後到），因為它最難在測試裡想起來。
func TestRefund_ReclaimUsesTheSameConversion(t *testing.T) {
	ctx := context.Background()
	orderNo := "O000000001"
	inbox := newFakeInbox()
	docs := &fakeDocs{bySourceRef: map[string]domain.Document{}}
	applier := &recordingApplier{}

	// 退款先到 ⇒ DEFERRED。
	refund := NewRefundFinalizedConsumer(inbox, docs, applier, nil)
	if err := refund.Handle(ctx, "order.refund.finalized", producerShapedRefundBody(t, nil)); err != nil {
		t.Fatalf("refund handle: %v", err)
	}
	if applier.calls != 0 {
		t.Fatal("查不到 document 時不得套用調整")
	}

	// 付款後到 ⇒ 反向認領。
	docs.bySourceRef[orderNo] = domain.Document{ID: 77}
	paid := NewOrderPaidConsumer(inbox, &fakePreparer{res: usecase.PrepareResult{DocumentID: 77}},
		&fakeOrderFacts{}, &fakePayFacts{facts: PaymentFacts{PayTime: time.Now().UTC()}}, applier, docs, nil)
	if err := paid.Handle(ctx, "order.paid", orderPaidBody(t, "p-9", orderNo, "2026-08-30T09:42:00")); err != nil {
		t.Fatalf("paid handle: %v", err)
	}
	if applier.calls != 1 {
		t.Fatalf("反向認領應套用 1 筆退款，實際 %d", applier.calls)
	}
	if applier.last.CumulativeRefundAmount != 350 || applier.last.RefundOutcome != "PARTIAL_REFUND" {
		t.Fatalf("反向認領走的是第二份組裝、沒帶上新欄位：cumulative=%d outcome=%q",
			applier.last.CumulativeRefundAmount, applier.last.RefundOutcome)
	}
}

func TestRefund_FailLoudOnMissingRefundRef(t *testing.T) {
	body := producerShapedRefundBody(t, map[string]any{
		"event_id": "r", "order_no": "O1", "occurred_at": "2026-08-25T09:40:00Z", "refund_ref": nil,
	})
	c := NewRefundFinalizedConsumer(newFakeInbox(), &fakeDocs{}, &fakeApplier{}, nil)
	if err := c.Handle(context.Background(), "order.refund.finalized", body); err == nil {
		t.Fatal("refund_ref 缺失必須 fail-loud —— 退回 event_id 當冪等鍵會讓同一筆退款每小時開一張新折讓單")
	}
}

// TestRefund_IdempotentAcrossEventIDs 是上一條的正面版本：refund_ref 相同、event_id 每次
// 都換（RefundSyncWorker 的真實行為），只能套用一次。
func TestRefund_IdempotentAcrossEventIDs(t *testing.T) {
	inbox := newFakeInbox()
	docs := &fakeDocs{bySourceRef: map[string]domain.Document{"O5": {ID: 1}}}
	applier := &fakeApplier{}
	c := NewRefundFinalizedConsumer(inbox, docs, applier, nil)

	for _, eventID := range []string{"u1", "u2", "u3"} {
		body := producerShapedRefundBody(t, map[string]any{
			"event_id": eventID, "order_no": "O5", "occurred_at": "2026-08-25T09:40:00Z",
			"refunded_at": "2026-08-25T09:40:00Z", "refund_ref": "TXN-SAME", "refund_amount": "50",
		})
		if err := c.Handle(context.Background(), "order.refund.finalized", body); err != nil {
			t.Fatalf("handle(%s): %v", eventID, err)
		}
	}
	if applier.calls != 1 {
		t.Fatalf("套用 %d 次，want 1", applier.calls)
	}
}

// TestRefund_ApplierFailureIsNotAcked 守「inbox 寫進去了但退款沒被套用」不得 ACK 成完成。
//
// 同時釘住失敗的**歸宿**：它不進 DLQ（pkg/mq 的重投會被 Insert 的去重擋掉並 return nil，
// retry counter 永遠停在 1），事實只留在 invoice_inbox_events 的 status=FAILED ＋ last_error。
// 少了 MarkFailed 那一次，這筆退款會停在 RECEIVED、既沒有錯誤訊號也沒有處置留證，
// 而 ADM 異常工作台答不出「這筆退款為什麼沒開折讓」。
func TestRefund_ApplierFailureIsNotAcked(t *testing.T) {
	inbox := newFakeInbox()
	docs := &fakeDocs{bySourceRef: map[string]domain.Document{"O6": {ID: 2}}}
	c := NewRefundFinalizedConsumer(inbox, docs, UnimplementedAdjustmentApplier{}, nil)
	body := producerShapedRefundBody(t, map[string]any{
		"event_id": "x", "order_no": "O6", "occurred_at": "2026-08-25T09:40:00Z",
		"refunded_at": "2026-08-25T09:40:00Z", "refund_ref": "TXN-9", "refund_amount": "10",
	})
	if err := c.Handle(context.Background(), "order.refund.finalized", body); !errors.Is(err, ErrAdjustmentNotImplemented) {
		t.Fatalf("want ErrAdjustmentNotImplemented, got %v", err)
	}
	if len(inbox.failed) != 1 {
		t.Fatalf("MarkFailed 被呼叫 %d 次，want 1（失敗的歸宿是 inbox 的 FAILED，不是 DLQ）", len(inbox.failed))
	}
}

// ---------------------------------------------------------------------------
// handler 與 INV.dl 兜底掃描的交界
// ---------------------------------------------------------------------------

// TestOrderPaid_InsertReservesLeaseWhileHandlerRuns 守 handlerLease。
//
// 🔴 invoice_inbox_events.next_attempt_at **兼租約欄**。Insert 若寫 `now`，這一列在
// handler 還在跑（兩支 peer gRPC）的時候就已經到期 ⇒ INV.dl 的下一輪可以合法搶走一列
// **正在被處理**的事件，兩邊同時跑 §7.1.2 步驟 5–9。
//
// 在兜底掃描接上之前這件事無害（沒有人在掃），所以它是「新增執行者時要回頭看既有寫入端
// 假設」的實例——而那個假設沒有任何測試守著，直到這一支。
//
// 數值面的下界（租約 > 兩支 peer client 的 deadline 總和）在
// internal/worker/deadline_worker_test.go 的 TestDeadlineScanCannotPreemptARunningHandler：
// 本 package 不能 import internal/client（client/order 與 client/payment 都 import 本
// package，反向即 cycle）。
func TestOrderPaid_InsertReservesLeaseWhileHandlerRuns(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	inbox := newFakeInbox()
	c := NewOrderPaidConsumer(inbox, &fakePreparer{},
		UnavailableOrderFactsClient{}, UnavailablePaymentFactsClient{},
		&fakeApplier{}, &fakeDocs{}, nil).WithClock(func() time.Time { return now })

	// peer client 不可用 ⇒ handler 在步驟 5 失敗，該列留在 RECEIVED —— 正是兜底掃描要
	// 接手的形狀。
	if err := c.Handle(ctx, "order.paid", orderPaidBody(t, "e-lease", "O000000010", "2026-08-30T11:59:00Z")); err == nil {
		t.Fatal("peer client 不可用時必須 fail-loud")
	}

	paid := string(pkgmq.EventTypeOrderPaid)
	if due, err := inbox.FetchDueByType(ctx, paid, now, 10); err != nil || len(due) != 0 {
		t.Fatalf("剛落庫的列在 handler 執行期間就已到期（%d 列，err=%v）—— INV.dl 可以搶走它",
			len(due), err)
	}
	if due, err := inbox.FetchDueByType(ctx, paid, now.Add(handlerLease), 10); err != nil || len(due) != 1 {
		t.Fatalf("租約到期後該列沒有回到兜底候選集合（%d 列，err=%v）—— 這則事件將永遠停在 RECEIVED",
			len(due), err)
	}
}

// TestRefund_SkipMarksInboxSkippedNotProcessed 守 §6.3.5.4 步驟 3 的最後一句：
// 「判定為不需要任何發票調整 ⇒ MarkInboxEventSkipped(id, disposition, reason, now)，
// **disposition 不得為空**」。
//
// 🔴 標成 PROCESSED 的後果不是「狀態不精確」：PROCESSED 說「業務動作做完了」，
// 而這裡什麼都沒做。事後查「為什麼這筆退款沒有折讓單」時，那一列與真的開過折讓的
// 列長得一模一樣——而 invoice_inbox_events 存在的理由正是回答這個問題。
func TestRefund_SkipMarksInboxSkippedNotProcessed(t *testing.T) {
	ctx := context.Background()
	orderNo := "O000000001"
	inbox := newFakeInbox()
	docs := &fakeDocs{bySourceRef: map[string]domain.Document{orderNo: {ID: 77}}}
	applier := &recordingApplier{skip: &RefundDisposition{
		Disposition: "NOT_AMEGO_AUTO", Reason: "fulfillment_mode=SELLER_SELF_ISSUE",
	}}
	c := NewRefundFinalizedConsumer(inbox, docs, applier, nil)

	if err := c.Handle(ctx, "order.refund.finalized", producerShapedRefundBody(t, nil)); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if len(inbox.rows) != 1 {
		t.Fatalf("inbox 應落 1 列，實際 %d", len(inbox.rows))
	}
	if got := inbox.rows[0].Status; got != "SKIPPED" {
		t.Fatalf("inbox status = %q，want SKIPPED —— PROCESSED 會讓「刻意不處理」"+
			"與「處理完成」在帳上變成同一件事", got)
	}
}

// TestRefund_SuccessStillMarksProcessed 是上一支的反面。
//
// 少了它，一個「一律標 SKIPPED」的實作會讓上一支全綠 —— 而那會讓每一筆真的建了
// 折讓的退款都被記成「刻意不處理」。
func TestRefund_SuccessStillMarksProcessed(t *testing.T) {
	ctx := context.Background()
	orderNo := "O000000001"
	inbox := newFakeInbox()
	docs := &fakeDocs{bySourceRef: map[string]domain.Document{orderNo: {ID: 77}}}
	c := NewRefundFinalizedConsumer(inbox, docs, &recordingApplier{}, nil)

	if err := c.Handle(ctx, "order.refund.finalized", producerShapedRefundBody(t, nil)); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if got := inbox.rows[0].Status; got != "PROCESSED" {
		t.Fatalf("inbox status = %q，want PROCESSED", got)
	}
}
