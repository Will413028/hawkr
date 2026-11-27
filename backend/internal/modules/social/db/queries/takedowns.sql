
-- name: InsertTakedowns :many
INSERT INTO social.content_takedowns (measure_id, post_id, comment_id, forum_entry_id)
SELECT sqlc.arg(measure_id)::uuid, p, NULL::uuid, NULL::uuid FROM unnest(sqlc.arg(post_ids)::uuid[]) AS p
UNION ALL
SELECT sqlc.arg(measure_id)::uuid, NULL::uuid, c, NULL::uuid FROM unnest(sqlc.arg(comment_ids)::uuid[]) AS c
UNION ALL
SELECT sqlc.arg(measure_id)::uuid, NULL::uuid, NULL::uuid, f FROM unnest(sqlc.arg(forum_entry_ids)::uuid[]) AS f
ON CONFLICT DO NOTHING
RETURNING post_id, comment_id, forum_entry_id;

-- name: InvalidTakedownContents :many
SELECT 'POST'::text AS content_kind, p::uuid AS content_id FROM unnest(sqlc.arg(post_ids)::uuid[]) AS p
WHERE EXISTS (SELECT 1 FROM social.posts AS x WHERE x.id = p AND x.store_id <> sqlc.arg(store_id)::bigint)
UNION ALL
SELECT 'COMMENT'::text, c::uuid FROM unnest(sqlc.arg(comment_ids)::uuid[]) AS c
WHERE EXISTS (SELECT 1 FROM social.comments AS x WHERE x.id = c AND NOT (x.actor_kind = 'STORE' AND x.actor_id = sqlc.arg(store_id)::bigint))
UNION ALL
SELECT 'FORUM_ENTRY'::text, f::uuid FROM unnest(sqlc.arg(forum_entry_ids)::uuid[]) AS f
WHERE EXISTS (SELECT 1 FROM social.forum_entries AS x WHERE x.id = f AND NOT (x.actor_kind = 'STORE' AND x.actor_id = sqlc.arg(store_id)::bigint))
ORDER BY 1, 2;

-- name: GoneTakedownContents :many
SELECT 'POST'::text AS content_kind, p::uuid AS content_id FROM unnest(sqlc.arg(post_ids)::uuid[]) AS p
WHERE NOT EXISTS (SELECT 1 FROM social.posts AS x WHERE x.id = p AND x.deleted_at IS NULL)
UNION ALL
SELECT 'COMMENT'::text, c::uuid FROM unnest(sqlc.arg(comment_ids)::uuid[]) AS c
WHERE NOT EXISTS (SELECT 1 FROM social.comments AS x WHERE x.id = c AND x.deleted_at IS NULL)
UNION ALL
SELECT 'FORUM_ENTRY'::text, f::uuid FROM unnest(sqlc.arg(forum_entry_ids)::uuid[]) AS f
WHERE NOT EXISTS (SELECT 1 FROM social.forum_entries AS x WHERE x.id = f AND x.deleted_at IS NULL)
ORDER BY 1, 2;

-- name: LockActiveTakedownsOfMeasure :many
SELECT content_kind, content_id FROM seller.pub_lock_content_takedowns(sqlc.arg(measure_id)::uuid);

-- name: InsertActiveTakedowns :many
INSERT INTO social.content_takedowns (measure_id, post_id, comment_id, forum_entry_id)
SELECT t.measure_id,
       CASE WHEN t.content_kind = 'POST' THEN t.content_id END,
       CASE WHEN t.content_kind = 'COMMENT' THEN t.content_id END,
       CASE WHEN t.content_kind = 'FORUM_ENTRY' THEN t.content_id END
FROM seller.pub_lock_content_takedowns(sqlc.arg(measure_id)::uuid) AS t
WHERE (t.content_kind = 'POST' AND t.content_id = ANY(sqlc.arg(post_ids)::uuid[]))
   OR (t.content_kind = 'COMMENT' AND t.content_id = ANY(sqlc.arg(comment_ids)::uuid[]))
   OR (t.content_kind = 'FORUM_ENTRY' AND t.content_id = ANY(sqlc.arg(forum_entry_ids)::uuid[]))
