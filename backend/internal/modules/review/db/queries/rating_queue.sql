-- 商品評分的待重算佇列（D3）：登記在評價集合改變的 transaction 內，worker 認領、完成、失敗與重排都同時以 seq 與 claim_token 比對。

-- name: RequestRatingRecompute :exec
-- 登記一批商品（呼叫方的 transaction 內，依商品 ID 升序加鎖避免與其他批次互鎖）。沒有列就新增；有列就 seq 加一（處理中的 worker 完成時看到序號變了就重排），
-- 到期時間取較早的；已停手的列（failed_at 非空）被新請求重置成全新的一列。
INSERT INTO review.rating_recompute_queue AS q (product_id, due_at, priority)
SELECT p, sqlc.arg(now)::timestamptz, 0 FROM (SELECT DISTINCT unnest(sqlc.arg(product_ids)::uuid[]) AS p ORDER BY p) AS ids
ON CONFLICT (product_id) DO UPDATE SET
    seq = q.seq + 1,
    due_at = CASE WHEN q.failed_at IS NOT NULL THEN EXCLUDED.due_at ELSE LEAST(q.due_at, EXCLUDED.due_at) END,
    attempts = CASE WHEN q.failed_at IS NOT NULL THEN 0 ELSE q.attempts END,
    last_error = CASE WHEN q.failed_at IS NOT NULL THEN NULL ELSE q.last_error END,
    failed_at = NULL,
    priority = 0;

-- name: RequestRatingReconcileBatch :many
-- 定期全量對帳的一批：評價表裡 product_id 大於游標的商品（升序，最多 max_rows 個）各登記一次（priority 1，排在即時請求之後），回這一批挑到的商品 ID（下一批的游標取其中最大的，不依賴寫入的列數）。
-- 與即時請求不同：已停手的列（failed_at 非空）不動，只有即時請求能重置它；其他列 seq 加一、到期時間取較早的、priority 取較小的。
WITH picked AS (
    SELECT DISTINCT r.product_id FROM review.reviews AS r
    WHERE r.product_id > sqlc.arg(after_id)::uuid
    ORDER BY r.product_id
    LIMIT sqlc.arg(max_rows)),
ins AS (
    INSERT INTO review.rating_recompute_queue AS q (product_id, due_at, priority)
    SELECT p.product_id, sqlc.arg(now)::timestamptz, 1 FROM picked AS p ORDER BY p.product_id
    ON CONFLICT (product_id) DO UPDATE SET
        seq = q.seq + 1,
        due_at = LEAST(q.due_at, EXCLUDED.due_at),
        priority = LEAST(q.priority, EXCLUDED.priority)
    WHERE q.failed_at IS NULL)
SELECT p.product_id FROM picked AS p ORDER BY p.product_id;

-- name: ClaimRatingRecompute :many
-- worker 認領到期、沒被認領（或租約已過）的列：SKIP LOCKED 讓多個 worker 分開，寫租約 lease_until，先 priority 後 due_at，回認領當下的 seq 與 attempts；claim_token 是這次認領的憑證。
WITH picked AS (
    SELECT product_id FROM review.rating_recompute_queue
    WHERE failed_at IS NULL AND due_at <= sqlc.arg(now)::timestamptz AND (claimed_until IS NULL OR claimed_until < sqlc.arg(now)::timestamptz)
    ORDER BY priority, due_at, product_id
    LIMIT sqlc.arg(max_rows)
    FOR UPDATE SKIP LOCKED)
UPDATE review.rating_recompute_queue AS q SET claimed_until = sqlc.arg(lease_until)::timestamptz, claim_token = sqlc.arg(claim_token)::uuid
FROM picked AS p WHERE q.product_id = p.product_id
RETURNING q.product_id, q.seq, q.attempts;

-- name: FinishRatingRecompute :execrows
-- 處理完：只在 seq 還是認領時讀到的值、且 claim_token 還是自己的才刪；回 0 表示處理中又有新請求（seq 變了）或租約已被別人接手，呼叫方改用 Rearm 分辨。
DELETE FROM review.rating_recompute_queue WHERE product_id = sqlc.arg(product_id) AND seq = sqlc.arg(seq) AND claim_token = sqlc.arg(claim_token)::uuid;

-- name: RearmRatingRecompute :execrows
-- 處理中來了新請求：放掉租約與憑證、立即到期，下一輪再處理；失敗次數與最後錯誤保留（新請求不代表失敗原因消失，持續失敗的商品仍會在上限停手）。
-- 只動 claim_token 還是自己的列；回 0 表示租約已被別人接手或列已不在，呼叫方什麼都不做。
UPDATE review.rating_recompute_queue SET claimed_until = NULL, claim_token = NULL, due_at = sqlc.arg(due_at)
WHERE product_id = sqlc.arg(product_id) AND claim_token = sqlc.arg(claim_token)::uuid;

-- name: FailRatingRecompute :execrows
-- 處理失敗：同一列次數加一、退避到 due_at、放掉租約並記錯誤；failed_at 有給就停手（超過上限）。只在 seq 沒變且 claim_token 還是自己的時；回 0 表示處理中有新請求或租約已被接手，呼叫方改用 Rearm 分辨。
UPDATE review.rating_recompute_queue SET attempts = attempts + 1, due_at = sqlc.arg(due_at), claimed_until = NULL, claim_token = NULL, last_error = sqlc.arg(last_error), failed_at = sqlc.narg(failed_at)
WHERE product_id = sqlc.arg(product_id) AND seq = sqlc.arg(seq) AND claim_token = sqlc.arg(claim_token)::uuid;
