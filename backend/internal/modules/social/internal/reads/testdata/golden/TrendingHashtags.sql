SELECT h.name AS "name",
     COUNT(*) AS "post_count"
FROM social.post_hashtags AS h
     INNER JOIN social.posts AS p ON (p.id = h.post_id)
WHERE (((p.status = 'PUBLISHED') AND (p.deleted_at IS NULL)) AND (EXISTS (SELECT 1 FROM seller.pub_exposable_stores($1::timestamp with time zone) AS pub WHERE pub.store_id = p.store_id))) AND ((p.published_at >= $2::timestamp with time zone) AND (p.published_at < $3::timestamp with time zone))
GROUP BY h.name
ORDER BY COUNT(*) DESC, h.name ASC
LIMIT $4;

-- args:
-- $1 = 2026-10-23 12:00:00 +0000 UTC
-- $2 = 2026-10-16 12:00:00 +0000 UTC
-- $3 = 2026-10-23 12:00:00 +0000 UTC
-- $4 = 20
