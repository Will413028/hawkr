-- name: GetRun :one
SELECT * FROM procurement.agent_runs WHERE id = @id;

-- name: NextRunSeq :one
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
SELECT id FROM procurement.agent_runs
WHERE status IN ('QUEUED', 'RUNNING', 'CANCEL_REQUESTED') AND id > @after
ORDER BY id
LIMIT @batch;

-- name: LockActiveRun :one
SELECT * FROM procurement.agent_runs
WHERE id = @id AND status IN ('QUEUED', 'RUNNING', 'CANCEL_REQUESTED')
FOR UPDATE SKIP LOCKED;

-- name: ListWaitingRunIDs :many
SELECT id FROM procurement.agent_runs
WHERE buyer_id = @buyer_id AND request_id = @request_id AND status = 'WAITING_FOR_INPUT'
ORDER BY id;

-- name: LockInProgressAnswerRuns :many
SELECT a.* FROM procurement.agent_runs a
JOIN procurement.agent_run_interrupts i ON i.id = a.interrupt_id
WHERE i.run_id = @run_id AND a.status IN ('QUEUED', 'RUNNING')
ORDER BY a.id
FOR UPDATE OF a;

-- name: ExpireRunEvents :one
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
SELECT i.*, EXISTS (
    SELECT 1 FROM procurement.agent_runs a
    WHERE a.interrupt_id = i.id AND a.status IN ('QUEUED', 'RUNNING', 'CANCEL_REQUESTED')
) AS processing
FROM procurement.agent_run_interrupts i
JOIN procurement.agent_runs r ON r.id = i.run_id
WHERE i.run_id = @run_id AND r.buyer_id = @buyer_id AND r.status = 'WAITING_FOR_INPUT' AND i.status = 'PENDING';

-- name: GetOpenInterruptForRequest :one
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
UPDATE procurement.agent_run_interrupts
SET status = 'RESOLVED', closed_at = @now
WHERE id = @id AND status = 'PENDING';

-- name: CancelInterruptsOfRun :execrows
UPDATE procurement.agent_run_interrupts
SET status = 'CANCELLED', closed_at = @now
WHERE run_id = @run_id AND status = 'PENDING';

-- name: GetInterruptForBuyer :one
SELECT i.* FROM procurement.agent_run_interrupts i
JOIN procurement.agent_runs r ON r.id = i.run_id
WHERE i.id = @id AND r.buyer_id = @buyer_id;

-- name: OldestBuyerRunsSince :many
SELECT created_at FROM procurement.agent_runs
WHERE buyer_id = @buyer_id AND created_at > @since::timestamptz
ORDER BY created_at DESC
LIMIT @max_rows::int;

-- name: ListRunsByRequest :many
SELECT * FROM procurement.agent_runs
WHERE request_id = @request_id AND buyer_id = @buyer_id
ORDER BY created_at DESC, id DESC
LIMIT @max_rows::int;

-- name: GetInProgressRunForRequest :one
SELECT * FROM procurement.agent_runs
WHERE request_id = @request_id AND buyer_id = @buyer_id AND status IN ('QUEUED', 'RUNNING', 'CANCEL_REQUESTED')
ORDER BY created_at DESC, id DESC
LIMIT 1;

-- name: ListUnfinishedRunIDsByRequest :many
SELECT id FROM procurement.agent_runs
WHERE request_id = @request_id AND buyer_id = @buyer_id AND status IN ('QUEUED', 'RUNNING', 'CANCEL_REQUESTED', 'WAITING_FOR_INPUT')
ORDER BY id;
