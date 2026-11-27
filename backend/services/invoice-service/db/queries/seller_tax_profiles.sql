
-- name: ExpirePendingTaxProfiles :many
UPDATE seller_tax_profiles
SET review_status = 'EXPIRED',
    cooldown_until = sqlc.arg(cooldown_until),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id IN (
  SELECT id FROM seller_tax_profiles
  WHERE review_status IN ('SELF_ISSUE_PENDING', 'SUPPLEMENT_REQUIRED')
    AND pending_valid_until <= sqlc.arg(now)
  ORDER BY pending_valid_until LIMIT sqlc.arg(row_limit))
  AND review_status IN ('SELF_ISSUE_PENDING', 'SUPPLEMENT_REQUIRED')
  AND pending_valid_until <= sqlc.arg(now)
RETURNING *;

-- name: CountApprovalRecheckDue :one
SELECT COUNT(*) FROM seller_tax_profiles
WHERE review_status IN ('APPROVED_INVOICE_REQUIRED', 'APPROVED_INVOICE_EXEMPT')
  AND approval_valid_until IS NOT NULL
  AND approval_valid_until <= sqlc.arg(now);

-- name: GetSellerTaxProfileByIssuer :one
SELECT * FROM seller_tax_profiles WHERE issuer_id = sqlc.arg(issuer_id);

-- name: GetSellerTaxProfileByID :one
SELECT * FROM seller_tax_profiles WHERE id = sqlc.arg(id);


-- name: InsertSellerTaxProfile :one
INSERT INTO seller_tax_profiles (id, issuer_id, review_status, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(issuer_id), 'DRAFT', sqlc.arg(now), sqlc.arg(now))
ON CONFLICT DO NOTHING
RETURNING *;

-- name: ResetSellerTaxProfileToDraft :execrows
UPDATE seller_tax_profiles
SET review_status = 'DRAFT',
    submitted_at = NULL,
    pending_valid_until = NULL,
    extension_count = 0,
    cooldown_until = NULL,
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND review_status IN ('REJECTED', 'EXPIRED')
  AND (cooldown_until IS NULL OR cooldown_until <= sqlc.arg(now));

-- name: UpdateSellerTaxProfileDraft :execrows
UPDATE seller_tax_profiles
SET claimed_obligation = sqlc.arg(claimed_obligation),
    entity_tax_type = sqlc.narg(entity_tax_type),
    legal_name = sqlc.narg(legal_name),
    tax_id = sqlc.narg(tax_id),
    responsible_person = sqlc.narg(responsible_person),
    business_address = sqlc.narg(business_address),
    address_city = sqlc.narg(address_city),
    address_district = sqlc.narg(address_district),
    default_tax_category = sqlc.arg(default_tax_category),
    evidence_object_keys = sqlc.arg(evidence_object_keys),
    review_note = sqlc.narg(review_note),
    review_note_at = CASE WHEN review_note IS DISTINCT FROM sqlc.narg(review_note)
                          THEN sqlc.arg(now) ELSE review_note_at END,
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND review_status = 'DRAFT'
  AND version = sqlc.arg(expected_version);

-- name: SubmitSellerTaxProfile :execrows
UPDATE seller_tax_profiles
SET review_status = 'SUBMITTED',
    submitted_at = sqlc.arg(now),
    pending_valid_until = sqlc.arg(pending_valid_until),
    attestation_version = sqlc.narg(attestation_version),
    attested_at = sqlc.arg(now),
    attested_by_account_id = sqlc.narg(attested_by_account_id),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND review_status = 'DRAFT'
  AND version = sqlc.arg(expected_version);

-- name: AcceptSellerTaxProfileReview :execrows
UPDATE seller_tax_profiles
SET review_status = 'UNDER_REVIEW',
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND review_status = 'SUBMITTED'
  AND version = sqlc.arg(expected_version);

