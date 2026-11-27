-- name: InsertProductViewEvent :exec
INSERT INTO seller.product_view_events (event_id, product_id, occurred_at)
VALUES (sqlc.arg(event_id), sqlc.arg(product_id), sqlc.arg(occurred_at))
ON CONFLICT (event_id) DO NOTHING;

-- name: PendingProductViewEvents :many
SELECT event_id, product_id
FROM seller.product_view_events
WHERE processed_at IS NULL
ORDER BY occurred_at, event_id
LIMIT sqlc.arg(batch_limit);

-- name: LockPendingProductViewEvents :many
SELECT event_id, product_id
FROM seller.product_view_events
WHERE event_id = ANY(sqlc.arg(event_ids)::uuid[]) AND processed_at IS NULL
ORDER BY occurred_at, event_id
FOR UPDATE SKIP LOCKED;

-- name: ApplyProductViewResolutions :one
WITH resolved AS (
    SELECT a.event_id, s.store_id
    FROM unnest(sqlc.arg(event_ids)::uuid[]) WITH ORDINALITY AS a(event_id, n)
    JOIN unnest(sqlc.arg(store_ids)::bigint[]) WITH ORDINALITY AS b(store_id, n) USING (n)
    LEFT JOIN seller.stores s ON s.store_id = NULLIF(b.store_id, 0)
), marked AS (
    UPDATE seller.product_view_events e
    SET store_id = resolved.store_id,
        processed_at = sqlc.arg(processed_at)::timestamptz,
        outcome = CASE WHEN resolved.store_id IS NULL THEN 'UNRESOLVED' ELSE 'COUNTED' END
    FROM resolved
    WHERE e.event_id = resolved.event_id AND e.processed_at IS NULL
    RETURNING e.store_id, e.occurred_at
), bumped AS (
    INSERT INTO seller.product_views_daily (store_id, day, views)
    SELECT store_id, (occurred_at AT TIME ZONE 'Asia/Taipei')::date, count(*)
    FROM marked
    WHERE store_id IS NOT NULL
    GROUP BY 1, 2
    ON CONFLICT (store_id, day) DO UPDATE SET views = seller.product_views_daily.views + EXCLUDED.views
    RETURNING 1
)
SELECT count(*)::bigint FROM marked;

-- name: LockProductViewMaintenance :exec
SELECT pg_advisory_xact_lock_shared(hashtextextended('seller.product_views', 0));

-- name: LockProductViewRebuild :exec
SELECT pg_advisory_xact_lock(hashtextextended('seller.product_views', 0));

-- name: DeleteProductViewsDaily :exec
DELETE FROM seller.product_views_daily
WHERE day >= sqlc.arg(from_day)::date AND day <= sqlc.arg(to_day)::date;

-- name: InsertRebuiltProductViewsDaily :exec
INSERT INTO seller.product_views_daily (store_id, day, views)
SELECT e.store_id, (e.occurred_at AT TIME ZONE 'Asia/Taipei')::date AS day, count(*)
FROM seller.product_view_events e
JOIN seller.stores s ON s.store_id = e.store_id
WHERE e.outcome = 'COUNTED'
  AND (e.occurred_at AT TIME ZONE 'Asia/Taipei')::date >= sqlc.arg(from_day)::date
  AND (e.occurred_at AT TIME ZONE 'Asia/Taipei')::date <= sqlc.arg(to_day)::date
GROUP BY e.store_id, day;

-- name: PurgeProcessedProductViewEvents :one
WITH doomed AS (
    SELECT event_id
    FROM seller.product_view_events
    WHERE processed_at IS NOT NULL AND occurred_at < sqlc.arg(cutoff)::timestamptz
    ORDER BY occurred_at
    LIMIT sqlc.arg(batch_limit)
    FOR UPDATE SKIP LOCKED
), deleted AS (
    DELETE FROM seller.product_view_events e USING doomed WHERE e.event_id = doomed.event_id RETURNING 1
)
SELECT count(*)::bigint FROM deleted;

-- name: ProductViewsBetween :one
SELECT COALESCE(SUM(views), 0)::bigint AS views
FROM seller.product_views_daily
WHERE store_id = sqlc.arg(store_id) AND day >= sqlc.arg(first_day)::date AND day < sqlc.arg(end_day)::date;
