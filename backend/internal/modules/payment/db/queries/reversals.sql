-- name: InsertRefundReversal :one
INSERT INTO payment.refund_reversals (id, refund_id, payment_order_id, seq, target_refund_id, target_external_id, amount, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(refund_id), sqlc.arg(payment_order_id), sqlc.arg(seq), sqlc.narg(target_refund_id), sqlc.arg(target_external_id), sqlc.arg(amount), sqlc.arg(now), sqlc.arg(now))
RETURNING *;

-- name: RefundReversalByID :one
SELECT *
FROM payment.refund_reversals
WHERE id = $1;

-- name: LockRefundReversal :one
SELECT *
FROM payment.refund_reversals
WHERE id = $1
FOR UPDATE;

-- name: RefundReversalsByRefund :many
SELECT *
FROM payment.refund_reversals
WHERE refund_id = $1
ORDER BY seq;

-- name: ReversingReversalsByRefund :many
SELECT *
FROM payment.refund_reversals
WHERE refund_id = $1 AND status = 'REVERSING'
ORDER BY seq;

-- name: MarkRefundReversalCalled :one
UPDATE payment.refund_reversals
SET called_at = COALESCE(called_at, sqlc.arg(now)::timestamptz),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'REVERSING'
RETURNING *;

-- name: ApplyRefundReversal :one
UPDATE payment.refund_reversals
SET status = 'REVERSED',
    external_id = sqlc.arg(external_id),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'REVERSING'
RETURNING *;

-- name: ReleaseReversal :one
UPDATE payment.refund_reversals
SET status = 'RELEASED',
    reason = sqlc.arg(reason),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'REVERSING'
RETURNING *;

-- name: RotateReversalKey :one
UPDATE payment.refund_reversals
SET idempotency_key = gen_random_uuid(),
    called_at = NULL,
    lookup_failures = 0,
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'REVERSING' AND idempotency_key = sqlc.arg(old_key)
RETURNING *;

-- name: BumpReversalLookupFailures :one
UPDATE payment.refund_reversals
SET lookup_failures = lookup_failures + 1,
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'REVERSING'
RETURNING lookup_failures;

-- name: ClearReversalLookupFailures :exec
UPDATE payment.refund_reversals
SET lookup_failures = 0
WHERE id = sqlc.arg(id) AND status = 'REVERSING' AND lookup_failures > 0;

-- name: AnomalyReversal :one
UPDATE payment.refund_reversals
SET status = 'ANOMALY',
    reason = sqlc.arg(reason),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'REVERSING'
RETURNING *;

-- name: ResolveReversalDone :one
UPDATE payment.refund_reversals
SET status = 'REVERSED',
    external_id = sqlc.arg(external_id),
    called_at = COALESCE(called_at, sqlc.arg(now)::timestamptz),
    reason = NULL,
    lookup_failures = 0,
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'ANOMALY'
RETURNING *;

-- name: ResolveReversalReleased :one
UPDATE payment.refund_reversals
SET status = 'RELEASED',
    reason = sqlc.arg(reason),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'ANOMALY'
RETURNING *;

-- name: SettleRefundReversal :exec
UPDATE payment.refunds AS r
SET reversal_status = CASE WHEN agg.anomalies > 0 THEN 'ANOMALY' WHEN agg.reversing > 0 THEN 'REVERSING' WHEN agg.released > 0 THEN 'RELEASED' ELSE 'REVERSED' END,
    reversal_recovered = agg.recovered,
    reversal_reason = CASE WHEN agg.anomalies > 0 THEN agg.anomaly_reason WHEN agg.reversing > 0 THEN NULL WHEN agg.released > 0 THEN agg.release_reason ELSE NULL END,
    updated_at = sqlc.arg(now)
FROM (
    SELECT COALESCE(SUM(amount) FILTER (WHERE status = 'REVERSED'), 0)::bigint AS recovered,
           COUNT(*) FILTER (WHERE status = 'ANOMALY') AS anomalies,
           COUNT(*) FILTER (WHERE status = 'REVERSING') AS reversing,
           COUNT(*) FILTER (WHERE status = 'RELEASED') AS released,
           MIN(reason) FILTER (WHERE status = 'ANOMALY') AS anomaly_reason,
           MIN(reason) FILTER (WHERE status = 'RELEASED') AS release_reason
    FROM payment.refund_reversals
    WHERE refund_id = sqlc.arg(id)
) AS agg
WHERE r.id = sqlc.arg(id) AND r.reversal_status IN ('REVERSING', 'ANOMALY');

-- name: ReversingRefundIDs :many
SELECT refund_id
FROM payment.refund_reversals
WHERE status = 'REVERSING' AND updated_at <= sqlc.arg(idle_before) AND refund_id > sqlc.arg(after_id)
GROUP BY refund_id
ORDER BY refund_id
LIMIT sqlc.arg(batch);

-- name: OpenReversalTotalOnTransfer :one
SELECT COALESCE(SUM(amount), 0)::bigint AS total
FROM payment.refund_reversals
WHERE payment_order_id = $1 AND target_refund_id IS NULL AND status IN ('REVERSING', 'ANOMALY');

-- name: ReversibleTopups :many
SELECT r.id, r.topup_external_id AS external_id,
       (r.amount - r.topup_reversed_amount - COALESCE((SELECT SUM(v.amount) FROM payment.refund_reversals AS v WHERE v.target_refund_id = r.id AND v.status IN ('REVERSING', 'ANOMALY')), 0))::bigint AS free
FROM payment.refunds AS r
WHERE r.payment_order_id = $1 AND r.topup_status = 'DONE'
  AND r.amount - r.topup_reversed_amount - COALESCE((SELECT SUM(v.amount) FROM payment.refund_reversals AS v WHERE v.target_refund_id = r.id AND v.status IN ('REVERSING', 'ANOMALY')), 0) > 0
ORDER BY r.id;

-- name: DoneTopupCapacity :one
SELECT COALESCE(SUM(amount - topup_reversed_amount), 0)::bigint AS total
FROM payment.refunds
WHERE payment_order_id = $1 AND topup_status = 'DONE';

-- name: AddTopupReversal :one
UPDATE payment.refunds
SET topup_reversed_amount = topup_reversed_amount + sqlc.arg(amount),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: LockRefundsByID :many
SELECT *
FROM payment.refunds
WHERE id = ANY(sqlc.arg(ids)::uuid[])
ORDER BY id
FOR UPDATE;
