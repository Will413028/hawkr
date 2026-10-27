-- 買家刪帳的事實在 social 的複本（schema.sql 的 inactive_authors）：訂閱者在買家進入刪除中與關閉的同一個 transaction 寫入，每日對帳核對。

-- name: InsertInactiveAuthor :execrows
-- 冪等：買家的狀態不會回到營運中，所以只插入不刪除；已經有的列 0 列（呼叫方不必再重新求值）。
INSERT INTO social.inactive_authors (buyer_id, since)
VALUES (sqlc.arg(buyer_id), sqlc.arg(since))
ON CONFLICT (buyer_id) DO NOTHING;

-- name: LockAuthorComments :many
-- 買家寫過的全部留言（含已刪除的），依 ID 由小到大鎖住（FOR NO KEY UPDATE 在排序之後依序取鎖），帶回要鎖的 stats 的鍵。
SELECT id, post_id, parent_id FROM social.comments
WHERE actor_kind = 'BUYER' AND actor_id = sqlc.arg(buyer_id)
ORDER BY id
FOR NO KEY UPDATE;

-- name: LockAuthorForumEntries :many
SELECT id, parent_id FROM social.forum_entries
WHERE actor_kind = 'BUYER' AND actor_id = sqlc.arg(buyer_id)
ORDER BY id
FOR NO KEY UPDATE;

-- name: MissingInactiveAuthors :many
-- buyer 發布的「此刻不是營運中的買家」裡，複本還沒有的（漂移）：每日對帳補上。
SELECT b.buyer_id FROM buyer.pub_inactive_buyers() AS b
WHERE NOT EXISTS (SELECT 1 FROM social.inactive_authors AS ia WHERE ia.buyer_id = b.buyer_id)
ORDER BY b.buyer_id;
