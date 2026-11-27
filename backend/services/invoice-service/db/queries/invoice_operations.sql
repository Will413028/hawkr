
-- name: FetchDueInvoiceOperations :many
SELECT * FROM invoice_operations
WHERE status IN ('READY', 'RETRY_SCHEDULED', 'SENDING')
  AND next_attempt_at <= sqlc.arg(now)
ORDER BY statutory_deadline_at NULLS LAST, next_attempt_at
LIMIT sqlc.arg(row_limit);

-- name: ClaimInvoiceOperation :execrows
UPDATE invoice_operations
SET status = 'SENDING', next_attempt_at = sqlc.arg(lease_until),
    started_at = COALESCE(started_at, sqlc.arg(now)), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id)
  AND status IN ('READY', 'RETRY_SCHEDULED', 'SENDING')
  AND next_attempt_at <= sqlc.arg(claimable_before);

-- name: MarkInvoiceOperationDone :execrows
UPDATE invoice_operations
SET status = 'SUCCEEDED', completed_at = sqlc.arg(now),
    updated_at = sqlc.arg(now), version = version + 1
WHERE id = sqlc.arg(id) AND status = 'SENDING';

-- name: MarkInvoiceOperationFailed :execrows
UPDATE invoice_operations
SET status = 'FAILED_PERMANENT', last_error = sqlc.arg(last_error),
    completed_at = sqlc.arg(now), updated_at = sqlc.arg(now), version = version + 1
WHERE id = sqlc.arg(id) AND status IN ('SENDING', 'RETRY_SCHEDULED');

-- name: MarkInvoiceOperationFailedWithProviderResult :execrows
UPDATE invoice_operations
SET status = 'FAILED_PERMANENT',
    last_error = sqlc.arg(last_error),
    provider_code = sqlc.narg(provider_code),
    provider_message = sqlc.narg(provider_message),
    completed_at = sqlc.arg(now), updated_at = sqlc.arg(now), version = version + 1
WHERE id = sqlc.arg(id) AND status IN ('SENDING', 'RETRY_SCHEDULED');

-- name: UpdateInvoiceOperationAttempt :exec
UPDATE invoice_operations
SET attempt_count = sqlc.arg(attempt_count),
    next_attempt_at = sqlc.arg(next_attempt_at),
    last_error = sqlc.arg(last_error),
    status = 'RETRY_SCHEDULED',
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id);

-- name: FetchPendingInvoiceRechecks :many
SELECT * FROM invoice_operations
WHERE status IN ('WAITING_CONFIRMATION', 'PROVIDER_STATUS_RECHECK', 'FAILED_UNKNOWN',
                 'CANCEL_REQUESTED', 'CANCEL_BLOCKED_UNKNOWN')
  AND next_attempt_at <= sqlc.arg(now)
ORDER BY statutory_deadline_at NULLS LAST, next_attempt_at
LIMIT sqlc.arg(row_limit);

-- name: ClaimInvoiceRecheck :execrows
UPDATE invoice_operations
SET next_attempt_at = sqlc.arg(lease_until), status = 'PROVIDER_STATUS_RECHECK', updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id)
  AND status IN ('WAITING_CONFIRMATION', 'PROVIDER_STATUS_RECHECK', 'FAILED_UNKNOWN',
                 'CANCEL_REQUESTED', 'CANCEL_BLOCKED_UNKNOWN')
  AND next_attempt_at <= sqlc.arg(claimable_before);

-- name: MarkInvoiceRecheckResolved :execrows
UPDATE invoice_operations
SET status = sqlc.arg(next_status),
    next_attempt_at = sqlc.arg(next_attempt_at),
    unknown_result = false,
    unknown_since = NULL,
    recheck_list_page = 0,
    provider_code = sqlc.narg(provider_code),
    provider_message = sqlc.narg(provider_message),
    completed_at = sqlc.narg(completed_at),
    cancel_evidence_kind = COALESCE(sqlc.narg(cancel_evidence_kind), cancel_evidence_kind),
    cancel_confirmed_at = COALESCE(sqlc.narg(cancel_confirmed_at), cancel_confirmed_at),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id) AND status = 'PROVIDER_STATUS_RECHECK';

