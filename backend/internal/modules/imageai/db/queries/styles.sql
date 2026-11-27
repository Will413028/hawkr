-- name: ListEnabledStyles :many
SELECT id, name, label, prompt, thumbnail_key, display_order, is_default
FROM imageai.styles
WHERE category = @category AND enabled
ORDER BY display_order, name;

-- name: ListStylesForAdmin :many
SELECT * FROM imageai.styles
WHERE @category::text = '' OR category = @category::text
ORDER BY display_order, name, id;

-- name: GetStyle :one
SELECT * FROM imageai.styles WHERE id = $1;

-- name: LockStyle :one
SELECT * FROM imageai.styles WHERE id = $1 FOR UPDATE;

-- name: InsertStyle :one
INSERT INTO imageai.styles (
    id, name, label, prompt, category, thumbnail_key, thumbnail_asset_id, display_order, is_default, enabled, created_at, updated_at
) VALUES (@id, @name, @label, @prompt, @category, @thumbnail_key, @thumbnail_asset_id, @display_order, FALSE, @enabled, @now, @now)
RETURNING *;

-- name: UpdateStyle :one
UPDATE imageai.styles
SET label = @label, prompt = @prompt, thumbnail_key = @thumbnail_key, thumbnail_asset_id = COALESCE(sqlc.narg(thumbnail_asset_id), thumbnail_asset_id),
    display_order = @display_order, enabled = @enabled,
    version = version + 1, updated_at = @now
WHERE id = @id AND version = @expected_version
RETURNING *;

-- name: LockCategoryStyles :many
SELECT * FROM imageai.styles WHERE category = $1 ORDER BY id FOR UPDATE;

-- name: ClearDefaultStyle :many
UPDATE imageai.styles SET is_default = FALSE, version = version + 1, updated_at = @now
WHERE category = @category AND is_default AND id <> @exclude_id
RETURNING id;

-- name: MarkStyleDefault :one
UPDATE imageai.styles SET is_default = TRUE, version = version + 1, updated_at = @now
WHERE id = @id
RETURNING *;

-- name: DeleteStyle :execrows
DELETE FROM imageai.styles WHERE id = $1;

-- name: LockStyleThumbnailAsset :one
SELECT thumbnail_asset_id FROM imageai.styles WHERE id = $1 FOR UPDATE;
