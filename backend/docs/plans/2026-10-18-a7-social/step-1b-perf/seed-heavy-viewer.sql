
\set ON_ERROR_STOP on
\if :{?heavy} \else \set heavy 12346 \endif

INSERT INTO buyer.blocked_stores (blocker_buyer_id, store_id, created_at)
SELECT :heavy, g, now() FROM generate_series(5000, 5999) g
ON CONFLICT DO NOTHING;
INSERT INTO buyer.blocked_buyers (blocker_buyer_id, blocked_buyer_id, created_at)
SELECT :heavy, g, now() FROM generate_series(100, 1099) g
ON CONFLICT DO NOTHING;
ANALYZE buyer.blocked_stores;
ANALYZE buyer.blocked_buyers;
SELECT (SELECT count(*) FROM buyer.blocked_stores WHERE blocker_buyer_id = :heavy) AS heavy_store_blocks,
       (SELECT count(*) FROM buyer.blocked_buyers WHERE blocker_buyer_id = :heavy) AS heavy_buyer_blocks;
