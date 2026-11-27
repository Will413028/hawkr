
-- name: MarkOverdueComplaints :many
UPDATE invoice_complaints
SET status = 'OVERDUE', overdue_at = sqlc.arg(now), updated_at = sqlc.arg(now), version = version + 1
WHERE id IN (
  SELECT id FROM invoice_complaints
  WHERE status = 'OPEN' AND sla_due_at <= sqlc.arg(now)
  ORDER BY sla_due_at LIMIT sqlc.arg(row_limit))
  AND status = 'OPEN' AND sla_due_at <= sqlc.arg(now)
RETURNING *;

-- name: EscalateOverdueComplaints :many
UPDATE invoice_complaints
SET status = 'ESCALATED_TO_ADM', updated_at = sqlc.arg(now), version = version + 1
WHERE id IN (
  SELECT sub.id FROM invoice_complaints AS sub
  WHERE sub.status = 'OVERDUE' AND sub.overdue_at <= sqlc.arg(escalate_before)
  ORDER BY sub.overdue_at LIMIT sqlc.arg(row_limit))
  AND status = 'OVERDUE' AND overdue_at <= sqlc.arg(escalate_before)
RETURNING *;

-- name: ResolveExpiredBuyerReviews :many
UPDATE invoice_complaints
SET status = 'RESOLVED',
    resolved_at = sqlc.arg(now),
    reopen_allowed_after = sqlc.arg(reopen_allowed_after),
    resolution_code = sqlc.narg(resolution_code),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id IN (
  SELECT id FROM invoice_complaints
  WHERE status = 'SELLER_RESPONDED' AND buyer_review_due_at <= sqlc.arg(now)
  ORDER BY buyer_review_due_at LIMIT sqlc.arg(row_limit))
  AND status = 'SELLER_RESPONDED' AND buyer_review_due_at <= sqlc.arg(now)
RETURNING *;

-- name: CountConfirmedViolationOrders :one
SELECT COUNT(DISTINCT order_no)::bigint
FROM invoice_complaints
WHERE issuer_id = sqlc.arg(issuer_id)
  AND status = 'VIOLATION_CONFIRMED'
  AND resolved_at >= sqlc.arg(window_start)
  AND order_no IS NOT NULL;

-- name: GetInvoiceComplaintByID :one
SELECT * FROM invoice_complaints WHERE id = sqlc.arg(id);


-- name: InsertInvoiceComplaint :one
INSERT INTO invoice_complaints (
  id, document_id, issuer_id, order_no, complainant_buyer_id,
  reason_code, description, status, sla_due_at, created_at, updated_at)
VALUES (
  sqlc.arg(id), sqlc.arg(document_id), sqlc.arg(issuer_id), sqlc.narg(order_no),
  sqlc.arg(complainant_buyer_id), sqlc.arg(reason_code), sqlc.narg(description),
  'OPEN', sqlc.arg(sla_due_at), sqlc.arg(now), sqlc.arg(now))
ON CONFLICT DO NOTHING
RETURNING *;

-- name: CountWithdrawnComplaintsByDocument :one
SELECT COUNT(*)::bigint FROM invoice_complaints
WHERE document_id = sqlc.arg(document_id)
  AND status = 'WITHDRAWN';

-- name: MarkInvoiceComplaintSellerResponded :execrows
UPDATE invoice_complaints
SET status = 'SELLER_RESPONDED',
    buyer_review_due_at = sqlc.arg(buyer_review_due_at),
    first_responded_at = COALESCE(first_responded_at, sqlc.arg(now)),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE invoice_complaints.id = sqlc.arg(id)
  AND invoice_complaints.status IN ('OPEN', 'OVERDUE')
  AND invoice_complaints.version = sqlc.arg(expected_version)
  AND invoice_complaints.issuer_id = (
        SELECT i.id FROM invoice_issuers i
         WHERE i.issuer_type = 'SELLER' AND i.issuer_ref_id = sqlc.arg(store_id)::bigint);

