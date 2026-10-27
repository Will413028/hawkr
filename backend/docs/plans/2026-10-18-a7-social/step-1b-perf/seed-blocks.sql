-- A7-1b 封鎖查詢的量測資料：接在 step-1a-perf/seed.sql 之後執行（同一個用完即丟的 database，已套用單體的 migration）。
-- 觀看者是買家 :viewer（預設 12345，營運中）：封鎖 40 家店（2–21 號加 5000–5019 號，共占已發布貼文的 3.46%；不封鎖 1 號，那是最熱門的店，量測要看
-- 「封鎖了一些店，但仍在熱門店的列表裡翻頁」）、封鎖 40 位買家（1–40 號，只占買家留言的 1.54%，不是最活躍的留言者）、被 20 位買家封鎖（41–60 號）。另外加
-- 其他買家的隨機封鎖（買家封鎖店與買家封鎖買家各 10,000 筆），讓兩張表（buyer.blocked_stores、buyer.blocked_buyers）不是只有觀看者的列。用法見 step-1b-perf/README.md。

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
