-- name: UpsertCommodity :one
-- 品項目錄由 adapter 在每次 ingest 時維護：來源新增魚種／欄位時自動出現，
-- 不需要另外的維護流程。name／unit／canonical_unit 會被覆寫——它們是來源或
-- 治理的當前值，不是歷史。price_layer 是 unique key 的一部分，改不了：
-- 換層級等於換成另一個品項。
INSERT INTO commodity (
    id, source, commodity_code, name, price_layer, unit, canonical_unit
) VALUES (
    sqlc.arg(id), sqlc.arg(source), sqlc.arg(commodity_code), sqlc.arg(name),
    sqlc.arg(price_layer), sqlc.arg(unit), sqlc.narg(canonical_unit)
)
ON CONFLICT (source, commodity_code, price_layer) DO UPDATE SET
    name           = EXCLUDED.name,
    unit           = EXCLUDED.unit,
    canonical_unit = EXCLUDED.canonical_unit,
    updated_at     = now()
RETURNING id;

-- name: ListCommoditiesBySource :many
SELECT * FROM commodity
WHERE source = sqlc.arg(source) AND active
ORDER BY commodity_code;

-- name: ListCommoditiesByCategory :many
-- rollup 用：取某個**L3 分類**對應的品項。
--
-- 🔴 join category_crop_mapping 而不是讀 commodity 上的欄位：rollup 的葉節點
-- 是 L3（walkNode 在 level==3 讀 l3Results[n.GetId()]），而且一條序列可以對到
-- 多個 L3——分類樹的「雞肉」L2 底下有「雞肉」與「生鮮雞肉」兩個 L3，同一條
-- 白肉雞產地價對兩者都成立。單值欄位表達不了這件事。
SELECT c.* FROM commodity c
JOIN category_crop_mapping m ON m.commodity_id = c.id
WHERE m.category_id = sqlc.arg(category_id)
  AND m.status = 'resolved'
  AND c.active
ORDER BY c.commodity_code;

-- name: UpsertCategoryCommodityMapping :exec
-- adapter 在 ingest 時宣告自己的序列對應哪些 L3 分類。
--
-- 🔴 ON CONFLICT DO NOTHING 而不是 DO UPDATE：這張表同時承載人工治理的結果
-- （admin API 寫入），一次 ingest 不該覆蓋人工判斷。adapter 只負責把「還沒有
-- 任何人表示意見」的格子填起來。
INSERT INTO category_crop_mapping (
    category_id, status, commodity_id, mapping_version, source_note, updated_by
) VALUES (
    sqlc.arg(category_id), 'resolved', sqlc.arg(commodity_id),
    sqlc.arg(mapping_version), sqlc.narg(source_note), sqlc.arg(updated_by)
)
ON CONFLICT (category_id) DO NOTHING;

-- name: InsertCommodityObservation :exec
-- append-only。同一 (品項, 市場, 交易日) 的官方修訂以遞增的 revision_no 插入，
-- 不 UPDATE；重複插入同一 revision 會被 partial unique index 擋下。
INSERT INTO commodity_observation (
    id, commodity_id, market_code, trade_date, revision_no, available_at,
    avg_price, canonical_price, quantity, quantity_unit, source, source_digest
) VALUES (
    sqlc.arg(id), sqlc.arg(commodity_id), sqlc.narg(market_code), sqlc.arg(trade_date),
    sqlc.arg(revision_no), sqlc.arg(available_at), sqlc.narg(avg_price), sqlc.narg(canonical_price),
    sqlc.narg(quantity), sqlc.narg(quantity_unit), sqlc.arg(source), sqlc.arg(source_digest)
);

