
-- name: InsertDelegationAuditLog :one
INSERT INTO delegation.admin_operation_audit_logs (
    id, session_id, actor_admin_id, actor_display_name,
    target_store_id, target_seller_account_id, target_store_name,
    origin, capability, action, resource_type, resource_id, resource_name,
    result, operation_reason, case_reference,
    request_id, correlation_id, idempotency_key, payload_hash,
    before_snapshot, resource_version_before, user_agent,
    risk_level, notify_seller, created_at
) VALUES (
    sqlc.arg(id), sqlc.arg(session_id), sqlc.arg(actor_admin_id), sqlc.arg(actor_display_name),
    sqlc.arg(target_store_id), sqlc.arg(target_seller_account_id), sqlc.arg(target_store_name),
    sqlc.arg(origin), sqlc.arg(capability), sqlc.arg(action), sqlc.arg(resource_type), sqlc.narg(resource_id),
    sqlc.narg(resource_name),
    'PENDING', sqlc.narg(operation_reason), sqlc.narg(case_reference),
    sqlc.arg(request_id), sqlc.arg(correlation_id), sqlc.narg(idempotency_key), sqlc.narg(payload_hash),
    sqlc.narg(before_snapshot), sqlc.narg(resource_version_before), sqlc.narg(user_agent),
    sqlc.arg(risk_level), sqlc.arg(notify_seller), sqlc.arg(now)::timestamptz
)
ON CONFLICT (session_id, idempotency_key) WHERE idempotency_key IS NOT NULL DO NOTHING
RETURNING *;

-- name: InsertScheduledPublishAuditLog :one
INSERT INTO delegation.admin_operation_audit_logs (
    id, session_id, actor_admin_id, actor_display_name,
    target_store_id, target_seller_account_id, target_store_name,
    origin, capability, action, resource_type, resource_id, resource_name,
    result, operation_reason, case_reference,
    request_id, correlation_id, idempotency_key,
    risk_level, notify_seller, created_at, confirmed_at, notified_at
) VALUES (
    sqlc.arg(id), sqlc.arg(session_id), sqlc.arg(actor_admin_id), sqlc.arg(actor_display_name),
    sqlc.arg(target_store_id), sqlc.arg(target_seller_account_id), sqlc.arg(target_store_name),
    'SCHEDULED', 'posts.publish', 'POST_PUBLISH', 'POST', sqlc.arg(resource_id)::text, sqlc.narg(resource_name),
    'SUCCEEDED', sqlc.narg(operation_reason), sqlc.narg(case_reference),
    sqlc.arg(request_id), sqlc.arg(correlation_id), sqlc.arg(idempotency_key)::text,
    sqlc.arg(risk_level), sqlc.arg(notify_seller)::boolean, sqlc.arg(now)::timestamptz, date_trunc('second', sqlc.arg(now)::timestamptz),
    CASE WHEN sqlc.arg(notify_seller)::boolean THEN date_trunc('second', sqlc.arg(now)::timestamptz) END
)
ON CONFLICT (session_id, idempotency_key) WHERE idempotency_key IS NOT NULL DO NOTHING
RETURNING *;

-- name: InsertSucceededAuditLog :one
INSERT INTO delegation.admin_operation_audit_logs (
    id, session_id, actor_admin_id, actor_display_name,
    target_store_id, target_seller_account_id, target_store_name,
    origin, capability, action, resource_type, resource_id, resource_name,
    result, operation_reason, case_reference,
    request_id, correlation_id, idempotency_key, payload_hash,
    resource_version_before, user_agent,
    risk_level, notify_seller, created_at, confirmed_at, notified_at
) VALUES (
    sqlc.arg(id), sqlc.arg(session_id), sqlc.arg(actor_admin_id), sqlc.arg(actor_display_name),
    sqlc.arg(target_store_id), sqlc.arg(target_seller_account_id), sqlc.arg(target_store_name),
    'INTERACTIVE', sqlc.arg(capability), sqlc.arg(action), sqlc.arg(resource_type), sqlc.narg(resource_id)::text, sqlc.narg(resource_name),
    'SUCCEEDED', sqlc.arg(operation_reason), sqlc.narg(case_reference),
    sqlc.arg(request_id), sqlc.arg(correlation_id), sqlc.arg(idempotency_key)::text, sqlc.arg(payload_hash),
    sqlc.narg(resource_version_before), sqlc.narg(user_agent),
    sqlc.arg(risk_level), sqlc.arg(notify_seller)::boolean, sqlc.arg(now)::timestamptz, date_trunc('second', sqlc.arg(now)::timestamptz),
    CASE WHEN sqlc.arg(notify_seller)::boolean THEN date_trunc('second', sqlc.arg(now)::timestamptz) END
)
ON CONFLICT (session_id, idempotency_key) WHERE idempotency_key IS NOT NULL DO NOTHING
RETURNING *;

-- name: InsertFailedAuditLog :one
INSERT INTO delegation.admin_operation_audit_logs (
    id, session_id, actor_admin_id, actor_display_name,
    target_store_id, target_seller_account_id, target_store_name,
    origin, capability, action, resource_type, resource_id, resource_name,
    result, failure_reason, operation_reason, case_reference,
    request_id, correlation_id, resource_version_before, user_agent,
    risk_level, notify_seller, created_at, confirmed_at
) VALUES (
    sqlc.arg(id), sqlc.arg(session_id), sqlc.arg(actor_admin_id), sqlc.arg(actor_display_name),
    sqlc.arg(target_store_id), sqlc.arg(target_seller_account_id), sqlc.arg(target_store_name),
    'INTERACTIVE', sqlc.arg(capability), sqlc.arg(action), sqlc.arg(resource_type), sqlc.narg(resource_id)::text, sqlc.narg(resource_name),
    'FAILED', sqlc.arg(failure_reason)::text, sqlc.narg(operation_reason), sqlc.narg(case_reference),
    sqlc.arg(request_id), sqlc.arg(correlation_id), sqlc.narg(resource_version_before), sqlc.narg(user_agent),
    sqlc.arg(risk_level), false, sqlc.arg(now)::timestamptz, date_trunc('second', sqlc.arg(now)::timestamptz)
)
RETURNING *;

