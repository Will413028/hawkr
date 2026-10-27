-- invoice_inbox_events｜入向事件（去重 ＋ 亂序延後 ＋ 處置留證）
-- SSOT：設計書 58 號 §5.1.9（DDL）＋ §6.3.5.2／§7.2.5（亂序與反向認領）＋ §5.4.6（payload 清理）
--
-- 這張表同時解三個問題：① 去重 UNIQUE(event_id) ② 亂序——refund 早於 paid 抵達時
-- （RabbitMQ 跨 queue 無序）不得丟棄，寫成 DEFERRED 由**反向認領**推進
-- ③ 處置留證——ADM 必須答得出「這筆退款為什麼沒有開折讓」。
--
-- ⚠️ 本表**沒有自己的排程器**（§4.1.3／§4.3.3 第 7 組）：DEFERRED 列由開票流程成功之後的
--    反向認領推進（主路徑）；到期重試（兜底）的候選集合由 idx_invinbox_due 承載，
--    由 **INV.dl（internal/worker/deadline_worker.go）順帶跑**，不新增第 N 支排程器。
--    看到「inbox worker」出現在 PR 裡，先問它為什麼不是反向認領——**但「掛在 INV.dl 上的
--    兜底掃描」正是本節指定的機制，不是它要擋的東西**。
--    （2026-08-30：這個機制先前只存在於 SQL、repo 方法與註解裡，Go 呼叫端是零 ⇒ 失敗的
--     ORDER_PAID 列永遠停在 RECEIVED。dev 上曾有 4 筆卡住、對應訂單零發票文件。）
--
-- ⚠️ 本表**永不刪列**（I-49）：去重鍵是「同一筆退款不會被開第二張折讓單」的唯一防線，
--    而上游 aftersales RefundSyncWorker 的候選述詞是 `status=DONE AND updated_at < NOW-1h`
--    ——DONE 是終態、沒有任何後續寫入會把它推出候選集合，且每次換一個新的 UUID event_id
--    ⇒ 重發沒有時間上界。鍵被刪掉的那一刻，下一輪重發就是一個「全新的事件」。
--    一列鍵的成本是 160 bytes；一張錯開的折讓單的成本是稅務更正。
--    ⇒ 本檔不存在也不得新增任何 DELETE FROM invoice_inbox_events。

-- name: InsertInboxEvent :many
-- ⚠️ ON CONFLICT DO NOTHING **不得指名 conflict target**（§7.6 X05）：本表有**兩個**唯一約束
--    （uq_invinbox_event_id 與 uq_invinbox_business_key），指名其一 ⇒ 另一個衝突時直接拋 23505。
--    不指名時對任何唯一約束衝突都靜默跳過——而在 PostgreSQL 上「讓 DB 拋唯一鍵錯再 catch」
--    根本行不通：交易內任一句 SQL 失敗即整個交易 aborted（25P02），之後每一句都被拒。
-- **RETURNING id** ⇒ 回 1 列＝新事件（id 可用），回 0 列＝已處理過、consumer 直接 ACK。
--    `ON CONFLICT DO NOTHING RETURNING id` 在衝突時不回列，資訊量嚴格大於 rowcount
--    且**零額外 round-trip**。這一欄是必要的：所有 Mark* 都以 id 為鍵，拿不到 id 的話
--    這一列永遠停在 RECEIVED ＋ next_attempt_at=now，也就是永久留在 idx_invinbox_due
--    的兜底候選集合裡，INV.dl 每一輪都會白撈它一次（Prepare 冪等所以不出錯，只是白燒）。
--    用 :many 而不是 :one：衝突時回 0 列是**正常的去重結果**，不該表現成 ErrNoRows 這種錯誤。
-- status **在 INSERT 當下就要決定**，不是先寫 RECEIVED 再更新（§7.1.2）：handler 在同一個交易內
--    已經知道前置條件到了沒有——查得到該 order_no 的 document ⇒ RECEIVED；查不到 ⇒ DEFERRED。
--    兩段寫入只會多一次 round-trip 與一個中間態。
-- 欄名不要沿用別服務的舊名：idempotency_key→business_dedup_key、consumed_at→received_at、
--    order_no→aggregate_ref；本表**沒有** store_id 欄。
INSERT INTO invoice_inbox_events (
  event_id, event_type, routing_key, exchange, business_dedup_key, aggregate_ref,
  payload, status, deferred_until, attempt_count, next_attempt_at, occurred_at, received_at)
VALUES (
  sqlc.arg(event_id), sqlc.arg(event_type), sqlc.arg(routing_key), sqlc.arg(exchange),
  sqlc.arg(business_dedup_key), sqlc.narg(aggregate_ref), sqlc.arg(payload),
  sqlc.arg(status), sqlc.narg(deferred_until), 0,
  sqlc.arg(next_attempt_at), sqlc.arg(occurred_at), sqlc.arg(now))
