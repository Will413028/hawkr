-- invoice_outbox_events｜出向事件（INV.ob，走 pkg/outbox.Worker）
-- SSOT：設計書 58 號 §5.1.10（DDL）＋ §5.2.3 第 6 組（租約 30s，下界 10.2s）
-- 形狀逐句照抄 pkg/outbox/query_template.sql.md 的 canonical template（<table> = invoice_outbox_events）。
-- 那份 template 是「複製用」不是共用程式碼：sqlc 逐服務獨立編譯，而 claim 述詞依規範 ⑦
-- 必須是非 PK 且鏡射各表自己的候選查詢。
--
-- payload 必須在 enqueue 當下（同一交易內）就快照完成——這張表的存在理由就是
-- 「業務寫入與事件發布同一個交易」，事後才組 payload 等於把 outbox 的保證丟掉。

-- name: InsertInvoiceOutboxEvent :one
-- 由業務交易內呼叫（§5.2.3 第 10、11 組的「例外 A」正是靠這一句成立：
-- 推出候選集合的寫入就是狀態轉移本身，副作用是同一交易內的 outbox 列，交易裡沒有網路呼叫）。
INSERT INTO invoice_outbox_events (
  aggregate_type, aggregate_id, event_type, exchange, routing_key, payload,
  status, attempt_count, next_attempt_at, created_at)
VALUES (
  sqlc.arg(aggregate_type), sqlc.arg(aggregate_id), sqlc.arg(event_type),
  sqlc.arg(exchange), sqlc.arg(routing_key), sqlc.arg(payload),
  'pending', 0, sqlc.arg(next_attempt_at), sqlc.arg(now))
RETURNING *;

-- name: FetchPendingInvoiceOutboxEvents :many
-- 刻意不帶 FOR UPDATE SKIP LOCKED —— 在自動提交下跑，SELECT 一回傳鎖就釋放。
-- 多副本互斥來自 ClaimInvoiceOutboxEvents，不是本查詢。吃 idx_invoutbox_pending。
SELECT * FROM invoice_outbox_events
WHERE status = 'pending' AND next_attempt_at <= sqlc.arg(now)
ORDER BY next_attempt_at
LIMIT sqlc.arg(row_limit);

-- name: ClaimInvoiceOutboxEvents :execrows
-- Lease CAS：把 next_attempt_at 推到 lease_until（30s）。rows=1 ⇒ 本副本擁有該列；
-- rows=0 ⇒ 別的副本先搶到，或該列已離開候選集合。
-- status = 'pending' 是規範 ⑦ 要求的非 PK 述詞。
UPDATE invoice_outbox_events
SET next_attempt_at = sqlc.arg(lease_until)
WHERE id = sqlc.arg(id)
  AND status = 'pending'
  AND next_attempt_at <= sqlc.arg(claimable_before);

-- name: UpdateAttemptInvoiceOutboxEvents :exec
UPDATE invoice_outbox_events
SET attempt_count = sqlc.arg(attempt_count),
    next_attempt_at = sqlc.arg(next_attempt_at),
    last_error = sqlc.arg(last_error)
WHERE id = sqlc.arg(id);

-- name: MarkDoneInvoiceOutboxEvents :exec
UPDATE invoice_outbox_events
SET status = 'done', completed_at = sqlc.arg(now)
WHERE id = sqlc.arg(id);

