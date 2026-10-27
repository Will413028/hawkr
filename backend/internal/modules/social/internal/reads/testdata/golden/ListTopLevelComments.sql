SELECT c.id AS "id",
     c.post_id AS "post_id",
     c.parent_id AS "parent_id",
     c.reply_to_id AS "reply_to_id",
     (CASE WHEN v.visible THEN 'VISIBLE' WHEN (c.deleted_at IS NOT NULL) OR (EXISTS (
          SELECT ia.buyer_id AS "ia.buyer_id"
          FROM social.inactive_authors AS ia
          WHERE (ia.buyer_id = c.actor_id) AND (c.actor_kind = 'BUYER')
     )) THEN 'DELETED' ELSE 'HIDDEN' END) AS "state",
     COALESCE((CASE WHEN v.visible THEN c.actor_kind END), $1::text) AS "actor_kind",
     COALESCE((CASE WHEN v.visible THEN c.actor_id END), $2::bigint) AS "actor_id",
     COALESCE((CASE WHEN v.visible THEN c.body END), $3::text) AS "body",
     c.created_at AS "created_at",
     c.updated_at AS "updated_at",
     s.like_count AS "like_count",
     s.reply_count AS "reply_count"
FROM social.comments AS c
     INNER JOIN social.comment_stats AS s ON (s.comment_id = c.id)
     CROSS JOIN LATERAL (
          SELECT c.counted AS "visible"
     ) AS v
WHERE ((((c.post_id = $4::uuid) AND (c.parent_id IS NULL)) AND (EXISTS (
           SELECT cp.id AS "cp.id"
           FROM social.posts AS cp
           WHERE (((cp.status = 'PUBLISHED') AND (cp.deleted_at IS NULL)) AND (EXISTS (SELECT 1 FROM seller.pub_exposable_stores($5::timestamp with time zone) AS pub WHERE pub.store_id = cp.store_id))) AND (cp.id = $6::uuid)
      ))) AND (v.visible OR (EXISTS (
           SELECT r.id AS "r.id"
           FROM social.comments AS r
           WHERE (r.parent_id = c.id) AND r.counted
      )))) AND (ROW(c.created_at, c.id) > ROW($7::timestamp with time zone, $8::uuid))
ORDER BY c.created_at ASC, c.id ASC
LIMIT $9;

-- args:
-- $1 = 
-- $2 = 0
-- $3 = 
-- $4 = 018f0000-0000-7000-8000-000000000001
-- $5 = 2026-10-23 12:00:00 +0000 UTC
-- $6 = 018f0000-0000-7000-8000-000000000001
-- $7 = 2026-10-23 12:00:00 +0000 UTC
-- $8 = 018f0000-0000-7000-8000-000000000001
-- $9 = 51
