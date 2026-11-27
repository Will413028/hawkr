-- name: KYCApplicationByID :one
SELECT *
FROM seller.kyc_applications
WHERE application_id = $1;

-- name: LockKYCApplication :one
SELECT *
FROM seller.kyc_applications
WHERE application_id = $1
FOR UPDATE;

-- name: CurrentKYCApplication :one
SELECT *
FROM seller.kyc_applications
WHERE store_id = $1 AND status <> 'CANCELLED';

-- name: InsertKYCApplication :one
INSERT INTO seller.kyc_applications (
    application_id, store_id, seller_type,
    product_type, website_url,
    company_name, tax_id, company_register_date,
    company_town_code, company_address_detail, company_phone,
    created_at, updated_at
)
VALUES (
    sqlc.arg(application_id), sqlc.arg(store_id), sqlc.arg(seller_type),
    sqlc.narg(product_type), sqlc.narg(website_url),
    sqlc.narg(company_name), sqlc.narg(tax_id), sqlc.narg(company_register_date),
    sqlc.narg(company_town_code), sqlc.narg(company_address_detail), sqlc.narg(company_phone),
    sqlc.arg(now), sqlc.arg(now)
)
RETURNING *;

-- name: UpdateKYCDraft :one
UPDATE seller.kyc_applications
SET status = 'DRAFT',
    decided_at = NULL,
    seller_type = sqlc.arg(seller_type),
    product_type = sqlc.narg(product_type),
    website_url = sqlc.narg(website_url),
    company_name = sqlc.narg(company_name),
    tax_id = sqlc.narg(tax_id),
    company_register_date = sqlc.narg(company_register_date),
    company_town_code = sqlc.narg(company_town_code),
    company_address_detail = sqlc.narg(company_address_detail),
    company_phone = sqlc.narg(company_phone),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE application_id = sqlc.arg(application_id) AND version = sqlc.arg(expected_version) AND status IN ('DRAFT', 'REJECTED')
RETURNING *;

-- name: TransitionKYCApplication :one
UPDATE seller.kyc_applications
SET status = sqlc.arg(to_status),
    submitted_at = COALESCE(sqlc.narg(submitted_at)::timestamptz, submitted_at),
    submission_seq = submission_seq + CASE WHEN sqlc.arg(to_status)::text = 'SUBMITTED' THEN 1 ELSE 0 END,
    decided_at = sqlc.narg(decided_at)::timestamptz,
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE application_id = sqlc.arg(application_id) AND version = sqlc.arg(expected_version) AND status = sqlc.arg(from_status)
RETURNING *;

-- name: KYCDocuments :many
SELECT *
FROM seller.kyc_documents
WHERE application_id = $1
ORDER BY position;

-- name: DeleteKYCDocuments :exec
DELETE FROM seller.kyc_documents WHERE application_id = $1;

-- name: InsertKYCDocument :exec
INSERT INTO seller.kyc_documents (application_id, position, asset_id, object_key)
VALUES (sqlc.arg(application_id), sqlc.arg(position), sqlc.arg(asset_id), sqlc.arg(object_key));

-- name: InsertKYCReview :one
INSERT INTO seller.kyc_reviews (review_id, application_id, action, reviewer_admin_id, reason, internal_note, decided_at, registration_check_id)
VALUES (sqlc.arg(review_id), sqlc.arg(application_id), sqlc.arg(action), sqlc.narg(reviewer_admin_id),
        sqlc.narg(reason), sqlc.narg(internal_note), sqlc.arg(now), sqlc.narg(registration_check_id))
RETURNING *;

-- name: InsertKYCReviewIssue :exec
INSERT INTO seller.kyc_review_issues (issue_id, review_id, position, field, document_asset_id, code, message)
VALUES (sqlc.arg(issue_id), sqlc.arg(review_id), sqlc.arg(position), sqlc.arg(field), sqlc.narg(document_asset_id), sqlc.arg(code), sqlc.arg(message));

-- name: KYCReviewIssues :many
SELECT i.*
FROM seller.kyc_review_issues i
JOIN seller.kyc_reviews r ON r.review_id = i.review_id
WHERE r.application_id = $1
ORDER BY r.decided_at, r.review_id, i.position;

-- name: UnresolvedKYCReviewIssues :many
SELECT i.issue_id, i.field, i.document_asset_id
FROM seller.kyc_review_issues i
JOIN seller.kyc_reviews r ON r.review_id = i.review_id
WHERE r.application_id = $1 AND i.resolved_at IS NULL;

-- name: ResolveKYCReviewIssues :exec
UPDATE seller.kyc_review_issues
SET resolved_at = sqlc.arg(now)::timestamptz
WHERE issue_id = ANY(sqlc.arg(issue_ids)::uuid[]) AND resolved_at IS NULL;

-- name: KYCReviews :many
SELECT *
FROM seller.kyc_reviews
WHERE application_id = $1
ORDER BY decided_at, review_id;

