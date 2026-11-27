-- name: InsertPayment :exec
INSERT INTO payment.payments (id, buyer_id, amount, provider, description, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(buyer_id), sqlc.arg(amount), sqlc.arg(provider), sqlc.arg(description), sqlc.arg(now), sqlc.arg(now));

-- name: InsertPaymentOrder :execrows
INSERT INTO payment.payment_orders (id, payment_id, order_no, store_id, amount, created_at)
VALUES (sqlc.arg(id), sqlc.arg(payment_id), sqlc.arg(order_no), sqlc.arg(store_id), sqlc.arg(amount), sqlc.arg(now))
ON CONFLICT (order_no) DO NOTHING;

-- name: UpdatePaymentOrderTerms :one
UPDATE payment.payment_orders
SET store_id = sqlc.arg(store_id),
    amount = sqlc.arg(amount)
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: PaymentByID :one
SELECT *
FROM payment.payments
WHERE id = $1;

-- name: PaymentByOrderNo :one
SELECT p.*
FROM payment.payments p
JOIN payment.payment_orders o ON o.payment_id = p.id
WHERE o.order_no = $1;

-- name: PaymentByExternal :one
SELECT p.*
FROM payment.payments p
JOIN payment.payment_attempts a ON a.payment_id = p.id
WHERE a.provider = sqlc.arg(provider) AND a.external_payment_id = sqlc.arg(external_payment_id);

-- name: LockPayment :one
SELECT *
FROM payment.payments
WHERE id = $1
FOR UPDATE;

-- name: MarkPaymentCreateCalled :one
UPDATE payment.payments
SET create_called_at = COALESCE(create_called_at, sqlc.arg(now)::timestamptz),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'CREATING'
RETURNING *;

-- name: ApplyPaymentIntentSnapshot :one
UPDATE payment.payments
SET status = sqlc.arg(status),
    provider = sqlc.arg(provider),
    external_payment_id = sqlc.arg(external_payment_id),
    create_idempotency_key = sqlc.arg(create_idempotency_key),
    create_called_at = sqlc.arg(create_called_at),
    external_charge_id = sqlc.narg(external_charge_id),
    paid_at = sqlc.narg(paid_at),
    last_error = sqlc.narg(last_error),
    fetched_at = sqlc.arg(fetched_at),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: RotateCreateKey :one
UPDATE payment.payments
SET create_idempotency_key = gen_random_uuid(),
    create_called_at = NULL,
    create_lookup_failures = 0,
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'CREATING' AND external_payment_id IS NULL AND create_idempotency_key = sqlc.arg(old_key)
RETURNING *;

-- name: BumpCreateLookupFailures :one
UPDATE payment.payments
SET create_lookup_failures = create_lookup_failures + 1,
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'CREATING' AND external_payment_id IS NULL
RETURNING create_lookup_failures;

-- name: ClearCreateLookupFailures :exec
UPDATE payment.payments
SET create_lookup_failures = 0
WHERE id = sqlc.arg(id) AND status = 'CREATING' AND create_lookup_failures > 0;

-- name: AnomalyPayment :one
UPDATE payment.payments
SET status = 'ANOMALY',
    failure_reason = sqlc.arg(reason),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'CREATING' AND external_payment_id IS NULL
RETURNING *;

-- name: ResolvePaymentLinked :one
UPDATE payment.payments
SET status = 'AWAITING_PAYMENT',
    external_payment_id = sqlc.arg(external_payment_id),
    failure_reason = NULL,
    create_called_at = COALESCE(create_called_at, sqlc.arg(now)::timestamptz),
    create_lookup_failures = 0,
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'ANOMALY' AND external_payment_id IS NULL
RETURNING *;

-- name: ResolvePaymentFailed :one
UPDATE payment.payments
SET status = 'CREATE_FAILED',
    failure_reason = sqlc.arg(failure_reason),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'ANOMALY'
RETURNING *;

-- name: EffectiveAttempt :one
SELECT *
FROM payment.payment_attempts
WHERE payment_id = sqlc.arg(payment_id) AND superseded_at IS NULL;

-- name: LinkEffectiveAttempt :one
UPDATE payment.payment_attempts
SET external_payment_id = sqlc.arg(external_payment_id),
    called_at = COALESCE(called_at, sqlc.arg(now)::timestamptz)
WHERE payment_id = sqlc.arg(payment_id) AND superseded_at IS NULL AND external_payment_id IS NULL
RETURNING *;

-- name: FailPaymentCreate :one
UPDATE payment.payments
SET status = 'CREATE_FAILED',
    failure_reason = sqlc.arg(failure_reason),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'CREATING' AND external_payment_id IS NULL
RETURNING *;

-- name: ResetPayment :one
UPDATE payment.payments
SET status = 'CREATING',
    failure_reason = NULL,
    last_error = NULL,
    external_payment_id = NULL,
    external_charge_id = NULL,
    paid_at = NULL,
    fetched_at = NULL,
    create_called_at = NULL,
    create_lookup_failures = 0,
    create_idempotency_key = gen_random_uuid(),
    provider = sqlc.arg(provider),
    description = sqlc.arg(description),
    buyer_id = sqlc.arg(buyer_id),
    amount = sqlc.arg(amount),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status IN ('CREATE_FAILED', 'CANCELED')
