-- name: UpsertHashtagByName :one
INSERT INTO cms_hashtag (id, name)
VALUES ($1, $2)
ON CONFLICT (name) DO UPDATE SET update_time = NOW()
RETURNING id;

-- name: AdjustHashtagPostCount :exec
UPDATE cms_hashtag
SET post_count = post_count + $2, update_time = NOW()
WHERE id = $1;

-- name: ListPostHashtags :many
SELECT post_id, hashtag_id, display_name, position, create_time
FROM cms_post_hashtag
WHERE post_id = $1
ORDER BY position;

-- name: ListPostHashtagsByPostIDs :many
SELECT post_id, hashtag_id, display_name, position, create_time
FROM cms_post_hashtag
WHERE post_id = ANY(sqlc.arg(post_ids)::bigint[])
ORDER BY post_id, position;

-- name: ListHashtagsByIDs :many
SELECT id, name, post_count, create_time, update_time
FROM cms_hashtag
WHERE id = ANY(sqlc.arg(hashtag_ids)::bigint[])
ORDER BY id;

-- name: ListHashtagIDsByPostID :many
SELECT hashtag_id
FROM cms_post_hashtag
WHERE post_id = $1
ORDER BY hashtag_id;

-- name: InsertPostHashtag :exec
INSERT INTO cms_post_hashtag (post_id, hashtag_id, display_name, position)
VALUES ($1, $2, $3, $4);

-- name: UpdatePostHashtagBinding :exec
UPDATE cms_post_hashtag
SET display_name = $3, position = $4
WHERE post_id = $1 AND hashtag_id = $2;

-- name: DeletePostHashtags :exec
DELETE FROM cms_post_hashtag
WHERE post_id = $1 AND hashtag_id = ANY(sqlc.arg(hashtag_ids)::bigint[]);

-- name: DeleteAllPostHashtags :exec
DELETE FROM cms_post_hashtag
WHERE post_id = $1;

-- name: SearchHashtagsByPrefix :many
SELECT name, post_count
FROM cms_hashtag
WHERE name LIKE sqlc.arg(prefix)::text || '%' AND post_count > 0
ORDER BY post_count DESC, name ASC
LIMIT sqlc.arg(lim);

-- name: SearchHashtagsByContain :many
SELECT name, post_count, similarity(name, sqlc.arg(query)) AS sim
FROM cms_hashtag
WHERE name % sqlc.arg(query) AND post_count > 0
ORDER BY sim DESC, post_count DESC, name ASC
LIMIT sqlc.arg(lim);

-- name: SelectPopularHashtagsInRange :many
-- COUNT(*) is exact: PK (post_id, hashtag_id) on cms_post_hashtag means each
-- (post, tag) pair appears at most once; DISTINCT is therefore redundant.
SELECT h.name AS hashtag, COUNT(*)::int AS count
FROM cms_post_hashtag ph
JOIN cms_post p     ON p.id = ph.post_id
JOIN cms_hashtag h  ON h.id = ph.hashtag_id
WHERE ph.create_time >= sqlc.arg(start_at) AND ph.create_time < sqlc.arg(end_at)
  AND p.deleted_at IS NULL AND p.hidden_at IS NULL
  AND p.publish_status = sqlc.arg(published_status)::smallint
GROUP BY h.name
ORDER BY count DESC, h.name ASC
LIMIT sqlc.arg(lim);
