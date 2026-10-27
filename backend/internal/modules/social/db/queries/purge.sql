-- 買家刪帳與店家退場的清除 job 用的批次寫入。可見性在 job 跑之前已經由 pub_* 在讀取時擋住；這裡是實體清除，冪等：以「還沒刪除」與
-- 「還存在」為述詞，重跑只處理剩下的。

-- name: LockBuyerCommentBatch :many
SELECT id, post_id, parent_id FROM social.comments
WHERE actor_kind = 'BUYER' AND actor_id = sqlc.arg(buyer_id) AND deleted_at IS NULL
ORDER BY id
LIMIT sqlc.arg(max_rows)
FOR UPDATE;

-- name: SoftDeleteCommentBatch :exec
UPDATE social.comments SET deleted_at = sqlc.arg(now), updated_at = sqlc.arg(now)
WHERE id = ANY(sqlc.arg(ids)::uuid[]) AND deleted_at IS NULL;

-- name: LockBuyerForumEntryBatch :many
SELECT id, store_id, parent_id FROM social.forum_entries
WHERE actor_kind = 'BUYER' AND actor_id = sqlc.arg(buyer_id) AND deleted_at IS NULL
ORDER BY id
LIMIT sqlc.arg(max_rows)
FOR UPDATE;

-- name: SoftDeleteForumEntryBatch :exec
UPDATE social.forum_entries SET deleted_at = sqlc.arg(now), updated_at = sqlc.arg(now)
WHERE id = ANY(sqlc.arg(ids)::uuid[]) AND deleted_at IS NULL;

-- name: ListBuyerLikedPostBatch :many
-- 先列出再刪除：清除 job 要先依固定順序鎖 stats 列，才能開始刪（counts-brief.md §4.6）。
SELECT post_id FROM social.post_likes WHERE buyer_id = sqlc.arg(buyer_id) ORDER BY post_id LIMIT sqlc.arg(max_rows);

-- name: DeleteBuyerPostLikes :exec
DELETE FROM social.post_likes WHERE buyer_id = sqlc.arg(buyer_id) AND post_id = ANY(sqlc.arg(post_ids)::uuid[]);

-- name: ListBuyerSharedPostBatch :many
SELECT post_id FROM social.post_shares WHERE buyer_id = sqlc.arg(buyer_id) ORDER BY post_id LIMIT sqlc.arg(max_rows);

-- name: DeleteBuyerPostShares :exec
DELETE FROM social.post_shares WHERE buyer_id = sqlc.arg(buyer_id) AND post_id = ANY(sqlc.arg(post_ids)::uuid[]);

-- name: ListActorCommentLikeBatch :many
-- 互動者（買家或店家）的留言讚；帶回留言的所屬貼文，給快取失效。
SELECT l.comment_id, c.post_id
FROM social.comment_likes l JOIN social.comments c ON c.id = l.comment_id
WHERE l.actor_kind = sqlc.arg(actor_kind) AND l.actor_id = sqlc.arg(actor_id)
ORDER BY l.comment_id
LIMIT sqlc.arg(max_rows);

-- name: DeleteActorCommentLikes :exec
DELETE FROM social.comment_likes
WHERE actor_kind = sqlc.arg(actor_kind) AND actor_id = sqlc.arg(actor_id) AND comment_id = ANY(sqlc.arg(comment_ids)::uuid[]);

-- name: LockStoreCommentBatch :many
-- 店家身分寫的留言。店家身分只會出現在它自己的貼文下（actor.ForWrite），所以也等於它的貼文底下店家寫的留言；以作者條件找，不依賴這個寫入規則。
SELECT id, post_id, parent_id FROM social.comments
WHERE actor_kind = 'STORE' AND actor_id = sqlc.arg(store_id) AND deleted_at IS NULL
ORDER BY id
LIMIT sqlc.arg(max_rows)
FOR UPDATE;

-- name: LockStoreForumEntryBatch :many
-- 討論區的容器是店家：該店討論區的全部留言（含買家寫的）。店家身分的留言也只會在自己的討論區，作者條件是不依賴寫入規則的後備。
SELECT id, store_id, parent_id FROM social.forum_entries
WHERE deleted_at IS NULL AND (store_id = sqlc.arg(store_id) OR (actor_kind = 'STORE' AND actor_id = sqlc.arg(store_id)))
ORDER BY id
LIMIT sqlc.arg(max_rows)
FOR UPDATE;

-- name: LockStorePostBatch :many
-- 店家的貼文：還沒刪除的，或已經軟刪但圖片引用還在的（軟刪保留引用，退場時一起釋放）。
SELECT id, image_asset_id FROM social.posts
WHERE store_id = sqlc.arg(store_id) AND (deleted_at IS NULL OR image_asset_id IS NOT NULL)
ORDER BY id
LIMIT sqlc.arg(max_rows)
FOR UPDATE;

-- name: PurgePostBatch :exec
-- 軟刪（已經刪除的保留原時間）並清掉圖片欄位（引用由同步 job 釋放）。
UPDATE social.posts
SET deleted_at = COALESCE(deleted_at, sqlc.arg(now)), image_key = NULL, image_asset_id = NULL,
    version = version + 1, updated_at = sqlc.arg(now)
WHERE id = ANY(sqlc.arg(ids)::uuid[]);

-- name: ScrubBuyerCommentBodyBatch :execrows
-- 買家刪帳的清除：把他已刪除的留言的內文設為 NULL（保留列，討論串結構與佔位項目還在；comments_scrubbed_check 保證 NULL 只出現在已刪除的列）。
-- 冪等：只處理還有內文的列，重跑只處理剩下的。不動 updated_at（不是使用者的修改）。
UPDATE social.comments SET body = NULL
WHERE id IN (
    SELECT t.id FROM social.comments t
    WHERE t.actor_kind = 'BUYER' AND t.actor_id = sqlc.arg(buyer_id) AND t.deleted_at IS NOT NULL AND t.body IS NOT NULL
    ORDER BY t.id
    LIMIT sqlc.arg(max_rows)
    FOR UPDATE
);

-- name: ScrubBuyerForumEntryBodyBatch :execrows
UPDATE social.forum_entries SET body = NULL
WHERE id IN (
    SELECT t.id FROM social.forum_entries t
    WHERE t.actor_kind = 'BUYER' AND t.actor_id = sqlc.arg(buyer_id) AND t.deleted_at IS NOT NULL AND t.body IS NOT NULL
    ORDER BY t.id
    LIMIT sqlc.arg(max_rows)
    FOR UPDATE
);
