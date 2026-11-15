SELECT (CASE WHEN e.counted THEN 'VISIBLE' WHEN (e.deleted_at IS NOT NULL) OR (EXISTS (
          SELECT ia.buyer_id AS "ia.buyer_id"
          FROM social.inactive_authors AS ia
          WHERE (ia.buyer_id = e.actor_id) AND (e.actor_kind = 'BUYER')
     )) THEN 'DELETED' ELSE 'HIDDEN' END) AS "state"
FROM social.forum_entries AS e
WHERE e.id = $1::uuid;

-- args:
-- $1 = 018f0000-0000-7000-8000-000000000001
