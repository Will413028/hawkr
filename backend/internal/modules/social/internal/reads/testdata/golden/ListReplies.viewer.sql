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
WHERE ((((((c.actor_kind != 'BUYER') OR ((NOT (EXISTS (SELECT 1 FROM buyer.pub_blocked_buyers() AS pub WHERE pub.blocker_buyer_id = $1::bigint AND ((blocked_buyer_id = c.actor_id))))) AND (NOT (EXISTS (SELECT 1 FROM buyer.pub_blocked_buyers() AS pub WHERE pub.blocker_buyer_id = c.actor_id AND ((blocked_buyer_id = $2::bigint))))))) AND (c.parent_id = $3::uuid)) AND c.counted) AND (((((p.status = 'PUBLISHED') AND (p.deleted_at IS NULL)) AND (NOT (EXISTS (
           SELECT td.post_id AS "td.post_id"
           FROM social.content_takedowns AS td
           WHERE td.post_id = p.id
      )))) AND (EXISTS (SELECT 1 FROM seller.pub_exposable_stores($4::timestamp with time zone) AS pub WHERE pub.store_id = p.store_id))) AND (NOT (EXISTS (SELECT 1 FROM buyer.pub_blocked_stores() AS pub WHERE pub.blocker_buyer_id = $5::bigint AND ((store_id = p.store_id))))))) AND (EXISTS (
           SELECT pc.id AS "pc.id"
           FROM social.comments AS pc
           WHERE ((pc.actor_kind != 'BUYER') OR ((NOT (EXISTS (SELECT 1 FROM buyer.pub_blocked_buyers() AS pub WHERE pub.blocker_buyer_id = $6::bigint AND ((blocked_buyer_id = pc.actor_id))))) AND (NOT (EXISTS (SELECT 1 FROM buyer.pub_blocked_buyers() AS pub WHERE pub.blocker_buyer_id = pc.actor_id AND ((blocked_buyer_id = $7::bigint))))))) AND (pc.id = $8::uuid)
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
