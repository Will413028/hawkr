-- name: UpsertCategoryPriceRollup :exec
-- 每日 rollup 批次寫入用：category_id 是 PK，覆寫「目前最新」一列，不是
-- append-only（跟 crop_group_forecast_result 不同，這張表沒有歷史列）。
INSERT INTO category_price_rollup (
    category_id, level, status, latest_price, prev_price, points, source_group_count, computed_at, forecast_points,
    price_layer, unit
) VALUES (
    sqlc.arg(category_id), sqlc.arg(level), sqlc.arg(status),
    sqlc.narg(latest_price), sqlc.narg(prev_price), sqlc.narg(points),
    sqlc.arg(source_group_count), sqlc.arg(computed_at), sqlc.narg(forecast_points),
    sqlc.narg(price_layer), sqlc.narg(unit)
)
ON CONFLICT (category_id) DO UPDATE SET
    level              = EXCLUDED.level,
    status             = EXCLUDED.status,
    latest_price       = EXCLUDED.latest_price,
    prev_price         = EXCLUDED.prev_price,
    points             = EXCLUDED.points,
    source_group_count = EXCLUDED.source_group_count,
    computed_at        = EXCLUDED.computed_at,
    forecast_points    = EXCLUDED.forecast_points,
    price_layer        = EXCLUDED.price_layer,
    unit               = EXCLUDED.unit;

-- name: GetCategoryPriceRollup :one
-- 讀端點用：category_id 任一層皆可直接查。
SELECT * FROM category_price_rollup WHERE category_id = sqlc.arg(category_id);
