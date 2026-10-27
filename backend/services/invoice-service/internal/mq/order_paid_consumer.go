package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	pkgmq "github.com/Will413028/hawkr/go-pkg/mq"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"github.com/Will413028/hawkr/invoice-service/internal/usecase"
	"go.uber.org/zap"
)

// OrderPaidConsumer 是 C1：`order.paid` → 建立開票 document ＋ operation（§7.1.2 步驟 4–9）。
//
// 🔴 consumer **絕不呼叫 AMEGO**（§6.3.1）。它只做兩件事：寫 inbox（去重＋保存 payload）、
// 在同一交易內建立 document ＋ operation。真正打 AMEGO 的是 INV.op 的 lease worker。
// 理由：pkg/mq 的重試是「失敗 → republish 到 {queue}.retry fanout → 立刻回到原 queue」，
// 沒有退避；把外部 HTTP 放進 handler 等於在 broker 上做無退避的重打，而每一次重打都可能
// 真的開出一張發票。
type OrderPaidConsumer struct {
	inbox     domain.InboxRepository
	issuance  IssuancePreparer
	orderFact OrderFactsClient
	payFact   PaymentFactsClient
	applier   AdjustmentApplier
	docs      domain.DocumentRepository
	now       func() time.Time
	log       *zap.Logger
}

// NewOrderPaidConsumer 組出 handler 的依賴。now 為 nil 時用 time.Now().UTC()。
func NewOrderPaidConsumer(
	inbox domain.InboxRepository,
	issuance IssuancePreparer,
	orderFact OrderFactsClient,
	payFact PaymentFactsClient,
	applier AdjustmentApplier,
	docs domain.DocumentRepository,
	log *zap.Logger,
) *OrderPaidConsumer {
	return &OrderPaidConsumer{
		inbox: inbox, issuance: issuance, orderFact: orderFact, payFact: payFact,
		applier: applier, docs: docs,
		now: func() time.Time { return time.Now().UTC() },
		log: orNopLogger(log),
	}
}

// WithClock 注入時間來源（測試用；不以 sleep 建立不穩定測試）。
func (c *OrderPaidConsumer) WithClock(now func() time.Time) *OrderPaidConsumer {
	if now != nil {
		c.now = now
	}
	return c
}

