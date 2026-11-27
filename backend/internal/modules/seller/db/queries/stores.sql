-- name: CurrentStore :one
SELECT *
FROM seller.stores
WHERE account_id = $1 AND status <> 'CLOSED';

-- name: StoreByID :one
SELECT *
FROM seller.stores
WHERE store_id = $1;

-- name: StoresByIDs :many
SELECT *
FROM seller.stores
WHERE store_id = ANY(sqlc.arg(store_ids)::bigint[]);

-- name: StoreIDsByAccount :many
SELECT store_id
FROM seller.stores
WHERE account_id = $1
ORDER BY store_id;

-- name: StoreByCode :one
SELECT *
FROM seller.stores
WHERE store_code = $1;

-- name: ExposedStoresAfter :many
SELECT sqlc.embed(s), e.exposable_changed_at AS exposure_changed_at
FROM seller.store_exposures e
JOIN seller.stores s ON s.store_id = e.store_id
WHERE e.exposable AND e.store_id > sqlc.arg(after_store_id)::bigint
ORDER BY e.store_id
LIMIT sqlc.arg(batch_size)::int;

-- name: StoreExposureChangedAt :many
SELECT store_id, exposable_changed_at
FROM seller.store_exposures
WHERE store_id = ANY(sqlc.arg(store_ids)::bigint[]);

-- name: LockStore :one
SELECT *
FROM seller.stores
WHERE store_id = $1
FOR NO KEY UPDATE;

-- name: LockStoreKeyShare :one
SELECT *
FROM seller.stores
WHERE store_id = $1
FOR KEY SHARE;

-- name: CreateStore :one
INSERT INTO seller.stores (
    store_code, account_id, store_name, description, contact_phone, town_code, address_detail, prize,
    created_at, updated_at
)
VALUES (
    sqlc.arg(store_code), sqlc.arg(account_id), sqlc.arg(store_name)::text, sqlc.narg(description), sqlc.narg(contact_phone),
    sqlc.narg(town_code), sqlc.narg(address_detail), sqlc.narg(prize), sqlc.arg(now), sqlc.arg(now)
)
ON CONFLICT (store_code) DO NOTHING
RETURNING *;

-- name: UpdateProfile :one
UPDATE seller.stores
SET store_name = sqlc.arg(store_name)::text,
    description = sqlc.narg(description),
    contact_phone = sqlc.narg(contact_phone),
    contact_name = sqlc.narg(contact_name),
    contact_mobile = sqlc.narg(contact_mobile),
    support_email = sqlc.narg(support_email),
    town_code = sqlc.narg(town_code),
    address_detail = sqlc.narg(address_detail),
    prize = sqlc.narg(prize),
    website_url = sqlc.narg(website_url),
    onboarding_completed = sqlc.arg(onboarding_completed),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE store_id = sqlc.arg(store_id) AND version = sqlc.arg(expected_version) AND status <> 'CLOSED'
RETURNING *;

-- name: UpdateLogo :one
UPDATE seller.stores
SET logo_key = sqlc.arg(logo_key),
    logo_asset_id = sqlc.arg(logo_asset_id),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE store_id = sqlc.arg(store_id) AND version = sqlc.arg(expected_version) AND status <> 'CLOSED'
RETURNING *;

-- name: UpdateCover :one
UPDATE seller.stores
SET cover_key = sqlc.arg(cover_key),
    cover_asset_id = sqlc.arg(cover_asset_id),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE store_id = sqlc.arg(store_id) AND version = sqlc.arg(expected_version) AND status <> 'CLOSED'
RETURNING *;

-- name: MarkPendingDeletion :exec
UPDATE seller.stores
SET status = 'PENDING_DELETION',
    updated_at = sqlc.arg(now)
WHERE store_id = sqlc.arg(store_id) AND status = 'ACTIVE';

-- name: CloseStore :exec
UPDATE seller.stores
SET status = 'CLOSED',
    current_closed_at = sqlc.arg(now)::timestamptz,
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE store_id = sqlc.arg(store_id) AND status <> 'CLOSED';

