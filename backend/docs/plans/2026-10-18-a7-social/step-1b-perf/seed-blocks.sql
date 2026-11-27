
\set ON_ERROR_STOP on
\if :{?viewer} \else \set viewer 12345 \endif

SELECT setseed(0.42);

INSERT INTO buyer.blocked_stores (blocker_buyer_id, store_id, created_at)
SELECT :viewer, g, now() FROM generate_series(2, 21) g
UNION ALL SELECT :viewer, g, now() FROM generate_series(5000, 5019) g;

INSERT INTO buyer.blocked_buyers (blocker_buyer_id, blocked_buyer_id, created_at)
SELECT :viewer, g, now() FROM generate_series(1, 40) g
UNION ALL SELECT g, :viewer, now() FROM generate_series(41, 60) g;

INSERT INTO buyer.blocked_stores (blocker_buyer_id, store_id, created_at)
SELECT 61 + floor(random() * 49000)::bigint, 1 + floor(random() * 10000)::bigint, now()
FROM generate_series(1, 10000)
ON CONFLICT DO NOTHING;

INSERT INTO buyer.blocked_buyers (blocker_buyer_id, blocked_buyer_id, created_at)
SELECT b, t, now()
FROM (SELECT 61 + floor(random() * 49000)::bigint AS b, 1 + floor(random() * 10000)::bigint AS t
      FROM generate_series(1, 10000)) r
WHERE b <> t
ON CONFLICT DO NOTHING;

ANALYZE buyer.blocked_stores;
ANALYZE buyer.blocked_buyers;
SELECT (SELECT count(*) FROM buyer.blocked_stores) AS store_blocks, (SELECT count(*) FROM buyer.blocked_buyers) AS buyer_blocks,
       (SELECT count(*) FROM buyer.blocked_stores WHERE blocker_buyer_id = :viewer) AS viewer_store_blocks,
       (SELECT count(*) FROM buyer.blocked_buyers WHERE blocker_buyer_id = :viewer) AS viewer_buyer_blocks,
       (SELECT count(*) FROM buyer.blocked_buyers WHERE blocked_buyer_id = :viewer) AS blocking_viewer;