-- name: EscalateInvoiceComplaintBySeller :execrows
UPDATE invoice_complaints
SET status = 'ESCALATED_TO_ADM',
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE invoice_complaints.id = sqlc.arg(id)
  AND invoice_complaints.status = 'OPEN'
  AND invoice_complaints.version = sqlc.arg(expected_version)
  AND invoice_complaints.issuer_id = (
        SELECT i.id FROM invoice_issuers i
         WHERE i.issuer_type = 'SELLER' AND i.issuer_ref_id = sqlc.arg(store_id)::bigint);

-- name: EscalateInvoiceComplaintByBuyer :execrows
UPDATE invoice_complaints
SET status = 'ESCALATED_TO_ADM',
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND status = 'SELLER_RESPONDED'
  AND complainant_buyer_id = sqlc.arg(complainant_buyer_id)
  AND version = sqlc.arg(expected_version);

-- name: DecideEscalatedInvoiceComplaint :execrows
UPDATE invoice_complaints
SET status = sqlc.arg(next_status),
    resolved_at = sqlc.arg(now),
    resolution_code = sqlc.narg(resolution_code),
    resolution_note = sqlc.narg(resolution_note),
    reopen_allowed_after = sqlc.narg(reopen_allowed_after),
    decided_by_account_id = sqlc.arg(decided_by_account_id),
    decided_at = sqlc.arg(now),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND status = 'ESCALATED_TO_ADM'
  AND version = sqlc.arg(expected_version);

-- name: ReturnInvoiceComplaintToSeller :execrows
UPDATE invoice_complaints
SET status = 'OPEN',
    sla_due_at = sqlc.arg(sla_due_at),
    overdue_at = NULL,
    buyer_review_due_at = NULL,
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND status = 'ESCALATED_TO_ADM'
  AND version = sqlc.arg(expected_version);

-- name: WithdrawInvoiceComplaint :execrows
UPDATE invoice_complaints
SET status = 'WITHDRAWN',
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND status IN ('OPEN', 'OVERDUE', 'SELLER_RESPONDED')
  AND complainant_buyer_id = sqlc.arg(complainant_buyer_id)
  AND version = sqlc.arg(expected_version);

-- name: AutoResolveInvoiceComplaintOnIssued :execrows
UPDATE invoice_complaints
SET status = 'RESOLVED',
    resolved_at = sqlc.arg(now),
    reopen_allowed_after = sqlc.arg(reopen_allowed_after),
    resolution_code = sqlc.arg(resolution_code),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND status = 'OPEN';

-- name: ReopenInvoiceComplaint :execrows
UPDATE invoice_complaints
SET status = 'OPEN',
    resolved_at = NULL,
    resolution_code = NULL,
    reopen_count = reopen_count + 1,
    reopen_allowed_after = NULL,
    sla_due_at = sqlc.arg(sla_due_at),
    buyer_review_due_at = NULL,
    overdue_at = NULL,
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND status = 'RESOLVED'
  AND complainant_buyer_id = sqlc.arg(complainant_buyer_id)
  AND version = sqlc.arg(expected_version)
  AND reopen_count < 2
  AND reopen_allowed_after IS NOT NULL
  AND reopen_allowed_after <= sqlc.arg(now);

-- name: GetInvoiceComplaintForBuyer :one
SELECT * FROM invoice_complaints
WHERE id = sqlc.arg(id)
  AND complainant_buyer_id = sqlc.arg(complainant_buyer_id);


-- name: ListBuyerOrderInvoiceComplaints :many
SELECT * FROM invoice_complaints
WHERE complainant_buyer_id = sqlc.arg(complainant_buyer_id)::bigint
  AND order_no = sqlc.arg(order_no)::text
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(row_limit) OFFSET sqlc.arg(row_offset);

-- name: CountBuyerOrderInvoiceComplaints :one
SELECT COUNT(*)::bigint FROM invoice_complaints
WHERE complainant_buyer_id = sqlc.arg(complainant_buyer_id)::bigint
  AND order_no = sqlc.arg(order_no)::text;

-- name: ListSellerInvoiceComplaints :many
SELECT * FROM invoice_complaints c
WHERE c.issuer_id = (
        SELECT i.id FROM invoice_issuers i
         WHERE i.issuer_type = 'SELLER' AND i.issuer_ref_id = sqlc.arg(store_id)::bigint)
  AND (COALESCE(cardinality(sqlc.arg(status_filters)::text[]), 0) = 0
       OR c.status = ANY(sqlc.arg(status_filters)::text[]))
