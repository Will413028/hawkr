package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Will413028/hawkr/go-pkg/database"
	pkgmq "github.com/Will413028/hawkr/go-pkg/mq"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"go.uber.org/zap"
)

// OrderCancelledConsumer 是 C3：`order.cancelled` → 在本地記下「這張訂單已取消」（#2904）。
//
// # 它解決的問題
//
// 端點 8（B04 訂單發票區）在「這張訂單沒有任何 invoice_documents 列」時，本來一律回
// state=AWAITING_PAYMENT ＋「付款完成後會立即開立發票」。那句話對**已取消**的訂單
// 永遠不會成真，而買家在同一個畫面上方看到的是紅色的「已取消」時間軸。
// invoice-service 的讀路徑看不到訂單狀態 ⇒ 那個事實必須由這條 queue 送進來。
//
// # 為什麼它與 C1／C2 的形狀不同：整個 handler 在**一個交易**內
//
// 🔴 C1／C2 是「先寫 inbox 去重列，再做業務」，因為它們的業務要打 gRPC（訂單快照、
// 付款事實），不能放進交易。那個順序有一個已知代價：重投的第二次會被去重擋掉並
// return nil ⇒ `mq_consume_total{result="dlq"}` 對它們**結構上恆為 0**，失敗的歸宿
// 是 inbox 兜底掃描而不是 DLQ。
//
// C3 的業務只有一次本地 INSERT，沒有任何外部呼叫 ⇒ 可以（也應該）把去重列與業務
// 寫入放進同一個交易。這樣做換到三件事：
//
//	① **失敗時 inbox 列一起 rollback** ⇒ 重投會真的重跑，而不是被自己剛寫下的去重鍵擋住
//	② ⇒ MaxRetries 打滿之後訊息**真的會進 DLQ**（原文留在 broker 可查可重放）
//	③ ⇒ 這條 queue **不可能**留下卡住的 inbox 列：ORDER_CANCELLED 只會是 PROCESSED，
//	   或者根本沒有那一列。所以它不需要兜底掃描，也不會餵養 M30
//	   `invoice_inbox_stranded_rows`（INV.dl 的 FetchDueByType 只掃 ORDER_PAID，
//	   若這裡留得下 RECEIVED 列，它會是一列**沒有任何執行者**的孤兒）。
//
// ⚠️ ③ 是本設計的關鍵不變式，由 TestOrderCancelledLeavesNoNonTerminalInboxRow
// （整合測試，真 DB）守住。改成「先寫 inbox 再另起交易寫終態」會靜默破壞它。
//
// # 🔴 已知缺口：producer 不是對每一次取消都發事件
//
// order-service 的 `OrderService.insertCancelledOutbox` 第一件事是：
//
//	StockTxRecordEntity deductRecord = stockTxRecordRepository.findDeductByOrderNo(orderNo);
//	if (deductRecord == null || !STOCK_TX_STATUS_SUCCESS.equals(deductRecord.getStatus())) {
//	    return;   // ← 不落 outbox
//	}
//
// 也就是說 `order.cancelled` 的語意其實是「**有庫存要歸還的**訂單取消了」，不是
// 「訂單取消了」。三個取消入口（買家取消／售後內部取消／逾時・停權取消）全部經過
// 它，所以三者一律受這個守衛拘束。
//
// 對本服務的後果：沒有成功扣減紀錄的取消訂單收不到事件 ⇒ 它的 B04 仍會顯示舊的錯誤文案。
//
// ⚠️ **實查之後這個缺口比它看起來窄**：`OrderCreationTxHandler` 的兩條建單路徑
// （`createOrderInTx` / `createOrderInTxExtended`）都在**建單交易內**把該筆
// stock_tx_record 更新成 `DEDUCT` ＋ `SUCCESS`，而 `StockCompensationWorker` 只
// 新增 `RESTORE` 列、不改 `DEDUCT` 列 ⇒ 一張成功建立的訂單必然有那筆紀錄，守衛必然通過。
// #2904 回報的付款逾時自動取消走的正是這條主線，本波修得到。
//
// 真正落在守衛外的只有「訂單存在但沒有成功扣減紀錄」這個組合，而它在現行建單路徑下
// 建不出來。所以這裡登記的是**契約風險不是已知漏修**：`order.cancelled` 的語意與它的
// 名字不一致，日後有人新增一條不扣庫存的建單路徑（例如零元訂單、贈品單）時，
// 那條路徑的取消會靜默收不到事件，而**沒有任何東西會報**。
// 要根治得改 producer 的守衛，而那個守衛的存在理由是庫存歸還 ⇒ 屬 order-service
// 擁有者的裁決，不在本波範圍。
type OrderCancelledConsumer struct {
	tx        database.TxManager
	inbox     domain.InboxRepository
	cancelled domain.CancelledOrderRepository
	now       func() time.Time
	log       *zap.Logger
}

