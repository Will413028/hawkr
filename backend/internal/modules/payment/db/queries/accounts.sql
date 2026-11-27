-- name: PaymentAccountByStore :one
SELECT *
FROM payment.accounts
WHERE store_id = $1;

-- name: PaymentAccountByExternal :one
SELECT *
FROM payment.accounts
WHERE provider = $1 AND external_account_id = $2;

-- name: LockPaymentAccount :one
SELECT *
FROM payment.accounts
WHERE store_id = $1
FOR UPDATE;

-- name: InsertPaymentAccount :execrows
INSERT INTO payment.accounts (store_id, provider, requirement_collection, created_at, updated_at)
VALUES (sqlc.arg(store_id), sqlc.arg(provider), sqlc.arg(requirement_collection), sqlc.arg(now), sqlc.arg(now))
ON CONFLICT (store_id) DO NOTHING;

-- name: MarkPaymentAccountCreateCalled :one
UPDATE payment.accounts
SET create_called_at = COALESCE(create_called_at, sqlc.arg(now)::timestamptz),
    updated_at = sqlc.arg(now)
WHERE store_id = sqlc.arg(store_id) AND create_completed_at IS NULL AND purge_pending_at IS NULL
RETURNING *;

-- name: SetPaymentAccountExternal :one
UPDATE payment.accounts
SET external_account_id = sqlc.arg(external_account_id),
    updated_at = sqlc.arg(now)
WHERE store_id = sqlc.arg(store_id) AND external_account_id IS NULL AND create_completed_at IS NULL AND purge_pending_at IS NULL
RETURNING *;

-- name: SetPaymentAccountExternalForPurge :one
UPDATE payment.accounts
SET external_account_id = sqlc.arg(external_account_id),
    updated_at = sqlc.arg(now)
WHERE store_id = sqlc.arg(store_id) AND external_account_id IS NULL AND purge_pending_at IS NOT NULL
RETURNING *;

-- name: CompletePaymentAccountCreate :one
UPDATE payment.accounts
SET create_completed_at = sqlc.arg(now)::timestamptz,
    status = 'ACTION_REQUIRED',
    create_failed_at = NULL,
    create_refused_at = NULL,
    updated_at = sqlc.arg(now)
WHERE store_id = sqlc.arg(store_id) AND external_account_id IS NOT NULL AND create_completed_at IS NULL AND purge_pending_at IS NULL
RETURNING *;

-- name: RotatePaymentAccountIdempotencyKey :one
UPDATE payment.accounts
SET create_idempotency_key = gen_random_uuid(),
    create_called_at = sqlc.arg(now)::timestamptz,
    updated_at = sqlc.arg(now)
WHERE store_id = sqlc.arg(store_id) AND external_account_id IS NULL AND create_completed_at IS NULL AND purge_pending_at IS NULL
RETURNING *;

-- name: MarkPaymentAccountCreateFailed :execrows
UPDATE payment.accounts
SET create_failed_at = COALESCE(create_failed_at, sqlc.arg(now)::timestamptz),
    create_refused_at = CASE WHEN sqlc.arg(refused)::boolean THEN COALESCE(create_refused_at, sqlc.arg(now)::timestamptz) ELSE create_refused_at END,
    updated_at = sqlc.arg(now)
WHERE store_id = sqlc.arg(store_id) AND create_completed_at IS NULL AND purge_pending_at IS NULL;

-- name: ClearPaymentAccountCreateFailed :execrows
UPDATE payment.accounts
SET create_failed_at = NULL,
    create_refused_at = NULL,
    updated_at = sqlc.arg(now)
WHERE store_id = sqlc.arg(store_id) AND create_completed_at IS NULL AND create_failed_at IS NOT NULL;

-- name: ApplyPaymentSnapshot :one
UPDATE payment.accounts
SET status = sqlc.arg(status),
    transfers_active = sqlc.arg(transfers_active),
    payouts_active = sqlc.arg(payouts_active),
    requirements = sqlc.arg(requirements),
    requirements_deadline = sqlc.narg(requirements_deadline),
    first_activated_at = COALESCE(first_activated_at, CASE WHEN sqlc.arg(transfers_active)::boolean THEN sqlc.arg(now)::timestamptz END),
    status_reason = sqlc.narg(status_reason),
    fetched_at = sqlc.arg(fetched_at),
    updated_at = sqlc.arg(now)
WHERE store_id = sqlc.arg(store_id) AND create_completed_at IS NOT NULL
  AND (fetched_at IS NULL OR fetched_at <= sqlc.arg(fetched_at))
RETURNING *;

-- name: ClaimPaymentAccountSellerRead :one
UPDATE payment.accounts
SET seller_read_claimed_at = sqlc.arg(now)
WHERE store_id = sqlc.arg(store_id) AND create_completed_at IS NOT NULL AND purge_pending_at IS NULL
  AND (seller_read_claimed_at IS NULL OR seller_read_claimed_at <= sqlc.arg(stale_before))
  AND (fetched_at IS NULL OR fetched_at <= sqlc.arg(stale_before))
