UPDATE social.forum_entry_stats
SET reply_count = v.new_reply
FROM (
          SELECT es.forum_entry_id AS "id",
               es.reply_count AS "old_reply",
               COALESCE(rp.n, $1) AS "new_reply"
          FROM social.forum_entry_stats AS es
               LEFT JOIN (
                    SELECT e.parent_id AS "key",
                         COUNT(*) AS "n"
                    FROM social.forum_entries AS e
                    WHERE ((e.parent_id > $2::uuid) AND (e.parent_id <= $3::uuid)) AND e.counted
                    GROUP BY e.parent_id
               ) AS rp ON (rp.key = es.forum_entry_id)
          WHERE (es.forum_entry_id > $4::uuid) AND (es.forum_entry_id <= $5::uuid)
     ) AS v
WHERE (forum_entry_stats.forum_entry_id = v.id) AND (v.old_reply != v.new_reply)
RETURNING forum_entry_stats.forum_entry_id AS "id",
          v.old_reply AS "old_reply",
          forum_entry_stats.reply_count AS "new_reply";

-- args:
-- $1 = 0
-- $2 = 018f0000-0000-7000-8000-000000000001
-- $3 = 018f0000-0000-7000-8000-000000000002
-- $4 = 018f0000-0000-7000-8000-000000000001
-- $5 = 018f0000-0000-7000-8000-000000000002
