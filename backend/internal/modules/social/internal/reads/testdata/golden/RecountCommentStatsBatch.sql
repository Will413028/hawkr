UPDATE social.comment_stats
SET (like_count, reply_count) = (v.new_like, v.new_reply)
FROM (
          SELECT cs.comment_id AS "id",
               cs.like_count AS "old_like",
               cs.reply_count AS "old_reply",
               COALESCE(lk.n, $1) AS "new_like",
               COALESCE(rp.n, $2) AS "new_reply"
          FROM social.comment_stats AS cs
               LEFT JOIN (
                    SELECT l.comment_id AS "key",
                         COUNT(*) AS "n"
                    FROM social.comment_likes AS l
                    WHERE (l.comment_id > $3::uuid) AND (l.comment_id <= $4::uuid)
                    GROUP BY l.comment_id
               ) AS lk ON (lk.key = cs.comment_id)
               LEFT JOIN (
                    SELECT c.parent_id AS "key",
                         COUNT(*) AS "n"
                    FROM social.comments AS c
                    WHERE ((c.parent_id > $5::uuid) AND (c.parent_id <= $6::uuid)) AND c.counted
                    GROUP BY c.parent_id
               ) AS rp ON (rp.key = cs.comment_id)
          WHERE (cs.comment_id > $7::uuid) AND (cs.comment_id <= $8::uuid)
     ) AS v
WHERE (comment_stats.comment_id = v.id) AND ((v.old_like != v.new_like) OR (v.old_reply != v.new_reply))
RETURNING comment_stats.comment_id AS "id",
          v.old_like AS "old_like",
          v.old_reply AS "old_reply",
          comment_stats.like_count AS "new_like",
          comment_stats.reply_count AS "new_reply";

-- args:
-- $1 = 0
-- $2 = 0
-- $3 = 018f0000-0000-7000-8000-000000000001
-- $4 = 018f0000-0000-7000-8000-000000000002
-- $5 = 018f0000-0000-7000-8000-000000000001
-- $6 = 018f0000-0000-7000-8000-000000000002
-- $7 = 018f0000-0000-7000-8000-000000000001
-- $8 = 018f0000-0000-7000-8000-000000000002
