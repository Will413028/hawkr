
-- name: GetDelegationAuthorizationForStore :one
SELECT * FROM delegation.seller_delegation_authorizations
WHERE target_store_id = sqlc.arg(target_store_id)::bigint OR target_store_id IS NULL
ORDER BY (target_store_id IS NULL)
LIMIT 1;

-- name: GetDelegationAuthorizationOfStore :one
SELECT * FROM delegation.seller_delegation_authorizations
WHERE target_store_id = sqlc.arg(target_store_id)::bigint;

-- name: LockDelegationAuthorizationOfStore :one
SELECT * FROM delegation.seller_delegation_authorizations
WHERE target_store_id = sqlc.arg(target_store_id)::bigint
FOR UPDATE;

-- name: ListDelegationAuthorizations :many
SELECT a.*
FROM delegation.seller_delegation_authorizations a
WHERE a.target_store_id IS NOT NULL
  AND (sqlc.narg(target_store_id)::bigint IS NULL OR a.target_store_id = sqlc.narg(target_store_id)::bigint)
  AND (sqlc.arg(status)::text = ''
       OR (sqlc.arg(status)::text = 'effective')
          = (a.revoked_at IS NULL
             AND a.effective_from <= sqlc.arg(now)::timestamptz
             AND (a.effective_until IS NULL OR a.effective_until > sqlc.arg(now)::timestamptz)))
ORDER BY a.created_at DESC, a.id DESC
LIMIT sqlc.arg(row_limit) OFFSET sqlc.arg(row_offset);

-- name: CountDelegationAuthorizations :one
SELECT count(*) FROM delegation.seller_delegation_authorizations a
WHERE a.target_store_id IS NOT NULL
  AND (sqlc.narg(target_store_id)::bigint IS NULL OR a.target_store_id = sqlc.narg(target_store_id)::bigint)
  AND (sqlc.arg(status)::text = ''
       OR (sqlc.arg(status)::text = 'effective')
          = (a.revoked_at IS NULL
             AND a.effective_from <= sqlc.arg(now)::timestamptz
             AND (a.effective_until IS NULL OR a.effective_until > sqlc.arg(now)::timestamptz)));

-- name: GrantDelegationAuthorization :one
INSERT INTO delegation.seller_delegation_authorizations (
    id, target_store_id, source, source_reference, granted_capabilities,
    effective_from, effective_until, approved_by_admin_id, approved_at, created_at, updated_at
) VALUES (
    sqlc.arg(id), sqlc.arg(target_store_id)::bigint, sqlc.arg(source), sqlc.arg(source_reference),
    sqlc.arg(granted_capabilities)::text[],
    sqlc.arg(now)::timestamptz, sqlc.narg(effective_until), sqlc.arg(approved_by_admin_id),
    sqlc.arg(now)::timestamptz, sqlc.arg(now)::timestamptz, sqlc.arg(now)::timestamptz
)
ON CONFLICT (target_store_id) WHERE target_store_id IS NOT NULL
DO UPDATE SET
    source               = EXCLUDED.source,
    source_reference     = EXCLUDED.source_reference,
    granted_capabilities = EXCLUDED.granted_capabilities,
    effective_from       = EXCLUDED.effective_from,
    effective_until      = EXCLUDED.effective_until,
    approved_by_admin_id = EXCLUDED.approved_by_admin_id,
    approved_at          = EXCLUDED.approved_at,
    revoked_at           = NULL,
    revoked_by_admin_id  = NULL,
    revoke_reason        = NULL,
    updated_at           = EXCLUDED.updated_at
WHERE delegation.seller_delegation_authorizations.revoked_at IS NOT NULL
RETURNING *;

-- name: UpdateDelegationAuthorization :one
UPDATE delegation.seller_delegation_authorizations
SET source = sqlc.arg(source),
    source_reference = sqlc.arg(source_reference),
    granted_capabilities = sqlc.arg(granted_capabilities)::text[],
    effective_until = sqlc.narg(effective_until),
    updated_at = sqlc.arg(now)::timestamptz
WHERE target_store_id = sqlc.arg(target_store_id)::bigint AND revoked_at IS NULL
RETURNING *;

-- name: RevokeDelegationAuthorization :one
UPDATE delegation.seller_delegation_authorizations
SET revoked_at = sqlc.arg(now)::timestamptz,
    revoked_by_admin_id = sqlc.arg(revoked_by_admin_id)::bigint,
    revoke_reason = sqlc.arg(revoke_reason)::text,
    updated_at = sqlc.arg(now)::timestamptz
WHERE target_store_id = sqlc.arg(target_store_id)::bigint AND revoked_at IS NULL
RETURNING *;

-- name: InsertDelegationAuthorizationAuditLog :one
INSERT INTO delegation.delegation_authorization_audit_logs (
    id, authorization_id, target_store_id, action, actor_admin_id, actor_display_name, operation_reason,
    source, source_reference, capabilities_before, capabilities_after,
    effective_until_before, effective_until_after, idempotency_key, request_id, correlation_id, created_at
) VALUES (
    sqlc.arg(id), sqlc.arg(authorization_id), sqlc.arg(target_store_id), sqlc.arg(action), sqlc.arg(actor_admin_id),
    sqlc.arg(actor_display_name), sqlc.arg(operation_reason),
    sqlc.narg(source), sqlc.narg(source_reference), sqlc.narg(capabilities_before)::text[],
    sqlc.narg(capabilities_after)::text[],
    sqlc.narg(effective_until_before), sqlc.narg(effective_until_after), sqlc.narg(idempotency_key),
    sqlc.narg(request_id), sqlc.narg(correlation_id), sqlc.arg(now)::timestamptz
)
ON CONFLICT (authorization_id, idempotency_key) WHERE idempotency_key IS NOT NULL DO NOTHING
RETURNING *;

-- name: GetDelegationAuthorizationAuditLogByIdempotencyKey :one
SELECT * FROM delegation.delegation_authorization_audit_logs
WHERE authorization_id = sqlc.arg(authorization_id) AND idempotency_key = sqlc.arg(idempotency_key);

-- name: ListDelegationAuthorizationAuditLogs :many
SELECT * FROM delegation.delegation_authorization_audit_logs
WHERE target_store_id = sqlc.arg(target_store_id)
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(row_limit) OFFSET sqlc.arg(row_offset);

-- name: CountDelegationAuthorizationAuditLogs :one
SELECT count(*) FROM delegation.delegation_authorization_audit_logs WHERE target_store_id = $1;
