
-- name: InsertDelegationSession :one
INSERT INTO delegation.delegated_operation_sessions (
    display_code, handoff_code_hash, handoff_expires_at,
    actor_admin_id, actor_display_name, target_store_id, target_seller_account_id, target_store_name,
    reason_code, reason_label, case_reference, capabilities,
    issued_at, expires_at, last_active_at, status,
    idempotency_key, create_request_id, create_correlation_id, create_user_agent,
    created_at, updated_at
) VALUES (
    sqlc.arg(display_code), sqlc.arg(handoff_code_hash),
    sqlc.arg(handoff_expires_at)::timestamptz,
    sqlc.arg(actor_admin_id), sqlc.arg(actor_display_name), sqlc.arg(target_store_id),
    sqlc.arg(target_seller_account_id), sqlc.arg(target_store_name),
    sqlc.arg(reason_code), sqlc.arg(reason_label), sqlc.narg(case_reference), sqlc.arg(capabilities)::text[],
    sqlc.arg(issued_at)::timestamptz, sqlc.arg(expires_at)::timestamptz, sqlc.arg(issued_at)::timestamptz, 'ACTIVE',
    sqlc.arg(idempotency_key), sqlc.arg(request_id), sqlc.arg(correlation_id),
    sqlc.narg(user_agent), sqlc.arg(now)::timestamptz, sqlc.arg(now)::timestamptz
)
ON CONFLICT DO NOTHING
RETURNING *;

-- name: GetDelegationSession :one
SELECT * FROM delegation.delegated_operation_sessions WHERE id = $1;

-- name: GetDelegationSessionByIdempotencyKey :one
SELECT * FROM delegation.delegated_operation_sessions
WHERE actor_admin_id = sqlc.arg(actor_admin_id) AND idempotency_key = sqlc.arg(idempotency_key);

-- name: GetDelegationSessionByCredential :one
SELECT * FROM delegation.delegated_operation_sessions WHERE credential_hash = $1;

-- name: TouchDelegationSession :one
UPDATE delegation.delegated_operation_sessions
SET last_active_at = CASE
        WHEN status = 'ACTIVE' AND ended_at IS NULL
             AND sqlc.arg(now)::timestamptz < expires_at
             AND last_active_at > sqlc.arg(idle_since)::timestamptz
        THEN date_trunc('second', sqlc.arg(now)::timestamptz)
        ELSE last_active_at
    END
WHERE credential_hash = sqlc.arg(credential_hash)
RETURNING *;

-- name: TouchDelegationSessionByID :exec
UPDATE delegation.delegated_operation_sessions
SET last_active_at = date_trunc('second', sqlc.arg(now)::timestamptz)
WHERE id = sqlc.arg(id)
  AND status = 'ACTIVE' AND ended_at IS NULL
  AND sqlc.arg(now)::timestamptz < expires_at
  AND last_active_at > sqlc.arg(idle_since)::timestamptz;

-- name: GetActiveDelegationSessionOfActor :one
SELECT * FROM delegation.delegated_operation_sessions
WHERE actor_admin_id = sqlc.arg(actor_admin_id)
  AND status = 'ACTIVE' AND ended_at IS NULL
  AND sqlc.arg(now)::timestamptz < expires_at
  AND last_active_at > sqlc.arg(idle_since)::timestamptz;

-- name: GetActiveDelegationSessionOfStore :one
SELECT * FROM delegation.delegated_operation_sessions
WHERE target_store_id = sqlc.arg(target_store_id)
  AND status = 'ACTIVE' AND ended_at IS NULL
  AND sqlc.arg(now)::timestamptz < expires_at
  AND last_active_at > sqlc.arg(idle_since)::timestamptz;

-- name: DelegationActorSlotTaken :one
SELECT EXISTS (
    SELECT 1 FROM delegation.delegated_operation_sessions
    WHERE actor_admin_id = $1 AND status = 'ACTIVE' AND ended_at IS NULL
) AS taken;

-- name: DelegationStoreSlotTaken :one
SELECT EXISTS (
    SELECT 1 FROM delegation.delegated_operation_sessions
    WHERE target_store_id = $1 AND status = 'ACTIVE' AND ended_at IS NULL
) AS taken;

