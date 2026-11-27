
-- name: FetchDueUploadRechecks :many
SELECT * FROM invoice_documents
WHERE status = 'ISSUED'
  AND upload_recheck_next_at IS NOT NULL
  AND upload_recheck_next_at <= sqlc.arg(now)
  AND (provider_upload_status IS NULL OR provider_upload_status IN (1, 2, 3, 31, 32))
ORDER BY upload_recheck_next_at
LIMIT sqlc.arg(row_limit);

-- name: ClaimUploadRecheck :execrows
UPDATE invoice_documents
SET upload_recheck_next_at = sqlc.arg(lease_until),
    upload_recheck_attempts = upload_recheck_attempts + 1, updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id)
  AND status = 'ISSUED'
  AND upload_recheck_next_at IS NOT NULL
  AND upload_recheck_next_at <= sqlc.arg(claimable_before)
  AND (provider_upload_status IS NULL OR provider_upload_status IN (1, 2, 3, 31, 32));

-- name: RecordUploadRecheckResult :execrows
UPDATE invoice_documents
SET provider_upload_status = sqlc.narg(provider_upload_status),
    provider_upload_checked_at = sqlc.arg(now),
    upload_recheck_next_at = sqlc.narg(upload_recheck_next_at),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id) AND status = 'ISSUED';

-- name: FetchDuePdfFetches :many
SELECT * FROM invoice_documents
WHERE pdf_fetch_next_at IS NOT NULL
  AND pdf_object_key IS NULL
  AND pdf_fetch_next_at <= sqlc.arg(now)
ORDER BY pdf_fetch_next_at
LIMIT sqlc.arg(row_limit);

-- name: ClaimPdfFetch :execrows
UPDATE invoice_documents
SET pdf_fetch_next_at = sqlc.arg(lease_until),
    pdf_fetch_attempts = pdf_fetch_attempts + 1, updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id)
  AND pdf_fetch_next_at IS NOT NULL
  AND pdf_object_key IS NULL
  AND pdf_fetch_next_at <= sqlc.arg(claimable_before);

-- name: MarkPdfFetchDone :execrows
UPDATE invoice_documents
SET pdf_object_key = sqlc.arg(pdf_object_key),
    pdf_fetch_next_at = NULL,
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id) AND pdf_object_key IS NULL;

-- name: MarkPdfFetchStopped :execrows
UPDATE invoice_documents
SET pdf_fetch_next_at = NULL,
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id) AND pdf_object_key IS NULL;

-- name: UpdatePdfFetchAttempt :exec
UPDATE invoice_documents
SET pdf_fetch_next_at = sqlc.arg(next_attempt_at),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id);

-- name: FetchApproachingStatutoryDeadlines :many
SELECT * FROM invoice_documents
WHERE status IN ('PENDING', 'ISSUING', 'ISSUE_FAILED', 'AWAITING_MANUAL')
  AND statutory_deadline_at IS NOT NULL
  AND statutory_deadline_at <= sqlc.arg(threshold)
ORDER BY statutory_deadline_at
LIMIT sqlc.arg(row_limit);

-- name: GetInvoiceDocumentByID :one
SELECT * FROM invoice_documents WHERE id = sqlc.arg(id);

-- name: GetInvoiceDocumentForBuyer :one
SELECT * FROM invoice_documents
WHERE source_type = 'ORDER' AND source_ref = $1
  AND recipient_type = 'BUYER' AND recipient_ref_id = $2;

-- name: GetInvoiceDocumentBySourceRef :one
SELECT * FROM invoice_documents
WHERE source_type = 'ORDER' AND source_ref = sqlc.arg(source_ref)
ORDER BY created_at DESC, id DESC
LIMIT 1;