-- name: MarkInvoiceRecheckUnresolved :execrows
UPDATE invoice_operations
SET status = sqlc.arg(next_status),
    next_attempt_at = sqlc.arg(next_attempt_at),
    recheck_list_page = 0,
    provider_code = sqlc.narg(provider_code),
    provider_message = sqlc.narg(provider_message),
    last_error = sqlc.narg(last_error),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id) AND status = 'PROVIDER_STATUS_RECHECK';

-- name: UpdateInvoiceRecheckAttempt :execrows
UPDATE invoice_operations
SET status = sqlc.arg(next_status),
    next_attempt_at = sqlc.arg(next_attempt_at),
    recheck_list_page = sqlc.arg(recheck_list_page),
    provider_code = sqlc.narg(provider_code),
    provider_message = sqlc.narg(provider_message),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id) AND status = 'PROVIDER_STATUS_RECHECK';

-- name: ReleaseExpiredInvoiceOperationHolds :execrows
UPDATE invoice_operations
SET status = 'READY', held_until = NULL, updated_at = sqlc.arg(now)
WHERE status = 'HELD' AND held_until <= sqlc.arg(now);

-- name: PromoteDueInvoiceOperationTriggers :execrows
UPDATE invoice_operations o
SET status = 'READY', updated_at = sqlc.arg(now)
FROM invoice_documents d
WHERE d.id = o.document_id
  AND o.status = 'AWAITING_TRIGGER'
  AND d.issue_not_before <= sqlc.arg(now);


-- name: InsertInvoiceOperation :one
INSERT INTO invoice_operations (
  id, document_id, adjustment_id, operation_type, operation_key, provider_request_no,
  status, request_snapshot_hash, max_attempts, next_attempt_at,
  statutory_deadline_at, created_at, updated_at)
VALUES (
  sqlc.arg(id), sqlc.arg(document_id), sqlc.narg(adjustment_id),
  sqlc.arg(operation_type), sqlc.arg(operation_key), sqlc.arg(provider_request_no),
  'CREATED', sqlc.arg(request_snapshot_hash), sqlc.arg(max_attempts), sqlc.arg(next_attempt_at),
  sqlc.narg(statutory_deadline_at), sqlc.arg(now), sqlc.arg(now))
ON CONFLICT DO NOTHING
RETURNING *;

-- name: PromoteCreatedInvoiceOperation :execrows
UPDATE invoice_operations
SET status = sqlc.arg(next_status),
    next_attempt_at = sqlc.arg(next_attempt_at),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND status = 'CREATED';

-- name: HoldInvoiceOperation :execrows
UPDATE invoice_operations
SET status = 'HELD',
    held_until = sqlc.arg(held_until),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND status IN ('READY', 'RETRY_SCHEDULED');

-- name: CancelInvoiceOperationBeforeIssue :execrows
UPDATE invoice_operations
SET status = 'CANCELLED_BEFORE_ISSUE',
    held_until = NULL,
    cancel_reason_code = sqlc.narg(cancel_reason_code),
    cancel_reason = sqlc.narg(cancel_reason),
    completed_at = sqlc.arg(now),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND status IN ('CREATED', 'AWAITING_TRIGGER', 'READY', 'RETRY_SCHEDULED', 'HELD');

-- name: MarkInvoiceOperationWaitingConfirmation :execrows
UPDATE invoice_operations
SET status = 'WAITING_CONFIRMATION',
    unknown_result = true,
    unknown_since = COALESCE(unknown_since, sqlc.arg(now)),
    next_attempt_at = sqlc.arg(next_attempt_at),
    provider_code = sqlc.narg(provider_code),
    provider_message = sqlc.narg(provider_message),
    last_error = sqlc.narg(last_error),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND status IN ('SENDING', 'PROVIDER_STATUS_RECHECK');

-- name: MarkInvoiceOperationRetryExhausted :execrows
UPDATE invoice_operations
SET status = 'FAILED_RETRY_EXHAUSTED',
    last_error = sqlc.arg(last_error),
    completed_at = sqlc.arg(now),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND status = 'RETRY_SCHEDULED';

