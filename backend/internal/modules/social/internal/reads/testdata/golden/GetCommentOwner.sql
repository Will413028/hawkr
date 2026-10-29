SELECT c.actor_kind AS "actor_kind",
     c.actor_id AS "actor_id",
     p.store_id AS "store_id",
     (c.deleted_at IS NULL) AS "exists_now",
     (c.counted AND ((((p.status = 'PUBLISHED') AND (p.deleted_at IS NULL)) AND (NOT (EXISTS (
          SELECT td.post_id AS "td.post_id"
          FROM social.content_takedowns AS td
          WHERE td.post_id = p.id
     )))) AND (EXISTS (SELECT 1 FROM seller.pub_exposable_stores($1::timestamp with time zone) AS pub WHERE pub.store_id = p.store_id)))) AS "readable"
FROM social.comments AS c
     INNER JOIN social.posts AS p ON (p.id = c.post_id)
WHERE c.id = $2::uuid;

-- args:
-- $1 = 2026-10-23 12:00:00 +0000 UTC
-- $2 = 018f0000-0000-7000-8000-000000000001