-- name: InsertInvoiceDocument :one
INSERT INTO invoice_documents (
  id, issuer_id, issuer_name_snapshot, issuer_tax_id_snapshot,
  source_type, source_ref, trigger_type,
  recipient_type, recipient_ref_id, recipient_name, recipient_tax_id,
  recipient_email_ciphertext, recipient_email_hash, recipient_phone_ciphertext,
  carrier_type, carrier_id1_ciphertext, carrier_id2_ciphertext, carrier_value_hash, donation_code,
  document_kind, fulfillment_mode, status, not_applicable_reason,
  snapshot_version, issue_attempt_seq, provider_account_version, original_document_id,
  sales_amount, free_tax_sales_amount, zero_tax_sales_amount, tax_amount, total_amount,
  tax_type, tax_rate_bps,
  provider, provider_request_no, receipt_no, receipt_authorization_version,
  invoice_date_taipei, tax_period_code, issue_not_before, paid_at, statutory_deadline_at,
  source_snapshot, created_at, updated_at)
VALUES (
  sqlc.arg(id), sqlc.arg(issuer_id), sqlc.arg(issuer_name_snapshot), sqlc.narg(issuer_tax_id_snapshot),
  sqlc.arg(source_type), sqlc.narg(source_ref), sqlc.arg(trigger_type),
  sqlc.arg(recipient_type), sqlc.narg(recipient_ref_id), sqlc.arg(recipient_name), sqlc.arg(recipient_tax_id),
  sqlc.narg(recipient_email_ciphertext), sqlc.narg(recipient_email_hash), sqlc.narg(recipient_phone_ciphertext),
  sqlc.narg(carrier_type), sqlc.narg(carrier_id1_ciphertext), sqlc.narg(carrier_id2_ciphertext),
  sqlc.narg(carrier_value_hash), sqlc.narg(donation_code),
  sqlc.arg(document_kind), sqlc.arg(fulfillment_mode), sqlc.arg(status), sqlc.narg(not_applicable_reason),
  sqlc.arg(snapshot_version), sqlc.arg(issue_attempt_seq), sqlc.narg(provider_account_version),
  sqlc.narg(original_document_id),
  sqlc.arg(sales_amount), sqlc.arg(free_tax_sales_amount), sqlc.arg(zero_tax_sales_amount),
  sqlc.arg(tax_amount), sqlc.arg(total_amount),
  sqlc.arg(tax_type), sqlc.arg(tax_rate_bps),
  sqlc.narg(provider), sqlc.narg(provider_request_no), sqlc.narg(receipt_no),
  sqlc.narg(receipt_authorization_version),
  sqlc.narg(invoice_date_taipei), sqlc.narg(tax_period_code),
  sqlc.arg(issue_not_before), sqlc.narg(paid_at), sqlc.narg(statutory_deadline_at),
  sqlc.narg(source_snapshot), sqlc.arg(now), sqlc.arg(now))
ON CONFLICT DO NOTHING
RETURNING *;

-- name: MarkInvoiceDocumentIssuing :execrows
UPDATE invoice_documents
SET status = 'ISSUING',
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND status IN ('PENDING', 'ISSUE_FAILED');

-- name: MarkInvoiceDocumentIssued :execrows
UPDATE invoice_documents
SET status = 'ISSUED',
    manual_reason_code = NULL,
    provider = sqlc.arg(provider),
    provider_invoice_no = sqlc.arg(provider_invoice_no),
    provider_random_code = sqlc.narg(provider_random_code),
    provider_issued_at = sqlc.narg(provider_issued_at),
    invoice_date_taipei = sqlc.narg(invoice_date_taipei),
    tax_period_code = sqlc.narg(tax_period_code),
    pdf_fetch_next_at = sqlc.narg(pdf_fetch_next_at),
    upload_recheck_next_at = sqlc.narg(upload_recheck_next_at),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND status = 'ISSUING';

-- name: MarkInvoiceDocumentIssueFailed :execrows
UPDATE invoice_documents
SET status = 'ISSUE_FAILED',
    manual_reason_code = sqlc.arg(manual_reason_code),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND status = 'ISSUING';

-- name: MarkInvoiceDocumentPendingRetry :execrows
UPDATE invoice_documents
SET status = 'PENDING',
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND status = 'ISSUING';

-- name: MarkInvoiceDocumentAwaitingManual :execrows
UPDATE invoice_documents
SET status = 'AWAITING_MANUAL',
    manual_reason_code = sqlc.arg(manual_reason_code),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND status IN ('PENDING', 'ISSUE_FAILED');