RETURNING *;

-- name: PollablePaymentAccountStoreIDs :many
SELECT a.store_id
FROM payment.accounts a
WHERE (a.status IN ('UNDER_REVIEW', 'ACTION_REQUIRED')
       OR (a.status = 'DISABLED'
           AND (a.status_reason IS NULL
                OR (a.status_reason NOT LIKE 'rejected%' AND a.status_reason NOT LIKE 'unsupported%' AND a.status_reason <> 'closed'))))
  AND a.purge_pending_at IS NULL
  AND a.store_id > sqlc.arg(after_store_id)
ORDER BY a.store_id
LIMIT sqlc.arg(batch);

-- name: PaymentAccountDeadlines :many
SELECT store_id, requirements_deadline
FROM payment.accounts
WHERE create_completed_at IS NOT NULL AND purge_pending_at IS NULL
  AND requirements_deadline > sqlc.arg(now)::timestamptz AND requirements_deadline <= sqlc.arg(until)::timestamptz
  AND store_id > sqlc.arg(after_store_id)
ORDER BY store_id
LIMIT sqlc.arg(batch);

-- name: PaymentAccountStoreIDsByExternal :many
SELECT store_id
FROM payment.accounts
WHERE provider = sqlc.arg(provider) AND external_account_id = ANY(sqlc.arg(external_account_ids)::text[]) AND create_completed_at IS NOT NULL AND purge_pending_at IS NULL
ORDER BY store_id;

-- name: StalePaymentAccountCreateStoreIDs :many
SELECT store_id
FROM payment.accounts
WHERE create_completed_at IS NULL AND create_failed_at IS NULL AND purge_pending_at IS NULL
  AND updated_at < sqlc.arg(idle_before) AND store_id > sqlc.arg(after_store_id)
ORDER BY store_id
LIMIT sqlc.arg(batch);

-- name: UnresolvedPaymentAccountStoreIDs :many
SELECT store_id
FROM payment.accounts
WHERE external_account_id IS NULL AND create_called_at IS NOT NULL AND create_completed_at IS NULL AND purge_pending_at IS NULL AND create_refused_at IS NULL
  AND create_called_at <= sqlc.arg(called_before)::timestamptz
  AND NOT (create_failed_at IS NOT NULL AND create_called_at <= sqlc.arg(expired_before)::timestamptz)
  AND store_id > sqlc.arg(after_store_id)
ORDER BY store_id
LIMIT sqlc.arg(batch);

-- name: DeletePaymentAccounts :many
DELETE FROM payment.accounts
WHERE store_id = ANY(sqlc.arg(store_ids)::bigint[])
RETURNING provider, external_account_id;

-- name: MarkPaymentAccountPurgePending :execrows
UPDATE payment.accounts
SET purge_pending_at = COALESCE(purge_pending_at, sqlc.arg(now)),
    updated_at = sqlc.arg(now)
WHERE store_id = sqlc.arg(store_id);

-- name: PurgePendingPaymentAccountStoreIDs :many
SELECT store_id
FROM payment.accounts
WHERE purge_pending_at IS NOT NULL AND purge_blocked_at IS NULL AND store_id > sqlc.arg(after_store_id)
ORDER BY store_id
LIMIT sqlc.arg(batch);

-- name: BlockPaymentAccountPurge :execrows
UPDATE payment.accounts
SET purge_blocked_at = sqlc.arg(now)::timestamptz,
    purge_blocked_reason = sqlc.arg(reason)::text,
    updated_at = sqlc.arg(now)
WHERE store_id = sqlc.arg(store_id) AND purge_pending_at IS NOT NULL AND purge_blocked_at IS NULL;

-- name: RestartPaymentAccountPurge :one
UPDATE payment.accounts
SET purge_blocked_at = NULL,
    purge_blocked_reason = NULL,
    purge_pending_at = sqlc.arg(now)::timestamptz,
    purge_replay_requested_at = NULL,
    updated_at = sqlc.arg(now)
WHERE store_id = sqlc.arg(store_id) AND purge_blocked_at IS NOT NULL
RETURNING *;

-- name: RequestPaymentAccountPurgeReplay :execrows
UPDATE payment.accounts
SET purge_replay_requested_at = sqlc.arg(now)::timestamptz,
    updated_at = sqlc.arg(now)
WHERE store_id = sqlc.arg(store_id) AND purge_blocked_at IS NOT NULL;

-- name: ClearPaymentAccountPurgeReplay :execrows
UPDATE payment.accounts
SET purge_replay_requested_at = NULL,
    updated_at = sqlc.arg(now)
WHERE store_id = sqlc.arg(store_id) AND purge_replay_requested_at IS NOT NULL;