// Handle 是 pkgmq.HandlerFunc。三條慣例（§6.3.5.4，① 於 2026-08-30 修正）：
//
//	① 反序列化失敗 → return error（retry → DLQ）：重投救不了壞掉的 payload，但
//	   DLQ 讓原文留在 broker 上可查可重放；ACK 丟棄則是永久消失（本服務踩過，見 payload.go）
//	② 業務欄位解析失敗／冪等鍵缺失 → return error（retry → DLQ），**禁止吞成零值**
//	③ 只有真的寫入了才記 Info
func (c *OrderPaidConsumer) Handle(ctx context.Context, routingKey string, body []byte) error {
	var p orderPaidPayload
	if err := json.Unmarshal(body, &p); err != nil {
		// 🔴 **回 error 而不是 ACK 丟棄**（58 §6.3.5.4 慣例① 已同步修正；與 C2 同一條理由）。
		//
		// 一則被丟棄的 order.paid ＝ 一張已收款的訂單永遠不開發票，那是法定義務未履行，
		// 而且**不會在任何一張表上留下痕跡**（連 inbox 列都沒有）—— M8
		// invoice_paid_orders_without_operation 的基準是「inbox 裡有這一列」，同樣看不到。
		// 回 error 之後訊息原文留在 invoice.order.paid.dlq，可查可重放。
		//
		// 上界由 MaxRetries=3 保證（di.go 的 ConsumerMaxRetries）；只有 MaxRetries==0
		// 才會退化成 pkg/mq 的 legacy 無限 requeue。
		//
		// M30 照樣遞增：它守的是「這則事件在 invoice_inbox_events 沒有留下任何一列」。
		// 用 Error 而不是 Warn：payload 壞掉代表上游契約出事，不是常態。
		observeEventDropped(string(pkgmq.EventTypeOrderPaid), "unmarshal_failed")
		c.log.Error("invoice.order_paid.unmarshal_failed",
			zap.String("routing_key", routingKey), zap.Error(err))
		return fmt.Errorf("invoice: order.paid unmarshal (routing_key=%s): %w", routingKey, err)
	}

	if p.OrderNo == "" {
		observeEventDropped(string(pkgmq.EventTypeOrderPaid), "empty_order_no")
		return fmt.Errorf("invoice: order.paid payload has empty order_no (event_id=%s)", p.EventID)
	}
	occurredAt, err := parseEventTime(p.OccurredAt)
	if err != nil {
		return fmt.Errorf("invoice: order.paid %s: %w", p.OrderNo, err)
	}
	dedupKey := orderPaidDedupKey(p.OrderNo)
	if err := checkDedupKey(dedupKey); err != nil {
		return err
	}

	now := c.now()
	aggregateRef := p.OrderNo
	inboxID, inserted, err := c.inbox.Insert(ctx, domain.NewInboxEvent{
		EventID:          p.EventID,
		EventType:        string(pkgmq.EventTypeOrderPaid),
		RoutingKey:       routingKey,
		Exchange:         pkgmq.ExchangeOrder,
		BusinessDedupKey: dedupKey,
		AggregateRef:     &aggregateRef,
		Payload:          json.RawMessage(body),
		Status:           inboxStatusReceived,
		// 🔴 **不是 `now`**：這一列在 handler 還沒跑完之前就已經在兜底候選集合裡了，
		// 寫 now ⇒ INV.dl 的下一輪會合法搶走一列**正在被處理**的事件。
		// 落庫當下先給自己 handlerLease 這麼長的租約，見該常數的註解。
		NextAttemptAt: now.Add(handlerLease),
		OccurredAt:    occurredAt,
		Now:           now,
	})
	if err != nil {
		return fmt.Errorf("invoice: order.paid %s: insert inbox: %w", p.OrderNo, err)
	}
	if !inserted {
		// 正常的去重結果不是錯誤 ⇒ 直接 ACK，不重做業務。
		//
		// ⚠️ 這一行同時是「pkg/mq 的重試對本 handler 無效」的來源：重投的第二次會停在
		// 這裡什麼都不做。所以失敗的歸宿**不是** DLQ 而是 inbox 的兜底掃描（見 handedOff）。
		return nil
	}

	res, err := c.prepareFromPayload(ctx, p)
	if err != nil {
		c.handedOff(ctx, inboxID, p.OrderNo, now, err)
		return err
	}

	// 慣例③：真的寫入了才記 Info。
	c.log.Info("invoice.order_paid.prepared",
		zap.String("order_no", p.OrderNo),
		zap.Int64("document_id", res.DocumentID),
		zap.Int64("operation_id", res.OperationID),
		zap.String("document_status", res.DocumentStatus),
		zap.String("operation_status", res.OperationStatus),
		zap.Bool("already_existed", res.AlreadyExisted))

	c.markInboxProcessed(ctx, inboxID, p.OrderNo)
	c.reclaimDeferred(ctx, p.OrderNo)
	return nil
}

