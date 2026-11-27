package mq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	pkgmq "github.com/Will413028/hawkr/go-pkg/mq"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"go.uber.org/zap"
)

const (
	inboxStatusReceived = "RECEIVED"
	inboxStatusDeferred = "DEFERRED"
)

const deferWindow = 15 * time.Minute

const handlerLease = 180 * time.Second

// RefundFinalizedConsumer 是 C2：`order.refund.finalized` → 判定作廢／折讓（§6.3.2.3）。
//
// 兩件事必須同時成立，缺一都會讓一筆退款永遠不產生折讓單而且沒有任何錯誤：
//   - 冪等鍵是 `REFUND:{refund_ref}` 而**不是** event_id（aftersales 的 RefundSyncWorker
//     每小時重發、每次換新 UUID ⇒ 用 event_id 等於每小時開一張新折讓單）
//   - 查不到 document 時 inbox 列照寫、status 落 DEFERRED、**ACK**（不丟棄、不 NACK）：
//     跨 queue 無順序保證，退款先於付款抵達是正常情況不是異常
type RefundFinalizedConsumer struct {
	inbox   domain.InboxRepository
	docs    domain.DocumentRepository
	applier AdjustmentApplier
	now     func() time.Time
	log     *zap.Logger
}

// NewRefundFinalizedConsumer 組出 handler 的依賴。
func NewRefundFinalizedConsumer(
	inbox domain.InboxRepository,
	docs domain.DocumentRepository,
	applier AdjustmentApplier,
	log *zap.Logger,
) *RefundFinalizedConsumer {
	return &RefundFinalizedConsumer{
		inbox: inbox, docs: docs, applier: applier,
		now: func() time.Time { return time.Now().UTC() },
		log: orNopLogger(log),
	}
}

// WithClock 注入時間來源（測試用）。
func (c *RefundFinalizedConsumer) WithClock(now func() time.Time) *RefundFinalizedConsumer {
	if now != nil {
		c.now = now
	}
	return c
}

// Handle 是 pkgmq.HandlerFunc。順序逐字照 §6.3.5.4：
//
//  1. 先查該 order_no 的 document，**用查詢結果決定 status 再寫 inbox**（不是先寫
//     RECEIVED 再更新——handler 在寫入當下已經知道前置條件到了沒有，兩段寫入只會多
//     一次 round-trip 與一個中間態）
//  2. Insert → rowsAffected == 0 ⇒ 已處理過，return nil（不是 error）
//  3. DEFERRED ⇒ 就此結束；RECEIVED ⇒ 套用調整
func (c *RefundFinalizedConsumer) Handle(ctx context.Context, routingKey string, body []byte) error {
	var p orderRefundFinalizedPayload
	if err := json.Unmarshal(body, &p); err != nil {
		observeEventDropped(string(pkgmq.EventTypeOrderRefundFinalized), "unmarshal_failed")
		c.log.Error("invoice.order_refund.unmarshal_failed",
			zap.String("routing_key", routingKey), zap.Error(err))
		return fmt.Errorf("invoice: order.refund.finalized unmarshal (routing_key=%s): %w", routingKey, err)
	}

	if p.RefundRef == "" {
		observeEventDropped(string(pkgmq.EventTypeOrderRefundFinalized), "empty_refund_ref")
		return fmt.Errorf("invoice: order.refund.finalized has empty refund_ref (order_no=%s, event_id=%s)",
			p.OrderNo, p.EventID)
	}
	if p.OrderNo == "" {
		return fmt.Errorf("invoice: order.refund.finalized has empty order_no (refund_ref=%s)", p.RefundRef)
	}
	occurredAt, err := parseEventTime(p.OccurredAt)
	if err != nil {
		return fmt.Errorf("invoice: order.refund.finalized %s: %w", p.RefundRef, err)
	}
	in, err := p.toRefundFinalized(occurredAt)
	if err != nil {
		return fmt.Errorf("invoice: order.refund.finalized %s: %w", p.RefundRef, err)
	}
	dedupKey := refundDedupKey(p.RefundRef)
	if err := checkDedupKey(dedupKey); err != nil {
		return err
	}

	now := c.now()

	doc, docErr := c.docs.GetBySourceRef(ctx, p.OrderNo)
	found := docErr == nil
	if docErr != nil && !isNotFound(docErr) {
		return fmt.Errorf("invoice: order.refund.finalized %s: lookup document: %w", p.RefundRef, docErr)
	}

	status := inboxStatusReceived
	var deferredUntil *time.Time
	if !found {
		status = inboxStatusDeferred
		t := now.Add(deferWindow)
		deferredUntil = &t
	}

	aggregateRef := p.OrderNo
	nextAttemptAt := now.Add(handlerLease)
	if deferredUntil != nil {
		nextAttemptAt = *deferredUntil
	}
	inboxID, inserted, err := c.inbox.Insert(ctx, domain.NewInboxEvent{
		EventID:          p.EventID,
		EventType:        string(pkgmq.EventTypeOrderRefundFinalized),
		RoutingKey:       routingKey,
		Exchange:         pkgmq.ExchangeOrder,
		BusinessDedupKey: dedupKey,
		AggregateRef:     &aggregateRef,
		Payload:          json.RawMessage(body),
		Status:           status,
		DeferredUntil:    deferredUntil,
		NextAttemptAt:    nextAttemptAt,
		OccurredAt:       occurredAt,
		Now:              now,
	})
	if err != nil {
		return fmt.Errorf("invoice: order.refund.finalized %s: insert inbox: %w", p.RefundRef, err)
	}
	if !inserted {
		return nil
	}

	if !found {
		c.log.Info("invoice.order_refund.deferred",
			zap.String("order_no", p.OrderNo),
			zap.String("refund_ref", p.RefundRef),
			zap.Time("deferred_until", *deferredUntil))
		return nil
	}

	skip, err := c.applier.ApplyRefund(ctx, in, doc)
	if err != nil {
		if _, merr := c.inbox.MarkFailed(ctx, inboxID, err.Error(), c.now()); merr != nil {
			c.log.Error("invoice.order_refund.mark_inbox_failed_failed",
				zap.Int64("inbox_id", inboxID), zap.Error(merr))
		}
		c.log.Error("invoice.order_refund.apply_failed",
			zap.String("order_no", p.OrderNo),
			zap.String("refund_ref", p.RefundRef),
			zap.Int64("document_id", doc.ID),
			zap.Error(err))
		return fmt.Errorf("invoice: order.refund.finalized %s: apply: %w", p.RefundRef, err)
	}

	if skip != nil {
		if ok, merr := c.inbox.MarkSkipped(ctx, inboxID, skip.Disposition, skip.Reason, c.now()); merr != nil {
			c.log.Error("invoice.order_refund.mark_inbox_skipped_failed",
				zap.Int64("inbox_id", inboxID), zap.Error(merr))
		} else if !ok {
			c.log.Error("invoice.order_refund.mark_inbox_skipped_no_rows",
				zap.Int64("inbox_id", inboxID))
		}
		c.log.Info("invoice.order_refund.skipped",
			zap.String("order_no", p.OrderNo), zap.String("refund_ref", p.RefundRef),
			zap.Int64("document_id", doc.ID),
			zap.String("disposition", skip.Disposition), zap.String("reason", skip.Reason))
		return nil
	}

	if ok, merr := c.inbox.MarkProcessed(ctx, inboxID, c.now()); merr != nil {
		c.log.Error("invoice.order_refund.mark_inbox_processed_failed",
			zap.Int64("inbox_id", inboxID), zap.Error(merr))
	} else if !ok {
		c.log.Error("invoice.order_refund.mark_inbox_processed_no_rows",
			zap.Int64("inbox_id", inboxID))
	}

	c.log.Info("invoice.order_refund.applied",
		zap.String("order_no", p.OrderNo),
		zap.String("refund_ref", p.RefundRef),
		zap.Int64("document_id", doc.ID))
	return nil
}

