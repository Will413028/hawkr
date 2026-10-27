-- name: UpsertMarket :one
INSERT INTO market (id, market_code, name)
VALUES ($1, $2, $3)
ON CONFLICT (market_code) DO UPDATE SET name = EXCLUDED.name, updated_at = now()
RETURNING id;

-- name: GetMarketByCode :one
-- 市場代碼 → 顯示名稱，供 SearchCropCatalog（子類的 markets 清單）與
-- MarketComparisonEntry（跨市場比較）標籤用。market_code 是 UNIQUE
-- （db/schema/schema.sql），查無或恰一筆。
SELECT market_code, name FROM market WHERE market_code = sqlc.arg(market_code);

-- name: GetMarketsByCode :many
-- GetMarketByCode 的批次版。SearchCropCatalog 要為「整個結果集」裡每個子類
-- 標出市場清單，逐一 GetMarketByCode 等於每次按鍵都對 DB 扇出 N 次查詢
-- （N = limit x 每個子類的市場數）——review Finding I3。呼叫端先把整批需要的
-- market_code 去重排序，這裡一次拿回。
--
-- 查無的代碼不回列（不是回空名字的假列）：呼叫端沿用單筆版的 fallback，
-- 顯示代碼本身，與 GetMarketByCode 查無時回零值同一個「解析不到是業務結果」
-- 立場。陣列參數寫法比照 monorepo 既有慣例（social-service
-- db/queries/hashtag.sql 的 = ANY(sqlc.arg(...)::type[])）。
SELECT market_code, name FROM market WHERE market_code = ANY(sqlc.arg(market_codes)::text[]);

-- name: UpsertCropGroup :one
INSERT INTO crop_group (id, group_code, name, category_code)
VALUES ($1, $2, $3, $4)
ON CONFLICT (group_code) DO UPDATE
  SET name = EXCLUDED.name, category_code = EXCLUDED.category_code, updated_at = now()
RETURNING id;

-- name: UpsertCrop :one
INSERT INTO crop (id, crop_code, group_id, name, variant_name, category_code)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (crop_code) DO UPDATE
  SET name = EXCLUDED.name, group_id = EXCLUDED.group_id,
      variant_name = EXCLUDED.variant_name, category_code = EXCLUDED.category_code,
      updated_at = now()
RETURNING id;

-- name: LatestObservationRevision :one
SELECT revision_no, source_digest
FROM price_observation
WHERE market_id = $1 AND crop_id = $2 AND trade_date = $3
ORDER BY revision_no DESC
LIMIT 1;

-- name: InsertPriceObservation :exec
INSERT INTO price_observation (
    id, market_id, crop_id, trade_date, revision_no, available_at,
    avg_price, upper_price, middle_price, lower_price, volume_kg,
    source, source_digest
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13);

-- name: LatestObservationRevisionsByMarket :many
-- ingestBatch 用：一次撈回整批 (crop_id, trade_date) 在指定 market 下的最新
-- revision_no + source_digest。DISTINCT ON 取每個 key 的最大 revision_no，
-- 對應 LatestObservationRevision 單筆版「ORDER BY revision_no DESC LIMIT 1」
-- 的語意，只是一次處理整批 key。
--
-- 🔴 crop_ids/trade_dates 是兩個等長、逐一對應的平行陣列（呼叫端保證），
-- 不是各自獨立的集合——這裡用 unnest 在 SELECT list 中對齊配對（Postgres
-- 對多個等長 set-returning function 的既有 lockstep 展開行為）。陣列裡
-- 出現重複 (crop_id, trade_date) pair 是合法輸入，join 出的重複列會被
-- DISTINCT ON 收斂，不影響正確性。
SELECT DISTINCT ON (po.crop_id, po.trade_date)
    po.crop_id, po.trade_date, po.revision_no, po.source_digest
FROM price_observation po
JOIN (
    SELECT unnest(sqlc.arg(crop_ids)::bigint[]) AS crop_id,
           unnest(sqlc.arg(trade_dates)::date[]) AS trade_date
) keys ON keys.crop_id = po.crop_id AND keys.trade_date = po.trade_date
WHERE po.market_id = sqlc.arg(market_id)
ORDER BY po.crop_id, po.trade_date, po.revision_no DESC;

