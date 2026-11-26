-- 補貨：SKU 由不可買變可買（有庫存、商品 ACTIVE 且未刪、SKU 未刪、店家在 at 可曝光）的偵測、執行時重新確認，與補貨、清訂閱兩張佇列表（D26）。

-- name: GetSkuRestockState :one
-- 一個 SKU 此刻的庫存與可買與否，加上通知用的最新快照（商品名稱、主圖 key）。SKU 已刪除也回列（歸還不看刪除），purchasable 為 false。
SELECT s.product_id, p.store_id, p.title, s.stock_qty,
       COALESCE((SELECT i.object_key FROM catalog.product_images i WHERE i.product_id = p.id AND i.kind = 'GALLERY' ORDER BY i.position LIMIT 1), '')::text AS cover_key,
       (s.deleted_at IS NULL AND s.stock_qty > 0 AND p.deleted_at IS NULL AND p.status = 'ACTIVE'
        AND p.store_id IN (SELECT e.store_id FROM seller.pub_exposable_stores(sqlc.arg(at)::timestamptz) e))::boolean AS purchasable
FROM catalog.product_skus s JOIN catalog.products p ON p.id = s.product_id
WHERE s.id = sqlc.arg(id);


-- name: EnqueueRestock :exec
-- 排一個 SKU 的請求（呼叫方的 transaction 內）。沒有列就新增；有列且沒被認領就不動（合併，保留較早的 due_at）；
-- 被認領處理中（claimed_until 非空）只把 requested_at 往前推（至少加 1 微秒，時鐘不動也一定不同），worker 完成時比對它而不是刪列，所以處理中的新轉換不會遺失；
-- 已停手的列（failed_at 非空）被新請求重置成全新的一列。
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
-- worker 認領到期、沒被認領（或租約已過）的列：SKIP LOCKED 讓多個 worker 分開，寫租約 lease_until，回認領當下的 requested_at 與 attempts。
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
-- 處理完（成功或不需要通知）：只在 requested_at 還是認領時讀到的值才刪；回 0 表示處理中又有新請求（或列已被別人處理掉）。
DELETE FROM catalog.restock_queue WHERE sku_id = sqlc.arg(sku_id) AND requested_at = sqlc.arg(requested_at);

-- name: RearmRestock :exec
-- 處理中來了新請求：放掉租約、重新計 Delay 與失敗次數，下一輪再處理（只動還被認領的列）。
UPDATE catalog.restock_queue SET claimed_until = NULL, due_at = sqlc.arg(due_at), attempts = 0, last_error = NULL
WHERE sku_id = sqlc.arg(sku_id) AND claimed_until IS NOT NULL;

-- name: FailRestock :execrows
-- 處理失敗：同一列次數加一、退避到 due_at、放掉租約並記錯誤；failed_at 有給就停手（超過上限）。只在 requested_at 沒變時；回 0 表示處理中有新請求，呼叫方改用 Rearm。
UPDATE catalog.restock_queue SET attempts = attempts + 1, due_at = sqlc.arg(due_at), claimed_until = NULL, last_error = sqlc.arg(last_error), failed_at = sqlc.narg(failed_at)
WHERE sku_id = sqlc.arg(sku_id) AND requested_at = sqlc.arg(requested_at);

-- name: EnqueuePurge :exec
-- 排一個 SKU 的請求（呼叫方的 transaction 內）。沒有列就新增；有列且沒被認領就不動（合併，保留較早的 due_at）；
-- 被認領處理中（claimed_until 非空）只把 requested_at 往前推（至少加 1 微秒，時鐘不動也一定不同），worker 完成時比對它而不是刪列，所以處理中的新轉換不會遺失；
-- 已停手的列（failed_at 非空）被新請求重置成全新的一列。
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
-- worker 認領到期、沒被認領（或租約已過）的列：SKIP LOCKED 讓多個 worker 分開，寫租約 lease_until，回認領當下的 requested_at 與 attempts。
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
-- 處理完（成功或不需要通知）：只在 requested_at 還是認領時讀到的值才刪；回 0 表示處理中又有新請求（或列已被別人處理掉）。
DELETE FROM catalog.restock_purge_queue WHERE sku_id = sqlc.arg(sku_id) AND requested_at = sqlc.arg(requested_at);

-- name: RearmPurge :exec
-- 處理中來了新請求：放掉租約、重新計 Delay 與失敗次數，下一輪再處理（只動還被認領的列）。
UPDATE catalog.restock_purge_queue SET claimed_until = NULL, due_at = sqlc.arg(due_at), attempts = 0, last_error = NULL
WHERE sku_id = sqlc.arg(sku_id) AND claimed_until IS NOT NULL;

-- name: FailPurge :execrows
-- 處理失敗：同一列次數加一、退避到 due_at、放掉租約並記錯誤；failed_at 有給就停手（超過上限）。只在 requested_at 沒變時；回 0 表示處理中有新請求，呼叫方改用 Rearm。
UPDATE catalog.restock_purge_queue SET attempts = attempts + 1, due_at = sqlc.arg(due_at), claimed_until = NULL, last_error = sqlc.arg(last_error), failed_at = sqlc.narg(failed_at)
WHERE sku_id = sqlc.arg(sku_id) AND requested_at = sqlc.arg(requested_at);

-- name: EnqueueRestockOfProduct :exec
-- 商品切成 ACTIVE：它每一個沒刪除的 SKU 都排一列，不看庫存（庫存與可買由 worker 執行時重新確認），所以不需要在這個 transaction 讀別的 SKU 的庫存，
-- 也就不會漏掉與另一個 SKU 的 0 到正庫存變更並行時雙方都看不到對方的情況。語意同 EnqueueRestock。
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