-- name: CountActiveDelegationSessionsOfStore :one
SELECT count(*) FROM delegation.delegated_operation_sessions
WHERE target_store_id = sqlc.arg(target_store_id)
  AND status = 'ACTIVE' AND ended_at IS NULL
  AND sqlc.arg(now)::timestamptz < expires_at
  AND last_active_at > sqlc.arg(idle_since)::timestamptz;

-- name: EndStaleDelegationSessions :execrows
UPDATE delegation.delegated_operation_sessions
SET ended_at = sqlc.arg(now)::timestamptz, updated_at = sqlc.arg(now)::timestamptz
WHERE status = 'ACTIVE' AND ended_at IS NULL
  AND (expires_at <= sqlc.arg(now)::timestamptz OR last_active_at <= sqlc.arg(idle_since)::timestamptz)
  AND (actor_admin_id = sqlc.arg(actor_admin_id) OR target_store_id = sqlc.arg(target_store_id));

-- name: EndDelegationSession :execrows
UPDATE delegation.delegated_operation_sessions
SET ended_at = sqlc.arg(now)::timestamptz,
    expires_at = GREATEST(issued_at + INTERVAL '1 second',
                          LEAST(expires_at, date_trunc('second', sqlc.arg(now)::timestamptz))),
    revision = revision + 1,
    updated_at = sqlc.arg(now)::timestamptz
WHERE id = sqlc.arg(id) AND status = 'ACTIVE' AND ended_at IS NULL;

-- name: RedeemDelegationHandoff :one
UPDATE delegation.delegated_operation_sessions
SET handoff_code_hash = NULL, credential_hash = sqlc.arg(credential_hash)
WHERE handoff_code_hash = sqlc.arg(handoff_code_hash)
  AND handoff_expires_at > sqlc.arg(now)::timestamptz
  AND credential_hash IS NULL
  AND status = 'ACTIVE' AND ended_at IS NULL
  AND sqlc.arg(now)::timestamptz < expires_at
  AND last_active_at > sqlc.arg(idle_since)::timestamptz
RETURNING *;

-- name: ReissueDelegationHandoff :one
UPDATE delegation.delegated_operation_sessions
SET handoff_code_hash = sqlc.arg(handoff_code_hash), handoff_expires_at = sqlc.arg(handoff_expires_at)::timestamptz
WHERE id = sqlc.arg(id) AND credential_hash IS NULL
RETURNING *;

-- name: EndActiveDelegationsOfActor :many
UPDATE delegation.delegated_operation_sessions
SET ended_at = sqlc.arg(now)::timestamptz,
    expires_at = GREATEST(issued_at + INTERVAL '1 second',
                          LEAST(expires_at, date_trunc('second', sqlc.arg(now)::timestamptz))),
    revision = revision + 1,
    updated_at = sqlc.arg(now)::timestamptz
WHERE actor_admin_id = sqlc.arg(actor_admin_id)
  AND status = 'ACTIVE' AND ended_at IS NULL
  AND sqlc.arg(now)::timestamptz < expires_at
  AND last_active_at > sqlc.arg(idle_since)::timestamptz
RETURNING id;

-- name: EndActiveDelegationsOfStore :many
UPDATE delegation.delegated_operation_sessions
SET ended_at = sqlc.arg(now)::timestamptz,
    expires_at = GREATEST(issued_at + INTERVAL '1 second',
                          LEAST(expires_at, date_trunc('second', sqlc.arg(now)::timestamptz))),
    revision = revision + 1,
    updated_at = sqlc.arg(now)::timestamptz
WHERE target_store_id = sqlc.arg(target_store_id)
  AND status = 'ACTIVE' AND ended_at IS NULL
  AND sqlc.arg(now)::timestamptz < expires_at
  AND last_active_at > sqlc.arg(idle_since)::timestamptz
RETURNING id;

-- name: ListUnrevokedDelegationSessionIDsOfActor :many
SELECT id FROM delegation.delegated_operation_sessions
WHERE actor_admin_id = sqlc.arg(actor_admin_id) AND status <> 'REVOKED' AND id = ANY(sqlc.arg(ids)::bigint[])
ORDER BY id;