-- name: BatchInsertPriceObservation :exec
-- 單筆版 InsertPriceObservation 的批次版（呼叫端見 PriceIngest.ingestBatch），
-- 欄位、語意完全相同（append-only，呼叫端已保證 revision_no 不衝突），差別
-- 只在一次寫入整批。
-- 13 個陣列參數彼此等長、依索引逐列對應，呼叫端負責保證。
INSERT INTO price_observation (
    id, market_id, crop_id, trade_date, revision_no, available_at,
    avg_price, upper_price, middle_price, lower_price, volume_kg,
    source, source_digest
)
SELECT * FROM ROWS FROM (
    unnest(sqlc.arg(ids)::bigint[]),
    unnest(sqlc.arg(market_ids)::bigint[]),
    unnest(sqlc.arg(crop_ids)::bigint[]),
    unnest(sqlc.arg(trade_dates)::date[]),
    unnest(sqlc.arg(revision_nos)::int[]),
    unnest(sqlc.arg(available_ats)::timestamptz[]),
    unnest(sqlc.arg(avg_prices)::numeric[]),
    unnest(sqlc.arg(upper_prices)::numeric[]),
    unnest(sqlc.arg(middle_prices)::numeric[]),
    unnest(sqlc.arg(lower_prices)::numeric[]),
    unnest(sqlc.arg(volume_kgs)::numeric[]),
    unnest(sqlc.arg(sources)::text[]),
    unnest(sqlc.arg(source_digests)::text[])
) AS t(id, market_id, crop_id, trade_date, revision_no, available_at,
       avg_price, upper_price, middle_price, lower_price, volume_kg, source, source_digest);

-- name: UpsertMarketRestDay :exec
INSERT INTO market_rest_day (id, market_id, market_type, rest_date, reason, fetched_at)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (market_id, market_type, rest_date) DO UPDATE SET fetched_at = EXCLUDED.fetched_at;

-- name: DeleteLegacyRestDaysWithoutMarketType :execrows
-- Task 3 之前寫入的列沒有 market_type 維度（等於跨營業類別的聯集，會把只有
-- 花卉休市的那些天對蔬菜也標成休市）。這些列不會被任何讀取查詢命中
-- （讀取端一律用 market_type = $n），但留著會讓 SELECT count(*) 之類的人工
-- 排查得到誤導性的數字。休市日曆每次 ingest 都是整批重抓（單次 791 KB 拿完
-- 全部市場全部年份），所以刪掉是安全的——下一輪就會用正確的維度重新寫入。
-- 這是一次性清理，第二次之後回傳 0 列，不是常駐的 reconciliation job。
DELETE FROM market_rest_day WHERE market_type IS NULL;