-- name: MarkInvoiceOperationSellerActionRequired :execrows
UPDATE invoice_operations
SET status = 'SELLER_ACTION_REQUIRED',
    provider_code = sqlc.narg(provider_code),
    provider_message = sqlc.narg(provider_message),
    last_error = sqlc.narg(last_error),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND status = 'SENDING';

-- name: PromoteSellerActionRequiredOperations :execrows
UPDATE invoice_operations o
SET status = 'READY',
    attempt_count = 0,
    next_attempt_at = sqlc.arg(now),
    updated_at = sqlc.arg(now),
    version = o.version + 1
FROM invoice_documents d
WHERE d.id = o.document_id
  AND o.status = 'SELLER_ACTION_REQUIRED'
  AND d.issuer_id = sqlc.arg(issuer_id);

-- name: AdminRetryInvoiceOperationAfterFix :execrows
UPDATE invoice_operations
SET status = 'READY',
    request_snapshot_hash = sqlc.arg(request_snapshot_hash),
    next_attempt_at = sqlc.arg(now),
    last_error = NULL,
    completed_at = NULL,
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND status = 'FAILED_PERMANENT'
  AND version = sqlc.arg(expected_version);

-- name: AdminResetInvoiceOperationAttempts :execrows
UPDATE invoice_operations
SET status = 'READY',
    attempt_count = 0,
    next_attempt_at = sqlc.arg(now),
    last_error = NULL,
    completed_at = NULL,
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND status IN ('FAILED_RETRY_EXHAUSTED', 'SELLER_ACTION_REQUIRED')
  AND version = sqlc.arg(expected_version);

-- name: RequestInvoiceOperationCancel :execrows
UPDATE invoice_operations
SET status = 'CANCEL_REQUESTED',
    cancel_requested_by = sqlc.arg(cancel_requested_by),
    cancel_requested_at = sqlc.arg(now),
    cancel_reason_code = sqlc.arg(cancel_reason_code),
    cancel_reason = sqlc.arg(cancel_reason),
    next_attempt_at = sqlc.arg(now),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND status IN ('READY', 'RETRY_SCHEDULED', 'FAILED_PERMANENT',
                 'FAILED_RETRY_EXHAUSTED', 'SELLER_ACTION_REQUIRED')
  AND version = sqlc.arg(expected_version);

-- name: ConfirmInvoiceOperationCancel :execrows
UPDATE invoice_operations
SET status = 'CANCELLED_BY_ADMIN',
    cancel_confirmed_by = sqlc.arg(cancel_confirmed_by),
    cancel_confirmed_at = sqlc.arg(now),
    cancel_evidence_kind = sqlc.arg(cancel_evidence_kind),
    cancel_evidence_object_key = sqlc.narg(cancel_evidence_object_key),
    completed_at = sqlc.arg(now),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND status = 'CANCEL_BLOCKED_UNKNOWN'
  AND cancel_requested_at IS NOT NULL
  AND cancel_confirmed_at IS NULL
  AND version = sqlc.arg(expected_version);

-- name: AllowInvoiceOperationManualProcess :execrows
UPDATE invoice_operations
SET status = 'MANUAL_PROCESS_ALLOWED',
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND status = 'CANCELLED_BY_ADMIN'
  AND cancel_requested_by IS NOT NULL
  AND cancel_requested_by <> sqlc.arg(actor_account_id)
  AND version = sqlc.arg(expected_version);

-- name: MarkInvoiceOperationResolvedExternally :execrows
UPDATE invoice_operations o
SET status = 'RESOLVED_EXTERNALLY',
    completed_at = sqlc.arg(now),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE o.id = sqlc.arg(id)
  AND o.version = sqlc.arg(expected_version)
  AND (
    o.status = 'MANUAL_PROCESS_ALLOWED'
    OR (o.status = 'NOT_APPLICABLE' AND EXISTS (
          SELECT 1 FROM invoice_documents d
          WHERE d.id = o.document_id
            AND d.fulfillment_mode IN ('SELLER_SELF_ISSUE', 'LEGACY_MANUAL')))
  );

