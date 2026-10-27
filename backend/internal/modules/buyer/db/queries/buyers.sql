-- name: Current :one
-- 讀取帳號目前存活的買家 stint。
SELECT *
FROM buyer.buyers
WHERE account_id = $1 AND status <> 'CLOSED';

-- name: CurrentBuyerID :one
SELECT buyer_id
FROM buyer.buyers
WHERE account_id = $1 AND status <> 'CLOSED';

-- name: ByID :one
-- 讀取指定 stint，包含已關閉資料。
SELECT *
FROM buyer.buyers
WHERE buyer_id = $1;

-- name: LockBuyer :one
-- 鎖定指定 stint，序列化其業務寫入。
SELECT *
FROM buyer.buyers
WHERE buyer_id = $1
FOR UPDATE;

-- name: LockBuyerShared :one
-- 以共享鎖讀取指定 stint：與 BeginDeletion 的 FOR UPDATE 互斥，與別的共享鎖不互斥。其他模組的買家寫入用它與刪帳序列化。
SELECT *
FROM buyer.buyers
WHERE buyer_id = $1
FOR SHARE;

-- name: CreateBuyer :one
-- 建帳時建立帳號唯一的買家；同一帳號再建立違反 buyers_account，視為不變量被破壞。
INSERT INTO buyer.buyers (account_id, display_name, created_at, updated_at)
VALUES ($1, $2, $3, $3)
RETURNING *;

-- name: UpdateProfile :one
-- 寫入已在列鎖內合併的 profile 欄位。
UPDATE buyer.buyers
SET display_name = $2,
    phone = $3,
    gender = $4,
    birth_date = $5,
    city = $6,
    job = $7,
    updated_at = $8,
    version = version + 1
WHERE buyer_id = $1 AND version = $9
RETURNING *;

-- name: UpdateAvatar :one
-- 寫入已轉正的頭像 object key。
UPDATE buyer.buyers
SET avatar = sqlc.arg(avatar),
    avatar_asset_id = sqlc.arg(avatar_asset_id),
    updated_at = sqlc.arg(updated_at),
    version = version + 1
WHERE buyer_id = sqlc.arg(buyer_id) AND version = sqlc.arg(version)
RETURNING *;

-- name: AvatarAsset :one
-- 同步 job 讀的：買家目前的頭像 asset（沒有頭像時是 NULL）。
SELECT status, avatar_asset_id
FROM buyer.buyers
WHERE buyer_id = $1;

-- name: BatchProfiles :many
-- 批次讀取未關閉且未清除個資的 profile。
SELECT *
FROM buyer.buyers
WHERE buyer_id = ANY($1::bigint[]) AND status <> 'CLOSED' AND pii_cleared_at IS NULL
ORDER BY buyer_id;

-- name: BatchDisplay :many
-- 回傳原始 nullable 個資與 Deleted；模組對已關閉 stint 隱藏個資。
SELECT buyer_id, display_name, avatar, (status = 'CLOSED')::boolean AS deleted
FROM buyer.buyers
WHERE buyer_id = ANY($1::bigint[])
ORDER BY buyer_id;

-- name: Principals :many
-- 只解析存活的 stint；未回傳的 ID 由呼叫方列為 unresolved。
SELECT buyer_id, account_id
FROM buyer.buyers
WHERE buyer_id = ANY($1::bigint[]) AND status <> 'CLOSED'
ORDER BY buyer_id;

-- name: BuyerIDs :many
-- 列出帳號所有歷史與目前 stint。
SELECT buyer_id
FROM buyer.buyers
WHERE account_id = $1
ORDER BY buyer_id;

-- name: Stats :one
-- 計算目前存活的買家數量。
SELECT count(*)
FROM buyer.buyers
WHERE status <> 'CLOSED';

-- name: Pending :exec
-- 將買家標記為等待訂單結束。
UPDATE buyer.buyers
SET status = 'PENDING_DELETION',
    updated_at = $2,
    version = version + 1
WHERE buyer_id = $1;

-- name: Close :exec
-- 關閉 stint 並釋放 current 位置。
UPDATE buyer.buyers
SET status = 'CLOSED',
    deleted_at = $2,
    updated_at = $2,
    version = version + 1
WHERE buyer_id = $1;

-- name: CleanupBatch :many
-- 按 buyer_id 鎖定已關閉超過保留期限的個資批次。
SELECT buyer_id
FROM buyer.buyers
WHERE status = 'CLOSED' AND deleted_at < $1 AND pii_cleared_at IS NULL
ORDER BY buyer_id
LIMIT $2
FOR UPDATE SKIP LOCKED;

-- name: ClearPII :exec
-- 只清理指定 closed stint 的個資。
UPDATE buyer.buyers
SET display_name = NULL,
    phone = NULL,
    gender = NULL,
    avatar = NULL,
    avatar_asset_id = NULL,
    birth_date = NULL,
    city = NULL,
    job = NULL,
    pii_cleared_at = $2,
    updated_at = $2,
    version = version + 1
WHERE buyer_id = $1 AND status = 'CLOSED' AND pii_cleared_at IS NULL;