-- name: CreateIngestRun :one
INSERT INTO ingest_run (id, kind, range_start, range_end, status, started_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id;

-- name: FinishIngestRun :exec
UPDATE ingest_run
SET status = $2, fetched_count = $3, inserted_count = $4,
    revised_count = $5, error_message = $6, finished_at = $7
WHERE id = $1;

-- name: GetIngestRunByID :one
-- 測試專用：查單筆 ingest_run 稽核列，驗證 status/error_message。
SELECT * FROM ingest_run WHERE id = sqlc.arg(id);

-- name: ExportObservations :many
-- Task 9 對帳 gate 用：對 [start_date, end_date]（含頭尾）內每個
-- (market_id, crop_id, trade_date) series，只取 revision_no 最大的那一列——
-- 即該 series 目前最新已知值，不是每一次修訂都吐出來。相關子查詢刻意不重覆
-- 加上 trade_date 範圍限制：po2 已用 po2.trade_date = po.trade_date 鎖死成
-- 與外層同一天，範圍限制只在外層 WHERE 做一次即可。
-- 排序交給呼叫端的 WriteObservationCSV（Go 端就地重排並固定小數位），這裡的
-- ORDER BY 只是方便人工檢查產出，不是排序的唯一保證來源。
SELECT m.market_code, c.crop_code, po.trade_date, po.avg_price, po.volume_kg
FROM price_observation po
JOIN market m ON m.id = po.market_id
JOIN crop c ON c.id = po.crop_id
WHERE po.trade_date BETWEEN sqlc.arg(start_date) AND sqlc.arg(end_date)
  AND po.revision_no = (
    SELECT MAX(po2.revision_no) FROM price_observation po2
    WHERE po2.market_id = po.market_id AND po2.crop_id = po.crop_id AND po2.trade_date = po.trade_date
  )
ORDER BY m.market_code, c.crop_code, po.trade_date;

-- name: ObservedPriceByCrop :many
-- 給 GetObservedPrice 用：回傳一段區間內、每個 (market, crop, trade_date)
-- 恰一筆的原始觀測列，含 avg/upper/middle/lower 四個價格欄位——這是與
-- forecast.sql 的 SeriesObservationsAsOf 唯一的差異（那支只選 avg_price，
-- 因為 P4 推論只需要 avg 當 ridge 特徵）。不做任何聚合，同一 DISTINCT ON
-- 樣式（見 SeriesObservationsAsOf 的說明）。
--
-- cutoff 語意與 SeriesObservationsAsOf 完全一致（point-in-time：只看
-- available_at <= cutoff 的列）。不可用 to_date 推導（先前版本是
-- `to_date::timestamptz + interval '1 day'`）：呼叫端（usecase 層）在
-- trade_date 被 pin 到過去時，to 會等於那個過去日期，但 available_at 是
-- ingest/backfill 時間，與 trade_date 無關——cron 每天回補
-- [today-3, today] 視窗做自我修復，trade_date=T 的列 available_at 落在
-- T+1 天以後是設計上會發生的情況（見 price_ingest.go）。把 cutoff 收斂成
-- to+1 天，會讓 pin 過去日期的查詢對合法存在的資料回空結果，且無法與
-- 「真的沒有資料」區分——review Finding I1（2026-07-31）。呼叫端一律傳
-- now，不是 to。
--
-- market_code 支援空字串 bypass（同 ObservedPriceByGroup），對應 proto
-- GetObservedPriceRequest.market_code 的「Omitted = all markets」語意——
-- review Finding I2（2026-07-31）：先前這裡是裸的 `=` 比對，crop_code 專屬
-- 查詢在 market_code 留空時會回 0 列。DISTINCT ON 鍵一併加回 m.market_code
-- （原本只有 po.trade_date）：bypass 後同一 trade_date 可能同時有多個市場
-- 的觀測，若不把 market_code 放進 DISTINCT ON，會被併成一列、由哪個市場
-- 勝出未定義——與 GroupObservationsAsOf 的 DISTINCT ON 鍵必須含
-- market_code 是同一個道理（見該查詢註解、forecast.sql review Finding 1，
-- 2026-07-30）。crop_code 本身在這支查詢是固定值（不 bypass），故不需要
-- 進 DISTINCT ON 鍵。
--
-- source/source_digest/available_at/revision_no（issue #2249 Slice A
-- dataMeta 前置，2026-08-07 加）：GetObservedPrice 的 dataMeta 需要這四欄
-- 才能算 datasetDigest（Rule 3 跨列政策）與 freshnessStatus（MAX
-- available_at 餵 forecast.FreshnessConfig.Classify）——與 avg/upper/
-- middle/lower 四個價格欄位同一批新增，理由相同：這支查詢是 dataMeta 唯一
-- 能取得「實際回傳列」真身分的地方，晚一步在 usecase 層另外查會跟 Snapshot
-- 一致性規則（dataMeta 必須描述同一份 payload）衝突。
SELECT DISTINCT ON (m.market_code, po.trade_date)
    m.market_code,
    c.crop_code,
    c.category_code,
    po.trade_date,
    po.avg_price,
    po.upper_price,
    po.middle_price,
    po.lower_price,
    po.volume_kg,
    po.source,
    po.source_digest,
    po.available_at,
    po.revision_no
FROM price_observation po
JOIN market m ON m.id = po.market_id
JOIN crop c ON c.id = po.crop_id
WHERE (sqlc.arg(market_code)::text = '' OR m.market_code = sqlc.arg(market_code)::text)
  AND c.crop_code = sqlc.arg(crop_code)
  AND po.trade_date BETWEEN sqlc.arg(from_date) AND sqlc.arg(to_date)
  AND po.available_at <= sqlc.arg(cutoff)
ORDER BY m.market_code, po.trade_date, po.revision_no DESC;

-- name: ObservedPriceByGroup :many
-- 同上，但涵蓋一個大類底下的所有子品項（market_code 傳空字串代表不過濾，
-- 語意與 GroupObservationsAsOf 一致：每個市場各自的最新觀測，不合併）。
-- cutoff 語意同 ObservedPriceByCrop 上方的說明（point-in-time，呼叫端傳
-- now，不是 to——review Finding I1）。source/source_digest/available_at/
-- revision_no 的新增理由同 ObservedPriceByCrop 上方註解。
SELECT DISTINCT ON (m.market_code, c.crop_code, po.trade_date)
    m.market_code,
    c.crop_code,
    c.category_code,
    po.trade_date,
    po.avg_price,
    po.upper_price,
    po.middle_price,
    po.lower_price,
    po.volume_kg,
    po.source,
    po.source_digest,
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

-- name: MarketHasAnyObservation :one
-- P2 修復：判斷這個市場代碼在 before_date（含）之前是否曾有任何 price_observation，
-- 用來決定空回應（MOA 壅塞時的 HTTP 200 + 空 Data）要不要當成可疑訊號重試——
-- 市場尚未存在於這個歷史區間時合法回空（回填全歷史很常見），不該被誤判成資料
-- 遺失。日期上界是必要的：dev/prod 都已有 cron 持續收集的近期資料，若不設上界，
-- 回填 2012 年資料時任何市場都會因為「2026 年有資料」被誤判成「一直都有資料」，
-- 讓這個 gate 在回填全歷史時完全失效。見 usecase.PriceIngest.fetchAllWithEmptyRetry。
SELECT EXISTS (
    SELECT 1 FROM price_observation po
    JOIN market m ON m.id = po.market_id
    WHERE m.market_code = sqlc.arg(market_code)
      AND po.trade_date <= sqlc.arg(before_date)
) AS has_observation;
