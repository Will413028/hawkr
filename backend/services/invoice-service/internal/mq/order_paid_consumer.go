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
		NextAttemptAt:    now.Add(handlerLease),
		OccurredAt:       occurredAt,
		Now:              now,
	})
	if err != nil {
		return fmt.Errorf("invoice: order.paid %s: insert inbox: %w", p.OrderNo, err)
	}
	if !inserted {
		return nil
	}

	res, err := c.prepareFromPayload(ctx, p)
	if err != nil {
		c.handedOff(ctx, inboxID, p.OrderNo, now, err)
		return err
	}

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

func (c *OrderPaidConsumer) prepareFromPayload(ctx context.Context, p orderPaidPayload) (usecase.PrepareResult, error) {
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

	c.reclaimDeferred(ctx, p.OrderNo)
	return nil
}

func (c *OrderPaidConsumer) handedOff(_ context.Context, inboxID int64, orderNo string, now time.Time, cause error) {
	c.log.Error("invoice.order_paid.handed_off_to_deadline_scan",
		zap.Int64("inbox_id", inboxID),
		zap.String("order_no", orderNo),
		zap.Time("retry_not_before", now.Add(handlerLease)),
		zap.String("impact", "本則訊息不會再進入 handler（去重）；改由 INV.dl 的 inbox 兜底掃描重跑"),
		zap.Error(cause))
}

func (c *OrderPaidConsumer) markInboxProcessed(ctx context.Context, inboxID int64, orderNo string) {
	ok, err := c.inbox.MarkProcessed(ctx, inboxID, c.now())
	if err != nil {
		c.log.Error("invoice.order_paid.mark_inbox_processed_failed",
			zap.String("order_no", orderNo), zap.Int64("inbox_id", inboxID), zap.Error(err))
		return
	}
	if !ok {
		c.log.Error("invoice.order_paid.mark_inbox_processed_no_rows",
			zap.String("order_no", orderNo), zap.Int64("inbox_id", inboxID))
	}
}

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

func refundFromInbox(ev domain.InboxEvent) (RefundFinalized, error) {
	var p orderRefundFinalizedPayload
	if err := json.Unmarshal(ev.Payload, &p); err != nil {
		return RefundFinalized{}, err
	}
	if p.RefundRef == "" {
		return RefundFinalized{}, fmt.Errorf("invoice: inbox %d has no refund_ref (payload purged?)", ev.ID)
	}
	return p.toRefundFinalized(ev.OccurredAt)
}

func orNopLogger(l *zap.Logger) *zap.Logger {
	if l == nil {
		return zap.NewNop()
	}
	return l
}
