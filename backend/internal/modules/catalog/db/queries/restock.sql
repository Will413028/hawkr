
-- name: GetSkuRestockState :one
SELECT s.product_id, p.store_id, p.title, s.stock_qty,
       COALESCE((SELECT i.object_key FROM catalog.product_images i WHERE i.product_id = p.id AND i.kind = 'GALLERY' ORDER BY i.position LIMIT 1), '')::text AS cover_key,
       (s.deleted_at IS NULL AND s.stock_qty > 0
        AND EXISTS (SELECT 1 FROM catalog.buyer_visible_products(sqlc.arg(at)::timestamptz) v WHERE v.id = p.id))::boolean AS purchasable
FROM catalog.product_skus s JOIN catalog.products p ON p.id = s.product_id
WHERE s.id = sqlc.arg(id);


-- name: EnqueueRestock :exec
INSERT INTO catalog.restock_queue AS q (sku_id, due_at, requested_at)
VALUES (sqlc.arg(sku_id), sqlc.arg(due_at), sqlc.arg(now))
ON CONFLICT (sku_id) DO UPDATE SET
    requested_at = GREATEST(EXCLUDED.requested_at, q.requested_at + interval '1 microsecond'),
    due_at = CASE WHEN q.failed_at IS NOT NULL THEN EXCLUDED.due_at ELSE q.due_at END,
    attempts = CASE WHEN q.failed_at IS NOT NULL THEN 0 ELSE q.attempts END,
    last_error = CASE WHEN q.failed_at IS NOT NULL THEN NULL ELSE q.last_error END,
    failed_at = NULL
WHERE q.claimed_until IS NOT NULL OR q.failed_at IS NOT NULL;

-- name: ClaimRestock :many
WITH picked AS (
    SELECT sku_id FROM catalog.restock_queue
    WHERE failed_at IS NULL AND due_at <= sqlc.arg(now)::timestamptz AND (claimed_until IS NULL OR claimed_until < sqlc.arg(now)::timestamptz)
    ORDER BY due_at, sku_id
    LIMIT sqlc.arg(max_rows)
    FOR UPDATE SKIP LOCKED)
UPDATE catalog.restock_queue AS q SET claimed_until = sqlc.arg(lease_until)::timestamptz
FROM picked AS p WHERE q.sku_id = p.sku_id
RETURNING q.sku_id, q.requested_at, q.attempts;

-- name: FinishRestock :execrows
DELETE FROM catalog.restock_queue WHERE sku_id = sqlc.arg(sku_id) AND requested_at = sqlc.arg(requested_at);

-- name: RearmRestock :exec
UPDATE catalog.restock_queue SET claimed_until = NULL, due_at = sqlc.arg(due_at), attempts = 0, last_error = NULL
WHERE sku_id = sqlc.arg(sku_id) AND claimed_until IS NOT NULL;

-- name: FailRestock :execrows
UPDATE catalog.restock_queue SET attempts = attempts + 1, due_at = sqlc.arg(due_at), claimed_until = NULL, last_error = sqlc.arg(last_error), failed_at = sqlc.narg(failed_at)
WHERE sku_id = sqlc.arg(sku_id) AND requested_at = sqlc.arg(requested_at);

-- name: EnqueuePurge :exec
INSERT INTO catalog.restock_purge_queue AS q (sku_id, due_at, requested_at)
VALUES (sqlc.arg(sku_id), sqlc.arg(due_at), sqlc.arg(now))
ON CONFLICT (sku_id) DO UPDATE SET
    requested_at = GREATEST(EXCLUDED.requested_at, q.requested_at + interval '1 microsecond'),
    due_at = CASE WHEN q.failed_at IS NOT NULL THEN EXCLUDED.due_at ELSE q.due_at END,
    attempts = CASE WHEN q.failed_at IS NOT NULL THEN 0 ELSE q.attempts END,
    last_error = CASE WHEN q.failed_at IS NOT NULL THEN NULL ELSE q.last_error END,
    failed_at = NULL
WHERE q.claimed_until IS NOT NULL OR q.failed_at IS NOT NULL;

-- name: ClaimPurge :many
WITH picked AS (
    SELECT sku_id FROM catalog.restock_purge_queue
    WHERE failed_at IS NULL AND due_at <= sqlc.arg(now)::timestamptz AND (claimed_until IS NULL OR claimed_until < sqlc.arg(now)::timestamptz)
    ORDER BY due_at, sku_id
    LIMIT sqlc.arg(max_rows)
    FOR UPDATE SKIP LOCKED)
UPDATE catalog.restock_purge_queue AS q SET claimed_until = sqlc.arg(lease_until)::timestamptz
FROM picked AS p WHERE q.sku_id = p.sku_id
RETURNING q.sku_id, q.requested_at, q.attempts;

-- name: FinishPurge :execrows
DELETE FROM catalog.restock_purge_queue WHERE sku_id = sqlc.arg(sku_id) AND requested_at = sqlc.arg(requested_at);

-- name: RearmPurge :exec
UPDATE catalog.restock_purge_queue SET claimed_until = NULL, due_at = sqlc.arg(due_at), attempts = 0, last_error = NULL
WHERE sku_id = sqlc.arg(sku_id) AND claimed_until IS NOT NULL;

-- name: FailPurge :execrows
UPDATE catalog.restock_purge_queue SET attempts = attempts + 1, due_at = sqlc.arg(due_at), claimed_until = NULL, last_error = sqlc.arg(last_error), failed_at = sqlc.narg(failed_at)
WHERE sku_id = sqlc.arg(sku_id) AND requested_at = sqlc.arg(requested_at);

-- name: EnqueueRestockOfProduct :exec
INSERT INTO catalog.restock_queue AS q (sku_id, due_at, requested_at)
SELECT s.id, sqlc.arg(due_at), sqlc.arg(now) FROM catalog.product_skus s
WHERE s.product_id = sqlc.arg(product_id) AND s.deleted_at IS NULL
ORDER BY s.id
ON CONFLICT (sku_id) DO UPDATE SET
    requested_at = GREATEST(EXCLUDED.requested_at, q.requested_at + interval '1 microsecond'),
    due_at = CASE WHEN q.failed_at IS NOT NULL THEN EXCLUDED.due_at ELSE q.due_at END,
    attempts = CASE WHEN q.failed_at IS NOT NULL THEN 0 ELSE q.attempts END,
    last_error = CASE WHEN q.failed_at IS NOT NULL THEN NULL ELSE q.last_error END,
    failed_at = NULL
WHERE q.claimed_until IS NOT NULL OR q.failed_at IS NOT NULL;