// NewOrderCancelledConsumer 組出 handler 的依賴。
func NewOrderCancelledConsumer(
	tx database.TxManager,
	inbox domain.InboxRepository,
	cancelled domain.CancelledOrderRepository,
	log *zap.Logger,
) *OrderCancelledConsumer {
	return &OrderCancelledConsumer{
		tx: tx, inbox: inbox, cancelled: cancelled,
		now: func() time.Time { return time.Now().UTC() },
		log: orNopLogger(log),
	}
}

// WithClock 注入時間來源（測試用；不以 sleep 建立不穩定測試）。
func (c *OrderCancelledConsumer) WithClock(now func() time.Time) *OrderCancelledConsumer {
	if now != nil {
		c.now = now
	}
	return c
}

// Handle 是 pkgmq.HandlerFunc，沿用 §6.3.5.4 的三條慣例：
//
//	① 反序列化失敗 → return error（retry → DLQ），**不是** ACK 丟棄
//	② 業務欄位解析失敗／冪等鍵缺失 → return error，禁止吞成零值
//	③ 只有真的寫入了才記 Info
func (c *OrderCancelledConsumer) Handle(ctx context.Context, routingKey string, body []byte) error {
	var p orderCancelledPayload
	if err := json.Unmarshal(body, &p); err != nil {
		// 與 C1／C2 同一條理由，但這裡的代價小得多（顯示文案錯，不是發票沒開）。
		// 仍然 return error：ACK 丟棄的那一則在**每一個觀測點**都看不見
		// （inbox 沒有列、DLQ 是 0、業務 metric 的基準是「有那一列」）。
		observeEventDropped(string(pkgmq.EventTypeOrderCancelled), "unmarshal_failed")
		c.log.Error("invoice.order_cancelled.unmarshal_failed",
			zap.String("routing_key", routingKey), zap.Error(err))
		return fmt.Errorf("invoice: order.cancelled unmarshal (routing_key=%s): %w", routingKey, err)
	}

	if p.OrderNo == "" {
		observeEventDropped(string(pkgmq.EventTypeOrderCancelled), "empty_order_no")
		return fmt.Errorf("invoice: order.cancelled payload has empty order_no (event_id=%s)", p.EventID)
	}
	if p.BuyerID <= 0 {
		// 🔴 buyer_id 是讀路徑的**授權述詞**（Tier A），不是可有可無的欄位。
		// 吞成 0 會寫出一列永遠對不到任何買家的死資料：畫面上沒有任何症狀
		// （買家仍看到舊的錯誤文案），而 DB 裡多了一列看起來已經修好的紀錄
		// —— 那比沒有那一列更糟，它會讓下一個人相信這條路徑已經在動了。
		observeEventDropped(string(pkgmq.EventTypeOrderCancelled), "empty_buyer_id")
		return fmt.Errorf("invoice: order.cancelled %s has non-positive buyer_id %d", p.OrderNo, p.BuyerID)
	}
	occurredAt, err := parseEventTime(p.OccurredAt)
	if err != nil {
		return fmt.Errorf("invoice: order.cancelled %s: %w", p.OrderNo, err)
	}
	dedupKey := orderCancelledDedupKey(p.OrderNo)
	if err := checkDedupKey(dedupKey); err != nil {
		return err
	}

	now := c.now()
	aggregateRef := p.OrderNo
	deduped := false

	// 🔴 一個交易包住三件事：去重列、業務事實、終態標記。
	// 任何一步失敗 ⇒ 整批 rollback ⇒ 去重鍵沒有留下 ⇒ 重投會真的重跑 ⇒ 打滿
	// MaxRetries 之後進 DLQ。這正是 C1／C2 拿不到的性質（見型別註解）。
	err = c.tx.WithTx(ctx, func(ctx context.Context) error {
		inboxID, inserted, err := c.inbox.Insert(ctx, domain.NewInboxEvent{
			EventID:          p.EventID,
			EventType:        string(pkgmq.EventTypeOrderCancelled),
			RoutingKey:       routingKey,
			Exchange:         pkgmq.ExchangeOrder,
			BusinessDedupKey: dedupKey,
			AggregateRef:     &aggregateRef,
			Payload:          json.RawMessage(body),
			Status:           inboxStatusReceived,
			// 寫 `now` 而不是 `now + handlerLease`：那個租約是給 C1／C2 用的，
			// 它防的是「兜底掃描搶走一列正在被處理的事件」。本 handler 在同一個
			// 交易內就把這一列推進終態 ⇒ 它從來不會以非終態的形式對外可見，
			// 沒有任何掃描器搶得走它（INV.dl 的 FetchDueByType 也只撈 ORDER_PAID）。
			NextAttemptAt: now,
			OccurredAt:    occurredAt,
			Now:           now,
		})
		if err != nil {
			return fmt.Errorf("insert inbox: %w", err)
		}
		if !inserted {
			// 正常的去重結果不是錯誤 ⇒ 交易正常結束、handler ACK，不重做業務。
			deduped = true
			return nil
		}

		if _, err := c.cancelled.Insert(ctx, domain.CancelledOrder{
			OrderNo:      p.OrderNo,
			BuyerID:      p.BuyerID,
			CancelReason: p.CancelReason,
			OccurredAt:   occurredAt,
			Now:          now,
		}); err != nil {
			return fmt.Errorf("insert cancelled order: %w", err)
		}

		// 同一個交易內標終態。allowed-from 含 RECEIVED ⇒ 這裡必定 rows=1；
		// rows=0 代表 SQL 述詞被改過，那是不變式破了，要 fail-loud 而不是記一行 log
		// 就算了（C1 那邊只能記 log，因為它的業務已經完成、rollback 不回去）。
		ok, err := c.inbox.MarkProcessed(ctx, inboxID, now)
		if err != nil {
			return fmt.Errorf("mark inbox processed: %w", err)
		}
		if !ok {
			return fmt.Errorf("mark inbox %d processed affected 0 rows (allowed-from 述詞被改過？)", inboxID)
		}
		return nil
	})
	if err != nil {
		// 失敗的歸宿**是 DLQ**（本 handler 全程單交易，去重列已隨之 rollback）。
		// 這一行與 C1 的 handed_off_to_deadline_scan 刻意講不同的話：兩條路徑的
		// 歸宿真的不同，寫成一樣會讓 on-call 去翻一張不會有東西的表。
		c.log.Error("invoice.order_cancelled.failed",
			zap.String("order_no", p.OrderNo),
			zap.String("impact", "整個交易已 rollback（含去重列）；本則訊息會重投，打滿 MaxRetries 後進 invoice.order.cancelled.dlq"),
			zap.Error(err))
		return fmt.Errorf("invoice: order.cancelled %s: %w", p.OrderNo, err)
	}

	if deduped {
		return nil
	}
	// 慣例③：真的寫入了才記 Info。
	c.log.Info("invoice.order_cancelled.recorded",
		zap.String("order_no", p.OrderNo),
		zap.Int64("buyer_id", p.BuyerID),
		zap.String("cancel_reason", p.CancelReason),
		zap.Time("occurred_at", occurredAt))
	return nil
}
