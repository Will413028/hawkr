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
WHERE ((((((c.actor_kind != 'BUYER') OR ((NOT (EXISTS (SELECT 1 FROM buyer.pub_blocked_buyers() AS pub WHERE pub.blocker_buyer_id = $1::bigint AND ((blocked_buyer_id = c.actor_id))))) AND (NOT (EXISTS (SELECT 1 FROM buyer.pub_blocked_buyers() AS pub WHERE pub.blocker_buyer_id = c.actor_id AND ((blocked_buyer_id = $2::bigint))))))) AND (c.parent_id = $3::uuid)) AND c.counted) AND ((EXISTS (SELECT 1 FROM seller.pub_exposable_stores($4::timestamp with time zone) AS pub WHERE pub.store_id = c.store_id)) AND (NOT (EXISTS (SELECT 1 FROM buyer.pub_blocked_stores() AS pub WHERE pub.blocker_buyer_id = $5::bigint AND ((store_id = c.store_id))))))) AND (EXISTS (
           SELECT pe.id AS "pe.id"
           FROM social.forum_entries AS pe
           WHERE ((pe.actor_kind != 'BUYER') OR ((NOT (EXISTS (SELECT 1 FROM buyer.pub_blocked_buyers() AS pub WHERE pub.blocker_buyer_id = $6::bigint AND ((blocked_buyer_id = pe.actor_id))))) AND (NOT (EXISTS (SELECT 1 FROM buyer.pub_blocked_buyers() AS pub WHERE pub.blocker_buyer_id = pe.actor_id AND ((blocked_buyer_id = $7::bigint))))))) AND (pe.id = $8::uuid)
      ))) AND (ROW(c.created_at, c.id) > ROW($9::timestamp with time zone, $10::uuid))
ORDER BY c.created_at ASC, c.id ASC
LIMIT $11;

-- args:
-- $1 = 42
-- $2 = 42
-- $3 = 018f0000-0000-7000-8000-000000000001
-- $4 = 2026-10-23 12:00:00 +0000 UTC
-- $5 = 42
-- $6 = 42
-- $7 = 42
-- $8 = 018f0000-0000-7000-8000-000000000001
-- $9 = 2026-10-23 12:00:00 +0000 UTC
-- $10 = 018f0000-0000-7000-8000-000000000001
-- $11 = 51
