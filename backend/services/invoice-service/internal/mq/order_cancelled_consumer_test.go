package mq

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	pkgmq "github.com/Will413028/hawkr/go-pkg/mq"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

// ---------------------------------------------------------------------------
// fakes
// ---------------------------------------------------------------------------

// fakeCancelled 是 invoice_cancelled_orders 的 fake。鍵是 order_no（＝真表的 PK）。
type fakeCancelled struct {
	rows      map[string]domain.CancelledOrder
	inserts   int
	insertErr error
}

func newFakeCancelled() *fakeCancelled {
	return &fakeCancelled{rows: map[string]domain.CancelledOrder{}}
}

func (f *fakeCancelled) Insert(_ context.Context, in domain.CancelledOrder) (bool, error) {
	if f.insertErr != nil {
		return false, f.insertErr
	}
	f.inserts++
	if _, dup := f.rows[in.OrderNo]; dup {
		return false, nil
	}
	f.rows[in.OrderNo] = in
	return true, nil
}

func (f *fakeCancelled) IsCancelledForBuyer(_ context.Context, orderNo string, buyerID int64) (bool, error) {
	r, ok := f.rows[orderNo]
	return ok && r.BuyerID == buyerID, nil
}

// rollbackTx 是**會真的還原**的 TxManager fake。
//
// 🔴 既有的 nopTx／xTx 只是 `return fn(ctx)`：它們證明不了本 handler 的核心不變式
// （「失敗時去重列一起消失」）——用它們寫出來的測試，即使 handler 改成兩個獨立交易
// 也照樣全綠。這裡以 snapshot/restore 重現 rollback 的**可觀察後果**。
//
// ⚠️ 它仍然是 fake。真正的原子性由 TestIntegration_OrderCancelled*（真 DB、真
// TxManager）證明；這裡要的是「單元層跑得快、而且改壞了會紅」。
type rollbackTx struct {
	inbox     *fakeInbox
	cancelled *fakeCancelled
	rollbacks int
}

func (r *rollbackTx) WithTx(ctx context.Context, fn func(ctx context.Context) error) error {
	inboxSnap := *r.inbox
	inboxSnap.byEventID = cloneBoolMap(r.inbox.byEventID)
	inboxSnap.byBizKey = cloneBoolMap(r.inbox.byBizKey)
	inboxSnap.rows = append([]domain.InboxEvent(nil), r.inbox.rows...)
	inboxSnap.processed = append([]int64(nil), r.inbox.processed...)
	cancelledSnap := *r.cancelled
	cancelledSnap.rows = map[string]domain.CancelledOrder{}
	for k, v := range r.cancelled.rows {
		cancelledSnap.rows[k] = v
	}

	if err := fn(ctx); err != nil {
		*r.inbox = inboxSnap
		*r.cancelled = cancelledSnap
		r.rollbacks++
		return err
	}
	return nil
}