ON CONFLICT DO NOTHING
RETURNING post_id, comment_id, forum_entry_id;

-- name: TakedownsOfMeasure :many
SELECT post_id, comment_id, forum_entry_id FROM social.content_takedowns
WHERE measure_id = sqlc.arg(measure_id);

-- name: TakedownMeasuresOfPosts :many
SELECT post_id, measure_id FROM social.content_takedowns
WHERE post_id = ANY(sqlc.arg(post_ids)::uuid[])
ORDER BY post_id;

-- name: DeleteTakedownsOfMeasure :execrows
DELETE FROM social.content_takedowns WHERE measure_id = sqlc.arg(measure_id);

-- name: DeleteTakedownContents :execrows
DELETE FROM social.content_takedowns
WHERE measure_id = sqlc.arg(measure_id)
  AND (post_id = ANY(sqlc.arg(post_ids)::uuid[])
    OR comment_id = ANY(sqlc.arg(comment_ids)::uuid[])
    OR forum_entry_id = ANY(sqlc.arg(forum_entry_ids)::uuid[]));

-- name: LockCommentsByID :many
SELECT id FROM social.comments WHERE id = ANY(sqlc.arg(ids)::uuid[]) ORDER BY id FOR NO KEY UPDATE;

-- name: LockForumEntriesByID :many
SELECT id FROM social.forum_entries WHERE id = ANY(sqlc.arg(ids)::uuid[]) ORDER BY id FOR NO KEY UPDATE;

-- name: TakedownPostStores :many
SELECT p.id, p.store_id FROM social.posts AS p WHERE p.id = ANY(sqlc.arg(post_ids)::uuid[])
UNION
SELECT p.id, p.store_id FROM social.comments AS c JOIN social.posts AS p ON p.id = c.post_id WHERE c.id = ANY(sqlc.arg(comment_ids)::uuid[])
ORDER BY 1;

-- name: TakedownForumStores :many
SELECT DISTINCT store_id FROM social.forum_entries WHERE id = ANY(sqlc.arg(ids)::uuid[]) ORDER BY store_id;

-- name: MissingTakedowns :many
SELECT t.measure_id, t.content_kind, t.content_id FROM seller.pub_content_takedowns() AS t
WHERE NOT EXISTS (
    SELECT 1 FROM social.content_takedowns AS c
    WHERE c.measure_id = t.measure_id
      AND ((t.content_kind = 'POST' AND c.post_id = t.content_id)
        OR (t.content_kind = 'COMMENT' AND c.comment_id = t.content_id)
        OR (t.content_kind = 'FORUM_ENTRY' AND c.forum_entry_id = t.content_id)))
  AND ((t.content_kind = 'POST' AND EXISTS (SELECT 1 FROM social.posts AS p WHERE p.id = t.content_id))
    OR (t.content_kind = 'COMMENT' AND EXISTS (SELECT 1 FROM social.comments AS m WHERE m.id = t.content_id))
    OR (t.content_kind = 'FORUM_ENTRY' AND EXISTS (SELECT 1 FROM social.forum_entries AS f WHERE f.id = t.content_id)))
ORDER BY t.measure_id, t.content_kind, t.content_id;

-- name: ExtraTakedowns :many
SELECT c.measure_id, c.post_id, c.comment_id, c.forum_entry_id FROM social.content_takedowns AS c
WHERE NOT EXISTS (
    SELECT 1 FROM seller.pub_content_takedowns() AS t
    WHERE t.measure_id = c.measure_id
      AND ((t.content_kind = 'POST' AND c.post_id = t.content_id)
        OR (t.content_kind = 'COMMENT' AND c.comment_id = t.content_id)
        OR (t.content_kind = 'FORUM_ENTRY' AND c.forum_entry_id = t.content_id)))
ORDER BY c.measure_id, c.post_id, c.comment_id, c.forum_entry_id;
