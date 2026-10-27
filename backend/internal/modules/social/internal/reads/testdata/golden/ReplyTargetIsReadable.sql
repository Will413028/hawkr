SELECT (EXISTS (
          SELECT c.id AS "c.id"
          FROM social.comments AS c
          WHERE (((c.id = $1::uuid) AND (c.post_id = $2::uuid)) AND (c.thread_id = $3::uuid)) AND c.counted
     )) AS "value";

-- args:
-- $1 = 018f0000-0000-7000-8000-000000000001
-- $2 = 018f0000-0000-7000-8000-000000000002
-- $3 = 018f0000-0000-7000-8000-000000000003
