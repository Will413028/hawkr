
-- name: InsertInactiveAuthor :execrows
INSERT INTO social.inactive_authors (buyer_id, since)
VALUES (sqlc.arg(buyer_id), sqlc.arg(since))
ON CONFLICT (buyer_id) DO NOTHING;

-- name: LockAuthorComments :many
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
SELECT b.buyer_id FROM buyer.pub_inactive_buyers() AS b
WHERE NOT EXISTS (SELECT 1 FROM social.inactive_authors AS ia WHERE ia.buyer_id = b.buyer_id)
ORDER BY b.buyer_id;
