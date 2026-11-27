
-- name: GetSellerInvoiceOverviewCounts :one
SELECT
  (SELECT COUNT(*)::bigint FROM invoice_documents d
    WHERE d.issuer_id = sqlc.arg(issuer_id)
      AND d.status = ANY(sqlc.arg(pending_document_statuses)::text[])) AS pending_document_count,
  (SELECT COUNT(*)::bigint FROM invoice_complaints c
    WHERE c.issuer_id = sqlc.arg(issuer_id)
      AND c.status = ANY(sqlc.arg(open_complaint_statuses)::text[]))   AS open_complaint_count,
  (SELECT COUNT(*)::bigint FROM invoice_complaints c
    WHERE c.issuer_id = sqlc.arg(issuer_id)
      AND (c.status = ANY(sqlc.arg(overdue_complaint_statuses)::text[])
           OR (c.status = ANY(sqlc.arg(open_complaint_statuses)::text[])
               AND c.sla_due_at <= sqlc.arg(now)::timestamptz)))       AS overdue_complaint_count;

-- name: GetInvoiceOverviewCountsForAdmin :one
SELECT
  (SELECT COUNT(*)::bigint FROM seller_tax_profiles p
    WHERE p.review_status = ANY(sqlc.arg(tax_review_queue_statuses)::text[]))
      AS pending_tax_review_count,
  (SELECT COUNT(*)::bigint FROM seller_tax_profiles p
    WHERE p.review_status = ANY(sqlc.arg(tax_review_queue_statuses)::text[])
      AND p.pending_valid_until IS NOT NULL
      AND p.pending_valid_until <= sqlc.arg(near_deadline_before)::timestamptz)
      AS near_deadline_count,
  (SELECT COUNT(*)::bigint FROM invoice_provider_accounts a
    WHERE a.status = ANY(sqlc.arg(provider_error_statuses)::text[]))
      AS provider_error_count,
  (SELECT COUNT(*)::bigint FROM invoice_operations o
    WHERE o.status = ANY(sqlc.arg(pending_job_statuses)::text[]))
      AS pending_job_count,
  (SELECT COUNT(*)::bigint FROM invoice_operations o
     JOIN invoice_documents d ON d.id = o.document_id
    WHERE (o.status = ANY(sqlc.arg(needs_manual_job_statuses)::text[])
        OR d.status = ANY(sqlc.arg(manual_document_statuses)::text[]))
      AND NOT (d.status = ANY(COALESCE(sqlc.arg(resolved_document_statuses)::text[], ARRAY[]::text[]))))
      AS needs_manual_count,
  (SELECT COUNT(*)::bigint FROM invoice_complaints c
    WHERE c.status = ANY(sqlc.arg(open_complaint_statuses)::text[]))
      AS open_complaint_count,
  (SELECT COUNT(*)::bigint FROM invoice_complaints c
    WHERE c.status = ANY(sqlc.arg(overdue_complaint_statuses)::text[])
       OR (c.status = ANY(sqlc.arg(open_complaint_statuses)::text[])
           AND c.sla_due_at <= sqlc.arg(now)::timestamptz))
      AS overdue_complaint_count;
