-- name: RecordRecomputeAttempt :exec
-- 一次重算開始：記下開始時間（沒有這一列就建立）。取較大的值，亂序到達的舊嘗試不會把它倒退。
INSERT INTO discovery.recompute_state (job, last_attempt_started_at)
VALUES (sqlc.arg(job), sqlc.arg(started_at))
ON CONFLICT (job) DO UPDATE
SET last_attempt_started_at = GREATEST(discovery.recompute_state.last_attempt_started_at, EXCLUDED.last_attempt_started_at);

-- name: LockRecomputeState :one
-- 取代快照的 transaction 先鎖狀態列，序列化重疊的重算；回傳的 last_succeeded_started_at 決定這次能不能寫。
SELECT job, last_attempt_started_at, last_succeeded_started_at, last_succeeded_at, last_failed_at, last_error
FROM discovery.recompute_state
WHERE job = sqlc.arg(job)
FOR UPDATE;

-- name: MarkRecomputeSucceeded :exec
-- 與取代快照同一個 transaction：快照與「它是哪一次重算的結果」一起生效。
UPDATE discovery.recompute_state
SET last_succeeded_started_at = sqlc.arg(started_at), last_succeeded_at = sqlc.arg(finished_at)
WHERE job = sqlc.arg(job);

-- name: RecordRecomputeFailure :exec
-- 記下失敗的時間與（已截斷的）錯誤。只有比目前記錄更晚的失敗才覆蓋，亂序到達的舊失敗不會蓋掉新的。
INSERT INTO discovery.recompute_state (job, last_attempt_started_at, last_failed_at, last_error)
VALUES (sqlc.arg(job), sqlc.arg(started_at), sqlc.arg(failed_at), sqlc.arg(error)::text)
ON CONFLICT (job) DO UPDATE
SET last_failed_at = GREATEST(discovery.recompute_state.last_failed_at, EXCLUDED.last_failed_at),
    last_error = CASE
        WHEN discovery.recompute_state.last_failed_at IS NULL OR EXCLUDED.last_failed_at >= discovery.recompute_state.last_failed_at
        THEN EXCLUDED.last_error ELSE discovery.recompute_state.last_error END,
    last_attempt_started_at = GREATEST(discovery.recompute_state.last_attempt_started_at, EXCLUDED.last_attempt_started_at);

-- name: GetRecomputeState :one
SELECT job, last_attempt_started_at, last_succeeded_started_at, last_succeeded_at, last_failed_at, last_error
FROM discovery.recompute_state
WHERE job = sqlc.arg(job);
