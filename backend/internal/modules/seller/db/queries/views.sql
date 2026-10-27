-- name: InsertProductViewEvent :exec
-- 寫入端只追加一列；event_id 由產生端給（UUIDv7），重送的事件不會多一列。
INSERT INTO seller.product_view_events (event_id, product_id, occurred_at)
VALUES (sqlc.arg(event_id), sqlc.arg(product_id), sqlc.arg(occurred_at))
ON CONFLICT (event_id) DO NOTHING;

-- name: PendingProductViewEvents :many
-- 還沒處理的事件，依發生順序取一批（不加鎖：解析在 transaction 之外，寫入時才鎖）。
SELECT event_id, product_id
FROM seller.product_view_events
WHERE processed_at IS NULL
ORDER BY occurred_at, event_id
LIMIT sqlc.arg(batch_limit);

-- name: LockPendingProductViewEvents :many
-- 鎖住這批裡仍然沒處理的事件；已被別的彙總 job 鎖住的跳過，已處理的不會再選中。
SELECT event_id, product_id
FROM seller.product_view_events
WHERE event_id = ANY(sqlc.arg(event_ids)::uuid[]) AND processed_at IS NULL
ORDER BY occurred_at, event_id
FOR UPDATE SKIP LOCKED;

-- name: ApplyProductViewResolutions :one
-- 一句話內標記事件並累加每日彙總：被標記的列才進彙總，所以標記與計數不會分開成功。store_ids 與 event_ids 逐項對應，0 表示
-- 商品不存在或沒有店家，或店家不在 seller.stores（UNRESOLVED，不計入，不讓每日彙總違反 store_id 的外鍵）。台北日曆日取自
-- 事件的 occurred_at，不是處理時間。回傳標記的事件數。
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
-- 彙總 job 取共享鎖（彼此可以並行，以 SKIP LOCKED 分批）；重算與清理取獨占鎖（見 LockProductViewRebuild）。
SELECT pg_advisory_xact_lock_shared(hashtextextended('seller.product_views', 0));

-- name: LockProductViewRebuild :exec
-- 重算與清理取獨占鎖：重算的刪除與重建之間不會有彙總 job 插進來，清理也不會在重算讀事件時刪掉它們。
SELECT pg_advisory_xact_lock(hashtextextended('seller.product_views', 0));

-- name: DeleteProductViewsDaily :exec
-- 重算的第一步：刪區間 [from_day, to_day] 內的每日彙總。與下一句分成兩句：同一句的 CTE 看到同一份 snapshot，刪除還沒生效，
-- 重建會撞主鍵。兩句在同一個 transaction（獨占鎖之下），沒有彙總缺漏的空窗。
DELETE FROM seller.product_views_daily
WHERE day >= sqlc.arg(from_day)::date AND day <= sqlc.arg(to_day)::date;

-- name: InsertRebuiltProductViewsDaily :exec
-- 重算的第二步：以已處理的事件（COUNTED，儲存的 store_id）重建區間 [from_day, to_day] 的每日彙總；日期取自 occurred_at 的
-- 台北日曆日。與解析路徑一樣只認 seller.stores 裡有的店：切換之前已計入的事件，store_id 可能是舊 seller-service 的店號
-- （product_view_events.store_id 沒有 FK），不在 seller.stores 的略過，不讓整個重算被 product_views_daily 的 FK 擋下。
INSERT INTO seller.product_views_daily (store_id, day, views)
SELECT e.store_id, (e.occurred_at AT TIME ZONE 'Asia/Taipei')::date AS day, count(*)
FROM seller.product_view_events e
JOIN seller.stores s ON s.store_id = e.store_id
WHERE e.outcome = 'COUNTED'
  AND (e.occurred_at AT TIME ZONE 'Asia/Taipei')::date >= sqlc.arg(from_day)::date
  AND (e.occurred_at AT TIME ZONE 'Asia/Taipei')::date <= sqlc.arg(to_day)::date
GROUP BY e.store_id, day;

-- name: PurgeProcessedProductViewEvents :one
-- 刪除發生時間早於 cutoff 且已處理的事件（沒處理的不刪：它們還沒進彙總）；一次最多 batch_limit 列，回傳刪除的列數。
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
-- 店家在 [first_day, end_day)（台北日曆日，左閉右開）的瀏覽總數；沒有彙總列的日子是 0。
SELECT COALESCE(SUM(views), 0)::bigint AS views
FROM seller.product_views_daily
WHERE store_id = sqlc.arg(store_id) AND day >= sqlc.arg(first_day)::date AND day < sqlc.arg(end_day)::date;
