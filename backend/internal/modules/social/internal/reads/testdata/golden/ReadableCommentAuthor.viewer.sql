SELECT c.actor_kind AS "actor_kind",
     c.actor_id AS "actor_id",
     p.store_id AS "store_id"
FROM social.comments AS c
     INNER JOIN social.posts AS p ON (p.id = c.post_id)
WHERE (((((c.actor_kind != 'BUYER') OR (NOT (EXISTS (SELECT 1 FROM buyer.pub_blocked_buyers() AS pub WHERE pub.blocker_buyer_id = c.actor_id AND ((blocked_buyer_id = $1::bigint)))))) AND (c.id = $2::uuid)) AND c.counted) AND ((((p.status = 'PUBLISHED') AND (p.deleted_at IS NULL)) AND (NOT (EXISTS (
           SELECT td.post_id AS "td.post_id"
           FROM social.content_takedowns AS td
           WHERE td.post_id = p.id
      )))) AND (EXISTS (SELECT 1 FROM seller.pub_exposable_stores($3::timestamp with time zone) AS pub WHERE pub.store_id = p.store_id)))) AND ((c.parent_id IS NULL) OR (EXISTS (
           SELECT pc.id AS "pc.id"
           FROM social.comments AS pc
           WHERE ((pc.actor_kind != 'BUYER') OR (NOT (EXISTS (SELECT 1 FROM buyer.pub_blocked_buyers() AS pub WHERE pub.blocker_buyer_id = pc.actor_id AND ((blocked_buyer_id = $4::bigint)))))) AND (pc.id = c.parent_id)
      )));

-- args:
-- $1 = 42
-- $2 = 018f0000-0000-7000-8000-000000000001
-- $3 = 2026-10-23 12:00:00 +0000 UTC
-- $4 = 42
