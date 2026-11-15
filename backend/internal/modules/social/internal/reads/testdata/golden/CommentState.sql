SELECT (CASE WHEN c.counted THEN 'VISIBLE' WHEN (c.deleted_at IS NOT NULL) OR (EXISTS (
          SELECT ia.buyer_id AS "ia.buyer_id"
          FROM social.inactive_authors AS ia
          WHERE (ia.buyer_id = c.actor_id) AND (c.actor_kind = 'BUYER')
     )) THEN 'DELETED' ELSE 'HIDDEN' END) AS "state"
FROM social.comments AS c
WHERE c.id = $1::uuid;

-- args:
-- $1 = 018f0000-0000-7000-8000-000000000001