-- name: ParkInvoiceOutboxEvents :execrows
-- 「重試次數用盡」的歸宿：**停泊而非終結**（§29 的 dlParkInterval 同一形狀）。
--
-- 🔴 這裡曾經是 `SET status='failed', completed_at=now()`，而那是一個**永久旗標**：
--    候選述詞（FetchPending／Claim）與 idx_invoutbox_pending 都是 status='pending'，
--    本表沒有任何時間欄能讓一列 'failed' 回到候選集合，而 DeleteDoneBefore 只刪 'done'
--    ⇒ 那一列永久留在表上、永無執行者。
--    更糟的是**告警方向是反的**：CountPending 與 OldestPending 的述詞同樣是 'pending'，
--    所以那一列一落到 'failed' 就退出 outbox_oldest_pending_age_seconds ⇒
--    告警 InvoiceOutboxLag 會在事件**真正卡死的那一刻由 firing 轉 resolved**。
--
-- 停泊之後 status 仍是 'pending' ⇒ 那一列留在積壓指標裡、告警持續亮著，而 next_attempt_at
-- 被推到 park_until ⇒ 下一輪不會重撈（沒有退避的熱迴圈正是 §29 要防的另一半）。
--
-- 為什麼不乾脆終結：發票通知是法定義務（§9.3.4 R8 的同一理由），「永遠不再發」沒有合法
-- 終點；而真實的失敗成因幾乎都是可修的——broker 不可用、exchange 尚未 apply、DB 短暫不可
-- 用。修好之後停泊到期那一輪會自己把它撿回來，不需要任何 backfill CLI（§22 禁止）。
-- 重送的代價由 notification-service 的冪等鍵承接（見 internal/worker/outbox_worker.go 檔頭）。
--
-- last_error 一定要寫：只在非終態重試時寫 last_error 的話，會在列真的耗盡重試的那一刻
-- 剛好丟掉維運唯一需要的那句錯誤訊息。
--
-- 對應的監控面是 M31 invoice_outbox_stranded_rows（monitoring/sql-exporter/sql_exporter.yml）：
-- 停泊太久的 'pending' 與被維運 SQL 隔離的 'failed' 都在它的述詞裡，而**那支查詢會被
-- internal/repository/postgres/outbox_park_integration_test.go 直接執行**（不是抄一份）。
--
-- ⚠️ 述詞帶 status='pending'：一列若已被維運 SQL 隔離成 'failed'，停泊不得把它拉回來。
-- :execrows 讓呼叫端看得到 rows=0（＝那一列已離開 pending），而不是靜默成功。
UPDATE invoice_outbox_events
SET next_attempt_at = sqlc.arg(park_until), last_error = sqlc.arg(last_error)
WHERE id = sqlc.arg(id) AND status = 'pending';

-- name: CountPendingInvoiceOutboxEvents :one
-- 🔴 述詞是 'pending' ⇒ **停泊中的列必須留在 'pending'**，否則它一離開就同時離開
-- outbox_pending_rows 與 outbox_oldest_pending_age_seconds，告警在卡死那一刻熄滅。
-- 這是 ParkInvoiceOutboxEvents 不改 status 的理由，兩者綁在一起。
SELECT COUNT(*) FROM invoice_outbox_events WHERE status = 'pending';

-- name: OldestPendingInvoiceOutboxEventsCreatedAt :one
-- 空表時回 pgx.ErrNoRows —— 刻意不用 nullable 的 MIN() 聚合，讓 Go 呼叫端拿到明確的
-- 「沒有列」訊號，而不是去特判一個 NULL 聚合值。
SELECT created_at FROM invoice_outbox_events
WHERE status = 'pending'
ORDER BY created_at ASC
LIMIT 1;

-- name: DeleteDoneInvoiceOutboxEventsBefore :execrows
-- pkg/outbox.CleanupDone（由 INV.gov 帶跑，§5.2.3 第 12 組）：status='done' 且
-- completed_at < now − 30d 整列刪（§5.4.6）。Postgres 沒有 DELETE ... LIMIT，用子查詢框住一輪。
-- 內層 SELECT 必須 alias（AS sub）並限定欄位：sqlc 的分析器會把未 alias 的自我參照子查詢欄位
-- 判成 ambiguous，即使 Postgres 本身解得開。
DELETE FROM invoice_outbox_events
WHERE id IN (
  SELECT sub.id FROM invoice_outbox_events AS sub
  WHERE sub.status = 'done' AND sub.completed_at < sqlc.arg(cutoff)
  LIMIT sqlc.arg(row_limit)
);
