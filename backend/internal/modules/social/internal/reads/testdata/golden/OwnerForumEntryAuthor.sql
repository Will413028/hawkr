SELECT e.actor_kind AS "actor_kind",
     e.actor_id AS "actor_id",
     e.store_id AS "store_id"
FROM social.forum_entries AS e
WHERE ((e.id = $1::uuid) AND (e.store_id = $2::bigint)) AND e.counted;

-- args:
-- $1 = 018f0000-0000-7000-8000-000000000001
-- $2 = 7
