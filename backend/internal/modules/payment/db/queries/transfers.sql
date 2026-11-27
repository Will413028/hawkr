-- name: LockPaymentByOrderNo :one
SELECT p.*
FROM payment.payments p
JOIN payment.payment_orders o ON o.payment_id = p.id
WHERE o.order_no = $1
FOR UPDATE OF p;

-- name: LockPaymentOrderByOrderNo :one
SELECT *
FROM payment.payment_orders
WHERE order_no = $1
FOR UPDATE;

-- name: LockPaymentOrder :one
SELECT *
FROM payment.payment_orders
WHERE id = $1
FOR UPDATE;

-- name: PaymentOrderByID :one
SELECT *
FROM payment.payment_orders
WHERE id = $1;

-- name: PaymentOrderByOrderNo :one
SELECT *
FROM payment.payment_orders
WHERE order_no = $1;

-- name: PaymentOrdersOfPayment :many
SELECT *
FROM payment.payment_orders
WHERE payment_id = $1
ORDER BY id;

-- name: NonFailedRefundTotal :one
SELECT COALESCE(SUM(amount), 0)::bigint AS total
FROM payment.refunds
WHERE payment_order_id = $1 AND status <> 'FAILED';

-- name: InFlightRefundTotal :one
SELECT COALESCE(SUM(amount), 0)::bigint AS total
FROM payment.refunds
WHERE payment_order_id = $1 AND status IN ('PENDING', 'REFUNDING', 'ANOMALY');

-- name: InFlightRefundIDs :many
SELECT id
FROM payment.refunds
WHERE payment_order_id = $1 AND status IN ('PENDING', 'REFUNDING', 'ANOMALY')
ORDER BY id;

-- name: InsertTransfer :execrows
INSERT INTO payment.transfers (id, payment_order_id, store_id, amount, destination_account_id, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(payment_order_id), sqlc.arg(store_id), sqlc.arg(amount), sqlc.arg(destination_account_id), sqlc.arg(now), sqlc.arg(now))
ON CONFLICT DO NOTHING;

-- name: ActiveTransferByOrder :one
SELECT *
FROM payment.transfers
WHERE payment_order_id = $1 AND status <> 'FAILED';

-- name: SucceededTransferByOrder :one
SELECT *
FROM payment.transfers
WHERE payment_order_id = $1 AND status = 'SUCCEEDED';

-- name: TransferByID :one
SELECT *
FROM payment.transfers
WHERE id = $1;

-- name: LockTransfer :one
SELECT *
FROM payment.transfers
WHERE id = $1
FOR UPDATE;

-- name: ClaimTransfer :one
UPDATE payment.transfers
SET amount = CASE WHEN called_at IS NULL THEN sqlc.arg(amount)::bigint ELSE amount END,
    called_at = COALESCE(called_at, sqlc.arg(now)::timestamptz),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'PENDING'
RETURNING *;

-- name: SucceedTransfer :one
UPDATE payment.transfers
SET status = 'SUCCEEDED',
    external_transfer_id = sqlc.arg(external_transfer_id),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'PENDING'
RETURNING *;

-- name: FailTransfer :one
UPDATE payment.transfers
SET status = 'FAILED',
    failure_reason = sqlc.arg(failure_reason),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'PENDING'
RETURNING *;

-- name: RotateTransferKey :one
UPDATE payment.transfers
SET idempotency_key = gen_random_uuid(),
    called_at = NULL,
    lookup_failures = 0,
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'PENDING' AND idempotency_key = sqlc.arg(old_key)
RETURNING *;

-- name: BumpTransferLookupFailures :one
UPDATE payment.transfers
SET lookup_failures = lookup_failures + 1,
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'PENDING'
RETURNING lookup_failures;

-- name: ClearTransferLookupFailures :exec
UPDATE payment.transfers
SET lookup_failures = 0
WHERE id = sqlc.arg(id) AND status = 'PENDING' AND lookup_failures > 0;

-- name: AnomalyTransfer :one
UPDATE payment.transfers
SET status = 'ANOMALY',
    failure_reason = sqlc.arg(reason),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'PENDING'
RETURNING *;

-- name: ResolveTransferSucceeded :one
UPDATE payment.transfers
SET status = 'SUCCEEDED',
    external_transfer_id = sqlc.arg(external_transfer_id),
    reversed_amount = sqlc.arg(verified_reversed)::bigint,
    failure_reason = NULL,
    called_at = COALESCE(called_at, sqlc.arg(now)::timestamptz),
    lookup_failures = 0,
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'ANOMALY'
RETURNING *;

