UPDATE social.post_stats
SET (like_count, comment_count, share_count) = (v.new_like, v.new_comment, v.new_share)
FROM (
          SELECT ps.post_id AS "id",
               ps.like_count AS "old_like",
               ps.comment_count AS "old_comment",
               ps.share_count AS "old_share",
               COALESCE(lk.n, $1) AS "new_like",
               COALESCE(cc.n, $2) AS "new_comment",
               COALESCE(shs.n, $3) AS "new_share"
          FROM social.post_stats AS ps
               LEFT JOIN (
                    SELECT l.post_id AS "key",
                         COUNT(*) AS "n"
                    FROM social.post_likes AS l
                    WHERE (l.post_id > $4::uuid) AND (l.post_id <= $5::uuid)
                    GROUP BY l.post_id
               ) AS lk ON (lk.key = ps.post_id)
               LEFT JOIN (
                    SELECT sh.post_id AS "key",
                         COUNT(*) AS "n"
                    FROM social.post_shares AS sh
                    WHERE (sh.post_id > $6::uuid) AND (sh.post_id <= $7::uuid)
                    GROUP BY sh.post_id
               ) AS shs ON (shs.key = ps.post_id)
               LEFT JOIN (
                    SELECT c.post_id AS "key",
                         COUNT(*) AS "n"
                    FROM social.comments AS c
                    WHERE ((c.post_id > $8::uuid) AND (c.post_id <= $9::uuid)) AND c.counted
                    GROUP BY c.post_id
               ) AS cc ON (cc.key = ps.post_id)
          WHERE (ps.post_id > $10::uuid) AND (ps.post_id <= $11::uuid)
     ) AS v
WHERE (post_stats.post_id = v.id) AND (((v.old_like != v.new_like) OR (v.old_comment != v.new_comment)) OR (v.old_share != v.new_share))
RETURNING post_stats.post_id AS "id",
          v.old_like AS "old_like",
          v.old_comment AS "old_comment",
          v.old_share AS "old_share",
          post_stats.like_count AS "new_like",
          post_stats.comment_count AS "new_comment",
          post_stats.share_count AS "new_share";

-- args:
-- $1 = 0
-- $2 = 0
-- $3 = 0
-- $4 = 018f0000-0000-7000-8000-000000000001
-- $5 = 018f0000-0000-7000-8000-000000000002
-- $6 = 018f0000-0000-7000-8000-000000000001
-- $7 = 018f0000-0000-7000-8000-000000000002
-- $8 = 018f0000-0000-7000-8000-000000000001
-- $9 = 018f0000-0000-7000-8000-000000000002
-- $10 = 018f0000-0000-7000-8000-000000000001
-- $11 = 018f0000-0000-7000-8000-000000000002
