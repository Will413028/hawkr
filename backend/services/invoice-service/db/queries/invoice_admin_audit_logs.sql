
-- name: InsertInvoiceAdminAuditLog :one
WITH ins AS (
INSERT INTO invoice_admin_audit_logs (
  actor_account_id, action, resource_type, resource_id, before_state, after_state,
  reason_code, reason_detail, approval_actor_account_id,
  trace_id, request_id, correlation_id, client_ip, user_agent, occurred_at)
VALUES (
  sqlc.arg(actor_account_id), sqlc.arg(action), sqlc.arg(resource_type), sqlc.arg(resource_id),
  sqlc.narg(before_state), sqlc.narg(after_state),
  sqlc.arg(reason_code), sqlc.arg(reason_detail), sqlc.narg(approval_actor_account_id),
  sqlc.narg(trace_id), sqlc.narg(request_id), sqlc.narg(correlation_id),
  sqlc.narg(client_ip), sqlc.narg(user_agent), sqlc.arg(now))
ON CONFLICT DO NOTHING
RETURNING id
)
SELECT id FROM ins
UNION ALL
SELECT id FROM invoice_admin_audit_logs
WHERE sqlc.narg(request_id)::text IS NOT NULL
  AND actor_account_id = sqlc.arg(actor_account_id)
  AND action = sqlc.arg(action)
  AND request_id = sqlc.narg(request_id)
LIMIT 1;

-- name: ListInvoiceAdminAuditLogsByResource :many
SELECT * FROM invoice_admin_audit_logs
WHERE resource_type = sqlc.arg(resource_type) AND resource_id = sqlc.arg(resource_id)
ORDER BY occurred_at DESC
LIMIT sqlc.arg(row_limit);

-- name: ListInvoiceAuditLogsForAdmin :many
SELECT sqlc.embed(l),
       d.source_ref     AS document_source_ref,
       d.id             AS resolved_document_id,
       o.id             AS resolved_operation_id,
       o.operation_type AS resolved_operation_type,
       i.issuer_ref_id  AS issuer_ref_id
FROM invoice_admin_audit_logs l
LEFT JOIN invoice_operations o
       ON l.resource_type = sqlc.arg(job_resource_type)::text AND o.id = l.resource_id
LEFT JOIN invoice_complaints cp
       ON l.resource_type = sqlc.arg(complaint_resource_type)::text AND cp.id = l.resource_id
LEFT JOIN seller_tax_profiles tp
       ON l.resource_type = sqlc.arg(tax_profile_resource_type)::text AND tp.id = l.resource_id
LEFT JOIN invoice_provider_accounts pa
       ON l.resource_type = sqlc.arg(provider_account_resource_type)::text AND pa.id = l.resource_id
LEFT JOIN invoice_documents d
       ON d.id = COALESCE(
            CASE WHEN l.resource_type = sqlc.arg(document_resource_type)::text
                 THEN l.resource_id END,
            o.document_id, cp.document_id)
LEFT JOIN invoice_issuers i
       ON i.id = COALESCE(d.issuer_id, tp.issuer_id, pa.issuer_id)
WHERE (NOT sqlc.arg(has_target_actor)::boolean
       OR l.actor_account_id = sqlc.arg(target_actor_account_id)::bigint)
  AND (NOT sqlc.arg(has_store_id)::boolean
       OR (i.issuer_type = 'SELLER' AND i.issuer_ref_id = sqlc.arg(store_id)::bigint))
  AND (sqlc.arg(order_no)::text = ''
       OR (d.source_type = 'ORDER' AND d.source_ref = sqlc.arg(order_no)::text))
  AND (NOT sqlc.arg(has_document_id)::boolean OR d.id = sqlc.arg(document_id)::bigint)
  AND (NOT sqlc.arg(has_job_id)::boolean      OR o.id = sqlc.arg(job_id)::bigint)
  AND (sqlc.arg(operation_type_filter)::text = ''
       OR o.operation_type = sqlc.arg(operation_type_filter)::text)
  AND (NOT sqlc.arg(has_date_from)::boolean
       OR l.occurred_at >= sqlc.arg(date_from)::timestamptz)
  AND (NOT sqlc.arg(has_date_to)::boolean
       OR l.occurred_at <= sqlc.arg(date_to)::timestamptz)
ORDER BY l.occurred_at DESC, l.id DESC
LIMIT sqlc.arg(row_limit) OFFSET sqlc.arg(row_offset);

-- name: CountInvoiceAuditLogsForAdmin :one
SELECT COUNT(*)::bigint
FROM invoice_admin_audit_logs l
LEFT JOIN invoice_operations o
       ON l.resource_type = sqlc.arg(job_resource_type)::text AND o.id = l.resource_id
LEFT JOIN invoice_complaints cp
       ON l.resource_type = sqlc.arg(complaint_resource_type)::text AND cp.id = l.resource_id
LEFT JOIN seller_tax_profiles tp
       ON l.resource_type = sqlc.arg(tax_profile_resource_type)::text AND tp.id = l.resource_id
LEFT JOIN invoice_provider_accounts pa
       ON l.resource_type = sqlc.arg(provider_account_resource_type)::text AND pa.id = l.resource_id
LEFT JOIN invoice_documents d
       ON d.id = COALESCE(
            CASE WHEN l.resource_type = sqlc.arg(document_resource_type)::text
                 THEN l.resource_id END,
            o.document_id, cp.document_id)
LEFT JOIN invoice_issuers i
       ON i.id = COALESCE(d.issuer_id, tp.issuer_id, pa.issuer_id)
WHERE (NOT sqlc.arg(has_target_actor)::boolean
       OR l.actor_account_id = sqlc.arg(target_actor_account_id)::bigint)
  AND (NOT sqlc.arg(has_store_id)::boolean
       OR (i.issuer_type = 'SELLER' AND i.issuer_ref_id = sqlc.arg(store_id)::bigint))
  AND (sqlc.arg(order_no)::text = ''
       OR (d.source_type = 'ORDER' AND d.source_ref = sqlc.arg(order_no)::text))
  AND (NOT sqlc.arg(has_document_id)::boolean OR d.id = sqlc.arg(document_id)::bigint)
  AND (NOT sqlc.arg(has_job_id)::boolean      OR o.id = sqlc.arg(job_id)::bigint)
  AND (sqlc.arg(operation_type_filter)::text = ''
       OR o.operation_type = sqlc.arg(operation_type_filter)::text)
  AND (NOT sqlc.arg(has_date_from)::boolean
       OR l.occurred_at >= sqlc.arg(date_from)::timestamptz)
  AND (NOT sqlc.arg(has_date_to)::boolean
       OR l.occurred_at <= sqlc.arg(date_to)::timestamptz);