-- name: ResumeInvoiceDocumentFromManual :execrows
UPDATE invoice_documents
SET status = 'PENDING',
    manual_reason_code = NULL,
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND status = 'AWAITING_MANUAL'
  AND version = sqlc.arg(expected_version);

-- name: MarkInvoiceDocumentResolvedExternally :execrows
UPDATE invoice_documents
SET status = 'RESOLVED_EXTERNALLY',
    external_invoice_no = sqlc.arg(external_invoice_no),
    external_invoice_date = sqlc.arg(external_invoice_date),
    external_reported_at = sqlc.arg(external_reported_at),
    manual_reason_code = NULL,
    not_applicable_reason = NULL,
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND (
    status IN ('ISSUE_FAILED', 'AWAITING_MANUAL')
    OR (status = 'NOT_APPLICABLE'
        AND fulfillment_mode IN ('SELLER_SELF_ISSUE', 'LEGACY_MANUAL'))
  );

-- name: MarkInvoiceDocumentCancelledBeforeIssue :execrows
UPDATE invoice_documents
SET status = 'CANCELLED_BEFORE_ISSUE',
    cancel_reason_code = sqlc.arg(cancel_reason_code),
    cancel_reason = sqlc.narg(cancel_reason),
    manual_reason_code = NULL,
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND status IN ('PENDING', 'AWAITING_MANUAL');

-- name: MarkInvoiceDocumentVoiding :execrows
UPDATE invoice_documents
SET status = 'VOIDING',
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND status = 'ISSUED';

-- name: MarkInvoiceDocumentVoided :execrows
UPDATE invoice_documents
SET status = 'VOIDED',
    voided_at = sqlc.arg(now),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND status = 'VOIDING';

-- name: RestoreInvoiceDocumentToIssued :execrows
UPDATE invoice_documents
SET status = 'ISSUED',
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND status = 'VOIDING';

-- name: MarkInvoiceDocumentSuperseded :execrows
UPDATE invoice_documents
SET status = 'SUPERSEDED',
    superseded_by_id = sqlc.arg(superseded_by_id),
    voided_at = NULL,
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND status IN ('VOIDED', 'ISSUED');

-- name: AddInvoiceDocumentAllowanceTotal :execrows
UPDATE invoice_documents
SET allowance_total_amount = allowance_total_amount + sqlc.arg(delta_amount),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND allowance_total_amount + sqlc.arg(delta_amount) <= total_amount
  AND allowance_total_amount + sqlc.arg(delta_amount) >= 0;

-- name: NextReceiptNo :one
SELECT ('R' || lpad(nextval('invoice_receipt_number_seq')::text, 12, '0'))::text AS receipt_no;


-- name: ListBuyerInvoiceDocuments :many
SELECT * FROM invoice_documents
WHERE recipient_type = 'BUYER'
  AND recipient_ref_id = sqlc.arg(buyer_id)::bigint
  AND (NOT sqlc.arg(has_date_from)::boolean
       OR COALESCE(provider_issued_at, created_at) >= sqlc.arg(date_from)::timestamptz)
  AND (NOT sqlc.arg(has_date_to)::boolean
       OR COALESCE(provider_issued_at, created_at) <= sqlc.arg(date_to)::timestamptz)
  AND (sqlc.arg(status_filter)::text = ''        OR status        = sqlc.arg(status_filter)::text)
  AND (sqlc.arg(document_kind_filter)::text = '' OR document_kind = sqlc.arg(document_kind_filter)::text)
ORDER BY COALESCE(provider_issued_at, created_at) DESC, id DESC
LIMIT sqlc.arg(row_limit) OFFSET sqlc.arg(row_offset);

-- name: CountBuyerInvoiceDocuments :one
SELECT COUNT(*)::bigint FROM invoice_documents
WHERE recipient_type = 'BUYER'
  AND recipient_ref_id = sqlc.arg(buyer_id)::bigint
  AND (NOT sqlc.arg(has_date_from)::boolean
       OR COALESCE(provider_issued_at, created_at) >= sqlc.arg(date_from)::timestamptz)
  AND (NOT sqlc.arg(has_date_to)::boolean
       OR COALESCE(provider_issued_at, created_at) <= sqlc.arg(date_to)::timestamptz)
  AND (sqlc.arg(status_filter)::text = ''        OR status        = sqlc.arg(status_filter)::text)
  AND (sqlc.arg(document_kind_filter)::text = '' OR document_kind = sqlc.arg(document_kind_filter)::text);