-- name: FailOpenInvoiceOperationsForIssuer :execrows
UPDATE invoice_operations o
SET status = 'FAILED_PERMANENT',
    held_until = NULL,
    cancel_reason_code = sqlc.arg(cancel_reason_code),
    last_error = sqlc.arg(last_error),
    completed_at = sqlc.arg(now),
    updated_at = sqlc.arg(now),
    version = o.version + 1
FROM invoice_documents d
WHERE d.id = o.document_id
  AND d.issuer_id = sqlc.arg(issuer_id)
  AND o.status IN ('CREATED','AWAITING_TRIGGER','READY','HELD','SENDING','RETRY_SCHEDULED',
                   'WAITING_CONFIRMATION','PROVIDER_STATUS_RECHECK','FAILED_PERMANENT',
                   'FAILED_RETRY_EXHAUSTED','FAILED_UNKNOWN','SELLER_ACTION_REQUIRED',
                   'CANCEL_REQUESTED','CANCEL_BLOCKED_UNKNOWN','CANCELLED_BY_ADMIN',
                   'MANUAL_PROCESS_ALLOWED');

-- name: CountOpenInvoiceOperationsForIssuer :one
SELECT COUNT(*)::bigint
FROM invoice_operations o
JOIN invoice_documents d ON d.id = o.document_id
WHERE d.issuer_id = sqlc.arg(issuer_id)
  AND o.status IN ('CREATED','AWAITING_TRIGGER','READY','HELD','SENDING','RETRY_SCHEDULED',
                   'WAITING_CONFIRMATION','PROVIDER_STATUS_RECHECK','FAILED_PERMANENT',
                   'FAILED_RETRY_EXHAUSTED','FAILED_UNKNOWN','SELLER_ACTION_REQUIRED',
                   'CANCEL_REQUESTED','CANCEL_BLOCKED_UNKNOWN','CANCELLED_BY_ADMIN',
                   'MANUAL_PROCESS_ALLOWED');

-- name: GetInvoiceOperationByID :one
SELECT * FROM invoice_operations WHERE id = sqlc.arg(id);

-- name: GetInvoiceOperationByKey :one
SELECT * FROM invoice_operations WHERE operation_key = sqlc.arg(operation_key);

-- name: ListInvoiceOperationsByDocument :many
SELECT * FROM invoice_operations
WHERE document_id = sqlc.arg(document_id)
ORDER BY created_at DESC;


-- name: GetSellerInvoiceOperationByID :one
SELECT sqlc.embed(o),
       d.source_ref            AS document_source_ref,
       d.manual_reason_code    AS document_manual_reason_code,
       d.document_kind         AS document_kind,
       d.status                AS document_status,
       d.external_invoice_no   AS document_external_invoice_no,
       d.external_invoice_date AS document_external_invoice_date,
       d.external_reported_at  AS document_external_reported_at,
       i.issuer_ref_id         AS issuer_ref_id
FROM invoice_operations o
JOIN invoice_documents d ON d.id = o.document_id
JOIN invoice_issuers   i ON i.id = d.issuer_id
WHERE o.id = sqlc.arg(id)
  AND i.issuer_type = 'SELLER'
  AND i.issuer_ref_id = sqlc.arg(store_id)::bigint;

-- name: ListOrderExternalIssuances :many
SELECT sqlc.embed(o),
       d.source_ref            AS document_source_ref,
       d.manual_reason_code    AS document_manual_reason_code,
       d.document_kind         AS document_kind,
       d.status                AS document_status,
       d.external_invoice_no   AS document_external_invoice_no,
       d.external_invoice_date AS document_external_invoice_date,
       d.external_reported_at  AS document_external_reported_at,
       i.issuer_ref_id         AS issuer_ref_id
