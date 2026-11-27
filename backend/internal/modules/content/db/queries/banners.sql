-- name: ListBanners :many
SELECT * FROM content.banners
WHERE NOT sqlc.arg(has_cursor)::bool
   OR (display_order, created_at, id) > (sqlc.arg(after_order)::int, sqlc.arg(after_created)::timestamptz, sqlc.arg(after_id)::uuid)
ORDER BY display_order, created_at, id
LIMIT sqlc.arg(max_rows);

-- name: ListLiveBanners :many
SELECT * FROM content.banners
WHERE is_active
  AND (start_date IS NULL OR start_date <= sqlc.arg(now)::timestamptz)
  AND (end_date IS NULL OR end_date > sqlc.arg(now)::timestamptz)
  AND (NOT sqlc.arg(has_cursor)::bool
       OR (display_order, created_at, id) > (sqlc.arg(after_order)::int, sqlc.arg(after_created)::timestamptz, sqlc.arg(after_id)::uuid))
ORDER BY display_order, created_at, id
LIMIT sqlc.arg(max_rows);

-- name: CountActiveBanners :one
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
SELECT preview_image_asset_id, popup_image_asset_id FROM content.banners WHERE id = $1 FOR UPDATE;

-- name: DeleteBanner :execrows
DELETE FROM content.banners
WHERE id = $1 AND (sqlc.arg(expected_version)::bigint = 0 OR version = sqlc.arg(expected_version)::bigint);

-- name: BannerAssets :one
SELECT preview_image_asset_id, popup_image_asset_id FROM content.banners WHERE id = $1;
