-- 貼文的寫入與店主的讀取。帶「公開可讀」規則的讀取（單篇、動態牆、店家頁、標籤、熱門、建議）是 go-jet 建的，在 internal/reads，規則在
-- internal/visibility。

-- name: InsertPost :one
-- 同一家店的同一個 request_id 已經有時不寫入、回 0 列（不讓 transaction 失效，呼叫方接著讀原本那一篇）。
INSERT INTO social.posts (id, store_id, body, image_key, image_asset_id, product_id, status, published_at, created_at, updated_at, request_id, request_hash)
VALUES (
    sqlc.arg(id), sqlc.arg(store_id), sqlc.arg(body), sqlc.narg(image_key), sqlc.narg(image_asset_id), sqlc.narg(product_id),
    sqlc.arg(status), sqlc.narg(published_at), sqlc.arg(now), sqlc.arg(now), sqlc.narg(request_id), sqlc.narg(request_hash)
)
ON CONFLICT (store_id, request_id) DO NOTHING
RETURNING *;

-- name: PostByRequest :one
-- 冪等重送的查詢：範圍是 (店家, request_id)；含已刪除的貼文（request_id 仍占著）。
SELECT * FROM social.posts WHERE store_id = sqlc.arg(store_id) AND request_id = sqlc.arg(request_id);

-- name: CountStorePostsByStatus :many
-- 店家的貼文依狀態的篇數（不含已刪除）；沒有貼文的狀態沒有列，呼叫方補 0。
SELECT status, count(*)::bigint AS posts FROM social.posts WHERE store_id = sqlc.arg(store_id) AND deleted_at IS NULL GROUP BY status;

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

-- name: BumpPostVersion :one
-- 只遞增版本（排程與取消排程改變貼文的狀態組合，舊的 etag 隨之失效）。版本不符回 0 列。
UPDATE social.posts
SET version = version + 1, updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND deleted_at IS NULL AND version = sqlc.arg(expected_version)
RETURNING *;

-- name: BumpPostVersions :exec
-- 系統取消排程時遞增這些貼文的版本（不比對版本；呼叫方已鎖貼文列）。已軟刪或不存在的不動。
UPDATE social.posts
SET version = version + 1, updated_at = sqlc.arg(now)
WHERE id = ANY(sqlc.arg(ids)::uuid[]) AND deleted_at IS NULL;

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


-- name: ListAdminPosts :many
-- 後台看貼文（不含草稿與刪除）：最新建立的在前。taken_down 為真只列有內容下架複本的貼文，為假不篩選。keyset：(created_at, id) 由大到小。
SELECT sqlc.embed(p), sqlc.embed(s)
FROM social.posts p
JOIN social.post_stats s ON s.post_id = p.id
WHERE p.deleted_at IS NULL AND p.status <> 'DRAFT'
  AND (NOT sqlc.arg(taken_down)::boolean OR EXISTS (SELECT 1 FROM social.content_takedowns t WHERE t.post_id = p.id))
  AND (p.created_at, p.id) < (sqlc.arg(after_created)::timestamptz, sqlc.arg(after_id)::uuid)
ORDER BY p.created_at DESC, p.id DESC
LIMIT sqlc.arg(max_rows);
