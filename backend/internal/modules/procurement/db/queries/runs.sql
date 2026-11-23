-- name: GetRun :one
SELECT * FROM procurement.agent_runs WHERE id = @id;

-- name: NextRunSeq :one
-- 事件的序號由 run 列配發（同一個 run 的寫入者在列鎖下排隊）；run 是否還活著由 River job 的狀態決定，不看 updated_at。
-- 只在 run 的狀態屬於 @statuses 時配發：終態之後的寫入拿不到序號。
UPDATE procurement.agent_runs
SET last_event_seq = last_event_seq + 1, updated_at = @now
WHERE id = @id AND status = ANY(@statuses::text[])
RETURNING last_event_seq;

-- name: InsertRunEvent :exec
INSERT INTO procurement.agent_run_events (run_id, seq, event_type, payload, occurred_at, created_at)
VALUES (@run_id, @seq, @event_type, @payload, @now, @now);

-- name: NotifyRun :exec
SELECT pg_notify('procurement_run', @run_id::text);

-- name: ListRunEventsAfter :many
SELECT * FROM procurement.agent_run_events
WHERE run_id = @run_id AND seq > @after_seq
ORDER BY seq
LIMIT @page_size;

-- name: TransitionRun :one
-- 唯一的狀態轉換：只有目前狀態在 @from_statuses（由允許的邊算出）時才寫，UPDATE 持有 run 列鎖到 transaction 結束。
UPDATE procurement.agent_runs
SET status = @to_status,
    failure_reason = sqlc.narg('failure_reason'),
    started_at = CASE WHEN @to_status::text = 'RUNNING' THEN @now::timestamptz ELSE started_at END,
    completed_at = CASE WHEN @terminal::boolean THEN @now::timestamptz ELSE completed_at END,
    updated_at = @now::timestamptz
WHERE id = @id AND status = ANY(@from_statuses::text[])
RETURNING *;

-- name: LockRun :one
SELECT * FROM procurement.agent_runs WHERE id = @id FOR UPDATE;

-- name: SetRunJob :exec
UPDATE procurement.agent_runs SET job_id = @job_id WHERE id = @id;

-- name: ListActiveRunIDsAfter :many
-- reconciler 要對帳的 run：有 job 在管的三個狀態（WAITING_FOR_INPUT 沒有 job）。以 id 為游標分頁（UUIDv7 依時間遞增）。
SELECT id FROM procurement.agent_runs
WHERE status IN ('QUEUED', 'RUNNING', 'CANCEL_REQUESTED') AND id > @after
ORDER BY id
LIMIT @batch;

-- name: LockActiveRun :one
-- 逐列處理時鎖住該列；別的交易正在寫它（例如 executor）就略過，下一輪再看。
SELECT * FROM procurement.agent_runs
WHERE id = @id AND status IN ('QUEUED', 'RUNNING', 'CANCEL_REQUESTED')
FOR UPDATE SKIP LOCKED;

-- name: ListWaitingRunIDs :many
-- 同一個買家、同一個需求裡等待輸入的 run（每個需求最多一筆 PENDING 中斷的索引也是以需求為單位）；不鎖：呼叫端依全域鎖順序（提問的 run、中斷、回答的 run）逐個鎖並重讀狀態。
SELECT id FROM procurement.agent_runs
WHERE buyer_id = @buyer_id AND request_id = @request_id AND status = 'WAITING_FOR_INPUT'
ORDER BY id;

-- name: LockInProgressAnswerRuns :many
-- 回答 @run_id 的中斷、還在進行中（排隊或執行中）的回答 run；提問的 run 被取消或被取代時一併要求取消它們。
SELECT a.* FROM procurement.agent_runs a
JOIN procurement.agent_run_interrupts i ON i.id = a.interrupt_id
WHERE i.run_id = @run_id AND a.status IN ('QUEUED', 'RUNNING')
ORDER BY a.id
FOR UPDATE OF a;

-- name: ExpireRunEvents :one
-- 結束超過保留期的 run，一次最多 @batch 個：min_available_seq 推到 last_event_seq + 1，並刪掉範圍之外的事件（A9-D20 R1）。
-- 回傳推進的 run 數與刪除的事件數；推進過的 run 不再符合條件，呼叫端迴圈到 runs 為 0。
WITH due AS (
    SELECT id FROM procurement.agent_runs
    WHERE completed_at < @cutoff::timestamptz AND min_available_seq <= last_event_seq
    ORDER BY completed_at
    LIMIT @batch::int
    FOR UPDATE SKIP LOCKED
), advanced AS (
    UPDATE procurement.agent_runs r
    SET min_available_seq = r.last_event_seq + 1
    FROM due
    WHERE r.id = due.id
    RETURNING r.id, r.min_available_seq
), deleted AS (
    DELETE FROM procurement.agent_run_events e
    USING advanced a
    WHERE e.run_id = a.id AND e.seq < a.min_available_seq
    RETURNING e.run_id
)
SELECT (SELECT count(*) FROM advanced)::bigint AS runs, (SELECT count(*) FROM deleted)::bigint AS events;

-- name: GetRunByIdempotencyKey :one
SELECT * FROM procurement.agent_runs WHERE buyer_id = @buyer_id AND client_request_id = @client_request_id;