func cloneBoolMap(in map[string]bool) map[string]bool {
	out := make(map[string]bool, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func newCancelledFixture(t *testing.T) (*OrderCancelledConsumer, *fakeInbox, *fakeCancelled, *rollbackTx) {
	t.Helper()
	inbox := newFakeInbox()
	cancelled := newFakeCancelled()
	tx := &rollbackTx{inbox: inbox, cancelled: cancelled}
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	c := NewOrderCancelledConsumer(tx, inbox, cancelled, nil).
		WithClock(func() time.Time { return now })
	return c, inbox, cancelled, tx
}

// producerShapedCancelledBody 造一個**與 producer 形狀相同**的 body。
//
// 🔴 型別逐欄取自 order-service 的 `OrderCancelledPayload`：`buyer_id` /
// `store_id` / `notify_*` 是 `Long` ⇒ **JSON 數字**；其餘是 `String` ⇒ 字串；
// `items` 是巢狀 record 陣列。這與正上方的 refund fixture（全字串）**相反**，
// 兩份都對——判別方式只有讀 producer。
// 欄位集合本身由 TestCancelledFixtureCoversEveryProducerField 對 Java 原始碼守住。
func producerShapedCancelledBody(t *testing.T, overrides map[string]any) []byte {
	t.Helper()
	body := map[string]any{
		"event_id":    "3f8b1d20-0000-4c11-9a02-000000000009",
		"order_no":    "O320270909",
		"stock_tx_id": "STK2026083000001",
		"items": []any{
			map[string]any{"sku_id": "7381920374651238401", "quantity": "2"},
		},
		// 🔴 無 Z、無時區：producer 寫的是 DateTimeUtils.nowUtc().toString()
		// （LocalDateTime.toString()），不是 formatIso8601。
		"occurred_at":              "2026-08-30T11:58:03",
		"buyer_id":                 2623900520,
		"notify_buyer_account_id":  2623900520,
		"store_id":                 2685914682,
		"notify_seller_account_id": 2685914682,
		"seller_name":              "測試賣場",
		"seller_avatar_key":        "public/seller/logo/1.jpg",
		"cancel_reason":            "PAYMENT_TIMEOUT",
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

// ---------------------------------------------------------------------------
// happy path
// ---------------------------------------------------------------------------

// TestOrderCancelled_RealProducerPayloadIsConsumed 是 #2904 的主證明。
//
// 斷言四件事同時成立，缺一都可能是「看起來修好了」：
//  1. producer 形狀（buyer_id 是 JSON **數字**）的 body 解得開
//  2. invoice_cancelled_orders 真的落了一列，且 buyer_id 對
//  3. inbox 那一列被推進 PROCESSED（不是留在 RECEIVED）
//  4. 讀路徑用得到它（IsCancelledForBuyer 回 true）
func TestOrderCancelled_RealProducerPayloadIsConsumed(t *testing.T) {
	ctx := context.Background()
	c, inbox, cancelled, _ := newCancelledFixture(t)

	if err := c.Handle(ctx, pkgmq.RoutingKeyOrderCancelled, producerShapedCancelledBody(t, nil)); err != nil {
		t.Fatalf("producer 形狀的 payload 應該要能處理：%v", err)
	}

	row, ok := cancelled.rows["O320270909"]
	if !ok {
		t.Fatal("invoice_cancelled_orders 沒有落列 —— 買家仍會看到「付款完成後會立即開立發票」")
	}
	if row.BuyerID != 2623900520 {
		t.Errorf("buyer_id = %d, want 2623900520（它是讀路徑的授權述詞，錯了整列就對不到人）", row.BuyerID)
	}
	if row.CancelReason != "PAYMENT_TIMEOUT" {
		t.Errorf("cancel_reason = %q, want PAYMENT_TIMEOUT", row.CancelReason)
	}
	// occurred_at 必須是解析後的事件時間，不是 handler 的 now。
	want := time.Date(2026, 8, 30, 11, 58, 3, 0, time.UTC)
	if !row.OccurredAt.Equal(want) {
		t.Errorf("occurred_at = %v, want %v（無時區的 LocalDateTime 一律以 UTC 解讀）", row.OccurredAt, want)
	}

	if len(inbox.rows) != 1 {
		t.Fatalf("inbox 落了 %d 列，want 1", len(inbox.rows))
	}
	if got := inbox.rows[0].Status; got != "PROCESSED" {
		t.Errorf("inbox 列停在 %q，want PROCESSED —— 非終態的 ORDER_CANCELLED 列沒有任何執行者"+
			"（INV.dl 只掃 ORDER_PAID），它會是一列孤兒並在 7 天後觸發 InvoiceInboxStranded", got)
	}
	if inbox.rows[0].EventType != string(pkgmq.EventTypeOrderCancelled) {
		t.Errorf("event_type = %q, want %q", inbox.rows[0].EventType, pkgmq.EventTypeOrderCancelled)
	}

	got, err := cancelled.IsCancelledForBuyer(ctx, "O320270909", 2623900520)
	if err != nil || !got {
		t.Errorf("讀路徑查不到這一列（got=%v err=%v）", got, err)
	}
}

// TestOrderCancelled_StringBuyerIDIsRejected 是**反例**：把 refund payload 的慣例
// （每一欄都是字串）套到這條 queue 上。
//
// 🔴 它必須失敗。若哪天 producer 真的把 buyer_id 改成 String，本測試會轉紅並指出
// 「payload 型別變了」——那正是我們要的訊號，不是「測試過時了」。
func TestOrderCancelled_StringBuyerIDIsRejected(t *testing.T) {
	c, inbox, cancelled, _ := newCancelledFixture(t)
	body := producerShapedCancelledBody(t, map[string]any{"buyer_id": "2623900520"})

	err := c.Handle(context.Background(), pkgmq.RoutingKeyOrderCancelled, body)
	if err == nil {
		t.Fatal("buyer_id 是字串時 json.Unmarshal 必定失敗，handler 卻回 nil —— " +
			"那是 ACK 丟棄，訊息永久消失且 DB 零痕跡")
	}
	if len(inbox.rows) != 0 || len(cancelled.rows) != 0 {
		t.Error("解析失敗卻寫了資料")
	}
}

// ---------------------------------------------------------------------------
// 失敗處置（skill checklist §B）
// ---------------------------------------------------------------------------

// TestOrderCancelled_MalformedBodyGoesToDLQ 釘住慣例①：反序列化失敗 return error。
func TestOrderCancelled_MalformedBodyGoesToDLQ(t *testing.T) {
	c, inbox, _, _ := newCancelledFixture(t)
	if err := c.Handle(context.Background(), pkgmq.RoutingKeyOrderCancelled, []byte("{not json")); err == nil {
		t.Fatal("壞掉的 payload 必須 return error（→ retry → DLQ，原文留在 broker 可查可重放），" +
			"不得 return nil（＝ ACK 丟棄，每一個觀測點都看不到它）")
	}
	if len(inbox.rows) != 0 {
		t.Error("解不開的 payload 不該留下 inbox 列")
	}
}

// TestOrderCancelled_MissingBuyerIDIsRejected 釘住慣例②：授權述詞缺失不得吞成零值。
func TestOrderCancelled_MissingBuyerIDIsRejected(t *testing.T) {
	for _, tc := range []struct {
		name string
		val  any
	}{
		{"欄位缺席", nil},
		{"值為 0", 0},
		{"負值", -1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, inbox, cancelled, _ := newCancelledFixture(t)
			body := producerShapedCancelledBody(t, map[string]any{"buyer_id": tc.val})
			if err := c.Handle(context.Background(), pkgmq.RoutingKeyOrderCancelled, body); err == nil {
				t.Fatal("buyer_id 不可用時必須 fail-loud：吞成 0 會寫出一列永遠對不到任何買家的死資料，" +
					"畫面上零症狀，而 DB 裡多一列看起來已經修好的紀錄")
			}
			if len(cancelled.rows) != 0 || len(inbox.rows) != 0 {
				t.Error("授權述詞不可用卻寫了資料")
			}
		})
	}
}

// TestOrderCancelled_EmptyOrderNoIsRejected 同上，另一個冪等鍵欄位。
func TestOrderCancelled_EmptyOrderNoIsRejected(t *testing.T) {
	c, _, cancelled, _ := newCancelledFixture(t)
	body := producerShapedCancelledBody(t, map[string]any{"order_no": ""})
	if err := c.Handle(context.Background(), pkgmq.RoutingKeyOrderCancelled, body); err == nil {
		t.Fatal("order_no 是主鍵與冪等鍵，缺席必須 fail-loud")
	}
	if len(cancelled.rows) != 0 {
		t.Error("order_no 為空卻寫了資料")
	}
}

// TestOrderCancelled_BadOccurredAtIsRejected 釘住「時間解析失敗不得吞成零值」。
func TestOrderCancelled_BadOccurredAtIsRejected(t *testing.T) {
	c, _, cancelled, _ := newCancelledFixture(t)
	body := producerShapedCancelledBody(t, map[string]any{"occurred_at": "30/08/2026"})
	if err := c.Handle(context.Background(), pkgmq.RoutingKeyOrderCancelled, body); err == nil {
		t.Fatal("occurred_at 解不開必須 fail-loud，不得吞成 1970")
	}
	if len(cancelled.rows) != 0 {
		t.Error("時間解析失敗卻寫了資料")
	}
}

// TestOrderCancelled_AcceptsEveryProducerTimeLayout 涵蓋 producer 真的會送的三種形狀。
//
// LocalDateTime.toString() 在秒與奈秒同為 0 時**連秒都省略** ⇒ 只用 RFC3339 解必定失敗，
// 而 occurred_at 解不開的後果是整則事件進 DLQ、該訂單的文案永遠不會被修正。
func TestOrderCancelled_AcceptsEveryProducerTimeLayout(t *testing.T) {
	for _, raw := range []string{
		"2026-08-30T11:58:03.123456789",
		"2026-08-30T11:58:03",
		"2026-08-30T11:58", // 秒與奈秒同為 0
		"2026-08-30T11:58:03Z",
	} {
		t.Run(raw, func(t *testing.T) {
			c, _, cancelled, _ := newCancelledFixture(t)
			body := producerShapedCancelledBody(t, map[string]any{"occurred_at": raw})
			if err := c.Handle(context.Background(), pkgmq.RoutingKeyOrderCancelled, body); err != nil {
				t.Fatalf("occurred_at=%q 應該解得開：%v", raw, err)
			}
			if len(cancelled.rows) != 1 {
				t.Fatal("沒有落列")
			}
		})
	}
}

// TestOrderCancelled_NullCancelReasonIsNormal 釘住「買家自願取消時 producer 送 null」。
//
// 空的 cancel_reason **不是**錯誤（producer 的 blankToNull ／ 建構子傳 null），
// 拿它當 fail-loud 依據會讓買家自願取消的訂單整批進 DLQ。
func TestOrderCancelled_NullCancelReasonIsNormal(t *testing.T) {
	c, _, cancelled, _ := newCancelledFixture(t)
	body := producerShapedCancelledBody(t, map[string]any{"cancel_reason": nil})
	if err := c.Handle(context.Background(), pkgmq.RoutingKeyOrderCancelled, body); err != nil {
		t.Fatalf("cancel_reason 為 null 是正常值：%v", err)
	}
	row, ok := cancelled.rows["O320270909"]
	if !ok {
		t.Fatal("沒有落列")
	}
	if row.CancelReason != "" {
		t.Errorf("cancel_reason = %q, want 空字串", row.CancelReason)
	}
}

// ---------------------------------------------------------------------------
// 原子性：本 handler 與 C1／C2 的關鍵差別
// ---------------------------------------------------------------------------

// TestOrderCancelled_FailureRollsBackTheDedupKey 守住本設計唯一的理由。
//
// 🔴 C1／C2 是「先寫去重列、再做業務」，所以失敗時去重鍵已經留下 ⇒ 重投的第二次
// 會被自己擋掉 ⇒ 訊息**永遠進不了 DLQ**，`mq_consume_total{result="dlq"}` 結構上恆 0。
// C3 把兩者放進同一個交易就是為了不要有那個性質。
//
// 這一支的鑑別力：把 handler 改回「兩個獨立交易」時，第二次 Handle 會因為去重鍵還在
// 而 return nil（假裝成功），本測試就會紅在「第二次應該仍然回錯」。
func TestOrderCancelled_FailureRollsBackTheDedupKey(t *testing.T) {
	ctx := context.Background()
	c, inbox, cancelled, tx := newCancelledFixture(t)
	cancelled.insertErr = errors.New("boom")
	body := producerShapedCancelledBody(t, nil)

	if err := c.Handle(ctx, pkgmq.RoutingKeyOrderCancelled, body); err == nil {
		t.Fatal("業務寫入失敗必須 return error")
	}
	if tx.rollbacks != 1 {
		t.Fatalf("rollback 次數 = %d, want 1", tx.rollbacks)
	}
	if len(inbox.rows) != 0 {
		t.Fatalf("rollback 之後 inbox 還留著 %d 列 —— 去重鍵留下了，重投會被自己擋掉，"+
			"訊息永遠進不了 DLQ", len(inbox.rows))
	}

	// 第二次投遞（pkg/mq 的 retry）必須真的重跑 —— 而不是被去重擋掉之後回 nil。
	if err := c.Handle(ctx, pkgmq.RoutingKeyOrderCancelled, body); err == nil {
		t.Fatal("重投時仍然失敗才對；回 nil 代表它被自己上一次留下的去重鍵擋掉了，" +
			"那是「假裝成功」——打滿 MaxRetries 也不會進 DLQ")
	}

	// 修好之後同一則訊息必須能成功（證明 rollback 沒有留下任何殘留）。
	cancelled.insertErr = nil
	if err := c.Handle(ctx, pkgmq.RoutingKeyOrderCancelled, body); err != nil {
		t.Fatalf("下游恢復之後重投應該成功：%v", err)
	}
	if len(cancelled.rows) != 1 || len(inbox.rows) != 1 {
		t.Errorf("恢復後的結果不對：cancelled=%d inbox=%d", len(cancelled.rows), len(inbox.rows))
	}
}

// TestOrderCancelled_DuplicateDeliveryIsAcked 釘住 at-least-once 的正常路徑。
func TestOrderCancelled_DuplicateDeliveryIsAcked(t *testing.T) {
	ctx := context.Background()
	c, inbox, cancelled, _ := newCancelledFixture(t)
	body := producerShapedCancelledBody(t, nil)

	for i := 0; i < 3; i++ {
		if err := c.Handle(ctx, pkgmq.RoutingKeyOrderCancelled, body); err != nil {
			t.Fatalf("第 %d 次投遞回錯：%v", i+1, err)
		}
	}
	if len(inbox.rows) != 1 {
		t.Errorf("inbox 有 %d 列，want 1（去重失效）", len(inbox.rows))
	}
	if cancelled.inserts != 1 {
		t.Errorf("業務寫入跑了 %d 次，want 1 —— 重投必須在去重那一步就停住", cancelled.inserts)
	}
}

// TestOrderCancelled_DedupIsKeyedOnOrderNoNotEventID 釘住冪等鍵的選擇。
//
// order_outbox 的重投會保留原 event_id，但「換一個 event_id 重送」在本 repo 是真實
// 存在的形狀（aftersales 的 RefundSyncWorker 每小時換一個新 UUID）。用 order_no
// 當業務鍵讓那件事在這條 queue 上也不成立。
func TestOrderCancelled_DedupIsKeyedOnOrderNoNotEventID(t *testing.T) {
	ctx := context.Background()
	c, _, cancelled, _ := newCancelledFixture(t)

	first := producerShapedCancelledBody(t, nil)
	second := producerShapedCancelledBody(t, map[string]any{
		"event_id": "00000000-0000-4000-8000-000000000002", // 換一個 UUID
	})
	if err := c.Handle(ctx, pkgmq.RoutingKeyOrderCancelled, first); err != nil {
		t.Fatal(err)
	}
	if err := c.Handle(ctx, pkgmq.RoutingKeyOrderCancelled, second); err != nil {
		t.Fatal(err)
	}
	if cancelled.inserts != 1 {
		t.Errorf("業務寫入跑了 %d 次，want 1 —— 換一個 event_id 就繞過去重，代表冪等鍵選錯了",
			cancelled.inserts)
	}
}

// TestCancelledFixtureCoversEveryProducerField 守住**測試 fixture 自己**。
//
// 缺陷能活下來的原因是「自己跟自己對」。所以 fixture 的欄位集合與**型別**也要對
// Java 原始碼，不能只對 Go struct。
func TestCancelledFixtureCoversEveryProducerField(t *testing.T) {
	comps := parseJavaRecord(t,
		"services/java/order-service/src/main/java/io/github/will413028/hawkr/order/mq/dto/OrderCancelledPayload.java")
	var fixture map[string]any
	if err := json.Unmarshal(producerShapedCancelledBody(t, nil), &fixture); err != nil {
		t.Fatalf("fixture 不是合法 JSON：%v", err)
	}
	byWire := map[string]javaComponent{}
	for _, c := range comps {
		byWire[c.Wire] = c
		if _, ok := fixture[c.Wire]; !ok {
			t.Errorf("fixture 少了 producer 會發的 %q", c.Wire)
		}
	}
	for k, v := range fixture {
		c, ok := byWire[k]
		if !ok {
			t.Errorf("fixture 多了 producer 不會發的 %q —— 這種欄位會讓測試證明一件不存在的事", k)
			continue
		}
		if v == nil {
			continue
		}
		// JSON 形狀必須與 Java 型別相符：String ⇒ 字串、Long ⇒ 數字。
		switch c.Type {
		case "String":
			if _, ok := v.(string); !ok {
				t.Errorf("fixture 的 %q 是 %T，但 producer 的 %s 是 String ⇒ wire 上是字串", k, v, c.Name)
			}
		case "Long":
			if _, ok := v.(float64); !ok {
				t.Errorf("fixture 的 %q 是 %T，但 producer 的 %s 是 Long ⇒ wire 上是數字"+
					"（fixture 寫成字串會讓 consumer 的 int64 宣告看起來是錯的，反過來也一樣）", k, v, c.Name)
			}
		case "List<StockItemPayload>":
			if _, ok := v.([]any); !ok {
				t.Errorf("fixture 的 %q 是 %T，但 producer 送的是陣列", k, v)
			}
		default:
			t.Errorf("producer 欄位 %s 的型別 %q 沒有被本測試涵蓋 —— 請先決定它的 JSON 形狀", c.Name, c.Type)
		}
	}
}
