-- 貼文的寫入與店主的讀取。帶「公開可讀」規則的讀取（單篇、動態牆、店家頁、標籤、熱門、建議）是 go-jet 建的，在 internal/reads，規則在
-- internal/visibility。

-- name: InsertPost :one
INSERT INTO social.posts (id, store_id, body, image_key, image_asset_id, product_id, status, published_at, created_at, updated_at)
VALUES (
    sqlc.arg(id), sqlc.arg(store_id), sqlc.arg(body), sqlc.narg(image_key), sqlc.narg(image_asset_id), sqlc.narg(product_id),
    sqlc.arg(status), sqlc.narg(published_at), sqlc.arg(now), sqlc.arg(now)
)
RETURNING *;

-- name: LockPost :one
-- 鎖住貼文並回傳目前的狀態與圖（換圖、刪除與狀態轉移要依它判斷）；已刪除的貼文不回。
SELECT * FROM social.posts WHERE id = sqlc.arg(id) AND deleted_at IS NULL FOR UPDATE;

-- name: UpdatePost :one
-- 版本不符回 0 列。圖：換新（new_image_key、new_image_asset_id 有值）、清掉（clear_image）或沿用。
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
-- published_at 固定在第一次發布：轉成 PUBLISHED 時只在它還是 NULL 時設成現在，下架再發布沿用原值（動態牆的順序不能靠下架再發布來洗）；
-- 其他轉移不動它。已經是 PUBLISHED 時再發布是 Go 端的 no-op，不會呼叫這支。
UPDATE social.posts
SET status = sqlc.arg(status),
    published_at = CASE WHEN sqlc.arg(status)::text = 'PUBLISHED' THEN COALESCE(published_at, sqlc.arg(now)::timestamptz) ELSE published_at END,
    version = version + 1,
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND deleted_at IS NULL AND version = sqlc.arg(expected_version)
RETURNING *;

-- name: SoftDeletePost :execrows
-- 軟刪；已經刪除的 0 列（呼叫方不做副作用）。expected_version 為 0 表示不檢查版本。
UPDATE social.posts
SET deleted_at = sqlc.arg(now), updated_at = sqlc.arg(now), version = version + 1
WHERE id = sqlc.arg(id) AND deleted_at IS NULL AND (sqlc.arg(expected_version)::bigint = 0 OR version = sqlc.arg(expected_version));

-- name: GetPost :one
-- 任何狀態、沒有刪除的貼文與它的計數（店主與模組內部用；公開讀取走 GetPublicPost）。
SELECT sqlc.embed(p), sqlc.embed(s)
FROM social.posts p
JOIN social.post_stats s ON s.post_id = p.id
WHERE p.id = sqlc.arg(id) AND p.deleted_at IS NULL;

-- name: ListOwnPosts :many
-- 店主看自己的貼文（含草稿與下架，不含刪除）：最新建立的在前。status 空字串不篩選。keyset：(created_at, id) 由大到小。
SELECT sqlc.embed(p), sqlc.embed(s)
FROM social.posts p
JOIN social.post_stats s ON s.post_id = p.id
WHERE p.store_id = sqlc.arg(store_id) AND p.deleted_at IS NULL
  AND (sqlc.arg(status)::text = '' OR p.status = sqlc.arg(status)::text)
  AND (p.created_at, p.id) < (sqlc.arg(after_created)::timestamptz, sqlc.arg(after_id)::uuid)
ORDER BY p.created_at DESC, p.id DESC
LIMIT sqlc.arg(max_rows);

-- name: DeletePostHashtags :exec
-- 整批替換貼文的標籤（值的集合）：先刪再寫（兩個語句，同一個 transaction）。
DELETE FROM social.post_hashtags WHERE post_id = sqlc.arg(post_id);

-- name: InsertPostHashtag :exec
INSERT INTO social.post_hashtags (post_id, position, name, display_name)
VALUES (sqlc.arg(post_id), sqlc.arg(position), sqlc.arg(name), sqlc.arg(display_name));

-- name: ListHashtagsOfPosts :many
SELECT post_id, position, name, display_name
FROM social.post_hashtags
WHERE post_id = ANY(sqlc.arg(post_ids)::uuid[])
ORDER BY post_id, position;

