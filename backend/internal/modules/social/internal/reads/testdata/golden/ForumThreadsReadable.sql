SELECT c.id AS "id",
     c.store_id AS "store_id",
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
     c.version AS "version",
     s.reply_count AS "reply_count"
FROM social.forum_entries AS c
     INNER JOIN social.forum_entry_stats AS s ON (s.forum_entry_id = c.id)
     CROSS JOIN LATERAL (
          SELECT c.counted AS "visible"
     ) AS v
WHERE ((((c.store_id = c.store_id) AND (c.parent_id IS NULL)) AND (EXISTS (SELECT 1 FROM seller.pub_exposable_stores($4::timestamp with time zone) AS pub WHERE pub.store_id = c.store_id))) AND (v.visible OR (EXISTS (
           SELECT r.id AS "r.id"
           FROM social.forum_entries AS r
           WHERE (r.parent_id = c.id) AND r.counted
      )))) AND (c.id IN ($5::uuid, $6::uuid));

-- args:
-- $1 = 
-- $2 = 0
-- $3 = 
-- $4 = 2026-10-23 12:00:00 +0000 UTC
-- $5 = 018f0000-0000-7000-8000-000000000001
-- $6 = 018f0000-0000-7000-8000-000000000002
