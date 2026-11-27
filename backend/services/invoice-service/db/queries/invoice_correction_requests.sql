
-- name: GetInvoiceCorrectionRequestByID :one
SELECT * FROM invoice_correction_requests WHERE id = sqlc.arg(id);

-- name: GetOpenCorrectionRequestByDocument :one
SELECT * FROM invoice_correction_requests
WHERE document_id = sqlc.arg(document_id)
  AND status IN ('REQUESTED', 'VOID_REISSUE_IN_PROGRESS');


-- name: InsertInvoiceCorrectionRequest :one
INSERT INTO invoice_correction_requests (
  id, document_id, requested_by_buyer_id, expected_snapshot_version,
  before_payload, after_payload, status, created_at, updated_at)
VALUES (
  sqlc.arg(id), sqlc.arg(document_id), sqlc.arg(requested_by_buyer_id),
  sqlc.arg(expected_snapshot_version), sqlc.arg(before_payload), sqlc.arg(after_payload),
  'REQUESTED', sqlc.arg(now), sqlc.arg(now))
ON CONFLICT DO NOTHING
RETURNING *;

-- name: RejectInvoiceCorrectionRequest :execrows
UPDATE invoice_correction_requests
SET status = 'REJECTED',
    reject_reason_code = sqlc.arg(reject_reason_code),
    reject_reason_detail = sqlc.narg(reject_reason_detail),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND status IN ('REQUESTED', 'VOID_REISSUE_IN_PROGRESS');

-- name: MarkInvoiceCorrectionUpdatedBeforeIssue :execrows
UPDATE invoice_correction_requests
SET status = 'UPDATED_BEFORE_ISSUE',
    handling_mode = 'UPDATE_SNAPSHOT',
    completed_at = sqlc.arg(now),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND status = 'REQUESTED';

-- name: StartInvoiceCorrectionVoidReissue :execrows
UPDATE invoice_correction_requests
SET status = 'VOID_REISSUE_IN_PROGRESS',
    handling_mode = 'VOID_AND_REISSUE',
    void_adjustment_id = sqlc.arg(void_adjustment_id),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND status = 'REQUESTED';

-- name: MarkInvoiceCorrectionReissueFailed :execrows
UPDATE invoice_correction_requests
SET status = 'REISSUE_FAILED',
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND status = 'VOID_REISSUE_IN_PROGRESS';

-- name: RetryInvoiceCorrectionReissue :execrows
UPDATE invoice_correction_requests
SET status = 'VOID_REISSUE_IN_PROGRESS',
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND status = 'REISSUE_FAILED'
  AND version = sqlc.arg(expected_version);

-- name: GetReissueFailedCorrectionByDocument :one
SELECT * FROM invoice_correction_requests
WHERE document_id = sqlc.arg(document_id)
  AND status = 'REISSUE_FAILED'
ORDER BY id DESC
LIMIT 1;

-- name: CompleteInvoiceCorrectionRequest :execrows
UPDATE invoice_correction_requests
SET status = 'COMPLETED',
    reissued_document_id = sqlc.arg(reissued_document_id),
    completed_at = sqlc.arg(now),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND status IN ('VOID_REISSUE_IN_PROGRESS', 'REISSUE_FAILED');

-- name: GetInvoiceCorrectionRequestForBuyer :one
SELECT r.* FROM invoice_correction_requests r
JOIN invoice_documents d ON d.id = r.document_id
WHERE r.id = sqlc.arg(id)
  AND r.requested_by_buyer_id = sqlc.arg(buyer_id)::bigint
  AND d.source_type = 'ORDER'
  AND d.source_ref = sqlc.arg(source_ref)::text
  AND d.recipient_type = 'BUYER'
  AND d.recipient_ref_id = sqlc.arg(buyer_id)::bigint;

-- name: GetLatestCorrectionRequestByOrder :one
SELECT r.* FROM invoice_correction_requests r
JOIN invoice_documents d ON d.id = r.document_id
WHERE d.source_type = 'ORDER'
  AND d.source_ref = sqlc.arg(source_ref)::text
  AND r.requested_by_buyer_id = sqlc.arg(buyer_id)::bigint
ORDER BY r.id DESC
LIMIT 1;