// prepareFromPayload 是 §7.1.2 步驟 5–9：讀三份事實、跑決策樹、落庫。
//
// 抽成具名函式而不是留在 Handle 內：**兜底掃描（INV.dl）要重跑的就是這一段**。留在
// Handle 裡的話，重跑的唯一辦法是「把原始訊息再送一次 MQ」，而那條路被 Insert 的去重
// 擋死（第二次進 handler 就 return nil）——也就是說，那樣寫出來的兜底掃描不會有任何效果。
//
// 這裡刻意不碰 invoice_inbox_events 的任何狀態：兩個呼叫端對「這一列接下來怎麼標」的
// 答案不同（handler 自己標 PROCESSED；worker 交給 pkg/outbox.Worker 標），把標記寫進
// 這裡會讓 worker 那半在 MarkDone 之前就先動過狀態。
func (c *OrderPaidConsumer) prepareFromPayload(ctx context.Context, p orderPaidPayload) (usecase.PrepareResult, error) {
	// 步驟 5：讀三份事實。前兩份走 gRPC（品項與買家發票偏好刻意不上 MQ，§6.3.2.6），
	// 第三份（seller_tax_profiles ＋ invoice_provider_accounts）由 Prepare 自己讀。
	// buyer_id 取自 payload（其值由 order-service 自 orders.buyer_id 寫出）＝「取自訂單」，
	// 這是 I-47 歸屬驗證唯一合法的來源。
	facts, err := c.orderFact.GetOrderInvoiceSnapshot(ctx, p.OrderNo, p.BuyerID)
	if err != nil {
		return usecase.PrepareResult{}, fmt.Errorf("invoice: order.paid %s: order snapshot: %w", p.OrderNo, err)
	}
	pay, err := c.payFact.GetPaymentFacts(ctx, p.OrderNo)
	if err != nil {
		return usecase.PrepareResult{}, fmt.Errorf("invoice: order.paid %s: payment facts: %w", p.OrderNo, err)
	}

	res, err := c.issuance.Prepare(ctx, usecase.OrderPaidInput{
		OrderNo:                     p.OrderNo,
		BuyerID:                     p.BuyerID,
		StoreID:                     p.StoreID,
		SnapshotVersion:             facts.SnapshotVersion,
		PaidAt:                      pay.PaidAt(),
		Snapshot:                    facts.Snapshot,
		BuyerTaxID:                  facts.BuyerTaxID,
		BuyerTitle:                  facts.BuyerTitle,
		CarrierType:                 facts.CarrierType,
		CarrierID1:                  facts.CarrierID1,
		CarrierID2:                  facts.CarrierID2,
		DonationCode:                facts.DonationCode,
		NotifyEmail:                 facts.NotifyEmail,
		HasPlatformFundedDiscount:   facts.HasPlatformFundedDiscount,
		IsConsolidatedOrder:         facts.IsConsolidatedOrder,
		PreferenceOwnershipMismatch: facts.PreferenceOwnershipMismatch,
		CorrelationID:               p.CorrelationIDMetadata,
	})
	if err != nil {
		return usecase.PrepareResult{}, fmt.Errorf("invoice: order.paid %s: prepare: %w", p.OrderNo, err)
	}
	return res, nil
}

// ReprocessFromInbox 是 §7.2.5 第二列（到期重試）的入口：拿一列**已經落庫**的 ORDER_PAID
// 事件重跑 §7.1.2 步驟 5–9。呼叫端是 INV.dl 的兜底掃描（internal/worker/deadline_worker.go）。
//
// 🔴 它**不碰 invoice_inbox_events 的狀態**（不 MarkProcessed、不 MarkFailed）：標記完全
// 交給 pkg/outbox.Worker，那是它的硬契約（Process 回 nil 之後才 MarkDone）。在這裡順手標
// 一次 ⇒ Worker 的 MarkDone 會打在一列已經離開候選集合的資料上，rows=0 靜默、attempt
// 計數與退避全部失準。
//
// 為什麼不需要上游重發：payload 就是原始 MQ body（Handle 內 `json.RawMessage(body)`），
// 而 PurgeProcessedInboxPayloads 只清 PROCESSED／SKIPPED ⇒ 卡住的列 payload 永遠還在。
func (c *OrderPaidConsumer) ReprocessFromInbox(ctx context.Context, ev domain.InboxEvent) error {
	var p orderPaidPayload
	if err := json.Unmarshal(ev.Payload, &p); err != nil {
		return fmt.Errorf("invoice: inbox %d: unmarshal payload: %w", ev.ID, err)
	}
	if p.OrderNo == "" {
		// payload 被 90 天保留規則覆寫成 `{}` 之後會走到這裡 ⇒ fail-loud。
		// 同 refundFromInbox：不得當成「一張沒有訂單的發票」開下去。
		return fmt.Errorf("invoice: inbox %d has no order_no (payload purged?)", ev.ID)
	}

	res, err := c.prepareFromPayload(ctx, p)
	if err != nil {
		return err
	}

	c.log.Info("invoice.order_paid.reprocessed",
		zap.Int64("inbox_id", ev.ID),
		zap.String("order_no", p.OrderNo),
		zap.Int32("prior_attempts", ev.AttemptCount),
		zap.Int64("document_id", res.DocumentID),
		zap.Int64("operation_id", res.OperationID),
		zap.String("document_status", res.DocumentStatus),
		zap.String("operation_status", res.OperationStatus),
		zap.Bool("already_existed", res.AlreadyExisted))

	// 與 Handle 同一條路徑：付款事件補跑成功之後，先到的退款事件才有 document 可以掛。
	// 少了它，被兜底掃描救回來的訂單其 DEFERRED 退款列會永遠留在 DEFERRED——因為反向
	// 認領只在 Handle 內跑，而 Handle 對這筆訂單永遠不會再跑第二次（去重擋住）。
	//
	// ⚠️ 已知限制（v2 前置）：ListDeferredByAggregateRef 只看 status='DEFERRED' 不看租約，
	// 所以它與 Handle 併發時沒有 CAS 保護。v1 的兜底掃描只掃 ORDER_PAID、不掃 DEFERRED，
	// 窗口與「兩個 consumer 副本同時處理同一筆」等價，不是本波新增的形狀。
	c.reclaimDeferred(ctx, p.OrderNo)
	return nil
}

