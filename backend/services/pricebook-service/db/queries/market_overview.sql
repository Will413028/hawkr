-- name: OverviewMarketValid :one
SELECT (sqlc.arg(market_code)::text = '' OR EXISTS (
 SELECT 1 FROM market WHERE market_code = sqlc.arg(market_code)::text AND active
))::boolean AS valid;

-- name: OverviewCandidates :one
-- JSON here is only a typed transport for the two bounded projections. The
-- independent count subquery preserves totals even on an out-of-range page.
WITH candidates AS (
 SELECT c.id AS crop_id, m.id AS market_id, w.id AS watch_target_id,
  c.crop_code, c.name AS crop_name, c.variant_name,
  m.market_code, m.name AS market_name, (w.id IS NOT NULL) AS is_watched,
  (c.active AND coalesce(m.active, true)) AS active, w.created_at
 FROM crop c
 LEFT JOIN market m ON m.market_code = sqlc.arg(market_code)::text
 LEFT JOIN market_watch_target w ON w.account_id = sqlc.arg(account_id)::bigint
  AND w.crop_id = c.id AND w.market_id IS NOT DISTINCT FROM m.id
 WHERE sqlc.arg(scope)::text = sqlc.arg(all_scope)::text AND c.active
 UNION ALL
 SELECT c.id, m.id, w.id, c.crop_code, c.name, c.variant_name,
  m.market_code, m.name, true, (c.active AND coalesce(m.active, true)), w.created_at
 FROM market_watch_target w
 JOIN crop c ON c.id = w.crop_id
 LEFT JOIN market m ON m.id = w.market_id
 WHERE sqlc.arg(scope)::text <> sqlc.arg(all_scope)::text
  AND w.account_id = sqlc.arg(account_id)::bigint
), filtered AS (
 SELECT * FROM candidates
 WHERE strpos(lower(crop_code), lower(sqlc.arg(search_query)::text)) > 0
  OR strpos(lower(crop_name), lower(sqlc.arg(search_query)::text)) > 0
  OR strpos(lower(coalesce(variant_name, '')), lower(sqlc.arg(search_query)::text)) > 0
), paged AS (
 SELECT * FROM filtered
 ORDER BY CASE WHEN sqlc.arg(scope)::text = sqlc.arg(all_scope)::text THEN crop_code END,
  created_at DESC, watch_target_id DESC
 LIMIT sqlc.arg(page_size)::bigint OFFSET sqlc.arg(page_offset)::bigint
), options AS (
 SELECT m.market_code, m.name
 FROM market m WHERE m.active AND EXISTS (
  SELECT 1 FROM price_observation po JOIN crop c ON c.id = po.crop_id AND c.active
  WHERE po.market_id = m.id AND po.available_at <= sqlc.arg(cutoff)::timestamptz
   AND po.trade_date <= sqlc.arg(to_date)::date
 ) ORDER BY m.market_code
)
SELECT (SELECT count(*) FROM filtered)::bigint AS total_items,
 coalesce((SELECT jsonb_agg(to_jsonb(paged)) FROM paged), '[]'::jsonb)::text AS items,
 coalesce((SELECT jsonb_agg(to_jsonb(options)) FROM options), '[]'::jsonb)::text AS market_options;

-- name: OverviewObservations :many
-- Parallel arrays carry identities in page order. Zero market ID represents
-- the all-market identity; persisted IDs are positive. Resolve revisions
-- before nullable-price eligibility, exactly as DailyObservedPrices does.
WITH targets AS (
 SELECT t.crop_id, (sqlc.arg(market_ids)::bigint[])[t.ordinal] AS market_id, t.ordinal
 FROM unnest(sqlc.arg(crop_ids)::bigint[])
  WITH ORDINALITY AS t(crop_id, ordinal)
), latest AS (
 SELECT DISTINCT ON (t.ordinal, po.market_id, po.trade_date)
  t.ordinal, po.market_id, po.trade_date, po.avg_price, po.volume_kg,
  po.available_at, po.source_digest
 FROM targets t
 JOIN crop c ON c.id = t.crop_id AND c.active
 JOIN price_observation po ON po.crop_id = c.id
 JOIN market m ON m.id = po.market_id AND m.active
 WHERE (t.market_id = 0 OR t.market_id = po.market_id)
  AND po.trade_date <= sqlc.arg(to_date)::date
  AND po.available_at <= sqlc.arg(cutoff)::timestamptz
 ORDER BY t.ordinal, po.market_id, po.trade_date, po.revision_no DESC
), eligible AS (
 SELECT *, count(*) FILTER (WHERE volume_kg > 0)
  OVER (PARTITION BY ordinal, trade_date) AS positive_count
 FROM latest WHERE avg_price IS NOT NULL
), participants AS (
 SELECT * FROM eligible WHERE positive_count = 0 OR volume_kg > 0
), daily AS (
 SELECT ordinal, trade_date,
  (CASE WHEN max(positive_count) > 0 THEN sum(avg_price * volume_kg) / sum(volume_kg)
   ELSE avg(avg_price) END)::numeric AS avg_price,
  sum(volume_kg)::numeric AS volume_kg,
  array_agg(source_digest ORDER BY market_id)::text[] AS source_digests,
  max(available_at)::timestamptz AS available_at,
  (max(positive_count) = 0)::boolean AS volume_weight_fallback
 FROM participants GROUP BY ordinal, trade_date
), ranked AS (
 SELECT *, row_number() OVER (PARTITION BY ordinal ORDER BY trade_date DESC) AS day_rank,
  lead(avg_price) OVER (PARTITION BY ordinal ORDER BY trade_date DESC) AS previous_price
 FROM daily
)
SELECT ordinal::bigint AS ordinal, trade_date, avg_price::numeric AS avg_price,
 volume_kg::numeric AS volume_kg, source_digests::text[] AS source_digests,
 available_at::timestamptz AS available_at, volume_weight_fallback::boolean AS volume_weight_fallback,
 day_rank::bigint AS day_rank, (trade_date >= sqlc.arg(from_date)::date)::boolean AS in_window,
 (CASE WHEN day_rank = 1 THEN avg_price - previous_price END)::numeric AS absolute_change,
 (CASE WHEN day_rank = 1 AND previous_price <> 0
  THEN (avg_price - previous_price) / previous_price * 100 END)::numeric AS percent_change,
 coalesce((CASE WHEN day_rank = 1 THEN sign(avg_price - previous_price) END), 0)::integer AS change_sign
FROM ranked
WHERE day_rank <= 2 OR trade_date >= sqlc.arg(from_date)::date
ORDER BY ordinal, trade_date;