-- name: GetBuyerInvoiceDocumentByID :one
SELECT * FROM invoice_documents
WHERE id = sqlc.arg(id)
  AND recipient_type = 'BUYER'
  AND recipient_ref_id = sqlc.arg(buyer_id)::bigint;

-- name: ListOrderInvoiceDocumentsForBuyer :many
SELECT * FROM invoice_documents
WHERE source_type = 'ORDER'
  AND source_ref = sqlc.arg(source_ref)
  AND recipient_type = 'BUYER'
  AND recipient_ref_id = sqlc.arg(buyer_id)::bigint
ORDER BY snapshot_version DESC, issue_attempt_seq DESC, id DESC;

-- name: ListSellerInvoiceDocuments :many
SELECT sqlc.embed(d),
       COALESCE((SELECT o.id FROM invoice_operations o
                  WHERE o.document_id = d.id
                  ORDER BY o.created_at DESC, o.id DESC
                  LIMIT 1), 0)::bigint AS latest_operation_id
FROM invoice_documents d
JOIN invoice_issuers i ON i.id = d.issuer_id
WHERE i.issuer_type = 'SELLER'
  AND i.issuer_ref_id = sqlc.arg(store_id)::bigint
  AND (NOT sqlc.arg(has_date_from)::boolean
       OR COALESCE(d.provider_issued_at, d.created_at) >= sqlc.arg(date_from)::timestamptz)
  AND (NOT sqlc.arg(has_date_to)::boolean
       OR COALESCE(d.provider_issued_at, d.created_at) <= sqlc.arg(date_to)::timestamptz)
  AND (sqlc.arg(mode_filter)::text = ''   OR d.fulfillment_mode = sqlc.arg(mode_filter)::text)
  AND (sqlc.arg(status_filter)::text = '' OR d.status           = sqlc.arg(status_filter)::text)
  AND (sqlc.arg(order_no)::text = ''
       OR (d.source_type = 'ORDER' AND d.source_ref = sqlc.arg(order_no)::text))
  AND (NOT sqlc.arg(abnormal_only)::boolean
       OR d.status = ANY(sqlc.arg(abnormal_statuses)::text[]))
ORDER BY COALESCE(d.provider_issued_at, d.created_at) DESC, d.id DESC
LIMIT sqlc.arg(row_limit) OFFSET sqlc.arg(row_offset);

-- name: CountSellerInvoiceDocuments :one
SELECT COUNT(*)::bigint
FROM invoice_documents d
JOIN invoice_issuers i ON i.id = d.issuer_id
WHERE i.issuer_type = 'SELLER'
  AND i.issuer_ref_id = sqlc.arg(store_id)::bigint
  AND (NOT sqlc.arg(has_date_from)::boolean
       OR COALESCE(d.provider_issued_at, d.created_at) >= sqlc.arg(date_from)::timestamptz)
  AND (NOT sqlc.arg(has_date_to)::boolean
       OR COALESCE(d.provider_issued_at, d.created_at) <= sqlc.arg(date_to)::timestamptz)
  AND (sqlc.arg(mode_filter)::text = ''   OR d.fulfillment_mode = sqlc.arg(mode_filter)::text)
  AND (sqlc.arg(status_filter)::text = '' OR d.status           = sqlc.arg(status_filter)::text)
  AND (sqlc.arg(order_no)::text = ''
       OR (d.source_type = 'ORDER' AND d.source_ref = sqlc.arg(order_no)::text))
  AND (NOT sqlc.arg(abnormal_only)::boolean
       OR d.status = ANY(sqlc.arg(abnormal_statuses)::text[]));

-- name: GetSellerInvoiceDocumentByID :one
SELECT sqlc.embed(d),
       COALESCE((SELECT o.id FROM invoice_operations o
                  WHERE o.document_id = d.id
                  ORDER BY o.created_at DESC, o.id DESC
                  LIMIT 1), 0)::bigint AS latest_operation_id