-- name: ResolveTransferFailed :one
UPDATE payment.transfers
SET status = 'FAILED',
    failure_reason = sqlc.arg(failure_reason),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'ANOMALY'
RETURNING *;

-- name: AddTransferReversal :one
UPDATE payment.transfers
SET reversed_amount = reversed_amount + sqlc.arg(amount),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: PendingTransferIDs :many
SELECT id
FROM payment.transfers
WHERE status = 'PENDING' AND updated_at <= sqlc.arg(idle_before) AND id > sqlc.arg(after_id)
ORDER BY id
LIMIT sqlc.arg(batch);

-- name: RefundByID :one
SELECT *
FROM payment.refunds
WHERE id = $1;

-- name: LockRefund :one
SELECT *
FROM payment.refunds
WHERE id = $1
FOR UPDATE;

-- name: ReleaseRefundReversal :one
UPDATE payment.refunds
SET reversal_status = 'RELEASED',
    reversal_reason = sqlc.arg(reason),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND reversal_status = 'WAITING'
RETURNING *;

-- name: StartableReversals :many
SELECT *
FROM payment.refunds
WHERE payment_order_id = $1 AND reversal_status = 'WAITING' AND status IN ('REFUNDING', 'SUCCEEDED')
ORDER BY id
FOR UPDATE;

-- name: StartRefundReversal :one
UPDATE payment.refunds
SET reversal_status = 'REVERSING',
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND reversal_status = 'WAITING' AND status IN ('REFUNDING', 'SUCCEEDED')
RETURNING *;

-- name: WaitingReversals :many
SELECT *
FROM payment.refunds
WHERE payment_order_id = $1 AND reversal_status = 'WAITING'
ORDER BY created_at, id
FOR UPDATE;

-- name: SetRefundReversalAmount :one
UPDATE payment.refunds
SET reversal_amount = sqlc.arg(reversal_amount),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND reversal_status = 'WAITING'
RETURNING *;

-- name: RefundByRequestKey :one
SELECT *
FROM payment.refunds
WHERE payment_order_id = sqlc.arg(payment_order_id) AND request_key = sqlc.arg(request_key);

-- name: PendingReversalTotal :one
SELECT (
    COALESCE((SELECT SUM(w.reversal_amount) FROM payment.refunds AS w WHERE w.payment_order_id = $1 AND w.reversal_status = 'WAITING'), 0)
    + COALESCE((SELECT SUM(v.amount) FROM payment.refund_reversals AS v WHERE v.payment_order_id = $1 AND v.status IN ('REVERSING', 'ANOMALY')), 0)
)::bigint AS total;

-- name: InsertRefund :one
INSERT INTO payment.refunds (id, payment_order_id, request_key, amount, auto, reversal_status, reversal_amount, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(payment_order_id), sqlc.arg(request_key), sqlc.arg(amount), sqlc.arg(auto), sqlc.arg(reversal_status), sqlc.arg(reversal_amount), sqlc.arg(now), sqlc.arg(now))
RETURNING *;

-- name: MarkRefundCalled :one
UPDATE payment.refunds
SET refund_called_at = COALESCE(refund_called_at, sqlc.arg(now)::timestamptz),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'PENDING'
RETURNING *;

-- name: ApplyRefundSnapshot :one
UPDATE payment.refunds
SET status = sqlc.arg(status),
    reversal_status = CASE WHEN sqlc.arg(status)::text = 'FAILED' AND reversal_status = 'WAITING' THEN 'RELEASED' ELSE reversal_status END,
    reversal_reason = CASE WHEN sqlc.arg(status)::text = 'FAILED' AND reversal_status = 'WAITING' THEN 'refund_failed' ELSE reversal_reason END,
    topup_status = CASE WHEN sqlc.arg(status)::text = 'FAILED' AND transfer_deducted THEN 'WAITING' ELSE topup_status END,
    external_refund_id = sqlc.arg(external_refund_id),
    refund_called_at = COALESCE(refund_called_at, sqlc.arg(now)::timestamptz),
    refund_lookup_failures = 0,
    failure_reason = sqlc.narg(failure_reason),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status IN ('PENDING', 'REFUNDING')
  AND (external_refund_id IS NULL OR external_refund_id = sqlc.arg(external_refund_id))