// handedOff 記下「這一列的歸宿是兜底掃描」。
//
// 🔴 它不是裝飾性的 log：pkg/mq 的重試對本 handler **無效**（重投的第二次會被 Insert 的
// 去重擋掉並 return nil），所以 `mq_consume_total{result="dlq"}` 對 order.paid 結構上
// 恆為 0 ——失敗在 MQ 那一側完全看不見。在補上 INV.dl 的兜底掃描之前，這條路徑的實際
// 結果是「該列永遠停在 RECEIVED、發票永遠不會開、沒有任何告警」（dev 上實際卡了 4 筆）。
func (c *OrderPaidConsumer) handedOff(_ context.Context, inboxID int64, orderNo string, now time.Time, cause error) {
	c.log.Error("invoice.order_paid.handed_off_to_deadline_scan",
		zap.Int64("inbox_id", inboxID),
		zap.String("order_no", orderNo),
		zap.Time("retry_not_before", now.Add(handlerLease)),
		zap.String("impact", "本則訊息不會再進入 handler（去重）；改由 INV.dl 的 inbox 兜底掃描重跑"),
		zap.Error(cause))
}

// markInboxProcessed 把這一列推出兜底候選集合。
//
// 🔴 這裡刻意**不回 error**，理由與 reclaimDeferred 同源：業務已經完成（document 建好了），
// 回 error 只會讓 pkg/mq 重投，而重投會在 Insert 的去重那一步 return nil、什麼都不重做，
// 該列照樣停在 RECEIVED。標記失敗的正確歸宿是「留在 RECEIVED」——它仍在 idx_invinbox_due
// 的候選集合內由兜底掃描重試（Prepare 冪等），所以這裡的責任只有一個：記成 Error 讓它看得見。
func (c *OrderPaidConsumer) markInboxProcessed(ctx context.Context, inboxID int64, orderNo string) {
	ok, err := c.inbox.MarkProcessed(ctx, inboxID, c.now())
	if err != nil {
		c.log.Error("invoice.order_paid.mark_inbox_processed_failed",
			zap.String("order_no", orderNo), zap.Int64("inbox_id", inboxID), zap.Error(err))
		return
	}
	if !ok {
		// allowed-from 是 RECEIVED／DEFERRED／FAILED。走到這裡代表有別的路徑先動過它。
		c.log.Error("invoice.order_paid.mark_inbox_processed_no_rows",
			zap.String("order_no", orderNo), zap.Int64("inbox_id", inboxID))
	}
}