-- name: GetLatestCommodityObservation :one
-- 寫入前取這一格目前最新的一列，用來決定「要不要寫」與「寫幾號」：
-- digest 相同代表上游沒改，直接跳過；不同則以 revision_no + 1 追加。
-- 查無列時回 pgx.ErrNoRows，呼叫端視為「這一格還沒有任何資料」。
--
-- 🔴 market_code 用 IS NOT DISTINCT FROM 而非 =：全國序列的 market_code 是
-- NULL，而 `market_code = NULL` 在 SQL 裡回 NULL 不是 true，會讓這支查詢對
-- 全國序列永遠查不到既有列、每次都從 revision 1 重來，接著撞上 partial
-- unique index。
SELECT revision_no, source_digest
FROM commodity_observation
WHERE commodity_id = sqlc.arg(commodity_id)
  AND market_code IS NOT DISTINCT FROM sqlc.narg(market_code)
  AND trade_date = sqlc.arg(trade_date)
ORDER BY revision_no DESC
LIMIT 1;

-- name: ListLatestCommodityObservationsAsOf :many
-- point-in-time 讀取：只看 available_at <= cutoff 的列，並在每一格
-- (品項, 市場, 交易日) 取當時可見的最新 revision。這條規則與
-- price_observation 的 as-of 查詢完全相同，刻意不另創語意。
SELECT DISTINCT ON (o.commodity_id, o.market_code, o.trade_date)
    o.*
FROM commodity_observation o
WHERE o.commodity_id = sqlc.arg(commodity_id)
  AND o.available_at <= sqlc.arg(cutoff)
  AND o.trade_date >= sqlc.arg(from_date)
ORDER BY o.commodity_id, o.market_code, o.trade_date DESC, o.revision_no DESC;

-- name: DeclareCommodityIngestPartition :exec
-- 宣告「這一格應該要有資料」。已存在則不動——declared_at 是第一次宣告的時間，
-- 減去 trade_date 可量我方的宣告延遲，被重跑覆寫就失去意義。
INSERT INTO commodity_ingest_partition (source, trade_date, status)
VALUES (sqlc.arg(source), sqlc.arg(trade_date), sqlc.arg(status))
ON CONFLICT (source, trade_date) DO NOTHING;

-- name: MarkCommodityIngestPartitionAttempt :exec
UPDATE commodity_ingest_partition
SET status = sqlc.arg(status),
    attempt_count = attempt_count + 1,
    last_attempt_at = sqlc.arg(last_attempt_at),
    completed_at = sqlc.narg(completed_at),
    last_error = sqlc.narg(last_error)
WHERE source = sqlc.arg(source) AND trade_date = sqlc.arg(trade_date);

-- name: ListIncompleteCommodityIngestPartitions :many
-- 待重試的格子。跨執行記憶：真實的失敗形態是上游要 1-3 天才上架，
-- 不是單次請求內重試幾秒就能解決的。
SELECT * FROM commodity_ingest_partition
WHERE source = sqlc.arg(source)
  AND status <> sqlc.arg(complete_status)
  AND attempt_count < sqlc.arg(max_attempts)
ORDER BY trade_date;

-- name: GetCommodityByID :one
-- 對外 API 的 commodity_code 是這個 id 的十進位字串。用 id 而非
-- (source, commodity_code) 組合：後者的 commodity_code 是上游欄位名
-- （SpecPig_AvgPrice、pt_1japt），跨來源不唯一，不能單獨當公開識別。
SELECT * FROM commodity WHERE id = sqlc.arg(id) AND active;

-- name: ListCommodityObservationsOnLatestDate :many
-- 取某品項「當時可見的最新交易日」全部市場的觀測，供全國加權平均。
--
-- 兩層 as-of：先在 available_at <= cutoff 的可見範圍內找出最大 trade_date，
-- 再取該日每個市場的最新 revision。分開兩步是因為「最新交易日」本身也依賴
-- cutoff——不能先挑日期再過濾可見性，那會在上游延遲時挑到一個當時還看不到
-- 的日期，然後回空集合。
WITH visible AS (
    SELECT * FROM commodity_observation
    WHERE commodity_id = sqlc.arg(commodity_id) AND available_at <= sqlc.arg(cutoff)
), latest_day AS (
    SELECT MAX(trade_date) AS d FROM visible
)
SELECT DISTINCT ON (v.market_code) v.*
FROM visible v, latest_day
WHERE v.trade_date = latest_day.d
ORDER BY v.market_code, v.revision_no DESC;
