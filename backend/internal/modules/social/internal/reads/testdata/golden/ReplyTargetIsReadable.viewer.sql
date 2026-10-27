SELECT (EXISTS (
          SELECT c.id AS "c.id"
          FROM social.comments AS c
          WHERE ((((((c.actor_kind != 'BUYER') OR ((NOT (EXISTS (SELECT 1 FROM buyer.pub_blocked_buyers() AS pub WHERE pub.blocker_buyer_id = $1::bigint AND ((blocked_buyer_id = c.actor_id))))) AND (NOT (EXISTS (SELECT 1 FROM buyer.pub_blocked_buyers() AS pub WHERE pub.blocker_buyer_id = c.actor_id AND ((blocked_buyer_id = $2::bigint))))))) AND (c.id = $3::uuid)) AND (c.post_id = $4::uuid)) AND (c.thread_id = $5::uuid)) AND c.counted) AND (EXISTS (
                     SELECT pc.id AS "pc.id"
                     FROM social.comments AS pc
                     WHERE ((pc.actor_kind != 'BUYER') OR ((NOT (EXISTS (SELECT 1 FROM buyer.pub_blocked_buyers() AS pub WHERE pub.blocker_buyer_id = $6::bigint AND ((blocked_buyer_id = pc.actor_id))))) AND (NOT (EXISTS (SELECT 1 FROM buyer.pub_blocked_buyers() AS pub WHERE pub.blocker_buyer_id = pc.actor_id AND ((blocked_buyer_id = $7::bigint))))))) AND (pc.id = $8::uuid)
                ))
     )) AS "value";

-- args:
-- $1 = 42
-- $2 = 42
-- $3 = 018f0000-0000-7000-8000-000000000001
-- $4 = 018f0000-0000-7000-8000-000000000002
-- $5 = 018f0000-0000-7000-8000-000000000003
-- $6 = 42
-- $7 = 42
-- $8 = 018f0000-0000-7000-8000-000000000003
