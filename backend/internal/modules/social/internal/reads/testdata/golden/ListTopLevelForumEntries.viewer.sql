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
     s.reply_count AS "reply_count"
FROM social.forum_entries AS c
     INNER JOIN social.forum_entry_stats AS s ON (s.forum_entry_id = c.id)
     CROSS JOIN LATERAL (
          SELECT c.counted AS "visible"
     ) AS v
WHERE ((((((c.actor_kind != 'BUYER') OR ((NOT (EXISTS (SELECT 1 FROM buyer.pub_blocked_buyers() AS pub WHERE pub.blocker_buyer_id = $4::bigint AND ((blocked_buyer_id = c.actor_id))))) AND (NOT (EXISTS (SELECT 1 FROM buyer.pub_blocked_buyers() AS pub WHERE pub.blocker_buyer_id = c.actor_id AND ((blocked_buyer_id = $5::bigint))))))) AND (c.store_id = $6::bigint)) AND (c.parent_id IS NULL)) AND ((EXISTS (SELECT 1 FROM seller.pub_exposable_stores($7::timestamp with time zone) AS pub WHERE pub.store_id = $8::bigint)) AND (NOT (EXISTS (SELECT 1 FROM buyer.pub_blocked_stores() AS pub WHERE pub.blocker_buyer_id = $9::bigint AND ((store_id = $10::bigint))))))) AND (v.visible OR (EXISTS (
           SELECT r.id AS "r.id"
           FROM social.forum_entries AS r
           WHERE (((r.actor_kind != 'BUYER') OR ((NOT (EXISTS (SELECT 1 FROM buyer.pub_blocked_buyers() AS pub WHERE pub.blocker_buyer_id = $11::bigint AND ((blocked_buyer_id = r.actor_id))))) AND (NOT (EXISTS (SELECT 1 FROM buyer.pub_blocked_buyers() AS pub WHERE pub.blocker_buyer_id = r.actor_id AND ((blocked_buyer_id = $12::bigint))))))) AND (r.parent_id = c.id)) AND r.counted
      )))) AND (ROW(c.created_at, c.id) > ROW($13::timestamp with time zone, $14::uuid))
ORDER BY c.created_at ASC, c.id ASC
LIMIT $15;

-- args:
-- $1 = 
-- $2 = 0
-- $3 = 
-- $4 = 42
-- $5 = 42
-- $6 = 7
-- $7 = 2026-10-23 12:00:00 +0000 UTC
-- $8 = 7
-- $9 = 42
-- $10 = 7
-- $11 = 42
-- $12 = 42
-- $13 = 2026-10-23 12:00:00 +0000 UTC
-- $14 = 018f0000-0000-7000-8000-000000000001
-- $15 = 51