FROM invoice_documents d
JOIN invoice_issuers i ON i.id = d.issuer_id
WHERE d.id = sqlc.arg(id)
  AND i.issuer_type = 'SELLER'
  AND i.issuer_ref_id = sqlc.arg(store_id)::bigint;

-- name: ListRecentSellerInvoiceDocuments :many
SELECT * FROM invoice_documents d
WHERE d.issuer_id = sqlc.arg(issuer_id)
ORDER BY COALESCE(d.provider_issued_at, d.created_at) DESC, d.id DESC
LIMIT sqlc.arg(row_limit);

-- name: ListSellerReceivedInvoiceDocuments :many
SELECT sqlc.embed(d),
       COALESCE((SELECT l.description FROM invoice_document_lines l
                  WHERE l.document_id = d.id
                  ORDER BY l.line_seq
                  LIMIT 1), '')::text AS first_line_description,
       (SELECT COUNT(*)::bigint FROM invoice_document_lines l
         WHERE l.document_id = d.id) AS line_count
FROM invoice_documents d
WHERE d.recipient_type = 'SELLER'
  AND d.recipient_ref_id = sqlc.arg(store_id)::bigint
  AND (NOT sqlc.arg(has_date_from)::boolean
       OR COALESCE(d.provider_issued_at, d.created_at) >= sqlc.arg(date_from)::timestamptz)
  AND (NOT sqlc.arg(has_date_to)::boolean
       OR COALESCE(d.provider_issued_at, d.created_at) <= sqlc.arg(date_to)::timestamptz)
  AND (sqlc.arg(status_filter)::text = '' OR d.status = sqlc.arg(status_filter)::text)
ORDER BY COALESCE(d.provider_issued_at, d.created_at) DESC, d.id DESC
LIMIT sqlc.arg(row_limit) OFFSET sqlc.arg(row_offset);

-- name: CountSellerReceivedInvoiceDocuments :one
SELECT COUNT(*)::bigint FROM invoice_documents d
WHERE d.recipient_type = 'SELLER'
  AND d.recipient_ref_id = sqlc.arg(store_id)::bigint
  AND (NOT sqlc.arg(has_date_from)::boolean
       OR COALESCE(d.provider_issued_at, d.created_at) >= sqlc.arg(date_from)::timestamptz)
  AND (NOT sqlc.arg(has_date_to)::boolean
       OR COALESCE(d.provider_issued_at, d.created_at) <= sqlc.arg(date_to)::timestamptz)
  AND (sqlc.arg(status_filter)::text = '' OR d.status = sqlc.arg(status_filter)::text);

-- name: ListInvoiceDocumentsForAdmin :many
SELECT sqlc.embed(d),
       i.issuer_ref_id AS issuer_ref_id,
       i.issuer_type   AS issuer_type,
       COALESCE((SELECT o.id FROM invoice_operations o
                  WHERE o.document_id = d.id
                  ORDER BY o.created_at DESC, o.id DESC
                  LIMIT 1), 0)::bigint AS latest_operation_id
FROM invoice_documents d
JOIN invoice_issuers i ON i.id = d.issuer_id
WHERE (NOT sqlc.arg(has_store_id)::boolean
       OR (i.issuer_type = 'SELLER' AND i.issuer_ref_id = sqlc.arg(store_id)::bigint))
  AND (sqlc.arg(order_no)::text = ''
       OR (d.source_type = 'ORDER' AND d.source_ref = sqlc.arg(order_no)::text))
  AND (sqlc.arg(status_filter)::text = '' OR d.status           = sqlc.arg(status_filter)::text)
  AND (sqlc.arg(mode_filter)::text = ''   OR d.fulfillment_mode = sqlc.arg(mode_filter)::text)
  AND (NOT sqlc.arg(has_date_from)::boolean
       OR COALESCE(d.provider_issued_at, d.created_at) >= sqlc.arg(date_from)::timestamptz)
  AND (NOT sqlc.arg(has_date_to)::boolean
       OR COALESCE(d.provider_issued_at, d.created_at) <= sqlc.arg(date_to)::timestamptz)
