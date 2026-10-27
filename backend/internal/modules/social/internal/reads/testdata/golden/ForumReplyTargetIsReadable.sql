SELECT (EXISTS (
          SELECT e.id AS "e.id"
          FROM social.forum_entries AS e
          WHERE (((e.id = $1::uuid) AND (e.store_id = $2::bigint)) AND (e.thread_id = $3::uuid)) AND e.counted
     )) AS "value";

-- args:
-- $1 = 018f0000-0000-7000-8000-000000000001
-- $2 = 7
-- $3 = 018f0000-0000-7000-8000-000000000003
