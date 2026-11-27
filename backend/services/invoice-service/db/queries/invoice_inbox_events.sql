
-- name: InsertInboxEvent :many
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
SELECT * FROM invoice_inbox_events
WHERE status = 'DEFERRED' AND aggregate_ref = sqlc.arg(aggregate_ref)
ORDER BY received_at;

-- name: FetchDueInboxEvents :many
SELECT * FROM invoice_inbox_events
WHERE status IN ('RECEIVED', 'DEFERRED', 'FAILED')
  AND next_attempt_at <= sqlc.arg(now)
ORDER BY next_attempt_at
LIMIT sqlc.arg(row_limit);

-- name: FetchDueActionableInboxEventsByType :many
SELECT * FROM invoice_inbox_events
WHERE status IN ('RECEIVED', 'FAILED')
  AND event_type = sqlc.arg(event_type)
  AND next_attempt_at <= sqlc.arg(now)
ORDER BY next_attempt_at
LIMIT sqlc.arg(row_limit);

-- name: FetchDueInboxEventsByType :many
SELECT * FROM invoice_inbox_events
WHERE status IN ('RECEIVED', 'DEFERRED', 'FAILED')
  AND event_type = sqlc.arg(event_type)
  AND next_attempt_at <= sqlc.arg(now)
ORDER BY next_attempt_at
LIMIT sqlc.arg(row_limit);

-- name: ClaimInboxEvent :execrows
UPDATE invoice_inbox_events
SET next_attempt_at = sqlc.arg(lease_until)
WHERE id = sqlc.arg(id)
  AND status IN ('RECEIVED', 'DEFERRED', 'FAILED')
  AND next_attempt_at <= sqlc.arg(claimable_before);

-- name: MarkInboxEventDeferred :execrows
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
UPDATE invoice_inbox_events
SET attempt_count = sqlc.arg(attempt_count),
    next_attempt_at = sqlc.arg(next_attempt_at),
    last_error = sqlc.arg(last_error)
WHERE id = sqlc.arg(id) AND status IN ('RECEIVED', 'DEFERRED', 'FAILED');

-- name: CountStaleDeferredInboxEvents :one
SELECT COUNT(*) FROM invoice_inbox_events
WHERE status = 'DEFERRED' AND received_at < sqlc.arg(cutoff);

-- name: PurgeProcessedInboxPayloads :execrows
UPDATE invoice_inbox_events
SET payload = '{}'::jsonb, payload_purged_at = sqlc.arg(now)
WHERE id IN (
  SELECT sub.id FROM invoice_inbox_events AS sub
  WHERE sub.status IN ('PROCESSED', 'SKIPPED') AND sub.processed_at < sqlc.arg(cutoff) AND sub.payload_purged_at IS NULL
  ORDER BY sub.id LIMIT sqlc.arg(row_limit))
  AND status IN ('PROCESSED', 'SKIPPED') AND processed_at < sqlc.arg(cutoff) AND payload_purged_at IS NULL;
