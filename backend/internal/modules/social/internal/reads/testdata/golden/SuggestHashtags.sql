SELECT h.name AS "name",
     COUNT(*) AS "post_count"
FROM social.post_hashtags AS h
     INNER JOIN social.posts AS p ON (p.id = h.post_id)
WHERE ((((p.status = 'PUBLISHED') AND (p.deleted_at IS NULL)) AND (NOT (EXISTS (
           SELECT td.post_id AS "td.post_id"
           FROM social.content_takedowns AS td
           WHERE td.post_id = p.id
      )))) AND (EXISTS (SELECT 1 FROM seller.pub_exposable_stores($1::timestamp with time zone) AS pub WHERE pub.store_id = p.store_id))) AND (h.name LIKE $2::text)
GROUP BY h.name
ORDER BY COUNT(*) DESC, h.name ASC
LIMIT $3;

-- args:
-- $1 = 2026-10-23 12:00:00 +0000 UTC
-- $2 = org%
-- $3 = 10
