-- name: ListActiveCropGroups :many
-- 每日預測 cron 的外層迴圈用：列出所有 active 的 crop_group。
SELECT group_code FROM crop_group WHERE active = true ORDER BY group_code;

-- name: HasForecastResultToday :one
-- final whole-branch review Finding I2：startDailyForecastJob 的啟動補跑
-- 檢查用。crop_group_forecast_result 是 append-only 快照表、沒有回填機制
-- （設計「範圍外」段的既有決定），runDailyAt 只在下一個排定的 21:00 UTC
-- 觸發一次——pod 剛好在那個時刻前後重啟/panic/逾時就會讓那天永遠沒有快照，
-- 且事後無法重建（當天的 price_observation/model_artifact 狀態已經是過去式）。
-- 這裡只查「今天（UTC 曆日）是否已經有任何一列」，讓 forecastJob 在
-- OnStart 時判斷要不要立刻補跑一次，不用等到下一個排定時刻。
--
-- generated_at 轉曆日一律走 AT TIME ZONE 'UTC' 再 cast，跟
-- crop_group_forecast_result_dedup_idx（schema.sql）用的是同一個轉換，
-- 避免兩處對「今天」的定義不一致。
SELECT EXISTS (
    SELECT 1 FROM crop_group_forecast_result
    WHERE (generated_at AT TIME ZONE 'UTC')::date = (now() AT TIME ZONE 'UTC')::date
) AS exists;

-- name: InsertCropGroupForecastResult :exec
-- ForecastBatch.writeGroupResult 用：一次寫入一個 group 的整批 horizon-day 列
-- （通常 7 筆，一次 GroupForecast 呼叫的 Forecast 欄位長度）。17 個陣列參數
-- 彼此等長、依索引逐列對應，呼叫端負責保證。ON CONFLICT DO NOTHING 搭配
-- crop_group_forecast_result_dedup_idx（schema.sql 定義的 unique index，
-- group_code + target_date + generated_at 的 UTC 曆日）：rolling deploy
-- 重疊導致同一天跑兩次時，第二次的寫入變成無害的 no-op。
--
-- missing_reason 用 NULLIF 把呼叫端傳的空字串轉成真正的 SQL NULL 才寫入：sqlc 對
-- 「unnest(sqlc.arg(...)::text[]) 搭配 ROWS FROM 批次寫入」這個形狀的陣列參數，無論用
-- sqlc.arg 或 sqlc.narg，個別陣列元素的 nullability 一律被忽略、固定生成非 nullable 的
-- Go slice string 型別（sqlc-dev/sqlc issue 1851，回報於 1.15.0，本 repo 用的 1.31.1
-- 仍未修——本機已用 sqlc.narg 版本 + 兩種 sqlc.yaml overrides 各自實測過，narg 版本生成
-- 結果與 arg 版本逐位元組相同，overrides 則是要嘛對這個參數完全沒作用、要嘛誤傷其他無關
-- 欄位（例如把純量的 CropGroupForecastResult.MissingReason 誤變成 nullable text 陣列），
-- 兩條路都證實走不通，故不再嘗試用 Go 型別本身表達 nullability）。改在 SQL 層轉換：
-- 呼叫端仍傳 Go slice string（sqlc 唯一會生成的型別），沒有 reason 的列傳空字串即可，
-- DB 實際落地的是真正的 NULL，不是空字串 sentinel——欄位本身維持 nullable，不改成
-- NOT NULL 加預設空字串。
INSERT INTO crop_group_forecast_result (
    id, group_code, generated_at, target_date, horizon_days,
    status, missing_reason, aggregation_method,
    effective_variant_count, total_variant_count, market_support_count,
    p10, p50, p90, freshness, model_version, dataset_digest
)
SELECT id, group_code, generated_at, target_date, horizon_days,
       status, NULLIF(missing_reason, ''), aggregation_method,
       effective_variant_count, total_variant_count, market_support_count,
       p10, p50, p90, freshness, model_version, dataset_digest
FROM ROWS FROM (
    unnest(sqlc.arg(ids)::bigint[]),
    unnest(sqlc.arg(group_codes)::text[]),
    unnest(sqlc.arg(generated_ats)::timestamptz[]),
    unnest(sqlc.arg(target_dates)::date[]),
    unnest(sqlc.arg(horizon_days)::int[]),
    unnest(sqlc.arg(statuses)::text[]),
    unnest(sqlc.arg(missing_reasons)::text[]),
    unnest(sqlc.arg(aggregation_methods)::text[]),
    unnest(sqlc.arg(effective_variant_counts)::int[]),
    unnest(sqlc.arg(total_variant_counts)::int[]),
    unnest(sqlc.arg(market_support_counts)::int[]),
    unnest(sqlc.arg(p10s)::numeric[]),
    unnest(sqlc.arg(p50s)::numeric[]),
    unnest(sqlc.arg(p90s)::numeric[]),
    unnest(sqlc.arg(freshnesses)::text[]),
    unnest(sqlc.arg(model_versions)::text[]),
    unnest(sqlc.arg(dataset_digests)::text[])
) AS t(id, group_code, generated_at, target_date, horizon_days,
       status, missing_reason, aggregation_method,
       effective_variant_count, total_variant_count, market_support_count,
       p10, p50, p90, freshness, model_version, dataset_digest)
ON CONFLICT (group_code, target_date, ((generated_at AT TIME ZONE 'UTC')::date)) DO NOTHING;
