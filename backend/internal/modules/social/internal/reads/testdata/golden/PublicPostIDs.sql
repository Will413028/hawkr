SELECT p.id AS "id"
FROM social.posts AS p
WHERE ((((p.status = 'PUBLISHED') AND (p.deleted_at IS NULL)) AND (NOT (EXISTS (
           SELECT td.post_id AS "td.post_id"
           FROM social.content_takedowns AS td
           WHERE td.post_id = p.id
      )))) AND (EXISTS (SELECT 1 FROM seller.pub_exposable_stores($1::timestamp with time zone) AS pub WHERE pub.store_id = p.store_id))) AND (p.id IN ($2::uuid, $3::uuid));

-- args:
-- $1 = 2026-10-23 12:00:00 +0000 UTC
-- $2 = 018f0000-0000-7000-8000-000000000001
-- $3 = 018f0000-0000-7000-8000-000000000002
