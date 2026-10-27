-- name: SeriesObservationsAsOf :many
-- 單一 (market, crop) 在 [from_date, to_date] 內、cutoff 當下可見的每日最新
-- revision（一天恰一筆）。
--
-- 🔴 DISTINCT ON + ORDER BY revision_no DESC 是本服務唯一的 bitemporal 解析器。
-- 等價於上游 storage/revisions.py:1048-1075 的
-- LEAD(knowledge_from) OVER (PARTITION BY series_id, trade_date ORDER BY revision_no)
-- 區間判定，前提是同一 (market, crop, trade_date) 的 revision_no 與 available_at
-- 同向遞增——由 ingest 保證（source_digest 不同才插 revision_no+1、available_at
-- 取當次 ingest 時間），並由 schema_invariants_test 釘住。
-- available_at <= cutoff 這個條件不可省：少了它，今天剛抓到的修訂會污染
-- 「昨天的我方視角」，任何 backtest 都有 look-ahead 洩漏。
SELECT DISTINCT ON (po.trade_date)
    m.market_code,
    c.crop_code,
    c.category_code,
    m.active AS market_active,
    c.active AS crop_active,
    po.trade_date,
    po.avg_price,
    po.volume_kg,
    po.available_at,
    po.revision_no
FROM price_observation po
JOIN market m ON m.id = po.market_id
JOIN crop c ON c.id = po.crop_id
WHERE m.market_code = sqlc.arg(market_code)
  AND c.crop_code = sqlc.arg(crop_code)
  AND po.trade_date BETWEEN sqlc.arg(from_date) AND sqlc.arg(to_date)
  AND po.available_at <= sqlc.arg(cutoff)
ORDER BY po.trade_date, po.revision_no DESC;

-- name: GroupObservationsAsOf :many
-- 同上，但涵蓋一個大類底下的所有子品項（可選市場過濾：market_code 傳空字串
-- 代表不過濾）。每個 (market, crop, trade_date) 恰一筆。
--
-- 🔴 DISTINCT ON 的鍵必須含 m.market_code：大類兩階段加權的第一階段是
-- 「同一市場內、對各子品項按成交量加權」，需要每個市場各自的價格。少了市場
-- 鍵時，同一個 (crop, trade_date) 若存在於多個市場只會回一列，其餘市場的觀測
-- 靜默消失，勝出者由「哪個市場剛好 revision_no 較大」決定、與價格無關——
-- 這條鍵也必須跟 RecentVolumeByCropMarket 子查詢的
-- DISTINCT ON (market_id, crop_id, trade_date) 對齊，否則下游會拿
-- per-market 的成交量權重去乘「只剩一個市場」的價格，算出來的加權必然錯。
-- 見 review Finding 1（2026-07-30）：隔離容器實測過少了市場鍵時 104/109 兩個
-- 市場的觀測會被 DISTINCT ON 併成一筆。
SELECT DISTINCT ON (m.market_code, c.crop_code, po.trade_date)
    m.market_code,
    c.crop_code,
    c.category_code,
    m.active AS market_active,
    c.active AS crop_active,
    po.trade_date,
    po.avg_price,
    po.volume_kg,
    po.available_at,
    po.revision_no
FROM price_observation po
JOIN market m ON m.id = po.market_id
JOIN crop c ON c.id = po.crop_id
JOIN crop_group g ON g.id = c.group_id
WHERE g.group_code = sqlc.arg(group_code)
  AND (sqlc.arg(market_code)::text = '' OR m.market_code = sqlc.arg(market_code)::text)
  AND po.trade_date BETWEEN sqlc.arg(from_date) AND sqlc.arg(to_date)
  AND po.available_at <= sqlc.arg(cutoff)
ORDER BY m.market_code, c.crop_code, po.trade_date, po.revision_no DESC;

