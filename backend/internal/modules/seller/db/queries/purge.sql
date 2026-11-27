-- name: RetireDueStores :many
SELECT s.store_id, s.store_code, s.account_id, s.logo_asset_id, s.cover_asset_id
FROM seller.stores s
JOIN seller.store_closures c ON c.store_id = s.store_id AND c.reopened_at IS NULL
WHERE s.status = 'CLOSED'
  AND CASE WHEN sqlc.arg(second_stage)::boolean THEN s.retired_at IS NOT NULL AND s.pii_purged_at IS NULL ELSE s.retired_at IS NULL END
  AND (sqlc.narg(store_ids)::bigint[] IS NULL OR s.store_id = ANY(sqlc.narg(store_ids)::bigint[]))
  AND (sqlc.narg(after_store_id)::bigint IS NULL OR s.store_id > sqlc.narg(after_store_id)::bigint)
  AND sqlc.arg(now)::timestamptz > CASE
        WHEN c.closed_by = 'TERMINATION'
        THEN GREATEST(
          c.closed_at + make_interval(hours => sqlc.arg(hours)::int),
          (SELECT max(lt.starts_at) FROM seller.measures lt
           WHERE lt.account_id = s.account_id AND lt.kind = 'TERMINATION' AND lt.status = 'ENFORCED') + make_interval(hours => sqlc.arg(appeal_window_hours)::int))
        ELSE c.closed_at + make_interval(hours => sqlc.arg(hours)::int)
      END
ORDER BY s.store_id
LIMIT sqlc.arg(batch_limit)
FOR UPDATE OF s SKIP LOCKED;

-- name: StoreHolds :many
SELECT s.store_id,
       EXISTS (
         SELECT 1
         FROM seller.measures tm
         WHERE tm.account_id = s.account_id AND tm.kind = 'TERMINATION' AND tm.status = 'ENFORCED'
           AND EXISTS (
             SELECT 1 FROM seller.measure_requests r
             WHERE r.state = 'PENDING' AND r.measure_id IN (tm.id, tm.source_measure_id)
           )
       )::boolean AS appeal_hold,
       EXISTS (
         SELECT 1
         FROM seller.authority_order_targets pt
         JOIN seller.authority_orders po ON po.id = pt.order_id
         WHERE pt.account_id = s.account_id AND (pt.store_id IS NULL OR pt.store_id = s.store_id)
           AND po.revoked_at IS NULL AND po.hold_until > sqlc.arg(now)::timestamptz
       )::boolean AS preservation_hold
FROM seller.stores s
WHERE s.store_id = ANY(sqlc.arg(store_ids)::bigint[])
ORDER BY s.store_id;

-- name: RetireStores :many
UPDATE seller.stores
SET description = NULL,
    prize = NULL,
    website_url = NULL,
    logo_key = NULL,
    logo_asset_id = NULL,
    cover_key = NULL,
    cover_asset_id = NULL,
    retired_at = sqlc.arg(now),
    updated_at = sqlc.arg(now)
WHERE store_id = ANY(sqlc.arg(store_ids)::bigint[]) AND status = 'CLOSED' AND retired_at IS NULL
RETURNING store_id;

-- name: PurgeStores :many
UPDATE seller.stores
SET store_name = NULL,
    contact_phone = NULL,
    contact_name = NULL,
    contact_mobile = NULL,
    support_email = NULL,
    town_code = NULL,
    address_detail = NULL,
    pii_purged_at = sqlc.arg(now),
    updated_at = sqlc.arg(now)
WHERE store_id = ANY(sqlc.arg(store_ids)::bigint[]) AND retired_at IS NOT NULL AND pii_purged_at IS NULL
RETURNING store_id, store_code;

-- name: PurgeKYCApplications :many
UPDATE seller.kyc_applications a
SET website_url = NULL,
    company_name = NULL,
    tax_id = NULL,
    company_register_date = NULL,
    company_town_code = NULL,
    company_address_detail = NULL,
    company_phone = NULL,
    pii_purged_at = sqlc.arg(now)
FROM seller.stores s
WHERE a.store_id = ANY(sqlc.arg(store_ids)::bigint[]) AND s.store_id = a.store_id AND s.pii_purged_at IS NOT NULL AND a.pii_purged_at IS NULL
RETURNING a.application_id;

-- name: ExpiredIdentityHashStores :many
SELECT DISTINCT a.source_store_id AS store_id
FROM seller.account_identity_hashes a
JOIN seller.store_closures c ON c.store_id = a.source_store_id AND c.reopened_at IS NULL
WHERE sqlc.arg(now)::timestamptz > c.closed_at + make_interval(hours => sqlc.arg(hours)::int)
  AND (sqlc.narg(after_store_id)::bigint IS NULL OR a.source_store_id > sqlc.narg(after_store_id)::bigint)
ORDER BY a.source_store_id
LIMIT sqlc.arg(batch_limit);

-- name: DeleteExpiredAccountIdentityHashes :execrows
DELETE FROM seller.account_identity_hashes a
USING seller.store_closures c
WHERE c.store_id = a.source_store_id AND c.reopened_at IS NULL
  AND a.source_store_id = ANY(sqlc.arg(store_ids)::bigint[])
  AND sqlc.arg(now)::timestamptz > c.closed_at + make_interval(hours => sqlc.arg(hours)::int);

-- name: PurgeKYCDocuments :many
DELETE FROM seller.kyc_documents
WHERE application_id = ANY(sqlc.arg(application_ids)::uuid[])
RETURNING application_id, asset_id;
