-- 商品評分的待重算佇列（D3）：登記在評價集合改變的 transaction 內，worker 認領、完成、失敗與重排都以 seq 比對。

-- name: RequestRatingRecompute :exec
-- 登記一批商品（呼叫方的 transaction 內，依商品 ID 升序加鎖避免與其他批次互鎖）。沒有列就新增；有列就 seq 加一（處理中的 worker 完成時看到序號變了就重排），
-- 到期時間取較早的；已停手的列（failed_at 非空）被新請求重置成全新的一列。
INSERT INTO review.rating_recompute_queue AS q (product_id, due_at)
SELECT p, sqlc.arg(now)::timestamptz FROM (SELECT DISTINCT unnest(sqlc.arg(product_ids)::uuid[]) AS p ORDER BY p) AS ids
ON CONFLICT (product_id) DO UPDATE SET
    seq = q.seq + 1,
    due_at = CASE WHEN q.failed_at IS NOT NULL THEN EXCLUDED.due_at ELSE LEAST(q.due_at, EXCLUDED.due_at) END,
    attempts = CASE WHEN q.failed_at IS NOT NULL THEN 0 ELSE q.attempts END,
    last_error = CASE WHEN q.failed_at IS NOT NULL THEN NULL ELSE q.last_error END,
    failed_at = NULL;

-- name: RequestRatingReconcileBatch :many
-- 定期全量對帳的一批：評價表裡 product_id 大於游標的商品（升序，最多 max_rows 個）各登記一次，語意同 RequestRatingRecompute；回登記到的商品 ID，最後一個就是下一批的游標。
WITH picked AS (
    SELECT DISTINCT r.product_id FROM review.reviews AS r
    WHERE r.product_id > sqlc.arg(after_id)::uuid
    ORDER BY r.product_id
    LIMIT sqlc.arg(max_rows))
INSERT INTO review.rating_recompute_queue AS q (product_id, due_at)
SELECT p.product_id, sqlc.arg(now)::timestamptz FROM picked AS p ORDER BY p.product_id
ON CONFLICT (product_id) DO UPDATE SET
    seq = q.seq + 1,
    due_at = CASE WHEN q.failed_at IS NOT NULL THEN EXCLUDED.due_at ELSE LEAST(q.due_at, EXCLUDED.due_at) END,
    attempts = CASE WHEN q.failed_at IS NOT NULL THEN 0 ELSE q.attempts END,
    last_error = CASE WHEN q.failed_at IS NOT NULL THEN NULL ELSE q.last_error END,
    failed_at = NULL
RETURNING q.product_id;

-- name: ClaimRatingRecompute :many
-- worker 認領到期、沒被認領（或租約已過）的列：SKIP LOCKED 讓多個 worker 分開，寫租約 lease_until，回認領當下的 seq 與 attempts。
WITH picked AS (
    SELECT product_id FROM review.rating_recompute_queue
    WHERE failed_at IS NULL AND due_at <= sqlc.arg(now)::timestamptz AND (claimed_until IS NULL OR claimed_until < sqlc.arg(now)::timestamptz)
    ORDER BY due_at, product_id
    LIMIT sqlc.arg(max_rows)
    FOR UPDATE SKIP LOCKED)
UPDATE review.rating_recompute_queue AS q SET claimed_until = sqlc.arg(lease_until)::timestamptz
FROM picked AS p WHERE q.product_id = p.product_id
RETURNING q.product_id, q.seq, q.attempts;

-- name: FinishRatingRecompute :execrows
-- 處理完：只在 seq 還是認領時讀到的值才刪；回 0 表示處理中又有新請求（或列已被別人處理掉），呼叫方要重排。
DELETE FROM review.rating_recompute_queue WHERE product_id = sqlc.arg(product_id) AND seq = sqlc.arg(seq);

-- name: RearmRatingRecompute :exec
-- 處理中來了新請求：放掉租約、立即到期、清失敗次數，下一輪再處理（只動還被認領的列）。
UPDATE review.rating_recompute_queue SET claimed_until = NULL, due_at = sqlc.arg(due_at), attempts = 0, last_error = NULL
WHERE product_id = sqlc.arg(product_id) AND claimed_until IS NOT NULL;

-- name: FailRatingRecompute :execrows
-- 處理失敗：同一列次數加一、退避到 due_at、放掉租約並記錯誤；failed_at 有給就停手（超過上限）。只在 seq 沒變時；回 0 表示處理中有新請求，呼叫方改用 Rearm。
UPDATE review.rating_recompute_queue SET attempts = attempts + 1, due_at = sqlc.arg(due_at), claimed_until = NULL, last_error = sqlc.arg(last_error), failed_at = sqlc.narg(failed_at)
WHERE product_id = sqlc.arg(product_id) AND seq = sqlc.arg(seq);