-- name: RestDaysForMarket :many
-- 某市場在 [from_date, to_date] 內的休市日，按營業類別分開。
-- market_type IS NULL 的列是 Task 3 之前寫入的舊資料（跨類別聯集），一律排除
-- ——它們會被 RestDayIngest 刪掉，但在刪掉之前不能被當成任何類別的休市依據。
SELECT m.market_code, r.market_type, r.rest_date
FROM market_rest_day r
JOIN market m ON m.id = r.market_id
WHERE m.market_code = sqlc.arg(market_code)
  AND r.market_type IS NOT NULL
  AND r.rest_date BETWEEN sqlc.arg(from_date) AND sqlc.arg(to_date)
ORDER BY r.rest_date, r.market_type;

-- name: RestDayCoverageForMarket :many
-- 某市場有哪些 (營業類別, 年) 的休市日曆覆蓋。
-- 這是 VerifiedCalendar 的來源：設計書 §8 line 421 要求區分「這天沒休市」與
-- 「我們沒有這一年的日曆」，缺後者的日期一律回 insufficient_data。
-- 判準是「該 (市場, 類別, 年) 至少有一筆休市列」——MOA 的回應結構是
-- 市場 → MarketTypeList → YearList → MonthList，有列出該年就代表有覆蓋。
SELECT DISTINCT
    m.market_code,
    r.market_type,
    EXTRACT(YEAR FROM r.rest_date)::int AS calendar_year
FROM market_rest_day r
JOIN market m ON m.id = r.market_id
WHERE m.market_code = sqlc.arg(market_code)
  AND r.market_type IS NOT NULL
ORDER BY calendar_year, r.market_type;

-- name: CropsInGroup :many
SELECT c.crop_code, c.name, c.variant_name, c.category_code, c.active
FROM crop c
JOIN crop_group g ON g.id = c.group_id
WHERE g.group_code = sqlc.arg(group_code)
ORDER BY c.crop_code;

-- name: RecentVolumeByCropMarket :many
-- 大類加權用的成交量：每個 (market, crop) 在 [from_date, to_date] 內的成交量
-- 總和，只計每天最新 revision（否則同一天的多次修訂會被重複加總）。
-- NULL 成交量不計入（NULL 是「沒有成交/缺值」，不是 0）。
SELECT
    m.market_code,
    c.crop_code,
    SUM(GREATEST(latest.volume_kg, 0))::numeric(18,3) AS recent_volume
FROM (
    SELECT DISTINCT ON (po.market_id, po.crop_id, po.trade_date)
        po.market_id, po.crop_id, po.trade_date, po.volume_kg
    FROM price_observation po
    JOIN crop c2 ON c2.id = po.crop_id
    JOIN crop_group g2 ON g2.id = c2.group_id
    WHERE g2.group_code = sqlc.arg(group_code)
      AND po.trade_date BETWEEN sqlc.arg(from_date) AND sqlc.arg(to_date)
      AND po.available_at <= sqlc.arg(cutoff)
    ORDER BY po.market_id, po.crop_id, po.trade_date, po.revision_no DESC
) latest
JOIN market m ON m.id = latest.market_id
JOIN crop c ON c.id = latest.crop_id
WHERE latest.volume_kg IS NOT NULL
GROUP BY m.market_code, c.crop_code
ORDER BY m.market_code, c.crop_code;

-- name: RecentVolumeByCrop :many
-- RecentVolumeByCropMarket 的單一品項版：同一套 window/cutoff/latest-revision
-- 語意，少了 crop_group JOIN，直接用 crop_code 篩選——單一品項路徑沒有大類
-- 可以拆子品項，回傳一列一個 market 就夠（不需要 crop_code 欄位，呼叫端已經
-- 知道自己查的是哪個 crop）。
SELECT
    m.market_code,
    SUM(GREATEST(latest.volume_kg, 0))::numeric(18,3) AS recent_volume
