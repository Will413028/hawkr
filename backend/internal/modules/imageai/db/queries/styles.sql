-- name: ListEnabledStyles :many
-- 一個用途的啟用中風格，含預設風格（呼叫端把 is_default 那列分出來）；單一語句，預設風格與清單來自同一個快照。依顯示順序，同值依名稱。
SELECT id, name, label, prompt, thumbnail_key, display_order, is_default
FROM imageai.styles
WHERE category = @category AND enabled
ORDER BY display_order, name;

-- name: ListStylesForAdmin :many
-- 後台的清單（含停用的）：category 空字串是全部用途；依顯示順序，同值依名稱、id。
SELECT * FROM imageai.styles
WHERE @category::text = '' OR category = @category::text
ORDER BY display_order, name, id;

-- name: GetStyle :one
SELECT * FROM imageai.styles WHERE id = $1;

-- name: LockStyle :one
-- 鎖住風格列（更新與刪除先鎖，版本與預設的判斷讀同一個快照）。
SELECT * FROM imageai.styles WHERE id = $1 FOR UPDATE;

-- name: InsertStyle :one
-- 新增的風格都不是預設（換預設只有 SetDefaultAiStyle）。
INSERT INTO imageai.styles (
    id, name, label, prompt, category, thumbnail_key, thumbnail_asset_id, display_order, is_default, enabled, created_at, updated_at
) VALUES (@id, @name, @label, @prompt, @category, @thumbnail_key, @thumbnail_asset_id, @display_order, FALSE, @enabled, @now, @now)
RETURNING *;

-- name: UpdateStyle :one
-- 條件式 UPDATE：版本不符時沒有列被更新（pgx.ErrNoRows）；沿用目前的縮圖時 thumbnail_key 傳目前的值、thumbnail_asset_id 傳 NULL。
UPDATE imageai.styles
SET label = @label, prompt = @prompt, thumbnail_key = @thumbnail_key, thumbnail_asset_id = COALESCE(sqlc.narg(thumbnail_asset_id), thumbnail_asset_id),
    display_order = @display_order, enabled = @enabled,
    version = version + 1, updated_at = @now
WHERE id = @id AND version = @expected_version
RETURNING *;

-- name: LockCategoryStyles :many
-- 鎖住一個用途的全部風格列（依 id 排序，固定鎖順序）：換預設的兩個 SetDefault 因此串行，不會撞部分唯一索引；等鎖之後讀到的是最新的列。
SELECT * FROM imageai.styles WHERE category = $1 ORDER BY id FOR UPDATE;

-- name: ClearDefaultStyle :many
-- 把用途目前的預設風格（exclude_id 以外）取消預設；換預設時先清舊的再標新的（部分唯一索引逐列檢查）。
UPDATE imageai.styles SET is_default = FALSE, version = version + 1, updated_at = @now
WHERE category = @category AND is_default AND id <> @exclude_id
RETURNING id;

-- name: MarkStyleDefault :one
-- 把風格標成預設（呼叫端已鎖住用途的全部列並清掉舊的）。
UPDATE imageai.styles SET is_default = TRUE, version = version + 1, updated_at = @now
WHERE id = @id
RETURNING *;

-- name: DeleteStyle :execrows
DELETE FROM imageai.styles WHERE id = $1;

-- name: LockStyleThumbnailAsset :one
-- 鎖住風格並回傳目前的縮圖資產（沒有縮圖是 NULL；換圖與刪除要把被換掉的標記刪除）。
SELECT thumbnail_asset_id FROM imageai.styles WHERE id = $1 FOR UPDATE;