// reclaimDeferred 是 §6.3.6 的**反向認領**：付款事件後到時，把先到的退款事件從 DEFERRED
// 撈回來依 received_at 序套用。
//
// 🔴 這裡刻意**不回 error**。document 已經建好、inbox 的 ORDER_PAID 列也已經寫入 ⇒ 回
// error 只會讓 pkg/mq 重投，而重投會在 Insert 的去重那一步 return nil，什麼都不會重做。
// 反向認領失敗的正確歸宿是「那幾列留在 DEFERRED」——它們仍在 idx_invinbox_due 的候選集合
// 內，由兜底掃描（§7.2.5 第二列）重試，且有 `invoice_inbox_stranded_rows`（M30）
// ＋ InvoiceInboxStranded 的 7 天告警守著。
// 所以這裡的責任只有一個：**把它記成 Error，讓它看得見**。
func (c *OrderPaidConsumer) reclaimDeferred(ctx context.Context, orderNo string) {
	pending, err := c.inbox.ListDeferredByAggregateRef(ctx, orderNo)
	if err != nil {
		c.log.Error("invoice.order_paid.list_deferred_failed",
			zap.String("order_no", orderNo), zap.Error(err))
		return
	}
	if len(pending) == 0 {
		return
	}

	doc, err := c.docs.GetBySourceRef(ctx, orderNo)
	if err != nil {
		c.log.Error("invoice.order_paid.reclaim_document_missing",
			zap.String("order_no", orderNo), zap.Int("deferred_rows", len(pending)), zap.Error(err))
		return
	}

	for _, ev := range pending {
		in, perr := refundFromInbox(ev)
		if perr != nil {
			c.log.Error("invoice.order_paid.reclaim_payload_unusable",
				zap.String("order_no", orderNo), zap.Int64("inbox_id", ev.ID), zap.Error(perr))
			continue
		}
		skip, aerr := c.applier.ApplyRefund(ctx, in, doc)
		if aerr != nil {
			c.log.Error("invoice.order_paid.reclaim_apply_failed",
				zap.String("order_no", orderNo), zap.Int64("inbox_id", ev.ID),
				zap.String("refund_ref", in.RefundRef), zap.Error(aerr))
			continue
		}
		if skip != nil {
			// 認領回來卻判定不需要調整（例如那張 document 是 PLATFORM_RECEIPT）
			// ⇒ SKIPPED ＋ disposition，不是 PROCESSED。
			if _, merr := c.inbox.MarkSkipped(ctx, ev.ID, skip.Disposition, skip.Reason, c.now()); merr != nil {
				c.log.Error("invoice.order_paid.reclaim_mark_skipped_failed",
					zap.String("order_no", orderNo), zap.Int64("inbox_id", ev.ID), zap.Error(merr))
				continue
			}
			c.log.Info("invoice.order_paid.reclaim_skipped",
				zap.String("order_no", orderNo), zap.Int64("inbox_id", ev.ID),
				zap.String("refund_ref", in.RefundRef),
				zap.String("disposition", skip.Disposition), zap.String("reason", skip.Reason))
			continue
		}
		if _, merr := c.inbox.MarkProcessed(ctx, ev.ID, c.now()); merr != nil {
			c.log.Error("invoice.order_paid.reclaim_mark_processed_failed",
				zap.String("order_no", orderNo), zap.Int64("inbox_id", ev.ID), zap.Error(merr))
			continue
		}
		c.log.Info("invoice.order_paid.reclaimed_refund",
			zap.String("order_no", orderNo), zap.Int64("inbox_id", ev.ID),
			zap.String("refund_ref", in.RefundRef))
	}
}

// refundFromInbox 把一列已落庫的 inbox payload 還原成 RefundFinalized。
//
// payload 被 90 天保留規則覆寫成 `{}` 之後這裡會拿到空的 refund_ref ⇒ fail-loud，
// 不得當成「金額 0 的退款」套用下去。
func refundFromInbox(ev domain.InboxEvent) (RefundFinalized, error) {
	var p orderRefundFinalizedPayload
	if err := json.Unmarshal(ev.Payload, &p); err != nil {
		return RefundFinalized{}, err
	}
	if p.RefundRef == "" {
		return RefundFinalized{}, fmt.Errorf("invoice: inbox %d has no refund_ref (payload purged?)", ev.ID)
	}
	// 走與 handler 完全同一支轉換（payload.go 的 toRefundFinalized）：兩份各自組裝
	// 的話，新增 payload 欄位只會被接上其中一條路徑，而漏掉的那條正是這一條。
	return p.toRefundFinalized(ev.OccurredAt)
}

func orNopLogger(l *zap.Logger) *zap.Logger {
	if l == nil {
		return zap.NewNop()
	}
	return l
}
