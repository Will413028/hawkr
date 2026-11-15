SELECT c.actor_kind AS "actor_kind",
     c.actor_id AS "actor_id",
     p.store_id AS "store_id"
FROM social.comments AS c
     INNER JOIN social.posts AS p ON (p.id = c.post_id)
WHERE ((c.id = $1::uuid) AND c.counted) AND ((p.store_id = $2::bigint) AND (p.deleted_at IS NULL));

-- args:
-- $1 = 018f0000-0000-7000-8000-000000000001
-- $2 = 7