-- name: InsertKYCDocumentAccess :one
INSERT INTO seller.kyc_document_accesses (access_id, application_id, position, asset_id, admin_id, accessed_at)
VALUES (sqlc.arg(access_id), sqlc.arg(application_id), sqlc.arg(position), sqlc.arg(asset_id), sqlc.arg(admin_id), sqlc.arg(now))
RETURNING *;

-- name: ListKYCDocumentAccesses :many
SELECT access_id, admin_id, position, asset_id, accessed_at
FROM seller.kyc_document_accesses
WHERE application_id = sqlc.arg(application_id)
  AND (
    sqlc.narg(after_at)::timestamptz IS NULL OR
    (accessed_at, access_id) < (sqlc.narg(after_at)::timestamptz, sqlc.narg(after_id)::uuid)
  )
ORDER BY accessed_at DESC, access_id DESC
LIMIT sqlc.arg(page_size);

-- name: AdminKYCApplications :many
SELECT sqlc.embed(a), s.account_id, s.store_name
FROM seller.kyc_applications a
JOIN seller.stores s ON s.store_id = a.store_id
WHERE a.status <> 'DRAFT' AND a.submitted_at IS NOT NULL
  AND (sqlc.narg(status)::text IS NULL OR a.status = sqlc.narg(status)::text)
  AND (
    sqlc.narg(query_pattern)::text IS NULL OR
    s.store_name ILIKE sqlc.narg(query_pattern)::text ESCAPE '\' OR
    a.company_name ILIKE sqlc.narg(query_pattern)::text ESCAPE '\' OR
    a.tax_id ILIKE sqlc.narg(query_pattern)::text ESCAPE '\'
  )
  AND (
    sqlc.narg(after_submitted_at)::timestamptz IS NULL OR
    (a.submitted_at, a.application_id) < (sqlc.narg(after_submitted_at)::timestamptz, sqlc.narg(after_id)::uuid)
  )
ORDER BY a.submitted_at DESC, a.application_id DESC
LIMIT sqlc.arg(page_size);

-- name: AdminKYCApplicationsOldestFirst :many
SELECT sqlc.embed(a), s.account_id, s.store_name
FROM seller.kyc_applications a
JOIN seller.stores s ON s.store_id = a.store_id
WHERE a.status <> 'DRAFT' AND a.submitted_at IS NOT NULL
  AND (sqlc.narg(status)::text IS NULL OR a.status = sqlc.narg(status)::text)
  AND (
    sqlc.narg(query_pattern)::text IS NULL OR
    s.store_name ILIKE sqlc.narg(query_pattern)::text ESCAPE '\' OR
    a.company_name ILIKE sqlc.narg(query_pattern)::text ESCAPE '\' OR
    a.tax_id ILIKE sqlc.narg(query_pattern)::text ESCAPE '\'
  )
  AND (
    sqlc.narg(after_submitted_at)::timestamptz IS NULL OR
    (a.submitted_at, a.application_id) > (sqlc.narg(after_submitted_at)::timestamptz, sqlc.narg(after_id)::uuid)
  )
ORDER BY a.submitted_at, a.application_id
LIMIT sqlc.arg(page_size);

-- name: SnapshotMeasureIdentities :exec
INSERT INTO seller.measure_identities (measure_id, kind, key_id, hash)
SELECT sqlc.arg(measure_id)::uuid,
       CASE h.field WHEN 'TAX_ID' THEN 'TAX_ID' ELSE 'BANK_ACCOUNT' END,
       h.key_id, h.hash
FROM seller.account_identity_hashes h
WHERE h.account_id = sqlc.arg(account_id)
  AND EXISTS (SELECT 1 FROM seller.identity_effect_measures i WHERE i.id = sqlc.arg(measure_id)::uuid)
ON CONFLICT DO NOTHING;

-- name: MeasureIdentityMatches :many
SELECT DISTINCT s.account_id::bigint AS account_id, s.kind::text AS kind
FROM seller.measure_identity_sources(sqlc.arg(at)::timestamptz) s
WHERE s.hash = ANY(sqlc.arg(hashes)::bytea[]) AND s.account_id <> sqlc.arg(account_id)::bigint
ORDER BY 1, 2;

-- name: LinkedAccountHits :many
SELECT DISTINCT h.account_id::bigint AS account_id, h.field::text AS field
FROM seller.measure_identity_hits(sqlc.arg(at)::timestamptz) h
WHERE h.measure_id = sqlc.arg(measure_id)::uuid
  AND h.account_id <> sqlc.arg(except_account_id)::bigint
  AND (sqlc.narg(account_id)::bigint IS NULL OR h.account_id = sqlc.narg(account_id)::bigint)
ORDER BY 1, 2;

