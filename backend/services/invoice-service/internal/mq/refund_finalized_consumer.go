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

// invoice_inbox_events.status 的值域（chk_invinbox_status 正面列出五值）。
//
// 集中在此而不是散落在呼叫點：值域的 SSOT 是 §5.1.9 的 DDL，改 DDL 時 grep 得到全部用點。
const (
	inboxStatusReceived = "RECEIVED"
	inboxStatusDeferred = "DEFERRED"
)

// deferWindow 是亂序退款事件的延後長度（§7.2.5 時序圖：`deferred_until = now() + 15min`）。
//
// 它不是「多久之後一定會有 document」的猜測，而是兜底掃描的重試節奏——主路徑是
// 開票成功後的反向認領（§6.3.6 第 2 點），這條只負責「付款事件永遠沒來」的情況。
const deferWindow = 15 * time.Minute

// handlerLease 是 consumer 在 **handler 還在執行期間**替自己這一列預留的租約長度
// （寫進 invoice_inbox_events.next_attempt_at，該欄兼租約欄）。
//
// 🔴 沒有它的話 Insert 會寫 `now` ⇒ 該列**在 handler 還在跑的時候就已經到期**，
// INV.dl 的兜底掃描可以合法搶走一列正在被處理的事件。在兜底掃描接上之前這件事無害
// （沒有人在掃），掛上掃描器的那一刻它就變成真的競態——這正是「新增執行者時要回頭看
// 既有寫入端的假設」的實例。
//
// 下界推導與 INV.dl 的 ClaimLease 同源，而且刻意用**同一個保守版本**：
// 單筆最壞耗時 ＝ internal/client 底下**全部** peer client 的 per-call deadline 加總
// ＋ 1s 本地開銷（amego 30s ＋ media 30s ＋ ORD 3s ＋ PAY 3s ＋ SEL 3s ＋ 1s ＝ 70s）。
//
// 🔴 為什麼不用「這條路徑今天實際會打的那兩支」（ORD 3s ＋ PAY 3s ＋ 本地 ≈ 6.6s）：
// 手列一份「今天會打哪幾支」的清單，對「日後多了第三支外部呼叫」是全盲的——前提破了，
// 測試照樣綠。保守方向（高估）只會要求租約更長，而租約不夠長的後果是**兩個副本同時
// 處理同一列**（正確性），所以這一側必須 fail-closed。守它的是
// internal/worker 的 TestDeadlineScanCannotPreemptARunningHandler。
//
// 🔴 值從 60s 提高到 180s（2026-08-31）。原因不是路徑變慢了，是**量測本來就是錯的**：
// peerClientDeadlines 舊版只認 `const DefaultDeadline = <n> * time.<Unit>`，而
// internal/client/amego 從第一天起就是 `func DefaultDeadlines() Deadlines`（Issue 30s）
// ⇒ 它從未被加進那個加總，上界被少算了 30s，於是 60s 看起來安全。掃描改成 fail-closed
// 之後真實上界是 70s ⇒ 60s 不成立。
// 依《判斷 Rubric》§2.1.4 的優先序，這裡選的是「延長預算」而不是「把檢查改窄」：
// 把檢查改回只算那兩支，等於把剛修好的盲區重新鑿回去。
//
// 代價很小：這個值等於「consumer pod 在 handler 中途死掉之後，那一列要多久才回到兜底
// 候選集合」。INV.dl 的 tick 是 5 分鐘，180s 落在同一個量級內、實際等待時間幾乎不變。
// 上界不存在：候選窗口（status ∈ RECEIVED/DEFERRED/FAILED）不會關閉，所以租約再長也只是
// 延後第一次兜底，不會讓事件永久失去候選資格；但仍刻意小於 dlClaimLease(5min)。
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
		// 🔴 **回 error 而不是 ACK 丟棄**（58 §6.3.5.4 慣例① 已同步修正）。
		//
		// 丟棄的代價與收益不對稱：一則解不開的 order.refund.finalized ＝ 一筆真實退款
		// 永遠不產生折讓單，而那是稅務錯誤；ACK 之後 payload 消失、無法重放，M30 只留下
		// 一個「有東西被丟掉了」的計數。回 error 則是**有界**的——MaxRetries=3（di.go 的
		// ConsumerMaxRetries，與 topology.yaml 的 retry.max_attempts 對拍）⇒ 三次之後
		// pkg/mq 的 deadLetterToOwnDLQ 直投 invoice.order.refund.finalized.dlq，
		// 訊息原文留在 broker 上可查可重放，且 mq_consume_total{result="dlq"} 會亮。
		//
		// ⚠️ 「毒訊息無限重投」的風險只存在於 MaxRetries == 0（pkg/mq 的 legacy
		// infinite-requeue 路徑）。本 subscription 是 3，重投次數有上界。
		//
		// M30 照樣遞增：它守的是「這則事件在 invoice_inbox_events 沒有留下任何一列」，
		// 那件事無論 ACK 還是 DLQ 都成立，而 inbox 是 ADM 異常工作台唯一會去看的地方。
		observeEventDropped(string(pkgmq.EventTypeOrderRefundFinalized), "unmarshal_failed")
		c.log.Error("invoice.order_refund.unmarshal_failed",
			zap.String("routing_key", routingKey), zap.Error(err))
		return fmt.Errorf("invoice: order.refund.finalized unmarshal (routing_key=%s): %w", routingKey, err)
	}

	// 🔴 fail-loud，禁止退回 event_id：用 event_id 去重 ⇒ 每小時對同一筆退款開一張新折讓單。
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
	// 🔴 金額欄在 struct 上是 string（型別對齊 wire），數字在 toRefundFinalized 內解、
	// 失敗 fail-loud。慣例②：業務欄位解析失敗 → error，**禁止吞成零值**——cumulative
	// 吞成 0 ⇒ §6.3.5.4 步驟 3 判成「沒退過」⇒ 該作廢的發票不作廢，且沒有任何訊號。
	//
	// ⚠️ 解析放在 **Insert 之前**：Insert 之後才失敗的話，去重鍵已經落庫，重投會在
	// Insert 那一步 return nil ⇒ 這則事件連 retry 都用不上就永遠停在 RECEIVED。
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
	// 🔴 RECEIVED 分支寫 `now + handlerLease` 而不是 `now`：該欄兼租約欄，寫 now 等於
	// 讓這一列在 handler 還在跑的時候就已經到期、可被兜底掃描搶走（見 handlerLease）。
	// DEFERRED 分支用 deferred_until（15 分鐘）本來就遠大於單筆耗時，不需要另外加。
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
		// 亂序：退款先到。事實已經落在 DB 裡，訊息本身不重要了 ⇒ ACK。
		c.log.Info("invoice.order_refund.deferred",
			zap.String("order_no", p.OrderNo),
			zap.String("refund_ref", p.RefundRef),
			zap.Time("deferred_until", *deferredUntil))
		return nil
	}

	skip, err := c.applier.ApplyRefund(ctx, in, doc)
	if err != nil {
		// inbox 列已經寫進去了（去重鍵保住了），但這一則退款**沒有**被套用 ⇒ 不得 ACK
		// 成「處理完成」，所以回 error。
		//
		// ⚠️ **它不會進 DLQ。** pkg/mq 的 retry 是把同一則 body republish 回原 queue，
		// 而第二次進 handler 會被 Insert 的去重擋掉並 return nil ⇒ retry counter 永遠
		// 停在 1、打不到 MaxRetries、`mq_consume_total{result="dlq"}` 對本 handler
		// **結構上恆為 0**。（曾經這裡寫「走 retry → DLQ、DLQ 會亮」——照那句話去撈
		// DLQ 的人會撈到一個空的東西。）
		//
		// 真正的歸宿是標成 FAILED：它留在表上，last_error 說得出「這筆退款為什麼沒開折讓」。
		//
		// ✅ **這一列有自動執行者**（#3073 起）：INV.dl 的兜底掃描現在同時掃
		// ORDER_REFUND_FINALIZED，走 FetchDueActionableByType（RECEIVED／FAILED，
		// **排除 DEFERRED**），重跑入口是本檔的 ReprocessFromInbox。
		// 這裡曾經寫「今天沒有任何自動執行者」，那是 AdjustmentApplier 還是
		// Unimplemented 的年代。
		//
		// ⚠️ 仍然只有 RECEIVED／FAILED 會被兜底重跑：DEFERRED 的歸宿是**反向認領**
		// （order.paid 到達時 reclaimDeferred 撿走）。兜底掃描碰它就會在失敗時翻成
		// FAILED ⇒ 反向認領的述詞永久不成立、該筆退款再也不會被認領，而且完全無聲。
		// 「DEFERRED 但 order.paid 永遠不會來」那一批（cutover 積壓）不屬於任何自動
		// 執行者，走一次性維運 SQL（docs/runbooks/invoice-inbox-stranded-runbook.md），
		// 偵測面是 `invoice_inbox_stranded_rows`（M30）＋ InvoiceInboxStranded（7 天）。
		// ⚠️ 這裡曾經寫「ADM 異常工作台」與「M29 invoice_inbox_stuck」——**兩者都不存在**
		// （internal/grpc 與 api/proto 對 inbox 零命中；M29 全 repo 只在註解裡）。
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
		// 確定不需要任何發票調整 ⇒ SKIPPED ＋ disposition（§6.3.5.4 步驟 3）。
		// 🔴 **不是 PROCESSED**：那個狀態說「業務動作做完了」，而這裡什麼都沒做。
		// 兩者合併之後，事後查「為什麼這筆退款沒有折讓單」只能靠猜。
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

	// 推出兜底候選集合。不回 error 的理由同 OrderPaidConsumer.markInboxProcessed：
	// 折讓已經套用了，重投會在去重那一步 return nil、什麼都不重做。
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

// isNotFound 判斷 repository 回的是不是「查不到」而不是真的 DB 錯誤。
//
// 兩者必須分開：查不到是亂序的正常分支（走 DEFERRED），DB 錯誤要 fail-loud。
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
		// RECEIVED／FAILED 的列走到這裡代表 document 在事件抵達當下**存在過**
		// （否則 handler 會把它落成 DEFERRED），現在讀不到是真的異常 ⇒ 回 error
		// 讓 Worker 退避重試，不要靜默成功。
		return fmt.Errorf("invoice: inbox %d: lookup document for %s: %w", ev.ID, in.OrderNo, err)
	}
	skip, err := c.applier.ApplyRefund(ctx, in, doc)
	if err != nil {
		return fmt.Errorf("invoice: inbox %d: apply refund %s: %w", ev.ID, in.RefundRef, err)
	}
	if skip != nil {
		// 🔴 這裡**要自己標** SKIPPED，與本檔「不碰狀態」的規則不衝突：那條講的是
		// PROCESSED／FAILED（由 pkg/outbox.Worker 依 Process 的回傳值決定）。
		// SKIPPED 不在 Worker 的字彙裡——它回 nil 只會標 PROCESSED，
		// 而那會讓「刻意不處理」與「處理完成」變成同一件事。
		//
		// 標完仍然 return nil：Worker 接著會 MarkDone，而 MarkProcessed 的
		// allowed-from 是 RECEIVED／DEFERRED／FAILED ⇒ 對已經 SKIPPED 的列 rows=0，
		// 那條路徑本來就把 rows=0 當成「別人先標過了」而不是錯誤。
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