-- name: SetAuditLogOutcome :one
UPDATE delegation.admin_operation_audit_logs
SET resource_id = COALESCE(resource_id, sqlc.narg(resource_id)::text),
    resource_name = COALESCE(sqlc.narg(resource_name), resource_name),
    resource_version_after = sqlc.narg(resource_version_after)
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: GetDelegationAuditLog :one
SELECT * FROM delegation.admin_operation_audit_logs WHERE id = $1;

-- name: GetDelegationAuditLogByIdempotencyKey :one
SELECT * FROM delegation.admin_operation_audit_logs
WHERE session_id = sqlc.arg(session_id) AND idempotency_key = sqlc.arg(idempotency_key);

-- name: LatestDelegationAuditReason :one
SELECT operation_reason, notify_seller FROM delegation.admin_operation_audit_logs
WHERE session_id = sqlc.arg(session_id) AND resource_id = sqlc.arg(resource_id)::text AND action = sqlc.arg(action)
ORDER BY created_at DESC, id DESC
LIMIT 1;

-- name: ConfirmDelegationAuditLog :one
UPDATE delegation.admin_operation_audit_logs
SET result = sqlc.arg(result),
    resource_id = COALESCE(sqlc.narg(resource_id), resource_id),
    after_snapshot = sqlc.narg(after_snapshot),
    changed_fields = sqlc.narg(changed_fields),
    resource_version_after = sqlc.narg(resource_version_after),
    biz_code = sqlc.narg(biz_code),
    failure_reason = sqlc.narg(failure_reason),
    confirmed_at = date_trunc('second', sqlc.arg(now)::timestamptz),
    notified_at = CASE
        WHEN sqlc.arg(result)::text = 'SUCCEEDED' AND notify_seller
        THEN date_trunc('second', sqlc.arg(now)::timestamptz)
    END
WHERE id = sqlc.arg(id) AND result = 'PENDING'
RETURNING *;

-- name: ListDelegationAuditLogs :many
SELECT * FROM delegation.admin_operation_audit_logs
WHERE (sqlc.narg(session_id)::bigint IS NULL OR session_id = sqlc.narg(session_id)::bigint)
  AND (sqlc.narg(actor_admin_id)::bigint IS NULL OR actor_admin_id = sqlc.narg(actor_admin_id)::bigint)
  AND (sqlc.narg(target_store_id)::bigint IS NULL OR target_store_id = sqlc.narg(target_store_id)::bigint)
  AND (sqlc.arg(action)::text = '' OR action = sqlc.arg(action)::text)
  AND (sqlc.arg(resource_type)::text = '' OR resource_type = sqlc.arg(resource_type)::text)
  AND (sqlc.arg(result)::text = '' OR result = sqlc.arg(result)::text)
  AND (sqlc.narg(date_from)::timestamptz IS NULL OR created_at >= sqlc.narg(date_from)::timestamptz)
  AND (sqlc.narg(date_to)::timestamptz IS NULL OR created_at <= sqlc.narg(date_to)::timestamptz)
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(row_limit) OFFSET sqlc.arg(row_offset);

-- name: CountDelegationAuditLogs :one
SELECT count(*) FROM delegation.admin_operation_audit_logs
WHERE (sqlc.narg(session_id)::bigint IS NULL OR session_id = sqlc.narg(session_id)::bigint)
  AND (sqlc.narg(actor_admin_id)::bigint IS NULL OR actor_admin_id = sqlc.narg(actor_admin_id)::bigint)
  AND (sqlc.narg(target_store_id)::bigint IS NULL OR target_store_id = sqlc.narg(target_store_id)::bigint)
  AND (sqlc.arg(action)::text = '' OR action = sqlc.arg(action)::text)
  AND (sqlc.arg(resource_type)::text = '' OR resource_type = sqlc.arg(resource_type)::text)
  AND (sqlc.arg(result)::text = '' OR result = sqlc.arg(result)::text)
  AND (sqlc.narg(date_from)::timestamptz IS NULL OR created_at >= sqlc.narg(date_from)::timestamptz)
  AND (sqlc.narg(date_to)::timestamptz IS NULL OR created_at <= sqlc.narg(date_to)::timestamptz);

-- name: DelegationAuditSummary :one
SELECT count(*) FILTER (WHERE result = 'SUCCEEDED')                         AS succeeded_count,
       count(*) FILTER (WHERE risk_level = 'HIGH' AND result = 'SUCCEEDED') AS high_risk_count,
       count(*) FILTER (WHERE result = 'PENDING')                           AS pending_count,
       count(*) FILTER (WHERE notified_at IS NOT NULL)                      AS notified_count,
       count(*) FILTER (WHERE lower(action) LIKE ANY (sqlc.arg(fulfillment_patterns)::text[])) AS fulfillment_count,
       count(*) FILTER (WHERE lower(action) LIKE ANY (sqlc.arg(finance_patterns)::text[]))     AS finance_count
FROM delegation.admin_operation_audit_logs
WHERE session_id = sqlc.arg(session_id);
