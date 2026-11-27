-- name: RecordRecomputeAttempt :exec
INSERT INTO discovery.recompute_state (job, last_attempt_started_at)
VALUES (sqlc.arg(job), sqlc.arg(started_at))
ON CONFLICT (job) DO UPDATE
SET last_attempt_started_at = GREATEST(discovery.recompute_state.last_attempt_started_at, EXCLUDED.last_attempt_started_at);

-- name: LockRecomputeState :one
SELECT job, last_attempt_started_at, last_succeeded_started_at, last_succeeded_at, last_failed_at, last_error
FROM discovery.recompute_state
WHERE job = sqlc.arg(job)
FOR UPDATE;

-- name: MarkRecomputeSucceeded :exec
UPDATE discovery.recompute_state
SET last_succeeded_started_at = sqlc.arg(started_at), last_succeeded_at = sqlc.arg(finished_at)
WHERE job = sqlc.arg(job);

-- name: RecordRecomputeFailure :exec
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