-- name: AttestSellerSelfIssue :execrows
UPDATE seller_tax_profiles
SET review_status = 'SELF_ISSUE_PENDING',
    attestation_version = sqlc.arg(attestation_version),
    attested_at = sqlc.arg(now),
    attested_by_account_id = sqlc.narg(attested_by_account_id),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND review_status = 'SUBMITTED'
  AND version = sqlc.arg(expected_version);

-- name: RequestSellerTaxProfileSupplement :execrows
UPDATE seller_tax_profiles
SET review_status = 'SUPPLEMENT_REQUIRED',
    review_note = sqlc.narg(review_note),
    review_note_at = CASE WHEN review_note IS DISTINCT FROM sqlc.narg(review_note)
                          THEN sqlc.arg(now) ELSE review_note_at END,
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND review_status = 'UNDER_REVIEW'
  AND version = sqlc.arg(expected_version);

-- name: ResubmitSellerTaxProfileSupplement :execrows
UPDATE seller_tax_profiles
SET review_status = 'UNDER_REVIEW',
    evidence_object_keys = sqlc.arg(evidence_object_keys),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND review_status = 'SUPPLEMENT_REQUIRED'
  AND version = sqlc.arg(expected_version);

-- name: ApproveSellerTaxProfile :execrows
UPDATE seller_tax_profiles
SET review_status = sqlc.arg(next_status),
    approved_at = sqlc.arg(now),
    approval_valid_until = sqlc.arg(approval_valid_until),
    receipt_approval_basis_key = sqlc.narg(approval_basis_key),
    receipt_approval_basis_asset_id = sqlc.narg(approval_basis_asset_id),
    cooldown_until = NULL,
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND review_status IN ('UNDER_REVIEW', 'SELF_ISSUE_PENDING')
  AND version = sqlc.arg(expected_version);

-- name: RejectSellerTaxProfile :execrows
UPDATE seller_tax_profiles
SET review_status = 'REJECTED',
    cooldown_until = sqlc.arg(cooldown_until),
    review_note = sqlc.narg(review_note),
    review_note_at = CASE WHEN review_note IS DISTINCT FROM sqlc.narg(review_note)
                          THEN sqlc.arg(now) ELSE review_note_at END,
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND review_status IN ('UNDER_REVIEW', 'SELF_ISSUE_PENDING')
  AND version = sqlc.arg(expected_version);

-- name: ExtendSellerTaxProfilePending :execrows
UPDATE seller_tax_profiles
SET pending_valid_until = sqlc.arg(pending_valid_until),
    extension_count = 1,
    review_note = sqlc.narg(review_note),
    review_note_at = CASE WHEN review_note IS DISTINCT FROM sqlc.narg(review_note)
                          THEN sqlc.arg(now) ELSE review_note_at END,
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND review_status = 'SELF_ISSUE_PENDING'
  AND extension_count = 0
  AND version = sqlc.arg(expected_version);

-- name: AdminReopenSellerTaxProfileReview :execrows
UPDATE seller_tax_profiles
SET review_status = 'UNDER_REVIEW',
    recheck_reason = sqlc.arg(recheck_reason),
    cooldown_until = NULL,
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND review_status IN ('EXPIRED', 'APPROVED_INVOICE_REQUIRED', 'APPROVED_INVOICE_EXEMPT', 'REVOKED')
  AND version = sqlc.arg(expected_version);

-- name: RevokeSellerTaxProfile :execrows
UPDATE seller_tax_profiles
SET review_status = 'REVOKED',
    review_note = sqlc.narg(review_note),
    review_note_at = CASE WHEN review_note IS DISTINCT FROM sqlc.narg(review_note)
                          THEN sqlc.arg(now) ELSE review_note_at END,
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND review_status IN ('APPROVED_INVOICE_REQUIRED', 'APPROVED_INVOICE_EXEMPT')
  AND version = sqlc.arg(expected_version);

