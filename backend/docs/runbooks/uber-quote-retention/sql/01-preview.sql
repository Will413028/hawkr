-- 01：唯讀盤點。跑這支之前不要跑 02。
-- DB：hawkr_shipping
-- 用法：psql -v ON_ERROR_STOP=1 -f 01-preview.sql
--
-- 🔴 本支**不刪任何東西**，它回答三個不同的問題，三者的處置完全不同：
--    A. 可刪的（終態且逾期）        → 02 會刪
--    B. 逾期但**非終態**            → 🔴 這是缺陷訊號，**不刪**，見 runbook §4
--    C. 孤兒 package_plan           → 02 會刪（前一輪中斷留下的殘骸）

\set retention_days 90

\echo ''
\echo '=== A. 可刪除：終態且已逾保留期的 quote ==========================='
SELECT status,
       count(*)        AS rows,
       min(created_at) AS oldest,
       max(created_at) AS newest
FROM uber_quotes
WHERE status IN ('CONSUMED', 'EXPIRED', 'SUPERSEDED')
  AND created_at < NOW() - (:retention_days || ' days')::interval
GROUP BY status
ORDER BY status;

\echo ''
\echo '=== B. 🔴 不可刪：逾保留期但仍非終態的 quote ======================='
\echo '    CREATED           逾期未被 TTL 掃描推成 EXPIRED ⇒ TTL 掃描沒在跑'
\echo '    CHECKOUT_ACCEPTED 逾期未 CONSUMED               ⇒ 收了錢沒建單（66 §5.1.4）'
\echo '    有列 ⇒ 停在這裡，先查為什麼，不要往下跑 02。'
SELECT status,
       count(*)        AS rows,
       min(created_at) AS oldest,
       max(created_at) AS newest
FROM uber_quotes
WHERE status IN ('CREATED', 'CHECKOUT_ACCEPTED')
  AND created_at < NOW() - (:retention_days || ' days')::interval
GROUP BY status
ORDER BY status;

\echo ''
\echo '=== 連帶會被刪掉的 package_plan（掛在 A 的那些 quote 上）==========='
SELECT count(*) AS package_plan_rows
FROM package_plan p
WHERE EXISTS (
    SELECT 1 FROM uber_quotes q
    WHERE q.quote_id = p.quote_id
      AND q.status IN ('CONSUMED', 'EXPIRED', 'SUPERSEDED')
      AND q.created_at < NOW() - (:retention_days || ' days')::interval);

\echo ''
\echo '=== C. 孤兒 package_plan（對應的 quote 已不存在）=================='
\echo '    正常應為 0：兩者由 UberQuoteWriter 在同一個交易寫入。'
\echo '    非 0 ⇒ 前一輪清理中途中斷，02 會一併收乾淨。'
SELECT count(*) AS orphan_rows
FROM package_plan p
WHERE NOT EXISTS (SELECT 1 FROM uber_quotes q WHERE q.quote_id = p.quote_id);

\echo ''
\echo '=== 目前佔用（判斷這輪值不值得跑）================================='
SELECT relname AS table_name,
       pg_size_pretty(pg_total_relation_size(relid)) AS total_size,
       n_live_tup AS approx_rows
FROM pg_stat_user_tables
WHERE relname IN ('uber_quotes', 'package_plan')
ORDER BY relname;