func isNotFound(err error) bool {
	var de *pkgerrors.DomainError
	return errors.As(err, &de) && int32(de.Code) == bizcode.InvoiceDocumentNotFound
}

// ReprocessFromInbox 是 INV.dl 對退款事件的重跑入口（#3073）。
//
// 🔴 呼叫端只會餵 RECEIVED／FAILED 的列（FetchDueActionableByType 排除了 DEFERRED）：
// DEFERRED 的歸宿是反向認領，兜底掃描碰它就會在失敗時翻成 FAILED ⇒ 反向認領的述詞
// 永久不成立，而且完全無聲。這個約束靠 SQL 保證，不靠這裡的判斷。
//
// 🔴 **不碰 invoice_inbox_events 的狀態**：標記全部交給 pkg/outbox.Worker（Process 回 nil
// 之後才 MarkDone）。在這裡順手標一次，Worker 的 MarkDone 就會打在一列已經離開候選集合
// 的資料上——rows=0 靜默、attempt 計數與退避全部失準。同 OrderPaidConsumer.ReprocessFromInbox。
func (c *RefundFinalizedConsumer) ReprocessFromInbox(ctx context.Context, ev domain.InboxEvent) error {
	in, err := refundFromInbox(ev)
	if err != nil {
		return fmt.Errorf("invoice: inbox %d: %w", ev.ID, err)
	}
	doc, err := c.docs.GetBySourceRef(ctx, in.OrderNo)
	if err != nil {
		return fmt.Errorf("invoice: inbox %d: lookup document for %s: %w", ev.ID, in.OrderNo, err)
	}
	skip, err := c.applier.ApplyRefund(ctx, in, doc)
	if err != nil {
		return fmt.Errorf("invoice: inbox %d: apply refund %s: %w", ev.ID, in.RefundRef, err)
	}
	if skip != nil {
		if ok, merr := c.inbox.MarkSkipped(ctx, ev.ID, skip.Disposition, skip.Reason, c.now()); merr != nil {
			return fmt.Errorf("invoice: inbox %d: mark skipped: %w", ev.ID, merr)
		} else if !ok {
			c.log.Error("invoice.order_refund.reprocess_mark_skipped_no_rows",
				zap.Int64("inbox_id", ev.ID))
		}
		c.log.Info("invoice.order_refund.reprocess_skipped",
			zap.Int64("inbox_id", ev.ID), zap.String("refund_ref", in.RefundRef),
			zap.String("disposition", skip.Disposition), zap.String("reason", skip.Reason))
		return nil
	}
	c.log.Info("invoice.order_refund.reprocessed",
		zap.Int64("inbox_id", ev.ID), zap.String("order_no", in.OrderNo),
		zap.String("refund_ref", in.RefundRef), zap.Int32("prior_attempts", ev.AttemptCount),
		zap.Int64("document_id", doc.ID))
	return nil
}
