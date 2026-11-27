-- name: InsertStoreExposure :exec
INSERT INTO seller.store_exposures (store_id)
VALUES ($1);

-- name: LockStoreExposures :many
SELECT *
FROM seller.store_exposures
WHERE store_id = ANY(sqlc.arg(store_ids)::bigint[])
ORDER BY store_id
FOR UPDATE;

-- name: AnnounceStoreExposure :one
UPDATE seller.store_exposures
SET exposable = sqlc.arg(exposable)::boolean,
    version = version + 1,
    exposable_changed_at = CASE WHEN exposable_changed_at IS NULL OR exposable <> sqlc.arg(exposable)::boolean THEN sqlc.arg(now) ELSE exposable_changed_at END
WHERE store_id = sqlc.arg(store_id)
RETURNING *;

-- name: StampFirstExposed :execrows
UPDATE seller.stores
SET first_exposed_at = sqlc.arg(now)
WHERE store_id = ANY(sqlc.arg(store_ids)::bigint[]) AND first_exposed_at IS NULL;

-- name: ExposureReconcileCandidates :many
SELECT s.store_id
FROM seller.stores s
LEFT JOIN seller.store_exposures e ON e.store_id = s.store_id
WHERE (s.status <> 'CLOSED' OR COALESCE(e.exposable, false))
  AND s.store_id > sqlc.arg(after_store_id)::bigint
ORDER BY s.store_id
LIMIT sqlc.arg(batch_size)::int;

-- name: ExposureStoreIDs :many
SELECT store_id
FROM seller.store_exposures
WHERE store_id > sqlc.arg(after_store_id)::bigint
ORDER BY store_id
LIMIT sqlc.arg(batch_size)::int;

-- name: ExposableStoresByCounty :many
SELECT s.store_id, left(s.town_code, 5)::text AS county_code
FROM seller.store_exposures e
JOIN seller.stores s ON s.store_id = e.store_id
WHERE e.exposable AND left(s.town_code, 5) = ANY(sqlc.arg(county_codes)::text[])
ORDER BY s.store_id;

-- name: StoreRequirementFlags :many
SELECT f.store_id, f.active, f.not_pending_deletion, f.kyc_approved, f.transfers_active, f.has_store_name, f.has_logo, f.has_town_code
FROM seller.store_requirement_flags f
WHERE f.store_id = ANY(sqlc.arg(store_ids)::bigint[]);

-- name: ExposableStoreIDs :many
SELECT s.store_id
FROM seller.stores s
WHERE s.store_id = ANY(sqlc.arg(store_ids)::bigint[])
  AND s.store_id IN (SELECT e.store_id FROM seller.pub_exposable_stores(sqlc.arg(at)::timestamptz) e)
ORDER BY s.store_id;