RETURNING *;

-- name: FailRefundCall :one
UPDATE payment.refunds
SET status = 'FAILED',
    reversal_status = CASE WHEN reversal_status = 'WAITING' THEN 'RELEASED' ELSE reversal_status END,
    reversal_reason = CASE WHEN reversal_status = 'WAITING' THEN 'refund_failed' ELSE reversal_reason END,
    topup_status = CASE WHEN transfer_deducted THEN 'WAITING' ELSE topup_status END,
    failure_reason = sqlc.arg(failure_reason),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'PENDING'
RETURNING *;

-- name: RotateRefundKey :one
UPDATE payment.refunds
SET idempotency_key = gen_random_uuid(),
    refund_called_at = NULL,
    refund_lookup_failures = 0,
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'PENDING' AND idempotency_key = sqlc.arg(old_key)
RETURNING *;

-- name: BumpRefundLookupFailures :one
UPDATE payment.refunds
SET refund_lookup_failures = refund_lookup_failures + 1,
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'PENDING'
RETURNING refund_lookup_failures;

-- name: ClearRefundLookupFailures :exec
UPDATE payment.refunds
SET refund_lookup_failures = 0
WHERE id = sqlc.arg(id) AND status = 'PENDING' AND refund_lookup_failures > 0;

-- name: AnomalyRefund :one
UPDATE payment.refunds
SET status = 'ANOMALY',
    failure_reason = sqlc.arg(reason),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status IN ('PENDING', 'REFUNDING')
RETURNING *;

-- name: ResolveRefundRefunding :one
UPDATE payment.refunds
SET status = 'REFUNDING',
    external_refund_id = sqlc.arg(external_refund_id),
    refund_called_at = COALESCE(refund_called_at, sqlc.arg(now)::timestamptz),
    refund_lookup_failures = 0,
    failure_reason = NULL,
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'ANOMALY'
RETURNING *;

-- name: ResolveRefundCallFailed :one
UPDATE payment.refunds
SET status = 'FAILED',
    reversal_status = CASE WHEN reversal_status = 'WAITING' THEN 'RELEASED' ELSE reversal_status END,
    reversal_reason = CASE WHEN reversal_status = 'WAITING' THEN 'refund_failed' ELSE reversal_reason END,
    topup_status = CASE WHEN transfer_deducted THEN 'WAITING' ELSE topup_status END,
    failure_reason = sqlc.arg(failure_reason),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'ANOMALY'
RETURNING *;

-- name: RefundByExternal :one
SELECT r.*
FROM payment.refunds r
JOIN payment.payment_orders o ON o.id = r.payment_order_id
JOIN payment.payments p ON p.id = o.payment_id
WHERE p.provider = sqlc.arg(provider) AND r.external_refund_id = sqlc.arg(external_refund_id);

-- name: RefundIDsByExternal :many
SELECT r.id
FROM payment.refunds r
JOIN payment.payment_orders o ON o.id = r.payment_order_id
JOIN payment.payments p ON p.id = o.payment_id
WHERE p.provider = sqlc.arg(provider) AND r.external_refund_id = ANY(sqlc.arg(external_refund_ids)::text[]);

-- name: CallableRefundIDs :many
SELECT id
FROM payment.refunds
WHERE status = 'PENDING' AND updated_at <= sqlc.arg(idle_before) AND id > sqlc.arg(after_id)
ORDER BY id
LIMIT sqlc.arg(batch);

-- name: RefundingRefundIDs :many
SELECT id
FROM payment.refunds
WHERE status = 'REFUNDING' AND updated_at <= sqlc.arg(idle_before) AND id > sqlc.arg(after_id)
ORDER BY id
LIMIT sqlc.arg(batch);

-- name: RefundsByRequestKeys :many
SELECT sqlc.embed(r), o.order_no
FROM payment.refunds r
JOIN payment.payment_orders o ON o.id = r.payment_order_id
WHERE r.request_key = ANY(sqlc.arg(request_keys)::text[]);

-- name: LatestRefundsByOrderNos :many
SELECT DISTINCT ON (o.order_no) sqlc.embed(r), o.order_no
FROM payment.refunds r
JOIN payment.payment_orders o ON o.id = r.payment_order_id
WHERE o.order_no = ANY(sqlc.arg(order_nos)::text[])
ORDER BY o.order_no, r.created_at DESC, r.id DESC;
