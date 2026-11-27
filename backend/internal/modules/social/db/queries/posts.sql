
-- name: InsertPost :one
INSERT INTO social.posts (id, store_id, body, image_key, image_asset_id, product_id, status, published_at, created_at, updated_at, request_id, request_hash)
VALUES (
    sqlc.arg(id), sqlc.arg(store_id), sqlc.arg(body), sqlc.narg(image_key), sqlc.narg(image_asset_id), sqlc.narg(product_id),
    sqlc.arg(status), sqlc.narg(published_at), sqlc.arg(now), sqlc.arg(now), sqlc.narg(request_id), sqlc.narg(request_hash)
)
ON CONFLICT (store_id, request_id) DO NOTHING
RETURNING *;

-- name: PostByRequest :one
SELECT * FROM social.posts WHERE store_id = sqlc.arg(store_id) AND request_id = sqlc.arg(request_id);

-- name: CountStorePostsByStatus :many
SELECT status, count(*)::bigint AS posts FROM social.posts WHERE store_id = sqlc.arg(store_id) AND deleted_at IS NULL GROUP BY status;

-- name: LockPost :one
SELECT * FROM social.posts WHERE id = sqlc.arg(id) AND deleted_at IS NULL FOR UPDATE;

-- name: UpdatePost :one
UPDATE social.posts
SET body = sqlc.arg(body),
    image_key = CASE WHEN sqlc.arg(clear_image)::boolean THEN NULL ELSE COALESCE(sqlc.narg(new_image_key), image_key) END,
    image_asset_id = CASE WHEN sqlc.arg(clear_image)::boolean THEN NULL ELSE COALESCE(sqlc.narg(new_image_asset_id), image_asset_id) END,
    product_id = sqlc.narg(product_id),
    version = version + 1,
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND deleted_at IS NULL AND version = sqlc.arg(expected_version)
RETURNING *;

-- name: SetPostStatus :one
UPDATE social.posts
SET status = sqlc.arg(status),
    published_at = CASE WHEN sqlc.arg(status)::text = 'PUBLISHED' THEN COALESCE(published_at, sqlc.arg(now)::timestamptz) ELSE published_at END,
    version = version + 1,
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND deleted_at IS NULL AND version = sqlc.arg(expected_version)
RETURNING *;

-- name: BumpPostVersion :one
UPDATE social.posts
SET version = version + 1, updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND deleted_at IS NULL AND version = sqlc.arg(expected_version)
RETURNING *;

-- name: BumpPostVersions :exec
UPDATE social.posts
SET version = version + 1, updated_at = sqlc.arg(now)
WHERE id = ANY(sqlc.arg(ids)::uuid[]) AND deleted_at IS NULL;

-- name: SoftDeletePost :execrows
UPDATE social.posts
SET deleted_at = sqlc.arg(now), updated_at = sqlc.arg(now), version = version + 1
WHERE id = sqlc.arg(id) AND deleted_at IS NULL AND (sqlc.arg(expected_version)::bigint = 0 OR version = sqlc.arg(expected_version));

-- name: GetPost :one
SELECT sqlc.embed(p), sqlc.embed(s)
FROM social.posts p
JOIN social.post_stats s ON s.post_id = p.id
WHERE p.id = sqlc.arg(id) AND p.deleted_at IS NULL;

-- name: ListOwnPosts :many
SELECT sqlc.embed(p), sqlc.embed(s)
FROM social.posts p
JOIN social.post_stats s ON s.post_id = p.id
WHERE p.store_id = sqlc.arg(store_id) AND p.deleted_at IS NULL
  AND (sqlc.arg(status)::text = '' OR p.status = sqlc.arg(status)::text)
  AND (p.created_at, p.id) < (sqlc.arg(after_created)::timestamptz, sqlc.arg(after_id)::uuid)
ORDER BY p.created_at DESC, p.id DESC
LIMIT sqlc.arg(max_rows);

-- name: DeletePostHashtags :exec
DELETE FROM social.post_hashtags WHERE post_id = sqlc.arg(post_id);

-- name: InsertPostHashtag :exec
INSERT INTO social.post_hashtags (post_id, position, name, display_name)
VALUES (sqlc.arg(post_id), sqlc.arg(position), sqlc.arg(name), sqlc.arg(display_name));

-- name: ListHashtagsOfPosts :many
SELECT post_id, position, name, display_name
FROM social.post_hashtags
WHERE post_id = ANY(sqlc.arg(post_ids)::uuid[])
ORDER BY post_id, position;


-- name: ListAdminPosts :many
SELECT sqlc.embed(p), sqlc.embed(s)
FROM social.posts p
JOIN social.post_stats s ON s.post_id = p.id
WHERE p.deleted_at IS NULL AND p.status <> 'DRAFT'
  AND (NOT sqlc.arg(taken_down)::boolean OR EXISTS (SELECT 1 FROM social.content_takedowns t WHERE t.post_id = p.id))
  AND (p.created_at, p.id) < (sqlc.arg(after_created)::timestamptz, sqlc.arg(after_id)::uuid)
ORDER BY p.created_at DESC, p.id DESC
LIMIT sqlc.arg(max_rows);
