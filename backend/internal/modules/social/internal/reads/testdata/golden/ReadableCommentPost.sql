SELECT p.store_id AS "store_id"
FROM social.comments AS c
     INNER JOIN social.posts AS p ON (p.id = c.post_id)
WHERE ((c.id = $1::uuid) AND c.counted) AND (((p.status = 'PUBLISHED') AND (p.deleted_at IS NULL)) AND (EXISTS (SELECT 1 FROM seller.pub_exposable_stores($2::timestamp with time zone) AS pub WHERE pub.store_id = p.store_id)));

-- args:
-- $1 = 018f0000-0000-7000-8000-000000000001
-- $2 = 2026-10-23 12:00:00 +0000 UTC
