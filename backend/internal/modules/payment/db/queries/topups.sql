-- name: MarkRefundsDeducted :exec
UPDATE payment.refunds
SET transfer_deducted = (status <> 'FAILED')
WHERE payment_order_id = $1 AND transfer_deducted <> (status <> 'FAILED');

-- name: ClearRefundsDeducted :exec
UPDATE payment.refunds
SET transfer_deducted = false
WHERE payment_order_id = $1 AND transfer_deducted;

-- name: WaitingTopups :many
SELECT *
FROM payment.refunds
WHERE payment_order_id = $1 AND topup_status = 'WAITING'
ORDER BY id
FOR UPDATE;

-- name: StartRefundTopup :one
UPDATE payment.refunds
SET topup_status = 'SENDING',
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND topup_status = 'WAITING'
RETURNING *;

-- name: ReleaseRefundTopup :one
UPDATE payment.refunds
SET topup_status = 'RELEASED',
    topup_reason = sqlc.arg(reason),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND topup_status = ANY(sqlc.arg(from_statuses)::text[])
RETURNING *;

-- name: MarkRefundTopupCalled :one
UPDATE payment.refunds
SET topup_called_at = COALESCE(topup_called_at, sqlc.arg(now)::timestamptz),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND topup_status = 'SENDING'
RETURNING *;

-- name: ApplyRefundTopup :one
UPDATE payment.refunds
SET topup_status = 'DONE',
    topup_external_id = sqlc.arg(topup_external_id),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND topup_status = 'SENDING'
RETURNING *;

-- name: RotateRefundTopupKey :one
UPDATE payment.refunds
SET topup_idempotency_key = gen_random_uuid(),
    topup_called_at = NULL,
    topup_lookup_failures = 0,
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND topup_status = 'SENDING' AND topup_idempotency_key = sqlc.arg(old_key)
RETURNING *;

-- name: BumpRefundTopupLookupFailures :one
UPDATE payment.refunds
SET topup_lookup_failures = topup_lookup_failures + 1,
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND topup_status = 'SENDING'
RETURNING topup_lookup_failures;

-- name: ClearRefundTopupLookupFailures :exec
UPDATE payment.refunds
SET topup_lookup_failures = 0
WHERE id = sqlc.arg(id) AND topup_status = 'SENDING' AND topup_lookup_failures > 0;

-- name: AnomalyRefundTopup :one
UPDATE payment.refunds
SET topup_status = 'ANOMALY',
    topup_reason = sqlc.arg(reason),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND topup_status = 'SENDING'
RETURNING *;

-- name: ResolveRefundTopupDone :one
UPDATE payment.refunds
SET topup_status = 'DONE',
    topup_external_id = sqlc.arg(topup_external_id),
    topup_called_at = COALESCE(topup_called_at, sqlc.arg(now)::timestamptz),
    topup_reason = NULL,
    topup_lookup_failures = 0,
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND topup_status = 'ANOMALY'
RETURNING *;

-- name: ResolveRefundTopupReleased :one
UPDATE payment.refunds
SET topup_status = 'RELEASED',
    topup_reason = sqlc.arg(reason),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND topup_status = 'ANOMALY'
RETURNING *;

-- name: SendingTopupRefundIDs :many
SELECT id
FROM payment.refunds
WHERE topup_status = 'SENDING' AND updated_at <= sqlc.arg(idle_before) AND id > sqlc.arg(after_id)
ORDER BY id
LIMIT sqlc.arg(batch);
