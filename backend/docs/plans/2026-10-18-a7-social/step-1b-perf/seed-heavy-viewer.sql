-- A7-1b 的壓力變體：一位封鎖很多對象的觀看者（:heavy，預設 12346）封鎖 1,000 家冷門店（5000–5999 號）與 1,000 位買家（100–1099 號），
-- 用來看封鎖數成長 25 倍之後列表查詢的成本是否隨封鎖數線性增加。接在 seed-blocks.sql 之後執行。

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
