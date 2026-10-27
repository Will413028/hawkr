-- name: DailyObservedPrices :many
-- One NUMERIC aggregate per (crop, calendar day), reusable by overview.
-- Resolve revisions before filtering nullable prices; never resurrect an old
-- price after a NULL correction. Cutoff is independent of the date window.
-- Existing observed eligibility: active catalog and a non-NULL average price.
WITH latest AS (
 SELECT DISTINCT ON (m.market_code, c.crop_code, po.trade_date)
  m.market_code, c.crop_code, po.trade_date, po.avg_price, po.volume_kg,
  po.available_at, po.source_digest
 FROM price_observation po
 JOIN market m ON m.id = po.market_id AND m.active
 JOIN crop c ON c.id = po.crop_id AND c.active
 WHERE c.crop_code = ANY(sqlc.arg(crop_codes)::text[])
  AND (sqlc.arg(market_code)::text = '' OR m.market_code = sqlc.arg(market_code)::text)
  AND po.trade_date BETWEEN sqlc.arg(from_date)::date AND sqlc.arg(to_date)::date
  AND po.available_at <= sqlc.arg(cutoff)::timestamptz
 ORDER BY m.market_code, c.crop_code, po.trade_date, po.revision_no DESC
), eligible AS (
 SELECT *, count(*) FILTER (WHERE volume_kg > 0)
  OVER (PARTITION BY crop_code, trade_date) AS positive_count
 FROM latest WHERE avg_price IS NOT NULL
), participants AS (
 SELECT * FROM eligible WHERE positive_count = 0 OR volume_kg > 0
)
SELECT crop_code, trade_date,
 (CASE WHEN max(positive_count) > 0
   THEN sum(avg_price * volume_kg) / sum(volume_kg)
   ELSE avg(avg_price) END)::numeric AS avg_price,
 sum(volume_kg)::numeric AS volume_kg,
 array_agg(market_code ORDER BY market_code)::text[] AS market_codes,
 array_agg(source_digest ORDER BY market_code)::text[] AS source_digests,
 max(available_at)::timestamptz AS available_at,
 (max(positive_count) = 0)::boolean AS volume_weight_fallback
FROM participants
GROUP BY crop_code, trade_date
ORDER BY crop_code, trade_date;
