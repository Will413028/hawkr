SELECT c.id AS "id",
     c.post_id AS "post_id",
     c.parent_id AS "parent_id",
     c.reply_to_id AS "reply_to_id",
     'VISIBLE' AS "state",
     c.actor_kind AS "actor_kind",
     c.actor_id AS "actor_id",
     c.body AS "body",
     c.created_at AS "created_at",
     c.updated_at AS "updated_at",
     s.like_count AS "like_count",
     s.reply_count AS "reply_count"
FROM social.comments AS c
     INNER JOIN social.posts AS p ON (p.id = c.post_id)
     INNER JOIN social.comment_stats AS s ON (s.comment_id = c.id)
WHERE (((c.parent_id = $1::uuid) AND c.counted) AND (((p.status = 'PUBLISHED') AND (p.deleted_at IS NULL)) AND (EXISTS (SELECT 1 FROM seller.pub_exposable_stores($2::timestamp with time zone) AS pub WHERE pub.store_id = p.store_id)))) AND (ROW(c.created_at, c.id) > ROW($3::timestamp with time zone, $4::uuid))
ORDER BY c.created_at ASC, c.id ASC
LIMIT $5;

-- args:
-- $1 = 018f0000-0000-7000-8000-000000000001
-- $2 = 2026-10-23 12:00:00 +0000 UTC
-- $3 = 2026-10-23 12:00:00 +0000 UTC
-- $4 = 018f0000-0000-7000-8000-000000000001
-- $5 = 51
