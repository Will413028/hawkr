-- name: GetRun :one
SELECT * FROM procurement.agent_runs WHERE id = @id;

-- name: NextRunSeq :one
-- 事件的序號由 run 列配發（同一個 run 的寫入者在列鎖下排隊）；updated_at 同時是執行中 run 的心跳。
UPDATE procurement.agent_runs
SET last_event_seq = last_event_seq + 1, updated_at = @now
WHERE id = @id
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

-- name: ClaimRun :execrows
UPDATE procurement.agent_runs
SET status = 'RUNNING', started_at = @now::timestamptz, updated_at = @now::timestamptz
WHERE id = @id AND status = 'QUEUED';

-- name: FinishRun :execrows
-- 只有還沒到終態的 run 能被終結（單一寫入者的條件式終態寫入）；失敗原因只在 FAILED 時有值。
UPDATE procurement.agent_runs
SET status = @status, failure_reason = sqlc.narg('failure_reason'), completed_at = @now::timestamptz, updated_at = @now::timestamptz
WHERE id = @id AND status IN ('QUEUED', 'RUNNING', 'CANCEL_REQUESTED');

-- name: ListTimedOutRunIDs :many
-- 逾時的 QUEUED（受理後一直沒被領走）與 RUNNING（心跳 updated_at 太久沒動）。SKIP LOCKED：另一個 sweeper 正在處理的列略過。
SELECT id FROM procurement.agent_runs
WHERE (status = 'QUEUED' AND created_at < @queued_before)
   OR (status = 'RUNNING' AND updated_at < @running_before)
ORDER BY created_at
LIMIT @batch
FOR UPDATE SKIP LOCKED;

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
