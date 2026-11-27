
-- name: GetInvoiceAdjustmentByID :one
SELECT * FROM invoice_adjustments WHERE id = sqlc.arg(id);

-- name: ListInvoiceAdjustmentsByDocument :many
SELECT * FROM invoice_adjustments
WHERE document_id = sqlc.arg(document_id)
ORDER BY created_at;

-- name: ListStaleBlockedAdjustments :many
SELECT * FROM invoice_adjustments
WHERE status IN ('BLOCKED_UNKNOWN_DOCUMENT', 'BLOCKED_PENDING_CORRECTION')
  AND updated_at <= sqlc.arg(stale_before)
ORDER BY updated_at
LIMIT sqlc.arg(row_limit);


-- name: InsertInvoiceAdjustment :one
INSERT INTO invoice_adjustments (
  id, document_id, adjustment_type, status, trigger_kind,
  refund_no, correction_request_id,
  allowance_number, allowance_date_taipei, net_amount_ex_tax, tax_amount, gross_amount,
  void_reason_code, void_reason_detail, created_at, updated_at)
VALUES (
  sqlc.arg(id), sqlc.arg(document_id), sqlc.arg(adjustment_type), sqlc.arg(status),
  sqlc.arg(trigger_kind), sqlc.narg(refund_no), sqlc.narg(correction_request_id),
  sqlc.narg(allowance_number), sqlc.narg(allowance_date_taipei),
  sqlc.narg(net_amount_ex_tax), sqlc.narg(tax_amount), sqlc.narg(gross_amount),
  sqlc.narg(void_reason_code), sqlc.narg(void_reason_detail),
  sqlc.arg(now), sqlc.arg(now))
ON CONFLICT DO NOTHING
RETURNING *;

-- name: UnblockInvoiceAdjustmentAfterIssue :execrows
UPDATE invoice_adjustments
SET status = 'PENDING',
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND status = 'BLOCKED_UNKNOWN_DOCUMENT';

-- name: RebindInvoiceAdjustmentAfterCorrection :execrows
UPDATE invoice_adjustments
SET status = 'PENDING',
    document_id = sqlc.arg(document_id),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND status = 'BLOCKED_PENDING_CORRECTION';

-- name: CancelBlockedInvoiceAdjustment :execrows
UPDATE invoice_adjustments
SET status = 'CANCELLED',
    cancel_reason_code = sqlc.arg(cancel_reason_code),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND status IN ('BLOCKED_UNKNOWN_DOCUMENT', 'BLOCKED_PENDING_CORRECTION');

-- name: MarkInvoiceAdjustmentSending :execrows
UPDATE invoice_adjustments
SET status = 'SENDING',
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND status IN ('PENDING', 'SENDING');

-- name: MarkInvoiceAdjustmentCompleted :execrows
UPDATE invoice_adjustments
SET status = 'COMPLETED',
    provider_confirmed_at = sqlc.arg(now),
    provider_upload_status = sqlc.narg(provider_upload_status),
    provider_upload_checked_at = sqlc.narg(provider_upload_checked_at),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND status IN ('SENDING', 'WAITING_CONFIRMATION', 'FAILED_UNKNOWN');

-- name: MarkInvoiceAdjustmentFailedPermanent :execrows
UPDATE invoice_adjustments
SET status = 'FAILED_PERMANENT',
    blocked_reason_code = sqlc.arg(blocked_reason_code),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND status = 'SENDING';

-- name: RescheduleInvoiceAdjustment :execrows
UPDATE invoice_adjustments
SET status = 'PENDING',
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND status IN ('SENDING', 'WAITING_CONFIRMATION');

-- name: MarkInvoiceAdjustmentWaitingConfirmation :execrows
UPDATE invoice_adjustments
SET status = 'WAITING_CONFIRMATION',
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND status = 'SENDING';

-- name: MarkInvoiceAdjustmentFailedUnknown :execrows
UPDATE invoice_adjustments
SET status = 'FAILED_UNKNOWN',
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND status = 'WAITING_CONFIRMATION';

-- name: AdminRetryInvoiceAdjustment :execrows
UPDATE invoice_adjustments
SET status = 'PENDING',
    blocked_reason_code = NULL,
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND status = 'FAILED_PERMANENT'
  AND version = sqlc.arg(expected_version);

-- name: CancelUnknownInvoiceAdjustment :execrows
UPDATE invoice_adjustments
SET status = 'CANCELLED',
    cancel_reason_code = sqlc.arg(cancel_reason_code),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND status = 'FAILED_UNKNOWN'
  AND version = sqlc.arg(expected_version);

-- name: AdminCancelInvoiceAdjustment :execrows
UPDATE invoice_adjustments
SET status = 'CANCELLED',
    cancel_reason_code = sqlc.arg(cancel_reason_code),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND status IN ('PENDING', 'BLOCKED_UNKNOWN_DOCUMENT', 'BLOCKED_PENDING_CORRECTION')
  AND version = sqlc.arg(expected_version);

-- name: UpdateInvoiceAdjustmentPdfObjectKey :execrows
UPDATE invoice_adjustments
SET pdf_object_key = sqlc.arg(pdf_object_key),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND pdf_object_key IS NULL;

-- name: GetInvoiceAdjustmentByRefund :one
SELECT * FROM invoice_adjustments
WHERE document_id = sqlc.arg(document_id)
  AND refund_no = sqlc.arg(refund_no)
  AND adjustment_type = 'ALLOWANCE'
  AND status IN ('PENDING','BLOCKED_UNKNOWN_DOCUMENT','BLOCKED_PENDING_CORRECTION','SENDING',
                 'WAITING_CONFIRMATION','COMPLETED','FAILED_PERMANENT','FAILED_UNKNOWN');

-- name: NextAllowanceNumber :one
SELECT ('A' || lpad(nextval('invoice_allowance_number_seq')::text, 12, '0'))::text AS allowance_number;

-- name: CancelUnsentAdjustmentsForIssuer :execrows
UPDATE invoice_adjustments a
SET status = 'CANCELLED',
    cancel_reason_code = sqlc.arg(cancel_reason_code),
    updated_at = sqlc.arg(now),
    version = a.version + 1
FROM invoice_documents d
WHERE d.id = a.document_id
  AND d.issuer_id = sqlc.arg(issuer_id)
  AND a.status IN ('PENDING','SENDING','BLOCKED_UNKNOWN_DOCUMENT','BLOCKED_PENDING_CORRECTION');
