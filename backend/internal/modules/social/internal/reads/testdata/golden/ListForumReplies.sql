SELECT c.id AS "id",
     c.store_id AS "store_id",
     c.parent_id AS "parent_id",
     c.reply_to_id AS "reply_to_id",
     'VISIBLE' AS "state",
     c.actor_kind AS "actor_kind",
     c.actor_id AS "actor_id",
     c.body AS "body",
     c.created_at AS "created_at",
     c.updated_at AS "updated_at",
     s.reply_count AS "reply_count"
FROM social.forum_entries AS c
     INNER JOIN social.forum_entry_stats AS s ON (s.forum_entry_id = c.id)
WHERE (((c.parent_id = $1::uuid) AND c.counted) AND (EXISTS (SELECT 1 FROM seller.pub_exposable_stores($2::timestamp with time zone) AS pub WHERE pub.store_id = c.store_id))) AND (ROW(c.created_at, c.id) > ROW($3::timestamp with time zone, $4::uuid))
ORDER BY c.created_at ASC, c.id ASC
LIMIT $5;

-- args:
-- $1 = 018f0000-0000-7000-8000-000000000001
-- $2 = 2026-10-23 12:00:00 +0000 UTC
-- $3 = 2026-10-23 12:00:00 +0000 UTC
-- $4 = 018f0000-0000-7000-8000-000000000001
-- $5 = 51