ON CONFLICT DO NOTHING
RETURNING id;

-- name: ListDeferredInboxEventsByAggregateRef :many
-- 反向認領（主路徑）：開票流程成功寫入 invoice_documents 之後，在**同一事務內**撈出該訂單所有
-- DEFERRED 列，依 received_at 序套用，逐列 MarkInboxEventProcessed（判定不需調整者走
-- MarkInboxEventSkipped ＋ disposition）。
-- ⚠️ 它吃的索引是 idx_invinbox_deferred_aggregate（aggregate_ref，partial WHERE status='DEFERRED'）
--    ——同表另外三個索引都吃不下這個述詞。要改動或移除它之前先回來讀這一句：反向認領是每一次
--    開票成功都會跑一次的熱路徑，而本表永不刪列、單調長大 ⇒ 缺了索引就是一條會隨時間變慢、
--    但永遠不會報錯的路徑。
SELECT * FROM invoice_inbox_events
WHERE status = 'DEFERRED' AND aggregate_ref = sqlc.arg(aggregate_ref)
ORDER BY received_at;

-- name: FetchDueInboxEvents :many
-- 兜底掃描的候選（§7.2.5「到期重試」）。三個狀態正面列出，與 idx_invinbox_due 述詞逐字相同。
-- 沒有獨立排程器：這一輪由 INV.dl 順帶跑。
SELECT * FROM invoice_inbox_events
WHERE status IN ('RECEIVED', 'DEFERRED', 'FAILED')
  AND next_attempt_at <= sqlc.arg(now)
ORDER BY next_attempt_at
LIMIT sqlc.arg(row_limit);

-- name: FetchDueActionableInboxEventsByType :many
-- 兜底掃描的候選，限定單一 event_type，**排除 DEFERRED**。
--
-- 🔴 與 FetchDueInboxEventsByType 的差別只有一格述詞，而那一格是刻意的：
-- DEFERRED 的歸宿是**反向認領**（order.paid 到達時 reclaimDeferred 把它撿走），
-- 兜底掃描碰它就會在失敗時把它翻成 FAILED ⇒ `WHERE status='DEFERRED'` 的述詞
-- 永久不成立 ⇒ 那筆退款再也不會被認領，而且完全無聲。
--
-- 「DEFERRED 但 order.paid 永遠不會來」那一批（cutover 積壓）不屬於任何自動執行者，
-- 走一次性維運 SQL 標 SKIPPED ＋ disposition（docs/runbooks/invoice-inbox-stranded-runbook.md），
-- 偵測面是 CountStaleDeferredInboxEvents 的 7 天告警。
SELECT * FROM invoice_inbox_events
WHERE status IN ('RECEIVED', 'FAILED')
  AND event_type = sqlc.arg(event_type)
  AND next_attempt_at <= sqlc.arg(now)
ORDER BY next_attempt_at
LIMIT sqlc.arg(row_limit);

-- name: FetchDueInboxEventsByType :many
-- 兜底掃描的候選（§7.2.5「到期重試」），**限定單一 event_type**。
-- 述詞是 idx_invinbox_due 的子集，event_type 走 filter。
--
-- 為什麼限定型別而不是在 Go 端 skip：被跳過的列每輪都會重新佔掉 batch 名額而永遠不推進，
-- 於是一種事件的積壓會靜默吃光另一種的處理量。本表 event_type 不可變 ⇒ 用它切分是安全的。
--
-- ⚠️ **退款不要用這一支**（#3073 起）：它含 DEFERRED，而 DEFERRED 的歸宿是反向認領。
--    退款走 FetchDueActionableInboxEventsByType（排除 DEFERRED）。用錯的後果是
--    ListDeferredInboxEventsByAggregateRef 的述詞永久不成立、該筆退款再也不會被認領。
SELECT * FROM invoice_inbox_events
WHERE status IN ('RECEIVED', 'DEFERRED', 'FAILED')
  AND event_type = sqlc.arg(event_type)
  AND next_attempt_at <= sqlc.arg(now)
ORDER BY next_attempt_at
LIMIT sqlc.arg(row_limit);

-- name: ClaimInboxEvent :execrows
-- 租約 CAS（租約欄 ＝ next_attempt_at）。述詞逐字鏡射 FetchDueInboxEvents。
-- 租約基準逐筆在 claim 當下取。
UPDATE invoice_inbox_events
SET next_attempt_at = sqlc.arg(lease_until)
WHERE id = sqlc.arg(id)
  AND status IN ('RECEIVED', 'DEFERRED', 'FAILED')
  AND next_attempt_at <= sqlc.arg(claimable_before);

