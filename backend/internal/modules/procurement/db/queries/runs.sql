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

-- name: LockWaitingRunIDs :many
-- 同一個買家、同一個對話裡等待輸入的 run；鎖住它們，讓新的一輪在同一個 transaction 內把它們取消。
SELECT id FROM procurement.agent_runs
WHERE buyer_id = @buyer_id AND conversation_id = @conversation_id AND status = 'WAITING_FOR_INPUT'
ORDER BY id
FOR UPDATE;

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

-- name: UpsertRunArtifact :one
-- 每個 (run_id, artifact_type) 一列就地更新（A9-D20 R3）；version 遞增，當 ARTIFACT_UPSERTED 的去重序號。
INSERT INTO procurement.agent_run_artifacts (run_id, artifact_type, version, status, data, checksum, as_of, created_at, updated_at)
VALUES (@run_id, @artifact_type, 1, @status, @data, @checksum, @as_of, @now, @now)
ON CONFLICT (run_id, artifact_type) DO UPDATE
SET version = procurement.agent_run_artifacts.version + 1, status = EXCLUDED.status, data = EXCLUDED.data,
    checksum = EXCLUDED.checksum, as_of = EXCLUDED.as_of, updated_at = EXCLUDED.updated_at
RETURNING version;

-- name: GetRunArtifact :one
SELECT * FROM procurement.agent_run_artifacts WHERE run_id = @run_id AND artifact_type = @artifact_type;

-- name: GetRunByIdempotencyKey :one
SELECT * FROM procurement.agent_runs WHERE buyer_id = @buyer_id AND client_request_id = @client_request_id;
