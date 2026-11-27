
-- name: InsertInvoiceOutboxEvent :one
INSERT INTO invoice_outbox_events (
  aggregate_type, aggregate_id, event_type, exchange, routing_key, payload,
  status, attempt_count, next_attempt_at, created_at)
VALUES (
  sqlc.arg(aggregate_type), sqlc.arg(aggregate_id), sqlc.arg(event_type),
  sqlc.arg(exchange), sqlc.arg(routing_key), sqlc.arg(payload),
  'pending', 0, sqlc.arg(next_attempt_at), sqlc.arg(now))
RETURNING *;

-- name: FetchPendingInvoiceOutboxEvents :many
SELECT * FROM invoice_outbox_events
WHERE status = 'pending' AND next_attempt_at <= sqlc.arg(now)
ORDER BY next_attempt_at
LIMIT sqlc.arg(row_limit);

-- name: ClaimInvoiceOutboxEvents :execrows
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
UPDATE invoice_outbox_events
SET next_attempt_at = sqlc.arg(park_until), last_error = sqlc.arg(last_error)
WHERE id = sqlc.arg(id) AND status = 'pending';

-- name: CountPendingInvoiceOutboxEvents :one
SELECT COUNT(*) FROM invoice_outbox_events WHERE status = 'pending';

-- name: OldestPendingInvoiceOutboxEventsCreatedAt :one
SELECT created_at FROM invoice_outbox_events
WHERE status = 'pending'
ORDER BY created_at ASC
LIMIT 1;

-- name: DeleteDoneInvoiceOutboxEventsBefore :execrows
DELETE FROM invoice_outbox_events
WHERE id IN (
  SELECT sub.id FROM invoice_outbox_events AS sub
  WHERE sub.status = 'done' AND sub.completed_at < sqlc.arg(cutoff)
  LIMIT sqlc.arg(row_limit)
);