FROM invoice_operations o
JOIN invoice_documents d ON d.id = o.document_id
JOIN invoice_issuers   i ON i.id = d.issuer_id
WHERE i.issuer_type = 'SELLER'
  AND i.issuer_ref_id = sqlc.arg(store_id)::bigint
  AND d.source_type = 'ORDER'
  AND d.source_ref = sqlc.arg(order_no)::text
  AND o.status = ANY(sqlc.arg(allowed_statuses)::text[])
ORDER BY o.statutory_deadline_at ASC NULLS LAST, o.id ASC
LIMIT sqlc.arg(row_limit) OFFSET sqlc.arg(row_offset);

-- name: CountOrderExternalIssuances :one
SELECT COUNT(*)::bigint
FROM invoice_operations o
JOIN invoice_documents d ON d.id = o.document_id
JOIN invoice_issuers   i ON i.id = d.issuer_id
WHERE i.issuer_type = 'SELLER'
  AND i.issuer_ref_id = sqlc.arg(store_id)::bigint
  AND d.source_type = 'ORDER'
  AND d.source_ref = sqlc.arg(order_no)::text
  AND o.status = ANY(sqlc.arg(allowed_statuses)::text[]);

-- name: ListInvoiceOperationsForAdmin :many
SELECT sqlc.embed(o),
       d.source_ref            AS document_source_ref,
       d.manual_reason_code    AS document_manual_reason_code,
       d.document_kind         AS document_kind,
       d.status                AS document_status,
       d.external_invoice_no   AS document_external_invoice_no,
       d.external_invoice_date AS document_external_invoice_date,
       d.external_reported_at  AS document_external_reported_at,
       i.issuer_ref_id         AS issuer_ref_id
FROM invoice_operations o
JOIN invoice_documents d ON d.id = o.document_id
JOIN invoice_issuers   i ON i.id = d.issuer_id
WHERE (
        COALESCE(cardinality(sqlc.arg(status_filters)::text[]), 0) = 0
        OR o.status = ANY(sqlc.arg(status_filters)::text[])
        OR (sqlc.arg(include_manual_documents)::boolean
            AND d.status = ANY(sqlc.arg(manual_document_statuses)::text[]))
      )
  AND (NOT sqlc.arg(has_store_id)::boolean
       OR (i.issuer_type = 'SELLER' AND i.issuer_ref_id = sqlc.arg(store_id)::bigint))
  AND (sqlc.arg(order_no)::text = ''
       OR (d.source_type = 'ORDER' AND d.source_ref = sqlc.arg(order_no)::text))
  AND (sqlc.arg(operation_type_filter)::text = ''
       OR o.operation_type = sqlc.arg(operation_type_filter)::text)
  AND (NOT sqlc.arg(has_upload_status)::boolean
       OR (d.provider_upload_status = ANY(sqlc.arg(upload_status_codes)::smallint[])
           OR (sqlc.arg(upload_status_include_null)::boolean
               AND d.provider_upload_status IS NULL)))
  AND (sqlc.arg(manual_reason_filter)::text = ''
       OR d.manual_reason_code = sqlc.arg(manual_reason_filter)::text)
  AND (NOT sqlc.arg(exclude_resolved_documents)::boolean
       OR NOT (d.status = ANY(COALESCE(sqlc.arg(resolved_document_statuses)::text[], ARRAY[]::text[]))))
ORDER BY o.created_at DESC, o.id DESC
LIMIT sqlc.arg(row_limit) OFFSET sqlc.arg(row_offset);