-- name: LockBuyerSubmits :exec
-- 同一位買家的送出在這裡排隊（交易級 advisory lock），之後的進行中數與每分鐘送出數才看得到前一個交易已提交的 run。
SELECT pg_advisory_xact_lock(hashtextextended('procurement.run_submit:' || @buyer_id::bigint, 0));

-- name: CountBuyerActiveRuns :one
SELECT count(*) FROM procurement.agent_runs
WHERE buyer_id = @buyer_id AND status IN ('QUEUED', 'RUNNING', 'CANCEL_REQUESTED');

-- name: CountRequestActiveRuns :one
SELECT count(*) FROM procurement.agent_runs
WHERE request_id = @request_id AND status IN ('QUEUED', 'RUNNING', 'CANCEL_REQUESTED');

-- name: InsertInterrupt :exec
INSERT INTO procurement.agent_run_interrupts (id, run_id, request_id, interrupt_kind, status, payload, facts_as_of, created_at)
SELECT @id, r.id, r.request_id, 'CLARIFICATION', 'PENDING', @payload, sqlc.narg('facts_as_of'), @now
FROM procurement.agent_runs r WHERE r.id = @run_id;

-- name: GetOpenInterruptForRun :one
-- 一個 run 的未回答中斷：只認等待輸入的 run；被取消或被取代的 run 的中斷已是 CANCELLED。processing：有回答它的 run 還在進行中（QUEUED、RUNNING、CANCEL_REQUESTED）。
SELECT i.*, EXISTS (
    SELECT 1 FROM procurement.agent_runs a
    WHERE a.interrupt_id = i.id AND a.status IN ('QUEUED', 'RUNNING', 'CANCEL_REQUESTED')
) AS processing
FROM procurement.agent_run_interrupts i
JOIN procurement.agent_runs r ON r.id = i.run_id
WHERE i.run_id = @run_id AND r.buyer_id = @buyer_id AND r.status = 'WAITING_FOR_INPUT' AND i.status = 'PENDING';

-- name: GetOpenInterruptForRequest :one
-- 一個需求目前等買家回答的中斷（部分唯一索引保證最多一筆）；processing 同上。
SELECT i.*, EXISTS (
    SELECT 1 FROM procurement.agent_runs a
    WHERE a.interrupt_id = i.id AND a.status IN ('QUEUED', 'RUNNING', 'CANCEL_REQUESTED')
) AS processing
FROM procurement.agent_run_interrupts i
JOIN procurement.agent_runs r ON r.id = i.run_id
WHERE i.request_id = @request_id AND r.buyer_id = @buyer_id AND r.status = 'WAITING_FOR_INPUT' AND i.status = 'PENDING';

-- name: GetInterrupt :one
SELECT * FROM procurement.agent_run_interrupts WHERE id = @id;

-- name: LockInterrupt :one
SELECT * FROM procurement.agent_run_interrupts WHERE id = @id FOR UPDATE;

-- name: ResolveInterrupt :execrows
-- 回答成功的提交點：只有仍是 PENDING 的中斷會轉。
UPDATE procurement.agent_run_interrupts
SET status = 'RESOLVED', closed_at = @now
WHERE id = @id AND status = 'PENDING';

-- name: CancelInterruptsOfRun :execrows
-- 提問的 run 被取消或被取代：它的 PENDING 中斷轉 CANCELLED，讓每個需求最多一筆 PENDING 的索引不被它佔住。
UPDATE procurement.agent_run_interrupts
SET status = 'CANCELLED', closed_at = @now
WHERE run_id = @run_id AND status = 'PENDING';

-- name: GetInterruptForBuyer :one
SELECT i.* FROM procurement.agent_run_interrupts i
JOIN procurement.agent_runs r ON r.id = i.run_id
WHERE i.id = @id AND r.buyer_id = @buyer_id;

-- name: OldestBuyerRunsSince :many
-- 買家在 @since 之後建立的 run 的時間，新的在前，最多 @max_rows 筆；每分鐘送出上限用它判斷並算出要等多久。
SELECT created_at FROM procurement.agent_runs
WHERE buyer_id = @buyer_id AND created_at > @since::timestamptz
ORDER BY created_at DESC
LIMIT @max_rows::int;

-- name: ListRunsByRequest :many
-- 目前需求的 run，新到舊（UUIDv7 的 id 依時間遞增，同時間戳用 id 定序），最多 @max_rows 筆；讀取模型 GetRuntimeSnapshot.runs 用。
SELECT * FROM procurement.agent_runs
WHERE request_id = @request_id AND buyer_id = @buyer_id
ORDER BY created_at DESC, id DESC
LIMIT @max_rows::int;

-- name: GetInProgressRunForRequest :one
-- 需求目前進行中（QUEUED、RUNNING、CANCEL_REQUESTED）的 run；同一需求同時最多一個（A9-D24(6)），多筆時取最新的。
SELECT * FROM procurement.agent_runs
WHERE request_id = @request_id AND buyer_id = @buyer_id AND status IN ('QUEUED', 'RUNNING', 'CANCEL_REQUESTED')
ORDER BY created_at DESC, id DESC
LIMIT 1;