-- name: SharedBankAccountPendingAccounts :many
SELECT h.account_id
FROM (SELECT DISTINCT x.account_id::bigint AS account_id FROM seller.measure_identity_hits(sqlc.arg(at)::timestamptz) x WHERE x.measure_id = sqlc.arg(measure_id)::uuid) h
WHERE EXISTS (
    SELECT 1 FROM seller.review_shared_bank_account(sqlc.arg(at)::timestamptz) r
    WHERE r.measure_id = sqlc.arg(measure_id) AND r.account_id = h.account_id)
ORDER BY h.account_id;

-- name: MeasureLinkedReviews :many
SELECT account_id, reviewed_at
FROM seller.measure_linked_reviews
WHERE measure_id = $1
ORDER BY account_id;

-- name: BankFingerprintHitExists :one
SELECT EXISTS (
    SELECT 1
    FROM seller.measures m
    WHERE m.id = sqlc.arg(measure_id)
      AND EXISTS (
          SELECT 1 FROM seller.measure_identity_hits(sqlc.arg(at)::timestamptz) h
          WHERE h.measure_id = m.id AND h.account_id = sqlc.arg(account_id)::bigint AND h.field = 'BANK_FINGERPRINT' AND h.account_id <> m.account_id)
)::boolean;

-- name: MarkLinkedAccountReviewed :execrows
INSERT INTO seller.measure_linked_reviews (measure_id, account_id, reviewed_at, reviewed_by_admin_id)
VALUES (sqlc.arg(measure_id), sqlc.arg(account_id), sqlc.arg(now), sqlc.arg(admin_id))
ON CONFLICT DO NOTHING;

-- name: InsertAccountIdentityHash :exec
INSERT INTO seller.account_identity_hashes (source_store_id, field, key_id, hash, account_id, created_at)
SELECT s.store_id, sqlc.arg(field)::text, sqlc.arg(key_id)::text, sqlc.arg(hash)::bytea, s.account_id, sqlc.arg(created_at)::timestamptz
FROM seller.stores s
WHERE s.store_id = sqlc.arg(store_id)
ON CONFLICT DO NOTHING;

-- name: StoreKYCTaxIDs :many
SELECT tax_id::text AS tax_id
FROM seller.kyc_applications
WHERE store_id = $1 AND tax_id IS NOT NULL;

-- name: PruneAccountIdentityHashes :exec
DELETE FROM seller.account_identity_hashes a
WHERE a.source_store_id = sqlc.arg(store_id) AND a.field = 'TAX_ID'
  AND a.hash <> ALL(sqlc.arg(keep)::bytea[]);

-- name: KYCApplicationsMissingHashKey :many
SELECT a.*
FROM seller.kyc_applications a
WHERE a.pii_purged_at IS NULL
  AND a.tax_id IS NOT NULL
  AND a.store_id IN (
    SELECT DISTINCT m.store_id
    FROM seller.kyc_applications m
    WHERE m.pii_purged_at IS NULL
      AND m.tax_id IS NOT NULL
      AND (sqlc.narg(after_store_id)::bigint IS NULL OR m.store_id > sqlc.narg(after_store_id)::bigint)
      AND NOT EXISTS (
        SELECT 1 FROM seller.account_identity_hashes h
        WHERE h.source_store_id = m.store_id AND h.field = 'TAX_ID' AND h.key_id = sqlc.arg(key_id)::text)
    ORDER BY m.store_id
    LIMIT sqlc.arg(batch_limit))
ORDER BY a.store_id, a.application_id
FOR UPDATE OF a;

-- name: DeleteIdentitiesOutside :exec
DELETE FROM seller.measure_identities mi
USING seller.measures m
WHERE m.id = mi.measure_id AND m.account_id = sqlc.arg(account_id)
  AND mi.measure_id <> ALL(sqlc.arg(keep)::uuid[]);

-- name: AccountsWithMeasureSnapshots :many
SELECT DISTINCT m.account_id
FROM seller.measure_identities mi
JOIN seller.measures m ON m.id = mi.measure_id
ORDER BY m.account_id;

-- name: ReferencedKeyIDs :many
SELECT 'HASH'::text AS kind, k.key_id::text AS key_id
FROM (
    SELECT key_id FROM seller.account_identity_hashes
    UNION SELECT key_id FROM seller.measure_identities
) k
WHERE k.key_id IS NOT NULL
ORDER BY 1, 2;

-- name: CurrentKYCStatuses :many
SELECT store_id, status
FROM seller.kyc_applications
WHERE store_id = ANY(sqlc.arg(store_ids)::bigint[]) AND status <> 'CANCELLED';

-- name: ReviewKYCApplicationByIDs :many
SELECT r.kyc_application_id::uuid AS kyc_application_id, r.start_at::timestamptz AS start_at, r.due_at::timestamptz AS due_at
FROM seller.review_kyc_application(sqlc.arg(at)::timestamptz) r
WHERE r.kyc_application_id = ANY(sqlc.arg(ids)::uuid[]);
