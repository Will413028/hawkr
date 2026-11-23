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
INSERT INTO imageai.styles (
    id, name, label, prompt, category, thumbnail_key, thumbnail_asset_id, display_order, is_default, enabled, created_at, updated_at
) VALUES (@id, @name, @label, @prompt, @category, @thumbnail_key, @thumbnail_asset_id, @display_order, @is_default, @enabled, @now, @now)
RETURNING *;

-- name: UpdateStyle :one
-- 條件式 UPDATE：版本不符時沒有列被更新（pgx.ErrNoRows）；缺圖的 token 為空時 thumbnail_* 沿用（呼叫端傳目前的值）。
UPDATE imageai.styles
SET label = @label, prompt = @prompt, thumbnail_key = @thumbnail_key, thumbnail_asset_id = @thumbnail_asset_id,
    display_order = @display_order, is_default = @is_default, enabled = @enabled,
    version = version + 1, updated_at = @now
WHERE id = @id AND version = @expected_version
RETURNING *;

-- name: ClearDefaultStyle :many
-- 把用途目前的預設風格（exclude_id 以外）取消預設；換預設時先清舊的再標新的（部分唯一索引逐列檢查）。
UPDATE imageai.styles SET is_default = FALSE, version = version + 1, updated_at = @now
WHERE category = @category AND is_default AND id <> @exclude_id
RETURNING id;

-- name: DeleteStyle :execrows
DELETE FROM imageai.styles WHERE id = $1;

-- name: StyleThumbnailAsset :one
-- 同步 job 讀的：風格目前的縮圖資產（沒有列時 pgx.ErrNoRows，也就是終態；沒有縮圖是空字串）。
SELECT thumbnail_asset_id FROM imageai.styles WHERE id = $1;