ORDER BY COALESCE(d.provider_issued_at, d.created_at) DESC, d.id DESC
LIMIT sqlc.arg(row_limit) OFFSET sqlc.arg(row_offset);

-- name: CountInvoiceDocumentsForAdmin :one
SELECT COUNT(*)::bigint
FROM invoice_documents d
JOIN invoice_issuers i ON i.id = d.issuer_id
WHERE (NOT sqlc.arg(has_store_id)::boolean
       OR (i.issuer_type = 'SELLER' AND i.issuer_ref_id = sqlc.arg(store_id)::bigint))
  AND (sqlc.arg(order_no)::text = ''
       OR (d.source_type = 'ORDER' AND d.source_ref = sqlc.arg(order_no)::text))
  AND (sqlc.arg(status_filter)::text = '' OR d.status           = sqlc.arg(status_filter)::text)
  AND (sqlc.arg(mode_filter)::text = ''   OR d.fulfillment_mode = sqlc.arg(mode_filter)::text)
  AND (NOT sqlc.arg(has_date_from)::boolean
       OR COALESCE(d.provider_issued_at, d.created_at) >= sqlc.arg(date_from)::timestamptz)
  AND (NOT sqlc.arg(has_date_to)::boolean
       OR COALESCE(d.provider_issued_at, d.created_at) <= sqlc.arg(date_to)::timestamptz);

-- name: GetInvoiceDocumentByProviderRequestNo :one
SELECT * FROM invoice_documents
WHERE provider_request_no = sqlc.arg(provider_request_no);

-- name: ClaimInvoiceNotificationResend :one
UPDATE invoice_documents
SET notification_resent_at = sqlc.arg(now),
    notification_resend_count = notification_resend_count + 1,
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id)
  AND (notification_resent_at IS NULL OR notification_resent_at <= sqlc.arg(window_start))
RETURNING notification_resend_count;

-- name: ReviseInvoiceDocumentSnapshot :execrows
UPDATE invoice_documents
SET snapshot_version           = sqlc.arg(snapshot_version),
    document_kind              = sqlc.arg(document_kind),
    sales_amount               = sqlc.arg(sales_amount),
    free_tax_sales_amount      = sqlc.arg(free_tax_sales_amount),
    zero_tax_sales_amount      = sqlc.arg(zero_tax_sales_amount),
    tax_amount                 = sqlc.arg(tax_amount),
    total_amount               = sqlc.arg(total_amount),
    statutory_deadline_at      = sqlc.narg(statutory_deadline_at),
    recipient_type             = sqlc.arg(recipient_type),
    recipient_ref_id           = sqlc.narg(recipient_ref_id),
    recipient_name             = sqlc.arg(recipient_name),
    recipient_tax_id           = sqlc.arg(recipient_tax_id),
    recipient_email_ciphertext = sqlc.narg(recipient_email_ciphertext),
    recipient_email_hash       = sqlc.narg(recipient_email_hash),
    carrier_type               = sqlc.narg(carrier_type),
    carrier_id1_ciphertext     = sqlc.narg(carrier_id1_ciphertext),
    carrier_id2_ciphertext     = sqlc.narg(carrier_id2_ciphertext),
    carrier_value_hash         = sqlc.narg(carrier_value_hash),
    donation_code              = sqlc.narg(donation_code),
    updated_at                 = sqlc.arg(now),
    version                    = version + 1
WHERE id = sqlc.arg(id)
  AND version = sqlc.arg(expected_version)
  AND status IN ('PENDING', 'AWAITING_MANUAL');

-- name: LockInvoiceDocumentForCorrection :one
SELECT * FROM invoice_documents
WHERE id = sqlc.arg(id)
FOR UPDATE;

-- name: MarkInvoiceDocumentReceiptIssued :execrows
UPDATE invoice_documents
SET status = 'ISSUED',
    receipt_authorization_version = sqlc.arg(receipt_authorization_version),
    pdf_object_key = sqlc.arg(pdf_object_key),
    provider_issued_at = sqlc.arg(provider_issued_at),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND status = 'ISSUING'
  AND document_kind = 'RECEIPT';
