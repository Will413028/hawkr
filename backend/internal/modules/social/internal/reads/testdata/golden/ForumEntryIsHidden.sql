SELECT (EXISTS (
          SELECT h.id AS "h.id"
          FROM social.hides AS h
          WHERE (h.forum_entry_id = $1::uuid) AND (h.restored_at IS NULL)
     )) AS "value";

-- args:
-- $1 = 018f0000-0000-7000-8000-000000000001