ORDER BY (CASE WHEN sqlc.arg(sort_by_due)::boolean THEN c.sla_due_at END) ASC NULLS LAST,
         (CASE WHEN sqlc.arg(sort_by_due)::boolean THEN NULL::timestamptz ELSE c.created_at END) DESC,
         c.id DESC
LIMIT sqlc.arg(row_limit) OFFSET sqlc.arg(row_offset);

-- name: CountSellerInvoiceComplaints :one
SELECT COUNT(*)::bigint FROM invoice_complaints c
WHERE c.issuer_id = (
        SELECT i.id FROM invoice_issuers i
         WHERE i.issuer_type = 'SELLER' AND i.issuer_ref_id = sqlc.arg(store_id)::bigint)
  AND (COALESCE(cardinality(sqlc.arg(status_filters)::text[]), 0) = 0
       OR c.status = ANY(sqlc.arg(status_filters)::text[]));

-- name: GetSellerInvoiceComplaintByID :one
SELECT c.* FROM invoice_complaints c
JOIN invoice_issuers i ON i.id = c.issuer_id
WHERE c.id = sqlc.arg(id)
  AND i.issuer_type = 'SELLER'
  AND i.issuer_ref_id = sqlc.arg(store_id)::bigint;

-- name: ListInvoiceComplaintsForAdmin :many
SELECT sqlc.embed(c),
       i.issuer_ref_id AS issuer_ref_id,
       (SELECT COUNT(DISTINCT v.order_no)::bigint FROM invoice_complaints v
         WHERE v.issuer_id = c.issuer_id
           AND v.status = sqlc.arg(violation_status)::text
           AND v.resolved_at >= sqlc.arg(violation_window_start)::timestamptz
           AND v.order_no IS NOT NULL) AS violation_count_in_window
FROM invoice_complaints c
JOIN invoice_issuers i ON i.id = c.issuer_id
WHERE (
        (COALESCE(cardinality(sqlc.arg(status_filters)::text[]), 0) = 0
         OR c.status = ANY(sqlc.arg(status_filters)::text[]))
        OR (sqlc.arg(include_due_overdue)::boolean
            AND c.status = ANY(sqlc.arg(due_overdue_statuses)::text[])
            AND c.sla_due_at <= sqlc.arg(now)::timestamptz)
      )
  AND (NOT sqlc.arg(has_store_id)::boolean
       OR (i.issuer_type = 'SELLER' AND i.issuer_ref_id = sqlc.arg(store_id)::bigint))
  AND (sqlc.arg(order_no)::text = '' OR c.order_no = sqlc.arg(order_no)::text)
ORDER BY (CASE WHEN sqlc.arg(sort_by_due)::boolean THEN c.sla_due_at END) ASC NULLS LAST,
         (CASE WHEN sqlc.arg(sort_by_due)::boolean THEN NULL::timestamptz ELSE c.created_at END) DESC,
         c.id DESC
LIMIT sqlc.arg(row_limit) OFFSET sqlc.arg(row_offset);

-- name: CountInvoiceComplaintsForAdmin :one
SELECT COUNT(*)::bigint
FROM invoice_complaints c
JOIN invoice_issuers i ON i.id = c.issuer_id
WHERE (
        (COALESCE(cardinality(sqlc.arg(status_filters)::text[]), 0) = 0
         OR c.status = ANY(sqlc.arg(status_filters)::text[]))
        OR (sqlc.arg(include_due_overdue)::boolean
            AND c.status = ANY(sqlc.arg(due_overdue_statuses)::text[])
            AND c.sla_due_at <= sqlc.arg(now)::timestamptz)
      )
  AND (NOT sqlc.arg(has_store_id)::boolean
       OR (i.issuer_type = 'SELLER' AND i.issuer_ref_id = sqlc.arg(store_id)::bigint))
  AND (sqlc.arg(order_no)::text = '' OR c.order_no = sqlc.arg(order_no)::text);