FROM (
    SELECT DISTINCT ON (po.market_id, po.trade_date)
        po.market_id, po.trade_date, po.volume_kg
    FROM price_observation po
    JOIN crop c2 ON c2.id = po.crop_id
    WHERE c2.crop_code = sqlc.arg(crop_code)
      AND po.trade_date BETWEEN sqlc.arg(from_date) AND sqlc.arg(to_date)
      AND po.available_at <= sqlc.arg(cutoff)
    ORDER BY po.market_id, po.trade_date, po.revision_no DESC
) latest
JOIN market m ON m.id = latest.market_id
WHERE latest.volume_kg IS NOT NULL
GROUP BY m.market_code
ORDER BY m.market_code;

-- name: FindCropGroupByName :many
-- mapping 種子表解析率檢查用（Task 13，cmd/pricebook-seed-check）。
--
-- 🔴 是 :many 不是 :one：Task 2 之後 GroupCode 的 hash 含 category_code
-- （moa.GroupCode(groupName, categoryCode)），所以同一個大類名在不同 category
-- 下是不同的 crop_group 列——「甘藍」理論上可能同時存在於 N04（蔬菜）與
-- N05（水果）。回多筆時代表種子表的 crop_group_name 單靠名稱不足以唯一定位
-- 到一列，呼叫端（seed-check）要把全部列出來標記為需人工裁決，不能自己選
-- 一筆——這正是 seed-check 存在的目的：發現這類需要裁決的情況，不是掩蓋它。
SELECT group_code, name, category_code FROM crop_group WHERE name = sqlc.arg(name) ORDER BY group_code;

-- name: FindCropByCode :one
-- mapping 種子表解析率檢查用（Task 13）。crop_code 在 schema 上是
-- UNIQUE（db/schema/schema.sql），所以查無或恰一筆，不會有 :many 的疑慮。
SELECT crop_code, name, category_code FROM crop WHERE crop_code = sqlc.arg(crop_code);

-- name: ListGroupCropOptionsByRecentVolume :many
-- 一個大類底下的子品項，依「近期實際成交筆數」由多到少排序（ListGroupCropOptions
-- RPC 用）。跟 surface_form.sql 的 SearchSurfaceForms 的差別是 join 條件：那支是名稱
-- 比對，這支走 group_id 這條真正的父子邊，所以不會漏掉名字裡沒有大類名的子品項，
-- 也不會撈到別的大類（查「甘藍」時名稱比對會一起撈到球莖甘藍的子品項）。
--
-- LEFT JOIN 不是 INNER：近期完全沒成交的子品項要排在最後、不是消失。這是刻意的——
-- 淡季品項用 INNER JOIN 會整個不見，而排序本身已經把它們沉到底，再加一道新鮮度門檻
-- 只會讓「這個大類到底有哪些選擇」這個問題答得不完整。
--
-- window_days 由呼叫端給（usecase 有預設值），不寫死在 SQL 裡：這是排序用的人氣近似
-- 值，不是業務規則，調窗格不該改動 schema 層。
--
-- crop_code 是 tie-break，讓「近期都是 0 筆」的尾巴有穩定順序——少了它，同分列的順序
-- 由 Postgres 自由決定，同一個大類連續兩次呼叫可能給出不同的卡片。
SELECT c.crop_code, c.name, COALESCE(c.variant_name, '') AS variant_name, g.group_code, g.name AS group_name
FROM crop c
JOIN crop_group g ON g.id = c.group_id
LEFT JOIN price_observation o
  ON o.crop_id = c.id
  AND o.trade_date >= CURRENT_DATE - sqlc.arg(window_days)::int
WHERE c.active
  AND g.active
  AND g.group_code = sqlc.arg(group_code)::text
GROUP BY c.crop_code, c.name, c.variant_name, g.group_code, g.name
ORDER BY count(o.id) DESC, c.crop_code
LIMIT sqlc.arg(row_limit);