-- name: OpenStoreClosure :exec
INSERT INTO seller.store_closures (store_id, closed_at, closed_by, closed_by_measure_id)
VALUES (sqlc.arg(store_id), sqlc.arg(now), sqlc.arg(closed_by), sqlc.narg(closed_by_measure_id));

-- name: ReopenStore :one
UPDATE seller.stores
SET status = 'ACTIVE',
    current_closed_at = NULL,
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE store_id = sqlc.arg(store_id) AND status = 'CLOSED' AND retired_at IS NULL
RETURNING *;

-- name: EndStoreClosure :execrows
UPDATE seller.store_closures
SET reopened_at = sqlc.arg(now)
WHERE store_id = sqlc.arg(store_id) AND reopened_at IS NULL;

-- name: CurrentStoreClosures :many
SELECT *
FROM seller.store_closures
WHERE store_id = ANY(sqlc.arg(store_ids)::bigint[]) AND reopened_at IS NULL;

-- name: StoreClosures :many
SELECT *
FROM seller.store_closures
WHERE store_id = $1
ORDER BY closed_at;

-- name: LatestStoreOfAccount :one
SELECT *
FROM seller.stores
WHERE account_id = $1
ORDER BY store_id DESC
LIMIT 1;

-- name: LockAccountStores :many
SELECT *
FROM seller.stores
WHERE account_id = $1
ORDER BY store_id
FOR UPDATE;

-- name: AdminStores :many
SELECT st.*
FROM seller.stores st
WHERE st.status = ANY(sqlc.arg(statuses)::text[])
  AND (sqlc.narg(account_id)::bigint IS NULL OR st.account_id = sqlc.narg(account_id)::bigint)
  AND (
    sqlc.narg(query_pattern)::text IS NULL OR
    st.store_name ILIKE sqlc.narg(query_pattern)::text ESCAPE '\' OR
    st.contact_phone ILIKE sqlc.narg(query_pattern)::text ESCAPE '\' OR
    st.store_code = sqlc.narg(store_code)::bigint
  )
  AND (
    sqlc.narg(kyc_status)::text IS NULL OR
    EXISTS (
      SELECT 1 FROM seller.kyc_applications k
      WHERE k.store_id = st.store_id AND k.status <> 'CANCELLED' AND k.status = sqlc.narg(kyc_status)::text
    )
  )
  AND (
    NOT sqlc.arg(kyc_awaiting_review)::bool OR
    EXISTS (SELECT 1 FROM seller.review_kyc_application(sqlc.arg(now)::timestamptz) r WHERE r.store_id = st.store_id)
  )
  AND (
    sqlc.narg(suspended)::bool IS NULL OR
    (st.account_id = ANY(sqlc.arg(suspended_accounts)::bigint[])) = sqlc.narg(suspended)::bool
  )
  AND (
    sqlc.narg(after_created_at)::timestamptz IS NULL OR
    (st.created_at, st.store_id) < (sqlc.narg(after_created_at)::timestamptz, sqlc.narg(after_id)::bigint)
  )
ORDER BY st.created_at DESC, st.store_id DESC
LIMIT sqlc.arg(page_size);

-- name: AdminStoreStats :one
SELECT
  count(*)::int AS total,
  (count(*) FILTER (WHERE EXISTS (SELECT 1 FROM seller.review_kyc_application(sqlc.arg(now)::timestamptz) r WHERE r.store_id = st.store_id)))::int AS kyc_awaiting_review,
  (count(*) FILTER (WHERE k.status = 'APPROVED'))::int AS kyc_approved,
  (count(*) FILTER (WHERE k.status = 'REJECTED'))::int AS kyc_rejected,
  (count(*) FILTER (WHERE st.created_at >= sqlc.arg(month_start)::timestamptz AND st.created_at < sqlc.arg(month_end)::timestamptz))::int AS new_this_month
FROM seller.stores st
LEFT JOIN seller.kyc_applications k ON k.store_id = st.store_id AND k.status <> 'CANCELLED'
WHERE st.status <> 'CLOSED';

-- name: ListLiveStoreAccounts :many
SELECT store_id, account_id
FROM seller.stores
WHERE retired_at IS NULL AND store_id > sqlc.arg(after_store_id)
ORDER BY store_id
LIMIT sqlc.arg(page_size);