RETURNING *;

-- name: RebindCreatingPayment :one
UPDATE payment.payments
SET provider = sqlc.arg(provider),
    create_idempotency_key = gen_random_uuid(),
    create_called_at = NULL,
    create_lookup_failures = 0,
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'CREATING' AND external_payment_id IS NULL AND provider <> sqlc.arg(provider)
  AND create_called_at IS NULL
RETURNING *;

-- name: CreatingPaymentIDs :many
SELECT id
FROM payment.payments
WHERE status = 'CREATING' AND COALESCE(create_called_at, created_at) <= sqlc.arg(idle_before) AND id > sqlc.arg(after_id)
ORDER BY id
LIMIT sqlc.arg(batch);

-- name: PollablePaymentIDs :many
SELECT id
FROM payment.payments
WHERE status IN ('AWAITING_PAYMENT', 'PROCESSING') AND COALESCE(fetched_at, created_at) <= sqlc.arg(read_before) AND id > sqlc.arg(after_id)
ORDER BY id
LIMIT sqlc.arg(batch);

-- name: PaymentIDsByExternal :many
SELECT DISTINCT a.payment_id
FROM payment.payment_attempts a
WHERE a.provider = sqlc.arg(provider) AND a.external_payment_id = ANY(sqlc.arg(external_payment_ids)::text[])
ORDER BY a.payment_id;

-- name: EnsureEffectiveAttempt :exec
INSERT INTO payment.payment_attempts (id, payment_id, provider, idempotency_key, created_at)
SELECT sqlc.arg(id), p.id, p.provider, p.create_idempotency_key, sqlc.arg(now)
FROM payment.payments p
WHERE p.id = sqlc.arg(payment_id) AND p.status = 'CREATING'
ON CONFLICT (payment_id) WHERE superseded_at IS NULL DO NOTHING;

-- name: SupersedeEffectiveAttempt :one
UPDATE payment.payment_attempts
SET superseded_at = sqlc.arg(now)
WHERE payment_id = sqlc.arg(payment_id) AND superseded_at IS NULL
RETURNING *;

-- name: ReviveAttempt :one
UPDATE payment.payment_attempts
SET superseded_at = NULL
WHERE id = sqlc.arg(id) AND superseded_at IS NOT NULL
RETURNING *;

-- name: AttemptByID :one
SELECT *
FROM payment.payment_attempts
WHERE id = sqlc.arg(id) AND payment_id = sqlc.arg(payment_id);

-- name: AttemptByIDOnly :one
SELECT *
FROM payment.payment_attempts
WHERE id = sqlc.arg(id);

-- name: AttemptsOfPayment :many
SELECT *
FROM payment.payment_attempts
WHERE payment_id = sqlc.arg(payment_id) AND provider = sqlc.arg(provider) AND external_payment_id IS NOT NULL
ORDER BY created_at, id;

-- name: MarkAttemptCalled :one
UPDATE payment.payment_attempts
SET called_at = COALESCE(called_at, sqlc.arg(now)::timestamptz)
WHERE payment_id = sqlc.arg(payment_id) AND idempotency_key = sqlc.arg(idempotency_key) AND superseded_at IS NULL
RETURNING *;

-- name: ApplyAttemptSnapshot :one
UPDATE payment.payment_attempts
SET external_payment_id = COALESCE(external_payment_id, sqlc.arg(external_payment_id)::text),
    state = CASE WHEN state = 'SUCCEEDED' THEN state ELSE sqlc.arg(state)::text END,
    fetched_at = sqlc.arg(fetched_at)
WHERE id = sqlc.arg(id) AND (fetched_at IS NULL OR fetched_at <= sqlc.arg(fetched_at)::timestamptz)
RETURNING *;

-- name: MarkAttemptExcessPaid :exec
UPDATE payment.payment_attempts
SET excess_paid_at = COALESCE(excess_paid_at, sqlc.arg(now)::timestamptz)
WHERE id = sqlc.arg(id) AND state = 'SUCCEEDED' AND superseded_at IS NOT NULL;

-- name: ExcessPaidAttempts :many
SELECT *
FROM payment.payment_attempts
WHERE excess_paid_at IS NOT NULL AND id > sqlc.arg(after_id)
ORDER BY id
LIMIT sqlc.arg(batch);

-- name: StaleAttemptPaymentIDs :many
SELECT DISTINCT payment_id
FROM payment.payment_attempts
WHERE superseded_at IS NOT NULL AND external_payment_id IS NOT NULL AND (state IS NULL OR state NOT IN ('SUCCEEDED', 'CANCELED'))
  AND COALESCE(fetched_at, created_at) <= sqlc.arg(read_before)::timestamptz AND payment_id > sqlc.arg(after_id)
ORDER BY payment_id
LIMIT sqlc.arg(batch);

-- name: PaymentsByOrderNos :many
SELECT sqlc.embed(p), sqlc.embed(o)
FROM payment.payment_orders o
JOIN payment.payments p ON p.id = o.payment_id
WHERE o.order_no = ANY(sqlc.arg(order_nos)::text[]);
