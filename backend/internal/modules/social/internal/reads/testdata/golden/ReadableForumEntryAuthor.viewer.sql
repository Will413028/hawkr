SELECT e.actor_kind AS "actor_kind",
     e.actor_id AS "actor_id",
     e.store_id AS "store_id"
FROM social.forum_entries AS e
WHERE (((((e.actor_kind != 'BUYER') OR (NOT (EXISTS (SELECT 1 FROM buyer.pub_blocked_buyers() AS pub WHERE pub.blocker_buyer_id = e.actor_id AND ((blocked_buyer_id = $1::bigint)))))) AND (e.id = $2::uuid)) AND e.counted) AND (EXISTS (SELECT 1 FROM seller.pub_exposable_stores($3::timestamp with time zone) AS pub WHERE pub.store_id = e.store_id))) AND ((e.parent_id IS NULL) OR (EXISTS (
           SELECT pe.id AS "pe.id"
           FROM social.forum_entries AS pe
           WHERE ((pe.actor_kind != 'BUYER') OR (NOT (EXISTS (SELECT 1 FROM buyer.pub_blocked_buyers() AS pub WHERE pub.blocker_buyer_id = pe.actor_id AND ((blocked_buyer_id = $4::bigint)))))) AND (pe.id = e.parent_id)
      )));

-- args:
-- $1 = 42
-- $2 = 018f0000-0000-7000-8000-000000000001
-- $3 = 2026-10-23 12:00:00 +0000 UTC
-- $4 = 42
