-- 從既有資料推導出最近 40 天的攝取分區，讓分區模型上線的當下就反映真實狀態。
--
-- 沒有這一段的話會有一個安靜的回歸：ingest_partition 上線時是空的，而
-- hawkr_pricebook_market_unrecoverable_gap_days 改讀它之後會一律回 0
-- ——已知的 7 個永久缺口（含 2026-07-24 全 21 個市場）會從告警上消失，看起來
-- 像是問題被修好了。排程只宣告最近 DeclareLookbackDays 天，補不回這一段。
--
-- 判定沿用 usecase 的同一套語意，只是資料來源是我方既有的表而非上游回應：
--   有 price_observation           → complete（completed_at 取該格最早的寫入時間）
--   無資料但 market_rest_day 有記  → rest_day
--   兩者皆無                       → abandoned（已滑出舊的 windowDays 窗口，
--                                    排程再也不會去抓，正是需要人看的那種）
--
-- 40 天：與 PR #3044 那版 metric 的回看範圍一致，讓上線前後的數字可對照。
-- 上界取 CURRENT_DATE - 7，跟排程的宣告區間銜接，中間不重不漏。
--
-- 冪等：ON CONFLICT DO NOTHING；重跑或與排程宣告競爭都不會改寫既有格子。
-- 全新環境（market/price_observation 皆空）插入 0 列，安全。
INSERT INTO ingest_partition (market_id, trade_date, status, attempt_count, declared_at, completed_at)
SELECT m.id,
       d.trade_date,
       CASE
           WHEN obs.first_at IS NOT NULL THEN 2  -- PartitionStatusComplete
           WHEN EXISTS (SELECT 1 FROM market_rest_day r
                        WHERE r.market_id = m.id AND r.rest_date = d.trade_date) THEN 3  -- RestDay
           ELSE 4  -- Abandoned
       END,
       0,
       now(),
       obs.first_at
FROM market m
CROSS JOIN LATERAL (
    SELECT generate_series(CURRENT_DATE - 40, CURRENT_DATE - 7, interval '1 day')::date AS trade_date
) d
LEFT JOIN LATERAL (
    SELECT min(po.available_at) AS first_at
    FROM price_observation po
    WHERE po.market_id = m.id AND po.trade_date = d.trade_date
) obs ON TRUE
WHERE m.active
ON CONFLICT (market_id, trade_date) DO NOTHING;