-- name: CountInvoiceOperationsForAdmin :one
SELECT COUNT(*)::bigint
FROM invoice_operations o
JOIN invoice_documents d ON d.id = o.document_id
JOIN invoice_issuers   i ON i.id = d.issuer_id
WHERE (
        COALESCE(cardinality(sqlc.arg(status_filters)::text[]), 0) = 0
        OR o.status = ANY(sqlc.arg(status_filters)::text[])
        OR (sqlc.arg(include_manual_documents)::boolean
            AND d.status = ANY(sqlc.arg(manual_document_statuses)::text[]))
      )
  AND (NOT sqlc.arg(has_store_id)::boolean
       OR (i.issuer_type = 'SELLER' AND i.issuer_ref_id = sqlc.arg(store_id)::bigint))
  AND (sqlc.arg(order_no)::text = ''
       OR (d.source_type = 'ORDER' AND d.source_ref = sqlc.arg(order_no)::text))
  AND (sqlc.arg(operation_type_filter)::text = ''
       OR o.operation_type = sqlc.arg(operation_type_filter)::text)
  AND (NOT sqlc.arg(has_upload_status)::boolean
       OR (d.provider_upload_status = ANY(sqlc.arg(upload_status_codes)::smallint[])
           OR (sqlc.arg(upload_status_include_null)::boolean
               AND d.provider_upload_status IS NULL)))
  AND (sqlc.arg(manual_reason_filter)::text = ''
       OR d.manual_reason_code = sqlc.arg(manual_reason_filter)::text)
  AND (NOT sqlc.arg(exclude_resolved_documents)::boolean
       OR NOT (d.status = ANY(COALESCE(sqlc.arg(resolved_document_statuses)::text[], ARRAY[]::text[]))));

-- name: GetInvoiceOperationForAdminByID :one
SELECT sqlc.embed(o),
       d.source_ref            AS document_source_ref,
       d.manual_reason_code    AS document_manual_reason_code,
       d.document_kind         AS document_kind,
       d.status                AS document_status,
       d.external_invoice_no   AS document_external_invoice_no,
       d.external_invoice_date AS document_external_invoice_date,
       d.external_reported_at  AS document_external_reported_at,
       i.issuer_ref_id         AS issuer_ref_id
FROM invoice_operations o
JOIN invoice_documents d ON d.id = o.document_id
JOIN invoice_issuers   i ON i.id = d.issuer_id
WHERE o.id = sqlc.arg(id);

-- name: ListPriorityInvoiceOperationsForAdmin :many
SELECT sqlc.embed(o),
       d.source_ref            AS document_source_ref,
       d.manual_reason_code    AS document_manual_reason_code,
       d.document_kind         AS document_kind,
       d.status                AS document_status,
       d.external_invoice_no   AS document_external_invoice_no,
       d.external_invoice_date AS document_external_invoice_date,
       d.external_reported_at  AS document_external_reported_at,
       i.issuer_ref_id         AS issuer_ref_id
FROM invoice_operations o
JOIN invoice_documents d ON d.id = o.document_id
JOIN invoice_issuers   i ON i.id = d.issuer_id
WHERE (o.status = ANY(sqlc.arg(open_statuses)::text[])
    OR d.status = ANY(sqlc.arg(manual_document_statuses)::text[]))
  AND NOT (d.status = ANY(COALESCE(sqlc.arg(resolved_document_statuses)::text[], ARRAY[]::text[])))
ORDER BY o.statutory_deadline_at ASC NULLS LAST, o.id ASC
LIMIT sqlc.arg(row_limit);

-- name: CountCrossPeriodPending :one
SELECT count(*)::bigint
FROM invoice_operations o
JOIN invoice_documents d ON d.id = o.document_id
WHERE o.operation_type IN ('VOID', 'ALLOWANCE', 'VOID_ALLOWANCE')
  AND o.status NOT IN ('SUCCEEDED', 'NOT_APPLICABLE', 'CANCELLED_BEFORE_ISSUE',
                       'RESOLVED_EXTERNALLY', 'CANCELLED_BY_ADMIN')
  AND d.invoice_date_taipei IS NOT NULL
  AND d.invoice_date_taipei < make_date(
        EXTRACT(YEAR  FROM (sqlc.arg(now)::timestamptz AT TIME ZONE 'Asia/Taipei'))::int,
        (((EXTRACT(MONTH FROM (sqlc.arg(now)::timestamptz AT TIME ZONE 'Asia/Taipei'))::int - 1) / 2) * 2 + 1),
        1);

-- name: ReleaseInvoiceOperationHold :execrows
UPDATE invoice_operations
SET status          = 'READY',
    held_until      = NULL,
    next_attempt_at = sqlc.arg(now),
    updated_at      = sqlc.arg(now),
    version         = version + 1
WHERE id = sqlc.arg(id)
  AND status = 'HELD';
