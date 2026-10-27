SELECT e.actor_kind AS "actor_kind",
     e.actor_id AS "actor_id",
     e.store_id AS "store_id",
     (e.deleted_at IS NULL) AS "exists_now",
     (e.counted AND (EXISTS (SELECT 1 FROM seller.pub_exposable_stores($1::timestamp with time zone) AS pub WHERE pub.store_id = e.store_id))) AS "readable"
FROM social.forum_entries AS e
WHERE e.id = $2::uuid;

-- args:
-- $1 = 2026-10-23 12:00:00 +0000 UTC
-- $2 = 018f0000-0000-7000-8000-000000000001