-- name: KeepSellerTaxProfileUnderReview :execrows
UPDATE seller_tax_profiles
SET review_note = sqlc.narg(review_note),
    review_note_at = CASE WHEN review_note IS DISTINCT FROM sqlc.narg(review_note)
                          THEN sqlc.arg(now) ELSE review_note_at END,
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND review_status = 'UNDER_REVIEW'
  AND version = sqlc.arg(expected_version);

-- name: BumpSellerTaxProfilePendingOrderCounters :execrows
UPDATE seller_tax_profiles
SET pending_order_count = pending_order_count + 1,
    pending_order_amount = pending_order_amount + sqlc.arg(order_amount),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id);

-- name: GetSellerTaxProfileByStoreID :one
SELECT p.* FROM seller_tax_profiles p
JOIN invoice_issuers i ON i.id = p.issuer_id
WHERE i.issuer_type = 'SELLER'
  AND i.issuer_ref_id = sqlc.arg(store_id)::bigint;

-- name: ListTaxReviewsForAdmin :many
SELECT sqlc.embed(p),
       i.issuer_ref_id AS issuer_ref_id,
       (SELECT COUNT(*)::bigint FROM invoice_complaints c
         WHERE c.issuer_id = p.issuer_id
           AND c.status = ANY(sqlc.arg(open_complaint_statuses)::text[])) AS open_complaint_count
FROM seller_tax_profiles p
JOIN invoice_issuers i ON i.id = p.issuer_id
WHERE (COALESCE(cardinality(sqlc.arg(status_filters)::text[]), 0) = 0
       OR p.review_status = ANY(sqlc.arg(status_filters)::text[]))
  AND (NOT sqlc.arg(near_deadline_only)::boolean
       OR (p.pending_valid_until IS NOT NULL
           AND p.pending_valid_until <= sqlc.arg(near_deadline_before)::timestamptz))
  AND (NOT sqlc.arg(extended_only)::boolean OR p.extension_count >= 1)
  AND (sqlc.arg(keyword)::text = ''
       OR p.tax_id = sqlc.arg(keyword)::text
       OR i.issuer_ref_id::text = sqlc.arg(keyword)::text
       OR p.legal_name ILIKE '%' || sqlc.arg(keyword)::text || '%')
ORDER BY p.created_at ASC, p.id ASC
LIMIT sqlc.arg(row_limit) OFFSET sqlc.arg(row_offset);

-- name: CountTaxReviewsForAdmin :one
SELECT COUNT(*)::bigint
FROM seller_tax_profiles p
JOIN invoice_issuers i ON i.id = p.issuer_id
WHERE (COALESCE(cardinality(sqlc.arg(status_filters)::text[]), 0) = 0
       OR p.review_status = ANY(sqlc.arg(status_filters)::text[]))
  AND (NOT sqlc.arg(near_deadline_only)::boolean
       OR (p.pending_valid_until IS NOT NULL
           AND p.pending_valid_until <= sqlc.arg(near_deadline_before)::timestamptz))
  AND (NOT sqlc.arg(extended_only)::boolean OR p.extension_count >= 1)
  AND (sqlc.arg(keyword)::text = ''
       OR p.tax_id = sqlc.arg(keyword)::text
       OR i.issuer_ref_id::text = sqlc.arg(keyword)::text
       OR p.legal_name ILIKE '%' || sqlc.arg(keyword)::text || '%');

-- name: ConfirmSellerReceiptAuthorization :execrows
UPDATE seller_tax_profiles
SET receipt_authorized_at = sqlc.narg(authorized_at),
    receipt_consent_version = sqlc.narg(consent_version),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND review_status IN ('SUBMITTED', 'UNDER_REVIEW', 'SUPPLEMENT_REQUIRED',
                        'SELF_ISSUE_PENDING', 'APPROVED_INVOICE_EXEMPT')
  AND version = sqlc.arg(expected_version);
