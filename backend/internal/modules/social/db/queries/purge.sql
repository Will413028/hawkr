
-- name: LockBuyerCommentBatch :many
SELECT id, post_id, parent_id FROM social.comments
WHERE actor_kind = 'BUYER' AND actor_id = sqlc.arg(buyer_id) AND deleted_at IS NULL
ORDER BY id
LIMIT sqlc.arg(max_rows)
FOR UPDATE;

-- name: SoftDeleteCommentBatch :exec
UPDATE social.comments SET deleted_at = sqlc.arg(now), version = version + 1, updated_at = sqlc.arg(now)
WHERE id = ANY(sqlc.arg(ids)::uuid[]) AND deleted_at IS NULL;

-- name: LockBuyerForumEntryBatch :many
SELECT id, store_id, parent_id FROM social.forum_entries
WHERE actor_kind = 'BUYER' AND actor_id = sqlc.arg(buyer_id) AND deleted_at IS NULL
ORDER BY id
LIMIT sqlc.arg(max_rows)
FOR UPDATE;

-- name: SoftDeleteForumEntryBatch :exec
UPDATE social.forum_entries SET deleted_at = sqlc.arg(now), version = version + 1, updated_at = sqlc.arg(now)
WHERE id = ANY(sqlc.arg(ids)::uuid[]) AND deleted_at IS NULL;

-- name: ListBuyerLikedPostBatch :many
SELECT post_id FROM social.post_likes WHERE buyer_id = sqlc.arg(buyer_id) ORDER BY post_id LIMIT sqlc.arg(max_rows);

-- name: DeleteBuyerPostLikes :exec
DELETE FROM social.post_likes WHERE buyer_id = sqlc.arg(buyer_id) AND post_id = ANY(sqlc.arg(post_ids)::uuid[]);

-- name: ListBuyerSharedPostBatch :many
SELECT post_id FROM social.post_shares WHERE buyer_id = sqlc.arg(buyer_id) ORDER BY post_id LIMIT sqlc.arg(max_rows);

-- name: DeleteBuyerPostShares :exec
DELETE FROM social.post_shares WHERE buyer_id = sqlc.arg(buyer_id) AND post_id = ANY(sqlc.arg(post_ids)::uuid[]);

-- name: ListActorCommentLikeBatch :many
SELECT l.comment_id, c.post_id
FROM social.comment_likes l JOIN social.comments c ON c.id = l.comment_id
WHERE l.actor_kind = sqlc.arg(actor_kind) AND l.actor_id = sqlc.arg(actor_id)
ORDER BY l.comment_id
LIMIT sqlc.arg(max_rows);

-- name: DeleteActorCommentLikes :exec
DELETE FROM social.comment_likes
WHERE actor_kind = sqlc.arg(actor_kind) AND actor_id = sqlc.arg(actor_id) AND comment_id = ANY(sqlc.arg(comment_ids)::uuid[]);

-- name: LockStoreCommentBatch :many
SELECT id, post_id, parent_id FROM social.comments
WHERE actor_kind = 'STORE' AND actor_id = sqlc.arg(store_id) AND deleted_at IS NULL
ORDER BY id
LIMIT sqlc.arg(max_rows)
FOR UPDATE;

-- name: LockStoreForumEntryBatch :many
SELECT id, store_id, parent_id FROM social.forum_entries
WHERE deleted_at IS NULL AND (store_id = sqlc.arg(store_id) OR (actor_kind = 'STORE' AND actor_id = sqlc.arg(store_id)))
ORDER BY id
LIMIT sqlc.arg(max_rows)
FOR UPDATE;

-- name: LockStorePostBatch :many
SELECT id, image_asset_id FROM social.posts
WHERE store_id = sqlc.arg(store_id) AND (deleted_at IS NULL OR image_asset_id IS NOT NULL)
ORDER BY id
LIMIT sqlc.arg(max_rows)
FOR UPDATE;

-- name: PurgePostBatch :exec
UPDATE social.posts
SET deleted_at = COALESCE(deleted_at, sqlc.arg(now)), image_key = NULL, image_asset_id = NULL,
    version = version + 1, updated_at = sqlc.arg(now)
WHERE id = ANY(sqlc.arg(ids)::uuid[]);

-- name: ScrubBuyerCommentBodyBatch :execrows
UPDATE social.comments SET body = NULL, version = version + 1, request_id = NULL, request_hash = NULL
WHERE id IN (
    SELECT t.id FROM social.comments t
    WHERE t.actor_kind = 'BUYER' AND t.actor_id = sqlc.arg(buyer_id) AND t.deleted_at IS NOT NULL AND t.body IS NOT NULL
    ORDER BY t.id
    LIMIT sqlc.arg(max_rows)
    FOR UPDATE
);

-- name: ScrubBuyerForumEntryBodyBatch :execrows
UPDATE social.forum_entries SET body = NULL, version = version + 1, request_id = NULL, request_hash = NULL
WHERE id IN (
    SELECT t.id FROM social.forum_entries t
    WHERE t.actor_kind = 'BUYER' AND t.actor_id = sqlc.arg(buyer_id) AND t.deleted_at IS NOT NULL AND t.body IS NOT NULL
    ORDER BY t.id
    LIMIT sqlc.arg(max_rows)
    FOR UPDATE
);

-- name: ListInactiveBuyersWithSocialContent :many
SELECT ib.buyer_id FROM buyer.pub_inactive_buyers() AS ib
WHERE ib.buyer_id > sqlc.arg(after_id)::bigint
  AND (
    EXISTS (SELECT 1 FROM social.comments c WHERE c.actor_kind = 'BUYER' AND c.actor_id = ib.buyer_id AND (c.deleted_at IS NULL OR c.body IS NOT NULL))
    OR EXISTS (SELECT 1 FROM social.forum_entries e WHERE e.actor_kind = 'BUYER' AND e.actor_id = ib.buyer_id AND (e.deleted_at IS NULL OR e.body IS NOT NULL))
    OR EXISTS (SELECT 1 FROM social.post_likes l WHERE l.buyer_id = ib.buyer_id)
    OR EXISTS (SELECT 1 FROM social.post_shares s WHERE s.buyer_id = ib.buyer_id)
    OR EXISTS (SELECT 1 FROM social.comment_likes cl WHERE cl.actor_kind = 'BUYER' AND cl.actor_id = ib.buyer_id)
  )
ORDER BY ib.buyer_id
LIMIT sqlc.arg(max_rows);

-- name: ListRetiredStoresWithSocialContent :many
SELECT rs.store_id FROM seller.pub_retired_stores() AS rs
WHERE rs.store_id > sqlc.arg(after_id)::bigint
  AND (
    EXISTS (SELECT 1 FROM social.posts p WHERE p.store_id = rs.store_id AND (p.deleted_at IS NULL OR p.image_asset_id IS NOT NULL))
    OR EXISTS (SELECT 1 FROM social.comments c WHERE c.actor_kind = 'STORE' AND c.actor_id = rs.store_id AND c.deleted_at IS NULL)
    OR EXISTS (SELECT 1 FROM social.forum_entries e WHERE e.deleted_at IS NULL AND (e.store_id = rs.store_id OR (e.actor_kind = 'STORE' AND e.actor_id = rs.store_id)))
    OR EXISTS (SELECT 1 FROM social.comment_likes cl WHERE cl.actor_kind = 'STORE' AND cl.actor_id = rs.store_id)
  )
ORDER BY rs.store_id
LIMIT sqlc.arg(max_rows);