-- name: MarkInboxEventDeferred :execrows
-- 前置條件未到（找不到 document）⇒ 改 status='DEFERRED' ＋ deferred_until 後 **ACK**（§7.6 X15）。
-- 前者若改成「ACK 丟棄」⇒ 該退款永遠不會產生折讓或取消，之後 paid 事件照常開一張全額發票；
-- 後者若改成 NACK 重試 ⇒ 打滿 MaxRetries 後直投 DLQ，該筆退款靜默消失。
UPDATE invoice_inbox_events
SET status = 'DEFERRED',
    deferred_until = sqlc.arg(deferred_until),
    next_attempt_at = sqlc.arg(deferred_until),
    attempt_count = attempt_count + 1
WHERE id = sqlc.arg(id) AND status IN ('RECEIVED', 'DEFERRED', 'FAILED');

-- name: MarkInboxEventProcessed :execrows
UPDATE invoice_inbox_events
SET status = 'PROCESSED', processed_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status IN ('RECEIVED', 'DEFERRED', 'FAILED');

-- name: MarkInboxEventSkipped :execrows
-- disposition **不得為空**（chk_invinbox_skip_reason 保證 SKIPPED 一定說得出理由）：
-- 例「重複付款的退款」⇒ disposition='IGNORED_DUPLICATE_PAYMENT'。
-- ADM 異常工作台必須答得出「這筆退款為什麼沒開折讓」。
UPDATE invoice_inbox_events
SET status = 'SKIPPED',
    disposition = sqlc.arg(disposition),
    disposition_reason = sqlc.narg(disposition_reason),
    processed_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status IN ('RECEIVED', 'DEFERRED', 'FAILED');

-- name: MarkInboxEventFailed :execrows
UPDATE invoice_inbox_events
SET status = 'FAILED',
    last_error = sqlc.arg(last_error),
    attempt_count = attempt_count + 1,
    next_attempt_at = sqlc.arg(next_attempt_at)
WHERE id = sqlc.arg(id) AND status IN ('RECEIVED', 'DEFERRED', 'FAILED');

-- name: UpdateInboxEventAttempt :execrows
-- pkg/outbox.Worker 的 UpdateAttempt：**還要再試** ⇒ 不動 status，只記次數、退避與錯誤。
--
-- 🔴 attempt_count 用**絕對值**（與 UpdateAttemptInvoiceOutboxEvents 逐字同形），不用
--    `attempt_count + 1`：Worker 傳的是它自己算好的 newAttempt。兩種寫法今天的數值巧合
--    相等，而巧合不是契約——任一端改了計數語意就會靜默漂移（沒有任何東西會轉紅）。
--
-- 🔴 它與 MarkInboxEventFailed 的差別是 status：後者是「打滿次數、停下來讓人看見」，
--    本句是「這一輪失敗、待會再試」。把 status 寫進本句 ⇒ 第一次失敗就變成 FAILED，
--    退避還在但語意已經騙人（ADM 工作台會看到一批其實還在自動重試的「失敗」）。
UPDATE invoice_inbox_events
SET attempt_count = sqlc.arg(attempt_count),
    next_attempt_at = sqlc.arg(next_attempt_at),
    last_error = sqlc.arg(last_error)
WHERE id = sqlc.arg(id) AND status IN ('RECEIVED', 'DEFERRED', 'FAILED');

-- name: CountStaleDeferredInboxEvents :one
-- 告警：status='DEFERRED' AND received_at < now − 7 天 ⇒ 上游 bug，本服務修不了（§7.2.5）。
SELECT COUNT(*) FROM invoice_inbox_events
WHERE status = 'DEFERRED' AND received_at < sqlc.arg(cutoff);

-- name: PurgeProcessedInboxPayloads :execrows
-- §5.4.6：90 天後**只清 payload**，去重鍵永久保留——(event_type, business_dedup_key)／
-- event_id／disposition 一列都不刪不改。
-- 認領（stamp payload_purged_at）與副作用（覆寫 payload）是**同一句 UPDATE**，天然冪等、
-- 不需要租約。Postgres 無 DELETE ... LIMIT，用子查詢框住一輪。
UPDATE invoice_inbox_events
SET payload = '{}'::jsonb, payload_purged_at = sqlc.arg(now)
-- ⚠️ 內層 SELECT 必須 alias（AS sub）並限定欄位，才與 §5.4.6 的 body 有一處差異：
--    processed_at 同時出現在外層與內層，sqlc 1.31.1 判成 ambiguous 而 Postgres 解得開
--    （同 pkg/outbox/query_template.sql.md 末段記載的 quirk）。語意不變。
WHERE id IN (
  SELECT sub.id FROM invoice_inbox_events AS sub
  WHERE sub.status IN ('PROCESSED', 'SKIPPED') AND sub.processed_at < sqlc.arg(cutoff) AND sub.payload_purged_at IS NULL
  ORDER BY sub.id LIMIT sqlc.arg(row_limit))
  AND status IN ('PROCESSED', 'SKIPPED') AND processed_at < sqlc.arg(cutoff) AND payload_purged_at IS NULL;
