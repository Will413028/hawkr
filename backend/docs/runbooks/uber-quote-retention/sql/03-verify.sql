-- 03：COMMIT 之後的覆核。唯讀。
-- DB：hawkr_shipping
-- 用法：psql -v ON_ERROR_STOP=1 -f 03-verify.sql
--
-- 🔴 三項都必須為 0。任一項非 0 ⇒ 這一輪沒有做完，不要在交接或票上寫「已清理」。

\set retention_days 90

\echo '=== ① 逾期的終態 quote 殘留（須為 0）==='
SELECT count(*) AS remaining_expired_terminal
FROM uber_quotes
WHERE status IN ('CONSUMED', 'EXPIRED', 'SUPERSEDED')
  AND created_at < NOW() - (:retention_days || ' days')::interval;

\echo '=== ② 孤兒 package_plan（須為 0）==='
SELECT count(*) AS orphan_package_plan
FROM package_plan p
WHERE NOT EXISTS (SELECT 1 FROM uber_quotes q WHERE q.quote_id = p.quote_id);

\echo '=== ③ 逾期非終態（須為 0；非 0 ⇒ 缺陷，見 runbook §4）==='
SELECT count(*) AS overdue_non_terminal
FROM uber_quotes
WHERE status IN ('CREATED', 'CHECKOUT_ACCEPTED')
  AND created_at < NOW() - (:retention_days || ' days')::interval;

\echo '=== 清理後佔用 ==='
SELECT relname AS table_name,
       pg_size_pretty(pg_total_relation_size(relid)) AS total_size,
       n_live_tup AS approx_rows
FROM pg_stat_user_tables
WHERE relname IN ('uber_quotes', 'package_plan')
ORDER BY relname;