-- name: RevokeDelegationSession :one
UPDATE delegation.delegated_operation_sessions
SET status = 'REVOKED',
    revoked_at = date_trunc('second', sqlc.arg(now)::timestamptz),
    ended_at = COALESCE(ended_at, LEAST(expires_at, date_trunc('second', sqlc.arg(now)::timestamptz))),
    revoke_reason = sqlc.arg(revoke_reason)::text,
    revoked_by_admin_id = sqlc.arg(revoked_by_admin_id)::bigint,
    revision = revision + 1,
    updated_at = sqlc.arg(now)::timestamptz
WHERE id = sqlc.arg(id) AND revoked_at IS NULL
  AND (NOT sqlc.arg(only_active)::boolean OR (
    status = 'ACTIVE' AND ended_at IS NULL
    AND sqlc.arg(now)::timestamptz < expires_at
    AND last_active_at > sqlc.arg(idle_since)::timestamptz
  ))
RETURNING *;

-- name: RevokeActiveDelegationsOfActor :many
UPDATE delegation.delegated_operation_sessions
SET status = 'REVOKED',
    revoked_at = date_trunc('second', sqlc.arg(now)::timestamptz),
    ended_at = LEAST(expires_at, date_trunc('second', sqlc.arg(now)::timestamptz)),
    revoke_reason = sqlc.arg(revoke_reason)::text,
    revoked_by_admin_id = sqlc.narg(revoked_by_admin_id)::bigint,
    revision = revision + 1,
    updated_at = sqlc.arg(now)::timestamptz
WHERE actor_admin_id = sqlc.arg(actor_admin_id)
  AND status = 'ACTIVE' AND ended_at IS NULL
  AND sqlc.arg(now)::timestamptz < expires_at
  AND last_active_at > sqlc.arg(idle_since)::timestamptz
RETURNING id;

-- name: RevokeActiveDelegationsOfStore :many
UPDATE delegation.delegated_operation_sessions
SET status = 'REVOKED',
    revoked_at = date_trunc('second', sqlc.arg(now)::timestamptz),
    ended_at = LEAST(expires_at, date_trunc('second', sqlc.arg(now)::timestamptz)),
    revoke_reason = sqlc.arg(revoke_reason)::text,
    revoked_by_admin_id = sqlc.arg(revoked_by_admin_id)::bigint,
    revision = revision + 1,
    updated_at = sqlc.arg(now)::timestamptz
WHERE target_store_id = sqlc.arg(target_store_id)
  AND status = 'ACTIVE' AND ended_at IS NULL
  AND sqlc.arg(now)::timestamptz < expires_at
  AND last_active_at > sqlc.arg(idle_since)::timestamptz
RETURNING id;

-- name: ListDelegationSessions :many
SELECT s.*
FROM delegation.delegated_operation_sessions s
WHERE (sqlc.narg(actor_admin_id)::bigint IS NULL OR s.actor_admin_id = sqlc.narg(actor_admin_id)::bigint)
  AND (sqlc.narg(target_store_id)::bigint IS NULL OR s.target_store_id = sqlc.narg(target_store_id)::bigint)
  AND (sqlc.arg(display_code)::text = '' OR s.display_code = sqlc.arg(display_code)::text)
  AND (sqlc.narg(date_from)::timestamptz IS NULL OR s.issued_at >= sqlc.narg(date_from)::timestamptz)
  AND (sqlc.narg(date_to)::timestamptz IS NULL OR s.issued_at <= sqlc.narg(date_to)::timestamptz)
ORDER BY s.created_at DESC, s.id DESC
LIMIT sqlc.arg(row_limit) OFFSET sqlc.arg(row_offset);

-- name: CountDelegationSessions :one
SELECT count(*) FROM delegation.delegated_operation_sessions s
WHERE (sqlc.narg(actor_admin_id)::bigint IS NULL OR s.actor_admin_id = sqlc.narg(actor_admin_id)::bigint)
  AND (sqlc.narg(target_store_id)::bigint IS NULL OR s.target_store_id = sqlc.narg(target_store_id)::bigint)
  AND (sqlc.arg(display_code)::text = '' OR s.display_code = sqlc.arg(display_code)::text)
  AND (sqlc.narg(date_from)::timestamptz IS NULL OR s.issued_at >= sqlc.narg(date_from)::timestamptz)
  AND (sqlc.narg(date_to)::timestamptz IS NULL OR s.issued_at <= sqlc.narg(date_to)::timestamptz);
