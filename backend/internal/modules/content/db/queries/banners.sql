-- name: ListBanners :many
-- 後台的清單，依顯示順序。keyset 分頁：has_cursor 時從 (after_order, after_created, after_id) 之後開始。
SELECT * FROM content.banners
WHERE NOT sqlc.arg(has_cursor)::bool
   OR (display_order, created_at, id) > (sqlc.arg(after_order)::int, sqlc.arg(after_created)::timestamptz, sqlc.arg(after_id)::uuid)
ORDER BY display_order, created_at, id
LIMIT sqlc.arg(max_rows);

-- name: ListLiveBanners :many
-- 前台顯示的 banner：啟用中，而且 now 落在排程區間 [start_date, end_date) 內（沒設的一端不設限）。
-- 分頁同 ListBanners。
SELECT * FROM content.banners
WHERE is_active
  AND (start_date IS NULL OR start_date <= sqlc.arg(now)::timestamptz)
  AND (end_date IS NULL OR end_date > sqlc.arg(now)::timestamptz)
  AND (NOT sqlc.arg(has_cursor)::bool
       OR (display_order, created_at, id) > (sqlc.arg(after_order)::int, sqlc.arg(after_created)::timestamptz, sqlc.arg(after_id)::uuid))
ORDER BY display_order, created_at, id
LIMIT sqlc.arg(max_rows);

-- name: CountActiveBanners :one
-- 啟用中的 banner 數量，不含 except_id（新增時傳 uuid 的零值）。
SELECT count(*) FROM content.banners WHERE is_active AND id <> sqlc.arg(except_id)::uuid;

-- name: GetBanner :one
SELECT * FROM content.banners WHERE id = $1;

-- name: InsertBanner :one
INSERT INTO content.banners (
    id, title, button_text, preview_image_key, popup_image_key,
    canvas_json, action_type, action_target, display_order, is_active, start_date, end_date,
    preview_image_asset_id, popup_image_asset_id
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
RETURNING *;

-- name: UpdateBanner :one
-- 條件式 UPDATE：版本不符時沒有列被更新（pgx.ErrNoRows），由呼叫方分辨不存在還是版本衝突。
UPDATE content.banners
SET title             = $2,
    button_text       = $3,
    preview_image_key = $4,
    popup_image_key   = $5,
    preview_image_asset_id = COALESCE(sqlc.narg(preview_image_asset_id), preview_image_asset_id),
    popup_image_asset_id   = COALESCE(sqlc.narg(popup_image_asset_id), popup_image_asset_id),
    canvas_json       = $6,
    action_type       = $7,
    action_target     = $8,
    display_order     = $9,
    is_active         = $10,
    start_date        = $11,
    end_date          = $12,
    version           = version + 1,
    updated_at        = NOW()
WHERE id = $1 AND version = sqlc.arg(expected_version)
RETURNING *;

-- name: LockBannerAssets :one
-- 鎖住 banner 並回傳它目前的兩張圖的 asset ID（換圖與刪除要把被換掉的記成退場）。
SELECT preview_image_asset_id, popup_image_asset_id FROM content.banners WHERE id = $1 FOR UPDATE;

-- name: DeleteBanner :execrows
-- expected_version 為 0 表示不檢查版本。
DELETE FROM content.banners
WHERE id = $1 AND (sqlc.arg(expected_version)::bigint = 0 OR version = sqlc.arg(expected_version)::bigint);

-- name: BannerAssets :one
-- 同步 job 讀的：banner 目前的兩張圖（沒有列時 pgx.ErrNoRows，也就是終態）。
SELECT preview_image_asset_id, popup_image_asset_id FROM content.banners WHERE id = $1;
