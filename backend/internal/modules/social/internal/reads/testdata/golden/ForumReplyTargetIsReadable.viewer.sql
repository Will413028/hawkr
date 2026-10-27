SELECT (EXISTS (
          SELECT e.id AS "e.id"
          FROM social.forum_entries AS e
          WHERE ((((((e.actor_kind != 'BUYER') OR ((NOT (EXISTS (SELECT 1 FROM buyer.pub_blocked_buyers() AS pub WHERE pub.blocker_buyer_id = $1::bigint AND ((blocked_buyer_id = e.actor_id))))) AND (NOT (EXISTS (SELECT 1 FROM buyer.pub_blocked_buyers() AS pub WHERE pub.blocker_buyer_id = e.actor_id AND ((blocked_buyer_id = $2::bigint))))))) AND (e.id = $3::uuid)) AND (e.store_id = $4::bigint)) AND (e.thread_id = $5::uuid)) AND e.counted) AND (EXISTS (
                     SELECT pe.id AS "pe.id"
                     FROM social.forum_entries AS pe
                     WHERE ((pe.actor_kind != 'BUYER') OR ((NOT (EXISTS (SELECT 1 FROM buyer.pub_blocked_buyers() AS pub WHERE pub.blocker_buyer_id = $6::bigint AND ((blocked_buyer_id = pe.actor_id))))) AND (NOT (EXISTS (SELECT 1 FROM buyer.pub_blocked_buyers() AS pub WHERE pub.blocker_buyer_id = pe.actor_id AND ((blocked_buyer_id = $7::bigint))))))) AND (pe.id = $8::uuid)
                ))
     )) AS "value";

-- args:
-- $1 = 42
-- $2 = 42
-- $3 = 018f0000-0000-7000-8000-000000000001
-- $4 = 7
-- $5 = 018f0000-0000-7000-8000-000000000003
-- $6 = 42
-- $7 = 42
-- $8 = 018f0000-0000-7000-8000-000000000003
